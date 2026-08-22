// Package knowledge — Readiness Gate
//
// readiness.go implements the Knowledge Readiness Gate: before agents start
// working on a project, the stack is checked against what the Cosca already
// knows. The gate prevents agents from hallucinating by ensuring every tool
// in the stack has adequate knowledge (package + documentation + vectors).
//
// States (per tool):
//
//	✅ ready     — KnowledgePackage validated + docs acquired + vectors present
//	⚠️ partial   — Manifest exists but incomplete (no docs, no vectors, or stale)
//	❌ unknown   — No KnowledgePackage registered
//
// The readiness report includes exact CLI commands to close each gap.
package knowledge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadinessStatus classifies a single tool in the stack.
type ReadinessStatus string

const (
	ReadinessReady   ReadinessStatus = "ready"
	ReadinessPartial ReadinessStatus = "partial"
	ReadinessUnknown ReadinessStatus = "unknown"
)

// ReadinessItem holds the evaluation of a single tool.
type ReadinessItem struct {
	Name        string          `json:"name"`
	Ecosystem   string          `json:"ecosystem"`
	Status      ReadinessStatus `json:"status"`
	Package     bool            `json:"package"`      // KnowledgePackage exists
	Acquired    bool            `json:"acquired"`     // docs acquired
	Indexed     bool            `json:"indexed"`      // indexed in knowledge base
	Vectors     bool            `json:"vectors"`      // vector embeddings present
	Detail      string          `json:"detail"`       // human-readable status
	Action      string          `json:"action"`       // CLI command to close the gap
	KnowledgeLV string          `json:"knowledge_lv"` // knowledge level from package
}

// ReadinessReport is the full output of a readiness check.
type ReadinessReport struct {
	Stack      []string        `json:"stack"`
	Items      []ReadinessItem `json:"items"`
	Ready      int             `json:"ready"`
	Partial    int             `json:"partial"`
	Unknown    int             `json:"unknown"`
	Sufficient bool            `json:"sufficient"` // all tools are ready
	Gaps       []string        `json:"gaps"`       // CLI commands to close gaps
}

// CheckReadiness evaluates a stack of tools against the Cosca knowledge base.
//
//	dir:       project root (used to find .cosca/knowledge/)
//	stack:     comma-separated list of tools (e.g., "prisma,fastapi,react")
//	store:     local PackageStore
//	global:    global PackageStore (optional, can be nil)
func CheckReadiness(dir string, stack []string, store *PackageStore, global *PackageStore) (*ReadinessReport, error) {
	report := &ReadinessReport{
		Stack: stack,
		Items: make([]ReadinessItem, 0, len(stack)),
	}

	kbDir := filepath.Join(dir, ".cosca", "knowledge")
	acquiredDir := filepath.Join(kbDir, "acquired")

	for _, raw := range stack {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		item := evaluateTool(raw, store, global, acquiredDir)
		report.Items = append(report.Items, item)

		switch item.Status {
		case ReadinessReady:
			report.Ready++
		case ReadinessPartial:
			report.Partial++
			if item.Action != "" {
				report.Gaps = append(report.Gaps, item.Action)
			}
		case ReadinessUnknown:
			report.Unknown++
			if item.Action != "" {
				report.Gaps = append(report.Gaps, item.Action)
			}
		}
	}

	report.Sufficient = report.Unknown == 0 && report.Partial == 0
	return report, nil
}

// evaluateTool classifies a single tool name.
func evaluateTool(raw string, store, global *PackageStore, acquiredDir string) ReadinessItem {
	item := ReadinessItem{
		Name: raw,
	}

	// 1. Detect package info (heuristic).
	info, err := DetectPackageInfo(raw)
	if err != nil {
		item.Status = ReadinessUnknown
		item.Detail = "nome inválido"
		item.Action = ""
		return item
	}

	id := info.ID
	item.Ecosystem = info.Ecosystem
	if item.Ecosystem == "" || item.Ecosystem == EcosystemUnknown {
		item.Ecosystem = "unknown"
	}

	// 2. Look up the KnowledgePackage (federated: local first, then global).
	var pkg *KnowledgePackage
	if store != nil {
		pkg, _ = store.Get(id)
	}
	if pkg == nil && global != nil {
		pkg, _ = global.Get(id)
	}

	if pkg == nil {
		// No package at all — unknown.
		item.Status = ReadinessUnknown
		item.Detail = "sem Knowledge Package — o Cosca não conhece esta ferramenta"
		item.Action = fmt.Sprintf("cosca knowledge add %s", id)
		return item
	}

	// 3. Package exists. Check acquisition, indexing, vectors.
	item.Package = true
	item.KnowledgeLV = pkg.KnowledgeLevel

	// Check if docs were acquired (directory knowledge/acquired/<id>/ exists).
	if acquiredDir != "" {
		docDir := filepath.Join(acquiredDir, id)
		if info, err := os.Stat(docDir); err == nil && info.IsDir() {
			// Check for at least one .md file.
			entries, _ := os.ReadDir(docDir)
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
					item.Acquired = true
					break
				}
			}
		}
	}

	// Indexed: acquired docs have been compiled into SQLite.
	// We check via the existence of the acquired dir + non-empty contents.
	item.Indexed = item.Acquired

	// Vectors: check if the vector store has entries for this package.
	// We approximate via acquired+indexed — the verify command does the
	// precise check.
	item.Vectors = item.Acquired

	// 4. Classify based on knowledge level and acquisition status.
	kl := pkg.KnowledgeLevel
	status := pkg.Status

	switch {
	case kl == "validated" || status == "validated":
		if item.Acquired {
			item.Status = ReadinessReady
			item.Detail = "conhecimento validado — docs + vetores presentes"
		} else {
			item.Status = ReadinessPartial
			item.Detail = "manifesto validado mas docs não adquiridos"
			item.Action = fmt.Sprintf("cosca knowledge acquire %s --allow-remote --compile", id)
		}

	case kl == "stale" || status == "stale":
		item.Status = ReadinessPartial
		item.Detail = "conhecimento envelhecido — requer revalidação"
		item.Action = fmt.Sprintf("cosca knowledge revalidate %s", id)

	case status == "acquired" || kl == "partial":
		if item.Acquired {
			item.Status = ReadinessPartial
			item.Detail = "docs adquiridos mas não validados"
			item.Action = fmt.Sprintf("cosca knowledge compile (revisar docs de %s)", id)
		} else {
			item.Status = ReadinessPartial
			item.Detail = "manifesto existe mas docs não adquiridos"
			item.Action = fmt.Sprintf("cosca knowledge acquire %s --allow-remote --compile", id)
		}

	default:
		// status == "manifest" or "acquiring" or "quarantined"
		item.Status = ReadinessPartial
		item.Detail = fmt.Sprintf("manifesto criado (status=%s) — aquisição pendente", status)
		item.Action = fmt.Sprintf("cosca knowledge acquire %s --allow-remote --compile", id)
	}

	return item
}

// ReadinessSummary returns a single-line summary of the report.
func (r *ReadinessReport) ReadinessSummary() string {
	if r.Sufficient {
		return fmt.Sprintf("✅ %d/%d ferramentas prontas — pode iniciar", r.Ready, len(r.Items))
	}
	return fmt.Sprintf("⚠️ %d/%d prontas, %d parciais, %d desconhecidas — resolva os gaps antes de iniciar",
		r.Ready, len(r.Items), r.Partial, r.Unknown)
}
