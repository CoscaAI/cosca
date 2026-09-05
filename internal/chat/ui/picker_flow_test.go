package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/CoscaAI/cosca/internal/chat/provider"
	"github.com/CoscaAI/cosca/internal/kernel"
)

// TestModel_FullPickerFlow simula o fluxo completo estilo opencode:
// Ctrl+P abre o picker → navega → Enter seleciona → troca o provider ativo.
func TestModel_FullPickerFlow(t *testing.T) {
	reg := provider.NewRegistry()
	reg.Register(&fakeProvider{name: "deepseek", models: []string{"deepseek-v4-flash"}})
	reg.Register(&fakeProvider{name: "openai", models: []string{"gpt-4o"}})
	reg.SetPrimary("deepseek")

	m := New(nil, kernel.Identity(), reg, true)
	m.ready = true

	// Ctrl+P abre o picker.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m = mustModel(t, updated)
	if !m.picker.hasVisible() {
		t.Fatal("Ctrl+P should open the picker")
	}

	// Navega para baixo (do deepseek para openai) e seleciona.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mustModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)

	// O picker fechou e o provider ativo mudou.
	if m.picker.hasVisible() {
		t.Error("picker should close after selection")
	}
	if m.picker.activeName() != "openai" {
		t.Errorf("active provider = %q, want %q", m.picker.activeName(), "openai")
	}
}
