package orchestration

import (
	"testing"
	"time"
)

func TestPipelineDataExtras(t *testing.T) {
	pd := PipelineData{Extra: map[string]interface{}{
		"str":  "value",
		"flag": true,
	}}
	if got := pd.GetExtraString("str"); got != "value" {
		t.Fatalf("GetExtraString = %q", got)
	}
	if got := pd.GetExtraString("missing"); got != "" {
		t.Fatalf("GetExtraString(missing) = %q", got)
	}
	if !pd.GetExtraBool("flag") {
		t.Fatal("GetExtraBool(flag=true) deve ser true")
	}
	if pd.GetExtraBool("missing") {
		t.Fatal("GetExtraBool(missing) deve ser false")
	}
	if pd.GetExtraBool("str") {
		t.Fatal("GetExtraBool(string) deve ser false (exige bool real)")
	}
}

func TestPipelineContextBuilders(t *testing.T) {
	pc := NewPipelineContext("req", "task")

	pc2 := pc.WithExtra("key", 1)
	if got, _ := pc2.Data.Extra["key"]; got != 1 {
		t.Fatalf("WithExtra: %v", pc2.Data.Extra["key"])
	}
	// Immutable: o contexto original não mudou.
	if _, ok := pc.Data.Extra["key"]; ok {
		t.Fatal("WithExtra mutou o contexto original")
	}

	pc3 := pc.WithInput("in", "v")
	if got, _ := pc3.Inputs.Extra["in"]; got != "v" {
		t.Fatalf("WithInput: %v", pc3.Inputs.Extra["in"])
	}

	pc4 := pc.WithEmbeddingError("boom")
	if pc4.Data.EmbeddingError != "boom" {
		t.Fatalf("WithEmbeddingError: %q", pc4.Data.EmbeddingError)
	}
}

func TestStageResult(t *testing.T) {
	sr := NewStageResult("build", "out", 5*time.Millisecond)
	if sr.Stage != "build" || sr.Duration != 5*time.Millisecond {
		t.Fatalf("NewStageResult: %+v", sr)
	}
	sr2 := sr.AddWarning("warn1")
	if len(sr2.Warnings) != 1 || sr2.Warnings[0] != "warn1" {
		t.Fatalf("AddWarning: %+v", sr2.Warnings)
	}
	// Immutable.
	if len(sr.Warnings) != 0 {
		t.Fatal("AddWarning mutou o original")
	}
}

func TestMetricsSnapshotReset(t *testing.T) {
	m := &OrchestrationMetrics{}
	m.RecordEmbedCacheHit()
	snap := m.Snapshot()
	if snap.EmbedCacheHits <= 0 {
		t.Fatalf("snapshot embed hits = %d", snap.EmbedCacheHits)
	}
	m.Reset()
	snap2 := m.Snapshot()
	if snap2.EmbedCacheHits != 0 {
		t.Fatalf("após Reset, embed hits = %d", snap2.EmbedCacheHits)
	}
}

func TestNewFactory(t *testing.T) {
	e := NewFactory(FactoryConfig{})
	if e == nil {
		t.Fatal("NewFactory retornou nil")
	}
}
