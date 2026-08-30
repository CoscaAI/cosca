// Package cost instrumenta a métrica de TokenEfficiency do ADR-031 (Frente 1):
// o "useful work" decomposto por execução + o relatório `cosca cost`.
//
// A métrica-guia é Useful Work / Tokens, onde "useful work" é DECOMPOSTO em
// dimensões (knowledge_gain, task_progress, artifact_value, evidence_gain,
// decision_gain) — nunca uma métrica única (revisão do professor: só
// knowledge_gain ensinaria "só vale aprender").
//
// A persistência é um arquivo JSONL append-only em `.cosca/cost/records.jsonl`
// (runtime, gitignored): cada linha é um Record de uma execução. O `cosca cost`
// lê esse arquivo, agrega por (agent_id, task_id) e reporta tokens + vetor de
// valor + a métrica Useful Work / Tokens.
//
// FASE 0 — LIMITAÇÃO HONESTA: a decomposição do uso (base/contexto/tools/
// history/cached/delegated) ainda NÃO existe no motor. Os campos do Record
// existem (zero-default) para a saída decomposta, mas hoje só input/output são
// preenchidos de verdade — decompor o uso é a Fase 0.1.
package cost

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// RecordsDir é o subdiretório de runtime (relativo a `.cosca`) que guarda o
// log de custo. Gitignored como os outros derivados.
const RecordsDir = "cost"

// RecordsFile é o arquivo JSONL append-only dentro de RecordsDir.
const RecordsFile = "records.jsonl"

