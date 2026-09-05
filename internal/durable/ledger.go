// Package durable implements the opt-in durable run ledger. It is intentionally
// independent of orchestration and trace: callers persist references and hashes,
// never prompts, credentials, or effect payloads.
package durable

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/sqlite"
)

var (
	ErrFenced        = errors.New("durable: stale or invalid fencing token")
	ErrUnknownEffect = errors.New("durable: unknown effect type")
	ErrNotFound      = errors.New("durable: record not found")
	ErrConflict      = errors.New("durable: idempotency conflict")
	ErrInvalidState  = errors.New("durable: invalid state transition")
)

type RunState string

const (
	RunPending   RunState = "pending"
	RunRunning   RunState = "running"
	RunCompleted RunState = "completed"
	RunFailed    RunState = "failed"
	RunReleased  RunState = "released"
)

type StepState string

const (
	StepStarted   StepState = "started"
	StepCompleted StepState = "completed"
	StepFailed    StepState = "failed"
)

type Fencing struct {
	Generation int64
	Token      string
}
type Run struct {
	RunID, WorkflowRef, InputHash              string
	State                                      RunState
	Generation                                 int64
	Fencing                                    Fencing
	Lease                                      time.Duration
	WorkerID, LeaseUntil, CreatedAt, UpdatedAt string
}
type BeginInput struct {
	RunID, WorkflowRef, InputRef string
	InputHash                    string
	Lease                        time.Duration
}
type Event struct{ Type, PayloadRef, PayloadHash string }
type Checkpoint struct{ Key, StepKey, DataRef, DataHash string }
type Effect struct{ Key, Type, ResultRef, ResultHash, ErrorRef, Status string }
type Commit struct {
	OutputRef, OutputHash, ErrorRef string
	Checkpoint                      *Checkpoint
	Event                           *Event
	Effects                         []Effect
}

