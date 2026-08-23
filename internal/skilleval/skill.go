// Package skilleval provides the data model and scorers for the skill
// evaluation harness (meta-loop A/B). It captures a snapshot of a skill and
// its evaluation cases, the grading results and summary statistics for a
// benchmark arm, and the scoring contract used to judge an output against a
// rubric.
//
// Design notes:
//   - Identity is immutable: a Skill is a snapshot (its Body is what evolves
//     in phase 2, referenced here by SnapshotRef).
//   - Robustness rule: summary statistics use median + IQR (never the mean)
//     and the mean's dispersion is measured via internal/evals.BootstrapStd.
//   - Parsing of LLM judge responses is deliberately defensive and
//     deterministic; a malformed verdict must never abort the eval loop.
package skilleval

// Skill is the identity snapshot of an evaluable skill.
type Skill struct {
	Name        string // canonical skill name
	Path        string // source path of the skill definition
	Description string
	Level       int    // proficiency level of the skill
	Body        string // body of the skill (what evolves in phase 2; here a snapshot)
	SnapshotRef string // hash of the baseline snapshot
}

// SkillCase is a single evaluation case attached to a skill. Rubric entries
// encode the expected_behavior — "what a good result DOES" — rather than how
// the result is written.
type SkillCase struct {
	ID     string
	Task   string
	Rubric []string // expected_behavior conditions
	Weight float64  // relative weight of this case (0 means default weight)
}
