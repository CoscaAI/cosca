package editors

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/editors/types"
)

func TestManagerNamesAndUnknown(t *testing.T) {
	t.Chdir(t.TempDir())
	m := NewManager(types.EditorConfig{ProjectDir: t.TempDir(), DryRun: true})

	names := m.Names()
	if len(names) == 0 {
		t.Fatal("Names vazio — adapters não registrados")
	}

	// Editor desconhecido → erro.
	if err := m.Validate("ghost-editor"); err == nil {
		t.Fatal("Validate(unknown) must error")
	}
	// Teardown desconhecido → erro.
	if err := m.Teardown("ghost-editor"); err == nil {
		t.Fatal("Teardown(unknown) must error")
	}
}

func TestManagerValidateWithoutConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	m := NewManager(types.EditorConfig{ProjectDir: t.TempDir(), DryRun: true})

	// Validate de um editor conhecido SEM config → erro claro (not configured),
	// nunca panic — para todos os adapters registrados.
	for _, name := range m.Names() {
		err := m.Validate(name)
		if err == nil {
			t.Logf("Validate(%s) retornou nil (config presente no ambiente)", name)
			continue
		}
		// Sem panic e com mensagem coerente é o contrato.
		if err.Error() == "" {
			t.Fatalf("Validate(%s) erro vazio", name)
		}
	}
}

func TestManagerSetupAllAndTeardownNoConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	m := NewManager(types.EditorConfig{ProjectDir: t.TempDir(), DryRun: true})

	// SetupAll sem config → resultados (erros ou ok), nunca panic.
	results := m.SetupAll()
	if len(results) == 0 {
		t.Fatal("SetupAll retornou vazio")
	}
	// Teardown de todos os editores sem config → nunca panic.
	for _, name := range m.Names() {
		_ = m.Teardown(name)
	}
}
