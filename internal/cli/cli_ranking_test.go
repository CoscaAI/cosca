//
// Tests for `cosca ranking explain` (internal/cli/ranking.go) — ranking
// multi-sinal explicável (score breakdown por sinal).
//
// Cobre:
//   - Registro de `ranking` sob a raiz e de `explain` sob `ranking`
//   - Com um knowledge.db real semeado: text output com Score total, breakdown
//     por sinal (BM25/Vetor/Grafo/Frescor/Popularidade) e Justificação
//   - --index N escolhe outro resultado (#2)
//   - --index fora do intervalo → erro
//   - --json emite o breakdown estruturado
//
// NOTE: os testes fazem chdir() e NÃO rodam em paralelo. Formatter injetado
// via newContextWithFormatter (padrão do CLI).
//

package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/knowledge"
)

// chdirRankingTemp muda para um diretório temporário e restaura o cwd no
// cleanup (mesmo padrão dos testes de CLI que usam knowledge.db real).
func chdirRankingTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Chdir registra o restore do cwd DEPOIS do RemoveAll do TempDir
	// (cleanups rodam em LIFO) — no Windows não dá para remover o
	// diretório que é o CWD do processo.
	t.Chdir(dir)
	globalFlags = GlobalFlags{}
	return dir
}

// runRankingExplainCommand executa o RunE com formatter injetado em buffer.
func runRankingExplainCommand(t *testing.T, cmd *cobra.Command, args []string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true) // noColor
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)
	err := cmd.RunE(cmd, args)
	return buf.String(), err
}

// seedRankingDB cria um knowledge.db real no diretório atual com documentos
// que mencionam "backup" e outros temas.
func seedRankingDB(t *testing.T) {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	ke, err := knowledge.New(knowledge.Config{
		DBPath:      filepath.Join(dir, "knowledge.db"),
		RootDir:     dir,
		AutoMigrate: true,
	})
	if err != nil {
		t.Fatalf("knowledge.New: %v", err)
	}
	if err := ke.Init(); err != nil {
		t.Fatalf("ke.Init: %v", err)
	}

	docs := []struct{ name, content string }{
		{"backup1.md", "# Backup Estratégico\n\nBackup de dados diário com retenção de 30 dias e verificação de integridade."},
		{"backup2.md", "# Backup Incremental\n\nBackup incremental para armazenamento externo com criptografia."},
		{"database.md", "# Banco de Dados\n\nSchema do banco de dados relacional e índices."},
	}
	for _, d := range docs {
		p := filepath.Join(dir, d.name)
		if err := os.WriteFile(p, []byte(d.content), 0o644); err != nil {
			t.Fatalf("write %s: %v", d.name, err)
		}
		if err := ke.IndexDocument(context.Background(), p); err != nil {
			t.Fatalf("index %s: %v", d.name, err)
		}
	}
	if err := ke.Close(); err != nil {
		t.Fatalf("ke.Close: %v", err)
	}
}

// =============================================================================
// Registro — `ranking` na raiz e `explain` sob `ranking`
// =============================================================================

func TestRootCommand_RegistersRanking(t *testing.T) {
	root := NewRootCommand()
	found := false
	for _, sub := range root.Commands() {
		if sub.Name() == "ranking" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("ranking subcommand not registered under cosca")
	}
}

func TestRankingCommand_HasExplainSubcommand(t *testing.T) {
	cmd := NewRankingCommand()
	if cmd == nil {
		t.Fatal("NewRankingCommand returned nil")
	}
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "explain" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("explain subcommand not registered under cosca ranking")
	}
}

