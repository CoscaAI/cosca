package memory

import (
	"encoding/json"
	"strings"
	"testing"
)

// curatedFailuresFixture mirrors the real format verified in
// internal/embed/cosca/memory/agent/cosca-specialist-backend/failures.md:
// three entries — one complete, one minimal, one malformed — with a
// multi-line Avoidance Pattern to exercise value accumulation.
const curatedFailuresFixture = `# cosca-specialist-backend — Negative Memory (Failures)

> Auto-evolution memory. Failures are the most valuable teachers. Search before acting.

## Active Failures

### F100 | 2026-07-29 | Complete Failure — lesson shared

| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-backend |
| **Task** | The Don ordered a secret migration of the auth vault — internal details X |
| **Failed Approach** | Tried to rollback the migration — internal approach Y |
| **Root Cause** | The lock ordering — internal root cause Z |
| **Consequence** | The auth vault was locked — internal consequence W |
| **Lesson** | **(1) Always take a checkpoint before any destructive migration. (2) Validate the rollback path in a dry run first.** |
| **Confidence Impact** | -0.10 |
| **Tags** | #failure #learned #checkpoint #migration #dry-run |
| **Avoidance Pattern** | Before any destructive migration: capture a
checkpoint, dry-run the rollback, and verify restore. Never skip. |

---

### F101 | 2026-07-30 | Minimal Failure — lesson only

| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-backend |
| **Task** | Internal details for minimal |
| **Lesson** | Keep it minimal: verify before you claim. |

---

### broken header without pipes

| **Lesson** | This entry is malformed and must be skipped. |
`

// TestParseCuratedFailures verifies extraction of the sanitized fields,
// multi-line value accumulation, and graceful skipping of malformed entries.
func TestParseCuratedFailures(t *testing.T) {
	got := ParseCuratedFailures(curatedFailuresFixture, "cosca-specialist-backend")

	if len(got) != 2 {
		t.Fatalf("expected 2 curated failures, got %d", len(got))
	}

	complete := got[0]
	if complete.ID != "F100" {
		t.Errorf("ID = %q, want %q", complete.ID, "F100")
	}
	if complete.Date != "2026-07-29" {
		t.Errorf("Date = %q, want %q", complete.Date, "2026-07-29")
	}
	if complete.Name != "Complete Failure — lesson shared" {
		t.Errorf("Name = %q, want %q", complete.Name, "Complete Failure — lesson shared")
	}
	if complete.Agent != "cosca-specialist-backend" {
		t.Errorf("Agent = %q, want %q", complete.Agent, "cosca-specialist-backend")
	}
	if complete.Lesson == "" {
		t.Error("Lesson is empty, want the extracted lesson")
	}
	if !strings.Contains(complete.Lesson, "checkpoint") {
		t.Errorf("Lesson = %q, want it to contain the lesson text", complete.Lesson)
	}
	if complete.AvoidancePattern != "Before any destructive migration: capture a checkpoint, dry-run the rollback, and verify restore. Never skip." {
		t.Errorf("AvoidancePattern = %q, want the accumulated multi-line value", complete.AvoidancePattern)
	}

	wantTags := []string{"failure", "learned", "checkpoint", "migration", "dry-run"}
	if len(complete.Tags) != len(wantTags) {
		t.Fatalf("Tags = %v, want %v", complete.Tags, wantTags)
	}
	for i := range wantTags {
		if complete.Tags[i] != wantTags[i] {
			t.Errorf("Tags[%d] = %q, want %q", i, complete.Tags[i], wantTags[i])
		}
	}

	minimal := got[1]
	if minimal.ID != "F101" {
		t.Errorf("ID = %q, want %q", minimal.ID, "F101")
	}
	if minimal.Lesson != "Keep it minimal: verify before you claim." {
		t.Errorf("Lesson = %q, want %q", minimal.Lesson, "Keep it minimal: verify before you claim.")
	}
	if minimal.AvoidancePattern != "" {
		t.Errorf("AvoidancePattern = %q, want empty for minimal entry", minimal.AvoidancePattern)
	}
	if len(minimal.Tags) != 0 {
		t.Errorf("Tags = %v, want empty for minimal entry", minimal.Tags)
	}
}

// proseFailureFixture mirrors the real prose style used in
// internal/embed/cosca/memory/agent/cosca-specialist-backend-service/failures.md:
// a `### {ID} | {date} | {Name}` header followed by prose paragraphs and NO
// table rows. The lesson must fall back to the joined prose body.
const proseFailureFixture = `# cosca-specialist-backend-service — Negative Memory (Failures)

> Auto-evolution memory. Failures are the most valuable teachers. Search before acting.

## Active Failures

### F009 | 2026-08-04 | Non-idempotent cleanup

The cleanup was invoked twice because a test called the returned
function explicitly and also deferred it. The second close(2) could
reuse the descriptor elsewhere. Fix by guarding the callback with
sync.Once so it runs exactly once.

---
> **Protocol**: [LEARNING_PROTOCOL.md](../../LEARNING_PROTOCOL.md) | **Constitution**: P5 — A família aprende com erros
`

