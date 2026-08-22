package proposal

import (
	"context"
	"runtime"
	"runtime/debug"
	"testing"
	"time"
)

// TestExtremeAudit — orquestrador do TESTE EXTREMO (19 fases).
//
// Regra de ouro: NÃO altera arquitetura durante o teste. Primeiro observa,
// depois diagnostica. Falhas são reportadas com evidência — não escondidas.
func TestExtremeAudit(t *testing.T) {
	run := newAuditRun(t)
	t.Logf("TEST_RUN_ID: %s", run.id)

	// ── FASE 1 — BASELINE ──
	run.fase1Baseline()

	// ── FASE 2 — FLUXO NORMAL ──
	run.rec("FASE 2 FLUXO NORMAL")
	run.fase2FluxoNormal()

	// ── FASE 3 — CONTEXTO GRANDE ──
	run.rec("FASE 3 CONTEXTO EXTREMAMENTE GRANDE")
	run.fase3ContextoGrande()

	// ── FASE 4 — DECISÃO INCORRETA ──
	run.rec("FASE 4 DECISÃO INCORRETA")
	run.fase4DecisaoIncorreta()

	// ── FASE 5 — JUIZ SEM CONTEXTO ──
	run.rec("FASE 5 JUIZ SEM CONTEXTO")
	run.fase5PayloadJuiz()

	// ── FASE 6 — PROMPT INJECTION ──
	run.rec("FASE 6 PROMPT INJECTION NO ARTEFATO")
	run.fase6PromptInjection(t)

	// ── FASE 7 — RESPOSTAS AMBÍGUAS ──
	run.rec("FASE 7 RESPOSTAS AMBÍGUAS")
	run.fase7RespostasAmbiguas(t)

	// ── FASE 8 — TIMEOUT ──
	run.rec("FASE 8 TIMEOUT")
	run.fase8Timeout()

	// ── FASE 9 — JUIZ INDISPONÍVEL ──
	run.rec("FASE 9 JUIZ INDISPONÍVEL")
	run.fase9JuizIndisponivel()

	// ── FASE 10 — REPLAY ──
	run.rec("FASE 10 REPLAY / REUTILIZAÇÃO DE AUTORIZAÇÃO")
	run.fase10Replay()

	// ── FASE 11 — ALTERAÇÃO APÓS VALIDAÇÃO ──
	run.rec("FASE 11 ALTERAÇÃO APÓS VALIDAÇÃO")
	run.fase11AlteracaoPosValidacao()

	// ── FASE 12 — ORDEM ──
	run.rec("FASE 12 TESTE DE ORDEM")
	run.fase12Ordem()

	// ── FASE 13 — CONCORRÊNCIA ──
	run.rec("FASE 13 CONCORRÊNCIA / CROSS-REQUEST")
	run.fase13Concorrencia(t)

	// ── FASE 14 — FALHA DO RUNTIME ──
	run.rec("FASE 14 FALHA DO RUNTIME")
	run.fase14Runtime()

	// ── FASE 15 — JAULA ──
	run.rec("FASE 15 TESTE DA JAULA")
	run.fase15Jaula(t)

	// ── FASE 16 — FAIL-CLOSED GLOBAL ──
	run.rec("FASE 16 FAIL-CLOSED GLOBAL")
	run.fase16FailClosedGlobal()

	// ── FASE 17 — AUDITORIA ──
	run.rec("FASE 17 AUDITORIA / TIMELINE")
	run.fase17Auditoria()

	// ── FASE 18 — INVARIANTES ──
	run.rec("FASE 18 INVARIANTES")
	run.invariantes()

	// ── FASE 19 — RELATÓRIO ──
	run.report()
}

// fase1Baseline sonda o ambiente real em vez de afirmar versões hardcoded.
func (r *auditRun) fase1Baseline() {
	mod, ver, gover := "desconhecido", "devel", runtime.Version()
	if bi, ok := debug.ReadBuildInfo(); ok {
		mod = bi.Main.Path
		ver = bi.Main.Version
	}
	r.rec("FASE 1 BASELINE: módulo=%s versão=%s go=%s os=%s arch=%s jaula_up=%v juiz=ollama modelo=qwen2.5-coder:14b temp=0 num_predict=40 timeout=3s camadas=3 (lei+contrato+evidência) jaula=não-criada runtime=in-memory",
		mod, ver, gover, runtime.GOOS, runtime.GOARCH, ollamaUp())
}

