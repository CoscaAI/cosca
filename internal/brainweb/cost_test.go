package brainweb

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/cost"
)

// TestClassifyEnergy valida a regra do professor: energia é INTENSIDADE
// operacional (não nota). Cobre os estados: neutro (sem dados), produtivo
// (🔥 alta conversão), congestionado (🌀 burn alto, pouco trabalho útil),
// unmeasured (Fase 0: consumo mas valor não decomposto) e idle (atividade modesta).
func TestClassifyEnergy(t *testing.T) {
	cases := []struct {
		name       string
		tokens     int
		usefulWork float64
		wantEnergy string
	}{
		{name: "sem dados → neutro", tokens: 0, usefulWork: 0, wantEnergy: EnergyNeutral},
		{name: "produtivo (alta conversão)", tokens: 150000, usefulWork: 300000, wantEnergy: EnergyProductive},
		{name: "congestionado (burn alto, pouco trabalho)", tokens: 150000, usefulWork: 100, wantEnergy: EnergyCongested},
		{name: "congestionado com burn moderado", tokens: 40000, usefulWork: 20, wantEnergy: EnergyCongested},
		{name: "unmeasured (valor não decomposto)", tokens: 40000, usefulWork: 0, wantEnergy: EnergyUnmeasured},
		{name: "idle (atividade modesta)", tokens: 5000, usefulWork: 10, wantEnergy: EnergyIdle},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			eff, energy := classifyEnergy(tc.tokens, tc.usefulWork)
			if energy != tc.wantEnergy {
				t.Fatalf("energy=%q, esperado %q", energy, tc.wantEnergy)
			}
			if tc.tokens > 0 {
				wantEff := tc.usefulWork / float64(tc.tokens)
				if eff != wantEff {
					t.Fatalf("efficiency=%v, esperado %v", eff, wantEff)
				}
			} else if eff != 0 {
				t.Fatalf("sem tokens: efficiency=%v, esperado 0", eff)
			}
		})
	}
}

// TestBuilder_CostProjection garante que a projeção de custo preenche os nós
// a partir do cost.Store real (tokens + vetor de valor), distinguindo um agente
// produtivo de um congestionado — e preservando efficiency e useful_work (não um
// único escalar), como manda a regra do professor.
func TestBuilder_CostProjection(t *testing.T) {
	dir := t.TempDir()
	store := cost.NewStore(dir)

	// Backend Chief: produtivo — muito trabalho útil por token convertido.
	mustAppend(t, store, cost.Record{AgentID: "Backend Chief", TokensTotal: 100000, KnowledgeGain: 120000, At: time.Now()})
	mustAppend(t, store, cost.Record{AgentID: "Backend Chief", TokensTotal: 50000, TaskProgress: 50000, At: time.Now()})
	// Security Chief: congestionado — gastou 150k tokens e rendeu 100 de valor útil.
	mustAppend(t, store, cost.Record{AgentID: "Security Chief", TokensTotal: 150000, KnowledgeGain: 100, At: time.Now()})
	// Don: sem telemetria → neutro (honesto, não inventa).

	b := NewBuilder(testAgents(), testSkills(), "1.5.0").WithCost(store)
	g := b.Build()

	var backend, security, don *NodeCost
	for i := range g.Nodes {
		n := &g.Nodes[i]
		switch n.ID {
		case "backend chief":
			backend = &n.Cost
		case "security chief":
			security = &n.Cost
		case "don":
			don = &n.Cost
		}
	}

	if backend == nil || !backend.HasData {
		t.Fatalf("backend chief sem projeção de custo: %+v", backend)
	}
	if backend.TokenUsage != 150000 {
		t.Fatalf("backend token_usage=%d, esperado 150000", backend.TokenUsage)
	}
	if backend.Runs != 2 {
		t.Fatalf("backend runs=%d, esperado 2", backend.Runs)
	}
	if backend.UsefulWork != 170000 {
		t.Fatalf("backend useful_work=%v, esperado 170000", backend.UsefulWork)
	}
	if backend.Energy != EnergyProductive {
		t.Fatalf("backend energy=%q, esperado %q (produtivo)", backend.Energy, EnergyProductive)
	}
	// Preserva a distinção: efficiency e useful_work expostos (não só um escalar).
	if backend.Efficiency <= 0 || backend.UsefulWork <= 0 {
		t.Fatalf("backend deve expor efficiency e useful_work positivos: %+v", backend)
	}

	if security == nil || !security.HasData {
		t.Fatalf("security chief sem projeção de custo: %+v", security)
	}
	if security.Energy != EnergyCongested {
		t.Fatalf("security energy=%q, esperado %q (congestionado)", security.Energy, EnergyCongested)
	}

	if don == nil || don.HasData || don.Energy != EnergyNeutral {
		t.Fatalf("don sem telemetria deve ser neutro: %+v", don)
	}
}

// TestBuilder_CostSemStore garante que, sem store, todos os nós são neutros
// (nil-safe — nunca pânico).
func TestBuilder_CostSemStore(t *testing.T) {
	b := NewBuilder(testAgents(), testSkills(), "1.5.0")
	g := b.Build()
	for _, n := range g.Nodes {
		if n.Cost.HasData || n.Cost.Energy != EnergyNeutral {
			t.Fatalf("nó %q sem store deve ser neutro: %+v", n.ID, n.Cost)
		}
	}
}

// TestBuilder_CostNilSafety garante que WithCost(nil) não causa pânico.
func TestBuilder_CostNilSafety(t *testing.T) {
	b := NewBuilder(testAgents(), testSkills(), "1.5.0").WithCost(nil)
	g := b.Build()
	for _, n := range g.Nodes {
		if n.Cost.Energy != EnergyNeutral {
			t.Fatalf("nó %q com store nil deve ser neutro: %+v", n.ID, n.Cost)
		}
	}
}

// TestBuilder_CostArmazenaInexistente garante que um store apontando para um
// diretório inexistente não causa pânico e projeta neutro (Load é best-effort).
func TestBuilder_CostArmazenaInexistente(t *testing.T) {
	store := cost.NewStore(filepath.Join(t.TempDir(), "cost"))
	b := NewBuilder(testAgents(), testSkills(), "1.5.0").WithCost(store)
	g := b.Build()
	for _, n := range g.Nodes {
		if n.Cost.HasData {
			t.Fatalf("nó %q sem registros deve ter has_data=false: %+v", n.ID, n.Cost)
		}
	}
}

func mustAppend(t *testing.T, store *cost.Store, r cost.Record) {
	t.Helper()
	if err := store.Append(r); err != nil {
		t.Fatalf("append: %v", err)
	}
}
