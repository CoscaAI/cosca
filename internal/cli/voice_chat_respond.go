package cli

// respondFromWorldState is the "interpreta" step of the voice-chat loop. It is a
// PURE function: it turns the current multimodal WorldState (what the vision
// Perception Loop is seeing NOW, as projected by the bus) plus the last utterance
// (what the Don asked) into a PT-BR spoken response. It has no side effects and
// no I/O, so it is trivially unit-testable (voice_chat_respond_test.go).
//
// Degradation contract (graceful, never panics):
//   - nil WorldState OR no vision payload  → "A visão não está ativa agora."
//   - vision present but zero entities     → "Não estou vendo objetos claros agora."
//   - entities present                     → "Estou vendo N objeto(s): <lista>…"
//
// The utterance is appended (when non-empty) so the COSCA acknowledges what the
// Don actually asked.

import (
	"fmt"
	"strings"

	"github.com/CoscaAI/cosca/internal/perception/bus"
)

func respondFromWorldState(state *bus.WorldState, utterance string) string {
	u := strings.TrimSpace(utterance)

	// 1) No perception bus state or no vision payload → vision not active.
	if state == nil || state.Vision == nil {
		if u != "" {
			return fmt.Sprintf("A visão não está ativa agora. Você perguntou: %q.", u)
		}
		return "A visão não está ativa agora."
	}

	entities := state.Vision.Entities

	// 2) Vision is active but nothing is clearly detected.
	if len(entities) == 0 {
		if u != "" {
			return fmt.Sprintf("Não estou vendo objetos claros agora. Você perguntou: %q.", u)
		}
		return "Não estou vendo objetos claros agora."
	}

	// 3) Vision sees entities — describe label + confidence (%) + depth (m).
	noun := "objeto"
	if len(entities) > 1 {
		noun = "objetos"
	}
	labels := make([]string, 0, len(entities))
	for _, e := range entities {
		label := strings.TrimSpace(e.Label)
		if label == "" {
			label = "objeto"
		}
		labels = append(labels, fmt.Sprintf("%s (%.0f%%, %.1fm)", label, e.Confidence*100, e.Depth))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Estou vendo %d %s: ", len(entities), noun)
	b.WriteString(strings.Join(labels, ", "))
	b.WriteString(".")
	if u != "" {
		fmt.Fprintf(&b, " Você perguntou: %q.", u)
	}
	return b.String()
}
