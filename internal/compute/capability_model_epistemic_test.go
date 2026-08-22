package compute

import (
	"testing"
	"time"
)

// =============================================================================
// F7 — REGRA DE OURO (L324): estado inferior não promove a si próprio
// =============================================================================

func TestCapability_Promote_InferredToMeasured(t *testing.T) {
	c := Capability{Name: "compute.cpu.avx2", Status: StatusInferred, Source: "arch-detector"}
	if err := c.Promote(StatusMeasured, "/proc/cpuinfo contém avx2"); err != nil {
		t.Fatalf("promote inferred→measured: %v", err)
	}
	if c.Status != StatusMeasured {
		t.Errorf("status = %q, want measured", c.Status)
	}
	if c.Evidence != "/proc/cpuinfo contém avx2" {
		t.Errorf("evidence não atualizada: %q", c.Evidence)
	}
}

func TestCapability_Promote_Chain(t *testing.T) {
	// A cadeia completa: inferred → measured → verified → usable.
	c := Capability{Name: "avx2", Status: StatusInferred}
	mustPromote := func(to CapabilityStatus, ev string) {
		if err := c.Promote(to, ev); err != nil {
			t.Fatalf("promote %q: %v", to, err)
		}
	}
	mustPromote(StatusMeasured, "flags observados")
	mustPromote(StatusVerified, "instrução executada com sucesso")
	mustPromote(StatusUsable, "runtime confirmou uso")

	if c.Status != StatusUsable {
		t.Errorf("status final = %q, want usable", c.Status)
	}
}

func TestCapability_Promote_SkipForbidden(t *testing.T) {
	// REGRA DE OURO: inferred NÃO pode pular direto para usable.
	c := Capability{Name: "avx2", Status: StatusInferred}
	if err := c.Promote(StatusUsable, "acho que funciona"); err == nil {
		t.Fatal("inferred → usable deveria ser PROIBIDO (precisa passar por measured/verified)")
	}
	if c.Status != StatusInferred {
		t.Errorf("status mudou sem permissão: %q", c.Status)
	}
}

func TestCapability_Promote_RequiresEvidence(t *testing.T) {
	c := Capability{Name: "avx2", Status: StatusInferred}
	if err := c.Promote(StatusMeasured, ""); err == nil {
		t.Fatal("promote sem evidência nova deveria falhar (L324)")
	}
}

func TestCapability_Promote_MeasuredToVerified(t *testing.T) {
	c := Capability{Name: "gpu", Status: StatusMeasured}
	if err := c.Promote(StatusVerified, "compute shader executou"); err != nil {
		t.Fatalf("promote measured→verified: %v", err)
	}
	// measured NÃO pode virar usable direto (pula verified).
	c2 := Capability{Name: "gpu", Status: StatusMeasured}
	if err := c2.Promote(StatusUsable, "direto"); err == nil {
		t.Fatal("measured → usable direto deveria ser PROIBIDO")
	}
}

// =============================================================================
// F7 — INVALIDATED e TEMPORALIDADE (L324)
// =============================================================================

func TestCapability_Invalidate(t *testing.T) {
	c := Capability{Name: "compute.gpu.rocm", Status: StatusUsable, Evidence: "era true"}
	c.Invalidate("driver mudou")
	if c.Status != StatusInvalidated {
		t.Errorf("status = %q, want invalidated", c.Status)
	}
	// Um INVALIDATED não pode ser promovido (precisa re-observar do zero).
	if err := c.Promote(StatusMeasured, "nova evidência"); err == nil {
		t.Error("invalidated não deveria aceitar promote — precisa re-observação")
	}
}

func TestCapability_IsStale(t *testing.T) {
	now := time.Now().UTC()
	c := Capability{Name: "x", Status: StatusUsable, ObservedAt: now.Add(-time.Hour), ExpiresAt: now.Add(-time.Minute)}
	if !c.IsStale(now) {
		t.Error("expired capability should be stale")
	}
	c2 := Capability{Name: "y", Status: StatusUsable, ObservedAt: now, ExpiresAt: now.Add(time.Hour)}
	if c2.IsStale(now) {
		t.Error("non-expired capability should not be stale")
	}
	c3 := Capability{Name: "z", Status: StatusUsable} // sem expiração
	if c3.IsStale(now) {
		t.Error("capability without expiry should not be stale")
	}
}

func TestCapability_DerivedFrom(t *testing.T) {
	// Herança explícita (L324 cuidado 12): inferência registra a origem.
	c := Capability{
		Name: "compute.cpu.avx2", Status: StatusInferred,
		Source: "arch-detector", Evidence: "GOARCH=amd64",
		DerivedFrom: "compute.cpu.arch", InferenceRule: "amd64-implies-avx2",
	}
	if c.DerivedFrom != "compute.cpu.arch" || c.InferenceRule != "amd64-implies-avx2" {
		t.Errorf("herança não registrada: %+v", c)
	}
}
