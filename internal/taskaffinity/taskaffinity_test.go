package taskaffinity

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/CoscaAI/cosca/internal/search"
)

// ─── F1: DeriveProfile ───────────────────────────────────────────────────────

// TestDeriveProfile_CompleteState valida a derivação com todos os sinais
// (DESIGN-001 §6.1): todos os campos preenchidos, Confidence ≥ 0.8.
func TestDeriveProfile_CompleteState(t *testing.T) {
	state := &ImplementaçãoState{
		Prompt:      "implementar endpoint REST em Go para o módulo de autenticação",
		WorkingDir:  "C:/Users/Henrique/Documents/cosca",
		OpenFiles:   []string{"internal/api/handlers.go", "internal/api/middleware.go"},
		RecentFiles: []string{"internal/api/handlers.go"},
		GoModExists: true,
	}

	profile := DeriveProfile(state)
	if profile == nil {
		t.Fatal("DeriveProfile returned nil for a complete state")
	}
	if profile.Task != state.Prompt {
		t.Errorf("Task = %q, want %q", profile.Task, state.Prompt)
	}
	if !profile.HasStack() || !contains(profile.Stack, "go") {
		t.Errorf("Stack = %v, want to contain \"go\"", profile.Stack)
	}
	if profile.Target != "internal/api/handlers.go" {
		t.Errorf("Target = %q, want %q", profile.Target, "internal/api/handlers.go")
	}
	if profile.TargetModule != "api" {
		t.Errorf("TargetModule = %q, want %q", profile.TargetModule, "api")
	}
	if !profile.HasAffinity() {
		t.Error("Affinity is empty, want non-empty")
	}
	if profile.Confidence < 0.8 {
		t.Errorf("Confidence = %v, want >= 0.8", profile.Confidence)
	}
}

// TestDeriveProfile_EmptyState valida a derivação sem sinais (DESIGN-001 §6.1):
// TaskProfile nil (estado nil) ou Confidence = 0 (estado vazio).
func TestDeriveProfile_EmptyState(t *testing.T) {
	// Nil-safe: estado nil → perfil nil.
	if p := DeriveProfile(nil); p != nil {
		t.Errorf("DeriveProfile(nil) = %+v, want nil", p)
	}

	// Estado vazio (não-nil) → perfil com Confidence = 0 e campos vazios.
	profile := DeriveProfile(&ImplementaçãoState{})
	if profile == nil {
		t.Fatal("DeriveProfile(&ImplementaçãoState{}) returned nil")
	}
	if profile.Task != "" {
		t.Errorf("Task = %q, want empty", profile.Task)
	}
	if profile.HasStack() {
		t.Errorf("Stack = %v, want empty", profile.Stack)
	}
	if profile.Target != "" || profile.TargetModule != "" {
		t.Errorf("Target = %q, TargetModule = %q, want empty", profile.Target, profile.TargetModule)
	}
	if profile.HasAffinity() {
		t.Errorf("Affinity = %v, want empty", profile.Affinity)
	}
	if profile.Confidence != 0 {
		t.Errorf("Confidence = %v, want 0", profile.Confidence)
	}
}

// TestDeriveProfile_StackDetection valida a detecção de stack por
// go.mod/package.json/extensões (DESIGN-001 §6.1): stack correta para cada sinal.
func TestDeriveProfile_StackDetection(t *testing.T) {
	tests := []struct {
		name  string
		state *ImplementaçãoState
		want  []string
	}{
		{
			name:  "go.mod",
			state: &ImplementaçãoState{GoModExists: true},
			want:  []string{"go"},
		},
		{
			name:  "package.json sem tsconfig",
			state: &ImplementaçãoState{PackageJSONExists: true},
			want:  []string{"node"},
		},
		{
			name:  "arquivo .go",
			state: &ImplementaçãoState{OpenFiles: []string{"internal/api/handlers.go"}},
			want:  []string{"go"},
		},
		{
			name:  "arquivo .ts",
			state: &ImplementaçãoState{OpenFiles: []string{"web/src/app.ts"}},
			want:  []string{"typescript"},
		},
		{
			name:  "arquivo .py",
			state: &ImplementaçãoState{RecentFiles: []string{"scripts/train.py"}},
			want:  []string{"python"},
		},
		{
			name:  "arquivo .rs",
			state: &ImplementaçãoState{RecentFiles: []string{"src/main.rs"}},
			want:  []string{"rust"},
		},
		{
			name:  "path internal/ (heurística Cosca)",
			state: &ImplementaçãoState{OpenFiles: []string{"internal/search/search.go"}},
			want:  []string{"go"},
		},
		{
			name:  "sem sinais",
			state: &ImplementaçãoState{},
			want:  []string{},
		},
		{
			name:  "framework chi + sqlite",
			state: &ImplementaçãoState{OpenFiles: []string{"internal/chi/router.go", "internal/sqlite/store.go"}},
			want:  []string{"chi", "go", "sqlite"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := DeriveProfile(tt.state)
			if profile == nil {
				t.Fatal("DeriveProfile returned nil")
			}
			if !reflect.DeepEqual(profile.Stack, tt.want) {
				t.Errorf("Stack = %v, want %v", profile.Stack, tt.want)
			}
		})
	}

	// package.json + tsconfig.json no WorkingDir → ["node", "typescript"].
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	profile := DeriveProfile(&ImplementaçãoState{WorkingDir: dir, PackageJSONExists: true})
	want := []string{"node", "typescript"}
	if !reflect.DeepEqual(profile.Stack, want) {
		t.Errorf("Stack com tsconfig = %v, want %v", profile.Stack, want)
	}
}

