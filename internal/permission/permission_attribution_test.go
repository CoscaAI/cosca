package permission

import (
	"reflect"
	"testing"
)

// ─── EvaluateAttributed: provenance of the winning rule ────────────────────────

func TestEvaluateAttributed_LastLayerWinsTie(t *testing.T) {
	// Equal specificity → the LATER layer in pass order wins, and is the
	// provenance of the verdict.
	global := NamedRuleset{Layer: LayerGlobal, Rules: Ruleset{
		{Permission: "edit", Pattern: "*.go", Action: Deny},
	}}
	workspace := NamedRuleset{Layer: LayerWorkspace, Rules: Ruleset{
		{Permission: "edit", Pattern: "*.go", Action: Allow},
	}}

	v := EvaluateAttributed("edit", "main.go", global, workspace)
	if v.Rule.Action != Allow {
		t.Fatalf("expected allow (last layer wins tie), got %s", v.Rule.Action)
	}
	if v.Layer != LayerWorkspace {
		t.Fatalf("expected source layer %q, got %q", LayerWorkspace, v.Layer)
	}
}

func TestEvaluateAttributed_MostSpecificWinsAcrossLayers(t *testing.T) {
	// Specificity beats merge-order: the allow for *.go lives in the GLOBAL layer
	// while the workspace only has a coarse wildcard deny. For a .go file the
	// more-specific GLOBAL rule wins, so provenance is global even though the
	// workspace is the higher-precedence layer.
	global := NamedRuleset{Layer: LayerGlobal, Rules: Ruleset{
		{Permission: "edit", Pattern: "*.go", Action: Allow},
	}}
	workspace := NamedRuleset{Layer: LayerWorkspace, Rules: Ruleset{
		{Permission: "edit", Pattern: "*", Action: Deny},
	}}

	v := EvaluateAttributed("edit", "main.go", workspace, global)
	if v.Rule.Action != Allow {
		t.Fatalf("expected allow (most specific wins), got %s", v.Rule.Action)
	}
	if v.Layer != LayerGlobal {
		t.Fatalf("expected source layer %q, got %q", LayerGlobal, v.Layer)
	}
}

func TestEvaluateAttributed_MostSpecificTieBreakUsesLaterLayer(t *testing.T) {
	// Two equally-specific rules across two layers → later layer is the source.
	global := NamedRuleset{Layer: LayerGlobal, Rules: Ruleset{
		{Permission: "edit", Pattern: "*.go", Action: Deny},
	}}
	workspace := NamedRuleset{Layer: LayerWorkspace, Rules: Ruleset{
		{Permission: "edit", Pattern: "*.go", Action: Allow},
	}}
	v := EvaluateAttributed("edit", "main.go", global, workspace)
	if v.Layer != LayerWorkspace {
		t.Errorf("expected workspace layer to win tie, got %q", v.Layer)
	}
}

func TestEvaluateAttributed_DefaultIsAskAtLayerDefault(t *testing.T) {
	// No rules at all (fail-safe) → ask attributed to the engine default layer.
	v := EvaluateAttributed("bash", "*")
	if v.Rule.Action != Ask {
		t.Errorf("expected ask default, got %s", v.Rule.Action)
	}
	if v.Layer != LayerDefault {
		t.Errorf("expected layer %q for unconfigured rule, got %q", LayerDefault, v.Layer)
	}

	// Non-matching rules across layers → still fail-safe ask @ default.
	global := NamedRuleset{Layer: LayerGlobal, Rules: Ruleset{
		{Permission: "shell", Pattern: "*", Action: Allow},
	}}
	v = EvaluateAttributed("bash", "*", global)
	if v.Rule.Action != Ask || v.Layer != LayerDefault {
		t.Errorf("expected ask @ default, got %s @ %q", v.Rule.Action, v.Layer)
	}
}

func TestEvaluateAttributed_EmptyLayerFallsBackToDefault(t *testing.T) {
	v := EvaluateAttributed("read", "*", NamedRuleset{Rules: Ruleset{
		{Permission: "read", Pattern: "*", Action: Allow},
	}})
	if v.Rule.Action != Allow {
		t.Errorf("expected allow, got %s", v.Rule.Action)
	}
	if v.Layer != LayerDefault {
		t.Errorf("expected empty layer normalized to %q, got %q", LayerDefault, v.Layer)
	}
}

func TestEvaluate_DelegationMatchesEvaluateAttributed(t *testing.T) {
	// Evaluate (the backwards-compatible API) must return exactly the same rule
	// that EvaluateAttributed would, with provenance dropped.
	global := Rule{Permission: "edit", Pattern: "*.go", Action: Allow}

	got := Evaluate("edit", "main.go",
		Ruleset{{Permission: "edit", Pattern: "*", Action: Deny}},
		Ruleset{{Permission: "edit", Pattern: "*.go", Action: Allow}},
	)
	if got != global {
		t.Errorf("Evaluate got %v, want %v", got, global)
	}

	v := EvaluateAttributed("edit", "main.go",
		NamedRuleset{Layer: LayerWorkspace, Rules: Ruleset{{Permission: "edit", Pattern: "*", Action: Deny}}},
		NamedRuleset{Layer: LayerAgent, Rules: Ruleset{{Permission: "edit", Pattern: "*.go", Action: Allow}}},
	)
	if v.Rule != got {
		t.Errorf("EvaluateAttributed.Rule = %v, Evaluate = %v (must be identical)", v.Rule, got)
	}
}

