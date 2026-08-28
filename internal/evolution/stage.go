// Package evolution — projetável do percurso de uma mudança (Stage observability,
// mineração AI-DLC). É PROJEÇÃO/OBSERVABILIDADE, NUNCA control plane.
//
// Regra de ouro (ADR-018 / ADR-016 I1): o Cosca JÁ tem os gates por-concern
// (proposal, quarantine, skilleval, deliberate — todos ZERO-LLM). O Stage é a
// projeção que AMARRA esses gates ao percurso de uma mudança, SEM re-implementar
// nenhum veredicto. Cada StagePhase referencia o gate que DECIDIU (GateUsed) e o
// artefato imutável (ArtifactRef = hash), mas quem julga continua sendo o gate.
//
// Duas regras de escopo:
//   - NÃO é control plane (não orquestra proposal/quarantine/skilleval; isso é o
//     internal/workflow). Aqui é somente a leitura de "onde a mudança está".
//   - Append-only (I5): o roll nunca é sobrescrito, só acrescido — auditável.
//
// Determinístico (I1): CanStageAdvance e Advance são funções puras, ZERO LLM.
package evolution

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Stage é um estágio observável do percurso de uma mudança (eixo B do lifecycle).
type Stage string

const (
	StageTask       Stage = "task"
	StageRequirement Stage = "requirement"
	StageDesign     Stage = "design"
	StageImplement  Stage = "implement"
	StageTest       Stage = "test"
	StageEvaluate   Stage = "evaluate"
	StagePromote    Stage = "promote"
)

// StageOrder é a ordem canônica do percurso (TASK→...→PROMOTE).
var StageOrder = []Stage{
	StageTask, StageRequirement, StageDesign, StageImplement,
	StageTest, StageEvaluate, StagePromote,
}

// Valid informa se s é um estágio conhecido.
func (s Stage) Valid() bool {
	for _, v := range StageOrder {
		if s == v {
			return true
		}
	}
	return false
}

// Index devolve a posição de s na ordem canônica (-1 se inválido).
func (s Stage) Index() int {
	for i, v := range StageOrder {
		if s == v {
			return i
		}
	}
	return -1
}

// Next devolve o próximo estágio após s ("" se s é o último/inválido).
func (s Stage) Next() Stage {
	i := s.Index()
	if i < 0 || i >= len(StageOrder)-1 {
		return ""
	}
	return StageOrder[i+1]
}

// CanStageAdvance é determinístico (I1): só avança EXATAMENTE um estágio, nunca
// pula nem retrocede. Função pura.
func CanStageAdvance(current, next Stage) bool {
	if !current.Valid() || !next.Valid() {
		return false
	}
	ci, ni := current.Index(), next.Index()
	return ni == ci+1
}

// =============================================================================
// StagePhase — uma transição de estágio, referenciando o gate que decidiu.
// =============================================================================

// StagePhase registra uma transição do percurso.
type StagePhase struct {
	// Stage atingido.
	Stage Stage `json:"stage"`
	// ArtifactRef é o hash imutável do artefato desta etapa (ProposalHash /
	// EvidenceHash / commit). Dado, nunca autoridade.
	ArtifactRef string `json:"artifact_ref"`
	// GateUsed identifica o gate determinístico que DECIDIU esta etapa
	// (proposal | quarantine | skilleval | deliberate | workflow). Referência,
	// não re-implementação.
	GateUsed string `json:"gate_used"`
	// Verdict é o veredicto do gate (APPROVE/DENY | PASS/FAIL | EMIT_*).
	Verdict string `json:"verdict"`
	// At é quando a transição ocorreu.
	At time.Time `json:"at"`
}

// =============================================================================
// StageRoll — a projeção observável (append-only) do percurso.
// =============================================================================

// StageRoll é a projeção "onde a mudança está".
type StageRoll struct {
	// Project identifica a mudança/tarefa (ex: "F1.5-skill-evolve").
	Project string `json:"project"`
	// Current é o estágio atual.
	Current Stage `json:"current"`
	// Completed é o histórico append-only de transições.
	Completed []StagePhase `json:"completed"`
	// Next é o próximo estágio sugerido ("" se no fim).
	Next Stage `json:"next"`
	// UpdatedAt é a última atualização.
	UpdatedAt time.Time `json:"updated_at"`
}

// NewStageRoll inicia um roll para um projeto.
func NewStageRoll(project string) *StageRoll {
	return &StageRoll{Project: project, Current: StageTask, UpdatedAt: time.Now().UTC()}
}

// Advance é função pura (I1): aplica uma transição válida e devolve o roll
// atualizado (append-only — nunca muta o original).
func (r *StageRoll) Advance(phase StagePhase) (*StageRoll, error) {
	if !CanStageAdvance(r.Current, phase.Stage) {
		return nil, fmt.Errorf("cannot advance %s → %s (deterministic stage order I1)", r.Current, phase.Stage)
	}
	phases := make([]StagePhase, len(r.Completed)+1)
	copy(phases, r.Completed)
	phases[len(r.Completed)] = phase
	return &StageRoll{
		Project:   r.Project,
		Current:   phase.Stage,
		Completed: phases,
		Next:      phase.Stage.Next(),
		UpdatedAt: time.Now().UTC(),
	}, nil
}

// =============================================================================
// StageRollStore — persistência append-only (I5), espelhando a disciplina do
// ledger. Volátil quando path vazio.
// =============================================================================

// StageRollStore persiste um StageRoll em arquivo (definida por projeto). Cada
// chamada grava o roll completo mais recente (snapshot) — a verdade é a última
// linha, e o arquivo histórico é append-only.
type StageRollStore struct {
	mu   sync.Mutex
	path string
}

// NewStageRollStore cria um store em `path`.
func NewStageRollStore(path string) *StageRollStore {
	return &StageRollStore{path: path}
}

// Save grava o snapshot do roll (append-only — nova linha).
func (s *StageRollStore) Save(roll *StageRoll) error {
	if roll == nil {
		return fmt.Errorf("stage store: nil roll")
	}
	if s.path == "" {
		return nil // modo volátil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("stage store: mkdir: %w", err)
	}
	data, err := json.Marshal(roll)
	if err != nil {
		return fmt.Errorf("stage store: marshal: %w", err)
	}
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("stage store: open: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("stage store: write: %w", err)
	}
	return nil
}

// Load devolve o ÚLTIMO snapshot (a linha mais recente). Volátil/inexistente
// devolve nil,nil.
func (s *StageRollStore) Load() (*StageRoll, error) {
	if s.path == "" {
		return nil, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stage store: load: %w", err)
	}
	var roll *StageRoll
	for _, line := range splitLines(string(data)) {
		if line == "" {
			continue
		}
		var r StageRoll
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			continue // linha corrompida: ignora, não derruba
		}
		roll = &r
	}
	return roll, nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
