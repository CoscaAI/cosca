package brainweb

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
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

// TestHandler_ServeAssets garante que os assets do visualizador (self-hostados
// via go:embed) são servidos com o MIME correto e com corpo NÃO-vazio — um
// corpo 0-byte indicaria que o asset não foi embutido de fato.
func TestHandler_ServeAssets(t *testing.T) {
	h := NewHandler(testAgents(), testSkills(), "1.5.0")

	cases := []struct {
		path string
		ct   string
	}{
		{path: "/brain/style.css", ct: "text/css"},
		{path: "/brain/jsm/loaders/OBJLoader.js", ct: "application/javascript"},
		{path: "/brain/jsm/controls/OrbitControls.js", ct: "application/javascript"},
		{path: "/brain/models/brain.obj", ct: "text/plain"},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			h.Serve(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d, esperado 200", rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, tc.ct) {
				t.Fatalf("content-type=%q, esperado prefix %q", ct, tc.ct)
			}
			if rec.Body.Len() == 0 {
				t.Fatal("body vazio — asset não foi embutido de fato")
			}
		})
	}
}

// TestHandler_TraversalBloqueadoRegras reforça a proteção contra path
// traversal: variações de encoding (URL-encoded, duplo dot, slash), todas
// devem devolver 404 e NÃO vazar conteúdo de arquivo real.
func TestHandler_TraversalBloqueadoRegras(t *testing.T) {
	h := NewHandler(testAgents(), testSkills(), "1.5.0")

	cases := []struct {
		name string
		path string
	}{
		{name: "dotdot_literal", path: "/brain/../../etc/passwd"},
		{name: "encoded_slash", path: "/brain/..%2f..%2fetc/passwd"},
		{name: "encoded_dot", path: "/brain/%2e%2e/%2e%2e/etc/passwd"},
		{name: "double_dot_slash", path: "/brain/....//....//etc/passwd"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			h.Serve(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status=%d, esperado 404 (traversal deve ser bloqueado)", rec.Code)
			}
			body := rec.Body.String()
			// Conteúdo característico de /etc/passwd — nunca deve vazar.
			if strings.Contains(body, "root:") || strings.Contains(body, "bin/bash") {
				t.Fatalf("body vaza conteúdo de arquivo real: %q", body)
			}
		})
	}
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
