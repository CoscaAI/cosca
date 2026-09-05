// Package security provides dependency vulnerability scanning backed by
// Google's osv-scanner (OSV.dev database). It is shared by the `cosca security
// scan` command, `cosca doctor` and the quality gate (`cosca qgate`).
package security

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/ossf/osv-schema/bindings/go/osvschema"

	"github.com/google/osv-scanner/v2/pkg/models"
	osvscanner "github.com/google/osv-scanner/v2/pkg/osvscanner"

	gocvss20 "github.com/pandatix/go-cvss/20"
	gocvss30 "github.com/pandatix/go-cvss/30"
	gocvss31 "github.com/pandatix/go-cvss/31"
	gocvss40 "github.com/pandatix/go-cvss/40"
)

// Options configures a dependency vulnerability scan.
type Options struct {
	// Dir is the directory to scan for dependency files.
	Dir string
	// Recursive walks subdirectories looking for dependency files.
	Recursive bool
	// MinSeverity is the minimum severity to report (CRITICAL/HIGH/MEDIUM/LOW,
	// case-insensitive). Defaults to LOW when empty. Vulnerabilities without a
	// severity rating are always reported.
	MinSeverity string
}

// Severity levels reported by the OSV ecosystem.
const (
	SeverityCritical = "CRITICAL"
	SeverityHigh     = "HIGH"
	SeverityMedium   = "MEDIUM"
	SeverityLow      = "LOW"
	SeverityUnknown  = "UNKNOWN"
)

// Vulnerability is a single known vulnerability affecting a package.
type Vulnerability struct {
	ID       string   `json:"id"`
	Aliases  []string `json:"aliases,omitempty"`
	Severity string   `json:"severity"`
	Score    string   `json:"score,omitempty"`
	Summary  string   `json:"summary,omitempty"`
	URL      string   `json:"url"`
}

// PackageVulns groups the vulnerabilities found for one package version.
type PackageVulns struct {
	Package         string          `json:"package"`
	Version         string          `json:"version"`
	Ecosystem       string          `json:"ecosystem"`
	Source          string          `json:"source"`
	Vulnerabilities []Vulnerability `json:"vulnerabilities"`
}

// ScanResult is the outcome of a dependency vulnerability scan.
type ScanResult struct {
	Dir                string         `json:"dir"`
	Sources            int            `json:"sources"`
	PackagesScanned    int            `json:"packages_scanned"`
	VulnerablePackages int            `json:"vulnerable_packages"`
	TotalVulns         int            `json:"total_vulnerabilities"`
	SeverityCounts     map[string]int `json:"severity_counts"`
	Vulnerabilities    []PackageVulns `json:"vulnerabilities"`
	// NoDependencyFiles is true when no dependency files were found in the dir.
	NoDependencyFiles bool `json:"no_dependency_files"`
}

// severityRank orders severities for filtering (higher is more severe).
var severityRank = map[string]int{
	SeverityCritical: 4,
	SeverityHigh:     3,
	SeverityMedium:   2,
	SeverityLow:      1,
	SeverityUnknown:  0,
}

// Scan runs an OSV dependency scan against the given directory using Google's
// osv-scanner. A result is always returned for a successful scan; network
// failures are surfaced as errors.
func Scan(ctx context.Context, opts Options) (*ScanResult, error) {
	if opts.Dir == "" {
		opts.Dir = "."
	}
	if opts.MinSeverity == "" {
		opts.MinSeverity = SeverityLow
	}
	opts.MinSeverity = strings.ToUpper(opts.MinSeverity)
	if _, ok := severityRank[opts.MinSeverity]; !ok {
		return nil, fmt.Errorf("invalid minimum severity %q (valid: CRITICAL, HIGH, MEDIUM, LOW)", opts.MinSeverity)
	}

	actions := osvscanner.ScannerActions{
		DirectoryPaths:  []string{opts.Dir},
		Recursive:       opts.Recursive,
		ShowAllPackages: true,
	}

	res, err := osvscanner.DoScan(actions)
	if errors.Is(err, osvscanner.ErrNoPackagesFound) {
		return &ScanResult{
			Dir:               opts.Dir,
			SeverityCounts:    map[string]int{},
			NoDependencyFiles: true,
		}, nil
	}
	if err != nil && !errors.Is(err, osvscanner.ErrVulnerabilitiesFound) {
		return nil, fmt.Errorf("dependency scan failed: %w", err)
	}

	return buildResult(opts, res)
}

