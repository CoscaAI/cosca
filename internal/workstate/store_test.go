package workstate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// =============================================================================
// Store — snapshot/restore, delta O(1), escrita atômica, manifest.
// =============================================================================

// helper: cria um store num TempDir com caps padrão.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(t.TempDir(), DefaultCaps())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

// (a) snapshot→restore retorna o estado exato.
func TestStoreSaveLoadRoundtrip(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)

	// ARRANGE: working-state com várias entradas.
	s.Set("task", "task-42")
	s.Set("partial", "análise incremental em andamento")
	s.Set("handle", "handle-7")

	if _, err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// ACT: "detach" (novo Store) e reattach (Load).
	dir := s.dir
	s2, err := NewStore(dir, DefaultCaps())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := s2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// ASSERT: estado exato preservado.
	want := map[string]string{
		"task":    "task-42",
		"partial": "análise incremental em andamento",
		"handle":  "handle-7",
	}
	for k, v := range want {
		if got, ok := s2.Get(k); !ok || got != v {
			t.Errorf("Get(%s) = %q, %v; want %q,true", k, got, ok, v)
		}
	}
	if s2.Len() != len(want) {
		t.Errorf("Len = %d, want %d", s2.Len(), len(want))
	}
}

// (b) detach/reattach preserva entradas em andamento (sem precisar de
// checkpoint — o Save persiste o delta; o Load reconstrói base+delta).
func TestStoreDetachReattachPreservesInProgress(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)

	// ARRANGE: trabalho em andamento (dirty, sem checkpoint).
	s.Set("progress", "50%")
	s.Set("partial_result", "lint: 12 warnings, 3 errors")
	// Set depois de um Save (delta contínuo).
	if _, err := s.Save(); err != nil {
		t.Fatalf("Save #1: %v", err)
	}
	s.Set("progress", "75%") // atualiza após o 1o save (dirty novo)

	// ACT: reattach num processo novo.
	s2, err := NewStore(s.dir, DefaultCaps())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := s2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// ASSERT: a mudança mais recente (feita APÓS o Save) é preservada porque
	// foi escrito no disco pelo 2o Save.
	if _, err := s.Save(); err != nil {
		t.Fatalf("Save #2: %v", err)
	}
	s3, _ := NewStore(s.dir, DefaultCaps())
	if err := s3.Load(); err != nil {
		t.Fatalf("Load #2: %v", err)
	}
	if v, _ := s3.Get("progress"); v != "75%" {
		t.Errorf("progress = %q, want 75%% (sobrevive a detach/reattach)", v)
	}
	if v, _ := s3.Get("partial_result"); v != "lint: 12 warnings, 3 errors" {
		t.Errorf("partial_result = %q", v)
	}
	if s3.Len() != 2 {
		t.Errorf("Len = %d, want 2", s3.Len())
	}
	// O primeiro Load (antes do 2o Save) ainda vê o estado do 1o Save.
	if v, _ := s2.Get("progress"); v != "50%" {
		t.Errorf("progress no 1o snapshot = %q, want 50%%", v)
	}
}

// (c) restauração é O(k) do delta — SÓ as entradas mudadas são re-lidas/gravadas.
// O upload do snapshot grava k (não total); o Load aplica k (não total).
// Verificamos que esse valor NÃO cresce com o tamanho do estado total.
func TestStoreDeltaO1Scale(t *testing.T) {
	t.Parallel()

	runCase := func(t *testing.T, nEntries, kDelta int) (uploaded, applied uint64) {
		t.Helper()
		s := newTestStore(t)
		// base com nEntries entradas.
		for i := 0; i < nEntries; i++ {
			s.Set(fmt.Sprintf("var-%03d", i), fmt.Sprintf("val-%d", i))
		}
		if _, err := s.Checkpoint(); err != nil {
			t.Fatalf("Checkpoint: %v", err)
		}
		// muta exatamente kDelta entradas (i%3 padrão determinístico).
		for i := 0; i < kDelta; i++ {
			s.Set(fmt.Sprintf("var-%03d", i), fmt.Sprintf("mod-%d", i))
		}
		snap, err := s.Save()
		if err != nil {
			t.Fatalf("Save: %v", err)
		}
		if snap.IsCheckpoint {
			t.Fatal("Save não deve marcar checkpoint")
		}
		uploaded = s.Stats().DeltaUploaded

		// reattach num processo novo.
		s2, _ := NewStore(s.dir, DefaultCaps())
		if err := s2.Load(); err != nil {
			t.Fatalf("Load: %v", err)
		}
		applied = s2.Stats().DeltaApplied
		return uploaded, applied
	}

	// Case 1: 100 entradas, delta de 4.
	u1, a1 := runCase(t, 100, 4)
	if u1 != 4 {
		t.Errorf("DeltaUploaded = %d, want 4 (O(k), não O(total)=100)", u1)
	}
	if a1 != 4 {
		t.Errorf("DeltaApplied = %d, want 4 (O(k))", a1)
	}
	if a1 >= 100 {
		t.Errorf("DeltaApplied (%d) deve ser < total (%d)", a1, 100)
	}

	// Case 2: MUITO mais entradas (10000), mesmo delta de 4 → os contadores de
	// delta NÃO devem crescer (é o delta que escala com k, não com o estado).
	u2, a2 := runCase(t, 10000, 4)
	if u2 != 4 {
		t.Errorf("DeltaUploaded com N=10000 = %d, want 4 (constante em k)", u2)
	}
	if a2 != 4 {
		t.Errorf("DeltaApplied com N=10000 = %d, want 4 (constante em k)", a2)
	}
}

