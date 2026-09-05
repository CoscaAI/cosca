package workstate

import (
	"strings"
	"testing"
)

// =============================================================================
// State — copy-on-write, checkpoint/rollback, tombstones, caps/pruning.
// =============================================================================

// TestStateSetGetRoundtrip verifica que Set/Get funcionam no estado corrente.
func TestStateSetGetRoundtrip(t *testing.T) {
	t.Parallel()
	st := NewState(DefaultCaps())
	st.Set("var", "value-1")
	st.Set("handle", "task-42")

	if v, ok := st.Get("var"); !ok || v != "value-1" {
		t.Errorf("Get(var) = %q, %v; want value-1,true", v, ok)
	}
	if v, ok := st.Get("handle"); !ok || v != "task-42" {
		t.Errorf("Get(handle) = %q, %v; want task-42,true", v, ok)
	}
	if st.Len() != 2 {
		t.Errorf("Len = %d, want 2", st.Len())
	}
	if v, ok := st.Get("missing"); ok {
		t.Errorf("Get(missing) = %q, %v; want \"\",false", v, ok)
	}
}

// TestStateCopyOnWriteBaseUntouched verifica que o Set NÃO muta o base até o
// Checkpoint — propriedade O(1) do copy-on-write (rollback O(edits)).
func TestStateCopyOnWriteBaseUntouched(t *testing.T) {
	t.Parallel()
	st := NewState(DefaultCaps())
	st.Set("a", "1")

	// BaseSnapshot é o estado COMMITADO — antes de qualquer checkpoint é vazio.
	base := st.BaseSnapshot()
	if _, ok := base["a"]; ok {
		t.Fatalf("base deveria estar vazio antes do checkpoint, possui a: %+v", base)
	}

	st.Set("a", "99")
	st.Set("b", "novo")

	// Durante o delta, Get lê do delta; o base lógico (sem o delta) intacto.
	if v, _ := st.Get("a"); v != "99" {
		t.Errorf("delta deveria ser visível no Get: %s", v)
	}
	if b := st.BaseSnapshot(); len(b) != 0 {
		t.Errorf("base não deve mudar durante o delta (O(1) rollback): %+v", b)
	}
	if st.DirtyCount() != 2 {
		t.Errorf("DirtyCount = %d, want 2 (a e b)", st.DirtyCount())
	}
}

// TestStateRollbackDiscardsDelta verifica que Rollback restaura o base intacto.
func TestStateRollbackDiscardsDelta(t *testing.T) {
	t.Parallel()
	st := NewState(DefaultCaps())
	st.Set("a", "1")
	st.Checkpoint() // base = {a:1}

	st.Set("a", "999")
	st.Set("c", "3")
	if st.DirtyCount() != 2 {
		t.Fatalf("DirtyCount = %d, want 2", st.DirtyCount())
	}

	st.Rollback()

	if v, _ := st.Get("a"); v != "1" {
		t.Errorf("rollback deveria restaurar a=1: %s", v)
	}
	if _, ok := st.Get("c"); ok {
		t.Error("rollback deveria remover a chave nova c")
	}
	if st.DirtyCount() != 0 {
		t.Errorf("após rollback DirtyCount = %d, want 0", st.DirtyCount())
	}
}

// TestStateCheckpointCommitsDelta verifica que Checkpoint sela delta→base.
func TestStateCheckpointCommitsDelta(t *testing.T) {
	t.Parallel()
	st := NewState(DefaultCaps())
	st.Set("a", "1")
	st.Set("b", "novo")
	st.Checkpoint()

	base := st.BaseSnapshot()
	if base["a"] != "1" || base["b"] != "novo" {
		t.Errorf("checkpoint deveria commitar o delta: %+v", base)
	}
	if st.DirtyCount() != 0 {
		t.Errorf("após checkpoint DirtyCount = %d, want 0", st.DirtyCount())
	}
}

// TestStateDeleteTombstone verifica que Delete esconde a entrada (e nem base
// nem delta são mutados até o Checkpoint).
func TestStateDeleteTombstone(t *testing.T) {
	t.Parallel()
	st := NewState(DefaultCaps())
	st.Set("a", "1")
	st.Set("b", "2")
	st.Checkpoint() // base = {a:1,b:2}

	st.Delete("a")

	if _, ok := st.Get("a"); ok {
		t.Error("após Delete, Get(a) deveria ser false (tombstone)") //nolint:revive // mensagem clara
	}
	if _, ok := st.Get("b"); !ok {
		t.Error("Get(b) deveria continuar true")
	}
	// base ainda tem a (tombstone é só no delta) até o checkpoint.
	if _, ok := st.BaseSnapshot()["a"]; !ok {
		t.Errorf("base ainda deve conter a até o checkpoint: %+v", st.BaseSnapshot())
	}

	st.Checkpoint()
	if _, ok := st.BaseSnapshot()["a"]; ok {
		t.Errorf("após checkpoint, a deveria ser removida do base: %+v", st.BaseSnapshot())
	}
}

// TestStateDeltaSnapshotSetsAndDeletes verifica o par (sets, deletes) usado
// na serialização do delta O(k).
func TestStateDeltaSnapshotSetsAndDeletes(t *testing.T) {
	t.Parallel()
	st := NewState(DefaultCaps())
	st.Set("keep", "k")
	st.Set("kill", "k2")
	st.Checkpoint()

	st.Set("keep", "kk")   // update → set
	st.Set("add", "new")   // add → set
	st.Delete("kill")      // delete → tombstones

	sets, deletes := st.DeltaSnapshot()
	if sets["keep"] != "kk" || sets["add"] != "new" {
		t.Errorf("sets = %+v, want keep=kk add=new", sets)
	}
	if len(sets) != 2 {
		t.Errorf("len(sets) = %d, want 2", len(sets))
	}
	if len(deletes) != 1 || deletes[0] != "kill" {
		t.Errorf("deletes = %v, want [kill]", deletes)
	}
}

