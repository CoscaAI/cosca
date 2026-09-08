// BuildContext fornece o TaskContext — o "ponto de entrada" do Task-Aware Search
// (ADR-045 §5.6). Construído ANTES de chamar BuildContext pelo executor/orchestrator.
//
// O TaskContext encapsula o perfil de tarefa (DeriveProfile) e a detecção de fase
// (DetectPhase), prontos para serem consumidos pelo contextpipeline.
//
// Nil-safe: ImplementaçãoState nil → retorna nil (pipeline ignora TAS).
package taskaffinity

import (
	"strings"

	"github.com/CoscaAI/cosca/internal/taskphase"
)

// TaskContext é o que o Task-Aware Search fornece ao pipeline.
// É opcional: nil = pipeline se comporta como hoje.
type TaskContext struct {
	// Profile é o perfil de tarefa derivado.
	Profile *TaskProfile

	// PhaseDetection é a detecção de fase.
	PhaseDetection *taskphase.PhaseDetection
}

// BuildTaskContext constrói o contexto de tarefa a partir do estado de
// implementação. É o ponto de entrada único do TAS (ADR-045 §5.6).
//
// Nil-safe: ImplementaçãoState nil → retorna nil (pipeline ignora TAS).
func BuildTaskContext(state *ImplementaçãoState) *TaskContext {
	if state == nil {
		return nil
	}

	profile := DeriveProfile(state)
	if profile == nil {
		return nil
	}

	// Detectar fase a partir dos sinais derivados do state.
	signals := BuildPhaseSignals(state)
	detection := taskphase.DetectPhase(signals)

	return &TaskContext{
		Profile:        profile,
		PhaseDetection: &detection,
	}
}

// BuildPhaseSignals converte ImplementaçãoState em PhaseSignals.
// Função pública (exportada) para testes e uso externo.
func BuildPhaseSignals(state *ImplementaçãoState) *taskphase.PhaseSignals {
	if state == nil {
		return nil
	}

	return &taskphase.PhaseSignals{
		HasOpenFiles:           len(state.OpenFiles) > 0,
		HasRecentModifications: len(state.RecentFiles) > 0,
		OpenFileCount:          len(state.OpenFiles),
		TargetIsSpecific:       isSpecificTarget(state.TargetHint, state.OpenFiles),
		HasTestTerms:           taskphase.ContainsTerm(strings.ToLower(state.Prompt), taskphase.VerificationTerms),
		HasImplementationTerms: taskphase.ContainsTerm(strings.ToLower(state.Prompt), taskphase.ImplementationTerms),
		HasExplorationTerms:    taskphase.ContainsTerm(strings.ToLower(state.Prompt), taskphase.ExplorationTerms),
		SearchHistoryCount:     0, // será incrementado pelo executor por sessão
		TargetHasTests:         false, // será detectado por filesystem check
	}
}

// isSpecificTarget determina se o alvo é um arquivo específico (não um módulo).
// Um path termina em extensão de arquivo = específico; um path de diretório = genérico.
func isSpecificTarget(targetHint string, openFiles []string) bool {
	target := targetHint
	if target == "" && len(openFiles) > 0 {
		target = openFiles[0]
	}
	if target == "" {
		return false
	}
	// Se o path termina em extensão de arquivo (ex.: ".go", ".ts"), é específico.
	for _, ext := range []string{".go", ".ts", ".tsx", ".py", ".rs", ".js", ".jsx"} {
		if strings.HasSuffix(strings.ToLower(target), ext) {
			return true
		}
	}
	return false
}
