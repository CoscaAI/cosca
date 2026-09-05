package cli

import (
	"context"

	"github.com/CoscaAI/cosca/internal/diagnostics"
)

// Diagnostics Adapter
// =============================================================================

// diagnosticsAdapter wraps diagnostics.Engine.
type diagnosticsAdapter struct {
	inner *diagnostics.Engine
}

// newDiagnosticsAdapter creates a diagnostics adapter.
func newDiagnosticsAdapter(_ string) *diagnosticsAdapter {
	return &diagnosticsAdapter{inner: diagnostics.NewEngine()}
}

// QuickCheck runs a quick health check. Returns a report with health booleans.
func (a *diagnosticsAdapter) QuickCheck() QuickCheckReport {
	report := a.inner.RunAll(context.Background())
	qc := QuickCheckReport{}
	for _, check := range report.Checks {
		switch check.Name {
		case "runtime":
			qc.RuntimeOK = check.Status == diagnostics.StatusPass
		case "indexer", "knowledge":
			qc.IndexOK = check.Status == diagnostics.StatusPass
		case "memory":
			qc.MemoryOK = check.Status == diagnostics.StatusPass
		case "database", "filesystem":
			qc.DatabaseOK = check.Status == diagnostics.StatusPass
		}
	}
	return qc
}

// QuickCheckReport holds quick check results.
type QuickCheckReport struct {
	RuntimeOK  bool `json:"runtime_ok"`
	IndexOK    bool `json:"index_ok"`
	MemoryOK   bool `json:"memory_ok"`
	DatabaseOK bool `json:"database_ok"`
}

// =============================================================================
