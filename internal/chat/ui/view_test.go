package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/CoscaAI/cosca/internal/kernel"
)

// TestView_RendersKernelHeader verifica que a View renderiza a identidade do
// Kernel no cabeçalho (sem depender de pty real).
func TestView_RendersKernelHeader(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.ready = true

	// Simula o redimensionamento inicial que o bubbletea envia.
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = mustModel(t, updated)

	m.appendMessage("user", "teste")
	m.appendMessage("assistant", "resposta")

	out := m.View()

	clean := cleanRendered(out)
	if !strings.Contains(clean, "Cosca Kernel") {
		t.Errorf("View should render Kernel identity in header, got:\n%s", clean)
	}
	if !strings.Contains(clean, "teste") {
		t.Error("View should render user message")
	}
	if !strings.Contains(clean, "resposta") {
		t.Error("View should render assistant message")
	}
}

// TestView_RendersStreamingBuffer verifica que o texto em streaming aparece
// com o cursor de atividade.
func TestView_RendersStreamingBuffer(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.ready = true
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = mustModel(t, updated)

	m.streaming = true
	m.currentRole = "assistant"
	m.currentText.Reset()
	m.currentText.WriteString("gerando resposta")

	out := m.View()
	clean := cleanRendered(out)

	if !strings.Contains(clean, "gerando resposta") {
		t.Errorf("View should render streaming text, got:\n%s", clean)
	}
}

// TestView_RendersSlashOutput verifica que resultados de slash commands
// aparecem no corpo.
func TestView_RendersSlashOutput(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.ready = true
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = mustModel(t, updated)

	m.input.SetValue("/kernel")
	updated, _ = sendEnter(m)
	m = mustModel(t, updated)

	out := m.View()
	clean := cleanRendered(out)

	if !strings.Contains(clean, "CONSTITUIÇÃO") {
		t.Errorf("View should render /kernel constitution output, got:\n%s", clean)
	}
}

// TestView_HelpToggle verifica que o painel de ajuda renderiza quando ativado.
func TestView_HelpRenders(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 40
	m.ready = true
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = mustModel(t, updated)

	m.showHelp = true
	out := m.View()
	clean := cleanRendered(out)

	if !strings.Contains(clean, "COMANDOS") {
		t.Errorf("help panel should render, got:\n%s", clean)
	}
	if !strings.Contains(clean, "/kernel") {
		t.Error("help panel should list /kernel")
	}
}

// TestView_WelcomeRenders verifica que a tela de boas-vindas aparece quando o
// histórico está vazio, com a identidade do Kernel e dicas.
func TestView_WelcomeRenders(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 40
	m.ready = true
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = mustModel(t, updated)

	out := m.View()
	clean := cleanRendered(out)

	if !strings.Contains(clean, "Cosca Kernel") {
		t.Errorf("welcome should render Kernel identity, got:\n%s", clean)
	}
	if !strings.Contains(clean, "Digite /") {
		t.Error("welcome should show usage hints")
	}
	if !strings.Contains(clean, "Ctrl+P") {
		t.Error("welcome should mention Ctrl+P")
	}
}

// TestView_WelcomeHidesAfterMessage verifica que as boas-vindas dão lugar ao
// histórico assim que a primeira mensagem é enviada.
func TestView_WelcomeHidesAfterMessage(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 40
	m.ready = true
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = mustModel(t, updated)

	m.appendMessage("user", "primeira mensagem")
	out := m.View()
	clean := cleanRendered(out)

	if strings.Contains(clean, "Digite /") {
		t.Error("welcome should hide after a message is sent")
	}
	if !strings.Contains(clean, "primeira mensagem") {
		t.Error("history should render after welcome hides")
	}
}

// TestProgram_Smoke roda o bubbletea program com saída em buffer para
// garantir que a TUI completa inicializa e processa mensagens sem crash.
func TestProgram_Smoke(t *testing.T) {
	var out bytes.Buffer

	p := tea.NewProgram(
		New(nil, kernel.Identity(), nil, true),
		tea.WithOutput(&out),
		tea.WithInput(strings.NewReader("\n")),
	)

	go func() {
		time.Sleep(200 * time.Millisecond)
		p.Send(tea.WindowSizeMsg{Width: 100, Height: 30})
		p.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	}()

	if _, err := p.Run(); err != nil {
		t.Fatalf("TUI program failed: %v", err)
	}
}
