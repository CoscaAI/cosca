// Package oracle provides orchestration hooks — the bridge between
// the Cosca Kernel and the deterministic Oracle.
package oracle

import "time"

// ============================================================
// ORCHESTRATION HOOKS
// ============================================================
//
// These are the functions the Kernel calls during orchestration:
//
//  1. ClassifyOrder  — "what type of task is this?" → routing
//  2. PreFlightCheck — "is this code safe to delegate?" → gate
//  3. AssessChange   — "what's the risk of this change?" → priority
//  4. AgentTrust     — "how reliable is this agent?" → review intensity
//
// All hooks are deterministic, local, and audited.

// ClassifyOrder is the Kernel hook to classify a Don order.
// Returns routing info WITHOUT LLM.
func ClassifyOrder(o *Oracle, caller, order string) (*Classification, error) {
	return o.Classify(caller, order)
}

// PreFlightCheck is the Kernel hook to gate a delegation.
// Scans the target file and returns risk before spending LLM budget.
func PreFlightCheck(o *Oracle, caller, filePath string) (*PreFlight, error) {
	return o.PreFlight(caller, filePath)
}

// AssessChange assesses the risk of a multi-file change.
func (o *Oracle) AssessChange(caller string, files []string) (*RiskReport, error) {
	report := &RiskReport{
		Files:      make([]*FileRisk, 0),
		AssessedAt: time.Now(),
	}

	totalCritical := 0
	totalWarnings := 0

	for _, file := range files {
		pf, err := o.PreFlight(caller, file)
		if err != nil {
			// File might not exist or not be Go — skip quietly
			continue
		}

		fr := &FileRisk{
			File:     file,
			Critical: pf.Critical,
			Warnings: pf.Warnings,
		}
		report.Files = append(report.Files, fr)
		totalCritical += pf.Critical
		totalWarnings += pf.Warnings
	}

	// Overall risk level
	report.TotalCritical = totalCritical
	report.TotalWarnings = totalWarnings
	switch {
	case totalCritical > 0:
		report.Risk = "high"
		report.Reason = "critical issues found in change"
	case totalWarnings > 5:
		report.Risk = "medium"
		report.Reason = "multiple warnings in change"
	default:
		report.Risk = "low"
		report.Reason = "no critical issues in change"
	}

	o.auditEntry(caller, "assess-change", report.Risk)
	return report, nil
}

// RiskReport summarizes the risk of a change.
type RiskReport struct {
	Files         []*FileRisk `json:"files"`
	TotalCritical int         `json:"total_critical"`
	TotalWarnings int         `json:"total_warnings"`
	Risk          string      `json:"risk"` // high, medium, low
	Reason        string      `json:"reason"`
	AssessedAt    time.Time   `json:"assessed_at"`
}

// FileRisk is per-file risk in a change.
type FileRisk struct {
	File     string `json:"file"`
	Critical int    `json:"critical"`
	Warnings int    `json:"warnings"`
}

// AgentTrust returns the Kernel hook for agent reliability.
func AgentTrust(o *Oracle, caller, agent string) (float64, error) {
	return o.Trust(caller, agent)
}
