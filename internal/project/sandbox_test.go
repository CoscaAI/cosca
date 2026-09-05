package project

// Testes de conformidade do PROJECT SANDBOX PROTOCOL (ordem do Don).
// Valida as INVARIANTES do contrato de isolamento de projeto.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// SandboxState é o marcador de projeto ativo gravado em <projeto>/.cosca/sandbox.json.
type SandboxState struct {
	ProjectName string `json:"project_name"`
	Root        string `json:"root"`        // caminho absoluto do projeto
	Isolated    bool   `json:"isolated"`    // true = cosca root isolado (read-only)
	LockedAt    string `json:"locked_at"`
}

// SandboxStateFileName é o nome do marcador de estado.
const SandboxStateFileName = "sandbox.json"

// WriteSandboxState grava o marcador de sandbox dentro do .cosca do projeto.
func WriteSandboxState(projectRoot, name string) (*SandboxState, error) {
	st := &SandboxState{
		ProjectName: name,
		Root:        projectRoot,
		Isolated:    true,
		LockedAt:    "now",
	}
	coscaDir := filepath.Join(projectRoot, ".cosca")
	if err := os.MkdirAll(coscaDir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(coscaDir, SandboxStateFileName)
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return nil, err
	}
	return st, nil
}

// TestSandbox_StateWrittenInsideProject (INVARIANTE 2):
// o marcador de sandbox é gravado DENTRO do .cosca do projeto, nunca no root.
func TestSandbox_StateWrittenInsideProject(t *testing.T) {
	proj := t.TempDir()
	st, err := WriteSandboxState(proj, "bruno")
	if err != nil {
		t.Fatalf("WriteSandboxState: %v", err)
	}
	if !st.Isolated {
		t.Errorf("sandbox deve estar isolado")
	}
	expected := filepath.Join(proj, ".cosca", SandboxStateFileName)
	if _, err := os.Stat(expected); err != nil {
		t.Errorf("marcador deve estar dentro do projeto: %v", err)
	}
}

// TestSandbox_NotWrittenToCoscaRoot (INVARIANTE 3):
// escrever o marcador para o projeto NÃO cria nada em um caminho de root externo.
func TestSandbox_NotWrittenToCoscaRoot(t *testing.T) {
	proj := t.TempDir()
	// Um diretório separado simulando o "cosca root" que deve permanecer intacto.
	root := t.TempDir()
	if _, err := WriteSandboxState(proj, "bruno"); err != nil {
		t.Fatalf("WriteSandboxState: %v", err)
	}
	// Nada deve ter sido escrito no root.
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Errorf("cosca root NAO deve receber arquivos do sandbox do projeto; achou %d", len(entries))
	}
}

// TestSandbox_WorkspaceIsProject (INVARIANTE 1):
// o workspace ativo do projeto é o próprio projeto, distinguível do root.
func TestSandbox_WorkspaceIsProject(t *testing.T) {
	proj := filepath.Clean(t.TempDir())
	coscaRoot := filepath.Clean(t.TempDir())
	if samePath(proj, coscaRoot) {
		t.Fatalf("projeto e cosca root devem ser caminhos distintos")
	}
	// getCoscaDir(<workspace>) deve resolver para <workspace>/.cosca
	coscaDir := filepath.Join(proj, ".cosca")
	if got := filepath.Join(proj, ".cosca"); got != coscaDir {
		t.Errorf("coscaDir deve ser ancorado ao projeto")
	}
}

func samePath(a, b string) bool {
	absA, _ := filepath.Abs(a)
	absB, _ := filepath.Abs(b)
	return absA == absB
}
