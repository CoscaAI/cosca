//
// Tests for the `cosca cofre` command tree (internal/cli/cofre.go).
//
// Cobre:
//   - Registro de `cofre` no root command + propriedades + subcomandos
//   - `cofre validate` (a fronteira de validação do Cofre), no padrão AAA:
//       • pacote válido (evidência + proveniência + confiança)          → ACCEPT
//       • resultado sem evidência/proveniência/confiança               → INCONCLUSIVE
//       • Confidence HIGH sem evidência                                → REJECT
//       • pacote vazio                                                 → REJECT
//     Entrada via stdin, arquivo JSON e JSON inline; saída texto e JSON (--json).
//   - `cofre gate` → espelho das regras do Gate (auditoria)
//   - `cofre health` → diagnóstico (só propriedades; evita dial de rede nos testes)
//
// NOTE: os comandos validate/gate operam no cwd real; usam SetOut + formatter
// injetado via newContextWithFormatter (padrão do CLI). validate lê de stdin
// quando não há argumento — os testes injetam o pacote com cmd.SetIn.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/oracle"
)

// cofreJSONs de pacotes para os cenários de validação.
const (
	cofrePkgAccept = `{"intent":"diagnosticar lentidão","result":"StepRunner executava build/test por request","source":"codebase","provenance":"commit d8f5c7c","evidence":"benchmark: 29s → 5.4s após desativar","context":"produção","confidence":"HIGH"}`

	cofrePkgInconclusive = `{"intent":"diagnosticar lentidão","result":"achei algo no pipeline"}`

	cofrePkgRejectHighNoEvidence = `{"intent":"diagnosticar lentidão","result":"StepRunner causa a lentidão","confidence":"HIGH"}`

	cofrePkgEmpty = `{}`
)

// =============================================================================
// Registro — `cofre` no root
// =============================================================================

func TestCofreCommand_RegisteredInRoot(t *testing.T) {
	cmd := NewRootCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "cofre" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("cofre subcommand not registered in root command")
	}
}

func TestCofreCommand_Properties(t *testing.T) {
	cmd := NewCofreCommand()
	if cmd == nil {
		t.Fatal("NewCofreCommand returned nil")
	}
	if cmd.Use != "cofre" {
		t.Errorf("expected Use='cofre', got %q", cmd.Use)
	}
	if cmd.Short == "" || cmd.Long == "" {
		t.Error("expected non-empty Short/Long description")
	}
	if len(cmd.Example) == 0 {
		t.Error("expected non-empty Example")
	}
	expected := []string{"validate", "gate", "health"}
	if len(cmd.Commands()) != len(expected) {
		t.Errorf("expected %d subcommands, got %d (%v)", len(expected), len(cmd.Commands()), namesOf(cmd.Commands()))
	}
	for _, name := range expected {
		found := false
		for _, sub := range cmd.Commands() {
			if sub.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing cofre subcommand: %s", name)
		}
	}
}

func namesOf(cmds []*cobra.Command) []string {
	names := make([]string, 0, len(cmds))
	for _, c := range cmds {
		names = append(names, c.Name())
	}
	return names
}

// =============================================================================
// validate — entrada via stdin (cenário principal)
// =============================================================================

func TestCofreValidate_Accept(t *testing.T) {
	cmd := NewCofreValidateCommand()
	out, err := runStdinCofre(t, cmd, nil, cofrePkgAccept)

	if err != nil {
		t.Fatalf("ACCEPT deveria sair com exit 0, got error: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, string(oracle.Accept)) {
		t.Errorf("esperado DECISÃO ACCEPT no output, got:\n%s", out)
	}
	if !strings.Contains(out, "Motivo") {
		t.Errorf("esperado o Motivo (reason) no output, got:\n%s", out)
	}
}

func TestCofreValidate_Inconclusive(t *testing.T) {
	cmd := NewCofreValidateCommand()
	out, err := runStdinCofre(t, cmd, nil, cofrePkgInconclusive)

	if got := ExitCode(err); got != 2 {
		t.Errorf("INCONCLUSIVE deveria sair com exit 2, got %d (err=%v)", got, err)
	}
	if !strings.Contains(out, string(oracle.Inconclusive)) {
		t.Errorf("esperado DECISÃO INCONCLUSIVE no output, got:\n%s", out)
	}
	// "ainda não sabemos": o oráculo faz perguntas, nunca inventa conclusão.
	for _, want := range []string{"Perguntas do Oráculo", "De onde veio esse resultado?"} {
		if !strings.Contains(out, want) {
			t.Errorf("INCONCLUSIVE deveria gerar perguntas (§19), faltou %q no output:\n%s", want, out)
		}
	}
}

func TestCofreValidate_RejectHighWithoutEvidence(t *testing.T) {
	cmd := NewCofreValidateCommand()
	out, err := runStdinCofre(t, cmd, nil, cofrePkgRejectHighNoEvidence)

	if got := ExitCode(err); got != 1 {
		t.Errorf("REJECT deveria sair com exit 1, got %d (err=%v)", got, err)
	}
	if !strings.Contains(out, string(oracle.Reject)) {
		t.Errorf("esperado DECISÃO REJECT no output, got:\n%s", out)
	}
	if !strings.Contains(out, "promoção ilegítima") {
		t.Errorf("REJECT HIGH sem evidência deveria citar promoção ilegítima, got:\n%s", out)
	}
}

func TestCofreValidate_RejectEmptyPackage(t *testing.T) {
	cmd := NewCofreValidateCommand()
	out, err := runStdinCofre(t, cmd, nil, cofrePkgEmpty)

	if got := ExitCode(err); got != 1 {
		t.Errorf("REJECT deveria sair com exit 1, got %d (err=%v)", got, err)
	}
	if !strings.Contains(out, string(oracle.Reject)) {
		t.Errorf("esperado DECISÃO REJECT para pacote vazio, got:\n%s", out)
	}
}

// =============================================================================
// validate — entrada via arquivo e JSON inline
// =============================================================================

func TestCofreValidate_ReadsFromFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "pacote.json")
	writeCofreFile(t, p, cofrePkgAccept)

	cmd := NewCofreValidateCommand()
	out, err := runStdinCofre(t, cmd, []string{p}, "")

	if err != nil {
		t.Fatalf("ACCEPT a partir de arquivo deveria sair com exit 0, got: %v\n%s", err, out)
	}
	if !strings.Contains(out, string(oracle.Accept)) {
		t.Errorf("esperado DECISÃO ACCEPT lendo arquivo, got:\n%s", out)
	}
}

