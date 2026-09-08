package taskaffinity

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/search"
	"github.com/CoscaAI/cosca/internal/taskphase"
)

// Benchmarks de performance do Task-Aware Search (ADR-045 §6.4 / F5).
// Critério de aceite: overhead total do TAS < 2ms (derivação + re-ponderação).

// BenchmarkDeriveProfile mede o custo da derivação do perfil de tarefa.
// Critério: < 1ms (CPU puro, sem I/O).
func BenchmarkDeriveProfile(b *testing.B) {
	state := &ImplementaçãoState{
		Prompt:      "implementar endpoint REST em Go para o módulo de autenticação",
		WorkingDir:  "C:/proj",
		OpenFiles:   []string{"internal/api/handlers.go", "internal/api/middleware.go"},
		RecentFiles: []string{"internal/api/handlers.go"},
		GoModExists: true,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := DeriveProfile(state)
		if p == nil {
			b.Fatal("perfil nil")
		}
	}
}

// BenchmarkAffinityRerank mede o custo da re-ponderação por afinidade sobre
// 30 resultados (o teto do Limit de exploração).
// Critério: < 500μs para 30 resultados.
func BenchmarkAffinityRerank(b *testing.B) {
	profile := &TaskProfile{
		Task:       "implementar endpoint REST em Go",
		Stack:      []string{"go"},
		Target:     "internal/api/handlers.go",
		Affinity:   []string{"api", "handler", "rest", "go", "endpoint", "route", "autenticação", "módulo"},
		Confidence: 0.95,
	}
	results := make([]search.SearchResult, 30)
	for i := range results {
		results[i] = search.SearchResult{
			ID:           "r" + string(rune('a'+i)),
			Title:        "internal/api/handlers.go",
			Content:      "handler rest api endpoint route go autenticação módulo",
			DocumentPath: "internal/api/handlers.go",
			Score:        0.5 + float64(i)/100,
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out := AffinityRerank(results, profile)
		if len(out) == 0 {
			b.Fatal("resultados vazios")
		}
	}
}

// BenchmarkDetectPhase mede o custo da detecção de fase (via taskphase).
// Critério: < 100μs.
func BenchmarkDetectPhase(b *testing.B) {
	signals := &taskphase.PhaseSignals{
		HasOpenFiles:           true,
		HasRecentModifications: true,
		OpenFileCount:          2,
		TargetIsSpecific:       true,
		HasImplementationTerms: true,
		HasTestTerms:           false,
		HasExplorationTerms:    false,
		SearchHistoryCount:     1,
		TargetHasTests:         false,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d := taskphase.DetectPhase(signals)
		if d.Phase == 0 {
			b.Fatal("fase inválida")
		}
	}
}

// BenchmarkTAS_Overhead mede o overhead total do TAS: derivação + detecção de
// fase + re-ponderação. Critério de aceite: < 2ms.
func BenchmarkTAS_Overhead(b *testing.B) {
	state := &ImplementaçãoState{
		Prompt:      "implementar endpoint REST em Go para o módulo de autenticação",
		WorkingDir:  "C:/proj",
		OpenFiles:   []string{"internal/api/handlers.go", "internal/api/middleware.go"},
		RecentFiles: []string{"internal/api/handlers.go"},
		GoModExists: true,
	}
	results := make([]search.SearchResult, 20)
	for i := range results {
		results[i] = search.SearchResult{
			ID:           "r" + string(rune('a'+i)),
			Title:        "internal/api/handlers.go",
			Content:      "handler rest api endpoint route go autenticação módulo",
			DocumentPath: "internal/api/handlers.go",
			Score:        0.5 + float64(i)/100,
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		profile := DeriveProfile(state)
		if profile == nil {
			b.Fatal("perfil nil")
		}
		signals := BuildPhaseSignals(state)
		detection := taskphase.DetectPhase(signals)
		if detection.Phase == 0 {
			b.Fatal("fase inválida")
		}
		out := AffinityRerank(results, profile)
		if len(out) == 0 {
			b.Fatal("resultados vazios")
		}
	}
}
