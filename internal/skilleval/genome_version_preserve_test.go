package skilleval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGenomeApply_PreservesFrontmatterFlags é a regressão da F1.1: o bug
// histórico era o Genome.Apply re-marshalar só name/description/level e
// DESCARTAR version/category/status/license/metadata ao evoluir uma skill.
// Com o fix, o frontmatter original é re-emitido integralmente.
func TestGenomeApply_PreservesFrontmatterFlags(t *testing.T) {
	doc := `---
name: alpha-skill
description: handles retries with backoff
version: 2.4.0
category: reliability
license: MIT
compatibility: linux, windows
allowed-tools: bash,file_read
---

# ALPHA

Some body that may be mutated.
`
	p := filepath.Join(t.TempDir(), "SKILL.md")
	if err := os.WriteFile(p, []byte(doc), 0644); err != nil {
		t.Fatal(err)
	}

	g, err := ParseSkill(p)
	if err != nil {
		t.Fatalf("ParseSkill: %v", err)
	}

	out := g.Apply()

	// Re-emitido deve preservar todos os campos de frontmatter.
	for _, want := range []string{"version: 2.4.0", "category: reliability", "license: MIT",
		"compatibility: linux, windows", "allowed-tools: bash,file_read"} {
		if !strings.Contains(out, want) {
			t.Errorf("Apply() perdeu frontmatter %q:\n%s", want, out)
		}
	}

	// E a identidade continua reconstruível: reparse mantém name/description/level.
	rp := filepath.Join(t.TempDir(), "SKILL.md")
	if err := os.WriteFile(rp, []byte(out), 0644); err != nil {
		t.Fatal(err)
	}
	reparsed, err := ParseSkill(rp)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if reparsed.Identity.Name != "alpha-skill" || reparsed.Identity.Description != "handles retries with backoff" || reparsed.Identity.Level != 0 {
		t.Errorf("identity not preserved after Apply: %+v", reparsed.Identity)
	}
}

// TestGenomeApply_FallbackSemFrontmatter cobre o fallback quando um Genome é
// construído sem o frontmatter bruto (ainda renderiza a identidade).
func TestGenomeApply_FallbackSemFrontmatter(t *testing.T) {
	g := &Genome{
		Identity: Skill{Name: "fallback-skill", Description: "d", Level: 2},
		Body:     "body here",
	}
	out := g.Apply()
	for _, want := range []string{"name: fallback-skill", "description: d", "level: 2", "body here"} {
		if !strings.Contains(out, want) {
			t.Errorf("fallback Apply() faltando %q:\n%s", want, out)
		}
	}
}
