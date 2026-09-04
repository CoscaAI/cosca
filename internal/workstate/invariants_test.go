package workstate

import (
	"reflect"
	"sort"
	"testing"
)

// =============================================================================
// Helpers de comparação profunda (provas de invariantes).
// =============================================================================

// assertState checks que o State bate com uma expectativa EXPLÍCITA de
// (present map, absent keys, dirty keys). Comparação profunda: values,
// has-vs-deleted, e o conjunto de chaves sujas — não só um campo.
func assertState(t *testing.T, st *State, present map[string]string, absent []string, dirty []string) {
	t.Helper()

	entries := st.Entries()
	if len(entries) != len(present) {
		t.Fatalf("Entries() len = %d, want %d (present): %v", len(entries), len(present), entries)
	}
	for k, v := range present {
		got, ok := st.Get(k)
		if !ok || got != v {
			t.Errorf("Get(%q) = %q, %v; want %q,true", k, got, ok, v)
		}
		if !st.Has(k) {
			t.Errorf("Has(%q) = false, want true (present)", k)
		}
	}
	for _, k := range absent {
		if st.Has(k) {
			t.Errorf("Has(%q) = true, want false (absent/tombstone)", k)
		}
		if _, ok := st.Get(k); ok {
			t.Errorf("Get(%q) deveria retornar false (ausente/tombstone, não vazar do base)", k)
		}
	}
	if dirty != nil {
		want := append([]string(nil), dirty...)
		sort.Strings(want)
		if got := st.DirtyKeys(); !reflect.DeepEqual(got, want) {
			t.Errorf("DirtyKeys() = %v, want %v", got, want)
		}
		if st.DirtyCount() != len(want) {
			t.Errorf("DirtyCount() = %d, want %d", st.DirtyCount(), len(want))
		}
	}
}

// =============================================================================
// Invariantes de COW + tombstones + caps.
// =============================================================================

// Set → Get.
func TestStateSetGet(t *testing.T) {
	t.Parallel()
	st := NewState(DefaultCaps())
	st.Set("k", "v")
	assertState(t, st, map[string]string{"k": "v"}, nil, []string{"k"})
}

// Set → Checkpoint → Get.
func TestStateSetCheckpointGet(t *testing.T) {
	t.Parallel()
	st := NewState(DefaultCaps())
	st.Set("a", "1")
	st.Checkpoint()
	// Após checkpoint: a está no base, sem dirty; Get deve ler do base.
	assertState(t, st, map[string]string{"a": "1"}, nil, nil)
	if v, _ := st.Get("a"); v != "1" {
		t.Errorf("Get(a) = %q, want 1", v)
	}
}

// Set → Rollback → Get (o delta é descartado).
func TestStateSetRollbackGet(t *testing.T) {
	t.Parallel()
	st := NewState(DefaultCaps())
	st.Set("a", "1")
	st.Checkpoint() // base {a:1}
	st.Set("b", "2")
	st.Rollback()
	assertState(t, st, map[string]string{"a": "1"}, []string{"b"}, nil)
}

// Delete → Get/Has.
func TestStateDeleteGetHas(t *testing.T) {
	t.Parallel()
	st := NewState(DefaultCaps())
	st.Set("a", "1")
	st.Delete("a")
	assertState(t, st, map[string]string{}, []string{"a"}, []string{"a"})
}

// Delete → Checkpoint → Get/Has (tombstone vira remoção definitiva do base).
func TestStateDeleteCheckpointGetHas(t *testing.T) {
	t.Parallel()
	st := NewState(DefaultCaps())
	st.Set("a", "1")
	st.Checkpoint() // base {a:1}
	st.Delete("a")
	st.Checkpoint()
	assertState(t, st, map[string]string{}, []string{"a"}, nil)
	if _, ok := st.BaseSnapshot()["a"]; ok {
		t.Errorf("após Delete+Checkpoint, a deveria sumir do base: %+v", st.BaseSnapshot())
	}
}

