package deliberate

// SPEC validation (ADR-011 A7): the synthesis is an actionable SPEC, never a
// narrative summary. ValidateSpec enforces the FINDINGS / CROSS-AGENT / SPEC /
// ACTIONS / CONFIDENCE sections and the completeness of every SpecItem.

import (
	"fmt"
	"strings"
)

// ValidateSpec validates a SynthesisSpec against the A7 actionable-SPEC
// contract. It returns a descriptive error on the first violation found.
//
// Required sections (each must be present/non-empty):
//   - FINDINGS:   at least one finding.
//   - CROSS-AGENT: at least one non-owner cross-reviewer recorded.
//   - SPEC:       at least one SpecItem.
//   - ACTIONS:    at least one concrete action.
//   - CONFIDENCE: a numeric confidence in (0, 1] (never intuitive text, never 0).
//
// Every SpecItem must have File, Action, Metric and Acceptance non-empty.
// Vague sections (empty, whitespace-only) are rejected.
func ValidateSpec(spec SynthesisSpec) error {
	if len(spec.Findings) == 0 || strings.TrimSpace(spec.Findings[0]) == "" {
		return fmt.Errorf("deliberate spec: FINDINGS section is required (at least one finding)")
	}
	if len(spec.CrossAgent) == 0 || strings.TrimSpace(spec.CrossAgent[0]) == "" {
		return fmt.Errorf("deliberate spec: CROSS-AGENT section is required (at least one non-owner reviewer)")
	}
	if len(spec.Spec) == 0 {
		return fmt.Errorf("deliberate spec: SPEC section is required (at least one SpecItem)")
	}
	if len(spec.Actions) == 0 || strings.TrimSpace(spec.Actions[0]) == "" {
		return fmt.Errorf("deliberate spec: ACTIONS section is required (at least one action)")
	}
	if spec.Confidence <= 0 || spec.Confidence > 1 {
		return fmt.Errorf("deliberate spec: CONFIDENCE must be in (0,1], got %v", spec.Confidence)
	}

	for i, item := range spec.Spec {
		if err := validateSpecItem(i, item); err != nil {
			return err
		}
	}
	return nil
}

// validateSpecItem checks a single SpecItem for completeness (File + Action +
// Metric + Acceptance).
func validateSpecItem(i int, item SpecItem) error {
	if strings.TrimSpace(item.File) == "" {
		return fmt.Errorf("deliberate spec: SpecItem[%d] missing File", i)
	}
	if strings.TrimSpace(item.Action) == "" {
		return fmt.Errorf("deliberate spec: SpecItem[%d] missing Action", i)
	}
	if strings.TrimSpace(item.Metric) == "" {
		return fmt.Errorf("deliberate spec: SpecItem[%d] missing Metric", i)
	}
	if strings.TrimSpace(item.Acceptance) == "" {
		return fmt.Errorf("deliberate spec: SpecItem[%d] missing Acceptance", i)
	}
	return nil
}