func TestVerdict_String(t *testing.T) {
	v := Verdict{Rule: Rule{Permission: "edit", Pattern: "*.go", Action: Allow}, Layer: LayerWorkspace}
	if got, want := v.String(), "edit:*.go allow @workspace"; got != want {
		t.Errorf("Verdict.String got %q, want %q", got, want)
	}
}

// ─── Summarize: per-key winning rule + layer + exceptions ─────────────────────

func TestSummarize_PerKeyWinningRuleAndSourceLayer(t *testing.T) {
	global := NamedRuleset{Layer: LayerGlobal, Rules: Ruleset{
		{Permission: "*", Pattern: "*", Action: Ask},
		{Permission: "shell", Pattern: "*", Action: Deny},
		{Permission: "edit", Pattern: "*", Action: Ask},
	}}
	workspace := NamedRuleset{Layer: LayerWorkspace, Rules: Ruleset{
		{Permission: "read", Pattern: "*", Action: Allow},
		{Permission: "edit", Pattern: "*.go", Action: Allow},
	}}

	got := Summarize(nil, global, workspace)

	byKey := make(map[ActionKey]KeySummary, len(got))
	for _, ks := range got {
		byKey[ks.Key] = ks
	}

	// shell: only global has it → deny @ global.
	if ks := byKey[KeyShell]; ks.Verdict.Rule.Action != Deny || ks.Verdict.Layer != LayerGlobal {
		t.Errorf("shell: got %s @ %q, want deny @ global", ks.Verdict.Rule.Action, ks.Verdict.Layer)
	}
	// read: only workspace has an explicit rule → allow @ workspace (overrides global ask).
	if ks := byKey[KeyRead]; ks.Verdict.Rule.Action != Allow || ks.Verdict.Layer != LayerWorkspace {
		t.Errorf("read: got %s @ %q, want allow @ workspace", ks.Verdict.Rule.Action, ks.Verdict.Layer)
	}
	// edit: wildcard ask @ global; the *.go allow is an exception @ workspace.
	if ks := byKey[KeyEdit]; ks.Verdict.Rule.Action != Ask || ks.Verdict.Layer != LayerGlobal {
		t.Errorf("edit: got %s @ %q, want ask @ global", ks.Verdict.Rule.Action, ks.Verdict.Layer)
	}
	// web: no dedicated rule, but the global catch-all {*,*,ask} governs it →
	// ask attributed to global (even a catch-all rule carries its layer).
	if ks := byKey[KeyWeb]; ks.Verdict.Rule.Action != Ask || ks.Verdict.Layer != LayerGlobal {
		t.Errorf("web: got %s @ %q, want ask @ global", ks.Verdict.Rule.Action, ks.Verdict.Layer)
	}

	// Default key set is returned when keys is empty.
	if len(got) != len(DefaultActionKeys) {
		t.Errorf("expected %d default keys, got %d", len(DefaultActionKeys), len(got))
	}
}

func TestSummarize_ExceptionsAttributed(t *testing.T) {
	global := NamedRuleset{Layer: LayerGlobal, Rules: Ruleset{
		{Permission: "edit", Pattern: "*", Action: Ask},
	}}
	workspace := NamedRuleset{Layer: LayerWorkspace, Rules: Ruleset{
		{Permission: "edit", Pattern: "*.go", Action: Allow},
		{Permission: "edit", Pattern: "docs/**", Action: Deny},
	}}

	got := Summarize([]ActionKey{KeyEdit}, global, workspace)
	if len(got) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(got))
	}
	ks := got[0]
	if ks.Verdict.Rule.Action != Ask || ks.Verdict.Layer != LayerGlobal {
		t.Errorf("verdict want ask @ global, got %s @ %q", ks.Verdict.Rule.Action, ks.Verdict.Layer)
	}

	wantExceptions := []Exception{
		{Rule: Rule{Permission: "edit", Pattern: "*.go", Action: Allow}, Layer: LayerWorkspace},
		{Rule: Rule{Permission: "edit", Pattern: "docs/**", Action: Deny}, Layer: LayerWorkspace},
	}
	if !reflect.DeepEqual(ks.Exceptions, wantExceptions) {
		t.Errorf("exceptions got %v, want %v", ks.Exceptions, wantExceptions)
	}
}

func TestSummarize_OverrideKeesAndOrderPreserved(t *testing.T) {
	global := NamedRuleset{Layer: LayerGlobal, Rules: Ruleset{
		{Permission: "shell", Pattern: "*", Action: Allow},
	}}
	keys := []ActionKey{KeyMCP, KeyShell}
	got := Summarize(keys, global)
	if len(got) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(got))
	}
	// Order is preserved as given.
	if got[0].Key != KeyMCP || got[1].Key != KeyShell {
		t.Errorf("keys order not preserved: %v", got)
	}
	// mcp is unconfigured → fail-safe ask @ default.
	if got[0].Verdict.Layer != LayerDefault {
		t.Errorf("mcp layer want %q, got %q", LayerDefault, got[0].Verdict.Layer)
	}
	// shell is allow @ global.
	if got[1].Verdict.Rule.Action != Allow || got[1].Verdict.Layer != LayerGlobal {
		t.Errorf("shell want allow @ global, got %s @ %q", got[1].Verdict.Rule.Action, got[1].Verdict.Layer)
	}
}
