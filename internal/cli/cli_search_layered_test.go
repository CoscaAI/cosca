//
// Tests for `cosca search layered` (internal/cli/search.go) — busca em camadas
// com custo progressivo e parada antecipada.
//
// Cobre:
//   - Registro de `search layered` sob `cosca search` (não remove `search`)
//   - Propriedades do comando + flag --allow-llm (default false)
//   - Com um knowledge.db real semeado: os defaults (MinFTSCount=0,
//     MinBM25Score=999) forçam a busca até L3 ("L3-vector",
//     "✓ parou sem IA", "raciocínio não habilitado")
//   - Consulta sem hits: sem --allow-llm para antes do LLM ("L3-vector",
//     "✓ parou sem IA"); com --allow-llm alcança L4 ("L4-llm",
//     "✗ precisou de raciocínio", "LLM chamado: true")
//   - --json expõe o LayeredResult estruturado (layer, stopped_early, stats)
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

// chdirLayeredTemp muda para um diretório temporário e restaura o cwd no
// cleanup (mesmo padrão do cli_evidence_test.go).
func chdirLayeredTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Chdir registra o restore do cwd DEPOIS do RemoveAll do TempDir
	// (cleanups rodam em LIFO) — no Windows não dá para remover o
	// diretório que é o CWD do processo.
	t.Chdir(dir)
	globalFlags = GlobalFlags{}
	return dir
}