// TestDeriveProfile_TargetDetection valida a detecção de alvo por OpenFiles
// (DESIGN-001 §6.1): Target e TargetModule corretos.
func TestDeriveProfile_TargetDetection(t *testing.T) {
	// TargetHint tem prioridade máxima.
	profile := DeriveProfile(&ImplementaçãoState{
		TargetHint: "internal/ranking/ranking.go",
		OpenFiles:  []string{"internal/api/handlers.go"},
	})
	if profile.Target != "internal/ranking/ranking.go" {
		t.Errorf("Target = %q, want TargetHint", profile.Target)
	}
	if profile.TargetModule != "ranking" {
		t.Errorf("TargetModule = %q, want %q", profile.TargetModule, "ranking")
	}

	// OpenFiles: primeiro arquivo (o mais recentemente aberto).
	profile = DeriveProfile(&ImplementaçãoState{
		OpenFiles:   []string{"internal/api/handlers.go", "internal/api/middleware.go"},
		RecentFiles: []string{"internal/search/search.go"},
	})
	if profile.Target != "internal/api/handlers.go" {
		t.Errorf("Target = %q, want primeiro OpenFile", profile.Target)
	}
	if profile.TargetModule != "api" {
		t.Errorf("TargetModule = %q, want %q", profile.TargetModule, "api")
	}

	// RecentFiles: primeiro arquivo (o mais recentemente modificado).
	profile = DeriveProfile(&ImplementaçãoState{
		RecentFiles: []string{"internal/contextpipeline/contextpipeline.go"},
	})
	if profile.Target != "internal/contextpipeline/contextpipeline.go" {
		t.Errorf("Target = %q, want primeiro RecentFile", profile.Target)
	}
	if profile.TargetModule != "contextpipeline" {
		t.Errorf("TargetModule = %q, want %q", profile.TargetModule, "contextpipeline")
	}

	// Sem sinais → Target vazio.
	profile = DeriveProfile(&ImplementaçãoState{})
	if profile.Target != "" || profile.TargetModule != "" {
		t.Errorf("Target = %q, TargetModule = %q, want empty", profile.Target, profile.TargetModule)
	}

	// Path sem internal/ → penúltimo segmento não-vazio.
	profile = DeriveProfile(&ImplementaçãoState{OpenFiles: []string{"api/handlers.go"}})
	if profile.TargetModule != "api" {
		t.Errorf("TargetModule = %q, want %q (penúltimo segmento)", profile.TargetModule, "api")
	}
}

