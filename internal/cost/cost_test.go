// Package cost — testes da Fase 0 do ADR-031 (Useful Work / Tokens).
//
// Cobre: (a) a invariância do professor (TokensTotal == Input+Output), (b) a
// métrica Useful Work / Tokens, (c) a agregação por (agent_id, task_id), (d)
// compatibilidade (campos novos zero-default não quebram marshaling antigo).
package cost

import (
	"encoding/json"
	"testing"
)

func TestRecord_Validate_Consistent(t *testing.T) {
	r := Record{InputTokens: 80, OutputTokens: 20, TokensTotal: 100}
	if got := r.Validate(); got != "" {
		t.Fatalf("Validate() = %q, want '' (consistente)", got)
	}
}

func TestRecord_Validate_Inconsistent(t *testing.T) {
	// Invariância do professor: se Input+Output estiverem preenchidos, o
	// TokensTotal DEVE bater; senão a métrica de eficiência fica errada.
	r := Record{InputTokens: 80, OutputTokens: 20, TokensTotal: 150}
	if got := r.Validate(); got == "" {
		t.Fatal("Validate() = '', want não-vazio (inconsistente)")
	}
}

func TestRecord_Validate_Empty(t *testing.T) {
	// Nada preenchido — sem o que validar.
	r := Record{}
	if got := r.Validate(); got != "" {
		t.Fatalf("Validate() = %q, want '' (vazio)", got)
	}
}

func TestRecord_Efficiency(t *testing.T) {
	cases := []struct {
		name string
		r    Record
		want float64
	}{
		{"zero tokens", Record{TokensTotal: 0, TaskProgress: 1}, 0},
		{"trabalho util", Record{TokensTotal: 100, KnowledgeGain: 0.5, TaskProgress: 0.5}, 0.01},
		{"so tokens sem valor", Record{TokensTotal: 150, ArtifactValue: 0}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.r.Efficiency(); got != tc.want {
				t.Fatalf("Efficiency() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRecord_SerializeDeserialize(t *testing.T) {
	// Compatibilidade: campos novos com omitempty não devem quebrar o round-trip.
	orig := Record{
		AgentID: "cosca-backend", TaskID: "T-1", Model: "gpt-4o",
		InputTokens: 80, OutputTokens: 20, TokensTotal: 100,
		KnowledgeGain: 0.3, TaskProgress: 0.7, ArtifactValue: 1,
	}
	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Record
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.TokensTotal != orig.TokensTotal || got.KnowledgeGain != orig.KnowledgeGain {
		t.Fatalf("round-trip perdeu campos: %+v", got)
	}
}

func TestAggregate_GroupByAgentTask(t *testing.T) {
	recs := []Record{
		{AgentID: "a", TaskID: "t1", InputTokens: 50, OutputTokens: 50, TokensTotal: 100, TaskProgress: 1},
		{AgentID: "a", TaskID: "t1", InputTokens: 0, OutputTokens: 50, TokensTotal: 50, TaskProgress: 0.5},
		{AgentID: "b", TaskID: "t2", InputTokens: 700, OutputTokens: 50, TokensTotal: 750, KnowledgeGain: 0.1},
	}
	rep := Aggregate(recs)
	if len(rep.Grouped) != 2 {
		t.Fatalf("esperava 2 grupos, got %d", len(rep.Grouped))
	}
	// Ordenado por tokens desc: grupo b (750) primeiro.
	if rep.Grouped[0].AgentID != "b" || rep.Grouped[0].TokensTotal != 750 {
		t.Fatalf("ordem errada: %+v", rep.Grouped[0])
	}
	// Agregação soma dentro do grupo.
	if rep.Grouped[1].AgentID != "a" || rep.Grouped[1].TokensTotal != 150 {
		t.Fatalf("agregacao do grupo a errada: %+v", rep.Grouped[1])
	}
	if rep.Total.TokensTotal != 900 {
		t.Fatalf("total errado: %d, want 900", rep.Total.TokensTotal)
	}
}

func TestStore_AppendLoad(t *testing.T) {
	dir := t.TempDir()
	st := NewStore(dir)
	if err := st.Append(Record{AgentID: "x", TaskID: "t", TokensTotal: 10}); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := st.Append(Record{AgentID: "x", TaskID: "t", TokensTotal: 20}); err != nil {
		t.Fatalf("append2: %v", err)
	}
	recs, err := st.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("esperava 2 records, got %d", len(recs))
	}
}
