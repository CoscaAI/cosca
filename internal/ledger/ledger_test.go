package ledger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// openTemp abre um caderno num diretório temporário isolado.
func openTemp(t *testing.T, opts ...Option) *Ledger {
	t.Helper()
	l, err := Open(t.TempDir(), opts...)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return l
}

// P1 — Durabilidade: sobrevive a close/reopen.
func TestDurability(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := l.Put("familia", []byte("cosca"), 0); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := l.Put("don", []byte("chef"), 0); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	l2, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer l2.Close()

	for _, key := range []string{"familia", "don"} {
		v, _, ok := l2.Get(key)
		if !ok {
			t.Fatalf("chave %q sumiu após reopen", key)
		}
		if key == "familia" && string(v) != "cosca" {
			t.Fatalf("valor %q divergente após reopen", key)
		}
		if key == "don" && string(v) != "chef" {
			t.Fatalf("valor %q divergente após reopen", key)
		}
	}
}

// P2 — Consistência: CAS de versão impede escrita perdida.
func TestMVCCPreventsLostUpdate(t *testing.T) {
	l := openTemp(t)
	defer l.Close()

	v, err := l.Put("plano", []byte("rascunho"), 0)
	if err != nil || v != 1 {
		t.Fatalf("Put inicial: v=%d err=%v", v, err)
	}

	// Escrita com versão obsoleta deve falhar e não alterar o valor.
	if _, err := l.Put("plano", []byte("sobrescrita-ilegal"), 0); !errors.Is(err, ErrConflict) {
		t.Fatalf("esperava ErrConflict, veio %v", err)
	}
	if got, _, _ := l.Get("plano"); string(got) != "rascunho" {
		t.Fatalf("valor foi sobrescrito ilegalmente: %q", got)
	}

	// Escrita com a versão correta passa e avança a versão.
	v2, err := l.Put("plano", []byte("aprovado"), 1)
	if err != nil || v2 != 2 {
		t.Fatalf("Put CAS: v=%d err=%v", v2, err)
	}

	// Delete com versão errada também é barrado.
	if err := l.Delete("plano", 1); !errors.Is(err, ErrConflict) {
		t.Fatalf("esperava ErrConflict no delete, veio %v", err)
	}
	if _, vv, ok := l.Get("plano"); !ok || vv != 2 {
		t.Fatalf("estado corrompido após delete negado: ok=%v v=%d", ok, vv)
	}
}

// P3 — Concorrência: leitores concorrentes + escritor, sem race.
func TestConcurrentReads(t *testing.T) {
	l := openTemp(t)
	defer l.Close()

	// Preenche base.
	for i := 0; i < 50; i++ {
		if _, err := l.Put(fmt.Sprintf("k%02d", i), []byte("v0"), 0); err != nil {
			t.Fatalf("Put seed: %v", err)
		}
	}

	var wg sync.WaitGroup
	// Escritor serializado (CAS correto).
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			key := fmt.Sprintf("k%02d", i%50)
			_, ver, _ := l.Get(key)
			if _, err := l.Put(key, []byte(fmt.Sprintf("v%d", i)), ver); err != nil {
				// só conflito é aceitável (outro escritor não existe aqui)
				if !errors.Is(err, ErrConflict) {
					t.Errorf("Put concorrente: %v", err)
				}
				return
			}
		}
	}()

	// Leitores concorrentes.
	for r := 0; r < 8; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				l.Get(fmt.Sprintf("k%02d", i%50))
				_ = l.Search("v")
			}
		}()
	}
	wg.Wait()
}

// P4 — Latência: leitura O(1) por hash index (corretude + Lookup rápido).
func TestPointReadCorrectness(t *testing.T) {
	l := openTemp(t)
	defer l.Close()

	if _, _, ok := l.Get("inexistente"); ok {
		t.Fatal("chave inexistente reportada como presente")
	}

	v, err := l.Put("x", []byte("42"), 0)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	val, ver, ok := l.Get("x")
	if !ok || ver != v || string(val) != "42" {
		t.Fatalf("leitura pontual divergente: ok=%v ver=%d val=%q", ok, ver, val)
	}
}