// buildResult converts an osv-scanner result into a ScanResult, applying the
// minimum severity filter.
func buildResult(opts Options, res models.VulnerabilityResults) (*ScanResult, error) {
	minRank := severityRank[opts.MinSeverity]

	result := &ScanResult{
		Dir:            opts.Dir,
		SeverityCounts: map[string]int{},
	}

	sources := map[string]bool{}
	for _, src := range res.Results {
		sources[src.Source.Path] = true
		for _, pkg := range src.Packages {
			result.PackagesScanned++

			vulns := filterPackageVulns(pkg, minRank)
			if len(vulns) == 0 {
				continue
			}
			result.VulnerablePackages++

			pv := PackageVulns{
				Package:         pkg.Package.Name,
				Version:         pkg.Package.Version,
				Ecosystem:       pkg.Package.Ecosystem,
				Source:          src.Source.Path,
				Vulnerabilities: vulns,
			}
			result.Vulnerabilities = append(result.Vulnerabilities, pv)
			for _, v := range vulns {
				result.TotalVulns++
				result.SeverityCounts[v.Severity]++
			}
		}
	}

	result.Sources = len(sources)
	sort.Slice(result.Vulnerabilities, func(i, j int) bool {
		if result.Vulnerabilities[i].Package != result.Vulnerabilities[j].Package {
			return result.Vulnerabilities[i].Package < result.Vulnerabilities[j].Package
		}
		return result.Vulnerabilities[i].Version < result.Vulnerabilities[j].Version
	})

	return result, nil
}

// filterPackageVulns returns the vulnerabilities of a package that are at or
// above the minimum severity. Vulnerabilities with unknown severity are always
// kept so no known issue is silently hidden.
func filterPackageVulns(pkg models.PackageVulns, minRank int) []Vulnerability {
	out := make([]Vulnerability, 0, len(pkg.Vulnerabilities))
	for _, vuln := range pkg.Vulnerabilities {
		sev := vulnSeverity(vuln)
		if sev.Severity != SeverityUnknown && severityRank[sev.Severity] < minRank {
			continue
		}
		out = append(out, sev)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// vulnSeverity computes the severity of a single vulnerability from its CVSS
// vectors, mirroring osv-scanner's internal severity logic.
func vulnSeverity(vuln *osvschema.Vulnerability) Vulnerability {
	v := Vulnerability{
		ID:       vuln.GetId(),
		Aliases:  append([]string(nil), vuln.GetAliases()...),
		Summary:  vuln.GetSummary(),
		Severity: SeverityUnknown,
		URL:      "https://osv.dev/vulnerability/" + vuln.GetId(),
	}

	score := -1.0
	for _, sev := range vuln.GetSeverity() {
		var s float64
		var rating string
		switch sev.GetType() {
		case osvschema.Severity_CVSS_V2:
			vec, err := gocvss20.ParseVector(sev.GetScore())
			if err != nil {
				continue
			}
			s = vec.BaseScore()
			rating, _ = gocvss30.Rating(s)
		case osvschema.Severity_CVSS_V3:
			switch {
			case strings.HasPrefix(sev.GetScore(), "CVSS:3.0"):
				vec, err := gocvss30.ParseVector(sev.GetScore())
				if err != nil {
					continue
				}
				s = vec.BaseScore()
				rating, _ = gocvss30.Rating(s)
			case strings.HasPrefix(sev.GetScore(), "CVSS:3.1"):
				vec, err := gocvss31.ParseVector(sev.GetScore())
				if err != nil {
					continue
				}
				s = vec.BaseScore()
				rating, _ = gocvss31.Rating(s)
			}
		case osvschema.Severity_CVSS_V4:
			vec, err := gocvss40.ParseVector(sev.GetScore())
			if err != nil {
				continue
			}
			s = vec.Score()
			rating, _ = gocvss40.Rating(s)
		case osvschema.Severity_Ubuntu:
			v.Severity = strings.ToUpper(sev.GetScore())
			continue
		}
		if s > score {
			score = s
			v.Score = strconv.FormatFloat(s, 'f', 1, 64)
			if rating != "" {
				v.Severity = strings.ToUpper(rating)
			}
		}
	}

	return v
}

// CriticalHigh counts vulnerabilities rated CRITICAL or HIGH in the result.
func (r *ScanResult) CriticalHigh() int {
	return r.SeverityCounts[SeverityCritical] + r.SeverityCounts[SeverityHigh]
}

// HasCriticalHigh reports whether the result contains CRITICAL or HIGH
// vulnerabilities.
func (r *ScanResult) HasCriticalHigh() bool {
	return r.CriticalHigh() > 0
}