// Múltiplos Set da MESMA chave — o último vence.
func TestStateMultipleSetLastWins(t *testing.T) {
	t.Parallel()
	st := NewState(DefaultCaps())
	st.Set("k", "v1")
	st.Set("k", "v2")
	st.Set("k", "v3")
	if v, _ := st.Get("k"); v != "v3" {
		t.Errorf("Get(k) = %q, want v3 (último vence)", v)
	}
	// A mesma chave suja NÃO conta 3x — é 1 entry dirty.
	if st.DirtyCount() != 1 {
		t.Errorf("DirtyCount = %d, want 1 (chave única, não 3)", st.DirtyCount())
	}
}

// Set após Delete da MESMA chave — o tombstone é limpo e o set volta a valer.
func TestStateSetAfterDelete(t *testing.T) {
	t.Parallel()
	st := NewState(DefaultCaps())
	st.Set("a", "1")
	st.Delete("a")
	st.Set("a", "2") // re-set
	if v, _ := st.Get("a"); v != "2" {
		t.Errorf("Get(a) = %q, want 2 (set após delete limpa o tombstone)", v)
	}
	if !st.Has("a") {
		t.Error("Has(a) = false, want true")
	}
	// Não há tombstone pendente após o re-set.
	if _, deletes := st.DeltaSnapshot(); len(deletes) != 0 {
		t.Errorf("deletes = %v, want vazio (tombstone limpo pelo re-set)", deletes)
	}
}

// Tombstone deve esconder uma chave EXISTENTE NA BASE (não vazar o base).
func TestStateTombstoneHidesBase(t *testing.T) {
	t.Parallel()
	st := NewStateWithBase(map[string]string{"a": "base-value", "b": "2"}, DefaultCaps())
	st.Delete("a") // tombstone sobre a-base
	// Has=false e Get NÃO retorna o valor do base.
	assertState(t, st, map[string]string{"b": "2"}, []string{"a"}, []string{"a"})
	if _, ok := st.Get("a"); ok {
		t.Error("Get(a) não deve vazar o valor da base (tombstone)")
	}
	// O base lógico ainda TEM a (a remoção só se materializa no Checkpoint).
	if _, ok := st.BaseSnapshot()["a"]; !ok {
		t.Errorf("base ainda deve conter a até o checkpoint: %+v", st.BaseSnapshot())
	}
}

// DirtyCount dedicado.
func TestStateDirtyCount(t *testing.T) {
	t.Parallel()
	st := NewState(DefaultCaps())
	if st.DirtyCount() != 0 {
		t.Errorf("DirtyCount inicial = %d, want 0", st.DirtyCount())
	}
	st.Set("a", "1")
	st.Set("b", "2")
	st.Set("c", "3")
	if st.DirtyCount() != 3 {
		t.Errorf("DirtyCount = %d, want 3", st.DirtyCount())
	}
	st.Checkpoint()
	if st.DirtyCount() != 0 {
		t.Errorf("DirtyCount após checkpoint = %d, want 0", st.DirtyCount())
	}
	st.Set("d", "4")
	st.Delete("a") // a foi commitado no base, agora é tombstone → dirty
	if st.DirtyCount() != 2 {
		t.Errorf("DirtyCount = %d, want 2 (d + tombstone a)", st.DirtyCount())
	}
}

// DeltaSnapshot determinístico: deletes ordenados + sets corretos.
func TestStateDeltaSnapshotDeterministic(t *testing.T) {
	t.Parallel()
	st := NewState(DefaultCaps())
	st.Set("a", "1")
	st.Set("b", "2")
	st.Set("c", "3")
	st.Checkpoint() // base {a,b,c}

	st.Set("a", "updated") // set
	st.Set("d", "new")     // set
	st.Delete("c")         // delete (do base)
	st.Delete("a")         // delete (do base)* — mas a foi re-setado acima (não está no delta).

	sets, deletes := st.DeltaSnapshot()
	// a foi re-setado e depois deletado na MESMA sessão → colapsa para tombstone
	// (net effect = nao existe), então NÃO está em sets, entra em deletes.
	// c → delete (do base). Determinístico (deletes ordenado).
	if len(sets) != 1 || sets["d"] != "new" {
		t.Errorf("sets = %+v, want d=new (não deve conter 'a')", sets)
	}
	if _, ok := sets["a"]; ok {
		t.Errorf("sets should not contain 'a' (re-set then delete → tombstone)")
	}
	if !reflect.DeepEqual(deletes, []string{"a", "c"}) {
		t.Errorf("deletes = %v, want [a c] (determinístico)", deletes)
	}
}

