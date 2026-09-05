package pipeline

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type CognitiveBenchmark struct {
	GoalComprehension float64
	PlanQuality       float64
	PlanCompleteness  float64
	AgentRouting      float64
	KnowledgeUsage    float64
	ContextQuality    float64
	RecoveryRate      float64
	AutonomyLevel     float64
	VerificationRate  float64
	DeliveryRate      float64
	VADScore          float64
}

type BenchmarkReport struct {
	SessionID string
	Goal      string
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration

	Cognitive CognitiveBenchmark

	TasksPlanned   int
	TasksCompleted int
	TasksFailed    int
	AgentsUsed     []string
	ModelsUsed     []string
	ToolsUsed      []string
	KnowledgeItems int
	MemoriesUsed   int

	ErrorsEncountered  int
	ErrorsRecovered    int
	HumanInterventions int

	TotalCost   float64
	TotalTokens int64
	AvgLatency  time.Duration

	ArchitectureSufficient []string
	ArchitectureGaps       []string

	Strengths    []string
	Weaknesses   []string
	Improvements []string

	PlannedVsActual string

	// internal accumulators for scoring
	plansRecorded           bool
	planTaskCount           int
	routingDecisions        int
	routingCorrect          int
	knowledgeSearchCount    int
	knowledgeSearchNeeded   int
	contextDeliveredCount   int
	contextSufficientCount  int
	verificationAttempts    int
	verificationSuccesses   int
	deliveryAttempts        int
	deliverySuccesses       int
	taskLatencies           []time.Duration
}

func NewBenchmark(goal string) *BenchmarkReport {
	return &BenchmarkReport{
		SessionID: fmt.Sprintf("BENCH-%s-%s", time.Now().UTC().Format("20060102"), randomHex(6)),
		Goal:      goal,
		StartTime: time.Now().UTC(),
	}
}

func (br *BenchmarkReport) RecordPlan(plan *Plan) {
	if plan == nil {
		return
	}
	br.plansRecorded = true
	br.planTaskCount = len(plan.Tasks)
	br.TasksPlanned = len(plan.Tasks)

	for _, t := range plan.Tasks {
		if t.Agent != "" {
			br.addAgent(t.Agent)
		}
	}
}

func (br *BenchmarkReport) RecordTaskCompletion(task *TaskNode, success bool, agent string, model string) {
	if success {
		br.TasksCompleted++
	} else if task != nil && task.Status == TaskFailed {
		br.TasksFailed++
	}

	if agent != "" {
		br.addAgent(agent)
	}
	if model != "" {
		br.addModel(model)
	}
	if task != nil && task.Result != nil && task.Result.DurationMs > 0 {
		br.taskLatencies = append(br.taskLatencies, time.Duration(task.Result.DurationMs)*time.Millisecond)
	}
}

func (br *BenchmarkReport) RecordError(err error, recovered bool) {
	br.ErrorsEncountered++
	if recovered {
		br.ErrorsRecovered++
	}
}

func (br *BenchmarkReport) RecordHumanIntervention(reason string) {
	br.HumanInterventions++
}

func (br *BenchmarkReport) RecordKnowledgeUse(count int) {
	if count > 0 {
		br.KnowledgeItems += count
		br.knowledgeSearchCount++
	}
}

func (br *BenchmarkReport) RecordKnowledgeNeeded(needed bool) {
	if needed {
		br.knowledgeSearchNeeded++
	}
}

func (br *BenchmarkReport) RecordToolUse(tool string) {
	for _, t := range br.ToolsUsed {
		if t == tool {
			return
		}
	}
	br.ToolsUsed = append(br.ToolsUsed, tool)
}

func (br *BenchmarkReport) RecordRoutingDecision(agent string, correct bool) {
	br.routingDecisions++
	if correct {
		br.routingCorrect++
	}
	br.addAgent(agent)
}

func (br *BenchmarkReport) RecordContextDelivery(sufficient bool) {
	br.contextDeliveredCount++
	if sufficient {
		br.contextSufficientCount++
	}
}

func (br *BenchmarkReport) RecordVerification(success bool) {
	br.verificationAttempts++
	if success {
		br.verificationSuccesses++
	}
}

func (br *BenchmarkReport) RecordDelivery(success bool) {
	br.deliveryAttempts++
	if success {
		br.deliverySuccesses++
	}
}