// P5 — Memória: compactação automática limita o log; snapshot persiste o estado.
func TestAutoCompactionBoundsLog(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(dir, WithMaxLogBytes(4<<10)) // 4 KiB
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	for i := 0; i < 500; i++ {
		if _, err := l.Put(fmt.Sprintf("chave-%04d", i), []byte(strings.Repeat("x", 64)), 0); err != nil {
			t.Fatalf("Put %d: %v", i, err)
		}
	}

	st := l.Stats()
	// Após 500 escritas de ~64 bytes, sem compactação o log teria dezenas de
	// KiB; com o teto de 4 KiB a compactação deve ter mantido sob controle.
	if st.WALBytes > 8<<10 {
		t.Fatalf("log não foi compactado: %d bytes (esperava <= 8192)", st.WALBytes)
	}
	if _, err := os.Stat(filepath.Join(dir, "snapshot.json")); err != nil {
		t.Fatalf("snapshot não foi gravado: %v", err)
	}
	if st.CompactErr != nil {
		t.Fatalf("compactação automática falhou: %v", st.CompactErr)
	}

	// Estado completo continua legível.
	if val, _, ok := l.Get("chave-0499"); !ok || len(val) != 64 {
		t.Fatalf("estado após compactação corrompido: ok=%v len=%d", ok, len(val))
	}
	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen do snapshot + WAL residual.
	l2, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen pós-compactação: %v", err)
	}
	defer l2.Close()
	if st2 := l2.Stats(); st2.Keys != 500 {
		t.Fatalf("reopen: esperava 500 chaves, veio %d", st2.Keys)
	}
}

// P6 — Auditabilidade: delete é tombstone; seq é monotônico e nunca reusado.
func TestDeleteIsTombstoneAndSeqMonotonic(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if _, err := l.Put("segredo", []byte("xyz"), 0); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := l.Delete("segredo", 1); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, _, ok := l.Get("segredo"); ok {
		t.Fatal("chave deletada ainda visível")
	}
	seqAfterDelete := l.Stats().Seq
	if seqAfterDelete != 2 {
		t.Fatalf("seq deveria ser 2 (put+delete), veio %d", seqAfterDelete)
	}
	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen: o tombstone é reaplicado — a chave NÃO ressuscita.
	l2, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer l2.Close()
	if _, _, ok := l2.Get("segredo"); ok {
		t.Fatal("tombstone não foi reaplicado no replay — chave ressuscitou")
	}
	if l2.Stats().Seq != 2 {
		t.Fatalf("seq não preservado no replay: %d", l2.Stats().Seq)
	}
}

// P7 — Buscabilidade: índice invertido.
func TestSearch(t *testing.T) {
	l := openTemp(t)
	defer l.Close()

	docs := map[string]string{
		"artigo-go":     "golang runtime scheduler goroutines",
		"artigo-k8s":    "kubernetes scheduler controllers workqueue",
		"nota-pessoal":  "golang e kubernetes juntos na esteira",
	}
	for k, v := range docs {
		if _, err := l.Put(k, []byte(v), 0); err != nil {
			t.Fatalf("Put %s: %v", k, err)
		}
	}

	cases := []struct {
		query string
		want  []string
	}{
		{"golang", []string{"artigo-go", "nota-pessoal"}},
		{"scheduler", []string{"artigo-go", "artigo-k8s"}},
		{"golang kubernetes", []string{"nota-pessoal"}},
		{"inexistente", nil},
	}
	for _, c := range cases {
		got := l.Search(c.query)
		if len(got) != len(c.want) {
			t.Fatalf("Search(%q) = %v, esperava %v", c.query, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("Search(%q) = %v, esperava %v", c.query, got, c.want)
			}
		}
	}

	// Update reindexa: remover um token deve removê-lo da busca.
	if _, err := l.Put("nota-pessoal", []byte("somente esteira"), 1); err != nil {
		t.Fatalf("Put update: %v", err)
	}
	if got := l.Search("golang kubernetes"); len(got) != 0 {
		t.Fatalf("índice não foi reindexado no update: %v", got)
	}
}

// P8 — Integridade: adulteração no WAL é detectada.
func TestTamperEvidence(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	for i := 0; i < 10; i++ {
		if _, err := l.Put(fmt.Sprintf("k%02d", i), []byte(fmt.Sprintf("valor-%d", i)), 0); err != nil {
			t.Fatalf("Put: %v", err)
		}
	}

	if err := l.Verify(); err != nil {
		t.Fatalf("Verify limpo deveria passar: %v", err)
	}
	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Adultera um byte no meio do WAL (fora do length-prefix do 1º frame).
	walPath := filepath.Join(dir, "ledger.wal")
	data, err := os.ReadFile(walPath)
	if err != nil {
		t.Fatalf("ler WAL: %v", err)
	}
	mid := len(data) / 2
	data[mid] ^= 0xFF
	if err := os.WriteFile(walPath, data, 0o644); err != nil {
		t.Fatalf("adulterar WAL: %v", err)
	}

	if _, err := Open(dir); err == nil {
		t.Fatal("reopen de WAL adulterado deveria falhar na verificação de integridade")
	} else {
		t.Logf("reopen detectou adulteração: %v", err)
	}
}

