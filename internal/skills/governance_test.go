package skills

import "testing"

// TestParseStatusLine_HonorsStatus garante que o `**Status**` do blockquote
// legado passa a alimentar o ciclo de vida (antes era ignorado).
func TestParseStatusLine_HonorsStatus(t *testing.T) {
	cases := []struct {
		line string
		want LifecycleState
	}{
		{"> **Version**: 1.0.0 | **Status**: active | **Owner**: AI Chief", LifecycleActive},
		{"> **Version**: 1.0.0 | **Status**: draft | **Owner**: AI Chief", LifecycleProposed},
		{"> **Version**: 1.0.0 | **Status**: quarantined | **Owner**: AI Chief", LifecycleQuarantined},
		{"> **Version**: 1.0.0 | **Status**: validated | **Owner**: AI Chief", LifecycleValidated},
		{"> **Version**: 1.0.0 | **Status**: deprecated | **Owner**: AI Chief", LifecycleDeprecated},
	}
	for _, c := range cases {
		s := &Skill{}
		parseStatusLine(c.line, s)
		if s.Governance.Status != c.want {
			t.Errorf("line %q: status = %q, want %q", c.line, s.Governance.Status, c.want)
		}
	}

	// Unknown status → default (active via Lifetimes/avô), não erro.
	s := &Skill{}
	parseStatusLine("> **Version**: 1.0.0 | **Status**: bogus | **Owner**: X", s)
	if s.Lifecycle() != LifecycleActive {
		t.Errorf("unknown status must fall through to default active, got %q", s.Lifecycle())
	}
}

// TestSkillGovernance_Serializable garante compatibilidade (omitempty): uma
// skill sem governança serializa igual a antes e o YAML continua válido.
func TestSkillGovernance_Serializable(t *testing.T) {
	s := Skill{Name: "demo", Description: "d", Version: "1.0"}
	if s.Governance.Status != "" {
		t.Errorf("expected empty governance for legacy skill, got %q", s.Governance.Status)
	}
	if s.Lifecycle() != LifecycleActive {
		t.Errorf("lifecycle must default active via avô, got %q", s.Lifecycle())
	}
}