// runSearchLayeredCommand executa o RunE com formatter injetado em buffer
// (padrão de injeção de formatter do CLI).
func runSearchLayeredCommand(t *testing.T, cmd *cobra.Command, args []string) (string, error) {
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

// seedLayeredDB cria knowledge.db real no diretório atual com 3 documentos
// que mencionam "backup" (FTS5 → hits suficientes). O seed cobre os dois
// caminhos de DB em uso pelo CLI: `cosca search layered` lê
// .cosca/knowledge.db (knowledgeDBPath) e `cosca search --ttl` / cache lê
// knowledge.db na raiz (newCLIKnowledgeEngine).
func seedLayeredDB(t *testing.T) {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for _, dbPath := range []string{knowledgeDBPath(dir), filepath.Join(dir, "knowledge.db")} {
		seedLayeredDBAt(t, dbPath)
	}
}

func seedLayeredDBAt(t *testing.T, dbPath string) {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	ke, err := knowledge.New(knowledge.Config{
		DBPath:      dbPath,
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
		{"backup1.md", "# Backup Strategy One\n\nBackup de dados diário com retenção de 30 dias."},
		{"backup2.md", "# Backup Strategy Two\n\nBackup incremental e verificação de integridade."},
		{"backup3.md", "# Backup Strategy Three\n\nBackup para armazenamento externo com criptografia."},
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
// Registro — `search layered` sob `cosca search`
// =============================================================================

func TestSearchCommand_HasLayeredSubcommand(t *testing.T) {
	cmd := NewSearchCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "layered" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("layered subcommand not registered under cosca search")
	}
}

func TestSearchCommand_ExistingRunStillIntact(t *testing.T) {
	// No-regression: `cosca search` continua com Use original e Args.
	cmd := NewSearchCommand()
	if !strings.HasPrefix(cmd.Use, "search") {
		t.Errorf("Use deveria começar com 'search', got %q", cmd.Use)
	}
	if cmd.Args == nil {
		t.Error("Args deveria continuar validando (MinimumNArgs(1))")
	}
}

func TestSearchLayeredCommand_Properties(t *testing.T) {
	cmd := NewSearchLayeredCommand()
	if cmd == nil {
		t.Fatal("NewSearchLayeredCommand returned nil")
	}
	if cmd.Use != "layered <query>" {
		t.Errorf("Use = %q, want 'layered <query>'", cmd.Use)
	}
	if cmd.Short == "" || cmd.Long == "" {
		t.Error("Short/Long não podem ser vazios")
	}
	flag := cmd.Flags().Lookup("allow-llm")
	if flag == nil {
		t.Fatal("flag --allow-llm ausente")
	}
	if flag.DefValue != "false" {
		t.Errorf("--allow-llm default deveria ser false, got %q", flag.DefValue)
	}
}

// =============================================================================
// Fluxo real — L1 para quando FTS5 é suficiente
// =============================================================================

func TestSearchLayered_DefaultsReachL3_NoAI(t *testing.T) {
	chdirLayeredTemp(t)
	seedLayeredDB(t)

	cmd := NewSearchLayeredCommand()
	out, err := runSearchLayeredCommand(t, cmd, []string{"backup"})
	if err != nil {
		t.Fatalf("search layered: %v\n%s", err, out)
	}

	// Com os defaults atuais (MinFTSCount=0, MinBM25Score=999) a escada nunca
	// para em L1/L2 — sempre alcança L3 (vetor) e para antes do LLM.
	if !strings.Contains(out, "L3-vector") {
		t.Errorf("deveria alcançar L3-vector: %q", out)
	}
	if !strings.Contains(out, "✓ parou sem IA") {
		t.Errorf("deveria marcar parada sem IA: %q", out)
	}
	if !strings.Contains(out, "raciocínio não habilitado") {
		t.Errorf("deveria citar o motivo da parada determinística: %q", out)
	}
	if strings.Contains(out, "precisou de raciocínio") {
		t.Errorf("L3 não deveria envolver raciocínio: %q", out)
	}
	if !strings.Contains(out, "Backup Strategy") {
		t.Errorf("deveria listar os melhores candidatos: %q", out)
	}
	if strings.Contains(out, "FTS5 hits: 0") {
		t.Errorf("os hits FTS5 do seed deveriam ser contabilizados: %q", out)
	}
}

// =============================================================================
// --allow-llm — gate de L4
// =============================================================================

func TestSearchLayered_NoAllowLLM_StopsBeforeLLM(t *testing.T) {
	chdirLayeredTemp(t)
	seedLayeredDB(t)

	cmd := NewSearchLayeredCommand()
	out, err := runSearchLayeredCommand(t, cmd, []string{"zzz-inexistente-frase-123"})
	if err != nil {
		t.Fatalf("search layered: %v\n%s", err, out)
	}

	if !strings.Contains(out, "L3-vector") {
		t.Errorf("sem --allow-llm deveria parar em L3-vector: %q", out)
	}
	if !strings.Contains(out, "✓ parou sem IA") {
		t.Errorf("deveria marcar parada sem IA (0 tokens): %q", out)
	}
	if !strings.Contains(out, "LLM chamado: false") {
		t.Errorf("LLM não deveria ser chamado: %q", out)
	}
	if strings.Contains(out, "L4-llm") {
		t.Errorf("sem --allow-llm nunca alcança L4: %q", out)
	}
}

func TestSearchLayered_AllowLLM_ReachesL4(t *testing.T) {
	chdirLayeredTemp(t)
	seedLayeredDB(t)

	cmd := NewSearchLayeredCommand()
	if err := cmd.Flags().Set("allow-llm", "true"); err != nil {
		t.Fatalf("set --allow-llm: %v", err)
	}
	out, err := runSearchLayeredCommand(t, cmd, []string{"zzz-inexistente-frase-123"})
	if err != nil {
		t.Fatalf("search layered: %v\n%s", err, out)
	}

	if !strings.Contains(out, "L4-llm") {
		t.Errorf("com --allow-llm deveria alcançar L4-llm: %q", out)
	}
	if !strings.Contains(out, "✗ precisou de raciocínio") {
		t.Errorf("deveria marcar 'precisou de raciocínio': %q", out)
	}
	if !strings.Contains(out, "LLM chamado: true") {
		t.Errorf("LLM chamado deveria ser true: %q", out)
	}
	if !strings.Contains(out, "raciocínio necessário") {
		t.Errorf("deveria citar o motivo L4: %q", out)
	}
}

// =============================================================================
// --json — LayeredResult estruturado
// =============================================================================

func TestSearchLayered_JSONOutput(t *testing.T) {
	chdirLayeredTemp(t)
	seedLayeredDB(t)

	globalFlags.JSON = true
	cmd := NewSearchLayeredCommand()
	out, err := runSearchLayeredCommand(t, cmd, []string{"backup"})
	if err != nil {
		t.Fatalf("search layered --json: %v\n%s", err, out)
	}

	for _, want := range []string{
		`"layer": "L3-vector"`,
		`"stopped_early": true`,
		`"reason": "camadas determinísticas insuficientes e raciocínio não habilitado (--allow-llm) — retornando melhores candidatos determinísticos"`,
		`"fts5_hits":`,
		`"llm_called": false`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON deveria conter %s: %s", want, out)
		}
	}
}

func TestSearchLayered_JSON_AllowLLM(t *testing.T) {
	chdirLayeredTemp(t)
	seedLayeredDB(t)

	globalFlags.JSON = true
	cmd := NewSearchLayeredCommand()
	if err := cmd.Flags().Set("allow-llm", "true"); err != nil {
		t.Fatalf("set --allow-llm: %v", err)
	}
	out, err := runSearchLayeredCommand(t, cmd, []string{"zzz-inexistente-frase-123"})
	if err != nil {
		t.Fatalf("search layered --json: %v\n%s", err, out)
	}

	for _, want := range []string{
		`"layer": "L4-llm"`,
		`"stopped_early": false`,
		`"llm_called": true`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON deveria conter %s: %s", want, out)
		}
	}
}
