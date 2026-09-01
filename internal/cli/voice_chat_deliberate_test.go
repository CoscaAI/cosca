package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/perception/bus"
	"github.com/CoscaAI/cosca/internal/worldmodel"
	"github.com/CoscaAI/cosca/internal/worldmodel/vision"
)

// nopLogger devolve um logger silencioso para os testes de deliberação.
func nopLogger() zerolog.Logger { return zerolog.New(zerolog.Nop()) }

// ──────────────────────────────────────────────────────────────
// Fake provider (chat.ChatProvider) para injetar o cérebro nos testes,
// sem tocar no Ollama.
// ──────────────────────────────────────────────────────────────

// fakeBrainProvider returns a fixed smart response and records the prompt it
// received (so a test can assert the perceptual context was built).
type fakeBrainProvider struct {
	response string
	err      error
	gotMsg   chat.Message
}

func (f *fakeBrainProvider) Chat(_ context.Context, messages []chat.Message, _ chat.ChatOptions) (*chat.ChatResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	// A última mensagem (user) é o prompt perceptual — guardamos para o assert.
	if len(messages) > 0 {
		f.gotMsg = messages[len(messages)-1]
	}
	content := f.response
	if content == "" {
		content = "resposta inteligente fake"
	}
	return &chat.ChatResponse{
		ID:    "resp-1",
		Model: "fake-brain",
		Choices: []chat.Choice{
			{
				Index: 0,
				Message: chat.Message{
					Role:    chat.RoleAssistant,
					Content: content,
				},
				FinishReason: chat.FinishReasonStop,
			},
		},
		Usage: chat.Usage{TotalTokens: 42},
	}, nil
}

func (f *fakeBrainProvider) ChatStream(context.Context, []chat.Message, chat.ChatOptions) (chat.ChatStream, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeBrainProvider) Model() string { return "fake-brain" }
func (f *fakeBrainProvider) Name() string  { return "fake" }
func (f *fakeBrainProvider) Close() error  { return nil }

// ──────────────────────────────────────────────────────────────
// respondWithDeliberation — cérebro usa os sentidos
// ──────────────────────────────────────────────────────────────

// TestRespondWithDeliberation_NoBrain verifica a degradação graciosa: sem
// cérebro (provider nil), a deliberação cai no template antigo.
func TestRespondWithDeliberation_NoBrain(t *testing.T) {
	SetVoiceBrain(nil) // sem cérebro
	defer SetVoiceBrain(nil)

	st := &bus.WorldState{Vision: &vision.Observation{
		Entities: []worldmodel.WorldEntity{
			{Label: "pessoa", Confidence: 0.92, Depth: 3.1},
			{Label: "mesa", Confidence: 0.80, Depth: 2.0},
		},
	}}
	got, err := respondWithDeliberation(context.Background(), st, "o que você está vendo?", nil)
	if err != nil {
		t.Fatalf("no-brain should not return an error, got %v", err)
	}
	if !strings.Contains(got, "Estou vendo 2 objetos") || !strings.Contains(got, "pessoa (92%, 3.1m)") {
		t.Fatalf("no-brain fallback should be the template, got %q", got)
	}
}

// TestRespondWithDeliberation_BrainSmart verifica o caminho inteligente: com um
// provider fake injetado, o COSCa devolve a resposta do cérebro (e o prompt
// carrega o contexto perceptual + a pergunta do Don).
func TestRespondWithDeliberation_BrainSmart(t *testing.T) {
	fp := &fakeBrainProvider{response: "Vejo uma pessoa a 3 metros e uma mesa a 2 metros."}
	brain := newVoiceBrain(fp, 0, 0, nopLogger())
	SetVoiceBrain(brain)
	defer SetVoiceBrain(nil)

	st := &bus.WorldState{Vision: &vision.Observation{
		Entities: []worldmodel.WorldEntity{
			{Label: "pessoa", Confidence: 0.92, Depth: 3.1},
			{Label: "mesa", Confidence: 0.80, Depth: 2.0},
		},
	}}
	memHints := []string{`áudio: "o que você está vendo?"`, "visão: uma janela aberta"}

	got, err := respondWithDeliberation(context.Background(), st, "o que você está vendo?", memHints)
	if err != nil {
		t.Fatalf("brain smart returned error: %v", err)
	}
	if !strings.Contains(got, "Vejo uma pessoa") || !strings.Contains(got, "mesa") {
		t.Fatalf("brain smart response = %q, want the fake smart reply", got)
	}

	// O prompt enviado ao modelo deve conter o contexto perceptual e a pergunta.
	need := []string{"pessoa (conf 92%, ~3.1m)", "mesa (conf 80%, ~2.0m)", "o que você está vendo?", "lembrança"}
	for _, w := range need {
		if !strings.Contains(fp.gotMsg.Content, w) {
			t.Errorf("brain prompt missing %q in:\n%s", w, fp.gotMsg.Content)
		}
	}
}

