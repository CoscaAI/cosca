package authority

import (
	"path/filepath"
	"testing"
	"time"
)

// ─── Helpers herméticos (t.TempDir(), sem tocar o repo real) ────────────────

// mustWrite writes content to path, creating parent directories.
func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := mkdirAll(filepath.Dir(path)); err != nil {
		t.Fatalf("mkdir %q: %v", filepath.Dir(path), err)
	}
	if err := os_WriteFile(path, content); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}

// mustSetMtime forces a distinct mtime on a file so NEWER classification is
// deterministic regardless of filesystem clock granularity.
func mustSetMtime(t *testing.T, path string, tm time.Time) {
	t.Helper()
	if err := osChtimes(path, tm); err != nil {
		t.Fatalf("chtimes %q: %v", path, err)
	}
}

// setFrozenBase/frozenNewer-ish timestamps used across tests.
var (
	tPast   = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	tFuture = time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
)

func TestDrift_StateMatrix(t *testing.T) {
	// T1 MATCH, T2 FROZEN_NEWER, T3 LIVE_NEWER, T5 ONLY_FROZEN, T6 ONLY_LIVE,
	// T4 divergent content (never ties silently).
	frozen := t.TempDir()
	live := t.TempDir()

	// T1: identical file in both zones.
	mustWrite(t, filepath.Join(frozen, "same.md"), "idêntico")
	mustWrite(t, filepath.Join(live, "same.md"), "idêntico")

	// T2: FROZEN_NEWER — same path, different content, FROZEN mtime newer.
	mustWrite(t, filepath.Join(frozen, "v2.md"), "frozen-v2")
	mustWrite(t, filepath.Join(live, "v2.md"), "live-v1")
	mustSetMtime(t, filepath.Join(frozen, "v2.md"), tFuture)
	mustSetMtime(t, filepath.Join(live, "v2.md"), tPast)

	// T3: LIVE_NEWER — same path, different content, LIVE mtime newer.
	mustWrite(t, filepath.Join(frozen, "v3.md"), "frozen-v1")
	mustWrite(t, filepath.Join(live, "v3.md"), "live-v2")
	mustSetMtime(t, filepath.Join(frozen, "v3.md"), tPast)
	mustSetMtime(t, filepath.Join(live, "v3.md"), tFuture)

	// T4: divergent, equal mtime (tie) → must NOT tie silently; falls to FROZEN_NEWER.
	mustWrite(t, filepath.Join(frozen, "tie.md"), "TIE-frozen")
	mustWrite(t, filepath.Join(live, "tie.md"), "TIE-live")
	mustSetMtime(t, filepath.Join(frozen, "tie.md"), tPast)
	mustSetMtime(t, filepath.Join(live, "tie.md"), tPast)

	// T5: ONLY_FROZEN (ex.: blocks/, merkle/, chain.dat).
	mustWrite(t, filepath.Join(frozen, "blocks", "agent1.md"), "bloco-frozen")
	mustWrite(t, filepath.Join(frozen, "chain.dat"), "cadena")

	// T6: ONLY_LIVE (evolução ainda não promovida).
	mustWrite(t, filepath.Join(live, "skills", "evoluir.md"), "aprendizado-live")

	rep, err := RunDrift(frozen, live)
	if err != nil {
		t.Fatalf("RunDrift: %v", err)
	}

	expect := map[string]State{
		"same.md":            StateMatch,
		"v2.md":              StateFrozenNewer,
		"v3.md":              StateLiveNewer,
		"tie.md":             StateFrozenNewer, // tie → FROZEN authority
		"blocks/agent1.md":   StateOnlyFrozen,
		"chain.dat":          StateOnlyFrozen,
		"skills/evoluir.md":  StateOnlyLive,
	}

	byPath := map[string]State{}
	for _, e := range rep.Entries {
		byPath[e.Path] = e.State
		if e.State == StateMatch && e.Action != "none" {
			t.Errorf("MATCH deve ter action=none, obteve %q", e.Action)
		}
		if e.State == StateOnlyFrozen && e.Action != "preserve" {
			t.Errorf("ONLY_FROZEN deve ter action=preserve, obteve %q", e.Action)
		}
		if e.State == StateLiveNewer && e.Action != "propose_promotion" {
			t.Errorf("LIVE_NEWER deve ter action=propose_promotion, obteve %q", e.Action)
		}
	}

	for path, want := range expect {
		got, ok := byPath[path]
		if !ok {
			t.Errorf("estado de %q ausente", path)
			continue
		}
		if got != want {
			t.Errorf("%q = %s, esperado %s", path, got, want)
		}
	}

	// Totals aggregation.
	if rep.Totals[string(StateMatch)] != 1 {
		t.Errorf("Totals[MATCH] = %d, esperado 1", rep.Totals[string(StateMatch)])
	}
	if rep.Totals[string(StateFrozenNewer)] != 2 {
		t.Errorf("Totals[FROZEN_NEWER] = %d, esperado 2 (v2 + tie)", rep.Totals[string(StateFrozenNewer)])
	}
	if rep.Totals[string(StateLiveNewer)] != 1 {
		t.Errorf("Totals[LIVE_NEWER] = %d, esperado 1", rep.Totals[string(StateLiveNewer)])
	}
	if rep.Totals[string(StateOnlyFrozen)] != 2 {
		t.Errorf("Totals[ONLY_FROZEN] = %d, esperado 2", rep.Totals[string(StateOnlyFrozen)])
	}
	if rep.Totals[string(StateOnlyLive)] != 1 {
		t.Errorf("Totals[ONLY_LIVE] = %d, esperado 1", rep.Totals[string(StateOnlyLive)])
	}

	// HasDrift / DriftCount.
	if !rep.HasDrift() {
		t.Errorf("rep.HasDrift() = false, esperado true")
	}
	if rep.DriftCount() != 4 { // v2, v3, tie, skills/evoluir
		t.Errorf("DriftCount() = %d, esperado 4", rep.DriftCount())
	}

	// Determinism: run again, same output.
	rep2, _ := RunDrift(frozen, live)
	if len(rep2.Entries) != len(rep.Entries) {
		t.Fatalf("RunDrift não-idempotente: %d vs %d entradas", len(rep2.Entries), len(rep.Entries))
	}
	for i := range rep.Entries {
		if rep.Entries[i] != rep2.Entries[i] {
			t.Errorf("RunDrift não-determinístico na entrada %d: %+v vs %+v", i, rep.Entries[i], rep2.Entries[i])
		}
	}
}