// TestParseCuratedFailures_ProseFallback verifies that prose-style entries
// (no `| **Lesson** |` table row) fall back to their prose body as the Lesson,
// and that a prose entry without a `---` separator still parses.
func TestParseCuratedFailures_ProseFallback(t *testing.T) {
	got := ParseCuratedFailures(proseFailureFixture, "cosca-specialist-backend-service")
	if len(got) != 1 {
		t.Fatalf("expected 1 curated failure, got %d", len(got))
	}

	e := got[0]
	if e.ID != "F009" {
		t.Errorf("ID = %q, want %q", e.ID, "F009")
	}
	if e.Date != "2026-08-04" {
		t.Errorf("Date = %q, want %q", e.Date, "2026-08-04")
	}
	if e.Lesson == "" {
		t.Fatal("Lesson is empty, want the prose body as the fallback lesson")
	}
	if !strings.Contains(e.Lesson, "sync.Once") {
		t.Errorf("Lesson = %q, want it to contain %q", e.Lesson, "sync.Once")
	}
	want := "The cleanup was invoked twice because a test called the returned function explicitly and also deferred it. The second close(2) could reuse the descriptor elsewhere. Fix by guarding the callback with sync.Once so it runs exactly once."
	if e.Lesson != want {
		t.Errorf("Lesson = %q, want %q", e.Lesson, want)
	}
	if e.AvoidancePattern != "" {
		t.Errorf("AvoidancePattern = %q, want empty for prose entry", e.AvoidancePattern)
	}
	if len(e.Tags) != 0 {
		t.Errorf("Tags = %v, want empty for prose entry", e.Tags)
	}

	edge := ParseCuratedFailures("### F010 | 2026-08-04 | No separator\n\nPlain prose line one.\nPlain prose line two.\n", "cosca-specialist-backend-service")
	if len(edge) != 1 {
		t.Fatalf("expected 1 curated failure without separator, got %d", len(edge))
	}
	if edge[0].ID != "F010" || edge[0].Lesson != "Plain prose line one. Plain prose line two." {
		t.Errorf("prose entry without separator edge = %+v, want ID F010 with joined prose lesson", edge[0])
	}
}

// TestParseCuratedFailuresSanitized verifies that internal details never leak into
// the curated output: neither the sensitive values nor the internal field
// names may appear in the serialized form.
func TestCuratedFailuresSanitized(t *testing.T) {
	got := ParseCuratedFailures(curatedFailuresFixture, "cosca-specialist-backend")

	data, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	serialized := string(data)

	for _, internal := range []string{
		"auth vault",          // Task / Consequence detail
		"lock ordering",       // Root Cause detail
		"internal approach Y", // Failed Approach detail
		"Task",
		"Failed Approach",
		"Root Cause",
		"Consequence",
	} {
		if strings.Contains(serialized, internal) {
			t.Errorf("curated output leaks internal detail %q: %s", internal, serialized)
		}
	}
}

// TestCuratedFailuresDepartmentOf verifies that the Domain is derived from the
// agent name via DepartmentOf, including the empty domain for the Kernel.
func TestCuratedFailuresDepartmentOf(t *testing.T) {
	specialist := ParseCuratedFailures(curatedFailuresFixture, "cosca-specialist-backend")
	if len(specialist) == 0 || specialist[0].Domain != "backend" {
		t.Errorf("specialist Domain = %q, want %q", specialist[0].Domain, "backend")
	}

	kernel := ParseCuratedFailures("### F200 | 2026-08-01 | Kernel failure\n\n| **Lesson** | Lesson text |", "cosca-kernel")
	if len(kernel) != 1 {
		t.Fatalf("expected 1 curated failure, got %d", len(kernel))
	}
	if kernel[0].Domain != "" {
		t.Errorf("kernel Domain = %q, want empty", kernel[0].Domain)
	}
}

// TestCuratedFailuresMalformedOnly verifies that a document with only
// malformed entries yields an empty result without error.
func TestCuratedFailuresMalformedOnly(t *testing.T) {
	got := ParseCuratedFailures("### no pipes here\n\nsome prose without rows", "cosca-kernel")
	if len(got) != 0 {
		t.Fatalf("expected 0 curated failures, got %d", len(got))
	}

	got = ParseCuratedFailures("### F300 | 2026-08-01 | No lesson\n\n| **Task** | details |", "cosca-kernel")
	if len(got) != 0 {
		t.Fatalf("expected 0 curated failures for entry without lesson, got %d", len(got))
	}
}

// TestParseFailureTags verifies tag extraction tolerating stray separators.
func TestParseFailureTags(t *testing.T) {
	tags := parseFailureTags("#a #b, #c; #d-e ")
	want := []string{"a", "b", "c", "d-e"}
	if len(tags) != len(want) {
		t.Fatalf("Tags = %v, want %v", tags, want)
	}
	for i := range want {
		if tags[i] != want[i] {
			t.Errorf("Tags[%d] = %q, want %q", i, tags[i], want[i])
		}
	}
	if got := parseFailureTags("no hashes here"); len(got) != 0 {
		t.Errorf("expected no tags, got %v", got)
	}
}
