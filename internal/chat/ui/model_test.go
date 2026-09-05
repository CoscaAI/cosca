package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/CoscaAI/cosca/internal/chat/executor"
	"github.com/CoscaAI/cosca/internal/engine"
	"github.com/CoscaAI/cosca/internal/kernel"
)

// ─── Test helpers ─────────────────────────────────────────────────────────────

// newTestModel cria um modelo TUI mínimo para testes (engine nil).
func newTestModel() Model {
	m := New(nil, kernel.Identity(), nil, true)
	m.ready = true
	return m
}

// sendKey simula o envio de uma tecla ao modelo.
func sendKey(m Model, key string) (Model, tea.Cmd) {
	updated, cmd := m.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune(key),
	})
	if cmd == nil {
		return updated.(Model), nil
	}
	return updated.(Model), cmd
}

// sendEnter envia a tecla Enter.
func sendEnter(m Model) (Model, tea.Cmd) {
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return updated.(Model), cmd
}

// mustModel faz a type assertion de tea.Model para ui.Model em testes.
func mustModel(t *testing.T, updated tea.Model) Model {
	t.Helper()
	m, ok := updated.(Model)
	if !ok {
		t.Fatalf("expected ui.Model, got %T", updated)
	}
	return m
}

// ─── Testes do modelo básico ─────────────────────────────────────────────────

func TestNew_InitialState(t *testing.T) {
	m := newTestModel()

	if !m.ready {
		t.Error("model should be ready after New")
	}
	if m.input.Placeholder == "" {
		t.Error("input should have a placeholder")
	}
	if m.streaming || m.busy {
		t.Error("new model should not be streaming or busy")
	}
	if m.kernelIdentity.Name == "" {
		t.Error("kernel identity should be loaded")
	}
}

func TestNew_KernelIdentityLoaded(t *testing.T) {
	m := newTestModel()
	if m.kernelIdentity.ID != "cosca-kernel" {
		t.Errorf("kernel ID = %q, want %q", m.kernelIdentity.ID, "cosca-kernel")
	}
	if len(kernel.Laws) != 6 {
		t.Errorf("kernel laws = %d, want 6", len(kernel.Laws))
	}
	if len(kernel.Constitution) != 9 {
		t.Errorf("constitution principles = %d, want 9", len(kernel.Constitution))
	}
}

func TestEnter_EmptyPrompt_NoMessage(t *testing.T) {
	m := newTestModel()

	// Enter sem texto não deve adicionar mensagem.
	updated, _ := sendEnter(m)
	if len(updated.messages) != 0 {
		t.Errorf("empty prompt should not add messages, got %d", len(updated.messages))
	}
}

func TestEnter_UserPrompt_AddsMessage(t *testing.T) {
	m := newTestModel()
	m.input.SetValue("olá kernel")

	updated, cmd := sendEnter(m)

	if len(updated.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(updated.messages))
	}
	if updated.messages[0].role != "user" {
		t.Errorf("message role = %q, want %q", updated.messages[0].role, "user")
	}
	if updated.messages[0].content != "olá kernel" {
		t.Errorf("message content = %q, want %q", updated.messages[0].content, "olá kernel")
	}
	if !updated.busy {
		t.Error("engine should be busy after sending a prompt")
	}
	if cmd == nil {
		t.Error("enter should return a command (runStream)")
	}
}

func TestEnter_HistoryTracked(t *testing.T) {
	m := newTestModel()

	m.input.SetValue("primeira")
	updated, _ := sendEnter(m)
	m = updated

	m.input.SetValue("segunda")
	updated, _ = sendEnter(m)
	m = updated

	if len(m.history) != 2 {
		t.Fatalf("history = %d entries, want 2", len(m.history))
	}
	if m.history[0] != "primeira" || m.history[1] != "segunda" {
		t.Errorf("history = %v", m.history)
	}
}

func TestCtrlC_WhenIdle_Quits(t *testing.T) {
	m := newTestModel()
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = mustModel(t, updated)

	if !m.quit {
		t.Error("Ctrl+C when idle should set quit")
	}
	if cmd == nil {
		t.Error("Ctrl+C when idle should return tea.Quit")
	}
}

func TestCtrlD_Quits(t *testing.T) {
	m := newTestModel()
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = mustModel(t, updated)

	if !m.quit {
		t.Error("Ctrl+D should set quit")
	}
	if cmd == nil {
		t.Error("Ctrl+D should return tea.Quit")
	}
}

// ─── Testes dos slash commands ────────────────────────────────────────────────

func TestSlash_Help(t *testing.T) {
	m := newTestModel()
	m.input.SetValue("/help")

	updated, _ := sendEnter(m)

	found := false
	for _, msg := range updated.messages {
		if msg.role == "slash" && strings.Contains(msg.content, "/kernel") {
			found = true
		}
	}
	if !found {
		t.Error("/help should render the help panel mentioning /kernel")
	}
}

func TestSlash_Kernel(t *testing.T) {
	m := newTestModel()
	m.input.SetValue("/kernel")

	updated, _ := sendEnter(m)

	found := false
	for _, msg := range updated.messages {
		if msg.role == "slash" {
			if strings.Contains(msg.content, "Cosca Kernel") &&
				strings.Contains(msg.content, "CONSTITUIÇÃO") &&
				strings.Contains(msg.content, "LEIS") {
				found = true
			}
		}
	}
	if !found {
		t.Error("/kernel should render identity + laws + constitution")
	}
}

