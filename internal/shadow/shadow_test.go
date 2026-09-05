package shadow

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/deliberate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sampleTrace builds a deterministic ShadowTrace for the store round-trip tests.
func sampleTrace(id, requestID string, decision ShadowDecision, confidence float64) ShadowTrace {
	return ShadowTrace{
		RequestID:     requestID,
		Agent:         "cosca-backend",
		Decision:      decision,
		WouldEscalate: decision.WillEscalate(),
		WouldRespond:  "",
		Confidence:    confidence,
		Convergence:   0.60,
		EvidenceIDs:   []string{"ev:1", "ev:2"},
		Positions:     2,
		Reason:        "conflicting evidence",
		Breakdown:     "base=0.60 adjust=-0.05 final=0.55",
		DurationMs:    3,
		At:            time.Now().Add(time.Duration(id[0]) * time.Millisecond),
	}
}

func TestStore_AppendRead_RoundTrip(t *testing.T) {
	store := NewStore(t.TempDir())
	tr := sampleTrace("a", "req-1", EMIT_OK, 0.82)

	require.NoError(t, store.Append(tr))

	got, err := store.Read()
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, tr.RequestID, got[0].RequestID)
	assert.Equal(t, tr.Agent, got[0].Agent)
	assert.Equal(t, tr.Decision, got[0].Decision)
	assert.Equal(t, tr.WouldEscalate, got[0].WouldEscalate)
	assert.Equal(t, tr.Confidence, got[0].Confidence)
	assert.Equal(t, tr.Convergence, got[0].Convergence)
	assert.Equal(t, tr.EvidenceIDs, got[0].EvidenceIDs)
	assert.Equal(t, tr.Positions, got[0].Positions)
	assert.Equal(t, tr.Reason, got[0].Reason)
	assert.Equal(t, tr.Breakdown, got[0].Breakdown)
	assert.Equal(t, tr.DurationMs, got[0].DurationMs)
	assert.WithinDuration(t, tr.At, got[0].At, time.Second)
}

func TestStore_Read_MissingFile_Empty(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "nope"))
	got, err := store.Read()
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestStore_Read_ToleratesCorruptedLine(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	require.NoError(t, store.Append(sampleTrace("a", "req-1", ESCALATE, 0.20)))
	// Inject a corrupted line directly into the JSONL file.
	require.NoError(t, appendLine(filepath.Join(dir, RecordsFile), []byte("{not json")))

	got, err := store.Read()
	require.NoError(t, err)
	assert.Len(t, got, 1, "corrupted line must be skipped, never dropping the report")
}

func TestStore_List_FiltersByRequestID(t *testing.T) {
	store := NewStore(t.TempDir())
	require.NoError(t, store.Append(sampleTrace("a", "req-a", EMIT_OK, 0.80)))
	require.NoError(t, store.Append(sampleTrace("b", "req-b", ESCALATE, 0.30)))

	all, err := store.List("")
	require.NoError(t, err)
	assert.Len(t, all, 2)

	onlyA, err := store.List("req-a")
	require.NoError(t, err)
	require.Len(t, onlyA, 1)
	assert.Equal(t, "req-a", onlyA[0].RequestID)

	none, err := store.List("req-zzz")
	require.NoError(t, err)
	assert.Empty(t, none)
}

func TestAggregate_DecisionDistribution(t *testing.T) {
	traces := []ShadowTrace{
		sampleTrace("a", "r1", EMIT_OK, 0.85),             // not escalate
		sampleTrace("b", "r2", RETRIEVAL_INSUFFICIENT, 0.4), // escalate
		sampleTrace("c", "r3", ESCALATE, 0.3),              // escalate
		sampleTrace("d", "r4", EMIT_WITH_RESERVATIONS, 0.6), // escalate
	}
	s := Aggregate(traces)
	assert.Equal(t, 4, s.Total)
	assert.Equal(t, 1, s.Decisions[string(EMIT_OK)])
	assert.Equal(t, 1, s.Decisions[string(RETRIEVAL_INSUFFICIENT)])
	assert.Equal(t, 1, s.Decisions[string(ESCALATE)])
	assert.Equal(t, 1, s.Decisions[string(EMIT_WITH_RESERVATIONS)])
	// 3 of 4 would escalate.
	assert.InDelta(t, 0.75, s.EscalationRate, 1e-9)
	assert.InDelta(t, 0.25, s.SelfResolveRate, 1e-9)
	assert.InDelta(t, (0.85+0.4+0.3+0.6)/4, s.AvgConfidence, 1e-9)
}

func TestAggregate_Empty(t *testing.T) {
	s := Aggregate(nil)
	assert.Equal(t, 0, s.Total)
	assert.Equal(t, 0.0, s.EscalationRate)
	assert.Equal(t, 0.0, s.SelfResolveRate)
	assert.Equal(t, 0.0, s.AvgConfidence)
	assert.Empty(t, s.Decisions)
}

func TestBuildReport_Histogram(t *testing.T) {
	traces := []ShadowTrace{
		sampleTrace("a", "r1", EMIT_OK, 0.95),
		sampleTrace("b", "r2", EMIT_WITH_RESERVATIONS, 0.60),
		sampleTrace("c", "r3", ESCALATE, 0.30),
	}
	rep := BuildReport(traces)
	require.Len(t, rep.Histogram, 6)
	// 0.30 → bin [0.00,0.50); 0.60 → bin [0.60,0.70); 0.95 → bin [0.90,1.01).
	assert.Equal(t, 1, rep.Histogram[0].Count) // 0.30 → 0.00-0.50
	assert.Equal(t, 0, rep.Histogram[1].Count) // 0.50-0.60
	assert.Equal(t, 1, rep.Histogram[2].Count) // 0.60-0.70
	assert.Equal(t, 0, rep.Histogram[3].Count) // 0.70-0.80
	assert.Equal(t, 0, rep.Histogram[4].Count) // 0.80-0.90
	assert.Equal(t, 1, rep.Histogram[5].Count) // 0.95 → 0.90-1.01
}

func TestClassify_Taxonomy(t *testing.T) {
	cases := []struct {
		name       string
		verdict    deliberate.Emit
		positions  int
		want       ShadowDecision
		wantEscal  bool
	}{
		{"no evidence -> retrieval insufficient", deliberate.Escalate, 0, RETRIEVAL_INSUFFICIENT, true},
		{"emit ok -> emit ok", deliberate.EmitOK, 2, EMIT_OK, false},
		{"emit with reservations -> emit with reservations", deliberate.EmitWithReservations, 2, EMIT_WITH_RESERVATIONS, true},
		{"escalate -> escalate", deliberate.Escalate, 2, ESCALATE, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Classify(c.verdict, c.positions)
			assert.Equal(t, c.want, got)
			assert.Equal(t, c.wantEscal, got.WillEscalate())
		})
	}
}

// appendLine appends raw bytes as a line to the JSONL file (test helper).
func appendLine(path string, b []byte) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return err
}
