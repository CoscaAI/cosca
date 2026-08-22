package proposal

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ─── O Fluxo ─────────────────────────────────────────────────────────────────
//
// O Flow orquestra o ciclo de vida completo de uma proposta:
//
//	Submit → Kernel isolado → APPROVE → (Don? →) Execute → Feedback
//	                          ├── DENY → arquivado
//	                          └── REVIEW → corrige → re-submete (máx 2)
//
// Leis do fluxo:
//   - Fail-closed: sem veredicto, sem execução. Nunca.
//   - Timeout duro: o Kernel isolado tem tempo limitado para julgar.
//     Estourou → DENY de emergência + alerta.
//   - Don no loop: APPROVE do Kernel isolado em proposta destrutiva/
//     estratégica é recomendação — a execução exige o Don.
//   - REVIEW loop: no máximo MaxRevisions correções; depois, DENY.
//   - Feedback: o resultado da execução volta e alimenta a memória
//     validada (falha gera revisão obrigatória da proposta original).

// FailClosedReason identifica o motivo de um veredicto de emergência.
type FailClosedReason string

const (
	// FailTimeout: o Kernel isolado não respondeu no tempo.
	FailTimeout FailClosedReason = "timeout"
	// FailUnavailable: o Kernel isolado está indisponível.
	FailUnavailable FailClosedReason = "kernel-isolado-indisponivel"
	// FailClosedDefault: qualquer outra falha — nega sem exceção.
	FailClosedDefault FailClosedReason = "fail-closed"
)

// ErrNotFound é retornado quando a proposta não existe no fluxo.
var ErrNotFound = fmt.Errorf("proposta não encontrada")

// ErrDenied é retornado quando se tenta executar uma proposta sem veredicto
// APPROVE (fail-closed).
var ErrDenied = fmt.Errorf("execução negada: sem veredicto APPROVE — fail-closed")

// ErrNeedsDon é retornado quando se tenta executar sozinho uma proposta que
// exige o Don.
var ErrNeedsDon = fmt.Errorf("execução exige o Don: aprovação do Kernel isolado é recomendação, não liberação")

// ErrMaxRevisions é retornado quando a proposta já estourou o limite de
// revisões e ainda não foi aprovada.
var ErrMaxRevisions = fmt.Errorf("proposta atingiu o limite de revisões sem aprovação")

// Flow é o fluxo de validação do Kernel isolado.
type Flow struct {
	mu        sync.Mutex
	validator *Validator
	timeout   time.Duration
	// pendentes guarda propostas ainda não executadas (fila visível ao Don).
	pendentes map[string]*Proposal
	// veredictos guarda o último veredicto por proposta.
	veredictos map[string]*VerdictResult
	// aprovadas guarda a proposta EXATA aprovada (binding hash ↔ autorização):
	// a execução só vale para o artefato validado, nunca para mutações posteriores.
	aprovadas map[string]*Proposal
	// executadas guarda IDs cuja autorização já foi consumida (single-use).
	// Replay da mesma autorização → negado. Autorização é um passe de entrada,
	// não um salvo-conduto reutilizável.
	executadas map[string]bool
	// execuções guarda as execuções concluídas (feedback).
	execucoes map[string]*Execution
	// log é o registro de auditoria (proveniência) do fluxo.
	log *AuditLog
	// seq gera IDs de proposta sequenciais (P-0001).
	seq int
}

// NewFlow cria o fluxo de validação com o Kernel isolado e um timeout duro.
// Timeout zero ou negativo → sem limite de tempo (não recomendado).
func NewFlow(validator *Validator, timeout time.Duration) *Flow {
	if validator == nil {
		validator = NewValidator()
	}
	return &Flow{
		validator:  validator,
		timeout:    timeout,
		pendentes:  make(map[string]*Proposal),
		veredictos: make(map[string]*VerdictResult),
		aprovadas:  make(map[string]*Proposal),
		executadas: make(map[string]bool),
		execucoes:  make(map[string]*Execution),
		log:        NewAuditLog(),
	}
}

// nextID gera o próximo ID de proposta (P-0001, P-0002, ...).
func (f *Flow) nextID() string {
	f.seq++
	return fmt.Sprintf("P-%04d", f.seq)
}