func (br *BenchmarkReport) CalculateScores() {
	br.EndTime = time.Now().UTC()
	br.Duration = br.EndTime.Sub(br.StartTime)

	// Goal Comprehension: based on plan quality — did we produce a plan?
	if br.plansRecorded {
		br.Cognitive.GoalComprehension = clamp01(0.7)
	} else {
		br.Cognitive.GoalComprehension = clamp01(0.1)
	}

	// Plan Quality: ratio of completed tasks to planned tasks
	if br.TasksPlanned > 0 {
		completed := br.TasksCompleted
		total := br.TasksPlanned
		if total == 0 {
			br.Cognitive.PlanQuality = 0.5
		} else {
			br.Cognitive.PlanQuality = clamp01(float64(completed) / float64(total))
		}
	} else {
		br.Cognitive.PlanQuality = 0.3
	}

	// Plan Completeness: did plan cover the goal scope?
	if br.TasksPlanned > 0 {
		// Penalize under-planning (too few tasks for complex goals)
		switch {
		case br.TasksPlanned >= 5:
			br.Cognitive.PlanCompleteness = 0.9
		case br.TasksPlanned >= 3:
			br.Cognitive.PlanCompleteness = 0.7
		case br.TasksPlanned >= 1:
			br.Cognitive.PlanCompleteness = 0.5
		default:
			br.Cognitive.PlanCompleteness = 0.1
		}
	} else {
		br.Cognitive.PlanCompleteness = 0
	}

	// Agent Routing: correct routing decisions
	if br.routingDecisions > 0 {
		br.Cognitive.AgentRouting = clamp01(float64(br.routingCorrect) / float64(br.routingDecisions))
	} else if len(br.AgentsUsed) > 0 {
		br.Cognitive.AgentRouting = 0.6
	} else {
		br.Cognitive.AgentRouting = 0
	}

	// Knowledge Usage: used knowledge when it was needed
	if br.knowledgeSearchNeeded > 0 {
		br.Cognitive.KnowledgeUsage = clamp01(float64(br.knowledgeSearchCount) / float64(br.knowledgeSearchNeeded))
	} else if br.KnowledgeItems > 0 {
		br.Cognitive.KnowledgeUsage = 0.8
	} else {
		br.Cognitive.KnowledgeUsage = 0.5
	}

	// Context Quality
	if br.contextDeliveredCount > 0 {
		br.Cognitive.ContextQuality = clamp01(float64(br.contextSufficientCount) / float64(br.contextDeliveredCount))
	} else if br.TasksCompleted > 0 {
		br.Cognitive.ContextQuality = 0.7
	} else {
		br.Cognitive.ContextQuality = 0.3
	}

	// Recovery Rate
	if br.ErrorsEncountered > 0 {
		br.Cognitive.RecoveryRate = clamp01(float64(br.ErrorsRecovered) / float64(br.ErrorsEncountered))
	} else {
		br.Cognitive.RecoveryRate = 1.0
	}

	// Autonomy Level: inverse of human interventions
	if br.HumanInterventions == 0 && br.TasksCompleted > 0 {
		br.Cognitive.AutonomyLevel = 1.0
	} else if br.TasksCompleted > 0 {
		autonomy := 1.0 - float64(br.HumanInterventions)/float64(br.TasksCompleted)
		br.Cognitive.AutonomyLevel = clamp01(autonomy)
	} else {
		br.Cognitive.AutonomyLevel = 0.5
	}

	// Verification Rate
	if br.verificationAttempts > 0 {
		br.Cognitive.VerificationRate = clamp01(float64(br.verificationSuccesses) / float64(br.verificationAttempts))
	} else if br.TasksCompleted > 0 && br.TasksFailed == 0 {
		br.Cognitive.VerificationRate = 0.8
	} else {
		br.Cognitive.VerificationRate = 0
	}

	// Delivery Rate
	if br.deliveryAttempts > 0 {
		br.Cognitive.DeliveryRate = clamp01(float64(br.deliverySuccesses) / float64(br.deliveryAttempts))
	} else {
		completed := float64(br.TasksCompleted)
		planned := float64(br.TasksPlanned)
		if planned > 0 {
			br.Cognitive.DeliveryRate = clamp01(completed / planned)
		} else if br.TasksCompleted > 0 {
			br.Cognitive.DeliveryRate = 0.8
		} else {
			br.Cognitive.DeliveryRate = 0
		}
	}

	// VAD Score: Verified Autonomous Delivery (0-100)
	// Weighted composite of all cognitive dimensions
	br.Cognitive.VADScore = math.Round(
		(br.Cognitive.GoalComprehension*0.10 +
			br.Cognitive.PlanQuality*0.10 +
			br.Cognitive.PlanCompleteness*0.10 +
			br.Cognitive.AgentRouting*0.10 +
			br.Cognitive.KnowledgeUsage*0.10 +
			br.Cognitive.ContextQuality*0.05 +
			br.Cognitive.RecoveryRate*0.15 +
			br.Cognitive.AutonomyLevel*0.10 +
			br.Cognitive.VerificationRate*0.10 +
			br.Cognitive.DeliveryRate*0.15) * 100)

	// Compute avg latency
	if len(br.taskLatencies) > 0 {
		var sum time.Duration
		for _, l := range br.taskLatencies {
			sum += l
		}
		br.AvgLatency = sum / time.Duration(len(br.taskLatencies))
	}

	br.computeArchitectureAssessment()
	br.computeRecommendations()
}

