package cli

// voice_chat_deliberate.go — FASE 1: "o cérebro usar os sentidos".
//
// Substitui o TEMPLATE de `respondFromWorldState` (que só repete "Estou vendo N
// objetos") por uma DELIBERAÇÃO com o modelo: o COSCa recebe o WorldState
// multimodal (visão + áudio) + a memória episódica (o que lembra) e a pergunta
// do Don, e o modelo (o cérebro, ex.: qwen3:8b via Ollama) transforma isso em
// uma resposta INTELIGENTE em PT-BR.
//
// Contrato de degradação graciosa (nunca quebra):
//   - sem cérebro (provider nil / não construído) → cai no template antigo.
//   - o modelo falha / timeout / resposta vazia          → cai no template antigo.
//
// O cérebro é INJETÁVEL (SetVoiceBrain) para os testes: um provider fake pode
// ser injetado sem tocar no Ollama, e um brain nil exercita o fallback.

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/CoscaAI/cosca/internal/chat"
	chatprovider "github.com/CoscaAI/cosca/internal/chat/provider"
	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/memory"
	"github.com/CoscaAI/cosca/internal/perception/bus"
	"github.com/CoscaAI/cosca/internal/worldmodel/vision"
)

// defaultVoiceBrainTimeout é o limite de tempo da deliberação (a chamada ao
// modelo). Generoso de propósito: o Ollama pode carregar o modelo na primeira
// chamada. Acima dele o COSCa degrada graciosamente para o template.
const defaultVoiceBrainTimeout = 60 * time.Second

// ──────────────────────────────────────────────────────────────
// voiceBrain — o cérebro (provider de chat injetável)
// ──────────────────────────────────────────────────────────────

// voiceBrain is the deliberation brain: it wraps the chat.ChatProvider (the
// model) so respondWithDeliberation can call it with a bounded timeout and
// degrade gracefully. A nil Provider means the brain is absent (template
// fallback) — the honest "no cancer" state.
type voiceBrain struct {
	// provider é o modelo (o registro de chat resolve primário + fallbacks).
	provider chat.ChatProvider
	// timeout é o teto da chamada ao modelo.
	timeout time.Duration
	// system é o prompt de sistema (identidade + boundary do COSCa).
	system string
	// temperature é o parâmetro de amostragem (0.0–2.0). Para uma percepção
	// aterrada, valores mais baixos respondem de forma mais determinística.
	temperature float64
	// logger é o sink de logs da deliberação.
	logger zerolog.Logger
}

// newVoiceBrain creates a brain. A nil provider yields a disabled brain.
func newVoiceBrain(p chat.ChatProvider, timeout time.Duration, temperature float64, logger zerolog.Logger) *voiceBrain {
	if timeout <= 0 {
		timeout = defaultVoiceBrainTimeout
	}
	if temperature == 0 {
		temperature = 0.3
	}
	return &voiceBrain{
		provider:    p,
		timeout:     timeout,
		temperature: temperature,
		system:      defaultBrainSystemPrompt(),
		logger:      logger,
	}
}

// Enabled reports whether the brain can actually deliberate (a provider is set).
func (b *voiceBrain) Enabled() bool { return b != nil && b.provider != nil }

// Shutdown releases the provider (no-op when nil / already closed).
func (b *voiceBrain) Shutdown() error {
	if b == nil || b.provider == nil {
		return nil
	}
	return b.provider.Close()
}

// deliberate sends the perceptual prompt to the model and returns the trimmed
// response. It returns an error (never a partial string) on any failure so the
// caller can degrade.
func (b *voiceBrain) deliberate(ctx context.Context, prompt string) (string, error) {
	if b == nil || b.provider == nil {
		return "", fmt.Errorf("voice brain: no provider")
	}
	cctx, cancel := context.WithTimeout(ctx, b.timeout)
	defer cancel()

	messages := []chat.Message{
		{Role: chat.RoleSystem, Content: b.system},
		{Role: chat.RoleUser, Content: prompt},
	}
	opts := chat.DefaultChatOptions()
	opts.Temperature = b.temperature
	resp, err := b.provider.Chat(cctx, messages, opts)
	if err != nil {
		return "", err
	}
	if resp == nil || len(resp.Choices) == 0 {
		return "", fmt.Errorf("voice brain: empty response")
	}
	out := strings.TrimSpace(resp.Choices[0].Message.Content)
	if out == "" {
		return "", fmt.Errorf("voice brain: empty content")
	}
	return out, nil
}

// ──────────────────────────────────────────────────────────────
// Brain default (package-level, injectable)
// ──────────────────────────────────────────────────────────────

