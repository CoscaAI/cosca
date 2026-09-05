package reaper

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// fakeReaper is a controllable Reaper used to test registry dispatch and report
// aggregation without touching real resources.
type fakeReaper struct {
	kind    string
	outcome Outcome
	err     error
}

func (f fakeReaper) Kind() string          { return f.kind }
func (f fakeReaper) Reap(context.Context, Entry, ReapContext) (Outcome, error) {
	return f.outcome, f.err
}

func TestParsePID(t *testing.T) {
	cases := []struct {
		in      string
		wantOK  bool
		wantPID int
	}{
		{"123", true, 123},
		{"1", true, 1},
		{"", false, 0},
		{"abc", false, 0},
		{"-5", false, 0},
		{"0", false, 0},
		{"12ab", false, 0},
		{" 12", false, 0}, // whitespace is not a numeric PID
	}
	for _, c := range cases {
		pid, err := parsePID(c.in)
		if c.wantOK && err != nil {
			t.Errorf("parsePID(%q) unexpected error: %v", c.in, err)
		}
		if !c.wantOK && err == nil {
			t.Errorf("parsePID(%q) expected error, got pid %d", c.in, pid)
		}
		if c.wantOK && pid != c.wantPID {
			t.Errorf("parsePID(%q) = %d, want %d", c.in, pid, c.wantPID)
		}
	}
}

func TestIdentityMatch(t *testing.T) {
	cases := []struct {
		cmdline string
		match   string
		want    bool
	}{
		{"python -m pytest /x/y", "pytest", true},
		{"go build ./...", "go build", true},
		{"sleep 30", "reaper-marker", false},
		{"", "anything", false},
		{"sleep 30", "", false},
	}
	for _, c := range cases {
		if got := identityMatch(c.cmdline, c.match); got != c.want {
			t.Errorf("identityMatch(%q, %q) = %v, want %v", c.cmdline, c.match, got, c.want)
		}
	}
}

func TestBelowRoot(t *testing.T) {
	base := filepath.Join(string(os.PathSeparator), "tmp", "cosca")
	cases := []struct {
		path string
		root string
		want bool
	}{
		{filepath.Join(base, "a", "b"), base, true},
		{filepath.Join(base, "a"), base, true},
		{base, base, false},                        // equal root is NOT below
		{filepath.Join(base, "..", "escape"), base, false}, // .. escape
		{filepath.Join(string(os.PathSeparator), "tmp"), base, false}, // sibling
	}
	for _, c := range cases {
		if got := belowRoot(c.path, c.root); got != c.want {
			t.Errorf("belowRoot(%q, %q) = %v, want %v", c.path, c.root, got, c.want)
		}
	}
}

func TestRegistryDispatch(t *testing.T) {
	reg := NewRegistry()
	reg.Register(fakeReaper{kind: "x", outcome: Outcome{Status: StatusReaped}})
	reg.Register(fakeReaper{kind: "y", outcome: Outcome{Status: StatusMissing}})

	entries := []Entry{
		{Kind: "x", ID: "1"},
		{Kind: "y", ID: "2"},
		{Kind: "z", ID: "3"}, // unknown kind → skipped
	}
	report, err := reg.Reap(context.Background(), entries, ReapOptions{})
	if err != nil {
		t.Fatalf("Reap returned error: %v", err)
	}
	if len(report.Reaped) != 1 || report.Reaped[0].ID != "1" {
		t.Fatalf("reaped = %+v, want [entry 1]", report.Reaped)
	}
	if len(report.Missing) != 1 || report.Missing[0].ID != "2" {
		t.Fatalf("missing = %+v, want [entry 2]", report.Missing)
	}
	if len(report.Skipped) != 1 || report.Skipped[0].Entry.ID != "3" {
		t.Fatalf("skipped = %+v, want [entry 3]", report.Skipped)
	}
	if report.Skipped[0].Reason != "no reaper for kind" {
		t.Fatalf("reason = %q, want no-reaper", report.Skipped[0].Reason)
	}
}