// Finalize records the terminal state of a run. Only a fenced, currently
// leased worker may finalize a run; this prevents a stale worker from
// overwriting the result after another worker has claimed the lease.
func (l *Ledger) Finalize(ctx context.Context, runID string, fence Fencing, state RunState) error {
	if state != RunCompleted && state != RunFailed {
		return ErrInvalidState
	}
	unlock := l.db.LockWriter()
	defer unlock()
	tx, err := l.fencedTx(ctx, runID, fence)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE durable_runs SET state=?,worker_id='',completed_at=datetime('now'),updated_at=datetime('now') WHERE run_id=? AND state=? AND generation=? AND fencing_token_hash=?`, state, runID, RunRunning, fence.Generation, Hash([]byte(fence.Token)))
	if err != nil {
		return fmt.Errorf("durable finalize: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrFenced
	}
	return tx.Commit()
}

type IdempotencyResult struct{ Key, EffectType, Status, ResultRef, ResultHash string }
type Approval struct{ ID, RequestHash, Decision, DecidedBy, DecidedAt string }

type Ledger struct {
	db    *sqlite.DB
	lease time.Duration
}

// NewLedger does not change existing runtime flows; constructing it is the
// explicit opt-in. The normal sqlite Open path applies migration v6.
func NewLedger(db *sqlite.DB) (*Ledger, error) {
	if db == nil {
		return nil, errors.New("durable: nil database")
	}
	// Register only on this manager: normal application opens remain unchanged.
	m := sqlite.NewMigrationManager(db)
	if err := m.RegisterMigration(sqlite.DurableMigration()); err != nil {
		return nil, err
	}
	// Migration drift: the durable capability is version 6, but the default
	// knowledge schema has advanced past it (v7). Up() only applies migrations
	// with version > currentVersion, so on a database already migrated to v7
	// the v6 durable DDL would never run — and the post-apply schema
	// verification ("migration v6 drift: missing table durable_*") would fail,
	// silently disabling the ledger in production. Apply the idempotent durable
	// DDL directly first; Up() then verifies the schema and records v6 in the
	// history when the database starts below v7.
	if err := sqlite.ApplyMigrationSQL(db, sqlite.DurableMigration().UpSQL); err != nil {
		return nil, fmt.Errorf("durable schema: %w", err)
	}
	if err := m.Up(); err != nil {
		return nil, fmt.Errorf("durable migration: %w", err)
	}
	// v6 was already released before the lease duration was needed. Add the
	// nullable-at-the-migration-boundary column here so existing opt-in ledgers
	// are upgraded without changing the default (disabled) migration set.
	if _, err := db.Exec("ALTER TABLE durable_runs ADD COLUMN lease_duration_ns INTEGER NOT NULL DEFAULT 0"); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
		return nil, fmt.Errorf("durable lease migration: %w", err)
	}
	return &Ledger{db: db, lease: 30 * time.Second}, nil
}

func Hash(data []byte) string                { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
func HashApprovalRequest(data []byte) string { return Hash(data) }

func isSHA256Hex(s string) bool {
	if len(s) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func (l *Ledger) ensure(ctx context.Context) error {
	var n int
	return l.db.Conn().QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='durable_runs'").Scan(&n)
}

func (l *Ledger) Begin(ctx context.Context, in BeginInput) (*Run, error) {
	unlock := l.db.LockWriter()
	defer unlock()
	if err := l.ensure(ctx); err != nil {
		return nil, err
	}
	if in.RunID == "" || in.WorkflowRef == "" || in.InputHash == "" {
		return nil, errors.New("durable: run id, workflow ref, and input hash are required")
	}
	tok, err := newToken()
	if err != nil {
		return nil, err
	}
	effectiveLease := l.effectiveLease(in.Lease)
	lease := time.Now().UTC().Add(effectiveLease)
	_, err = l.db.Conn().ExecContext(ctx, `INSERT INTO durable_runs(run_id,workflow_ref,input_hash,state,generation,fencing_token_hash,lease_until,lease_duration_ns) VALUES(?,?,?, ?,1,?,?,?)`, in.RunID, in.WorkflowRef, in.InputHash, RunPending, Hash([]byte(tok)), lease.Format(time.RFC3339Nano), effectiveLease.Nanoseconds())
	if err != nil {
		return nil, fmt.Errorf("durable begin: %w", err)
	}
	return l.getRun(ctx, in.RunID, tok)
}

// BeginRun is the explicit lifecycle name used by integrations.
func (l *Ledger) BeginRun(ctx context.Context, in BeginInput) (*Run, error) {
	return l.Begin(ctx, in)
}

func (l *Ledger) Claim(ctx context.Context, runID, workerID string, fence Fencing) (*Run, error) {
	unlock := l.db.LockWriter()
	defer unlock()
	if runID == "" || workerID == "" || fence.Generation < 1 || fence.Token == "" {
		return nil, ErrFenced
	}
	var leaseNS int64
	if err := l.db.Conn().QueryRowContext(ctx, `SELECT lease_duration_ns FROM durable_runs WHERE run_id=? AND generation=? AND fencing_token_hash=?`, runID, fence.Generation, Hash([]byte(fence.Token))).Scan(&leaseNS); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrFenced
		}
		return nil, err
	}
	effectiveLease := time.Duration(leaseNS)
	if effectiveLease <= 0 {
		effectiveLease = l.lease
	}
	tok, err := newToken()
	if err != nil {
		return nil, err
	}
	res, err := l.db.Conn().ExecContext(ctx, `UPDATE durable_runs SET state=?, worker_id=?, generation=generation+1, fencing_token_hash=?, lease_until=?, updated_at=datetime('now') WHERE run_id=? AND generation=? AND fencing_token_hash=? AND (state=? OR (state=? AND julianday(lease_until) <= julianday('now')))`, RunRunning, workerID, Hash([]byte(tok)), time.Now().UTC().Add(effectiveLease).Format(time.RFC3339Nano), runID, fence.Generation, Hash([]byte(fence.Token)), RunPending, RunRunning)
	if err != nil {
		return nil, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, ErrFenced
	}
	return l.getRun(ctx, runID, tok)
}

func (l *Ledger) Heartbeat(ctx context.Context, runID string, fence Fencing) error {
	lease := l.lease
	var leaseNS int64
	if err := l.db.Conn().QueryRowContext(ctx, `SELECT lease_duration_ns FROM durable_runs WHERE run_id=?`, runID).Scan(&leaseNS); err != nil {
		return err
	}
	if leaseNS > 0 {
		lease = time.Duration(leaseNS)
	}
	if lease <= 0 {
		lease = l.lease
	}
	return l.mutate(ctx, `UPDATE durable_runs SET lease_until=?,updated_at=datetime('now') WHERE run_id=?`, time.Now().UTC().Add(lease).Format(time.RFC3339Nano), runID, fence)
}

func (l *Ledger) effectiveLease(requested time.Duration) time.Duration {
	if requested > 0 {
		return requested
	}
	return l.lease
}
func (l *Ledger) Release(ctx context.Context, runID string, fence Fencing) error {
	return l.mutate(ctx, `UPDATE durable_runs SET state=?,worker_id='',updated_at=datetime('now') WHERE run_id=?`, RunReleased, runID, fence)
}

func (l *Ledger) StartStep(ctx context.Context, runID, key string, fence Fencing) error {
	return l.step(ctx, runID, key, fence, StepStarted, "", "", "")
}
func (l *Ledger) CompleteStep(ctx context.Context, runID, key string, fence Fencing, outputRef, outputHash string) error {
	return l.step(ctx, runID, key, fence, StepCompleted, outputRef, outputHash, "")
}
func (l *Ledger) FailStep(ctx context.Context, runID, key string, fence Fencing, errorRef string) error {
	return l.step(ctx, runID, key, fence, StepFailed, "", "", errorRef)
}

func (l *Ledger) step(ctx context.Context, runID, key string, fence Fencing, state StepState, ref, hash, errRef string) error {
	unlock := l.db.LockWriter()
	defer unlock()
	if key == "" {
		return errors.New("durable: step key is required")
	}
	tx, err := l.fencedTx(ctx, runID, fence)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var old StepState
	err = tx.QueryRowContext(ctx, `SELECT status FROM durable_steps WHERE run_id=? AND step_key=?`, runID, key).Scan(&old)
	if errors.Is(err, sql.ErrNoRows) && state != StepStarted {
		return ErrInvalidState
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && !validStepTransition(old, state) {
		return ErrInvalidState
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO durable_steps(run_id,step_key,status,output_ref,output_hash,error_ref,started_at,completed_at) VALUES(?,?,?,?,?,?,CASE WHEN ?=? THEN datetime('now') ELSE '' END,CASE WHEN ?<>? THEN datetime('now') ELSE '' END) ON CONFLICT(run_id,step_key) DO UPDATE SET status=excluded.status,output_ref=excluded.output_ref,output_hash=excluded.output_hash,error_ref=excluded.error_ref,started_at=CASE WHEN excluded.status=? THEN datetime('now') ELSE durable_steps.started_at END,completed_at=CASE WHEN excluded.status<>? THEN datetime('now') ELSE durable_steps.completed_at END`, runID, key, state, ref, hash, errRef, state, StepStarted, state, StepStarted, StepStarted, StepStarted)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// CommitStep atomically records step completion, its checkpoint, next event,
// and effect/idempotency records. A failed insert rolls back all of them.
func (l *Ledger) CommitStep(ctx context.Context, runID, key string, fence Fencing, c Commit) error {
	unlock := l.db.LockWriter()
	defer unlock()
	tx, err := l.fencedTx(ctx, runID, fence)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var old StepState
	err = tx.QueryRowContext(ctx, `SELECT status FROM durable_steps WHERE run_id=? AND step_key=?`, runID, key).Scan(&old)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalidState
	}
	if err != nil {
		return err
	}
	if !validStepTransition(old, StepCompleted) {
		return ErrInvalidState
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO durable_steps(run_id,step_key,status,output_ref,output_hash,completed_at) VALUES(?,?,?,?,?,datetime('now')) ON CONFLICT(run_id,step_key) DO UPDATE SET status=?,output_ref=?,output_hash=?,completed_at=datetime('now')`, runID, key, StepCompleted, c.OutputRef, c.OutputHash, StepCompleted, c.OutputRef, c.OutputHash); err != nil {
		return err
	}
	if c.Checkpoint != nil {
		if err = insertCheckpoint(ctx, tx, runID, *c.Checkpoint); err != nil {
			return err
		}
	}
	if c.Event != nil {
		if err = insertEvent(ctx, tx, runID, *c.Event); err != nil {
			return err
		}
	}
	for _, e := range c.Effects {
		if err = insertEffect(ctx, tx, runID, e); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func validStepTransition(from, to StepState) bool {
	if from == "" {
		return to == StepStarted
	}
	if from == to {
		return true
	}
	return from == StepStarted && (to == StepCompleted || to == StepFailed)
}

func insertEvent(ctx context.Context, tx *sql.Tx, runID string, e Event) error {
	var seq int64
	if e.Type == "" {
		return errors.New("durable: event type is required")
	}
	if err := tx.QueryRowContext(ctx, `INSERT INTO durable_event_sequences(run_id,next_seq) VALUES(?,1) ON CONFLICT(run_id) DO UPDATE SET next_seq=durable_event_sequences.next_seq+1 RETURNING next_seq`, runID).Scan(&seq); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO durable_events(run_id,seq,event_type,payload_ref,payload_hash) VALUES(?,?,?,?,?)`, runID, seq, e.Type, e.PayloadRef, e.PayloadHash)
	return err
}
func insertCheckpoint(ctx context.Context, tx *sql.Tx, runID string, c Checkpoint) error {
	if c.Key == "" {
		return errors.New("durable: checkpoint key is required")
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO durable_checkpoints(run_id,checkpoint_key,step_key,data_ref,data_hash,seq) VALUES(?,?,?,?,?,COALESCE((SELECT MAX(seq) FROM durable_events WHERE run_id=?),0)) ON CONFLICT(run_id,checkpoint_key) DO UPDATE SET step_key=?,data_ref=?,data_hash=?,seq=excluded.seq,created_at=datetime('now')`, runID, c.Key, c.StepKey, c.DataRef, c.DataHash, runID, c.StepKey, c.DataRef, c.DataHash)
	return err
}

var knownEffects = map[string]bool{"tool": true, "external": true, "notification": true, "checkpoint": true}

func insertEffect(ctx context.Context, tx *sql.Tx, runID string, e Effect) error {
	if !knownEffects[e.Type] {
		return ErrUnknownEffect
	}
	if e.Key == "" {
		return errors.New("durable: effect key is required")
	}
	if e.Status == "" {
		e.Status = "completed"
	}
	var old Effect
	err := tx.QueryRowContext(ctx, `SELECT effect_type,status,result_ref,result_hash,error_ref FROM durable_effects WHERE run_id=? AND effect_key=?`, runID, e.Key).Scan(&old.Type, &old.Status, &old.ResultRef, &old.ResultHash, &old.ErrorRef)
	if err == nil {
		if old.Type != e.Type || old.Status != e.Status || old.ResultRef != e.ResultRef || old.ResultHash != e.ResultHash || old.ErrorRef != e.ErrorRef {
			return ErrConflict
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO durable_effects(run_id,effect_key,effect_type,status,result_ref,result_hash,error_ref) VALUES(?,?,?,?,?,?,?)`, runID, e.Key, e.Type, e.Status, e.ResultRef, e.ResultHash, e.ErrorRef)
	return err
}

