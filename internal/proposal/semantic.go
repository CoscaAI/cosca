package proposal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ─── Validação Semântica (L2) — o corpo da jaula ─────────────────────────────
//
// O OllamaValidator é a materialização da IA interna da jaula: um modelo
// local, isolado, sem rede externa, que confirma se o pensamento da alma é
// verdadeiro. Comunicação MÍNIMA — o Kernel principal só pergunta:
//
//	"este pensamento é coerente/verdadeiro?"
//	→ "VALIDAR" ou "REJEITAR: <motivo em 1 linha>"
//
// Leis:
//   - Resposta restrita (VALIDAR/REJEITAR + motivo curto) — sem divagação
//   - Temperature 0 — determinismo: o julgamento não pode variar por sorte
//   - num_predict limitado — velocidade (o timeout do fluxo é duro)
//   - Erro/timeout → err retornado → o fluxo aplica fail-closed (DENY)
//   - Nunca expõe segredos no prompt: só o contrato da proposta

// OllamaValidator é a validação semântica via IA local (Ollama na jaula).
type OllamaValidator struct {
	baseURL string
	model   string
	client  *http.Client
}

// NewOllamaValidator cria o validador semântico.
//   - baseURL: servidor Ollama local (default http://127.0.0.1:11434)
//   - model:   modelo local da jaula (ex: qwen2.5-coder:14b)
//   - timeout: tempo máximo da chamada (default 3s)
func NewOllamaValidator(baseURL, model string, timeout time.Duration) *OllamaValidator {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:11434"
	}
	if model == "" {
		model = "qwen2.5-coder:14b"
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &OllamaValidator{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		model:   model,
		client:  &http.Client{Timeout: timeout},
	}
}

// chatMessage é o formato de mensagem do /api/chat do Ollama.
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatRequest é o corpo da requisição para /api/chat.
// KeepAlive: a jaula é mantida sempre quente ("10m" = residente por 10 min
// após cada uso; o Ollama descarrega o modelo após 5min por padrão e o
// recarregamento de um modelo 14B custa ~14s — inaceitável para um gate
// que precisa responder em milissegundos). Com keep-alive, o primeiro
// request aquece o modelo e os seguintes são rápidos.
type chatRequest struct {
	Model     string         `json:"model"`
	Messages  []chatMessage  `json:"messages"`
	Stream    bool           `json:"stream"`
	KeepAlive string         `json:"keep_alive,omitempty"`
	Options   map[string]any `json:"options"`
}

// chatResponse é a resposta do /api/chat (stream=false).
type chatResponse struct {
	Message chatMessage `json:"message"`
}

// Validate julga o pensamento da proposta contra a IA local da jaula.
// Devolve:
//   - reason == "" → o corpo confirma: pensamento válido
//   - reason != "" → o corpo rejeita com o motivo em 1 linha
//   - err != nil → a jaula falhou (offline/timeout) — o fluxo nega (fail-closed)
func (v *OllamaValidator) Validate(ctx context.Context, p *Proposal) (string, error) {
	if p == nil {
		return "", fmt.Errorf("proposta nula")
	}

	prompt := buildSemanticPrompt(p)

	body, err := json.Marshal(chatRequest{
		Model: v.model,
		Messages: []chatMessage{
			{Role: "system", Content: "Você é o Kernel isolado da jaula — o corpo que confirma a verdade. Julgue o pensamento contra a realidade declarada. Responda apenas: VALIDAR, ou REJEITAR: seguido de um motivo em 1 linha."},
			{Role: "user", Content: prompt},
		},
		Stream:    false,
		KeepAlive: "10m",
		Options: map[string]any{
			"temperature": 0,
			"num_predict": 40,
		},
	})
	if err != nil {
		return "", fmt.Errorf("montar requisição: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		v.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("criar requisição: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("jaula inacessível: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("jaula respondeu %s", resp.Status)
	}

	var parsed chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("decodificar resposta da jaula: %w", err)
	}

	answer := strings.TrimSpace(parsed.Message.Content)
	upper := strings.ToUpper(answer)

	switch {
	case upper == "VALIDAR":
		// Protocolo do juiz: aprovação EXPLÍCITA E EXATA. Qualquer sufixo
		// ("VALIDAR e também...", "VALIDAR.") é resposta inválida →
		// fail-closed. O OllamaValidator é o tradutor do corpo da jaula:
		// nunca normaliza ambiguidade em aprovação. Resposta vazia nunca
		// significa aprovação (fail-closed).
		return "VALIDAR", nil
	case strings.HasPrefix(upper, "REJEITAR"):
		reason := strings.TrimSpace(strings.TrimPrefix(answer, "REJEITAR"))
		reason = strings.TrimPrefix(reason, ":")
		reason = strings.TrimSpace(reason)
		if reason == "" {
			reason = "pensamento rejeitado pelo corpo da jaula"
		}
		return reason, nil
	default:
		// O corpo não entendeu ou divagou — não confiamos no que não é claro.
		return "", fmt.Errorf("resposta ambígua da jaula: %q", answer)
	}
}

// buildSemanticPrompt monta o pensamento a ser julgado — apenas o contrato
// da proposta, sem segredos, sem contexto extra (a jaula julga o que vê).
func buildSemanticPrompt(p *Proposal) string {
	var b strings.Builder
	b.WriteString("PENSAMENTO A JULGAR:\n")
	fmt.Fprintf(&b, "- ação: %s\n", p.Action)
	if p.Target != "" {
		fmt.Fprintf(&b, "- alvo: %s\n", p.Target)
	}
	fmt.Fprintf(&b, "- motivo: %s\n", p.Motive)
	if p.State != "" {
		fmt.Fprintf(&b, "- estado: %s\n", p.State)
	}
	fmt.Fprintf(&b, "- classe de risco: %s\n", p.Risk)
	b.WriteString("\nÉ coerente e verdadeiro? Responda EXATAMENTE uma das duas formas: VALIDAR (palavra única, sem explicação) ou REJEITAR: <motivo em 1 linha>.")
	return b.String()
}