// TestRespondWithDeliberation_BrainFails verifica a degradação graciosa quando o
// modelo falha: a deliberação devolve o template (nunca quebra o loop).
func TestRespondWithDeliberation_BrainFails(t *testing.T) {
	fp := &fakeBrainProvider{err: errors.New("ollama offline")}
	brain := newVoiceBrain(fp, 0, 0, nopLogger())
	SetVoiceBrain(brain)
	defer SetVoiceBrain(nil)

	st := &bus.WorldState{Vision: &vision.Observation{
		Entities: []worldmodel.WorldEntity{{Label: "gato", Confidence: 0.95, Depth: 1.2}},
	}}
	got, err := respondWithDeliberation(context.Background(), st, "o que é isso?", nil)
	if err == nil {
		t.Fatal("brain failure should return a (non-nil) degradation error")
	}
	if !strings.Contains(got, "Estou vendo 1 objeto") || !strings.Contains(got, "gato (95%, 1.2m)") {
		t.Fatalf("brain failure fallback should be the template, got %q", got)
	}
}

// TestRespondWithDeliberation_BrainEmptyContent verifica que uma resposta vazia
// do modelo também degrada para o template (nunca fala string vazia).
func TestRespondWithDeliberation_BrainEmptyContent(t *testing.T) {
	fp := &fakeBrainProvider{response: "   "}
	brain := newVoiceBrain(fp, 0, 0, nopLogger())
	SetVoiceBrain(brain)
	defer SetVoiceBrain(nil)

	got, err := respondWithDeliberation(context.Background(), &bus.WorldState{Vision: &vision.Observation{}}, "oi?", nil)
	if err == nil {
		t.Fatal("empty model content should degrade with an error")
	}
	if strings.TrimSpace(got) == "" {
		t.Fatal("empty model content must NOT speak an empty string — template fallback expected")
	}
}

// ──────────────────────────────────────────────────────────────
// buildPerceptualPrompt
// ──────────────────────────────────────────────────────────────

// TestBuildPerceptualPrompt verifica que o prompt monta o contexto perceptual
// completo (visão + áudio + memória episódica + pergunta).
func TestBuildPerceptualPrompt(t *testing.T) {
	st := &bus.WorldState{
		Vision: &vision.Observation{
			Entities: []worldmodel.WorldEntity{
				{Label: "janela", Confidence: 0.75, Depth: 5.4},
				{Label: "mesa", Confidence: 0.81, Depth: 2.0},
			},
		},
		Audio: &bus.AudioPayload{Text: "o que você está vendo?", IsFinal: true},
	}
	mem := []string{`áudio: "você viu um carro antes"`}
	prompt := buildPerceptualPrompt(st, "o que você está vendo?", mem)

	for _, w := range []string{
		"janela (conf 75%, ~5.4m)",
		"mesa (conf 81%, ~2.0m)",
		`o que você está vendo?`, // áudio ouvido agora
		"lembrança",              // memória episódica
		"PERGUNTA DO DON",
		"português do Brasil",
	} {
		if !strings.Contains(prompt, w) {
			t.Errorf("prompt missing %q in:\n%s", w, prompt)
		}
	}
}

// TestBuildPerceptualPrompt_NoState verifica que um WorldState nulo (bus sem
// projeção) não quebra a montagem — o cérebro é informado da indisponibilidade.
func TestBuildPerceptualPrompt_NoState(t *testing.T) {
	prompt := buildPerceptualPrompt(nil, "oi", nil)
	if !strings.Contains(prompt, "indisponível") || !strings.Contains(prompt, "oi") {
		t.Fatalf("nil-state prompt should say vision unavailable and quote the question, got:\n%s", prompt)
	}
}
