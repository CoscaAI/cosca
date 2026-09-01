package cli

import (
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/perception/bus"
	"github.com/CoscaAI/cosca/internal/worldmodel"
	"github.com/CoscaAI/cosca/internal/worldmodel/vision"
)

// TestRespondFromWorldState_NoState verifies the graceful degradation when the
// bus has no WorldState at all (no perception loop / bus not running).
func TestRespondFromWorldState_NoState(t *testing.T) {
	got := respondFromWorldState(nil, "")
	if got != "A visão não está ativa agora." {
		t.Fatalf("no-state, no-utterance = %q, want %q", got, "A visão não está ativa agora.")
	}

	got = respondFromWorldState(nil, "o que você está vendo?")
	if !strings.Contains(got, "A visão não está ativa agora") || !strings.Contains(got, "o que você está vendo?") {
		t.Fatalf("no-state + utterance = %q, want a vision-inactive reply quoting the utterance", got)
	}
}

// TestRespondFromWorldState_NoVision verifies the degradation when a WorldState
// exists but carries no vision payload (vision source not wired / degraded).
func TestRespondFromWorldState_NoVision(t *testing.T) {
	st := &bus.WorldState{Version: 1}
	got := respondFromWorldState(st, "")
	if got != "A visão não está ativa agora." {
		t.Fatalf("no vision payload = %q, want vision-inactive reply", got)
	}
}

// TestRespondFromWorldState_NoEntities verifies the "não estou vendo objetos
// claros" branch when vision is active but detecte nothing.
func TestRespondFromWorldState_NoEntities(t *testing.T) {
	st := &bus.WorldState{Vision: &vision.Observation{}}
	got := respondFromWorldState(st, "")
	if got != "Não estou vendo objetos claros agora." {
		t.Fatalf("no entities = %q, want %q", got, "Não estou vendo objetos claros agora.")
	}

	got = respondFromWorldState(st, "o que há na tela?")
	if !strings.Contains(got, "Não estou vendo objetos claros agora") || !strings.Contains(got, "o que há na tela?") {
		t.Fatalf("no entities + utterance = %q, want no-clear-objects reply quoting the utterance", got)
	}
}

// TestRespondFromWorldState_Entities verifies the main branch: the response
// describes what the vision actually detected (label + depth) and acknowledges
// the Don's question.
func TestRespondFromWorldState_Entities(t *testing.T) {
	st := &bus.WorldState{Vision: &vision.Observation{
		Entities: []worldmodel.WorldEntity{
			{Label: "pessoa", Confidence: 0.92, Depth: 3.1},
			{Label: "mesa de trabalho", Confidence: 0.81, Depth: 2.0},
			{Label: "janela", Confidence: 0.75, Depth: 5.4},
		},
	}}

	got := respondFromWorldState(st, "o que você está vendo?")
	for _, want := range []string{
		"Estou vendo 3 objetos",
		"pessoa (92%, 3.1m)",
		"mesa de trabalho (81%, 2.0m)",
		"janela (75%, 5.4m)",
		"Você perguntou: \"o que você está vendo?\"",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("entities reply missing %q in: %q", want, got)
		}
	}
}

// TestRespondFromWorldState_SingleEntity verifies singular pluralisation + the
// append-utterance branch.
func TestRespondFromWorldState_SingleEntity(t *testing.T) {
	st := &bus.WorldState{Vision: &vision.Observation{
		Entities: []worldmodel.WorldEntity{{Label: "gato", Confidence: 0.95, Depth: 1.2}},
	}}
	got := respondFromWorldState(st, "o que é isso?")
	if !strings.HasPrefix(got, "Estou vendo 1 objeto") {
		t.Fatalf("single entity should use singular 'objeto', got %q", got)
	}
	if !strings.Contains(got, "gato (95%, 1.2m)") {
		t.Fatalf("single entity reply missing label/depth: %q", got)
	}
	if !strings.Contains(got, "o que é isso?") {
		t.Fatalf("single entity reply missing utterance: %q", got)
	}
}