// (d) caps/pruning: entrada acima do cap é truncada (save/load preserva a regra).
func TestStoreCapsPersisted(t *testing.T) {
	t.Parallel()
	// ARRANGE: cap por entrada = 4, truncate.
	s, err := NewStore(t.TempDir(), Caps{MaxEntryBytes: 4, TruncateOverflow: true})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	s.Set("text", "aaaaaaaaaaaaaaaa") // 16 bytes > cap 4

	// ACT: save + load.
	if _, err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	s2, _ := NewStore(s.dir, Caps{MaxEntryBytes: 4, TruncateOverflow: true})
	if err := s2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// ASSERT: truncado no load (a regra é re-aplicada deterministicamente).
	if v, ok := s2.Get("text"); !ok || v != "aaaa" {
		t.Errorf("Get(text) = %q, %v; want \"aaaa\",true (truncado a 4 bytes)", v, ok)
	}
}

// (d2) caps/pruning — modo OMIT: entrada acima do cap some do estado.
func TestStoreCapsOmitPersisted(t *testing.T) {
	t.Parallel()
	s, err := NewStore(t.TempDir(), Caps{MaxEntryBytes: 3, TruncateOverflow: false})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	s.Set("small", "ok")
	s.Set("huge", "toolarge") // > 3 → omitido

	if _, err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	s2, _ := NewStore(s.dir, Caps{MaxEntryBytes: 3, TruncateOverflow: false})
	if err := s2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := s2.Get("huge"); ok {
		t.Error("entrada acima do cap (omit) não deveria existir")
	}
	if v, _ := s2.Get("small"); v != "ok" {
		t.Errorf("Get(small) = %q, want ok", v)
	}
	// O manifest também não deve listar 'huge' como presente no estado corrente.
	if s2.Len() != 1 {
		t.Errorf("Len = %d, want 1", s2.Len())
	}
}

// (e) escrita atômica — se falhar no meio, o estado anterior permanece íntegro.
func TestStoreAtomicWriteFailureKeepsPriorIntegrity(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	// ARRANGE: um snapshot íntegro (gen 1).
	s.Set("a", "1")
	s.Set("b", "2")
	if _, err := s.Save(); err != nil {
		t.Fatalf("Save baseline: %v", err)
	}
	baselineGen := s.Generation()
	baselineMan, err := s.LastManifest()
	if err != nil || baselineMan == nil {
		t.Fatalf("LastManifest: %v %v", err, baselineMan)
	}

	// muta (dirty que seria gravado no próximo Save).
	s.Set("a", "999")

	// ACT: força falha no RENAME do manifest (2a escrita do Save), injetando um
	// rename que falha na 2a chamada (1a = delta). Simula crash/falha NO MEIO.
	calls := 0
	s.rename = func(old, new string) error {
		calls++
		if calls == 2 {
			return errors.New("simulated rename failure (mid-write)")
		}
		return os.Rename(old, new)
	}
	_, err = s.Save()

	// ASSERT: Save falhou E o snapshot anterior continua íntegro.
	if err == nil {
		t.Fatal("Save deveria falhar com o rename injetado")
	}
	// Manifest em disco ainda aponta para a geração anterior.
	man, _ := s.LastManifest()
	if man.Generation != baselineGen {
		t.Errorf("Generation após falha = %d, want %d (manifest anterior íntegro)", man.Generation, baselineGen)
	}
	if man.DeltaFile != baselineMan.DeltaFile {
		t.Errorf("DeltaFile mudou: %q -> %q", baselineMan.DeltaFile, man.DeltaFile)
	}
	// Estado em memória preserva o dirty (não foi perdido).
	if v, _ := s.Get("a"); v != "999" {
		t.Errorf("em memória o dirty deveria permanecer após falha: a=%q", v)
	}
	// Gerador não avançou.
	if s.Generation() != baselineGen {
		t.Errorf("Generation in-memory = %d, want %d", s.Generation(), baselineGen)
	}
}

