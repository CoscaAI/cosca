package durable

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLedger(t *testing.T) (*Ledger, func()) {
	t.Helper()
	db, err := sqlite.Open(sqlite.DefaultConfig(filepath.Join(t.TempDir(), "ledger.db")))
	require.NoError(t, err)
	l, err := NewLedger(db)
	require.NoError(t, err)
	return l, func() { require.NoError(t, db.Close()) }
}

func testRun(t *testing.T, l *Ledger) (*Run, Fencing) {
	t.Helper()
	r, err := l.Begin(context.Background(), BeginInput{RunID: "r1", WorkflowRef: "wf", InputHash: Hash([]byte("input"))})
	require.NoError(t, err)
	r, err = l.Claim(context.Background(), r.RunID, "test-worker", r.Fencing)
	require.NoError(t, err)
	return r, r.Fencing
}

func TestLedgerMigrationAndFencing(t *testing.T) {
	l, closeDB := testLedger(t)
	defer closeDB()
	r, old := func() (*Run, Fencing) {
		r, err := l.Begin(context.Background(), BeginInput{RunID: "r1", WorkflowRef: "wf", InputHash: Hash([]byte("input"))})
		require.NoError(t, err)
		return r, r.Fencing
	}()
	claimed, err := l.Claim(context.Background(), r.RunID, "worker-a", old)
	require.NoError(t, err)
	require.Equal(t, int64(2), claimed.Generation)
	require.ErrorIs(t, l.Heartbeat(context.Background(), r.RunID, old), ErrFenced)
	require.NoError(t, l.Heartbeat(context.Background(), r.RunID, claimed.Fencing))
}

func TestLedgerLeaseReadErrorsAreNotMaskedAsFenced(t *testing.T) {
	l, closeDB := testLedger(t)
	defer closeDB()
	r, err := l.Begin(context.Background(), BeginInput{RunID: "db-error", WorkflowRef: "wf", InputHash: Hash([]byte("input"))})
	require.NoError(t, err)
	_, err = l.db.Exec("DROP TABLE durable_runs")
	require.NoError(t, err)

	err = l.Heartbeat(context.Background(), r.RunID, r.Fencing)
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrFenced)

	_, err = l.Claim(context.Background(), r.RunID, "worker", r.Fencing)
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrFenced)
}

func TestLedgerCustomLeasePersistsAcrossBeginClaimAndHeartbeat(t *testing.T) {
	l, closeDB := testLedger(t)
	defer closeDB()
	want := 250 * time.Millisecond
	r, err := l.Begin(context.Background(), BeginInput{RunID: "lease", WorkflowRef: "wf", InputHash: Hash([]byte("input")), Lease: want})
	require.NoError(t, err)
	require.Equal(t, want, r.Lease)
	var stored int64
	require.NoError(t, l.db.QueryRow("SELECT lease_duration_ns FROM durable_runs WHERE run_id=?", "lease").Scan(&stored))
	require.Equal(t, want.Nanoseconds(), stored)

	claimed, err := l.Claim(context.Background(), "lease", "worker", r.Fencing)
	require.NoError(t, err)
	require.Equal(t, want, claimed.Lease)
	require.NoError(t, l.Heartbeat(context.Background(), claimed.RunID, claimed.Fencing))
	var until string
	require.NoError(t, l.db.QueryRow("SELECT lease_until FROM durable_runs WHERE run_id=?", "lease").Scan(&until))
	deadline, err := time.Parse(time.RFC3339Nano, until)
	require.NoError(t, err)
	assert.InDelta(t, want.Seconds(), time.Until(deadline).Seconds(), 0.05)
}

func TestLedgerMonotonicEventsAndAtomicCheckpoint(t *testing.T) {
	l, closeDB := testLedger(t)
	defer closeDB()
	_, fence := testRun(t, l)
	for _, typ := range []string{"started", "finished"} {
		require.NoError(t, l.AppendEvent(context.Background(), "r1", fence, Event{Type: typ, PayloadHash: Hash([]byte(typ))}))
	}
	var seq int64
	require.NoError(t, l.db.QueryRow("SELECT MAX(seq) FROM durable_events WHERE run_id=?", "r1").Scan(&seq))
	require.Equal(t, int64(2), seq)
	require.NoError(t, l.StartStep(context.Background(), "r1", "s1", fence))
	err := l.CommitStep(context.Background(), "r1", "s1", fence, Commit{Checkpoint: &Checkpoint{Key: "cp", StepKey: "s1", DataHash: Hash([]byte("cp"))}, Event: &Event{Type: "step"}, Effects: []Effect{{Key: "bad", Type: "not-registered"}}})
	require.ErrorIs(t, err, ErrUnknownEffect)
	var n int
	require.NoError(t, l.db.QueryRow("SELECT COUNT(*) FROM durable_checkpoints WHERE run_id=?", "r1").Scan(&n))
	require.Zero(t, n)
}

func TestLedgerIdempotencyAndApprovalHash(t *testing.T) {
	l, closeDB := testLedger(t)
	defer closeDB()
	_, fence := testRun(t, l)
	_, err := l.IdempotentEffect(context.Background(), "r1", fence, "k", "tool", "ref-1", Hash([]byte("result")))
	require.NoError(t, err)
	b, err := l.IdempotentEffect(context.Background(), "r1", fence, "k", "tool", "ref-2", Hash([]byte("other")))
	require.ErrorIs(t, err, ErrConflict)
	require.Nil(t, b)
	_, err = l.IdempotentEffect(context.Background(), "r1", fence, "unknown", "mystery", "", "")
	require.True(t, errors.Is(err, ErrUnknownEffect))
	approval, err := l.RecordApproval(context.Background(), "r1", fence, "a1", []byte("approval request"), "approved", "don")
	require.NoError(t, err)
	require.Equal(t, Hash([]byte("approval request")), approval.RequestHash)
	require.NotEqual(t, "approval request", approval.RequestHash)
	_, err = l.RecordApproval(context.Background(), "r1", fence, "a1", []byte("different request"), "approved", "don")
	require.ErrorIs(t, err, ErrConflict)
}