func (l *Ledger) AppendEvent(ctx context.Context, runID string, fence Fencing, e Event) error {
	unlock := l.db.LockWriter()
	defer unlock()
	tx, err := l.fencedTx(ctx, runID, fence)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = insertEvent(ctx, tx, runID, e); err != nil {
		return err
	}
	return tx.Commit()
}
func (l *Ledger) Checkpoint(ctx context.Context, runID string, fence Fencing, c Checkpoint) error {
	unlock := l.db.LockWriter()
	defer unlock()
	tx, err := l.fencedTx(ctx, runID, fence)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = insertCheckpoint(ctx, tx, runID, c); err != nil {
		return err
	}
	return tx.Commit()
}

func (l *Ledger) IdempotentEffect(ctx context.Context, runID string, fence Fencing, key, typ, resultRef, resultHash string) (*IdempotencyResult, error) {
	// A transaction-level immediate lock is the cross-process serialization
	// primitive. modernc can still surface SQLITE_BUSY at lock acquisition;
	// retry the whole read/insert/re-read unit so callers never observe a
	// spurious failure during a short writer handoff.
	for attempt := 0; ; attempt++ {
		result, err := l.idempotentEffectOnce(ctx, runID, fence, key, typ, resultRef, resultHash)
		if !isBusy(err) || attempt >= 50 {
			return result, err
		}
		wait := time.Duration(attempt+1) * 10 * time.Millisecond
		if wait > 100*time.Millisecond {
			wait = 100 * time.Millisecond
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func isBusy(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "SQLITE_BUSY") || strings.Contains(s, "database is locked")
}

func (l *Ledger) idempotentEffectOnce(ctx context.Context, runID string, fence Fencing, key, typ, resultRef, resultHash string) (*IdempotencyResult, error) {
	unlock := l.db.LockWriter()
	defer unlock()
	if !knownEffects[typ] {
		return nil, ErrUnknownEffect
	}
	if key == "" {
		return nil, errors.New("durable: idempotency key is required")
	}
	tx, err := l.fencedTx(ctx, runID, fence)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var r IdempotencyResult
	err = tx.QueryRowContext(ctx, `SELECT idempotency_key,effect_type,status,result_ref,result_hash FROM durable_idempotency WHERE run_id=? AND idempotency_key=?`, runID, key).Scan(&r.Key, &r.EffectType, &r.Status, &r.ResultRef, &r.ResultHash)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, `INSERT INTO durable_idempotency(run_id,idempotency_key,effect_type,status,result_ref,result_hash) VALUES(?,?,?,'completed',?,?)`, runID, key, typ, resultRef, resultHash)
		if err == nil {
			err = tx.QueryRowContext(ctx, `SELECT idempotency_key,effect_type,status,result_ref,result_hash FROM durable_idempotency WHERE run_id=? AND idempotency_key=?`, runID, key).Scan(&r.Key, &r.EffectType, &r.Status, &r.ResultRef, &r.ResultHash)
		}
	}
	if err != nil {
		return nil, err
	}
	if r.EffectType != typ || r.ResultRef != resultRef || r.ResultHash != resultHash {
		return nil, ErrConflict
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &r, nil
}

func (l *Ledger) RecordApproval(ctx context.Context, runID string, fence Fencing, id string, request []byte, decision, by string) (*Approval, error) {
	return l.RecordApprovalHash(ctx, runID, fence, id, HashApprovalRequest(request), decision, by)
}
func (l *Ledger) RecordApprovalHash(ctx context.Context, runID string, fence Fencing, id, requestHash, decision, by string) (*Approval, error) {
	unlock := l.db.LockWriter()
	defer unlock()
	if id == "" || len(requestHash) != 64 || !isSHA256Hex(requestHash) || decision == "" || by == "" {
		return nil, errors.New("durable: approval hash must be sha-256")
	}
	if decision != "approved" && decision != "rejected" {
		return nil, errors.New("durable: invalid approval decision")
	}
	tx, err := l.fencedTx(ctx, runID, fence)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var a Approval
	err = tx.QueryRowContext(ctx, `SELECT approval_id,request_hash,decision,decided_by,decided_at FROM durable_approvals WHERE run_id=? AND approval_id=?`, runID, id).Scan(&a.ID, &a.RequestHash, &a.Decision, &a.DecidedBy, &a.DecidedAt)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, `INSERT INTO durable_approvals(run_id,approval_id,request_hash,decision,decided_by) VALUES(?,?,?,?,?)`, runID, id, requestHash, decision, by)
		if err == nil {
			err = tx.QueryRowContext(ctx, `SELECT approval_id,request_hash,decision,decided_by,decided_at FROM durable_approvals WHERE run_id=? AND approval_id=?`, runID, id).Scan(&a.ID, &a.RequestHash, &a.Decision, &a.DecidedBy, &a.DecidedAt)
		}
	}
	if err != nil {
		return nil, err
	}
	if a.RequestHash != requestHash || a.Decision != decision || a.DecidedBy != by {
		return nil, ErrConflict
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &a, nil
}

