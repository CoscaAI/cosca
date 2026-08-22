package pipeline

import (
	"math"
	"sync"
	"time"
)

// CMITracker tracks Cognitive Maturity Index across 6 dimensions.
type CMITracker struct {
	mu sync.RWMutex

	// Internal accumulators
	totalTasks           int
	successCount         int
	partialCount         int
	totalKnowledgeGained int
	totalErrorsDetected  int
	totalKnowledgeReused int

	// Rolling history for consistency computation
	outcomeHistory []bool // true = success, false = non-success
	historyCap     int

	// Dimension scores (0-1)
	Learning     float64
	Judgment     float64
	Planning     float64
	SelfCritique float64
	Transfer     float64
	Consistency  float64
}

// CMISnapshot captures CMI at a point in time.
type CMISnapshot struct {
	Timestamp  time.Time
	Dimensions CMITracker
	Overall    float64
	TasksCount int
	AgentName  string
}

const defaultHistoryCap = 32

// NewCMITracker creates a new CMITracker with default initial values.
func NewCMITracker() *CMITracker {
	return &CMITracker{
		Learning:     0.50,
		Judgment:     0.50,
		Planning:     0.50,
		SelfCritique: 0.50,
		Transfer:     0.50,
		Consistency:  1.00,
		historyCap:   defaultHistoryCap,
	}
}

// RecordTask updates CMI based on task outcome.
// knowledgeGained is the count of discrete learnings acquired in this task.
// errorsDetected is the count of errors detected during self-critique.
// knowledgeReused is the count of previous learnings reused in this task.
func (c *CMITracker) RecordTask(outcome string, knowledgeGained int, errorsDetected int, knowledgeReused int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recordTaskLocked(outcome, knowledgeGained, errorsDetected, knowledgeReused)
}

// recordTaskLocked is the internal implementation. Caller must hold mu.
func (c *CMITracker) recordTaskLocked(outcome string, knowledgeGained int, errorsDetected int, knowledgeReused int) {
	c.totalTasks++
	c.totalKnowledgeGained += knowledgeGained
	c.totalErrorsDetected += errorsDetected
	c.totalKnowledgeReused += knowledgeReused

	isSuccess := outcome == "success"
	if isSuccess {
		c.successCount++
	} else if outcome == "partial" {
		c.partialCount++
	}

	c.outcomeHistory = append(c.outcomeHistory, isSuccess)
	if len(c.outcomeHistory) > c.historyCap {
		c.outcomeHistory = c.outcomeHistory[len(c.outcomeHistory)-c.historyCap:]
	}

	c.recalculate()
}

// RecordBenchmark feeds a benchmark result into CMI.
// Maps benchmark accuracy to CMI dimensions:
//   - Judgment: accuracy (correct/total)
//   - Learning: knowledge gained from benchmark
//   - SelfCritique: errors detected during benchmark
//   - Consistency: stability across multiple runs
func (c *CMITracker) RecordBenchmark(accuracy float64, totalQuestions int, errorsDetected int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.totalTasks++
	c.totalErrorsDetected += errorsDetected
	c.totalKnowledgeGained++ // benchmark itself is a learning event

	isSuccess := accuracy >= 0.5
	if isSuccess {
		c.successCount++
	} else if accuracy >= 0.25 {
		c.partialCount++
	}

	c.outcomeHistory = append(c.outcomeHistory, isSuccess)
	if len(c.outcomeHistory) > c.historyCap {
		c.outcomeHistory = c.outcomeHistory[len(c.outcomeHistory)-c.historyCap:]
	}

	// Adjust Judgment based on accuracy directly.
	c.Judgment = round2(c.Judgment*0.7 + accuracy*0.3)

	c.recalculate()
}

// Snapshot returns current CMI values.
func (c *CMITracker) Snapshot() CMISnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return CMISnapshot{
		Timestamp: time.Now().UTC(),
		Dimensions: CMITracker{
			Learning:     c.Learning,
			Judgment:     c.Judgment,
			Planning:     c.Planning,
			SelfCritique: c.SelfCritique,
			Transfer:     c.Transfer,
			Consistency:  c.Consistency,
		},
		Overall:    c.OverallScore(),
		TasksCount: c.totalTasks,
	}
}

// OverallScore computes weighted CMI:
//
//	Learning*0.25 + Judgment*0.20 + Planning*0.20 +
//	SelfCritique*0.15 + Transfer*0.10 + Consistency*0.10
func (c *CMITracker) OverallScore() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return round2(c.Learning*0.25 +
		c.Judgment*0.20 +
		c.Planning*0.20 +
		c.SelfCritique*0.15 +
		c.Transfer*0.10 +
		c.Consistency*0.10)
}

// recalculate recomputes all dimension scores. Caller must hold mu.
func (c *CMITracker) recalculate() {
	if c.totalTasks == 0 {
		return
	}
	n := float64(c.totalTasks)

	c.Learning = clamp01(round2(float64(c.totalKnowledgeGained) / (n * 3.0)))
	c.Judgment = round2(float64(c.successCount) / n)
	c.Planning = round2((float64(c.successCount)*1.0 + float64(c.partialCount)*0.5) / n)
	c.SelfCritique = clamp01(round2(float64(c.totalErrorsDetected) / (n * 2.0)))
	c.Transfer = clamp01(round2(float64(c.totalKnowledgeReused) / (n * 2.0)))
	c.Consistency = round2(c.computeConsistency())
}

func (c *CMITracker) computeConsistency() float64 {
	if len(c.outcomeHistory) < 2 {
		return 1.0
	}

	changes := 0
	for i := 1; i < len(c.outcomeHistory); i++ {
		if c.outcomeHistory[i] != c.outcomeHistory[i-1] {
			changes++
		}
	}

	maxChanges := len(c.outcomeHistory) - 1
	return clamp01(1.0 - float64(changes)/float64(maxChanges))
}

func clamp01(v float64) float64 {
	return math.Max(0.0, math.Min(1.0, v))
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