// Caps por entrada — truncate (MaxEntryBytes).
func TestStateCapsEntryTruncate(t *testing.T) {
	t.Parallel()
	st := NewState(Caps{MaxEntryBytes: 4, TruncateOverflow: true})
	st.Set("big", "abcdefgh")
	if v, ok := st.Get("big"); !ok || v != "abcd" {
		t.Errorf("Get(big) = %q, %v; want \"abcd\",true (truncado)", v, ok)
	}
}

// Caps por entrada — drop (MaxEntryBytes, TruncateOverflow=false).
func TestStateCapsEntryDrop(t *testing.T) {
	t.Parallel()
	st := NewState(Caps{MaxEntryBytes: 4, TruncateOverflow: false})
	st.Set("big", "abcdefgh")
	assertState(t, st, map[string]string{}, []string{"big"}, []string{"big"})
}

// MaxTotalBytes + pruning determinístico (LRU — o menos tocado cai).
func TestStateMaxTotalPruningLRU(t *testing.T) {
	t.Parallel()
	// cap total = 10 → 2 entradas de 5. Uma 3ª estoura e evicta a menos tocada.
	st := NewState(Caps{MaxTotalBytes: 10, TruncateOverflow: true})
	st.Set("a", "11111") // seq1
	st.Set("b", "22222") // seq2 → total 10 (ok)
	st.Set("c", "33333") // seq3 → total 15 > 10 → evicta a (seq1, menos tocada)
	assertState(t, st, map[string]string{"b": "22222", "c": "33333"}, []string{"a"}, []string{"b", "c"})
}

// Checkpoint depois de pruning — o estado evictado é selado; dirty zera.
func TestStateCheckpointAfterPruning(t *testing.T) {
	t.Parallel()
	st := NewState(Caps{MaxTotalBytes: 10, TruncateOverflow: true})
	st.Set("a", "11111") // seq1
	st.Set("b", "22222") // seq2 → total 10
	st.Set("c", "33333") // seq3 → evicta a → {b,c}
	st.Checkpoint()
	// Depois do checkpoint: base {b,c}, sem dirty, a permanece evictada.
	assertState(t, st, map[string]string{"b": "22222", "c": "33333"}, []string{"a"}, nil)
	if _, ok := st.BaseSnapshot()["a"]; ok {
		t.Errorf("a deveria permanecer evictada do base após checkpoint: %+v", st.BaseSnapshot())
	}
}

// Rollback depois de pruning — o delta é descartado; a evicção é PERMANENTE
// (não ressuscita a entrada evictada, que já saiu do base).
func TestStateRollbackAfterPruning(t *testing.T) {
	t.Parallel()
	st := NewState(Caps{MaxTotalBytes: 10, TruncateOverflow: true})
	st.Set("keep", "11111") // seq1
	st.Set("also", "22222") // seq2 → total 10
	st.Checkpoint()         // base {keep, also}
	st.Set("boom", "33333") // seq3 → total 15 → evicta keep (seq1, menos tocada) → {also, boom}
	// 'keep' foi evictado do base; 'boom' é delta-only.
	st.Rollback() // descarta o delta (boom) → base {also}; 'keep' continua evictado.
	assertState(t, st, map[string]string{"also": "22222"}, []string{"keep", "boom"}, nil)
}