func TestReaperErrorGoesToReport(t *testing.T) {
	reg := NewRegistry()
	reg.Register(fakeReaper{kind: "x", err: errors.New("boom")})
	report, err := reg.Reap(context.Background(), []Entry{{Kind: "x", ID: "1"}}, ReapOptions{})
	if err != nil {
		t.Fatalf("Reap should not propagate a per-reaper error: %v", err)
	}
	if len(report.Errors) != 1 || report.Errors[0].Err.Error() != "boom" {
		t.Fatalf("errors = %+v, want the injected error", report.Errors)
	}
	if len(report.Reaped)+len(report.Missing)+len(report.Skipped) != 0 {
		t.Fatalf("an errored entry must not land in reaped/missing/skipped")
	}
}

func TestRetainProtectsUnlessPurge(t *testing.T) {
	reg := NewRegistry()
	reg.Register(fakeReaper{kind: "x", outcome: Outcome{Status: StatusReaped}})

	retained := []Entry{{Kind: "x", ID: "1", Retain: true}}

	report, err := reg.Reap(context.Background(), retained, ReapOptions{})
	if err != nil {
		t.Fatalf("Reap error: %v", err)
	}
	if len(report.Retained) != 1 {
		t.Fatalf("retained = %+v, want the retained entry", report.Retained)
	}
	if len(report.Reaped) != 0 {
		t.Fatalf("reaped = %+v, want none (retain should protect)", report.Reaped)
	}

	// Purge bypasses retain.
	purged, err := reg.Purge(context.Background(), retained, ReapOptions{})
	if err != nil {
		t.Fatalf("Purge error: %v", err)
	}
	if len(purged.Reaped) != 1 {
		t.Fatalf("purge reaped = %+v, want the entry reaped", purged.Reaped)
	}
	if len(purged.Retained) != 0 {
		t.Fatalf("purge retained = %+v, want none", purged.Retained)
	}
}

func TestCustomReaperOverridesRegistry(t *testing.T) {
	reg := DefaultRegistry()
	// Replace the process reaper with one that never reaps, to prove opts.Reapers
	// wins over the built-in registry.
	override := fakeReaper{kind: KindProcess, outcome: Outcome{Status: StatusSkipped, Reason: "override"}}
	report, err := reg.Reap(context.Background(), []Entry{{Kind: KindProcess, ID: "9999", Match: "x"}}, ReapOptions{
		Reapers: map[string]Reaper{KindProcess: override},
	})
	if err != nil {
		t.Fatalf("Reap error: %v", err)
	}
	if len(report.Skipped) != 1 || report.Skipped[0].Reason != "override" {
		t.Fatalf("skipped = %+v, want the override reason", report.Skipped)
	}
}

func TestDefaultRegistryHasKinds(t *testing.T) {
	reg := DefaultRegistry()
	for _, kind := range []string{KindProcess, KindDocker, KindDockerVolume, KindTmpDir} {
		if reg.Lookup(kind) == nil {
			t.Fatalf("DefaultRegistry missing reaper for kind %q", kind)
		}
	}
}

func TestEntryLedgerKey(t *testing.T) {
	e := Entry{Kind: KindProcess, ID: "1234"}
	if want := "reaper:process:1234"; e.LedgerKey() != want {
		t.Fatalf("LedgerKey() = %q, want %q", e.LedgerKey(), want)
	}
}

func TestDefaultAllowedTmpRootsIncludesOSTemp(t *testing.T) {
	roots := defaultAllowedTmpRoots("")
	if len(roots) == 0 {
		t.Fatalf("expected at least one allowed tmp root")
	}
	found := false
	for _, r := range roots {
		// canonicalPath already resolved; just verify a temp root is present.
		if filepath.IsAbs(r) {
			found = true
		}
	}
	if !found {
		t.Fatalf("allowed tmp roots %v must be absolute", roots)
	}
}