// Compactação manual: continuidade da cadeia e replay pós-snapshot.
func TestCompactAndReopen(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	for i := 0; i < 20; i++ {
		if _, err := l.Put(fmt.Sprintf("k%02d", i), []byte(fmt.Sprintf("v%d", i)), 0); err != nil {
			t.Fatalf("Put: %v", err)
		}
	}
	if err := l.Compact(); err != nil {
		t.Fatalf("Compact: %v", err)
	}
	if st := l.Stats(); st.WALBytes != 0 || st.SnapshotSeq != 20 {
		t.Fatalf("pós-compactação: walBytes=%d snapSeq=%d", st.WALBytes, st.SnapshotSeq)
	}
	// Escreve depois da compactação (âncora vira a raiz do snapshot).
	if _, err := l.Put("k20", []byte("v20"), 0); err != nil {
		t.Fatalf("Put pós-compactação: %v", err)
	}
	if err := l.Verify(); err != nil {
		t.Fatalf("Verify pós-compactação: %v", err)
	}
	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	l2, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer l2.Close()
	if st := l2.Stats(); st.Keys != 21 || st.Seq != 21 {
		t.Fatalf("reopen: keys=%d seq=%d", st.Keys, st.Seq)
	}
	for i := 0; i <= 20; i++ {
		if _, _, ok := l2.Get(fmt.Sprintf("k%02d", i)); !ok {
			t.Fatalf("chave k%02d sumiu após compactação+reopen", i)
		}
	}
}

// Snapshot adulterado também é detectado.
func TestTamperSnapshot(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	for i := 0; i < 5; i++ {
		if _, err := l.Put(fmt.Sprintf("k%d", i), []byte("v"), 0); err != nil {
			t.Fatalf("Put: %v", err)
		}
	}
	if err := l.Compact(); err != nil {
		t.Fatalf("Compact: %v", err)
	}
	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	snapPath := filepath.Join(dir, "snapshot.json")
	data, err := os.ReadFile(snapPath)
	if err != nil {
		t.Fatalf("ler snapshot: %v", err)
	}
	// Troca o valor de uma chave no state sem recalcular a raiz.
	adulterado := strings.Replace(string(data), `"value":"dg=="`, `"value":"aGFja2Vk"`, 1) // "v" → "hacked"
	if adulterado == string(data) {
		t.Fatal("não consegui localizar o valor para adulterar")
	}
	if err := os.WriteFile(snapPath, []byte(adulterado), 0o644); err != nil {
		t.Fatalf("escrever snapshot: %v", err)
	}

	if _, err := Open(dir); err == nil {
		t.Fatal("snapshot adulterado deveria ser rejeitado")
	} else if !strings.Contains(err.Error(), "ADULTERADO") {
		t.Fatalf("erro esperado de adulteração, veio: %v", err)
	}
}

// P1 — Janela de crash entre snapshot e truncate do WAL: o snapshot já está
// gravado (mais novo) mas o WAL ainda contém os registros antigos. O replay
// precisa pular os registros cobertos pelo snapshot em vez de falhar — sem
// isto o ledger se recusa a abrir com "seq fora de ordem".
func TestCrashWindowBetweenSnapshotAndWALTruncate(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	for i := 0; i < 5; i++ {
		if _, err := l.Put(fmt.Sprintf("k%02d", i), []byte(fmt.Sprintf("v%d", i)), 0); err != nil {
			t.Fatalf("Put: %v", err)
		}
	}

	// Backup do WAL com os 5 registros (estado "pré-truncate").
	walPath := filepath.Join(dir, "ledger.wal")
	backup := filepath.Join(dir, "ledger.wal.backup")
	if data, err := os.ReadFile(walPath); err != nil {
		t.Fatalf("read wal: %v", err)
	} else if err := os.WriteFile(backup, data, 0o644); err != nil {
		t.Fatalf("backup wal: %v", err)
	}

	// Compactação normal: snapshot até seq 5 + WAL truncado.
	if err := l.Compact(); err != nil {
		t.Fatalf("Compact: %v", err)
	}

	// SIMULA A JANELA DE CRASH: restaura o WAL antigo (registros 1..5) sobre
	// o truncado — exatamente o estado de um crash entre o rename do snapshot
	// e o truncate do WAL.
	if err := os.WriteFile(walPath, mustRead(t, backup), 0o644); err != nil {
		t.Fatalf("restore wal: %v", err)
	}
	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// O ledger DEVE abrir (replay idempotente pula os cobertos).
	l2, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen após janela de crash: %v", err)
	}
	defer l2.Close()
	if st := l2.Stats(); st.Keys != 5 || st.Seq != 5 {
		t.Fatalf("reopen: keys=%d seq=%d (esperava 5/5)", st.Keys, st.Seq)
	}
	for i := 0; i < 5; i++ {
		if _, _, ok := l2.Get(fmt.Sprintf("k%02d", i)); !ok {
			t.Fatalf("chave k%02d sumiu após janela de crash", i)
		}
	}
	if err := l2.Verify(); err != nil {
		t.Fatalf("Verify após janela de crash: %v", err)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}