// Record é uma execução registrada — espelha o vetor do ADR-031 (Frente 1).
// Campos novos são opcionais (omitempty/zero-default) para não quebrar nenhuma
// fonte existente; a leitura tolera linhas de versões anteriores.
type Record struct {
	// AgentID — agente que executou (ex.: "cosca-backend").
	AgentID string `json:"agent_id,omitempty"`

	// TaskID — identificador da tarefa/execução (ex.: trace_id ou um id).
	TaskID string `json:"task_id,omitempty"`

	// Model — modelo usado (opcional).
	Model string `json:"model,omitempty"`

	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TokensTotal  int `json:"tokens_total"`

	// Decomposição do uso (Fase 0.1) — hoje NÃO preenchidos pelo motor; os
	// campos existem para a SAÍDA DECOMPOSTA assim que a Fase 0.1 for feita.
	ContextTokens  int `json:"context_tokens,omitempty"`
	ToolTokens     int `json:"tool_tokens,omitempty"`
	HistoryTokens  int `json:"history_tokens,omitempty"`
	SystemTokens   int `json:"system_tokens,omitempty"`
	CachedTokens   int `json:"cached_tokens,omitempty"`
	DelegatedTokens int `json:"delegated_tokens,omitempty"`
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`

	// DurationMs — duração da execução.
	DurationMs int64 `json:"duration_ms,omitempty"`

	// Result — resultado curto da execução (NUNCA a resposta completa).
	Result string `json:"result,omitempty"`

	// Useful Work — DECOMPOSTO em dimensões (não uma métrica única).
	KnowledgeGain float64 `json:"knowledge_gain,omitempty"`
	TaskProgress  float64 `json:"task_progress,omitempty"`
	ArtifactValue int     `json:"artifact_value,omitempty"`
	EvidenceGain  int     `json:"evidence_gain,omitempty"`
	DecisionGain  int     `json:"decision_gain,omitempty"`

	// At — quando a execução foi registrada.
	At time.Time `json:"at,omitempty"`
}

// ValueEvidence é a fonte ÚNICA e determinística de valor da Fase 0 do ADR-031:
// a evidência de execução JÁ verificada pelo pipeline (build/test) e a
// persistência real de memória. NUNCA opinião do LLM — é o "juiz em código"
// (o build/test verifica, o sistema decide).
//
// Honestidade: cada dimensão do vetor de valor fica 0 quando a evidência
// correspondente está ausente/negativa. Uma execução que não buildou, não rodou
// teste e não persistiu memória produz Useful Work = 0 — e isso é honesto.
type ValueEvidence struct {
	// BuildOK — o build verificou e passou (gera artifact_value).
	BuildOK bool
	// TestsRun — uma verificação de teste realmente executou (não só marcou).
	TestsRun bool
	// TestsPassed — número determinístico de testes aprovados (evidence_gain).
	TestsPassed int
	// MemStored — um record de memória foi persistido (knowledge_gain).
	MemStored bool
}

// ApplyValue preenche as dimensões de valor decompostas a partir de UMA única
// fonte de evidência determinística (build/test/memória). Não inventa nem
// infla: sem evidência real, a dimensão permanece 0.
func (r *Record) ApplyValue(ev ValueEvidence) {
	if ev.BuildOK {
		r.ArtifactValue = 1
		r.TaskProgress = 1
	}
	if ev.TestsRun && ev.TestsPassed > 0 {
		r.EvidenceGain = ev.TestsPassed
		r.TaskProgress = 1
	}
	if ev.MemStored {
		r.KnowledgeGain = 1
	}
}

// ApplyToolEvidence preenche as dimensões de valor (artifact/evidence) a partir
// das tools realmente executadas (Caminho A / ADR-031). Ferramentas de escrita
// (write_file/edit) geram artifact_value; comandos de BUILD/TEST bem-sucedidos
// geram evidence_gain. É determinístico — não opinião do LLM. Aceita uma
// projeção mínima (nome da tool + resultado text) para evitar acoplamento com o
// pacote pipeline/orchestration.
//
// PRECISÃO: um execute_command genérico (ls, cat, pwd) NÃO é evidência de
// trabalho útil — só comandos que verificam build/test produzem evidence_gain.
func (r *Record) ApplyToolEvidence(toolName, toolResult string, success bool) {
	toolResult = strings.ToLower(toolResult)
	switch {
	case toolName == "write_file" || toolName == "write_file_edit":
		if success {
			r.ArtifactValue = 1
			r.TaskProgress = 1
		}
	case toolName == "execute_command" || toolName == "run_command":
		// Só comando de build/test/validação conta como evidência.
		if success && (strings.Contains(toolResult, "build") || strings.Contains(toolResult, "test")) {
			r.EvidenceGain = 1
			r.TaskProgress = 1
		}
	}
}

// UsefulWork devolve a soma das dimensões do vetor de valor.
func (r Record) UsefulWork() float64 {
	return r.KnowledgeGain + r.TaskProgress +
		float64(r.ArtifactValue) + float64(r.EvidenceGain) + float64(r.DecisionGain)
}

// Efficiency devolve a métrica Useful Work / Tokens (tokens_total). Retorna 0
// quando nenhum token foi consumido.
func (r Record) Efficiency() float64 {
	if r.TokensTotal <= 0 {
		return 0
	}
	return r.UsefulWork() / float64(r.TokensTotal)
}

// Validate aplica a invariância do professor: quando InputTokens e OutputTokens
// estão disponíveis, TokensTotal DEVE ser igual à soma (senão a métrica de
// eficiência fica inconsistente). Não rejeita o registro (a telemetria não
// derruba a execução) — apenas reporta o desvio para caiçar no load/relatório.
//
// Retorna "" quando consistente; um descreve o desvio quando não.
func (r Record) Validate() string {
	sum := r.InputTokens + r.OutputTokens
	if sum == 0 {
		return "" // nada preenchido — sem o que validar
	}
	if r.TokensTotal != sum {
		return fmt.Sprintf("tokens_total(%d) != input(%d)+output(%d)", r.TokensTotal, r.InputTokens, r.OutputTokens)
	}
	return ""
}

// Store é o log de custo (JSONL append-only) em `.cosca/cost/`.
type Store struct {
	dir string
}

// NewStore cria um Store apontando para o diretório dado (ex.: `.cosca/cost`).
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

// ForCoscaDir cria o Store a partir do diretório `.cosca` do projeto.
func ForCoscaDir(coscaDir string) *Store {
	return NewStore(filepath.Join(coscaDir, RecordsDir))
}

// RecordsPath devolve o caminho do arquivo JSONL.
func (s *Store) RecordsPath() string {
	return filepath.Join(s.dir, RecordsFile)
}

// Append registra uma execução como uma linha JSONL (append-only). Best-effort:
// nunca deve falhar a execução que o registra.
func (s *Store) Append(r Record) error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(s.RecordsPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	line, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = f.Write(append(line, '\n'))
	return err
}

// Load lê todas as execuções registradas (JSONL). Arquivo inexistente → lista
// vazia. Linhas inválidas são ignoradas (a telemetria não pode derrubar o
// relatório por uma linha corrompida).
func (s *Store) Load() ([]Record, error) {
	data, err := os.ReadFile(s.RecordsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var recs []Record
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var r Record
		if err := json.Unmarshal(line, &r); err != nil {
			continue // nunca derrubar o relatório por uma linha corrompida
		}
		recs = append(recs, r)
	}
	return recs, scanner.Err()
}

// ExecutionRecord é a telemetria de UMA execução dentro de uma tarefa causal.
// Preserva a granularidade individual (a fase da execução, a tool usada, os
// tokens, a duração, o resultado e os artefatos/evidências produzidos).
type ExecutionRecord struct {
	ExecutionID string `json:"execution_id,omitempty"`
	Phase       string `json:"phase,omitempty"` // write|build|test|inspect|...
	Tool        string `json:"tool,omitempty"`  // write_file|execute_command|...

	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TokensTotal  int `json:"tokens_total"`

	DurationMs int64  `json:"duration_ms,omitempty"`
	Status     string `json:"status"`              // success|failed|error
	Result     string `json:"result,omitempty"`    // resumo curto do resultado

	// Evidência de valor desta execução (ADR-031), decomposta.
	KnowledgeGain float64 `json:"knowledge_gain,omitempty"`
	TaskProgress  float64 `json:"task_progress,omitempty"`
	ArtifactValue int     `json:"artifact_value,omitempty"`
	EvidenceGain  int     `json:"evidence_gain,omitempty"`
	DecisionGain  int     `json:"decision_gain,omitempty"`
}

// TaskRecord é a unidade CAUSAL de observabilidade do COSCA: agrega todas as
// execuções que pertencem à MESMA tarefa, permitindo ao Auto-Audit responder
// "qual tarefa gerou este artefato, quais execuções participaram, quanto custou
// e qual evidência validou". Diferente do Record (telemetria por run), o
// TaskRecord olha a tarefa como um todo (write_file + build juntos).
type TaskRecord struct {
	TaskID  string `json:"task_id"`
	AgentID string `json:"agent_id,omitempty"`
	Goal    string `json:"goal,omitempty"`
	Status  string `json:"status"` // started|executing|completed|failed

	StartedAt  time.Time `json:"started_at,omitempty"`
	FinishedAt time.Time `json:"finished_at,omitempty"`

	Runs        int   `json:"runs"`
	TokensTotal int   `json:"tokens_total"`
	DurationMs  int64 `json:"duration_ms"`

	// Vetor de valor decomposto da TAREFA (soma das execuções).
	KnowledgeGain float64 `json:"knowledge_gain"`
	TaskProgress  float64 `json:"task_progress"`
	ArtifactValue int     `json:"artifact_value"`
	EvidenceGain  int     `json:"evidence_gain"`
	DecisionGain  int     `json:"decision_gain"`

	// Executions preserva a granularidade individual de cada execução.
	Executions []ExecutionRecord `json:"executions,omitempty"`
}

// UsefulWork devolve a soma das dimensões do vetor de valor da tarefa.
func (t TaskRecord) UsefulWork() float64 {
	return t.KnowledgeGain + t.TaskProgress +
		float64(t.ArtifactValue) + float64(t.EvidenceGain) + float64(t.DecisionGain)
}

// Efficiency devolve a métrica Useful Work / Tokens da tarefa.
func (t TaskRecord) Efficiency() float64 {
	if t.TokensTotal <= 0 {
		return 0
	}
	return t.UsefulWork() / float64(t.TokensTotal)
}

// Summary é a agregação por (agent_id, task_id): soma os tokens, o vetor de
// valor e a duração de todas as execuções registradas para o mesmo grupo.
type Summary struct {
	AgentID     string `json:"agent_id"`
	TaskID      string `json:"task_id"`
	Runs        int    `json:"runs"`
	TokensTotal int    `json:"tokens_total"`
	DurationMs  int64  `json:"duration_ms"`


	KnowledgeGain float64 `json:"knowledge_gain"`
	TaskProgress  float64 `json:"task_progress"`
	ArtifactValue int     `json:"artifact_value"`
	EvidenceGain  int     `json:"evidence_gain"`
	DecisionGain  int     `json:"decision_gain"`
}

// UsefulWork devolve a soma das dimensões do vetor de valor do grupo.
func (s Summary) UsefulWork() float64 {
	return s.KnowledgeGain + s.TaskProgress +
		float64(s.ArtifactValue) + float64(s.EvidenceGain) + float64(s.DecisionGain)
}

// Efficiency devolve a métrica Useful Work / Tokens do grupo (0 quando sem
// tokens consumidos).
func (s Summary) Efficiency() float64 {
	if s.TokensTotal <= 0 {
		return 0
	}
	return s.UsefulWork() / float64(s.TokensTotal)
}

// Report é a saída agregada do `cosca cost`.
type Report struct {
	// Grouped — agregação por (agent_id, task_id), ordenada por tokens desc.
	Grouped []Summary `json:"grouped"`

	// Total — totais de TODAS as execuções registradas.
	Total Summary `json:"total"`

	// Runs — total de execuções registradas.
	Runs int `json:"runs"`

	// Commands — comandos que geraram os registros (para a SAÍDA DECOMPOSTA:
	// o vetor de valor por execução). Limitado a 100 execuções por legibilidade.
	Commands []Record `json:"commands,omitempty"`

	// Decomposed — true quando a decomposição do uso (contexto/tools/history)
	// está preenchida. Fase 0: false (é a limitação documentada).
	Decomposed bool `json:"decomposed"`
}

// Aggregate agrupa as execuções por (agent_id, task_id) e computa a métrica.
// A ordenação é por tokens totais desc (as execuções que "gastaram muito"
// aparecem primeiro). Execuções sem agent_id/task_id caem em "".
func Aggregate(records []Record) *Report {
	rep := &Report{Runs: len(records)}
	keyed := make(map[string]*Summary)
	order := make([]string, 0)

	for _, r := range records {
		key := r.AgentID + "\x00" + r.TaskID
		s, ok := keyed[key]
		if !ok {
			s = &Summary{AgentID: r.AgentID, TaskID: r.TaskID}
			keyed[key] = s
			order = append(order, key)
		}
		s.Runs++
		s.TokensTotal += r.TokensTotal
		s.DurationMs += r.DurationMs
		s.KnowledgeGain += r.KnowledgeGain
		s.TaskProgress += r.TaskProgress
		s.ArtifactValue += r.ArtifactValue
		s.EvidenceGain += r.EvidenceGain
		s.DecisionGain += r.DecisionGain

		rep.Total.Runs++
		rep.Total.TokensTotal += r.TokensTotal
		rep.Total.DurationMs += r.DurationMs
		rep.Total.KnowledgeGain += r.KnowledgeGain
		rep.Total.TaskProgress += r.TaskProgress
		rep.Total.ArtifactValue += r.ArtifactValue
		rep.Total.EvidenceGain += r.EvidenceGain
		rep.Total.DecisionGain += r.DecisionGain

		// SAÍDA DECOMPOSTA: preservar execuções individuais (limitado a 100).
		if len(rep.Commands) < 100 {
			rep.Commands = append(rep.Commands, r)
		}

		// A decomposição real existe apenas se algum campo de breakdown > 0
		// aparecer (Fase 0.1). No Fase 0 isso é sempre false — limitação
		// honesta reportada na saída.
		if r.ContextTokens > 0 || r.ToolTokens > 0 || r.HistoryTokens > 0 {
			rep.Decomposed = true
		}
		// ADR-031 Fase 0.1: decomposição de cache/reasoning também marca true.
		if r.CachedTokens > 0 || r.ReasoningTokens > 0 {
			rep.Decomposed = true
		}
	}

	for _, key := range order {
		rep.Grouped = append(rep.Grouped, *keyed[key])
	}
	sort.SliceStable(rep.Grouped, func(i, j int) bool {
		return rep.Grouped[i].TokensTotal > rep.Grouped[j].TokensTotal
	})
	return rep
}

// AggregateTasks é a agregação CAUSAL por task_id: as execuções com o mesmo
// task_id são agrupadas em UM TaskRecord com Executions[] aninhadas — em vez
// de records separados sem vínculo. É o que permite o Auto-Audit responder
// "qual tarefa gerou o artefato e qual execução validou", em vez de inferir
// que um build pertence a uma escrita pela ordem.
func AggregateTasks(records []Record) []TaskRecord {
	taskMap := make(map[string]*TaskRecord)
	order := make([]string, 0)

	for _, r := range records {
		key := r.TaskID
		if key == "" {
			key = "unknown"
		}
		t, ok := taskMap[key]
		if !ok {
			t = &TaskRecord{
				TaskID:  r.TaskID,
				AgentID: r.AgentID,
				StartedAt: r.At,
			}
			taskMap[key] = t
			order = append(order, key)
		}

		// Agrega o vetor de valor desta execução à tarefa.
		t.Runs++
		t.TokensTotal += r.TokensTotal
		t.DurationMs += r.DurationMs
		t.KnowledgeGain += r.KnowledgeGain
		t.TaskProgress += r.TaskProgress
		t.ArtifactValue += r.ArtifactValue
		t.EvidenceGain += r.EvidenceGain
		t.DecisionGain += r.DecisionGain
		if r.At.After(t.FinishedAt) {
			t.FinishedAt = r.At
		}

		// Preserva a execução individual (converte Record -> ExecutionRecord).
		t.Executions = append(t.Executions, recordToExecution(r))
	}

	// Ordena por tokens desc (tarefas que "gastaram muito" primeiro).
	out := make([]TaskRecord, 0, len(order))
	for _, key := range order {
		out = append(out, *taskMap[key])
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].TokensTotal > out[j].TokensTotal
	})
	return out
}

// recordToExecution converte um Record (telemetria de run) em um
// ExecutionRecord (telemetria de execução dentro de uma tarefa causal).
// Backward-compatible: preserva todos os campos de valor já registrados.
func recordToExecution(r Record) ExecutionRecord {
	// Deriva a fase da tool quando disponível (todo acesso é conservador).
	phase := "execution"
	switch {
	case r.Result != "":
		phase = "task"
	}

	return ExecutionRecord{
		ExecutionID:   r.TaskID,
		Phase:         phase,
		InputTokens:   r.InputTokens,
		OutputTokens:  r.OutputTokens,
		TokensTotal:   r.TokensTotal,
		DurationMs:    r.DurationMs,
		Status:        executionStatus(r),
		Result:        r.Result,
		KnowledgeGain: r.KnowledgeGain,
		TaskProgress:  r.TaskProgress,
		ArtifactValue: r.ArtifactValue,
		EvidenceGain:  r.EvidenceGain,
		DecisionGain:  r.DecisionGain,
	}
}

// executionStatus deriva um status simples do Record para a ExecutionRecord.
// Um record sem erro é "success"; com erro (sem artifact/evidence) é "failed".
func executionStatus(r Record) string {
	if r.ArtifactValue > 0 || r.EvidenceGain > 0 {
		return "success"
	}
	if r.TaskProgress > 0 || r.KnowledgeGain > 0 {
		return "success"
	}
	return "failed"
}
