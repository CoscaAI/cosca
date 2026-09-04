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

func TestRecord_ApplyValue_DecoupledRealValue(t *testing.T) {
	// Execução com valor mensurável REAL (build passou, testes passaram,
	// memória persistida) → o vetor decomposto NÃO pode ser só knowledge_gain:
	// artifact/evidence/progress também sobem. UsefulWork > 0 e Efficiency > 0.
	r := Record{TokensTotal: 100}
	r.ApplyValue(ValueEvidence{
		BuildOK:     true,
		TestsRun:    true,
		TestsPassed: 5,
		MemStored:   true,
	})
	if r.ArtifactValue != 1 {
		t.Fatalf("ArtifactValue = %d, want 1 (build OK = artefato real)", r.ArtifactValue)
	}
	if r.EvidenceGain != 5 {
		t.Fatalf("EvidenceGain = %d, want 5 (5 testes aprovados)", r.EvidenceGain)
	}
	if r.KnowledgeGain != 1 {
		t.Fatalf("KnowledgeGain = %v, want 1 (memória persistida)", r.KnowledgeGain)
	}
	if r.TaskProgress != 1 {
		t.Fatalf("TaskProgress = %v, want 1 (build/test verificados)", r.TaskProgress)
	}
	if r.UsefulWork() <= 0 {
		t.Fatalf("UsefulWork() = %v, want > 0", r.UsefulWork())
	}
	if r.Efficiency() <= 0 {
		t.Fatalf("Efficiency() = %v, want > 0 (valor real / tokens)", r.Efficiency())
	}
}

func TestRecord_ApplyValue_HonestZero(t *testing.T) {
	// Execução SEM valor mensurável (build falhou, 0 testes aprovados, sem
	// memória) → vetor honestamente zero. Não inflamos com opinião do LLM.
	r := Record{TokensTotal: 120}
	r.ApplyValue(ValueEvidence{
		BuildOK:     false,
		TestsRun:    true,
		TestsPassed: 0,
		MemStored:   false,
	})
	if got := r.UsefulWork(); got != 0 {
		t.Fatalf("UsefulWork() = %v, want 0 (honesto — nenhuma evidência de valor)", got)
	}
	if r.Efficiency() != 0 {
		t.Fatalf("Efficiency() = %v, want 0", r.Efficiency())
	}
}

func TestRecord_ApplyValue_EmptyEvidence(t *testing.T) {
	// Sem evidência alguma (execução sem build/test/memória) — continua 0.
	r := Record{TokensTotal: 50}
	r.ApplyValue(ValueEvidence{})
	if got := r.UsefulWork(); got != 0 {
		t.Fatalf("UsefulWork() = %v, want 0", got)
	}
}

// TestRecord_BudgetTokens_ExcludesCacheRead — nuance de orçamento (ADR-031 /
// prime autonomous.ts:186-194): o orçamento de um loop longo soma input+output+
// cacheWrite e EXCLUI cacheRead (o recontexto servido do cache). Não altera
// TokensTotal (a agregação existente permanece intacta).
func TestRecord_BudgetTokens_ExcludesCacheRead(t *testing.T) {
	r := Record{
		InputTokens:      900, // input "fresco" (não-cacheadado)
		OutputTokens:     100,
		CacheReadTokens:  900, // recontexto servido do cache — NÃO conta pro orçamento
		CacheWriteTokens: 50,  // escrita de cache — CONTA pro orçamento
		TokensTotal:      1000,
	}
	if got := r.BudgetTokens(); got != 1050 {
		t.Fatalf("BudgetTokens() = %d, want 1050 (900+100+50; cacheRead=900 EXCLUÍDO)", got)
	}
	// TokensTotal segue a regra antiga (input+output) — a agregação existente
	// permanece intacta e o Validate continua limpo.
	if r.TokensTotal != 1000 {
		t.Fatalf("TokensTotal = %d, want 1000 (invariante ADR-031 preservada)", r.TokensTotal)
	}
	if r.TokensTotal != r.InputTokens+r.OutputTokens {
		t.Fatalf("Validate() deveria estar limpo: %q", r.Validate())
	}
	// BudgetTokens (1050) > TokensTotal (1000) é CORRETO: o cacheWrite que ENTRA
	// no orçamento não está somado no TokensTotal antigo — a nuance é uma leitura
	// PARALELA para o budget de autonomia, não um override.
	if r.BudgetTokens() <= r.TokensTotal {
		t.Fatalf("BudgetTokens()=%d deveria exceder TokensTotal=%d (cacheWrite entra no orçamento)", r.BudgetTokens(), r.TokensTotal)
	}
}

// TestRecord_BudgetTokens_LongLoopDoesNotExhaust — um loop longo com MUITO
// cacheRead NÃO esgota o orçamento: BudgetTokens ignora cacheRead, então o total
// de orçamento fica bem abaixo do que seria contado naive (input+output+cacheRead).
func TestRecord_BudgetTokens_LongLoopDoesNotExhaust(t *testing.T) {
	const turns = 12
	var budget, naive int
	for i := 0; i < turns; i++ {
		r := Record{InputTokens: 900, OutputTokens: 100, CacheReadTokens: 900, CacheWriteTokens: 0}
		budget += r.BudgetTokens()      // 1000/turn
		naive += r.InputTokens + r.OutputTokens + r.CacheReadTokens // 1900/turn
	}
	if budget != 12000 {
		t.Fatalf("budget = %d, want 12000 (cacheRead excluído)", budget)
	}
	if naive != 22800 {
		t.Fatalf("naive = %d, want 22800 (demonstra por que a nuance importa)", naive)
	}
	// O orçamento de autonomia deve ser o budget (cacheRead excluído), não o naive.
	const budgetCap = 15000
	if budget > budgetCap {
		t.Fatalf("budget %d excedeu o teto %d — a nuance não funcionou", budget, budgetCap)
	}
	if naive <= budgetCap {
		t.Fatalf("o cenário não é válido: naive=%d deveria exceder %d", naive, budgetCap)
	}
}

// TestExecutionRecord_BudgetTokens — a ExecutionRecord (nível de execução)
// também carrega a nuance de orçamento via conversão RecordToExecution — e o
// BudgetTokens da execution não conta cacheRead.
func TestExecutionRecord_BudgetTokens(t *testing.T) {
	orig := Record{
		TaskID: "abc", InputTokens: 700, OutputTokens: 200, TokensTotal: 900,
		CacheReadTokens: 600, CacheWriteTokens: 100,
	}
	e := recordToExecution(orig)
	if e.CacheReadTokens != 600 || e.CacheWriteTokens != 100 {
		t.Fatalf("conversão perdeu a nuance de cache: read=%d write=%d", e.CacheReadTokens, e.CacheWriteTokens)
	}
	if got := e.InputTokens + e.OutputTokens + e.CacheWriteTokens; got != 1000 {
		t.Fatalf("budget da execution = %d, want 1000 (cacheRead excluído)", got)
	}
}
