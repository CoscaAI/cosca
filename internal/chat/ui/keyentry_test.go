package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// ─── Testes do catálogo de providers ─────────────────────────────────────────

func TestKnownProviders_HasAll(t *testing.T) {
	names := make(map[string]bool)
	for _, kp := range knownProviders {
		names[kp.name] = true
	}
	for _, want := range []string{"deepseek", "openai", "anthropic", "ollama"} {
		if !names[want] {
			t.Errorf("known providers missing %q", want)
		}
	}
}

func TestKnownProviders_NeedsKey(t *testing.T) {
	cases := map[string]bool{
		"deepseek":  true,
		"openai":    true,
		"anthropic": true,
		"ollama":    false,
	}
	for name, want := range cases {
		if got := providerNeedsKey(name); got != want {
			t.Errorf("providerNeedsKey(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestKnownProviders_Unknown(t *testing.T) {
	if _, ok := knownProviderByName("groq"); ok {
		t.Error("groq should not be a known provider")
	}
}

// ─── Testes do fluxo de entrada de API key ────────────────────────────────────

func TestModel_KeyRequest_OpensKeyEntry(t *testing.T) {
	m := newTestModel()

	updated, _ := m.Update(apiKeyRequestMsg{name: "openai"})
	m = mustModel(t, updated)

	if !m.keyEntry {
		t.Error("apiKeyRequestMsg should open key entry mode")
	}
	if m.keyProvider != "openai" {
		t.Errorf("keyProvider = %q, want %q", m.keyProvider, "openai")
	}
}

func TestModel_KeyEntry_Enter_EmitsSubmit(t *testing.T) {
	m := newTestModel()

	updated, _ := m.Update(apiKeyRequestMsg{name: "openai"})
	m = mustModel(t, updated)

	m.keyInput.SetValue("sk-test-123")
	updated, cmd := m.handleKeyEntry(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)

	if m.keyEntry {
		t.Error("enter in key entry should close the mode")
	}
	if cmd == nil {
		t.Fatal("enter should emit a submit command")
	}
	msg := cmd()
	submit, ok := msg.(apiKeySubmitMsg)
	if !ok {
		t.Fatalf("expected apiKeySubmitMsg, got %T", msg)
	}
	if submit.name != "openai" || submit.key != "sk-test-123" {
		t.Errorf("submit = %+v", submit)
	}
}

func TestModel_KeyEntry_EmptyKey_NoSubmit(t *testing.T) {
	m := newTestModel()

	updated, _ := m.Update(apiKeyRequestMsg{name: "openai"})
	m = mustModel(t, updated)

	m.keyInput.SetValue("   ")
	updated, cmd := m.handleKeyEntry(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)

	if !m.keyEntry {
		t.Error("empty key should keep key entry open")
	}
	if cmd != nil {
		t.Error("empty key should not emit a submit command")
	}
}

func TestModel_KeyEntry_Escape_Cancels(t *testing.T) {
	m := newTestModel()

	updated, _ := m.Update(apiKeyRequestMsg{name: "openai"})
	m = mustModel(t, updated)

	updated, cmd := m.handleKeyEntry(tea.KeyMsg{Type: tea.KeyEscape})
	m = mustModel(t, updated)

	if m.keyEntry {
		t.Error("esc should cancel key entry")
	}
	if cmd != nil {
		t.Error("esc should not emit a command")
	}
}

func TestModel_KeyEntry_PasswordMasked(t *testing.T) {
	m := newTestModel()
	if m.keyInput.EchoMode != textinput.EchoPassword {
		t.Error("key input should use password echo mode (masked)")
	}
}

func TestModel_ApplyAPIKey_SavesAndConnects(t *testing.T) {
	m := newTestModel()
	// engine nil → usa o caminho de save; a chave não é persistida de verdade
	// (config global), mas o fluxo de mensagens deve funcionar.

	result := m.applyAPIKey("ollama", "not-a-real-key")
	if strings.Contains(result, "Provider desconhecido") {
		t.Errorf("ollama should be known, got: %s", result)
	}
}

// ─── Testes do fluxo via slash /model <nome> ──────────────────────────────────

func TestSlash_Model_NoKey_RequestsKey(t *testing.T) {
	m := newTestModel()
	m.input.SetValue("/model openai")

	updated, _ := sendEnter(m)
	m = mustModel(t, updated)

	if !m.keyEntry {
		t.Error("/model openai (sem chave) deve abrir o campo de API key")
	}
	if m.keyProvider != "openai" {
		t.Errorf("keyProvider = %q, want %q", m.keyProvider, "openai")
	}
}

func TestSlash_Model_Ollama_Activates(t *testing.T) {
	m := newTestModel()
	m.input.SetValue("/model ollama")

	updated, _ := sendEnter(m)
	m = mustModel(t, updated)

	if m.keyEntry {
		t.Error("ollama não precisa de chave — não deve abrir key entry")
	}
}
