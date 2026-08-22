package supervisor

import (
	"fmt"
	"sync"
	"time"
)

// EscalationManager handles the escalation of situations to the Don.
// Levels 0-1 are automatic. Levels 2+ require Don approval.
type EscalationManager struct {
	history []Escalation
	mu      sync.Mutex
}

// NewEscalationManager creates a new escalation manager.
func NewEscalationManager() *EscalationManager {
	return &EscalationManager{
		history: make([]Escalation, 0),
	}
}

// Escalate creates an escalation record and returns whether the Don must be
// notified.
func (m *EscalationManager) Escalate(kernelID KernelID, taskID, summary, detail string, level EscalationLevel) Escalation {
	m.mu.Lock()
	defer m.mu.Unlock()

	e := Escalation{
		Level:     level,
		KernelID:  kernelID,
		TaskID:    taskID,
		Summary:   summary,
		Detail:    detail,
		NeedsDon:  level >= LevelRollback,
		Timestamp: time.Now(),
	}

	// Decide action based on level.
	switch level {
	case LevelAutoCorrect:
		e.Decision = "SUPERVISOR: correção automática aplicada — Don não notificado"
	case LevelRestart:
		e.Decision = fmt.Sprintf("SUPERVISOR: kernel %q reiniciado com checkpoint", kernelID)
	case LevelRollback:
		e.Decision = fmt.Sprintf("SUPERVISOR: rollback executado — Don notificado. Kernel %q, tarefa %s.",
			kernelID, taskID)
	case LevelConflict:
		e.Decision = fmt.Sprintf("SUPERVISOR: conflito detectado entre kernels. Aguardando decisão do Don.")
	case LevelEmergency:
		e.Decision = "SUPERVISOR: EMERGÊNCIA CRÍTICA — HALT GERAL. Don notificado IMEDIATAMENTE."
	}

	m.history = append(m.history, e)

	// Keep only last 100 escalations.
	if len(m.history) > 100 {
		m.history = m.history[len(m.history)-100:]
	}

	return e
}

// Recent returns the most recent escalations, newest first.
func (m *EscalationManager) Recent(limit int) []Escalation {
	m.mu.Lock()
	defer m.mu.Unlock()

	n := len(m.history)
	if limit > n {
		limit = n
	}
	result := make([]Escalation, limit)
	for i := 0; i < limit; i++ {
		result[i] = m.history[n-1-i]
	}
	return result
}

// PendingApproval returns escalations that need Don approval.
func (m *EscalationManager) PendingApproval() []Escalation {
	m.mu.Lock()
	defer m.mu.Unlock()

	var pending []Escalation
	for _, e := range m.history {
		if e.NeedsDon {
			pending = append(pending, e)
		}
	}
	return pending
}

// MessageForDon formats an escalation as a message for the Don.
func MessageForDon(e Escalation) string {
	switch e.Level {
	case LevelRollback:
		return fmt.Sprintf(
			"⚠️ Chef, o kernel %s tentou executar a tarefa %s mas causou regressão.\n"+
				"   • Problema: %s\n"+
				"   • Ação tomada: rollback executado.\n"+
				"   • Aguardando sua ordem para prosseguir.",
			e.KernelID, e.TaskID, e.Summary)

	case LevelConflict:
		return fmt.Sprintf(
			"⚡ Chef, conflito entre kernels detectado.\n"+
				"   • Kernel: %s\n"+
				"   • Tarefa: %s\n"+
				"   • Detalhe: %s\n"+
				"   • Ambos os kernels estão pausados. Preciso da sua decisão.",
			e.KernelID, e.TaskID, e.Summary)

	case LevelEmergency:
		return fmt.Sprintf(
			"🚨 CHEF — EMERGÊNCIA CRÍTICA 🚨\n"+
				"   • Todos os kernels em HALT.\n"+
				"   • Causa: %s\n"+
				"   • Detalhe: %s\n"+
				"   • Aguardando sua intervenção imediata.",
			e.Summary, e.Detail)

	default:
		return fmt.Sprintf("📋 Atualização: %s", e.Summary)
	}
}

// CountByLevel returns escalation counts grouped by level.
func (m *EscalationManager) CountByLevel() map[EscalationLevel]int {
	m.mu.Lock()
	defer m.mu.Unlock()

	counts := make(map[EscalationLevel]int)
	for _, e := range m.history {
		counts[e.Level]++
	}
	return counts
}
