package kernel

import "testing"

// TestEpistemology verifies the 5 epistemic principles are present and complete.
func TestEpistemology(t *testing.T) {
	if len(Epistemology) != 5 {
		t.Fatalf("expected 5 epistemic principles, got %d", len(Epistemology))
	}
	for _, p := range Epistemology {
		if p.Number == 0 || p.Title == "" || p.Rule == "" || p.Guardian == "" {
			t.Errorf("epistemic principle incomplete: %+v", p)
		}
	}
	// Principles must be numbered 1..5 in order.
	for i, p := range Epistemology {
		if p.Number != i+1 {
			t.Errorf("principle %d has number %d, want %d", i, p.Number, i+1)
		}
	}
	// Guardians must map to the governing agents (per the Don's order).
	guardians := []string{"cosca-kernel", "cosca-kernel", "cosca-evidence", "cosca-memory-chief", "cosca-critic"}
	for i, want := range guardians {
		if Epistemology[i].Guardian != want {
			t.Errorf("principle %d guardian = %q, want %q", Epistemology[i].Number, Epistemology[i].Guardian, want)
		}
	}
}

// TestEpistemicPrincipleCount verifies the count helper returns 5.
func TestEpistemicPrincipleCount(t *testing.T) {
	if got := EpistemicPrincipleCount(); got != 5 {
		t.Errorf("EpistemicPrincipleCount() = %d, want 5", got)
	}
}

// TestSelfTestIncludesEpistemology verifies the SelfTest reports the new check.
func TestSelfTestIncludesEpistemology(t *testing.T) {
	found := false
	for _, c := range SelfTest() {
		if c.Name == "epistemology" {
			found = true
			if !c.OK {
				t.Errorf("epistemology check not OK: %s", c.Detail)
			}
		}
	}
	if !found {
		t.Error("SelfTest missing 'epistemology' check")
	}
}