// TestDeriveProfile_AffinityGeneration valida a geração de termos de afinidade
// (DESIGN-001 §6.1): termos incluem taskTokens + stack + moduleExpansion.
func TestDeriveProfile_AffinityGeneration(t *testing.T) {
	state := &ImplementaçãoState{
		Prompt:      "implementar endpoint REST em Go para o módulo de autenticação",
		OpenFiles:   []string{"internal/api/handlers.go"},
		GoModExists: true,
	}
	profile := DeriveProfile(state)
	if profile == nil {
		t.Fatal("DeriveProfile returned nil")
	}

	// taskTokens (DESIGN-001 §2.4.1): ["autenticação", "endpoint", "go",
	// "implementar", "módulo", "rest"].
	for _, term := range []string{"autenticação", "endpoint", "go", "implementar", "módulo", "rest"} {
		if !contains(profile.Affinity, term) {
			t.Errorf("Affinity = %v, missing taskToken %q", profile.Affinity, term)
		}
	}
	// stack: "go" (já coberto acima).
	// moduleExpansion (api): "api", "handler", "route".
	for _, term := range []string{"api", "handler", "route"} {
		if !contains(profile.Affinity, term) {
			t.Errorf("Affinity = %v, missing moduleExpansion %q", profile.Affinity, term)
		}
	}

	// Deduplicado e ordenado alfabeticamente.
	if !sort.StringsAreSorted(profile.Affinity) {
		t.Errorf("Affinity = %v, want sorted", profile.Affinity)
	}
	seen := make(map[string]bool, len(profile.Affinity))
	for _, a := range profile.Affinity {
		if seen[a] {
			t.Errorf("Affinity = %v, has duplicate %q", profile.Affinity, a)
		}
		seen[a] = true
	}
}

// TestDeriveProfile_Determinism valida que a mesma entrada produz o mesmo
// perfil (DESIGN-001 §6.1): Profile1 == Profile2 (deep equal).
func TestDeriveProfile_Determinism(t *testing.T) {
	state := &ImplementaçãoState{
		Prompt:      "implementar endpoint REST em Go para o módulo de autenticação",
		WorkingDir:  "C:/Users/Henrique/Documents/cosca",
		OpenFiles:   []string{"internal/api/handlers.go", "internal/api/middleware.go"},
		RecentFiles: []string{"internal/api/handlers.go"},
		GoModExists: true,
	}

	p1 := DeriveProfile(state)
	p2 := DeriveProfile(state)
	if !reflect.DeepEqual(p1, p2) {
		t.Errorf("DeriveProfile não é determinístico:\np1 = %+v\np2 = %+v", p1, p2)
	}
}

// ─── F2: AffinityRerank ──────────────────────────────────────────────────────

// affinityTestProfile é o perfil usado nos testes de re-ponderação: afinidade
// do módulo "api" (taskTokens + stack + moduleExpansion), confiança total.
var affinityTestProfile = &TaskProfile{
	Affinity: []string{
		"api", "autenticação", "endpoint", "go",
		"handler", "implementar", "módulo", "rest", "route",
	},
	Confidence: 1.0,
}

// affinityTestResults: dois resultados genéricos (sem sobreposição de
// afinidade) na frente e um resultado do projeto (sobreposição total) atrás.
var affinityTestResults = []search.SearchResult{
	{ID: "generic-a", Title: "Introduction", Content: "welcome to the documentation", DocumentPath: "docs/intro.md", Score: 0.85, Rank: 1},
	{ID: "generic-b", Title: "Search overview", Content: "generic documentation about search", DocumentPath: "docs/search.md", Score: 0.80, Rank: 2},
	{ID: "project-c", Title: "handlers", Content: "api autenticação endpoint go handler implementar módulo rest route", DocumentPath: "internal/api/handlers.go", Score: 0.72, Rank: 3},
}

// TestAffinityRerank_BoostProjectResults valida que resultados do projeto
// sobem (DESIGN-001 §6.1): resultado com alta sobreposição sobe ≥ 2 posições.
func TestAffinityRerank_BoostProjectResults(t *testing.T) {
	originalPos := resultIndex(affinityTestResults, "project-c")
	if originalPos != 2 {
		t.Fatalf("setup: project-c na posição %d, want 2", originalPos)
	}

	reranked := AffinityRerank(affinityTestResults, affinityTestProfile)
	newPos := resultIndex(reranked, "project-c")

	if originalPos-newPos < 2 {
		t.Errorf("resultado do projeto subiu %d posições (de %d para %d), want >= 2", originalPos-newPos, originalPos, newPos)
	}
	if reranked[0].ID != "project-c" {
		t.Errorf("primeiro resultado = %q, want %q", reranked[0].ID, "project-c")
	}
}

// TestAffinityRerank_DampGenericResults valida que resultados genéricos descem
// (DESIGN-001 §6.1): resultado genérico com baixa sobreposição desce ≥ 1 posição.
func TestAffinityRerank_DampGenericResults(t *testing.T) {
	originalPos := resultIndex(affinityTestResults, "generic-b")
	if originalPos != 1 {
		t.Fatalf("setup: generic-b na posição %d, want 1", originalPos)
	}

	reranked := AffinityRerank(affinityTestResults, affinityTestProfile)
	newPos := resultIndex(reranked, "generic-b")

	if newPos-originalPos < 1 {
		t.Errorf("resultado genérico desceu %d posições (de %d para %d), want >= 1", newPos-originalPos, originalPos, newPos)
	}
}