func TestLedgerLeaseExpiryAndValidation(t *testing.T) {
	l, closeDB := testLedger(t)
	defer closeDB()
	_, fence := testRun(t, l)
	_, err := l.RecordApprovalHash(context.Background(), "r1", fence, "a", "not-a-hash", "approved", "worker")
	require.Error(t, err)
	_, err = l.RecordApproval(context.Background(), "r1", fence, "a", []byte("request"), "pending", "worker")
	require.Error(t, err)
	err = l.StartStep(context.Background(), "r1", "s", fence)
	require.NoError(t, err)
	require.NoError(t, l.CompleteStep(context.Background(), "r1", "s", fence, "ref", Hash([]byte("out"))))
	_, err = l.db.Exec("UPDATE durable_runs SET lease_until=? WHERE run_id=?", "2000-01-01T00:00:00Z", "r1")
	require.NoError(t, err)
	require.ErrorIs(t, l.Heartbeat(context.Background(), "r1", fence), ErrFenced)
}

func TestLedgerReleaseFencesAllMutations(t *testing.T) {
	l, closeDB := testLedger(t)
	defer closeDB()
	_, fence := testRun(t, l)
	require.NoError(t, l.Release(context.Background(), "r1", fence))
	require.ErrorIs(t, l.Heartbeat(context.Background(), "r1", fence), ErrFenced)
	require.ErrorIs(t, l.StartStep(context.Background(), "r1", "stale", fence), ErrFenced)
	_, err := l.IdempotentEffect(context.Background(), "r1", fence, "stale", "tool", "ref", "hash")
	require.ErrorIs(t, err, ErrFenced)
}

func TestLedgerFinalizeAndStaleFinalizeFencing(t *testing.T) {
	l, closeDB := testLedger(t)
	defer closeDB()
	_, fence := testRun(t, l)
	stale := fence
	stale.Token = "stale"
	require.ErrorIs(t, l.Finalize(context.Background(), "r1", stale, RunCompleted), ErrFenced)
	require.NoError(t, l.Finalize(context.Background(), "r1", fence, RunFailed))
	var state string
	require.NoError(t, l.db.QueryRow("SELECT state FROM durable_runs WHERE run_id=?", "r1").Scan(&state))
	require.Equal(t, string(RunFailed), state)
	require.ErrorIs(t, l.Finalize(context.Background(), "r1", fence, RunCompleted), ErrFenced)
}

func TestLedgerCompletionRequiresStartedStep(t *testing.T) {
	l, closeDB := testLedger(t)
	defer closeDB()
	_, fence := testRun(t, l)
	require.ErrorIs(t, l.CompleteStep(context.Background(), "r1", "missing", fence, "ref", Hash([]byte("output"))), ErrInvalidState)
	require.ErrorIs(t, l.CommitStep(context.Background(), "r1", "missing", fence, Commit{}), ErrInvalidState)
	require.NoError(t, l.StartStep(context.Background(), "r1", "step", fence))
	require.NoError(t, l.FailStep(context.Background(), "r1", "step", fence, "error"))
	require.ErrorIs(t, l.CommitStep(context.Background(), "r1", "step", fence, Commit{}), ErrInvalidState)
}

func TestLedgerEventSequenceMultiConnection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.db")
	db1, err := sqlite.Open(sqlite.DefaultConfig(path))
	require.NoError(t, err)
	defer db1.Close()
	l1, err := NewLedger(db1)
	require.NoError(t, err)
	r, fence := testRun(t, l1)
	db2, err := sqlite.Open(sqlite.DefaultConfig(path))
	require.NoError(t, err)
	defer db2.Close()
	l2, err := NewLedger(db2)
	require.NoError(t, err)
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ledger := l1
			if i%2 == 1 {
				ledger = l2
			}
			errs <- ledger.AppendEvent(context.Background(), r.RunID, fence, Event{Type: "event"})
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var count, max int
	require.NoError(t, db1.QueryRow("SELECT COUNT(*), MAX(seq) FROM durable_events WHERE run_id=?", r.RunID).Scan(&count, &max))
	require.Equal(t, 20, count)
	require.Equal(t, 20, max)
}

func TestLedgerIdempotencyMultiConnectionWithoutForeignKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.db")
	cfg := sqlite.DefaultConfig(path)
	cfg.ForeignKeys = false
	cfg.BusyTimeout = 30000
	db1, err := sqlite.Open(cfg)
	require.NoError(t, err)
	defer db1.Close()
	l1, err := NewLedger(db1)
	require.NoError(t, err)
	r, fence := testRun(t, l1)
	db2, err := sqlite.Open(cfg)
	require.NoError(t, err)
	defer db2.Close()
	l2, err := NewLedger(db2)
	require.NoError(t, err)

	const callers = 20
	var wg sync.WaitGroup
	errs := make(chan error, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ledger := l1
			if i%2 == 1 {
				ledger = l2
			}
			_, err := ledger.IdempotentEffect(context.Background(), r.RunID, fence, "same", "tool", "ref", "hash")
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var count int
	require.NoError(t, db1.QueryRow("SELECT COUNT(*) FROM durable_idempotency WHERE run_id=? AND idempotency_key=?", r.RunID, "same").Scan(&count))
	require.Equal(t, 1, count)
}
