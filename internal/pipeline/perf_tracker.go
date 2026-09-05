package pipeline

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type PerfTracker struct {
	mu sync.RWMutex

	PhaseTimings map[PerfPhase]time.Duration
	PhaseCounts  map[PerfPhase]int64

	TotalSessionTime time.Duration
	TotalTokens      int64
	TotalTasks       int
	SuccessCount     int
	CompletedCount   int

	StartTime time.Time
	EndTime   time.Time
}

type PerfPhase string

const (
	PhasePlanning    PerfPhase = "planning"
	PhaseSearch      PerfPhase = "search"
	PhaseGeneration  PerfPhase = "generation"
	PhaseToolExec    PerfPhase = "tool_exec"
	PhaseTest        PerfPhase = "test"
	PhaseRecovery    PerfPhase = "recovery"
	PhaseVerification PerfPhase = "verification"
)

func NewPerfTracker() *PerfTracker {
	return &PerfTracker{
		PhaseTimings: make(map[PerfPhase]time.Duration),
		PhaseCounts:  make(map[PerfPhase]int64),
		StartTime:    time.Now().UTC(),
	}
}

func (p *PerfTracker) RecordPhase(phase PerfPhase, d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.PhaseTimings[phase] += d
	p.PhaseCounts[phase]++
}

func (p *PerfTracker) RecordTokens(tokens int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.TotalTokens += tokens
}

func (p *PerfTracker) RecordTask(outcome string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.TotalTasks++
	switch outcome {
	case "success":
		p.SuccessCount++
		p.CompletedCount++
	case "partial":
		p.CompletedCount++
	}
}

func (p *PerfTracker) Finish() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.EndTime = time.Now().UTC()
	p.TotalSessionTime = p.EndTime.Sub(p.StartTime)
}

func (p *PerfTracker) TokenThroughput() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	elapsed := p.TotalSessionTime
	if elapsed == 0 && p.EndTime.IsZero() {
		elapsed = time.Since(p.StartTime)
	}
	if elapsed.Seconds() == 0 {
		return 0
	}
	return float64(p.TotalTokens) / elapsed.Seconds()
}

func (p *PerfTracker) SuccessRate() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.TotalTasks == 0 {
		return 0
	}
	return float64(p.SuccessCount) / float64(p.TotalTasks)
}

func (p *PerfTracker) PhaseAvg(phase PerfPhase) time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()

	count := p.PhaseCounts[phase]
	if count == 0 {
		return 0
	}
	return p.PhaseTimings[phase] / time.Duration(count)
}

func (p *PerfTracker) FormatSummary() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var b strings.Builder

	b.WriteString(fmt.Sprintf("=== PERFORMANCE SUMMARY ===\n"))
	b.WriteString(fmt.Sprintf("Total session time:   %s\n", p.TotalSessionTime.Round(time.Millisecond)))
	b.WriteString(fmt.Sprintf("Tasks:                %d (%d success, %.0f%%)\n",
		p.TotalTasks, p.SuccessCount, p.SuccessRate()*100))
	b.WriteString(fmt.Sprintf("Total tokens:         %d\n", p.TotalTokens))
	b.WriteString(fmt.Sprintf("Token throughput:     %.1f tok/s\n", p.TokenThroughput()))

	b.WriteString("\n--- Phase Breakdown ---\n")
	phases := []PerfPhase{PhasePlanning, PhaseSearch, PhaseGeneration, PhaseToolExec, PhaseTest, PhaseRecovery, PhaseVerification}
	for _, phase := range phases {
		total := p.PhaseTimings[phase]
		count := p.PhaseCounts[phase]
		if count == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("  %s: %s (%d calls, avg %s)\n",
			phase, total.Round(time.Millisecond), count, p.PhaseAvg(phase).Round(time.Microsecond)))
	}

	return b.String()
}
