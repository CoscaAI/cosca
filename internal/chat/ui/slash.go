package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/CoscaAI/cosca/internal/engine"
	"github.com/CoscaAI/cosca/internal/kernel"
	"github.com/CoscaAI/cosca/internal/vision"
)

// ─── Slash Commands ───────────────────────────────────────────────────────────
// Comandos de alto nível para interagir com o Kernel sem sair da TUI.
//
// `/kernel`   — identidade, leis, constituição e epistemologia do Kernel
// `/status`   — estado do engine, sandbox, sessão
// `/agents`   — lista os agentes registrados (capos da família)
// `/skills`   — lista as skills disponíveis
// `/memory`   — busca na memória semântica (ex: /memory segurança)
// `/model`    — mostra o modelo ativo
// `/help`     — painel de ajuda
// `/clear`    — limpa o histórico visual
// `/exit`     — encerra o chat

// renderSlashResult processa um comando slash e devolve o texto a exibir.
// Recebe o modelo por valor para leitura de estado (histórico, streaming).
func renderSlashResult(m Model, prompt string) string {
	parts := strings.Fields(prompt)
	cmd := strings.ToLower(parts[0])
	arg := ""
	if len(parts) > 1 {
		arg = strings.Join(parts[1:], " ")
	}

	switch cmd {
	case "/kernel":
		return renderKernelIdentity(m.kernelIdentity)

	case "/status":
		return renderStatus(m)

	case "/agents":
		return renderAgents(m.engine)

	case "/skills":
		return renderSkills(m.engine)

	case "/workflows":
		return renderWorkflows(m.engine)

	case "/memory":
		if arg == "" {
			return slashBubble.Render("Uso: /memory <consulta>\nEx: /memory segurança")
		}
		return renderMemoryQuery(m, arg)

	case "/ver":
		if arg == "" {
			return slashBubble.Render("Uso: /ver <caminho-da-imagem>\nEx: /ver ~/fotos/painel-inversor.jpg")
		}
		return renderVision(arg)

	case "/model":
		// Sem argumento: abre o seletor de modelo (Ctrl+P também abre).
		if arg == "" {
			return "\x00model"
		}
		// Com argumento: provider conhecido?
		if kp, ok := knownProviderByName(arg); ok {
			// Precisa de chave e ainda não tem → pedir a chave.
			if kp.needsKey && !m.picker.connected[arg] {
				return "\x00modelkey:" + arg
			}
			return "\x00model:" + arg
		}
		return errorBubble.Render("Provider não encontrado: " + arg + "\nUse Ctrl+P ou /model para ver os disponíveis.")

	case "/help":
		return renderHelp()

	case "/clear":
		// O histórico é limpo no Update (não aqui) — ver handleSlashClear.
		return "\x00clear"

	case "/exit":
		return "\x00exit"

	default:
		return "\x00error" + errorBubble.Render("Comando desconhecido: "+cmd+"\nDigite /help para a lista de comandos.")
	}
}

// ─── /kernel — identidade do consigliere ──────────────────────────────────────