func (l *Ledger) fencedTx(ctx context.Context, runID string, f Fencing) (*sql.Tx, error) {
	if f.Generation < 1 || f.Token == "" {
		return nil, ErrFenced
	}
	tx, err := l.db.Conn().BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	var n int
	err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM durable_runs WHERE run_id=? AND state=? AND generation=? AND fencing_token_hash=? AND julianday(lease_until) > julianday('now')`, runID, RunRunning, f.Generation, Hash([]byte(f.Token))).Scan(&n)
	if err != nil || n != 1 {
		tx.Rollback()
		return nil, ErrFenced
	}
	return tx, nil
}
func (l *Ledger) checkFence(ctx context.Context, runID string, f Fencing) error {
	var n int
	err := l.db.Conn().QueryRowContext(ctx, `SELECT COUNT(*) FROM durable_runs WHERE run_id=? AND state=? AND generation=? AND fencing_token_hash=? AND julianday(lease_until) > julianday('now')`, runID, RunRunning, f.Generation, Hash([]byte(f.Token))).Scan(&n)
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrFenced
	}
	return nil
}
func (l *Ledger) mutate(ctx context.Context, q string, args ...interface{}) error {
	unlock := l.db.LockWriter()
	defer unlock()
	f := args[len(args)-1].(Fencing)
	args = args[:len(args)-1]
	tx, err := l.fencedTx(ctx, args[1].(string), f)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, q, args...); err != nil {
		return err
	}
	return tx.Commit()
}
// RecoverStale releases runs that are stuck in Running state with an expired
// lease. Pending runs that have never been claimed after 2x the lease duration
// are released as well so a fresh Run can be started. Returns the count of
// released runs.
func (l *Ledger) RecoverStale(ctx context.Context) (int, error) {
	unlock := l.db.LockWriter()
	defer unlock()

	// Release Running runs whose lease has expired.
	result, err := l.db.Conn().ExecContext(ctx,
		`UPDATE durable_runs SET state=?,worker_id='',completed_at=datetime('now'),updated_at=datetime('now')
		 WHERE state=? AND julianday(lease_until) <= julianday('now')
		   AND lease_duration_ns > 0`,
		RunReleased, RunRunning)
	if err != nil {
		return 0, fmt.Errorf("durable recover stale running: %w", err)
	}
	nRunning, _ := result.RowsAffected()

	// Release Pending runs that have not been claimed within 2x the lease.
	defaultLease := l.lease.Seconds()
	result2, err := l.db.Conn().ExecContext(ctx,
		fmt.Sprintf(`UPDATE durable_runs SET state=?,completed_at=datetime('now'),updated_at=datetime('now')
		 WHERE state=? AND julianday(created_at) + %f <= julianday('now')
		   AND lease_duration_ns = 0`, defaultLease*2),
		RunReleased, RunPending)
	if err != nil {
		return int(nRunning), fmt.Errorf("durable recover stale pending: %w", err)
	}
	nPending, _ := result2.RowsAffected()

	return int(nRunning) + int(nPending), nil
}

func (l *Ledger) getRun(ctx context.Context, id, tok string) (*Run, error) {
	var r Run
	var leaseNS int64
	err := l.db.Conn().QueryRowContext(ctx, `SELECT run_id,workflow_ref,input_hash,state,generation,worker_id,lease_until,created_at,updated_at,lease_duration_ns FROM durable_runs WHERE run_id=?`, id).Scan(&r.RunID, &r.WorkflowRef, &r.InputHash, &r.State, &r.Generation, &r.WorkerID, &r.LeaseUntil, &r.CreatedAt, &r.UpdatedAt, &leaseNS)
	if err != nil {
		return nil, err
	}
	r.Fencing = Fencing{Generation: r.Generation, Token: tok}
	r.Lease = time.Duration(leaseNS)
	if r.Lease <= 0 {
		r.Lease = l.lease
	}
	return &r, nil
}
