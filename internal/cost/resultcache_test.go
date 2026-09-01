package cost

import (
	"testing"
	"time"
)

func TestTaskFingerprint_NormalizesEquivalentTasks(t *testing.T) {
	a := TaskFingerprint("Como fazer X no Bubblewrap?")
	b := TaskFingerprint("  como  fazer x no bubblewrap!! ")
	if a != b {
		t.Fatalf("tarefas equivalentes devem ter a MESMA fingerprint: %s vs %s", a, b)
	}
	if TaskFingerprint("tarefa A") == TaskFingerprint("tarefa B") {
		t.Fatal("tarefas diferentes devem ter fingerprints diferentes")
	}
}

func TestResultCache_HitSameSnapshot(t *testing.T) {
	c := NewResultCache(0)
	fp := TaskFingerprint("fazer deploy")
	snap := "snapshot-A"

	c.Set(fp, snap, "resultado-A")
	got, ok := c.Get(fp, snap)
	if !ok || got != "resultado-A" {
		t.Fatalf("esperava hit, got ok=%v value=%q", ok, got)
	}
}

func TestResultCache_VersionSafeNoMix(t *testing.T) {
	// CRITÉRIO DO ADR-031 Fase 1: duas tarefas iguais em versões diferentes
	// de conhecimento → cache NÃO mistura.
	c := NewResultCache(0)
	fp := TaskFingerprint("fazer deploy")

	c.Set(fp, "snapshot-A", "resultado-snap-A")
	got, ok := c.Get(fp, "snapshot-B")
	if ok {
		t.Fatalf("VERSION-SAFE VIOLADO: resultado do snapshot-A servido sob snapshot-B: %q", got)
	}

	// E o correto continua funcionando.
	got, ok = c.Get(fp, "snapshot-A")
	if !ok || got != "resultado-snap-A" {
		t.Fatalf("hit correto falhou: ok=%v value=%q", ok, got)
	}
}

func TestResultCache_MissOnDifferentTask(t *testing.T) {
	c := NewResultCache(0)
	c.Set(TaskFingerprint("tarefa 1"), "snap", "r1")
	if _, ok := c.Get(TaskFingerprint("tarefa 2"), "snap"); ok {
		t.Fatal("tarefa diferente não deveria dar hit")
	}
}

func TestResultCache_TTLExpiry(t *testing.T) {
	c := NewResultCache(50 * time.Millisecond)
	fp := TaskFingerprint("t")
	c.Set(fp, "snap", "valor")
	if _, ok := c.Get(fp, "snap"); !ok {
		t.Fatal("esperava hit antes do TTL")
	}
	time.Sleep(120 * time.Millisecond)
	if _, ok := c.Get(fp, "snap"); ok {
		t.Fatal("esperava miss após expirar o TTL")
	}
}

func TestResultCache_Clear(t *testing.T) {
	c := NewResultCache(0)
	c.Set("fp", "snap", "v")
	c.Clear()
	if c.Len() != 0 {
		t.Fatalf("Clear deveria esvaziar, Len=%d", c.Len())
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	type payload struct {
		Answer string `json:"answer"`
		Steps  int    `json:"steps"`
	}
	orig := payload{Answer: "ok", Steps: 3}
	raw, err := MarshalResult(orig)
	if err != nil {
		t.Fatalf("MarshalResult: %v", err)
	}
	var back payload
	if err := UnmarshalResult(raw, &back); err != nil {
		t.Fatalf("UnmarshalResult: %v", err)
	}
	if back != orig {
		t.Fatalf("round-trip divergiu: %+v", back)
	}
}
