package ui

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/chat/provider"
)

// ─── Fixtures ─────────────────────────────────────────────────────────────────

// fakeProvider implementa chat.Provider para testes do picker.
type fakeProvider struct {
	name   string
	models []string
}

func (f *fakeProvider) Name() string      { return f.name }
func (f *fakeProvider) Models() []string  { return f.models }
func (f *fakeProvider) IsAvailable() bool { return true }
func (f *fakeProvider) Chat(_ context.Context, _ chat.ChatRequest) (<-chan chat.ChatEvent, error) {
	ch := make(chan chat.ChatEvent)
	close(ch)
	return ch, nil
}

// ─── Testes ───────────────────────────────────────────────────────────────────

func TestProviderPicker_InitialHidden(t *testing.T) {
	p := newProviderPicker(nil)
	if p.hasVisible() {
		t.Error("picker should start hidden")
	}
}

func TestProviderPicker_Toggle(t *testing.T) {
	p := newProviderPicker(nil)
	p.toggle()
	if !p.hasVisible() {
		t.Error("toggle should open the picker")
	}
	p.toggle()
	if p.hasVisible() {
		t.Error("second toggle should close the picker")
	}
}

func TestProviderPicker_NoRegistry_ShowsKnownProviders(t *testing.T) {
	p := newProviderPicker(nil)
	// Sem registry, o picker ainda mostra o catálogo de providers conhecidos.
	if n := len(p.list.Items()); n != len(knownProviders) {
		t.Errorf("picker should list %d known providers, got %d", len(knownProviders), n)
	}
}

func TestProviderPicker_WithRegistry_ListsKnownProviders(t *testing.T) {
	reg := provider.NewRegistry()
	reg.Register(&fakeProvider{name: "deepseek", models: []string{"deepseek-v4-flash"}})
	reg.Register(&fakeProvider{name: "ollama", models: []string{"llama3"}})

	p := newProviderPicker(reg)
	// O catálogo (4) prevalece; registry refina modelos/disponibilidade.
	if n := len(p.list.Items()); n != len(knownProviders) {
		t.Errorf("picker should list %d known providers, got %d", len(knownProviders), n)
	}
}

func TestProviderPicker_Escape_Closes(t *testing.T) {
	p := newProviderPicker(nil)
	p.toggle()
	if !p.hasVisible() {
		t.Fatal("setup: picker should be visible")
	}

	updated, _ := p.update(tea.KeyMsg{Type: tea.KeyEscape})
	if updated.hasVisible() {
		t.Error("esc should close the picker")
	}
}

func TestProviderPicker_Enter_ActivatesOllama(t *testing.T) {
	// Ollama não precisa de chave → Enter ativa direto.
	reg := provider.NewRegistry()
	reg.Register(&fakeProvider{name: "ollama", models: []string{"llama3"}})

	p := newProviderPicker(reg)
	p.toggle()

	// Navega até o ollama (último do catálogo: deepseek, openai, anthropic, ollama).
	for i := 0; i < len(knownProviders)-1; i++ {
		updated, _ := p.update(tea.KeyMsg{Type: tea.KeyDown})
		p = updated
	}

	updated, cmd := p.update(tea.KeyMsg{Type: tea.KeyEnter})

	if updated.hasVisible() {
		t.Error("enter should close the picker")
	}
	if cmd == nil {
		t.Error("enter should emit an activate command")
	}
	if updated.activeName() != "ollama" {
		t.Errorf("active = %q, want %q", updated.activeName(), "ollama")
	}
}

func TestProviderPicker_Enter_WithoutKey_RequestsKey(t *testing.T) {
	// OpenAI sem chave configurada → Enter deve pedir a API key.
	reg := provider.NewRegistry()
	p := newProviderPicker(reg)
	p.toggle()

	// Navega até o openai (índice 1 no catálogo: deepseek, openai, ...).
	updated, _ := p.update(tea.KeyMsg{Type: tea.KeyDown})
	p = updated

	updated, cmd := p.update(tea.KeyMsg{Type: tea.KeyEnter})

	if updated.hasVisible() {
		t.Error("picker should close when requesting a key")
	}
	if cmd == nil {
		t.Fatal("enter without key should emit a key request command")
	}
	// O comando deve produzir apiKeyRequestMsg.
	msg := cmd()
	req, ok := msg.(apiKeyRequestMsg)
	if !ok {
		t.Fatalf("expected apiKeyRequestMsg, got %T", msg)
	}
	if req.name != "openai" {
		t.Errorf("key request provider = %q, want %q", req.name, "openai")
	}
	if updated.activeName() == "openai" {
		t.Error("provider without key should NOT be activated")
	}
}

func TestModel_CtrlP_OpensPicker(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m = mustModel(t, updated)

	if !m.picker.hasVisible() {
		t.Error("Ctrl+P should open the model picker")
	}
}

func TestModel_PickerVisible_RedirectsKeys(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m = mustModel(t, updated)

	// Com o picker aberto, esc fecha (em vez de digitar no input).
	updated2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = mustModel(t, updated2)

	if m.picker.hasVisible() {
		t.Error("esc with picker open should close the picker")
	}
}

func TestProviderPicker_View_HiddenReturnsEmpty(t *testing.T) {
	p := newProviderPicker(nil)
	if v := p.View(); v != "" {
		t.Errorf("hidden picker view should be empty, got %q", v)
	}
}