func TestDrift_FrozenAbsent_AllOnlyLive(t *testing.T) {
	// FROZEN não existe em disco (framework só embutido) → tudo ONLY_LIVE.
	frozen := filepath.Join(t.TempDir(), "frozen-not-there")
	live := t.TempDir()
	mustWrite(t, filepath.Join(live, "a.md"), "a")

	rep, err := RunDrift(frozen, live)
	if err != nil {
		t.Fatalf("RunDrift: %v", err)
	}
	if rep.Comparable {
		t.Errorf("Comparable = true, esperado false (FROZEN ausente)")
	}
	if len(rep.Entries) != 1 || rep.Entries[0].State != StateOnlyLive {
		t.Errorf("esperado 1 ONLY_LIVE, obteve %+v", rep.Entries)
	}
}

func TestDrift_CanonicalPathWindowsSafe(t *testing.T) {
	// A canonicalização usa "/" mesmo com separador nativo (anti filepath.Rel bug).
	frozen := t.TempDir()
	live := t.TempDir()
	mustWrite(t, filepath.Join(frozen, "a", "b.md"), "x")
	mustWrite(t, filepath.Join(live, "a", "b.md"), "x")

	rep, err := RunDrift(frozen, live)
	if err != nil {
		t.Fatalf("RunDrift: %v", err)
	}
	if len(rep.Entries) != 1 {
		t.Fatalf("esperado 1 entrada, obteve %d", len(rep.Entries))
	}
	if rep.Entries[0].Path != "a/b.md" {
		t.Errorf("caminho canônico = %q, esperado %q", rep.Entries[0].Path, "a/b.md")
	}
	if rep.Entries[0].State != StateMatch {
		t.Errorf("estado = %s, esperado %s", rep.Entries[0].State, StateMatch)
	}
}
