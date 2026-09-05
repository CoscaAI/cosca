package pipeline

import (
	"strings"
	"testing"
	"time"
)

func TestPerfTrackerRecord(t *testing.T) {
	p := NewPerfTracker()

	p.RecordPhase(PhasePlanning, 100*time.Millisecond)
	p.RecordPhase(PhasePlanning, 300*time.Millisecond)
	p.RecordPhase(PhaseTest, 50*time.Millisecond)

	if p.PhaseTimings[PhasePlanning] != 400*time.Millisecond {
		t.Fatalf("planning total = %v", p.PhaseTimings[PhasePlanning])
	}
	if p.PhaseCounts[PhasePlanning] != 2 {
		t.Fatalf("planning count = %d", p.PhaseCounts[PhasePlanning])
	}
	if avg := p.PhaseAvg(PhasePlanning); avg != 200*time.Millisecond {
		t.Fatalf("phase avg = %v", avg)
	}
	if avg := p.PhaseAvg(PhaseRecovery); avg != 0 {
		t.Fatalf("empty phase avg = %v", avg)
	}
}

func TestPerfTrackerTasksAndSuccessRate(t *testing.T) {
	p := NewPerfTracker()
	p.RecordTask("success")
	p.RecordTask("success")
	p.RecordTask("partial")
	p.RecordTask("failed")

	if p.TotalTasks != 4 || p.SuccessCount != 2 || p.CompletedCount != 3 {
		t.Fatalf("counters: %d/%d/%d", p.TotalTasks, p.SuccessCount, p.CompletedCount)
	}
	if p.SuccessRate() != 0.5 {
		t.Fatalf("success rate = %v, want 0.5", p.SuccessRate())
	}

	empty := NewPerfTracker()
	if empty.SuccessRate() != 0 {
		t.Fatal("empty success rate must be 0")
	}
}

func TestPerfTrackerTokensAndThroughput(t *testing.T) {
	p := NewPerfTracker()
	p.RecordTokens(1000)
	p.RecordTokens(500)
	if p.TotalTokens != 1500 {
		t.Fatalf("tokens = %d", p.TotalTokens)
	}

	// Before Finish, throughput uses elapsed wall time — assert only >= 0.
	if p.TokenThroughput() < 0 {
		t.Fatal("negative throughput")
	}

	// Zero elapsed → 0.
	z := NewPerfTracker()
	if z.TokenThroughput() != 0 {
		t.Fatal("empty throughput must be 0")
	}
}

func TestPerfTrackerFinishAndFormat(t *testing.T) {
	p := NewPerfTracker()
	p.RecordPhase(PhasePlanning, 10*time.Millisecond)
	p.RecordTask("success")
	p.RecordTokens(100)
	// Let measurable session time elapse: Windows time.Now() can jump in
	// ~0.5ms steps, so an instant Finish() may legitimately read 0s.
	time.Sleep(2 * time.Millisecond)
	p.Finish()

	if p.EndTime.IsZero() {
		t.Fatal("EndTime must be set after Finish")
	}
	if p.TotalSessionTime <= 0 {
		t.Fatalf("session time = %v", p.TotalSessionTime)
	}

	out := p.FormatSummary()
	if !strings.Contains(out, "PERFORMANCE SUMMARY") {
		t.Fatal("missing header")
	}
	if !strings.Contains(out, "planning") || !strings.Contains(out, "1 (1 success") {
		t.Fatalf("summary content: %q", out)
	}
	if !strings.Contains(out, "tok/s") {
		t.Fatalf("missing throughput: %q", out)
	}
}