func (br *BenchmarkReport) computeArchitectureAssessment() {
	if len(br.AgentsUsed) > 0 {
		br.ArchitectureSufficient = append(br.ArchitectureSufficient,
			fmt.Sprintf("Agent routing functional with %d unique agents", len(br.AgentsUsed)))
	}
	if br.Cognitive.PlanQuality >= 0.7 {
		br.ArchitectureSufficient = append(br.ArchitectureSufficient, "Planner produces viable task DAGs")
	}
	if br.Cognitive.RecoveryRate >= 0.5 {
		br.ArchitectureSufficient = append(br.ArchitectureSufficient, "Recovery loop operates effectively")
	}
	if br.Cognitive.AutonomyLevel >= 0.8 {
		br.ArchitectureSufficient = append(br.ArchitectureSufficient, "High autonomy — minimal human intervention")
	}
	if br.Cognitive.KnowledgeUsage >= 0.6 {
		br.ArchitectureSufficient = append(br.ArchitectureSufficient, "Knowledge integration works")
	}

	if br.Cognitive.PlanCompleteness < 0.5 {
		br.ArchitectureGaps = append(br.ArchitectureGaps, "Planner under-decomposes complex goals")
	}
	if br.Cognitive.ContextQuality < 0.6 {
		br.ArchitectureGaps = append(br.ArchitectureGaps, "Context delivery insufficient for some tasks")
	}
	if br.Cognitive.VerificationRate < 0.5 {
		br.ArchitectureGaps = append(br.ArchitectureGaps, "Verification pipeline needs improvement")
	}
	if br.Cognitive.DeliveryRate < 0.5 {
		br.ArchitectureGaps = append(br.ArchitectureGaps, "Delivery rate too low — tasks fail to complete")
	}
	if len(br.AgentsUsed) == 0 && br.TasksPlanned > 0 {
		br.ArchitectureGaps = append(br.ArchitectureGaps, "No agents available or routing failed")
	}
}

func (br *BenchmarkReport) computeRecommendations() {
	if br.Cognitive.VADScore >= 80 {
		br.Strengths = append(br.Strengths, "Fully autonomous delivery pipeline operational")
	}
	if br.Cognitive.VADScore >= 60 {
		br.Strengths = append(br.Strengths, "Reliable autonomous delivery with room for improvement")
	}
	if br.Cognitive.VADScore < 40 {
		br.Weaknesses = append(br.Weaknesses, "VAD score critically low — pipeline needs significant work")
	}
	if br.Cognitive.PlanQuality >= 0.8 {
		br.Strengths = append(br.Strengths, "Excellent planning — tasks decomposed effectively")
	}
	if br.Cognitive.PlanQuality < 0.3 {
		br.Weaknesses = append(br.Weaknesses, "Planning stage is the primary bottleneck")
	}
	if br.Cognitive.RecoveryRate >= 0.7 {
		br.Strengths = append(br.Strengths, "Strong error recovery — self-healing capability demonstrated")
	}
	if br.Cognitive.RecoveryRate < 0.3 && br.ErrorsEncountered > 0 {
		br.Weaknesses = append(br.Weaknesses, "Error recovery is poor — most failures are unrecoverable")
		br.Improvements = append(br.Improvements, "Strengthen recovery loop with better error classification")
	}
	if br.Cognitive.AutonomyLevel >= 0.9 {
		br.Strengths = append(br.Strengths, "Near-complete autonomy — zero or minimal human touches")
	}
	if br.HumanInterventions > 2 {
		br.Weaknesses = append(br.Weaknesses, fmt.Sprintf("Requires significant human intervention (%d times)", br.HumanInterventions))
		br.Improvements = append(br.Improvements, "Improve autonomous decision-making to reduce human-in-the-loop requirements")
	}
	if br.Cognitive.VerificationRate < 0.6 {
		br.Improvements = append(br.Improvements, "Add automated verification checks (build, test, lint) as mandatory gates")
	}
	if br.Cognitive.KnowledgeUsage < 0.5 {
		br.Improvements = append(br.Improvements, "Improve knowledge search integration — agent under-utilizes available knowledge")
	}
	if br.Cognitive.AgentRouting < 0.5 {
		br.Weaknesses = append(br.Weaknesses, "Agent routing is inaccurate — tasks assigned to wrong agents")
		br.Improvements = append(br.Improvements, "Refine agent capability matching and routing heuristics")
	}
	if br.Cognitive.ContextQuality < 0.6 {
		br.Improvements = append(br.Improvements, "Improve context assembly — agents lack sufficient information to complete tasks")
	}
	if len(br.ToolsUsed) == 0 && br.TasksPlanned > 0 {
		br.Weaknesses = append(br.Weaknesses, "No tool use detected — agents may lack tool access")
		br.Improvements = append(br.Improvements, "Ensure agents have appropriate tool contracts for task execution")
	}

	if br.TasksPlanned > 0 && br.TasksCompleted > 0 {
		pct := float64(br.TasksCompleted) / float64(br.TasksPlanned) * 100
		br.PlannedVsActual = fmt.Sprintf("Planned %d tasks, completed %d (%.0f%%). %d failed.",
			br.TasksPlanned, br.TasksCompleted, pct, br.TasksFailed)
		if br.TasksFailed > 0 {
			br.PlannedVsActual += " Investigation needed on failed tasks."
		}
	} else if br.TasksPlanned > 0 {
		br.PlannedVsActual = fmt.Sprintf("Planned %d tasks but none completed.", br.TasksPlanned)
	} else {
		br.PlannedVsActual = "No plan produced — execution never started."
	}
}

