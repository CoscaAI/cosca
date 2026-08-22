package proposal

import (
	"sync"
	"time"
)

// ─── Auditoria / Proveniência ────────────────────────────────────────────────
//
// O AuditLog é o registro append-only de cada veredicto do Kernel isolado.
// Nada no fluxo é invisível: o Don pode ver, em ordem, cada proposta, cada
// julgamento, cada regra acionada, cada bloqueio. É a base da forense
// pós-incidente (espelho do L187) e a prova de que "o que não é registrado,
// não aconteceu".

// AuditEntry registra um evento do fluxo de validação.
type AuditEntry struct {
	// ProposalID é a proposta envolvida.
	ProposalID string `json:"proposal_id"`
	// Verdict é o veredicto emitido.
	Verdict Verdict `json:"verdict"`
	// Reason é o motivo (obrigatório em DENY/REVIEW).
	Reason string `json:"reason"`
	// RulesHit são as regras da lei acionadas.
	RulesHit []string `json:"rules_hit,omitempty"`
	// NeedsDon indica se a proposta exige o Don.
	NeedsDon bool `json:"needs_don"`
	// IsFailClosed marca veredictos de emergência.
	IsFailClosed bool `json:"is_fail_closed,omitempty"`
	// At é o momento do evento.
	At time.Time `json:"at"`
}

// AuditLog é o registro append-only de auditoria (seguro para concorrência).
type AuditLog struct {
	mu      sync.Mutex
	entries []AuditEntry
}

// NewAuditLog cria um registro de auditoria vazio.
func NewAuditLog() *AuditLog { return &AuditLog{} }

// Record anexa um evento ao registro (append-only — nunca edita o passado).
func (l *AuditLog) Record(e AuditEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, e)
}

// Entries devolve uma cópia de todos os eventos registrados, em ordem.
func (l *AuditLog) Entries() []AuditEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]AuditEntry, len(l.entries))
	copy(out, l.entries)
	return out
}

// RecordVerdict registra um veredicto do Kernel isolado no log.
func (l *AuditLog) RecordVerdict(v *VerdictResult) {
	l.Record(AuditEntry{
		ProposalID:   v.ProposalID,
		Verdict:      v.Verdict,
		Reason:       v.Reason,
		RulesHit:     v.RulesHit,
		NeedsDon:     v.NeedsDon,
		IsFailClosed: v.IsFailClosed,
		At:           v.ValidatedAt,
	})
}
