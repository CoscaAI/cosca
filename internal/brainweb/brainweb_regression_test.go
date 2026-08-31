package brainweb

import (
	"reflect"
	"testing"

	"github.com/CoscaAI/cosca/internal/knowledge"
)

// TestActivity_SemCampoPrompt garante que a projeção pública de Activity NÃO
// expõe prompt/resposta/args do usuário. Após a correção do bug G6 o campo foi
// renomeado para Action (rótulo seguro). Esta guarda (reflect) impede que o
// campo Prompt vaze de novo no payload público.
func TestActivity_SemCampoPrompt(t *testing.T) {
	prohibited := []string{"Prompt", "PromptText", "UserInput", "UserPrompt", "Args", "Arguments", "Response"}
	walkFields(t, reflect.TypeOf(Activity{}), prohibited)
}

// TestObservatory_StatusVazioUnknown garante que um item de conhecimento com
// Status == "" (vazio) cai no ramo "UNKNOWN" (st == ""), e não em um estado
// nulo/indefinido — a base da "física semântica" honesta.
func TestObservatory_StatusVazioUnknown(t *testing.T) {
	b := NewObservatoryBuilder(func() []knowledge.KnowledgeItem {
		return []knowledge.KnowledgeItem{
			{ID: "K-9", Title: "Item sem status", Status: "", Confidence: 0.1},
		}
	}, nil, nil)
	obs := b.Build()

	if obs.Cognitive.KnowledgeItems != 1 {
		t.Fatalf("knowledge_items=%d, esperado 1", obs.Cognitive.KnowledgeItems)
	}

	found := false
	for _, c := range obs.Constellations {
		if c.Epistemic != "UNKNOWN" {
			continue
		}
		found = true
		if c.Count != 1 {
			t.Fatalf("constelação UNKNOWN count=%d, esperado 1", c.Count)
		}
		for _, it := range c.Items {
			if it.Status != "UNKNOWN" {
				t.Fatalf("item status=%q, esperado UNKNOWN (status vazio deve cair em UNKNOWN)", it.Status)
			}
		}
	}
	if !found {
		t.Fatalf("não há constelação UNKNOWN: %+v", obs.Constellations)
	}
}

// TestObservatory_StatsFnPopulaAgentsSkills garante que a função statsFn
// injetada no observatório popula Agents e Skills quando não-nil (o
// observatório reporta os stats reais do runtime).
func TestObservatory_StatsFnPopulaAgentsSkills(t *testing.T) {
	b := NewObservatoryBuilder(nil, nil, func() CognitiveSnapshot {
		return CognitiveSnapshot{Agents: 9, Skills: 4}
	})
	obs := b.Build()

	if obs.Cognitive.Agents != 9 {
		t.Fatalf("agents=%d, esperado 9", obs.Cognitive.Agents)
	}
	if obs.Cognitive.Skills != 4 {
		t.Fatalf("skills=%d, esperado 4", obs.Cognitive.Skills)
	}
}