// NewStateWithBase NÃO deve compartilhar o mapa do chamador (nem por cópia
// superficial): mudanças no caller não afetam o State, e vice-versa.
func TestNewStateWithBaseNoSharedMap(t *testing.T) {
	t.Parallel()
	callerBase := map[string]string{"a": "1"}
	st := NewStateWithBase(callerBase, DefaultCaps())

	// Mutar o SNAPSHOT em memória não afeta o mapa do caller.
	st.Set("a", "changed")
	if callerBase["a"] != "1" {
		t.Errorf("caller base foi mutado pelo State: %q", callerBase["a"])
	}
	// Mutar o mapa do caller após a construção não afeta o State.
	callerBase["b"] = "injected"
	if st.Has("b") {
		t.Error("State não deveria ver entradas adicionadas ao base do caller")
	}
}

// =============================================================================
// Item 4 — sequência de recuperação (base/checkpoint → mutations → snapshot →
// Load/restore → mutations → Checkpoint → Rollback), comparando por semântica.
// =============================================================================

func TestStateRecoverySequenceSemanticIdentity(t *testing.T) {
	t.Parallel()
	caps := DefaultCaps()
	s, err := NewStore(t.TempDir(), caps)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// 1) base/checkpoint (estado commitado).
	s.Set("a", "1")
	s.Set("b", "2")
	s.Set("c", "3")
	if _, err := s.Checkpoint(); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}

	// 2) mutations (dirty).
	s.Set("a", "1-updated") // update
	s.Set("d", "4")         // add
	s.Delete("b")           // delete (tombstone sobre o base)

	// 3) snapshot (delta persiste SÓ o que mudou).
	if _, err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// 4) Load/restore num "processo novo".
	s2, err := NewStore(s.dir, caps)
	if err != nil {
		t.Fatalf("NewStore (reattach): %v", err)
	}
	if err := s2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Estado restaurado = base {a,b,c} + delta {a upd, d add, b deleted}.
	// Semântica: a=1-updated, b ausente (tombstone), c=3, d=4. dirty = a,d,b.
	assertState(t, s2.State(), map[string]string{
		"a": "1-updated", "c": "3", "d": "4",
	}, []string{"b"}, []string{"a", "b", "d"})

	// 5) mutations no estado restaurado.
	s2.Set("e", "5") // add
	s2.Delete("c")   // delete (do base restaurado)

	// 6) Checkpoint (sela tudo).
	if _, err := s2.Checkpoint(); err != nil {
		t.Fatalf("Checkpoint (2): %v", err)
	}
	// Depois do checkpoint: base = {a=1-updated, d=4, e=5}; b e c removidos.
	baseAfter2 := s2.State().BaseSnapshot()
	wantBase := map[string]string{"a": "1-updated", "d": "4", "e": "5"}
	if !mapsEqual(baseAfter2, wantBase) {
		t.Errorf("BaseSnapshot pós-checkpoint = %v, want %v", baseAfter2, wantBase)
	}
	assertState(t, s2.State(), wantBase, []string{"b", "c"}, nil)

	// 7) Rollback (desfaz o delta — não deve tocar o base commitado).
	s2.Set("f", "6")            // dirty
	s2.Set("a", "corrupt")      // dirty
	s2.State().Rollback()       // Rollback é do State (o Store delega)
	// Estado volta ao checkpoint: a=1-updated, d=4, e=5; f e 'corrupt' somem.
	assertState(t, s2.State(), wantBase, []string{"b", "c", "f"}, nil)
	if v, _ := s2.Get("a"); v != "1-updated" {
		t.Errorf("Get(a) = %q, want 1-updated (rollback restaurou o base)", v)
	}

	// Ainda dá prova do delta O(k): o 1o Save gravou só 3 chaves (a,d,b).
	if got := s.Stats().DeltaUploaded; got != 3 {
		t.Errorf("DeltaUploaded = %d, want 3 (a upd, d add, b delete)", got)
	}
}

// mapsEqual compara dois map[string]string (nil-safe).
func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		bv, ok := b[k]
		if !ok || bv != v {
			return false
		}
	}
	return true
}