// TestStoreAtomicWriteHelper: a unidade atômica — destino fica intacto se o
// rename falhar.
func TestStoreAtomicWriteHelper(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	dest := filepath.Join(s.dir, "payload.bin")

	// ARRANGE: destino já tem conteúdo antigo.
	if err := os.WriteFile(dest, []byte("OLD-CONTENT"), 0o644); err != nil {
		t.Fatalf("seed dest: %v", err)
	}

	// ACT: renomeação falha.
	s.rename = func(_, _ string) error { return errors.New("boom") }
	err := s.atomicWrite(dest, []byte("NEW-CONTENT"))

	// ASSERT: erro, e o destino continua com o conteúdo ANTIGO (nunca parcial).
	if err == nil {
		t.Fatal("atomicWrite deveria falhar")
	}
	data, _ := os.ReadFile(dest)
	if string(data) != "OLD-CONTENT" {
		t.Errorf("destino deve permanecer com conteúdo antigo após falha: %q", string(data))
	}
	// Nenhum tmp órfão deixado para trás.
	leftovers, _ := filepath.Glob(filepath.Join(s.dir, ".ws-tmp-*"))
	if len(leftovers) != 0 {
		t.Errorf("tmp órfão deve ser limpo: %v", leftovers)
	}
}

// (f) manifest correto — descreve o snapshot (entries, tamanhos, geração).
func TestStoreManifestCorrect(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	s.SetName("agent-42")

	s.Set("task", "abc")         // 3 bytes
	s.Set("handle", "heap-123")  // 8 bytes
	if _, err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	man, err := s.LastManifest()
	if err != nil || man == nil {
		t.Fatalf("LastManifest: %v %v", err, man)
	}
	if man.Generation != 1 {
		t.Errorf("Generation = %d, want 1", man.Generation)
	}
	if !stringsHasSuffix(man.ID, "@1") {
		t.Errorf("ID = %q, want sufixo @1", man.ID)
	}
	if man.DeltaFile == "" {
		t.Error("DeltaFile deveria estar preenchido após um Save (não checkpoint)")
	}
	if man.TotalBytes != 3+8 {
		t.Errorf("TotalBytes = %d, want 11", man.TotalBytes)
	}
	// Manifest em disco é JSON válido e bate com o objeto.
	raw, _ := os.ReadFile(filepath.Join(s.dir, ManifestFilename))
	var disk Manifest
	if err := json.Unmarshal(raw, &disk); err != nil {
		t.Fatalf("manifest.json não é JSON válido: %v", err)
	}
	if len(disk.Entries) != 2 {
		t.Errorf("manifest Entries = %d, want 2", len(disk.Entries))
	}
	if disk.Entries["task"].Size != 3 {
		t.Errorf("Entries[task].Size = %d, want 3", disk.Entries["task"].Size)
	}
	if disk.Entries["handle"].Size != 8 {
		t.Errorf("Entries[handle].Size = %d, want 8", disk.Entries["handle"].Size)
	}
}

// TestStoreCheckpointManifestApontaBase: após Checkpoint o manifest aponta para
// o arquivo base (não delta).
func TestStoreCheckpointManifestApontaBase(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	s.Set("x", "1")
	s.Set("y", "2")
	if _, err := s.Checkpoint(); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}

	man, _ := s.LastManifest()
	if man == nil {
		t.Fatal("manifest nil após checkpoint")
	}
	if man.BaseFile == "" {
		t.Error("após Checkpoint, BaseFile deveria estar preenchido")
	}
	if man.DeltaFile != "" {
		t.Errorf("após Checkpoint, DeltaFile deveria ser vazio, foi %q", man.DeltaFile)
	}
	if !s.ExistsBaseFile(man.BaseFile) {
		t.Errorf("arquivo base %q não existe no disco", man.BaseFile)
	}
}

// ExistsBaseFile é um helper de teste que verifica a presença de um arquivo.
func (s *Store) ExistsBaseFile(name string) bool {
	_, err := os.Stat(filepath.Join(s.dir, name))
	return err == nil
}

// TestStoreLoadNoSnapshot: Load sem nenhum snapshot devolve estado vazio.
func TestStoreLoadNoSnapshot(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	if err := s.Load(); err != nil {
		t.Fatalf("Load em store vazio: %v", err)
	}
	if s.Len() != 0 {
		t.Errorf("Len = %d, want 0 (sem snapshot)", s.Len())
	}
	if s.Generation() != 0 {
		t.Errorf("Generation = %d, want 0", s.Generation())
	}
}

// TestStoreCheckpointThenDeltaScale: após Checkpoint, o próximo Save volta a
// ser só delta (base não é re-escrito no Save).
func TestStoreCheckpointThenDeltaScale(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	for i := 0; i < 100; i++ {
		s.Set(fmt.Sprintf("k-%03d", i), fmt.Sprintf("v-%d", i))
	}
	if _, err := s.Checkpoint(); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}
	// só 2 entradas mudam.
	s.Set("k-000", "changed")
	s.Set("k-001", "changed-too")
	if _, err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if got := s.Stats().DeltaUploaded; got != 2 {
		t.Errorf("DeltaUploaded = %d, want 2 (após checkpoint, só delta)", got)
	}
}

// stringsHasSuffix é um helper local (evita importar strings só p/ um teste).
func stringsHasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