func TestCofreValidate_ReadsInlineJSON(t *testing.T) {
	cmd := NewCofreValidateCommand()
	// JSON inline passado como argumento (sem aspas no mundo real; aqui o
	// próprio harness injeta como um único arg).
	out, err := runStdinCofre(t, cmd, []string{cofrePkgAccept}, "")

	if err != nil {
		t.Fatalf("ACCEPT a partir de JSON inline deveria sair com exit 0, got: %v\n%s", err, out)
	}
	if !strings.Contains(out, string(oracle.Accept)) {
		t.Errorf("esperado DECISÃO ACCEPT via JSON inline, got:\n%s", out)
	}
}

func TestCofreValidate_NoInput_Errors(t *testing.T) {
	cmd := NewCofreValidateCommand()
	_, err := runStdinCofre(t, cmd, nil, "")

	if err == nil {
		t.Fatal("sem entrada (stdin vazio) deveria falhar com erro claro")
	}
	if !strings.Contains(err.Error(), "nenhum pacote semântico fornecido") {
		t.Errorf("erro = %q, esperado mensagem clara de entrada ausente", err.Error())
	}
}

// =============================================================================
// validate — saída JSON (--json)
// =============================================================================

func TestCofreValidate_JSONOutput(t *testing.T) {
	oldJSON := globalFlags.JSON
	globalFlags.JSON = true
	defer func() { globalFlags.JSON = oldJSON }()

	cmd := NewCofreValidateCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetIn(strings.NewReader(cofrePkgInconclusive))

	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	_ = cmd.RunE(cmd, nil) // exit code 2 (INCONCLUSIVE) — ignorado; output é JSON

	var res cofreValidateResult
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("saída não é JSON válido: %v\n%s", err, buf.String())
	}
	if res.Verdict.Decision != oracle.Inconclusive {
		t.Errorf("verdict decision = %q, want INCONCLUSIVE", res.Verdict.Decision)
	}
	if len(res.Verdict.Questions) == 0 {
		t.Error("verdict JSON deveria incluir Questions")
	}
}

// =============================================================================
// gate — espelho das regras do Gate
// =============================================================================

func TestCofreGate_Properties(t *testing.T) {
	cmd := NewCofreGateCommand()
	if cmd == nil {
		t.Fatal("NewCofreGateCommand returned nil")
	}
	if cmd.Use != "gate" {
		t.Errorf("expected Use='gate', got %q", cmd.Use)
	}
	if cmd.Short == "" || cmd.Long == "" {
		t.Error("expected non-empty Short/Long")
	}
}

func TestCofreGate_TextOutput(t *testing.T) {
	cmd := NewCofreGateCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	cmd.SetContext(newContextWithFormatter(context.Background(), f))

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("gate RunE: %v", err)
	}
	for _, want := range []string{"Regras do Gate", "Pacote vazio", "REJECT", "INCONCLUSIVE"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("gate output missing %q:\n%s", want, buf.String())
		}
	}
}

// =============================================================================
// health — diagnóstico (propriedades apenas; evita o probe de rede)
// =============================================================================

func TestCofreHealth_Properties(t *testing.T) {
	cmd := NewCofreHealthCommand()
	if cmd == nil {
		t.Fatal("NewCofreHealthCommand returned nil")
	}
	if cmd.Use != "health" {
		t.Errorf("expected Use='health', got %q", cmd.Use)
	}
	if cmd.Short == "" || cmd.Long == "" {
		t.Error("expected non-empty Short/Long")
	}
	if cmd.Args(cmd, nil) != nil {
		t.Error("health não deveria aceitar argumentos (NoArgs)")
	}
}

// =============================================================================
// helpers
// =============================================================================

// runStdinCofre executa o RunE do comando com o stdin informado e retorna a
// saída em texto e o erro (exit code) do comando.
func runStdinCofre(t *testing.T, cmd *cobra.Command, args []string, stdin string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetIn(strings.NewReader(stdin))
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true) // noColor
	cmd.SetContext(newContextWithFormatter(context.Background(), f))
	err := cmd.RunE(cmd, args)
	return buf.String(), err
}

func writeCofreFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}