// TestStateDirtyKeysDeterministic verifica que DirtyKeys vem ordenado (para
// determinismo I1 na serialização).
func TestStateDirtyKeysDeterministic(t *testing.T) {
	t.Parallel()
	st := NewState(DefaultCaps())
	st.Set("z", "1")
	st.Set("a", "2")
	st.Set("m", "3")

	keys := st.DirtyKeys()
	want := []string{"a", "m", "z"}
	for i, k := range keys {
		if k != want[i] {
			t.Fatalf("DirtyKeys[%d] = %q, want %q (deterministico)", i, k, want[i])
		}
	}
}

// TestStateCapsTruncate verifica o cap por entrada no modo TRUNCATE.
func TestStateCapsTruncate(t *testing.T) {
	t.Parallel()
	caps := Caps{MaxEntryBytes: 4, TruncateOverflow: true}
	st := NewState(caps)

	st.Set("big", "abcdefgh") // 8 bytes > 4
	v, ok := st.Get("big")
	if !ok {
		t.Fatalf("no modo truncate, a entrada deve existir")
	}
	if v != "abcd" {
		t.Errorf("valor truncado = %q, want %q (primeiros 4 bytes)", v, "abcd")
	}
}

// TestStateCapsOmit verifica o cap por entrada no modo OMIT (drop da entrada).
func TestStateCapsOmit(t *testing.T) {
	t.Parallel()
	caps := Caps{MaxEntryBytes: 4, TruncateOverflow: false}
	st := NewState(caps)

	st.Set("big", "abcdefgh") // 8 bytes > 4 → omite a entrada
	if _, ok := st.Get("big"); ok {
		t.Error("no modo omit, a entrada acima do cap não deve existir")
	}
	if st.Len() != 0 {
		t.Errorf("Len = %d, want 0 (entrada omitida)", st.Len())
	}
}

// TestStateTotalCapPruningLRU verifica o pruning determinístico (LRU) quando o
// total excede MaxTotalBytes — as entradas menos recentemente atualizadas caem.
func TestStateTotalCapPruningLRU(t *testing.T) {
	t.Parallel()
	// cap total = 10 bytes (2 entradas de 5 bytes) → a mais antiga cai.
	caps := Caps{MaxTotalBytes: 10, TruncateOverflow: true}
	st := NewState(caps)

	st.Set("old", "11111") // 5 bytes, touchSeq=1
	st.Set("new", "22222") // 5 bytes, touchSeq=2 → total=10, ok

	// Tocamos "new" (para que seja a mais nova) e "old" caiu de fator.
	st.Set("new", "22222") // re-toca new, touchSeq=3 (mais nova)

	// Agora adicionamos uma 3ª → total=15 > 10 → pruning LRU.
	st.Set("third", "33333") // touchSeq=4

	// A mais antiga (old) deve ter sido descartada para caber.
	if _, ok := st.Get("old"); ok {
		t.Errorf("pruning LRU deveria ter descartado a entrada 'old': %+v", st.Entries())
	}
	if _, ok := st.Get("new"); !ok {
		t.Errorf("'new' deveria ter sido mantida")
	}
	if _, ok := st.Get("third"); !ok {
		t.Errorf("'third' deveria ter sido mantida")
	}
	if st.TotalBytes() > 10 {
		t.Errorf("TotalBytes = %d, want <= 10", st.TotalBytes())
	}
}

// TestStateEntriesMergedView verifica a visão fusionada base ∪ delta − deletes.
func TestStateEntriesMergedView(t *testing.T) {
	t.Parallel()
	st := NewState(DefaultCaps())
	st.Set("a", "1")
	st.Set("b", "2")
	st.Checkpoint()

	st.Set("a", "999") // atualiza (delta)
	st.Set("c", "3")   // nova (delta)
	st.Delete("b")     // deleta (tombstone)

	entries := st.Entries()
	if entries["a"] != "999" {
		t.Errorf("a = %q, want 999 (delta sobrepõe base)", entries["a"])
	}
	if entries["c"] != "3" {
		t.Errorf("c = %q, want 3", entries["c"])
	}
	if _, ok := entries["b"]; ok {
		t.Errorf("b deveria estar oculta (deleted): %+v", entries)
	}
	if len(entries) != 2 {
		t.Errorf("len(entries) = %d, want 2", len(entries))
	}
}

// TestStateNewStateWithBase verifica a reconstrução a partir de um base (cold).
func TestStateNewStateWithBase(t *testing.T) {
	t.Parallel()
	base := map[string]string{"a": "1", "b": "2"}
	st := NewStateWithBase(base, DefaultCaps())

	if v, _ := st.Get("a"); v != "1" {
		t.Errorf("Get(a) = %q, want 1 (do base)", v)
	}
	// base não é mutado.
	if len(base) != 2 {
		t.Errorf("base de entrada deveria ser copiado, não referenciado: %+v", base)
	}
}

// TestStateCapsDisabled verifica que caps=0 significa ilimitado.
func TestStateCapsDisabled(t *testing.T) {
	t.Parallel()
	caps := Caps{} // 0 = ilimitado
	st := NewState(caps)
	st.Set("big", strings.Repeat("x", 100000))
	if v, _ := st.Get("big"); len(v) != 100000 {
		t.Errorf("len(value) = %d, want 100000 (sem cap)", len(v))
	}
}