func TestRankingExplainCommand_Properties(t *testing.T) {
	cmd := NewRankingExplainCommand()
	if cmd == nil {
		t.Fatal("NewRankingExplainCommand returned nil")
	}
	if cmd.Use != "explain <query>" {
		t.Errorf("Use = %q, want 'explain <query>'", cmd.Use)
	}
	if cmd.Short == "" || cmd.Long == "" {
		t.Error("Short/Long não podem ser vazios")
	}
	flag := cmd.Flags().Lookup("index")
	if flag == nil {
		t.Fatal("flag --index ausente")
	}
	if flag.DefValue != "0" {
		t.Errorf("--index default deveria ser 0, got %q", flag.DefValue)
	}
}

// =============================================================================
// Fluxo real — text output com breakdown + justificação
// =============================================================================

func TestRankingExplain_TextOutput(t *testing.T) {
	chdirRankingTemp(t)
	seedRankingDB(t)

	cmd := NewRankingExplainCommand()
	out, err := runRankingExplainCommand(t, cmd, []string{"backup"})
	if err != nil {
		t.Fatalf("ranking explain: %v\n%s", err, out)
	}

	for _, want := range []string{
		"RANKING EXPLICÁVEL",
		"Score total",
		"BM25",
		"Vetor",
		"Grafo",
		"Frescor",
		"Popularidade",
		"Justificação",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("saída deveria conter %q: %s", want, out)
		}
	}
	if !strings.Contains(out, "/100") {
		t.Errorf("saída deveria escalar os scores para /100: %s", out)
	}
	if !strings.Contains(out, "(dominante)") {
		t.Errorf("justificação deveria citar o sinal dominante: %s", out)
	}
}

func TestRankingExplain_IndexSelectsAnotherResult(t *testing.T) {
	chdirRankingTemp(t)
	seedRankingDB(t)

	cmd := NewRankingExplainCommand()
	if err := cmd.Flags().Set("index", "1"); err != nil {
		t.Fatalf("set --index: %v", err)
	}
	out, err := runRankingExplainCommand(t, cmd, []string{"backup"})
	if err != nil {
		t.Fatalf("ranking explain --index 1: %v\n%s", err, out)
	}
	if !strings.Contains(out, "#2") {
		t.Errorf("--index 1 deveria exibir o 2º resultado (#2): %s", out)
	}
}

func TestRankingExplain_IndexOutOfRange(t *testing.T) {
	chdirRankingTemp(t)
	seedRankingDB(t)

	cmd := NewRankingExplainCommand()
	if err := cmd.Flags().Set("index", "99"); err != nil {
		t.Fatalf("set --index: %v", err)
	}
	_, err := runRankingExplainCommand(t, cmd, []string{"backup"})
	if err == nil {
		t.Fatal("--index fora do intervalo deveria retornar erro")
	}
	if !strings.Contains(err.Error(), "fora do intervalo") {
		t.Errorf("erro deveria mencionar o intervalo: %v", err)
	}
}

// =============================================================================
// --json — breakdown estruturado
// =============================================================================

func TestRankingExplain_JSONOutput(t *testing.T) {
	chdirRankingTemp(t)
	seedRankingDB(t)

	globalFlags.JSON = true
	cmd := NewRankingExplainCommand()
	out, err := runRankingExplainCommand(t, cmd, []string{"backup"})
	if err != nil {
		t.Fatalf("ranking explain --json: %v\n%s", err, out)
	}

	for _, want := range []string{
		`"query": "backup"`,
		`"breakdown"`,
		`"justification"`,
		`"total"`,
		`"bm25"`,
		`"vector"`,
		`"graph"`,
		`"freshness"`,
		`"popularity"`,
		`"signals"`,
		`"item_id"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON deveria conter %s: %s", want, out)
		}
	}
}

func TestRankingExplain_NoResults(t *testing.T) {
	chdirRankingTemp(t)
	seedRankingDB(t)

	cmd := NewRankingExplainCommand()
	out, err := runRankingExplainCommand(t, cmd, []string{"frase-inexistente-zzz-123"})
	if err != nil {
		t.Fatalf("ranking explain sem resultados: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Nenhum resultado") {
		t.Errorf("deveria avisar que não há resultados: %s", out)
	}
}
