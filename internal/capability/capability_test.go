package capability

import (
	"reflect"
	"strings"
	"testing"
)

func TestLevel_String(t *testing.T) {
	cases := []struct {
		level Level
		want  string
	}{
		{Level0Deterministic, "L0-determinístico"},
		{Level1Retrieval, "L1-retrieval"},
		{Level2Reasoning, "L2-raciocínio"},
		{Level3Autonomous, "L3-autônomo"},
	}
	for _, tc := range cases {
		if got := tc.level.String(); got != tc.want {
			t.Errorf("%v.String() = %q, want %q", tc.level, got, tc.want)
		}
	}
}

func TestLevel_Description(t *testing.T) {
	cases := []struct {
		level Level
		sub   string
	}{
		{Level0Deterministic, "determinístico"},
		{Level1Retrieval, "Retrieval"},
		{Level2Reasoning, "Raciocínio"},
		{Level3Autonomous, "autônoma"},
	}
	for _, tc := range cases {
		if got := tc.level.Description(); got == "" {
			t.Errorf("%v.Description() is empty", tc.level)
		} else if !strings.Contains(got, tc.sub) {
			t.Errorf("%v.Description() = %q, want it to contain %q", tc.level, got, tc.sub)
		}
	}
}

func TestCurrentLevel(t *testing.T) {
	cases := []struct {
		name       string
		provider   string
		hasRuntime bool
		want       Level
	}{
		{"empty provider is deterministic", "", false, Level0Deterministic},
		{"spaces provider is deterministic", "   ", false, Level0Deterministic},
		{"none provider is deterministic", "none", false, Level0Deterministic},
		{"NONE provider is deterministic", "NONE", false, Level0Deterministic},
		{"embeddings-only provider stays at retrieval", "local", false, Level1Retrieval},
		{"ollama is reasoning", "ollama", false, Level2Reasoning},
		{"deepseek is reasoning", "deepseek", false, Level2Reasoning},
		{"openai is reasoning", "openai", false, Level2Reasoning},
		{"anthropic is reasoning", "anthropic", false, Level2Reasoning},
		{"mixed case provider", "Ollama", false, Level2Reasoning},
		{"ollama with runtime is autonomous", "ollama", true, Level3Autonomous},
		{"deepseek with runtime is autonomous", "deepseek", true, Level3Autonomous},
		{"local with runtime is autonomous", "local", true, Level3Autonomous},
		{"none with runtime stays deterministic", "none", true, Level0Deterministic},
		{"empty with runtime stays deterministic", "", true, Level0Deterministic},
		{"unknown provider defaults to retrieval", "mystery-provider", false, Level1Retrieval},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CurrentLevel(tc.provider, tc.hasRuntime); got != tc.want {
				t.Errorf("CurrentLevel(%q, %v) = %v (%s), want %v (%s)",
					tc.provider, tc.hasRuntime, got, got.String(), tc.want, tc.want.String())
			}
		})
	}
}

func TestCapabilities(t *testing.T) {
	l0 := []string{"audit", "backup", "knowledge", "laws", "memory", "security", "workflow"}
	l1 := []string{"audit", "backup", "classification", "knowledge", "laws", "memory", "security", "semantic_search", "workflow"}
	l2 := []string{"audit", "backup", "classification", "code_gen", "knowledge", "laws", "memory", "planning", "reasoning", "security", "semantic_search", "workflow"}
	l3 := []string{"audit", "autonomous_execution", "backup", "classification", "code_gen", "knowledge", "laws", "memory", "planning", "reasoning", "security", "semantic_search", "workflow"}

	if got := Capabilities(Level0Deterministic); !reflect.DeepEqual(got, l0) {
		t.Errorf("Capabilities(L0) = %v, want %v", got, l0)
	}
	if got := Capabilities(Level1Retrieval); !reflect.DeepEqual(got, l1) {
		t.Errorf("Capabilities(L1) = %v, want %v", got, l1)
	}
	if got := Capabilities(Level2Reasoning); !reflect.DeepEqual(got, l2) {
		t.Errorf("Capabilities(L2) = %v, want %v", got, l2)
	}
	if got := Capabilities(Level3Autonomous); !reflect.DeepEqual(got, l3) {
		t.Errorf("Capabilities(L3) = %v, want %v", got, l3)
	}
}

func TestCapabilities_Level0HasBasics(t *testing.T) {
	caps := Capabilities(Level0Deterministic)
	for _, want := range []string{"knowledge", "laws", "audit", "memory", "security", "workflow", "backup"} {
		if !contains(caps, want) {
			t.Errorf("L0 should include %q, got %v", want, caps)
		}
	}
}

func TestCapabilities_Level3IsSupersetOfLevel0(t *testing.T) {
	l0 := make(map[string]bool)
	for _, c := range Capabilities(Level0Deterministic) {
		l0[c] = true
	}
	l3 := Capabilities(Level3Autonomous)
	for c := range l0 {
		if !contains(l3, c) {
			t.Errorf("L3 must be a superset of L0; missing %q", c)
		}
	}
	if !contains(l3, "autonomous_execution") {
		t.Errorf("L3 should include autonomous_execution")
	}
}

func TestCapabilities_StrictlyIncreasing(t *testing.T) {
	for i := Level0Deterministic; i < Level3Autonomous; i++ {
		lower := Capabilities(i)
		higher := Capabilities(i + 1)
		if len(higher) <= len(lower) {
			t.Errorf("L%d should have more capabilities than L%d", i+1, i)
		}
	}
}

func TestUnavailableCapabilities(t *testing.T) {
	missing := UnavailableCapabilities(Level0Deterministic)
	for _, m := range []string{"semantic_search", "classification", "reasoning", "planning", "code_gen", "autonomous_execution"} {
		if !contains(missing, m) {
			t.Errorf("L0 should NOT have %q available; unavailable list %v", m, missing)
		}
	}
	if contains(missing, "knowledge") {
		t.Errorf("L0 should have knowledge available; unavailable list %v", missing)
	}

	if got := UnavailableCapabilities(Level3Autonomous); len(got) != 0 {
		t.Errorf("L3 should have every capability available; got unavailable %v", got)
	}
}

func TestAllCapabilities(t *testing.T) {
	all := AllCapabilities()
	if len(all) != 13 {
		t.Errorf("expected 13 total capabilities, got %d: %v", len(all), all)
	}
	for _, c := range []string{"knowledge", "autonomous_execution", "reasoning", "semantic_search"} {
		if !contains(all, c) {
			t.Errorf("AllCapabilities should include %q", c)
		}
	}
}

func contains(s []string, sub string) bool {
	for _, v := range s {
		if v == sub {
			return true
		}
	}
	return false
}
