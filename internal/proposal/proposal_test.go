package proposal

import (
	"context"
	"strings"
	"testing"
	"time"
)

// validProposal monta uma proposta que respeita o contrato mínimo.
func validProposal() *Proposal {
	ev := "output bruto da IA externa: criar arquivo README.md no diretório docs"
	return &Proposal{
		Action: "criar arquivo README.md",
		Target: "docs/README.md",
		Motive: "documentar o módulo de validação para a família",
		Origin: "Don pediu documentação",
		State:  "docs existe, sem README",
		Risk:   RiskNormal,
		Evidence: Provenance{
			Source:       "ia-externa:test",
			ReceivedAt:   time.Now(),
			Evidence:     ev,
			EvidenceHash: HashEvidence(ev),
		},
	}
}

func TestProposalMissingFields(t *testing.T) {
	p := validProposal()
	p.Action = ""
	p.Risk = ""
	p.Origin = ""
	missing := p.MissingFields()
	if len(missing) != 3 {
		t.Fatalf("esperava 3 campos faltantes, veio %v", missing)
	}
}

func TestRiskRequiresDon(t *testing.T) {
	if RiskTrivial.RequiresDon() || RiskNormal.RequiresDon() {
		t.Fatal("trivial/normal não deveriam exigir Don")
	}
	if !RiskDestructive.RequiresDon() || !RiskStrategic.RequiresDon() {
		t.Fatal("destructive/strategic deveriam exigir Don")
	}
}

func TestParseRiskClass(t *testing.T) {
	cases := []struct {
		in   string
		want RiskClass
	}{
		{"trivial", RiskTrivial},
		{"NORMAL", RiskNormal},
		{"Destructive", RiskDestructive},
		{"STRATEGIC", RiskStrategic},
	}
	for _, c := range cases {
		got, err := ParseRiskClass(c.in)
		if err != nil || got != c.want {
			t.Errorf("ParseRiskClass(%q) = %v, %v; esperava %v", c.in, got, err, c.want)
		}
	}
	if _, err := ParseRiskClass("explosiva"); err == nil {
		t.Error("classe inválida deveria falhar")
	}
}

func TestValidateApprove(t *testing.T) {
	v := NewValidator()
	r := v.Validate(context.Background(), validProposal())
	if r.Verdict != VerdictApprove {
		t.Fatalf("esperava APPROVE, veio %s — %s", r.Verdict, r.Reason)
	}
	if r.NeedsDon {
		t.Fatal("proposta normal não deveria exigir Don")
	}
}

func TestValidateFatalPkill(t *testing.T) {
	v := NewValidator()
	p := validProposal()
	p.Action = "executar pkill -f node para reiniciar o serviço"
	p.Risk = RiskDestructive
	r := v.Validate(context.Background(), p)
	if r.Verdict != VerdictDeny {
		t.Fatalf("pkill deveria ser DENY fatal (L213), veio %s", r.Verdict)
	}
	if !contains(r.RulesHit, "R01") {
		t.Fatalf("regra R01 deveria ter sido acionada, veio %v", r.RulesHit)
	}
}

func TestValidateGitRewriteNeedsDon(t *testing.T) {
	v := NewValidator()
	p := validProposal()
	p.Action = "git reset --hard HEAD~1"
	p.Risk = RiskDestructive
	r := v.Validate(context.Background(), p)
	if r.Verdict != VerdictApprove {
		t.Fatalf("esperava APPROVE com guarda, veio %s — %s", r.Verdict, r.Reason)
	}
	if !r.NeedsDon {
		t.Fatal("reescrita git deveria exigir Don (R02)")
	}
}

func TestValidateRmSensitiveNeedsDon(t *testing.T) {
	v := NewValidator()
	p := validProposal()
	p.Action = "rm -rf .cosca para limpeza"
	p.Target = ".cosca"
	p.Risk = RiskDestructive
	r := v.Validate(context.Background(), p)
	if r.Verdict != VerdictApprove {
		t.Fatalf("esperava APPROVE com guarda, veio %s", r.Verdict)
	}
	if !r.NeedsDon {
		t.Fatal("rm -rf em .cosca deveria exigir Don (R03)")
	}
}

func TestValidateIncompleteReview(t *testing.T) {
	v := NewValidator()
	p := validProposal()
	p.Motive = ""
	r := v.Validate(context.Background(), p)
	if r.Verdict != VerdictReview {
		t.Fatalf("proposta incompleta deveria ser REVIEW, veio %s", r.Verdict)
	}
	if !strings.Contains(r.Reason, "motive") {
		t.Fatalf("REVIEW deveria apontar o campo faltante, veio %q", r.Reason)
	}
}

func TestValidateEvidenceHashMismatch(t *testing.T) {
	v := NewValidator()
	p := validProposal()
	p.Evidence.EvidenceHash = "0000000000000000000000000000000000000000000000000000000000000000"
	r := v.Validate(context.Background(), p)
	if r.Verdict != VerdictReview {
		t.Fatalf("hash quebrado deveria ser REVIEW, veio %s", r.Verdict)
	}
	if !strings.Contains(r.Reason, "hash") {
		t.Fatalf("REVIEW deveria apontar o hash, veio %q", r.Reason)
	}
}

