package skilleval

import (
	"strings"
	"testing"
)

// mkGenome builds a Genome deterministically by hand (no file I/O), so the
// guardrail unit tests exercise the pure functions directly.
func mkGenome(name, desc string, level int, body string) *Genome {
	return &Genome{
		Identity: Skill{Name: name, Description: desc, Level: level, Body: body},
		Body:     body,
		Source:   "---\nname: " + name + "\ndescription: " + desc + "\nlevel: " + itoa(level) + "\n---\n" + body,
	}
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// longBody is a multi-line, multi-word body used to keep a +1-line mutation
// within the default 20% growth cap and the similarity high.
func longBody() string {
	return strings.Join([]string{
		"# Overview",
		"",
		"This skill helps an agent build reliable REST endpoints.",
		"It covers validation, error handling, idempotency, and observability.",
		"Every request must be validated before the handler is invoked.",
		"",
		"## Steps",
		"1. Validate the incoming payload and reject malformed bodies.",
		"2. Run the business rule and collect the result object.",
		"3. Map domain errors to a stable, documented HTTP status code.",
		"4. Emit a structured log line with request and outcome metadata.",
		"5. Return the payload with the appropriate content-type header.",
		"",
		"The skill never leaks internals and always returns a bounded response.",
	}, "\n")
}

// =============================================================================
// CheckSizeLimit
// =============================================================================

func TestCheckSizeLimit_OKWithinCap(t *testing.T) {
	genome := mkGenome("rest-api", "Build REST endpoints", 2, longBody())
	report := CheckSizeLimit(genome, DefaultMaxSkillLines, DefaultMaxGrowthPct, nil)
	if !report.OK {
		t.Fatalf("expected OK, got violations: %v", report.Violations)
	}
	if len(report.Violations) != 0 {
		t.Fatalf("expected no violations, got: %v", report.Violations)
	}
}

func TestCheckSizeLimit_ExceedsLineCap(t *testing.T) {
	genome := mkGenome("rest-api", "Build REST endpoints", 2, strings.Repeat("line one\n", 600))
	report := CheckSizeLimit(genome, 500, DefaultMaxGrowthPct, nil)
	if report.OK {
		t.Fatal("expected a violation for exceeding the line cap")
	}
	if !strings.Contains(report.Violations[0], "exceeds the 500-line cap") {
		t.Fatalf("unexpected violation text: %v", report.Violations)
	}
}

func TestCheckSizeLimit_GrowthWithinAllowance(t *testing.T) {
	base := mkGenome("rest-api", "Build REST endpoints", 2, longBody())
	// +2 lines vs a 14-line baseline is ~14% growth — inside the (default) 20% cap.
	candidate := mkGenome("rest-api", "Build REST endpoints", 2, longBody()+"\nadded line one\nadded line two\n")
	report := CheckSizeLimit(candidate, DefaultMaxSkillLines, DefaultMaxGrowthPct, base)
	if !report.OK {
		t.Fatalf("expected growth to be allowed, got: %v", report.Violations)
	}
}

func TestCheckSizeLimit_GrowthExceedsCap(t *testing.T) {
	base := mkGenome("t", "d", 1, "one\n") // 1 body line
	candidate := mkGenome("t", "d", 1, "one\ntwo\nthree\nfour\nfive\nsix\n")
	report := CheckSizeLimit(candidate, DefaultMaxSkillLines, 10.0, base)
	if report.OK {
		t.Fatal("expected a violation for exceeding the growth cap")
	}
	if !strings.Contains(report.Violations[0], "growth cap") {
		t.Fatalf("unexpected violation text: %v", report.Violations)
	}
}

func TestCheckSizeLimit_TightGrowthCapRejects(t *testing.T) {
	base := mkGenome("t", "d", 1, longBody())
	candidate := mkGenome("t", "d", 1, longBody()+"\nan extra line with more words here\n")
	// A tiny cap rejects even a one-line added body.
	report := CheckSizeLimit(candidate, DefaultMaxSkillLines, 1.0, base)
	if report.OK {
		t.Fatal("expected a violation against a 1% growth cap")
	}
}

func TestCheckSizeLimit_NilGenomeFails(t *testing.T) {
	report := CheckSizeLimit(nil, 500, 20.0, nil)
	if report.OK {
		t.Fatal("expected a nil genome to fail")
	}
}

// =============================================================================
// CheckSemanticPreservation
// =============================================================================

func TestSemanticPreservation_IdenticalIsMax(t *testing.T) {
	body := longBody()
	sim, ok := CheckSemanticPreservation(body, body)
	if sim != 1.0 || !ok {
		t.Fatalf("expected similarity 1.0 and ok, got %.3f ok=%v", sim, ok)
	}
}

func TestSemanticPreservation_AppendingLinePreserves(t *testing.T) {
	base := longBody()
	mutated := base + "\n<!-- evolved: 4 feedback points -->\n"
	sim, ok := CheckSemanticPreservation(base, mutated)
	if !ok {
		t.Fatalf("expected a small append to preserve purpose, similarity %.3f", sim)
	}
	if sim < DefaultMinSimilarity {
		t.Fatalf("expected similarity >= %.2f, got %.3f", DefaultMinSimilarity, sim)
	}
}

func TestSemanticPreservation_RewriteDrifts(t *testing.T) {
	base := longBody()
	// An entirely different vocabulary: a "drifted" mutation.
	rewritten := strings.Join([]string{
		"docker orchestration with kubernetes pods and helm charts",
		"monitor prometheus metrics dashboards and alerting rules",
		"terraform state backend s3 bucket locking providers",
	}, "\n")
	sim, ok := CheckSemanticPreservation(base, rewritten)
	if ok {
		t.Fatalf("expected a rewritten body to fail the preservation guardrail, similarity %.3f", sim)
	}
	if sim >= DefaultMinSimilarity {
		t.Fatalf("expected similarity < %.2f, got %.3f", DefaultMinSimilarity, sim)
	}
}

func TestSemanticPreservation_SubsetKeepsAboveThreshold(t *testing.T) {
	base := "alpha beta gamma delta epsilon zeta eta theta iota kappa"
	mutated := "alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu" // adds 2 tokens
	sim, ok := CheckSemanticPreservation(base, mutated)
	if !ok {
		t.Fatalf("expected a superset to stay preserved, similarity %.3f", sim)
	}
	if sim < 0.8 {
		t.Fatalf("expected similarity >= 0.8, got %.3f", sim)
	}
}

func TestSemanticSimilarity_EmptyBodies(t *testing.T) {
	if got := SemanticSimilarity("", ""); got != 1.0 {
		t.Fatalf("two empty bodies should be identical, got %.3f", got)
	}
	if got := SemanticSimilarity("words here", ""); got != 0.0 {
		t.Fatalf("a non-empty vs empty body should be 0, got %.3f", got)
	}
}

// =============================================================================
// CheckStructural
// =============================================================================

func TestCheckStructural_ValidIdentity(t *testing.T) {
	genome := mkGenome("rest-api", "Build REST endpoints", 2, longBody())
	report := CheckStructural(genome)
	if !report.OK {
		t.Fatalf("expected a valid identity to pass, got: %v", report.Violations)
	}
	if len(report.Violations) != 0 {
		t.Fatalf("expected no violations, got: %v", report.Violations)
	}
}

func TestCheckStructural_MissingName(t *testing.T) {
	genome := mkGenome("", "some description", 2, longBody())
	report := CheckStructural(genome)
	if report.OK {
		t.Fatal("expected a missing name to fail")
	}
	if !strings.Contains(report.Violations[0], "name is missing") {
		t.Fatalf("unexpected violation: %v", report.Violations)
	}
}

func TestCheckStructural_MissingDescription(t *testing.T) {
	genome := mkGenome("rest-api", "", 2, longBody())
	report := CheckStructural(genome)
	if report.OK {
		t.Fatal("expected a missing description to fail")
	}
	if !strings.Contains(report.Violations[0], "description is missing") {
		t.Fatalf("unexpected violation: %v", report.Violations)
	}
}

func TestCheckStructural_InvalidLevel(t *testing.T) {
	genome := mkGenome("rest-api", "Build REST endpoints", 0, longBody())
	report := CheckStructural(genome)
	if report.OK {
		t.Fatal("expected an invalid level (0) to fail")
	}
	if !strings.Contains(report.Violations[0], "level is invalid") {
		t.Fatalf("unexpected violation: %v", report.Violations)
	}
}

func TestCheckStructural_NilGenomeFails(t *testing.T) {
	report := CheckStructural(nil)
	if report.OK {
		t.Fatal("expected a nil genome to fail")
	}
}

// =============================================================================
// CheckCaching ("só nova sessão")
// =============================================================================

func TestCheckCaching_NewSessionPasses(t *testing.T) {
	report := CheckCaching("evolve/rest-api-20260822T120000Z")
	if !report.OK {
		t.Fatalf("expected a fresh session to be allowed, got: %v", report.Violations)
	}
}

func TestCheckCaching_EmptySessionRejected(t *testing.T) {
	report := CheckCaching("")
	if report.OK {
		t.Fatal("expected an empty session to be rejected")
	}
	if !strings.Contains(report.Violations[0], "só nova sessão") {
		t.Fatalf("unexpected violation: %v", report.Violations)
	}
}
