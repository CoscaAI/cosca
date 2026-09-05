package reaper

import (
	"context"
	"os"
	"strconv"
	"testing"
)

// TestProcessReaperInvalidPid ensures a non-numeric / non-positive PID is
// rejected without touching any resource (pure logic, runs on every platform).
func TestProcessReaperInvalidPid(t *testing.T) {
	rp := ProcessReaper()
	for _, id := range []string{"", "abc", "0", "-5", "12.5"} {
		out, err := rp.Reap(context.Background(), Entry{Kind: KindProcess, ID: id, Match: "x"}, ReapContext{})
		if err != nil {
			t.Fatalf("id %q: unexpected error: %v", id, err)
		}
		if out.Status != StatusSkipped {
			t.Fatalf("id %q: status = %s, want skipped", id, out.Status)
		}
	}
}

// TestProcessReaperMissingIdentityMarker ensures a process with no match marker
// is never killed (it cannot be verified).
func TestProcessReaperMissingIdentityMarker(t *testing.T) {
	rp := ProcessReaper()
	// A PID that is alive but has no match marker must be skipped, not killed.
	pid := strconv.Itoa(os.Getpid())
	out, err := rp.Reap(context.Background(), Entry{Kind: KindProcess, ID: pid, Match: ""}, ReapContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Status != StatusSkipped {
		t.Fatalf("status = %s, want skipped (no identity marker)", out.Status)
	}
}
