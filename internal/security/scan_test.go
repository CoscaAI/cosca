// Unit tests for the internal/security scan engine — severity computation,
// result building and filtering. All offline-safe (no network access).
package security

import (
	"testing"

	"github.com/google/osv-scanner/v2/pkg/models"
	"github.com/ossf/osv-schema/bindings/go/osvschema"
)

// =============================================================================
// vulnSeverity — CVSS vector parsing
// =============================================================================

func TestVulnSeverity_CVSS31High(t *testing.T) {
	vuln := &osvschema.Vulnerability{
		Id:      "CVE-2026-0001",
		Summary: "test vuln",
		Severity: []*osvschema.Severity{
			{
				Type:  osvschema.Severity_CVSS_V3,
				Score: "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
			},
		},
	}
	v := vulnSeverity(vuln)
	if v.Severity != SeverityHigh && v.Severity != SeverityCritical {
		t.Errorf("expected HIGH/CRITICAL for 9.8 CVSS, got %q", v.Severity)
	}
	if v.ID != "CVE-2026-0001" {
		t.Errorf("expected ID CVE-2026-0001, got %q", v.ID)
	}
	if v.URL != "https://osv.dev/vulnerability/CVE-2026-0001" {
		t.Errorf("unexpected URL %q", v.URL)
	}
}

func TestVulnSeverity_NoSeverityUnknown(t *testing.T) {
	vuln := &osvschema.Vulnerability{Id: "GO-2026-0001", Summary: "no cvss"}
	v := vulnSeverity(vuln)
	if v.Severity != SeverityUnknown {
		t.Errorf("expected UNKNOWN severity, got %q", v.Severity)
	}
}

func TestVulnSeverity_InvalidVector(t *testing.T) {
	vuln := &osvschema.Vulnerability{
		Id: "GO-2026-0002",
		Severity: []*osvschema.Severity{
			{Type: osvschema.Severity_CVSS_V3, Score: "not-a-vector"},
		},
	}
	v := vulnSeverity(vuln)
	if v.Severity != SeverityUnknown {
		t.Errorf("expected UNKNOWN for invalid vector, got %q", v.Severity)
	}
}

func TestVulnSeverity_CVSS4(t *testing.T) {
	vuln := &osvschema.Vulnerability{
		Id: "GHSA-2026-0001",
		Severity: []*osvschema.Severity{
			{Type: osvschema.Severity_CVSS_V4, Score: "CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:N/VC:H/VI:H/VA:H/SC:N/SI:N/SA:N"},
		},
	}
	v := vulnSeverity(vuln)
	if v.Severity != SeverityCritical && v.Severity != SeverityHigh {
		t.Errorf("expected CRITICAL/HIGH for CVSS4 10.0, got %q", v.Severity)
	}
	if v.Score == "" {
		t.Error("expected non-empty score for CVSS4")
	}
}

// =============================================================================
// buildResult — empty results
// =============================================================================

func TestBuildResult_NoResults(t *testing.T) {
	res := models.VulnerabilityResults{}
	result, err := buildResult(Options{Dir: ".", MinSeverity: SeverityLow}, res)
	if err != nil {
		t.Fatalf("buildResult error: %v", err)
	}
	if result.PackagesScanned != 0 {
		t.Errorf("expected 0 packages, got %d", result.PackagesScanned)
	}
	if result.NoDependencyFiles {
		t.Error("expected NoDependencyFiles=false for empty results")
	}
}

// =============================================================================
// Invalid minimum severity
// =============================================================================

func TestScan_InvalidMinSeverity(t *testing.T) {
	_, err := Scan(t.Context(), Options{Dir: ".", MinSeverity: "banana"})
	if err == nil {
		t.Fatal("expected error for invalid min severity")
	}
}

// =============================================================================
// CriticalHigh counting
// =============================================================================

func TestScanResult_CriticalHigh(t *testing.T) {
	r := &ScanResult{
		SeverityCounts: map[string]int{
			SeverityCritical: 1,
			SeverityHigh:     2,
			SeverityMedium:   5,
		},
	}
	if r.CriticalHigh() != 3 {
		t.Errorf("expected CriticalHigh()=3, got %d", r.CriticalHigh())
	}
	if !r.HasCriticalHigh() {
		t.Error("expected HasCriticalHigh()=true")
	}

	clean := &ScanResult{SeverityCounts: map[string]int{SeverityMedium: 2}}
	if clean.HasCriticalHigh() {
		t.Error("medium-only result must not have critical/high")
	}
}
