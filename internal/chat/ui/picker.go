package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/CoscaAI/cosca/internal/chat/config"
	"github.com/CoscaAI/cosca/internal/chat/provider"
)

// ─── Model Picker (estilo opencode) ───────────────────────────────────────────
// Permite escolher/ativar um provider de IA. Se o provider escolhido ainda
// não tem API key, a TUI abre um campo de entrada para o Don digitar a chave,
// salva criptografada e conecta.
//
// Atalho: Ctrl+P ou comando slash /model.

// providerOption é um item da lista de providers do seletor.
type providerOption struct {
	name     string
	model    string
	baseURL  string
	active   bool
	needsKey bool
	hasKey   bool
	builtin  bool // provider nativo do Cosca (catálogo)
}

// Title implementa list.Item.
func (o providerOption) Title() string {
	mark := " "
	if o.active {
		mark = "●"
	}
	if o.model != "" {
		return fmt.Sprintf("%s %s (%s)", mark, o.name, o.model)
	}
	return fmt.Sprintf("%s %s", mark, o.name)
}

// Description implementa list.Item.
func (o providerOption) Description() string {
	switch {
	case o.active:
		return "ativo"
	case o.needsKey && !o.hasKey:
		return "sem chave — Enter para configurar"
	case o.needsKey && o.hasKey:
		return "chave configurada — Enter para ativar"
	case o.baseURL != "" && o.baseURL != "local":
		return o.baseURL
	default:
		return "Enter para ativar"
	}
}

// FilterValue implementa list.Item.
func (o providerOption) FilterValue() string {
	return o.name + " " + o.model
}

// providerPicker é a lista de seleção de providers/modelos.
type providerPicker struct {
	list     list.Model
	registry *provider.Registry
	active   string
	visible  bool
	// connected guarda quais providers conhecidos já têm chave configurada.
	connected map[string]bool
}

// ─── Construtor ───────────────────────────────────────────────────────────────

// newProviderPicker constrói o seletor com TODOS os providers conhecidos,
// marcando os que já estão conectados (chave presente).
func newProviderPicker(reg *provider.Registry) providerPicker {
	active := ""
	if reg != nil {
		active = reg.Primary()
	}

	// Quais providers conhecidos têm chave configurada?
	connected := detectConnectedProviders()

	items := make([]list.Item, 0, len(knownProviders))
	for _, kp := range knownProviders {
		opts := providerOption{
			name:     kp.name,
			model:    kp.defaultModel,
			baseURL:  kp.defaultURL,
			active:   kp.name == active,
			needsKey: kp.needsKey,
			hasKey:   !kp.needsKey || connected[kp.name],
			builtin:  true,
		}
		// Se o provider está no registry, usa o modelo real dele.
		if reg != nil {
			if p := reg.GetProvider(kp.name); p != nil {
				if models := p.Models(); len(models) > 0 {
					opts.model = models[0]
				}
				opts.hasKey = p.IsAvailable()
			}
		}
		items = append(items, opts)
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Selecionar modelo"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = headerSubStyle
	l.Styles.HelpStyle = footerStyle
	// Alinhamento visual com o tema: borda dourada no painel do seletor.
	l.Styles.FilterPrompt = lipgloss.NewStyle().
		Foreground(colorGold).
		Bold(true)
	l.Styles.FilterCursor = lipgloss.NewStyle().
		Foreground(colorGold).
		Background(colorBackground)
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Foreground(colorGold).
		Bold(true)
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().
		Foreground(colorGreen)
	l.SetDelegate(delegate)

	return providerPicker{
		list:      l,
		registry:  reg,
		active:    active,
		visible:   false,
		connected: connected,
	}
}

// detectConnectedProviders verifica no config global quais providers já têm
// chave configurada.
func detectConnectedProviders() map[string]bool {
	result := make(map[string]bool)
	cfg, err := config.Load(".")
	if err != nil || cfg == nil {
		return result
	}
	for _, name := range []string{"deepseek", "openai", "anthropic"} {
		if p := cfg.ResolveProviderConfig(name); p != nil && p.APIKey != "" {
			result[name] = true
		}
	}
	return result
}

// ─── Atualização ──────────────────────────────────────────────────────────────

// update processa mensagens do seletor quando visível.
func (p *providerPicker) update(msg tea.Msg) (providerPicker, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		w := minInt(msg.Width-8, 60)
		h := minInt(msg.Height-8, 20)
		if w < 20 {
			w = 20
		}
		if h < 5 {
			h = 5
		}
		p.list.SetSize(w, h)
	}

	// Teclas do seletor.
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "esc":
			p.visible = false
			return *p, nil
		case "enter":
			selected, ok := p.list.SelectedItem().(providerOption)
			if !ok {
				return *p, nil
			}
			// Provider que precisa de chave e ainda não tem → pedir a chave.
			if selected.needsKey && !selected.hasKey {
				p.visible = false
				return *p, p.requestKey(selected.name)
			}
			// Provider pronto → ativar direto.
			p.active = selected.name
			p.visible = false
			return *p, p.activate(selected.name)
		}
	}

	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return *p, cmd
}

// requestKey emite a mensagem que abre o campo de entrada de API key.
func (p *providerPicker) requestKey(name string) tea.Cmd {
	return func() tea.Msg {
		return apiKeyRequestMsg{name: name}
	}
}