func TestValidateNoEvidenceReview(t *testing.T) {
	v := NewValidator()
	p := validProposal()
	p.Evidence = Provenance{} // sem evidência
	r := v.Validate(context.Background(), p)
	if r.Verdict != VerdictReview {
		t.Fatalf("proposta sem evidência deveria ser REVIEW, veio %s", r.Verdict)
	}
}

func TestValidateMaxRevisions(t *testing.T) {
	v := NewValidator()
	p := validProposal()
	// MaxRevisions revisões são permitidas; a (Max+1)-ésima nega.
	p.Revisions = v.MaxRevisions + 1
	r := v.Validate(context.Background(), p)
	if r.Verdict != VerdictDeny {
		t.Fatalf("proposta além do limite de revisões deveria ser DENY, veio %s", r.Verdict)
	}
}

func TestValidateNilProposalFailClosed(t *testing.T) {
	v := NewValidator()
	r := v.Validate(context.Background(), nil)
	if r.Verdict != VerdictDeny || !r.IsFailClosed {
		t.Fatalf("proposta nula deveria ser DENY fail-closed, veio %v", r)
	}
}

func TestFlowSubmitApprove(t *testing.T) {
	f := NewFlow(nil, 2*time.Second)
	r := f.Submit(context.Background(), validProposal())
	if r.Verdict != VerdictApprove {
		t.Fatalf("esperava APPROVE no fluxo, veio %s — %s", r.Verdict, r.Reason)
	}
	if r.ProposalID == "" {
		t.Fatal("fluxo deveria gerar ID para a proposta")
	}
}

func TestFlowSubmitTimeoutFailClosed(t *testing.T) {
	// Kernel isolado que nunca responde (validação semântica travada).
	slow := NewValidator()
	slow.ValidateFunc = func(ctx context.Context, p *Proposal) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}
	f := NewFlow(slow, 50*time.Millisecond)
	r := f.Submit(context.Background(), validProposal())
	if r.Verdict != VerdictDeny || !r.IsFailClosed {
		t.Fatalf("timeout deveria gerar DENY fail-closed, veio %v", r)
	}
}

func TestFlowExecuteNeedsDon(t *testing.T) {
	f := NewFlow(nil, 2*time.Second)
	p := validProposal()
	p.Risk = RiskDestructive
	p.State = "com backup completo e rollback planejado"
	r := f.Submit(context.Background(), p)
	if !r.Approved() || !r.NeedsDon {
		t.Fatalf("destrutiva deveria aprovar exigindo Don, veio %v", r)
	}
	// Specialist não pode executar sozinho.
	if _, err := f.Execute(r.ProposalID, "specialist"); err == nil {
		t.Fatal("specialist não deveria executar proposta que exige Don")
	}
	// Don pode.
	if _, err := f.Execute(r.ProposalID, "don"); err != nil {
		t.Fatalf("Don deveria executar: %v", err)
	}
}

func TestFlowExecuteFailClosed(t *testing.T) {
	f := NewFlow(nil, 2*time.Second)
	p := validProposal()
	r := f.Submit(context.Background(), p)
	_ = r
	// Executa proposta que nunca foi aprovada.
	if _, err := f.Execute("P-9999", "don"); err == nil {
		t.Fatal("execução sem veredicto deveria falhar (fail-closed)")
	}
}

func TestFlowReviewLoop(t *testing.T) {
	f := NewFlow(nil, 2*time.Second)

	// Proposta incompleta → REVIEW.
	p := validProposal()
	p.Motive = ""
	r := f.Submit(context.Background(), p)
	if r.Verdict != VerdictReview {
		t.Fatalf("esperava REVIEW, veio %s", r.Verdict)
	}

	// Corrige → APPROVE.
	corr := validProposal()
	v, err := f.Review(r.ProposalID, corr)
	if err != nil {
		t.Fatalf("revisão falhou: %v", err)
	}
	if v.Verdict != VerdictApprove {
		t.Fatalf("esperava APPROVE após correção, veio %s — %s", v.Verdict, v.Reason)
	}
	if v.ProposalID != r.ProposalID {
		t.Fatalf("revisão deveria preservar o ID: %s vs %s", v.ProposalID, r.ProposalID)
	}
}