func (br *BenchmarkReport) GenerateReport() string {
	br.CalculateScores()

	var b strings.Builder

	b.WriteString("╔══════════════════════════════════════════════════════════════╗\n")
	b.WriteString("║         COSCA — COGNITIVE BENCHMARK REPORT                  ║\n")
	b.WriteString("║         Verified Autonomous Delivery (VAD)                  ║\n")
	b.WriteString("╚══════════════════════════════════════════════════════════════╝\n\n")

	b.WriteString(fmt.Sprintf("Session:      %s\n", br.SessionID))
	b.WriteString(fmt.Sprintf("Goal:         %s\n", br.Goal))
	b.WriteString(fmt.Sprintf("Duration:     %s\n", br.Duration.Round(time.Millisecond)))
	b.WriteString(fmt.Sprintf("Start:        %s\n", br.StartTime.Format("2006-01-02 15:04:05")))
	b.WriteString(fmt.Sprintf("End:          %s\n\n", br.EndTime.Format("2006-01-02 15:04:05")))

	b.WriteString(fmt.Sprintf("══════ VAD SCORE: %.0f / 100 ══════\n\n", br.Cognitive.VADScore))

	b.WriteString(br.vadBar())

	b.WriteString("\n── Cognitive Dimension Scores ──\n")
	b.WriteString(fmt.Sprintf("  Goal Comprehension:   %.0f%%\n", br.Cognitive.GoalComprehension*100))
	b.WriteString(fmt.Sprintf("  Plan Quality:         %.0f%%\n", br.Cognitive.PlanQuality*100))
	b.WriteString(fmt.Sprintf("  Plan Completeness:    %.0f%%\n", br.Cognitive.PlanCompleteness*100))
	b.WriteString(fmt.Sprintf("  Agent Routing:        %.0f%%\n", br.Cognitive.AgentRouting*100))
	b.WriteString(fmt.Sprintf("  Knowledge Usage:      %.0f%%\n", br.Cognitive.KnowledgeUsage*100))
	b.WriteString(fmt.Sprintf("  Context Quality:      %.0f%%\n", br.Cognitive.ContextQuality*100))
	b.WriteString(fmt.Sprintf("  Recovery Rate:        %.0f%%\n", br.Cognitive.RecoveryRate*100))
	b.WriteString(fmt.Sprintf("  Autonomy Level:       %.0f%%\n", br.Cognitive.AutonomyLevel*100))
	b.WriteString(fmt.Sprintf("  Verification Rate:    %.0f%%\n", br.Cognitive.VerificationRate*100))
	b.WriteString(fmt.Sprintf("  Delivery Rate:        %.0f%%\n", br.Cognitive.DeliveryRate*100))

	b.WriteString("\n── Execution Summary ──\n")
	b.WriteString(fmt.Sprintf("  Tasks Planned:     %d\n", br.TasksPlanned))
	b.WriteString(fmt.Sprintf("  Tasks Completed:   %d\n", br.TasksCompleted))
	b.WriteString(fmt.Sprintf("  Tasks Failed:      %d\n", br.TasksFailed))
	b.WriteString(fmt.Sprintf("  Agents Used:       %v\n", br.AgentsUsed))
	b.WriteString(fmt.Sprintf("  Models Used:       %v\n", br.ModelsUsed))
	b.WriteString(fmt.Sprintf("  Tools Used:        %v\n", br.ToolsUsed))
	b.WriteString(fmt.Sprintf("  Knowledge Items:   %d\n", br.KnowledgeItems))
	b.WriteString(fmt.Sprintf("  Memories Used:     %d\n", br.MemoriesUsed))
	b.WriteString(fmt.Sprintf("  Errors Encountered: %d\n", br.ErrorsEncountered))
	b.WriteString(fmt.Sprintf("  Errors Recovered:   %d\n", br.ErrorsRecovered))
	b.WriteString(fmt.Sprintf("  Human Interventions: %d\n", br.HumanInterventions))

	b.WriteString("\n── Cost & Performance ──\n")
	b.WriteString(fmt.Sprintf("  Total Cost:        $%.6f\n", br.TotalCost))
	b.WriteString(fmt.Sprintf("  Total Tokens:      %d\n", br.TotalTokens))
	b.WriteString(fmt.Sprintf("  Avg Latency:       %s\n", br.AvgLatency.Round(time.Millisecond)))

	b.WriteString("\n── Architecture Assessment ──\n")
	if len(br.ArchitectureSufficient) > 0 {
		b.WriteString("  Sufficient:\n")
		for _, s := range br.ArchitectureSufficient {
			b.WriteString(fmt.Sprintf("    ✓ %s\n", s))
		}
	}
	if len(br.ArchitectureGaps) > 0 {
		b.WriteString("  Gaps:\n")
		for _, g := range br.ArchitectureGaps {
			b.WriteString(fmt.Sprintf("    ✗ %s\n", g))
		}
	}
	if len(br.ArchitectureSufficient) == 0 && len(br.ArchitectureGaps) == 0 {
		b.WriteString("  No assessment data available.\n")
	}

	b.WriteString("\n── Recommendations ──\n")
	b.WriteString("  Strengths:\n")
	if len(br.Strengths) > 0 {
		for _, s := range br.Strengths {
			b.WriteString(fmt.Sprintf("    → %s\n", s))
		}
	} else {
		b.WriteString("    None identified.\n")
	}
	b.WriteString("  Weaknesses:\n")
	if len(br.Weaknesses) > 0 {
		for _, w := range br.Weaknesses {
			b.WriteString(fmt.Sprintf("    → %s\n", w))
		}
	} else {
		b.WriteString("    None identified.\n")
	}
	b.WriteString("  Improvements:\n")
	if len(br.Improvements) > 0 {
		for _, i := range br.Improvements {
			b.WriteString(fmt.Sprintf("    ↳ %s\n", i))
		}
	} else {
		b.WriteString("    No specific recommendations.\n")
	}

	b.WriteString("\n── Planned vs Actual ──\n")
	b.WriteString(fmt.Sprintf("  %s\n", br.PlannedVsActual))

	return b.String()
}