func renderKernelIdentity(p kernel.Persona) string {
	var b strings.Builder
	b.WriteString(headerSubStyle.Render("☯ KERNEL — " + p.Name))
	b.WriteString("\n")
	b.WriteString(slashBubble.Render(fmt.Sprintf("Papel:    %s", p.Role)))
	b.WriteString("\n")
	b.WriteString(slashBubble.Render(fmt.Sprintf("Modelo:   %s", p.Model)))
	b.WriteString("\n")
	b.WriteString(slashBubble.Render(fmt.Sprintf("Versão:   %s", p.Version)))
	b.WriteString("\n")
	b.WriteString(slashBubble.Render(fmt.Sprintf("Expertise: %s", strings.Join(p.Expertise, ", "))))
	b.WriteString("\n\n")

	// Leis do Kernel.
	b.WriteString(kernelBadgeStyle.Render("LEIS (6)"))
	b.WriteString("\n")
	for _, law := range kernel.Laws {
		b.WriteString(slashBubble.Render(fmt.Sprintf("  L%d · %s: %s", law.Number, law.Title, law.Rule)))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Constituição.
	b.WriteString(kernelBadgeStyle.Render("CONSTITUIÇÃO (8 princípios)"))
	b.WriteString("\n")
	for _, p := range kernel.Constitution {
		b.WriteString(slashBubble.Render(fmt.Sprintf("  P%d · %s (guardião: %s)", p.Number, p.Title, p.Guardian)))
		b.WriteString("\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// ─── /status — estado do sistema ──────────────────────────────────────────────

func renderStatus(m Model) string {
	var b strings.Builder
	b.WriteString(headerSubStyle.Render("STATUS"))
	b.WriteString("\n")
	b.WriteString(slashBubble.Render(fmt.Sprintf("Engine:   %s", statusEngine(m.engine))))
	b.WriteString("\n")
	b.WriteString(slashBubble.Render(fmt.Sprintf("Sessão:   %s", statusSession(m))))
	b.WriteString("\n")
	b.WriteString(slashBubble.Render(fmt.Sprintf("Mensagens no histórico: %d", len(m.messages))))
	b.WriteString("\n")
	b.WriteString(slashBubble.Render(fmt.Sprintf("Streaming: %v · Busy: %v", m.streaming, m.busy)))
	return b.String()
}

func statusEngine(e *engine.AgentEngine) string {
	if e == nil {
		return "indisponível"
	}
	return "ativo (AgentEngine)"
}

func statusSession(m Model) string {
	if len(m.history) == 0 {
		return "nova (sem entradas)"
	}
	return fmt.Sprintf("%d entradas", len(m.history))
}

// ─── /agents — capos da família ───────────────────────────────────────────────

func renderAgents(e *engine.AgentEngine) string {
	if e == nil {
		return errorBubble.Render("Engine indisponível.")
	}
	// O AgentEngine não expõe a lista de agentes diretamente; reportamos
	// a contagem de mensagens e o estado — a listagem completa fica via
	// `cosca agent list` (o binário único). Aqui mostramos o essencial.
	return slashBubble.Render(
		"Agentes: a lista completa está em `cosca agent list`.\n" +
			"Esta sessão usa roteamento automático (Router → capo da família).")
}

// ─── /skills — arsenal disponível ─────────────────────────────────────────────

func renderSkills(e *engine.AgentEngine) string {
	if e == nil {
		return errorBubble.Render("Engine indisponível.")
	}
	return slashBubble.Render(
		"Skills: o arsenal completo está em `cosca skill list`.\n" +
			"O Kernel carrega as skills do projeto (.cosca) e as injeta no contexto.")
}

// ─── /workflows — workflows ──────────────────────────────────────────────────

func renderWorkflows(e *engine.AgentEngine) string {
	if e == nil {
		return errorBubble.Render("Engine indisponível.")
	}
	return slashBubble.Render(
		"Workflows: os workflows estão configurados em .cosca/workflows/\n" +
			"Use `/workflows` no terminal ou `cosca workflows list` para detalhes.")
}

// ─── /memory — busca na memória semântica ─────────────────────────────────────

func renderMemoryQuery(m Model, query string) string {
	// A busca semântica completa vive no `cosca` (knowledge search) e no
	// AgentEngine via retriever de memória. Na TUI, reportamos o pedido e
	// apontamos o caminho — a memória real é consultada no contexto do engine.
	return slashBubble.Render(
		fmt.Sprintf("Memória: consulta '%s' registrada.\n", query) +
			"Busca semântica completa: `cosca knowledge search \"" + query + "\"`\n" +
			"ou `cosca exec \"busque na memória: " + query + "\"`")
}

// ─── /help — painel de ajuda ──────────────────────────────────────────────────

func renderHelp() string {
	var b strings.Builder
	b.WriteString(helpBubble.Render("COMANDOS"))
	b.WriteString("\n")
	for _, line := range [][2]string{
		{"/kernel", "identidade, leis, constituição do Kernel"},
		{"/status", "estado do engine, sessão e streaming"},
		{"/agents", "capos da família (roteamento automático)"},
		{"/skills", "arsenal de skills disponíveis"},
		{"/workflows", "workflows disponíveis"},
		{"/memory <q>", "consulta de memória semântica"},
		{"/ver <img>", "enxerga uma imagem (OCR local)"},
		{"/model", "seletor de modelo (Ctrl+P também abre)"},
		{"/model <nome>", "ativa um provider ou pede a chave dele"},
		{"/clear", "limpa o histórico visual"},
		{"/help", "este painel"},
		{"/exit", "encerra o chat"},
	} {
		b.WriteString(helpBubble.Render(fmt.Sprintf("  %-14s %s", line[0], line[1])))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(helpBubble.Render("TECLAS"))
	b.WriteString("\n")
	b.WriteString(helpBubble.Render("  ↑/↓            histórico de entradas"))
	b.WriteString("\n")
	b.WriteString(helpBubble.Render("  Ctrl+P          seletor de modelo (estilo opencode)"))
	b.WriteString("\n")
	b.WriteString(helpBubble.Render("  Ctrl+C         cancela a resposta em andamento"))
	b.WriteString("\n")
	b.WriteString(helpBubble.Render("  Ctrl+D         encerra o chat"))
	return b.String()
}

// ─── Ações especiais (/clear e /exit) ─────────────────────────────────────────

// handleSlashAction executa as ações que exigem mutação de estado fora do
// render (limpar histórico, sair, trocar/abrir seletor de modelo).
// Retorna (model, quit).
func (m Model) handleSlashAction(result string) (Model, bool) {
	switch {
	case result == "\x00clear":
		m.messages = nil
		return m, false
	case result == "\x00exit":
		return m, true
	case result == "\x00model":
		if m.picker.hasVisible() {
			m.picker.visible = false
		} else {
			m.picker.open(m.width, m.height)
		}
		return m, false
	case strings.HasPrefix(result, "\x00modelkey:"):
		// Provider sem chave → abre o campo de API key.
		name := strings.TrimPrefix(result, "\x00modelkey:")
		m.keyEntry = true
		m.keyProvider = name
		m.keyInput.SetValue("")
		m.keyInput.Focus()
		m.keyInput.Placeholder = "Cole a API key do " + name + "…"
		return m, false
	case strings.HasPrefix(result, "\x00model:"):
		name := strings.TrimPrefix(result, "\x00model:")
		msg := m.applyProviderChange(m.picker.registry, name)
		m.appendMessage("slash", msg)
		m.picker.active = name
		m.kernelIdentity.Model = name
		return m, false
	default:
		return m, false
	}
}

// isSlashAction reporta se o resultado de um slash command é uma ação de estado.
func isSlashAction(result string) bool {
	return result == "\x00clear" || result == "\x00exit" ||
		result == "\x00model" || strings.HasPrefix(result, "\x00model:") ||
		strings.HasPrefix(result, "\x00modelkey:")
}

// ─── /ver — visão do Kernel (OCR local) ───────────────────────────────────
// renderVision lê uma imagem com o tesseract local e devolve o que o Kernel
// "enxerga" — o texto extraído da imagem. 100% local, sem API externa.
func renderVision(arg string) string {
	path := expandHome(arg)

	res, err := vision.Describe(path, vision.Options{})
	if err != nil {
		return errorBubble.Render("Não consegui enxergar: " + err.Error() +
			"\nVerifique o caminho e que o tesseract esteja instalado.")
	}

	if res.IsEmpty() {
		return slashBubble.Render("👁 " + res.Summary() +
			"\nA imagem existe mas não encontrei texto legível nela.")
	}

	var b strings.Builder
	b.WriteString(slashBubble.Render("👁 " + res.Summary()))
	b.WriteString("\n\n")
	b.WriteString(slashBubble.Render("Conteúdo que enxerguei:\n" + res.Text))
	return b.String()
}

// expandHome resolves "~" to the user's home directory.
func expandHome(p string) string {
	// Handle both ~/ (Unix) and ~\ (Windows) for home directory expansion.
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~\\") {
		if home, err := os.UserHomeDir(); err == nil {
			if p == "~" {
				return home
			}
			return filepath.Join(home, p[2:])
		}
	}
	return p
}