func TestFlowReviewMaxRevisions(t *testing.T) {
	f := NewFlow(nil, 2*time.Second)

	p := validProposal()
	p.Motive = ""
	r := f.Submit(context.Background(), p)
	if r.Verdict != VerdictReview {
		t.Fatalf("esperava REVIEW inicial, veio %s", r.Verdict)
	}

	// Duas revisões corrigindo por completo são aprovadas.
	for i := 0; i < 2; i++ {
		corr := validProposal()
		v, err := f.Review(r.ProposalID, corr)
		if err != nil {
			t.Fatalf("revisão %d falhou: %v", i+1, err)
		}
		if v.Verdict != VerdictApprove {
			t.Fatalf("revisão %d deveria aprovar, veio %s", i+1, v.Verdict)
		}
	}

	// Proposta que nunca aprende: sempre incompleta.
	f2 := NewFlow(nil, 2*time.Second)
	p2 := validProposal()
	p2.Motive = ""
	r2 := f2.Submit(context.Background(), p2)
	for i := 0; i < f2.validator.MaxRevisions; i++ {
		bad := validProposal()
		bad.Motive = "" // nunca corrige
		v, _ := f2.Review(r2.ProposalID, bad)
		if i < f2.validator.MaxRevisions-1 && v.Verdict != VerdictReview {
			t.Fatalf("revisão %d deveria ser REVIEW (ainda incompleta), veio %s", i+1, v.Verdict)
		}
	}
	v, err := f2.Review(r2.ProposalID, validProposal())
	if err == nil || v.Verdict != VerdictDeny {
		t.Fatalf("proposta que nunca aprende deveria ser DENY no limite, veio %s err=%v", v.Verdict, err)
	}
}

func TestFlowFeedbackFailure(t *testing.T) {
	f := NewFlow(nil, 2*time.Second)
	r := f.Submit(context.Background(), validProposal())
	if _, err := f.Execute(r.ProposalID, "don"); err != nil {
		t.Fatalf("execução falhou: %v", err)
	}
	exec, err := f.Feedback(r.ProposalID, OutcomeFailure, "dados corrompidos pós-migração")
	if err != nil {
		t.Fatalf("feedback falhou: %v", err)
	}
	if !exec.Failed() {
		t.Fatal("feedback deveria marcar falha")
	}
	// Falha deveria gerar registro de revisão obrigatória no audit.
	entries := f.AuditLog()
	found := false
	for _, e := range entries {
		if e.Verdict == VerdictReview && strings.Contains(e.Reason, "FALHA PÓS-EXECUÇÃO") {
			found = true
		}
	}
	if !found {
		t.Fatal("falha pós-execução deveria gerar registro de revisão obrigatória no audit")
	}
}

func TestFlowPendingsQueue(t *testing.T) {
	f := NewFlow(nil, 2*time.Second)
	r1 := f.Submit(context.Background(), validProposal())
	r2 := f.Submit(context.Background(), validProposal())
	if r1.Verdict != VerdictApprove || r2.Verdict != VerdictApprove {
		t.Fatal("ambas deveriam aprovar")
	}
	if len(f.Pendings()) != 2 {
		t.Fatalf("fila deveria ter 2 pendentes, veio %d", len(f.Pendings()))
	}
	if _, err := f.Execute(r1.ProposalID, "don"); err != nil {
		t.Fatalf("execução falhou: %v", err)
	}
	if len(f.Pendings()) != 1 {
		t.Fatalf("fila deveria ter 1 pendente após execução, veio %d", len(f.Pendings()))
	}
}

func TestAuditLogAppendOnly(t *testing.T) {
	l := NewAuditLog()
	l.RecordVerdict(&VerdictResult{ProposalID: "P-0001", Verdict: VerdictApprove, ValidatedAt: time.Now()})
	l.RecordVerdict(&VerdictResult{ProposalID: "P-0002", Verdict: VerdictDeny, Reason: "lei", ValidatedAt: time.Now()})
	entries := l.Entries()
	if len(entries) != 2 {
		t.Fatalf("audit deveria ter 2 entradas, veio %d", len(entries))
	}
	if entries[0].ProposalID != "P-0001" || entries[1].ProposalID != "P-0002" {
		t.Fatal("audit deveria preservar ordem append-only")
	}
}

// TestJudgeMutationFailClosed prova a blindagem A6: um juiz sabotador que
// muta o artefato durante o julgamento gera DENY fail-closed, e o ponteiro
// do chamador permanece intocado (o Kernel isolado julga um snapshot).
func TestJudgeMutationFailClosed(t *testing.T) {
	malicious := NewValidator()
	malicious.ValidateFunc = func(ctx context.Context, p *Proposal) (string, error) {
		p.Action = "apagar o banco de produção" // juiz sabotador
		p.Risk = RiskDestructive
		return "VALIDAR", nil
	}
	f := NewFlow(malicious, time.Second)
	p := validProposal()
	baseAction, baseRisk := p.Action, p.Risk

	v := f.Submit(context.Background(), p)
	if v.Approved() {
		t.Fatal("juiz que mutou o artefato deveria gerar DENY fail-closed")
	}
	if !v.IsFailClosed {
		t.Fatalf("esperava fail-closed, veio: %s", v.String())
	}
	if p.Action != baseAction || p.Risk != baseRisk {
		t.Fatal("ponteiro do chamador foi alterado pelo juiz — snapshot não aplicado")
	}
	if _, err := f.Execute(p.ID, "don"); err == nil {
		t.Fatal("execução não pode acontecer com artefato mutado")
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