// TestAffinityRerank_NilSafe valida que profile nil não quebra (DESIGN-001
// §6.1): resultados inalterados quando profile é nil.
func TestAffinityRerank_NilSafe(t *testing.T) {
	results := []search.SearchResult{
		{ID: "a", Title: "alpha", Content: "content a", Score: 0.9, Rank: 1},
		{ID: "b", Title: "beta", Content: "content b", Score: 0.8, Rank: 2},
	}

	got := AffinityRerank(results, nil)
	if !reflect.DeepEqual(got, results) {
		t.Errorf("AffinityRerank(results, nil) alterou os resultados:\ngot  = %+v\nwant = %+v", got, results)
	}
}

// TestAffinityRerank_ZeroAffinity valida que Affinity vazia não altera
// (DESIGN-001 §6.1): resultados inalterados quando Affinity é vazio.
func TestAffinityRerank_ZeroAffinity(t *testing.T) {
	results := []search.SearchResult{
		{ID: "a", Title: "alpha", Content: "content a", Score: 0.9, Rank: 1},
		{ID: "b", Title: "beta", Content: "content b", Score: 0.8, Rank: 2},
	}
	profile := &TaskProfile{Affinity: []string{}, Confidence: 1.0}

	got := AffinityRerank(results, profile)
	if !reflect.DeepEqual(got, results) {
		t.Errorf("AffinityRerank com Affinity vazio alterou os resultados:\ngot  = %+v\nwant = %+v", got, results)
	}
}

// TestAffinityRerank_MaxBoostBound valida que o boost nunca excede 0.15
// (DESIGN-001 §6.1): para qualquer entrada, boost ≤ 0.15.
func TestAffinityRerank_MaxBoostBound(t *testing.T) {
	profile := &TaskProfile{
		Affinity:   []string{"api", "endpoint", "go", "handler", "rest", "route"},
		Confidence: 1.0,
	}

	// Resultado com TODOS os termos de afinidade → cobertura 1.0 → boost = teto.
	full := search.SearchResult{
		Title:        "api endpoint go handler rest route",
		Content:      "api endpoint go handler rest route",
		DocumentPath: "internal/api/handlers.go",
	}
	boost := computeAffinityBoost(full, profile)
	if math.Abs(boost-maxAffinityBoost) > 1e-9 {
		t.Errorf("boost = %v, want teto %v", boost, maxAffinityBoost)
	}
	if boost > maxAffinityBoost {
		t.Errorf("boost = %v, excede o teto %v", boost, maxAffinityBoost)
	}

	// Confidence modula o boost para baixo (0.15 × 1.0 × 0.5 = 0.075).
	profile.Confidence = 0.5
	boost = computeAffinityBoost(full, profile)
	if math.Abs(boost-0.075) > 1e-9 {
		t.Errorf("boost com confidence 0.5 = %v, want 0.075", boost)
	}

	// Para qualquer entrada, boost <= teto.
	profile.Confidence = 1.0
	inputs := []search.SearchResult{
		{Title: "api", Content: "api", DocumentPath: "internal/api/handlers.go"},
		{Title: "qualquer coisa", Content: "texto sem afinidade", DocumentPath: "docs/x.md"},
		{Title: "", Content: "", DocumentPath: ""},
	}
	for _, r := range inputs {
		if b := computeAffinityBoost(r, profile); b > maxAffinityBoost {
			t.Errorf("boost = %v para %+v, excede o teto %v", b, r, maxAffinityBoost)
		}
	}
}

// TestAffinityRerank_Determinism valida que a mesma entrada produz a mesma
// ordenação (DESIGN-001 §6.1): ordenação idêntica em duas execuções.
func TestAffinityRerank_Determinism(t *testing.T) {
	r1 := AffinityRerank(affinityTestResults, affinityTestProfile)
	r2 := AffinityRerank(affinityTestResults, affinityTestProfile)

	if !reflect.DeepEqual(r1, r2) {
		t.Errorf("AffinityRerank não é determinístico:\nr1 = %+v\nr2 = %+v", r1, r2)
	}
}

// ─── Helpers de teste ────────────────────────────────────────────────────────

// resultIndex devolve o índice do resultado com o ID dado (-1 se ausente).
func resultIndex(results []search.SearchResult, id string) int {
	for i, r := range results {
		if r.ID == id {
			return i
		}
	}
	return -1
}