var (
	voiceBrainMu      sync.RWMutex
	voiceBrainDefault *voiceBrain
)

// SetVoiceBrain injeta o cérebro padrão usado por respondWithDeliberation. Os
// testes injetam um provider fake (ou nil para exercitar o fallback).
func SetVoiceBrain(b *voiceBrain) {
	voiceBrainMu.Lock()
	voiceBrainDefault = b
	voiceBrainMu.Unlock()
}

// getVoiceBrain retorna o cérebro padrão atual (ou nil).
func getVoiceBrain() *voiceBrain {
	voiceBrainMu.RLock()
	defer voiceBrainMu.RUnlock()
	return voiceBrainDefault
}

// buildVoiceBrain monta o cérebro a partir da config do provider (a mesma que
// o `cosca chat` usa): registra os providers de chat, propaga a config para
// env vars e seleciona o primário declarado no config (ex.: ollama/qwen3:8b).
// Quando nenhum provider é selecionável retorna um brain DISABLED (provider
// nil) — o fallback para o template — nunca um erro fatal.
func buildVoiceBrain(ctx context.Context, cfg *config.Config, logger zerolog.Logger) *voiceBrain {
	if cfg == nil {
		logger.Warn().Msg("voice brain: no config — using template fallback")
		return newVoiceBrain(nil, 0, 0, logger)
	}

	// Reusa o MESMO pipeline de chat existente (não inventa um novo): registra
	// as factories e deixa o Select resolver o provider da config.
	registry := chat.NewChatRegistry()
	if err := chatprovider.RegisterChatProviders(registry, nil); err != nil {
		logger.Warn().Err(err).Msg("voice brain: register chat providers failed")
	}
	// Propaga a config (provider.name/model/base_url) para as env vars que as
	// factories leem (ollama → COSCA_OLLAMA_MODEL / OLLAMA_HOST).
	loadChatEnv(cfg)

	regCfg := chat.DefaultChatRegistryConfig()
	if cfg.Provider.Name != "" {
		regCfg.Primary = cfg.Provider.Name
	}
	if err := registry.Select(ctx, regCfg); err != nil {
		logger.Warn().Err(err).Msg("voice brain: no chat provider selected — using template fallback")
		return newVoiceBrain(nil, 0, 0, logger)
	}

	timeout := cfg.Provider.Timeout
	if timeout <= 0 {
		timeout = defaultVoiceBrainTimeout
	}
	logger.Info().
		Str("provider", registry.Name()).
		Str("model", registry.Model()).
		Dur("timeout", timeout).
		Msg("voice brain: deliberation provider ready")
	return newVoiceBrain(registry, timeout, cfg.Provider.Temperature, logger)
}

// ──────────────────────────────────────────────────────────────
// respondWithDeliberation — o cérebro usa os sentidos
// ──────────────────────────────────────────────────────────────

// respondWithDeliberation is the Phase-1 "interpreta" step: turns the current
// multimodal WorldState (what vision is seeing + what audio heard), the
// episodic memory hints (what the COSCa recalls) and the Don's question into an
// INTELLIGENT PT-BR response via the brain model (qwen3:8b via Ollama).
//
// Graceful degradation contract:
//   - no brain (provider nil)             → template (respondFromWorldState).
//   - brain call fails / timeout / empty  → template (never breaks the loop).
//   - brain succeeds                      → the smart response.
//
// The returned error is non-nil ONLY when deliberation was attempted but
// degraded to the template — the caller should log it but still speak the
// returned string.
func respondWithDeliberation(ctx context.Context, state *bus.WorldState, utterance string, memoryHints []string) (string, error) {
	brain := getVoiceBrain()
	if !brain.Enabled() {
		// Honesto: sem cérebro, o COSCa ainda consegue responder — só que no
		// template (a cognição descreve, não delibera).
		return respondFromWorldState(state, utterance), nil
	}

	prompt := buildPerceptualPrompt(state, utterance, memoryHints)
	resp, err := brain.deliberate(ctx, prompt)
	if err != nil {
		brain.logger.Warn().Err(err).Msg("voice brain: deliberation failed — falling back to template")
		return respondFromWorldState(state, utterance), err
	}
	return resp, nil
}