func TestSlash_Status(t *testing.T) {
	m := newTestModel()
	m.input.SetValue("/status")

	updated, _ := sendEnter(m)

	found := false
	for _, msg := range updated.messages {
		if msg.role == "slash" && strings.Contains(msg.content, "Engine") {
			found = true
		}
	}
	if !found {
		t.Error("/status should render engine state")
	}
}

func TestSlash_Model(t *testing.T) {
	m := newTestModel()
	m.input.SetValue("/model")

	updated, _ := sendEnter(m)

	if !updated.picker.hasVisible() {
		t.Error("/model should open the model picker")
	}
}

func TestSlash_Model_WithName(t *testing.T) {
	// /model <nome> ativa um provider diretamente. Sem registry, reporta erro.
	m := newTestModel()
	m.input.SetValue("/model openai")

	updated, _ := sendEnter(m)

	found := false
	for _, msg := range updated.messages {
		if msg.role == "slash" && strings.Contains(msg.content, "✓") {
			found = true
		}
	}
	if found {
		t.Error("/model <nome> sem registry não deveria ativar provider")
	}
}

func TestSlash_Unknown(t *testing.T) {
	m := newTestModel()
	m.input.SetValue("/naoexiste")

	updated, _ := sendEnter(m)

	found := false
	for _, msg := range updated.messages {
		if msg.role == "error" && strings.Contains(msg.content, "Comando desconhecido") {
			found = true
		}
	}
	if !found {
		t.Error("unknown slash command should render an error message")
	}
}

func TestSlash_Clear_ClearsHistory(t *testing.T) {
	m := newTestModel()
	m.appendMessage("user", "mensagem anterior")

	m.input.SetValue("/clear")
	updated, _ := sendEnter(m)

	if len(updated.messages) != 0 {
		t.Errorf("/clear should clear history, got %d messages", len(updated.messages))
	}
}

func TestSlash_Exit_Quits(t *testing.T) {
	m := newTestModel()
	m.input.SetValue("/exit")

	updated, _ := sendEnter(m)

	if !updated.quit {
		t.Error("/exit should set quit")
	}
}

func TestSlash_Memory_NoArg(t *testing.T) {
	m := newTestModel()
	m.input.SetValue("/memory")

	updated, _ := sendEnter(m)

	found := false
	for _, msg := range updated.messages {
		if msg.role == "slash" && strings.Contains(msg.content, "/memory <consulta>") {
			found = true
		}
	}
	if !found {
		t.Error("/memory without arg should show usage")
	}
}

// ─── Testes de eventos do engine ──────────────────────────────────────────────

func TestEngineEvent_Content(t *testing.T) {
	m := newTestModel()
	m.busy = true
	m.streaming = true
	m.currentRole = "assistant"
	m.currentText.Reset()

	updated, cmd := m.handleEngineEvent(engineEventMsg{
		event: &engine.EngineEvent{
			Type:    engine.EngineEventContent,
			Content: "olá ",
		},
	})
	m = mustModel(t, updated)

	if m.currentText.String() != "olá " {
		t.Errorf("buffer = %q, want %q", m.currentText.String(), "olá ")
	}
	if cmd == nil {
		t.Error("content event should continue consuming (consumeNext)")
	}
}

func TestEngineEvent_Done_FlushesBuffer(t *testing.T) {
	m := newTestModel()
	m.busy = true
	m.streaming = true
	m.currentRole = "assistant"
	m.currentText.Reset()
	m.currentText.WriteString("resposta final")

	updated, _ := m.handleEngineEvent(engineEventMsg{
		event: &engine.EngineEvent{Type: engine.EngineEventDone},
	})
	m = mustModel(t, updated)

	if m.busy {
		t.Error("done should clear busy")
	}
	if m.streaming {
		t.Error("done should clear streaming")
	}
	if len(m.messages) == 0 {
		t.Fatal("done should flush the buffer into history")
	}
	last := m.messages[len(m.messages)-1]
	if last.role != "assistant" || last.content != "resposta final" {
		t.Errorf("flushed message = %+v", last)
	}
}

func TestEngineEvent_Error(t *testing.T) {
	m := newTestModel()
	m.busy = true
	m.currentRole = "assistant"

	updated, _ := m.handleEngineEvent(engineEventMsg{
		event: &engine.EngineEvent{
			Type:  engine.EngineEventError,
			Error: errTest,
		},
	})
	m = mustModel(t, updated)

	if m.busy {
		t.Error("error should clear busy")
	}
	found := false
	for _, msg := range m.messages {
		if msg.role == "error" {
			found = true
		}
	}
	if !found {
		t.Error("error event should render an error message")
	}
}

func TestEngineEvent_ToolStart(t *testing.T) {
	m := newTestModel()
	m.busy = true
	m.streaming = true
	m.currentRole = "assistant"
	m.currentText.WriteString("pensando...")

	updated, cmd := m.handleEngineEvent(engineEventMsg{
		event: &engine.EngineEvent{
			Type: engine.EngineEventToolStart,
			ToolCall: &executor.ToolCall{
				Name: "read",
			},
		},
	})
	m = mustModel(t, updated)

	if m.streaming == false {
		t.Error("tool start should keep streaming state")
	}
	if cmd == nil {
		t.Error("tool start should continue consuming")
	}
	if len(m.messages) == 0 || m.messages[len(m.messages)-1].role != "assistant" {
		t.Error("tool start should flush the pending assistant text")
	}
}

// ─── Fixtures ────────────────────────────────────────────────────────────────

var errTest = &testErr{}

type testErr struct{}

func (e *testErr) Error() string { return "test error" }
