package skills

import "testing"

func TestLifecycleCanTransition_Allowed(t *testing.T) {
	cases := []struct {
		from, to LifecycleState
	}{
		{LifecycleProposed, LifecycleQuarantined},
		{LifecycleQuarantined, LifecycleValidated},   // gate pass
		{LifecycleQuarantined, LifecycleDeprecated},  // gate fail (discard)
		{LifecycleValidated, LifecycleActive},        // PR merge
		{LifecycleActive, LifecycleDeprecated},       // aging/regression
		{LifecycleActive, LifecycleActive},           // supersede (no-op self)
		{LifecycleDeprecated, LifecycleActive},       // rollback/restore
	}
	for _, c := range cases {
		if !CanTransition(c.from, c.to) {
			t.Errorf("expected %s -> %s allowed", c.from, c.to)
		}
	}
	// from == to no-op
	if !CanTransition(LifecycleProposed, LifecycleProposed) {
		t.Error("expected self no-op allowed")
	}
}

func TestLifecycleCanTransition_Forbidden(t *testing.T) {
	cases := []struct {
		from, to LifecycleState
	}{
		{LifecycleProposed, LifecycleValidated},  // pula quarentena+gate (I2)
		{LifecycleProposed, LifecycleActive},     // pula gate (I1/I2)
		{LifecycleState("bogus"), LifecycleQuarantined}, // inválido
		{LifecycleValidated, LifecycleQuarantined}, // não re-entra
		{LifecycleActive, LifecycleProposed},     // não regride a rascunho
		{LifecycleDeprecated, LifecycleValidated}, // re-aplicar = nova proposta
	}
	for _, c := range cases {
		if CanTransition(c.from, c.to) {
			t.Errorf("expected %s -> %s FORBIDDEN (I1/I2/I6)", c.from, c.to)
		}
	}
}

func TestLifecycleInvalid(t *testing.T) {
	if ValidLifecycle(LifecycleState("bogus")) {
		t.Error("expected bogus lifecycle invalid")
	}
	if !ValidLifecycle(LifecycleActive) {
		t.Error("expected active valid")
	}
	if DefaultLifecycle() != LifecycleActive {
		t.Error("default lifecycle must be active (cláusula de avô)")
	}
	if !IsTerminalLifecycle(LifecycleDeprecated) {
		t.Error("deprecated is terminal")
	}
	if IsActiveLifecycle(LifecycleDeprecated) {
		t.Error("deprecated is not active")
	}
	if !IsActiveLifecycle(LifecycleActive) {
		t.Error("active is active")
	}
}

func TestSkillLifecycleMethods(t *testing.T) {
	var s Skill
	if s.Lifecycle() != LifecycleActive {
		t.Errorf("empty governance must default to active (avô), got %q", s.Lifecycle())
	}
	if err := s.SetLifecycle(LifecycleProposed); err != nil {
		t.Fatalf("SetLifecycle proposed: %v", err)
	}
	if s.Lifecycle() != LifecycleProposed {
		t.Errorf("expected proposed, got %q", s.Lifecycle())
	}
	if err := s.SetLifecycle(LifecycleState("nope")); err == nil {
		t.Error("expected error for invalid lifecycle")
	}
}
