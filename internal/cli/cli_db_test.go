package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/dbhealth"
)

// setupDBCheckProject cria um projeto temporário com um .cosca e, opcionalmente,
// um knowledge.db real (via createKernelTestDB), e faz `cd` para ele. Devolve o
// diretório do projeto.
func setupDBCheckProject(t *testing.T, withKnowledge bool) string {
	t.Helper()
	origin, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// Usa os.MkdirTemp (não t.TempDir) para controlar a ordem do cleanup: como
	// os t.Cleanup rodam em LIFO, registrar RemoveAll ANTES de Chdir(origin)
	// garante que o cwd saia do diretório temporário ANTES da remoção — no
	// Windows não se pode deletar o diretório que é o cwd do processo.
	dir, err := os.MkdirTemp("", "cosca-dbcheck-")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) }) // roda por último (LIFO)
	t.Cleanup(func() { _ = os.Chdir(origin) })  // roda primeiro (LIFO)

	coscaDir := filepath.Join(dir, ".cosca")
	if err := os.MkdirAll(coscaDir, 0o755); err != nil {
		t.Fatalf("mkdir .cosca: %v", err)
	}
	if withKnowledge {
		createKernelTestDB(t, filepath.Join(coscaDir, "knowledge.db"))
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	return dir
}

// executeDBCheck executa `cosca db check <args...>` via NewRootCommand e
// devolve a saída (stdout) e o erro do Execute().
func executeDBCheck(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := NewRootCommand()
	root.PersistentPreRunE = nil // evita initConfig/telemetria; determinístico
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	formatter := NewOutputFormatter(buf, OutputFormatText, false, false, false)
	root.SetContext(newContextWithFormatter(context.Background(), formatter))
	root.SetArgs(append([]string{"db", "check"}, args...))
	err := root.Execute()
	return buf.String(), err
}

func TestDBCheck_ListText(t *testing.T) {
	setupDBCheckProject(t, true)

	out, err := executeDBCheck(t)
	if err != nil {
		t.Fatalf("db check returned error: %v", err)
	}
	for _, want := range []string{"DB Check", "knowledge", "ok"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q; output:\n%s", want, out)
		}
	}
}

func TestDBCheck_JSON_Output(t *testing.T) {
	setupDBCheckProject(t, true)

	out, err := executeDBCheck(t, "--json")
	if err != nil {
		t.Fatalf("db check --json returned error: %v", err)
	}
	var res dbhealth.Result
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(res.Databases) == 0 {
		t.Fatalf("esperava bancos listados no JSON")
	}
	found := false
	for _, d := range res.Databases {
		if d.RelPath == "knowledge.db" {
			found = true
			if !d.Found {
				t.Errorf("knowledge.db deveria estar found=true")
			}
			if d.DBSizeBytes <= 0 {
				t.Errorf("knowledge.db deveria ter db_size_bytes > 0, got %d", d.DBSizeBytes)
			}
		}
	}
	if !found {
		t.Errorf("JSON não incluiu knowledge.db")
	}
}

func TestDBCheck_Gate_Fail_ExitsNonZero(t *testing.T) {
	setupDBCheckProject(t, true)

	out, err := executeDBCheck(t, "--gate", "--limit-mb", "0.001")
	if err == nil {
		t.Fatalf("esperava erro (exit != 0) no gate fail, got nil\noutput:\n%s", out)
	}
	if code := ExitCode(err); code == 0 {
		t.Fatalf("esperava exit code != 0, got %d (err=%v)\noutput:\n%s", code, err, out)
	}
	if !strings.Contains(out, "BLOQUEIO") {
		t.Errorf("output deveria conter a marca de BLOQUEIO; output:\n%s", out)
	}
}

func TestDBCheck_Gate_Pass_ExitsZero(t *testing.T) {
	setupDBCheckProject(t, true)

	out, err := executeDBCheck(t, "--gate", "--limit-mb", "1000")
	if err != nil {
		t.Fatalf("esperava exit 0 (tudo abaixo de 1000 MB), got error: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "DB Check") {
		t.Errorf("output deveria conter o cabeçalho; output:\n%s", out)
	}
	if strings.Contains(out, "BLOQUEIO") {
		t.Errorf("output não deveria conter BLOQUEIO (gate aprovado); output:\n%s", out)
	}
}

func TestDBCheck_Gate_Warn_ExitsZero(t *testing.T) {
	setupDBCheckProject(t, true)

	// Teto default 100 MB (fail), mas alerta baixíssimo (1 KiB): o knowledge.db
	// real crusta o alerta mas NÃO o teto → warn (exit 0, não bloqueia).
	out, err := executeDBCheck(t, "--gate", "--warn-mb", "0.001")
	if err != nil {
		t.Fatalf("esperava exit 0 (apenas warn), got error: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "Atenção") {
		t.Errorf("output deveria conter o alerta; output:\n%s", out)
	}
	if strings.Contains(out, "BLOQUEIO") {
		t.Errorf("output não deveria conter BLOQUEIO (apenas warn); output:\n%s", out)
	}
}

func TestDBCheck_NoCosca_DoesNotBreak(t *testing.T) {
	// Projeto sem .cosca: nada quebra; todos os alvos reportados not_found.
	setupDBCheckProject(t, false)

	out, err := executeDBCheck(t)
	if err != nil {
		t.Fatalf("db check sem .cosca deveria passar, got: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "não encontrado") {
		t.Errorf("output deveria conter 'não encontrado'; output:\n%s", out)
	}
	if !strings.Contains(out, "DB Check") {
		t.Errorf("output deveria conter o cabeçalho; output:\n%s", out)
	}
}