// Submit entrega uma proposta do Kernel principal ao Kernel isolado.
// Devolve o veredicto da validação independente. Fail-closed: se a
// validação estourar o timeout ou falhar, o veredicto é DENY de emergência.
func (f *Flow) Submit(ctx context.Context, p *Proposal) *VerdictResult {
	if p == nil {
		return failClosed("proposta nula", FailClosedDefault)
	}

	f.mu.Lock()
	if strings.TrimSpace(p.ID) == "" {
		p.ID = f.nextID()
	} else if _, ok := f.pendentes[p.ID]; !ok && !strings.HasPrefix(p.ID, "P-") {
		// Mantém ID externo, mas normaliza para o formato da casa.
		p.ID = NormalizeProposalID(p.ID)
	}
	p.SubmittedAt = time.Now()
	f.mu.Unlock()

	// F11/A6: sela UMA cópia imutável ANTES da validação — o Kernel
	// isolado julga o snapshot, nunca o ponteiro vivo do chamador. Se o
	// juiz (ou qualquer componente da validação) mutar o artefato durante
	// o julgamento, o hash diverge e o veredicto vira DENY fail-closed.
	seal := cloneProposal(p)
	hashBefore := ProposalHash(seal)
	v := f.validate(ctx, seal)
	if got := ProposalHash(seal); got != hashBefore {
		v = &VerdictResult{
			ProposalID:   seal.ID,
			Verdict:      VerdictDeny,
			Reason:       "fail-closed: o artefato foi alterado durante a validação (hash divergiu)",
			IsFailClosed: true,
			ValidatedAt:  time.Now(),
		}
	}
	f.mu.Lock()
	f.veredictos[p.ID] = v
	// Auditoria: toda decisão é registrada (append-only) — INV-14.
	f.log.Record(AuditEntry{
		ProposalID: p.ID,
		Verdict:    v.Verdict,
		Reason:     v.Reason,
		NeedsDon:   v.NeedsDon,
		At:         time.Now(),
	})
	if v.Approved() {
		// Binding hash ↔ autorização: a proposta EXATA validada fica
		// SELADA (cópia profunda selada ANTES do julgamento). O chamador
		// pode mutar o ponteiro original à vontade — a execução só vale
		// para o artefato selado e julgado.
		f.aprovadas[p.ID] = seal
	}
	if v.Verdict != VerdictDeny {
		f.pendentes[p.ID] = p
	}
	f.mu.Unlock()
	return v
}

// validate aplica a validação respeitando o timeout duro (fail-closed).
func (f *Flow) validate(ctx context.Context, p *Proposal) *VerdictResult {
	if ctx == nil {
		ctx = context.Background()
	}
	if f.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, f.timeout)
		defer cancel()
	}

	done := make(chan *VerdictResult, 1)
	go func() {
		done <- f.validator.Validate(ctx, p)
	}()

	select {
	case v := <-done:
		return v
	case <-ctx.Done():
		return &VerdictResult{
			ProposalID:   p.ID,
			Verdict:      VerdictDeny,
			Reason:       "Kernel isolado não respondeu no tempo — fail-closed (sem veredicto, sem execução)",
			IsFailClosed: true,
			ValidatedAt:  time.Now(),
		}
	}
}