// buildMemoryHints consulta a memória episódica (FASE D) com a pergunta do Don
// e devolve uma lista de lembranças (áudio + visão) para injetar no contexto da
// deliberação. Degrada graciosamente: sem manager, erro de consulta, ou nenhum
// registro → retorna nil (o cérebro delibera só com o estado atual).
func buildMemoryHints(ctx context.Context, mem *memoryManagerAdapter, utterance string, limit int) []string {
	if mem == nil {
		return nil
	}
	if limit <= 0 {
		limit = 4
	}
	records, err := mem.QueryEpisodic(ctx, memory.EpisodicQuery{
		Query: utterance,
		Limit: limit,
	})
	if err != nil || len(records) == 0 {
		return nil
	}
	hints := make([]string, 0, len(records))
	for _, r := range records {
		var parts []string
		if text := strings.TrimSpace(r.AudioText); text != "" {
			parts = append(parts, "áudio: \""+truncate(text, 160)+"\"")
		}
		if summary := strings.TrimSpace(r.VisionSummary); summary != "" {
			parts = append(parts, "visão: "+truncate(summary, 160))
		}
		if len(parts) == 0 {
			continue
		}
		hints = append(hints, strings.Join(parts, " · "))
	}
	return hints
}

// ──────────────────────────────────────────────────────────────
// Prompt builder
// ──────────────────────────────────────────────────────────────

// buildPerceptualPrompt monta o contexto perceptual (visão + áudio + memória
// episódica + pergunta do Don) em um prompt para o cérebro. É uma função pura
// (sem I/O), testável.
func buildPerceptualPrompt(state *bus.WorldState, utterance string, memoryHints []string) string {
	var b strings.Builder

	b.WriteString(defaultBrainSystemPrompt())
	b.WriteString("\n\n")
	b.WriteString("CONTEXTO PERCEPTUAL AGORA (o que o COSCa vê/ouve):\n")

	// Visão.
	if state == nil {
		b.WriteString("- Visão: indisponível (o bus ainda não publicou um WorldState)\n")
	} else if state.Vision == nil {
		b.WriteString("- Visão: não ativa neste instante\n")
	} else {
		b.WriteString("- Visão: " + summarizeVision(state.Vision) + "\n")
	}

	// Áudio (o que o Don falou, transcrito pelo STT).
	if state != nil && state.Audio != nil && strings.TrimSpace(state.Audio.Text) != "" {
		b.WriteString("- Áudio ouvido agora: \"" + strings.TrimSpace(state.Audio.Text) + "\"\n")
	}

	// Degradação (nota honesta para o modelo não alucinar).
	if state != nil && state.Degraded {
		notes := strings.Join(state.Warnings, "; ")
		if notes == "" {
			notes = "sinal degradado"
		}
		b.WriteString("- Nota: a percepção está degradada (" + notes + ")\n")
	}

	// Memória episódica.
	if len(memoryHints) > 0 {
		b.WriteString("\nMEMÓRIA EPISÓDICA (o que você LEMBRA do que viu/ouviu antes):\n")
		for i, h := range memoryHints {
			h = strings.TrimSpace(h)
			if h == "" {
				continue
			}
			fmt.Fprintf(&b, "- [lembrança %d] %s\n", i+1, h)
		}
	}

	b.WriteString("\nPERGUNTA DO DON:\n\"" + strings.TrimSpace(utterance) + "\"\n")
	b.WriteString("\nResponda em português do Brasil, como o COSCa: 1-3 frases, natural e inteligente, ")
	b.WriteString("baseando-se APENAS no que percebeu/lembrou. Se não percebeu algo, diga honestamente. ")
	b.WriteString("Não invente objetos, sons ou memórias.")
	return b.String()
}

// summarizeVision transforma a observação de visão em uma descrição curta
// legível (labels + confiança + profundidade). Retorna texto claro quando a
// visão está ativa mas não detectou nada, e quando há degradação.
func summarizeVision(obs *vision.Observation) string {
	if obs == nil {
		return "sem observação"
	}
	if len(obs.Entities) == 0 {
		summary := "nenhum objeto claro detectado"
		if len(obs.Warnings) > 0 {
			summary += " (percepção degradada: " + strings.Join(obs.Warnings, "; ") + ")"
		}
		return summary
	}
	labels := make([]string, 0, len(obs.Entities))
	for _, e := range obs.Entities {
		label := strings.TrimSpace(e.Label)
		if label == "" {
			label = "objeto"
		}
		labels = append(labels, fmt.Sprintf("%s (conf %.0f%%, ~%.1fm)", label, e.Confidence*100, e.Depth))
	}
	return strings.Join(labels, ", ")
}

// defaultBrainSystemPrompt é a identidade do COSCa anexada a toda deliberação.
func defaultBrainSystemPrompt() string {
	return "Você é o COSCa, um agente de IA com percepção multimodal: você VÊ (visão por " +
		"modelo ONNX local), OUVE (STT sherpa local) e LEMBRA (memória episódica). " +
		"Você está observando o mundo real AGORA e a pessoa está falando com você. " +
		"Responda com inteligência e naturalidade, com base no contexto perceptual que recebe."
}
