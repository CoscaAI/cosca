package terminal

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/pipeline"
	tea "github.com/charmbracelet/bubbletea"
)

func TestInputStaysFocusedAfterSubmit(t *testing.T) {
	runner := pipeline.NewMockRunner()
	m := New(runner, nil, nil, nil, nil, ModelConfig{
		CurrentAgent: "test-agent",
		CurrentModel: "test-model",
	})

	var updated tea.Model
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)

	m.chatInput.SetValue("build a health endpoint")
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if !m.chatInput.Focused() {
		t.Fatalf("chat input lost focus after task submit")
	}
	if !m.busy {
		t.Fatalf("model should be busy after submit")
	}
	if !m.background {
		t.Fatalf("model should be in background mode after submit")
	}
	if m.operations == nil {
		t.Fatalf("operations panel not initialized")
	}
}

func TestOperationsPanelFeedOnEvents(t *testing.T) {
	runner := pipeline.NewMockRunner()
	m := New(runner, nil, nil, nil, nil, ModelConfig{CurrentAgent: "a"})

	var updated tea.Model
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)

	var out tea.Model
	out, _ = m.handlePipelineEvent(pipeline.RunEvent{Type: pipeline.EventBuildStart, Data: "build"})
	m = out.(Model)
	out, _ = m.handlePipelineEvent(pipeline.RunEvent{Type: pipeline.EventBuildEnd, Data: "ok"})
	m = out.(Model)

	if m.operations.events != 2 {
		t.Fatalf("operations events = %d, want 2", m.operations.events)
	}
	if m.hud.Progress <= 0 || m.hud.Progress > 1 {
		t.Fatalf("hud progress should be driven by real step counts, got %f", m.hud.Progress)
	}
}

func TestQueuePromptWhileBusy(t *testing.T) {
	runner := pipeline.NewMockRunner()
	m := New(runner, nil, nil, nil, nil, ModelConfig{CurrentAgent: "a"})

	var updated tea.Model
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)

	m.busy = true
	m.chatInput.SetValue("second task")
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if len(m.pendingPrompts) != 1 {
		t.Fatalf("pendingPrompts = %d, want 1 (queued while busy)", len(m.pendingPrompts))
	}
	if !m.chatInput.Focused() {
		t.Fatalf("chat input lost focus while queuing")
	}
}

func TestCtrlOTogglesOperationsPanel(t *testing.T) {
	runner := pipeline.NewMockRunner()
	m := New(runner, nil, nil, nil, nil, ModelConfig{})

	var updated tea.Model
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	m = updated.(Model)
	if m.activePanel != PanelOperations {
		t.Fatalf("activePanel = %d, want PanelOperations", m.activePanel)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	m = updated.(Model)
	if m.activePanel != PanelChat {
		t.Fatalf("activePanel = %d, want PanelChat after second toggle", m.activePanel)
	}
}