// Review devolve uma proposta em REVIEW corrigida ao Kernel isolado.
// A correção incrementa Revisions; o limite é imposto na validação.
func (f *Flow) Review(id string, corrected *Proposal) (*VerdictResult, error) {
	f.mu.Lock()
	orig, ok := f.pendentes[id]
	if !ok {
		f.mu.Unlock()
		return nil, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	f.mu.Unlock()

	if corrected == nil {
		return nil, fmt.Errorf("correção nula para a proposta %s", id)
	}

	// Preserva identidade e histórico de revisões.
	corrected.ID = id
	corrected.Revisions = orig.Revisions + 1
	corrected.SubmittedAt = time.Now()

	seal := cloneProposal(corrected)
	hashBefore := ProposalHash(seal)
	v := f.validate(context.Background(), seal)
	if got := ProposalHash(seal); got != hashBefore {
		v = &VerdictResult{
			ProposalID:   id,
			Verdict:      VerdictDeny,
			Reason:       "fail-closed: o artefato foi alterado durante a validação (hash divergiu)",
			IsFailClosed: true,
			ValidatedAt:  time.Now(),
		}
	}
	f.mu.Lock()
	f.veredictos[id] = v
	f.log.Record(AuditEntry{
		ProposalID: id,
		Verdict:    v.Verdict,
		Reason:     v.Reason,
		NeedsDon:   v.NeedsDon,
		At:         time.Now(),
	})
	if v.Approved() {
		f.aprovadas[id] = seal
	}
	if v.Verdict == VerdictDeny && v.Reason != "" && strings.Contains(v.Reason, "limite") {
		f.mu.Unlock()
		return v, ErrMaxRevisions
	}
	if v.Verdict != VerdictDeny {
		f.pendentes[id] = corrected
	} else {
		delete(f.pendentes, id)
	}
	f.mu.Unlock()
	return v, nil
}

// Execute executa uma proposta aprovada. Leis de ferro:
//   - Sem veredicto APPROVE registrado → ErrDenied (fail-closed)
//   - Proposta que exige Don e executor não é don/admin → ErrNeedsDon
//   - Autorização é single-use: reexecutar a mesma proposta → ErrDenied
//   - A proposta aprovada fica SELADA: a execução vale apenas para o
//     artefato exato validado (binding proposal_hash ↔ autorização).
//     Qualquer mutação posterior → ErrDenied.
func (f *Flow) Execute(id, actor string) (*Execution, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	v, ok := f.veredictos[id]
	if !ok || !v.Approved() {
		return nil, fmt.Errorf("%w (%s)", ErrDenied, id)
	}
	// F10: replay bloqueado — a autorização é um passe de entrada, consumido
	// na primeira execução. Não é salvo-conduto reutilizável.
	if f.executadas[id] {
		return nil, fmt.Errorf("%w (%s) — autorização já consumida (single-use)", ErrDenied, id)
	}
	// F11: binding de hash — executa a proposta SELADA validada, não o
	// ponteiro externo mutável. Mutação pós-validação nunca alcança o palco.
	selada, ok := f.aprovadas[id]
	if !ok {
		return nil, fmt.Errorf("%w (%s) — proposta selada não encontrada", ErrDenied, id)
	}
	if v.NeedsDon && !isApprover(actor) {
		return nil, fmt.Errorf("%w (%s) — papel %q não pode executar sozinho", ErrNeedsDon, id, actor)
	}

	exec := &Execution{
		ProposalID: id,
		Actor:      actor,
		ExecutedAt: time.Now(),
		Outcome:    OutcomeSuccess,
	}
	f.execucoes[id] = exec
	// Consome a autorização e libera a proposta selada.
	f.executadas[id] = true
	delete(f.aprovadas, id)
	delete(f.pendentes, id)
	// Auditoria da execução: prova de que a autorização foi usada exatamente
	// para o artefato selado, pelo ator autorizado.
	f.log.Record(AuditEntry{
		ProposalID: id,
		Verdict:    VerdictApprove,
		Reason:     "EXECUÇÃO: ator=" + actor + " hash=" + ProposalHash(selada),
		NeedsDon:   v.NeedsDon,
		At:         exec.ExecutedAt,
	})
	return exec, nil
}

// Feedback entrega o resultado real da execução de volta ao fluxo —
// alimenta a memória validada. Falha gera revisão obrigatória da proposta.
func (f *Flow) Feedback(id string, outcome Outcome, note string) (*Execution, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	exec, ok := f.execucoes[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	exec.Outcome = outcome
	exec.Note = note
	if exec.Failed() {
		// Falha vira lição: registra no log de auditoria com peso máximo.
		f.log.Record(AuditEntry{
			ProposalID: id,
			Verdict:    VerdictReview,
			Reason:     "FALHA PÓS-EXECUÇÃO: " + note + " — revisão obrigatória da proposta original",
			NeedsDon:   true,
			At:         time.Now(),
		})
	}
	return exec, nil
}

// Pendings devolve a fila de propostas ainda não executadas (visível ao Don).
func (f *Flow) Pendings() []*Proposal {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*Proposal, 0, len(f.pendentes))
	for _, p := range f.pendentes {
		out = append(out, p)
	}
	return out
}

// Verdict devolve o último veredicto registrado para uma proposta.
func (f *Flow) Verdict(id string) (*VerdictResult, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.veredictos[id]
	return v, ok
}

// AuditLog expõe o registro de auditoria do fluxo (append-only).
func (f *Flow) AuditLog() []AuditEntry {
	return f.log.Entries()
}

// Validator devolve o Kernel isolado do fluxo (para injetar a L2 semântica).
func (f *Flow) Validator() *Validator { return f.validator }

// isApprover define quem pode executar proposta que exige o Don.
func isApprover(actor string) bool {
	a := strings.ToLower(strings.TrimSpace(actor))
	return a == "don" || a == "admin"
}

// failClosed monta um veredicto de emergência fail-closed.
func failClosed(reason string, r FailClosedReason) *VerdictResult {
	return &VerdictResult{
		Verdict:      VerdictDeny,
		Reason:       "fail-closed (" + string(r) + "): " + reason,
		IsFailClosed: true,
		ValidatedAt:  time.Now(),
	}
}

// NormalizeProposalID normaliza um ID externo para o formato da casa.
func NormalizeProposalID(id string) string {
	return strings.ToUpper(strings.TrimSpace(id))
}