// activate emite a mensagem de troca de provider para o modelo principal.
func (p *providerPicker) activate(name string) tea.Cmd {
	return func() tea.Msg {
		return modelChangeMsg{name: name}
	}
}

// View renderiza o seletor (ou vazio quando oculto).
func (p *providerPicker) View() string {
	if !p.visible {
		return ""
	}
	return p.list.View()
}

// toggle abre/fecha o seletor.
func (p *providerPicker) toggle() {
	p.visible = !p.visible
}

// open abre o seletor com as dimensões atuais da janela.
func (p *providerPicker) open(width, height int) {
	p.resize(width, height)
	p.visible = true
}

// resize aplica o tamanho da janela ao seletor (idempotente).
func (p *providerPicker) resize(width, height int) {
	w := minInt(width-8, 60)
	h := minInt(height-8, 20)
	if w < 20 {
		w = 20
	}
	if h < 5 {
		h = 5
	}
	p.list.SetSize(w, h)
}

// hasVisible reporta se o seletor está aberto.
func (p *providerPicker) hasVisible() bool {
	return p.visible
}

// activeName retorna o provider ativo atual.
func (p *providerPicker) activeName() string {
	return p.active
}

// markConnected marca um provider como conectado após a chave ser salva.
func (p *providerPicker) markConnected(name string) {
	if p.connected == nil {
		p.connected = make(map[string]bool)
	}
	p.connected[name] = true
	// Atualiza o item na lista.
	items := p.list.Items()
	for i, it := range items {
		if opt, ok := it.(providerOption); ok && opt.name == name {
			opt.hasKey = true
			items[i] = opt
		}
	}
	p.list.SetItems(items)
}

// minInt retorna o menor entre dois inteiros.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ─── Mensagens ────────────────────────────────────────────────────────────────

// apiKeyRequestMsg pede ao modelo para abrir o campo de API key para o provider.
type apiKeyRequestMsg struct {
	name string
}

// apiKeySubmitMsg transporta a chave digitada para o modelo.
type apiKeySubmitMsg struct {
	name string
	key  string
}

// modelChangeMsg transporta a ordem de troca de provider para o modelo.
type modelChangeMsg struct {
	name string
}

// applyProviderChange troca o provider ativo no engine e no config.
// Retorna uma mensagem para renderizar na TUI.
func (m *Model) applyProviderChange(reg *provider.Registry, name string) string {
	if reg == nil {
		return errorBubble.Render("Registry de providers indisponível.")
	}
	p := reg.GetProvider(name)
	if p == nil {
		return errorBubble.Render("Provider não encontrado: " + name)
	}

	// Atualiza o engine com um novo adapter do provider selecionado.
	modelID := ""
	if models := p.Models(); len(models) > 0 {
		modelID = models[0]
	}
	adapter := provider.NewProviderAdapter(p, modelID)
	m.engine.SetProvider(adapter)

	// Persiste no config compartilhado.
	cfg, err := config.Load(".")
	if err == nil && cfg != nil {
		cfg.Provider.Primary = name
		if err := config.Save(".", cfg); err == nil {
			return slashBubble.Render(fmt.Sprintf("✓ Modelo ativo: %s (%s)", name, modelID))
		}
	}

	return slashBubble.Render(fmt.Sprintf("✓ Modelo ativo: %s (%s)", name, modelID))
}

// applyAPIKey salva a chave criptografada, conecta o provider e ativa.
func (m *Model) applyAPIKey(name, key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return errorBubble.Render("Chave vazia — nada foi salvo.")
	}

	kp, ok := knownProviderByName(name)
	if !ok {
		return errorBubble.Render("Provider desconhecido: " + name)
	}

	// 1. Salva a chave no config global (criptografada AES-256-GCM).
	if err := saveGlobalAPIKey(name, key); err != nil {
		return errorBubble.Render("Falha ao salvar chave: " + err.Error())
	}

	// 2. Constrói o provider conectado.
	p := kp.build(key, kp.defaultModel, kp.defaultURL)

	// 3. Registra no registry e ativa.
	if m.picker.registry != nil {
		m.picker.registry.Register(p)
		m.picker.registry.SetPrimary(name)
	}
	adapter := provider.NewProviderAdapter(p, kp.defaultModel)
	if m.engine != nil {
		m.engine.SetProvider(adapter)
	}
	m.picker.markConnected(name)
	m.picker.active = name
	m.kernelIdentity.Model = kp.defaultModel

	return slashBubble.Render(fmt.Sprintf("🔑 Chave salva (criptografada) — %s conectado (%s)", name, kp.defaultModel))
}

// saveGlobalAPIKey persiste a chave no config global do Cosca.
func saveGlobalAPIKey(name, key string) error {
	return config.StoreAPIKey(name, key)
}

// ─── Helpers de strings (testes) ──────────────────────────────────────────────

// pickerVisibleReport é um helper de teste para inspecionar a visibilidade.
func pickerVisibleReport(p *providerPicker) bool { return p.visible }

// pickerTitleEstilo é o estilo do título do seletor (isolado para testes).
func pickerTitleEstilo() lipgloss.Style { return headerSubStyle }

var _ = strings.TrimSpace // manter import em builds sem uso direto
