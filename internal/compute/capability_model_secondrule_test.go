package compute

import (
	"testing"
)

// =============================================================================
// F7 — SEGUNDA REGRA DE OURO (L325): nenhuma evidência altera o mecanismo
// que determina se ela é válida + UNKNOWN explicável + falha é evidência
// =============================================================================

func TestCapability_RecordFailure(t *testing.T) {
	// Falha derruba a certeza anterior e registra o motivo.
	c := Capability{Name: "compute.gpu.rocm", Status: StatusUsable, Evidence: "era true"}
	c.RecordFailure("rocminfo falhou: device não acessível")

	if c.Status != StatusUnverified {
		t.Errorf("status = %q, want unverified após falha", c.Status)
	}
	if c.WhyUnknown != WhyProbeFailed {
		t.Errorf("why_unknown = %q, want probe_failed", c.WhyUnknown)
	}
	if c.FailedEvidence == "" {
		t.Error("failed_evidence deveria preservar o motivo (L325: falha é evidência)")
	}
}

func TestCapability_RecordFailure_DoesNotPromote(t *testing.T) {
	// Falha registrada em capability unknown → continua unknown com razão.
	c := Capability{Name: "x", Status: StatusUnknown}
	c.RecordFailure("sonda não executou")
	if c.Status != StatusUnverified {
		t.Errorf("status = %q, want unverified", c.Status)
	}
	if c.WhyUnknown != WhyProbeFailed {
		t.Errorf("why_unknown = %q, want probe_failed", c.WhyUnknown)
	}
}

func TestCapability_EvidenceSufficient_Impact(t *testing.T) {
	// L325 princípio 7: evidência proporcional ao impacto.
	// High impact (executar código) exige verified/usable — reported NÃO basta.
	reported := Capability{Name: "shell.exec", Status: StatusReported}
	if reported.EvidenceSufficient(ImpactHigh) {
		t.Error("reported NÃO deveria ser suficiente para high impact")
	}
	if reported.EvidenceSufficient(ImpactLow) {
		t.Error("reported NÃO deveria ser suficiente nem para low — para USAR, o runtime precisa ao menos VER (visible), não só 'alguém declarou'")
	}

	visible := Capability{Name: "shell.exec", Status: StatusVisible}
	if !visible.EvidenceSufficient(ImpactLow) {
		t.Error("visible deveria ser suficiente para low impact (o runtime observa)")
	}
	if visible.EvidenceSufficient(ImpactHigh) {
		t.Error("visible NÃO deveria ser suficiente para high impact (só ver, não verificou)")
	}

	verified := Capability{Name: "shell.exec", Status: StatusVerified}
	if !verified.EvidenceSufficient(ImpactHigh) {
		t.Error("verified deveria ser suficiente para high impact (verificação real)")
	}

	// Medium exige measured+.
	measured := Capability{Name: "x", Status: StatusMeasured}
	if !measured.EvidenceSufficient(ImpactMedium) {
		t.Error("measured deveria ser suficiente para medium")
	}
}

func TestCapability_WhyUnknown_Explanable(t *testing.T) {
	// UNKNOWN é primeira classe e explicável (L325 princípio 6 e 10).
	c := Capability{Name: "compute.gpu.compute", Status: StatusUnknown, WhyUnknown: WhyEnvironmentMismatch}
	if c.Status != StatusUnknown {
		t.Error("status deveria ser unknown (não forçar false)")
	}
	if c.WhyUnknown != WhyEnvironmentMismatch {
		t.Errorf("why_unknown = %q, want environment_mismatch", c.WhyUnknown)
	}
	// UNKNOWN ≠ FALSE: um unknown não pode ser usado como se fosse false
	// (a capability não diz "não existe" — diz "não sei").
	if c.Reported {
		t.Error("unknown não implica reported")
	}
}

func TestCapability_ProvenanceImmutable(t *testing.T) {
	// L325 princípio 3: provenance é imutável — se mudou, INVALIDATE + nova,
	// nunca editar a evidência antiga.
	c := Capability{Name: "rocm", Status: StatusUsable, Evidence: "evidência original"}
	c.Invalidate("driver mudou — evidência original não é mais válida")
	if c.Status != StatusInvalidated {
		t.Errorf("status = %q, want invalidated (nunca editar a antiga)", c.Status)
	}
	// A evidência antiga NÃO foi reescrita com a nova — foi marcada inválida.
	if c.Evidence == "" {
		t.Error("a evidência original deveria permanecer visível (imutável)")
	}
}