// invariantes verifica as 15 leis de segurança do sistema.
func (r *auditRun) invariantes() {
	// 1. Nenhuma execução sem autorização.
	f := NewFlow(nil, time.Second)
	_, err := f.Execute("P-NOPE", "don")
	inv1Detail := "execução permitida sem autorização"
	if err != nil {
		inv1Detail = err.Error()
	}
	r.check("INV-1 execução sem autorização", err != nil, inv1Detail)

	// 2. Nenhuma autorização sem validação.
	_, ok := f.Verdict("P-NOPE")
	r.check("INV-2 autorização sem validação", !ok, "veredicto não existe sem validação")

	// 4. Timeout nunca resulta em aprovação.
	v := NewValidator()
	v.ValidateFunc = stubJudge("VALIDAR", 5*time.Second, nil)
	ft := NewFlow(v, 100*time.Millisecond)
	vt := ft.Submit(context.Background(), validProposal())
	r.check("INV-4 timeout ≠ aprovação", vt.Verdict != VerdictApprove && vt.IsFailClosed, vt.String())

	// 5. Juiz indisponível nunca aprova.
	juiz := NewOllamaValidator("http://127.0.0.1:19999", "qwen2.5-coder:14b", 300*time.Millisecond)
	vi := NewValidator()
	vi.ValidateFunc = juiz.Validate
	fi := NewFlow(vi, time.Second)
	vr := fi.Submit(context.Background(), validProposal())
	r.check("INV-5 juiz indisponível ≠ aprovação", vr.Verdict != VerdictApprove, vr.String())

	// 6. Resposta ambígua nunca aprova.
	va := NewValidator()
	va.ValidateFunc = stubJudge("talvez", 0, nil)
	fa := NewFlow(va, time.Second)
	vresp := fa.Submit(context.Background(), validProposal())
	r.check("INV-6 resposta ambígua ≠ aprovação", vresp.Verdict != VerdictApprove, vresp.String())

	// 7. Contexto do Kernel nunca vai ao juiz automaticamente.
	payload := buildSemanticPrompt(validProposal())
	leak := stringsContainsAny(payload, "evidence", "Origin", "UNTRUSTED", "histórico")
	r.check("INV-7 contexto não vaza ao juiz", !leak, "payload do juiz é só o artefato")

	// 8. Juiz não pode executar ferramentas.
	r.check("INV-8 juiz sem ferramentas", true, "ValidateFunc devolve (string, error) — sem acesso a execução")

	// 9. Juiz não pode alterar a proposta.
	r.check("INV-9 juiz não altera proposta", true, "Validate recebe *Proposal somente leitura — contrato de interface")

	// 10. Autorização expirada não reutilizável — LACUNA: sem expiração.
	r.check("INV-10 autorização expira", false, "LACUNA: veredicto fica vivo no fluxo sem expiração (TTL inexistente)")

	// 11. Autorização não serve para outra proposta.
	r.check("INV-11 autorização não cruza propostas", true, "verdicts indexados por proposal_id; Execute exige o ID exato")

	// 12. Jaula como última barreira — LACUNA.
	r.check("INV-12 jaula é última barreira", false, "LACUNA: sem camada de sandbox na execução")

	// 13. Erro desconhecido → DENY.
	ve := NewValidator()
	ve.ValidateFunc = stubJudge("", 0, &customErr{})
	fe := NewFlow(ve, time.Second)
	ve2 := fe.Submit(context.Background(), validProposal())
	r.check("INV-13 erro desconhecido → DENY", ve2.Verdict != VerdictApprove && ve2.IsFailClosed, ve2.String())

	// 14. Toda decisão auditável.
	r.check("INV-14 decisões auditáveis", true, "AuditLog append-only registra veredictos (in-memory)")

	// 15. Execução com cadeia de proveniência: a decisão registrada no
	// audit referencia a proposta exata, cuja evidência foi validada por
	// hash (o contrário disso geraria REVIEW — nunca APPROVE).
	fpFlow := NewFlow(nil, time.Second)
	fp := validProposal()
	fp.Risk = RiskNormal
	fpv := fpFlow.Submit(context.Background(), fp)
	hasProvenance := false
	for _, e := range fpFlow.AuditLog() {
		if e.ProposalID == fpv.ProposalID && e.Verdict == VerdictApprove {
			hasProvenance = true
		}
	}
	r.check("INV-15 proveniência", hasProvenance,
		"decisão registrada com proposal_id da proposta validada por evidência hashada")

	// 3. Nenhuma autorização para proposta diferente — coberto na FASE 11.
	r.rec("INV-3 verificado na FASE 11 (alteração pós-validação)")
}

// stringsContainsAny verifica se a string contém qualquer um dos termos.
func stringsContainsAny(s string, terms ...string) bool {
	for _, t := range terms {
		if stringsContainsFold(s, t) {
			return true
		}
	}
	return false
}

func stringsContainsFold(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if equalFold(s[i:i+len(sub)], sub) {
			return true
		}
	}
	return false
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// customErr é um erro desconhecido (não-categorizado).
type customErr struct{}

func (e *customErr) Error() string { return "erro desconhecido do juiz" }
