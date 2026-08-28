// Package worldloop é o LOOP cognitivo de mundos (ADR-024 §2.4) — a "engine
// cognitiva" que EXPERIMENTA mundos (não só gera): AUTHOR -> SIMULATE -> OBSERVE
// -> EVALUATE -> MODIFY -> (loop).
//
// Compõe o que já existe: worldspec (contrato canônico) + evalgo (juiz = medição
// + gate). Determinístico (I1) e fail-closed (I2). O que NÃO é medido, não
// vira "melhora" — sempre métrica + gate.
package worldloop

import (
	"fmt"
	"sort"

	"github.com/CoscaAI/cosca/internal/evalgo"
	"github.com/CoscaAI/cosca/internal/worldspec"
)

// Observation é uma observação QUALIFICADA (I4): o mundo sabe o que sabe.
type Observation struct {
	Key    string  `json:"key"`
	Value  float64 `json:"value"`
	Trust  string  `json:"trust"`  // "known" | "degraded" | "unknown"
	Source string  `json:"source"` // "simulation" | "observation"
}

// Simulator produz observações determinísticas de um mundo.
type Simulator interface {
	Simulate(spec *worldspec.WorldSpec) []Observation
}

// AccessibilitySimulator mede a conectividade (grau) de cada nó de navegação.
// Nó com 0 arestas = isolado (acessibilidade baixa). Determinístico.
type AccessibilitySimulator struct{}

// Simulate implementa Simulator.
func (AccessibilitySimulator) Simulate(spec *worldspec.WorldSpec) []Observation {
	if spec == nil {
		return nil
	}
	degree := map[string]int{}
	for _, e := range spec.Navigation.Edges {
		degree[e.From]++
		degree[e.To]++
	}
	var out []Observation
	names := make([]string, 0, len(spec.Navigation.Nodes))
	for _, n := range spec.Navigation.Nodes {
		names = append(names, n.ID)
	}
	sort.Strings(names)
	for _, id := range names {
		d := degree[id]
		val := 1.0
		if d == 0 {
			val = 0.15 // isolado
		}
		out = append(out, Observation{Key: "accessibility:" + id, Value: val, Trust: "known", Source: "simulation"})
	}
	return out
}

// toEvalgoResults converte observações em resultados do evalgo (juiz=medição).
func toEvalgoResults(obs []Observation, mean float64) []evalgo.Result {
	results := make([]evalgo.Result, 0, len(obs))
	for _, o := range obs {
		pass := o.Value >= mean*0.7
		sig := "ok"
		if !pass {
			sig = "isolated:" + o.Key[len("accessibility:"):]
		}
		results = append(results, evalgo.Result{CaseID: o.Key, Kind: "accessibility", Pass: pass, Signature: sig})
	}
	return results
}

// meanAccessibility calcula a média de acessibilidade (para o threshold ~70%).
func meanAccessibility(obs []Observation) float64 {
	if len(obs) == 0 {
		return 0
	}
	sum := 0.0
	for _, o := range obs {
		sum += o.Value
	}
	return sum / float64(len(obs))
}

// Apply é a mutação estrutural do mundo (MODIFY): aplica uma proposta.
type Apply interface {
	Apply(spec *worldspec.WorldSpec) (string, error)
}

// ConnectIsolated é um reparo determinístico: liga o nó mais isolado ao mais
// conectado. Muda a INTENÇÃO ESTRUTURAL (edge no grafo), não vértice a vértice.
type ConnectIsolated struct{}

// Apply implementa Apply.
func (ConnectIsolated) Apply(spec *worldspec.WorldSpec) (string, error) {
	if spec == nil || len(spec.Navigation.Nodes) == 0 {
		return "", fmt.Errorf("worldloop: mundo sem nós (fail-closed I2)")
	}
	edges := map[string]int{}
	for _, e := range spec.Navigation.Edges {
		edges[e.From]++
		edges[e.To]++
	}
	// nó mais conectado = hub.
	hub := spec.Navigation.Nodes[0].ID
	hubDeg := -1
	// nó mais isolado.
	iso := ""
	isoDeg := -1
	for _, n := range spec.Navigation.Nodes {
		d := edges[n.ID]
		if d == 0 && isoDeg < 0 {
			iso = n.ID
			isoDeg = d
		}
		if d > hubDeg {
			hubDeg = d
			hub = n.ID
		}
	}
	if iso == "" || iso == hub {
		return "", fmt.Errorf("worldloop: nenhum nó a corrigir (fail-closed)")
	}
	spec.Navigation.Edges = append(spec.Navigation.Edges, worldspec.NavEdge{From: iso, To: hub, Type: "road", Cost: 1.0})
	return fmt.Sprintf("road %s -> %s", iso, hub), nil
}

// Report é o resultado do loop.
type Report struct {
	Iterations int
	Final      worldspec.WorldSpec
	Clusters   []evalgo.Cluster
	Verdict    evalgo.GateDecision
}

// Run executa o loop até passar no gate ou esgotar as iterações.
func Run(spec worldspec.WorldSpec, sim Simulator, apply Apply, gate *evalgo.Gate, maxIter int) (Report, error) {
	cur := spec
	for i := 1; i <= maxIter; i++ {
		obs := sim.Simulate(&cur)
		mean := meanAccessibility(obs)
		results := toEvalgoResults(obs, mean)
		clusters := evalgo.Clusters(results)
		verdict := gate.Decide(results)

		if verdict.Promoted || len(clusters) == 0 {
			return Report{Iterations: i, Final: cur, Clusters: clusters, Verdict: verdict}, nil
		}
		if apply == nil {
			return Report{Iterations: i, Final: cur, Clusters: clusters, Verdict: verdict}, nil
		}
		// MODIFY: aplica a proposta (fail-closed: se rejeitar, para).
		if _, err := apply.Apply(&cur); err != nil {
			return Report{Iterations: i, Final: cur, Clusters: clusters, Verdict: verdict}, err
		}
	}
	return Report{Iterations: maxIter, Final: cur}, nil
}

// DefaultGate devolve o gate do loop (promove quando não há isolamento; bloqueia
// em assinatura "isolated:*" — fail-closed I2).
func DefaultGate() *evalgo.Gate {
	return evalgo.NewGate(
		evalgo.WithMinPassRate(1.0),
		evalgo.WithMinCases(1),
		evalgo.WithBlockingSignatures("isolated:"),
	)
}