func (br *BenchmarkReport) vadBar() string {
	score := int(br.Cognitive.VADScore)
	filled := score / 5
	empty := 20 - filled
	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)

	grade := br.vadGrade(score)
	return fmt.Sprintf("  [%s] %d/100 — %s\n\n", bar, score, grade)
}

func (br *BenchmarkReport) vadGrade(score int) string {
	switch {
	case score >= 90:
		return "EXCEPTIONAL — Ready for autonomous production"
	case score >= 80:
		return "STRONG — Reliable autonomous delivery"
	case score >= 65:
		return "CAPABLE — Autonomous with occasional guidance"
	case score >= 50:
		return "DEVELOPING — Requires human oversight"
	case score >= 30:
		return "WEAK — Significant gaps in autonomy pipeline"
	default:
		return "CRITICAL — Pipeline not viable for autonomous delivery"
	}
}

func (br *BenchmarkReport) addAgent(agent string) {
	for _, a := range br.AgentsUsed {
		if a == agent {
			return
		}
	}
	br.AgentsUsed = append(br.AgentsUsed, agent)
	sort.Strings(br.AgentsUsed)
}

func (br *BenchmarkReport) addModel(model string) {
	for _, m := range br.ModelsUsed {
		if m == model {
			return
		}
	}
	br.ModelsUsed = append(br.ModelsUsed, model)
	sort.Strings(br.ModelsUsed)
}
