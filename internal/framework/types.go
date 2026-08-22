// Package framework provides validation and health reporting for the Cosca
// framework directory (.cosca/framework). It is the Go replacement for the
// legacy bash validators: validate-cross-references.sh, validate-conventions.sh,
// detect-orphans.sh, validate-orphans.sh and health-report.sh.
package framework

import "time"

// Severity represents the severity level of a validation issue.
type Severity string

// Severity levels for validation issues.
const (
	// SeverityError indicates a broken or invalid construct.
	SeverityError Severity = "error"
	// SeverityWarning indicates a potential problem that should be reviewed.
	SeverityWarning Severity = "warning"
	// SeverityInfo indicates informational feedback.
	SeverityInfo Severity = "info"
)

// Issue represents a single validation issue found in a framework file.
type Issue struct {
	Severity Severity `json:"severity" yaml:"severity"`
	File     string   `json:"file" yaml:"file"`
	Message  string   `json:"message" yaml:"message"`
}

// HealthReport holds the health inventory of a Cosca framework directory.
type HealthReport struct {
	Root               string    `json:"root" yaml:"root"`
	GeneratedAt        time.Time `json:"generated_at" yaml:"generated_at"`
	TotalMarkdownFiles int       `json:"total_markdown_files" yaml:"total_markdown_files"`
	Departments        int       `json:"departments" yaml:"departments"`
	Skills             int       `json:"skills" yaml:"skills"`
	Engines            int       `json:"engines" yaml:"engines"`
	Workflows          int       `json:"workflows" yaml:"workflows"`
	Templates          int       `json:"templates" yaml:"templates"`
	Agents             int       `json:"agents" yaml:"agents"`
	MemoryRecords      int       `json:"memory_records" yaml:"memory_records"`
	ADRs               int       `json:"adrs" yaml:"adrs"`
	BrokenLinks        int       `json:"broken_links" yaml:"broken_links"`
	OrphanCount        int       `json:"orphan_count" yaml:"orphan_count"`
	Issues             []Issue   `json:"issues,omitempty" yaml:"issues,omitempty"`
}
