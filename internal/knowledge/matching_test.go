//
// Tests for internal/knowledge/matching.go — Projeto → Dependency Detection →
// Knowledge Matching (NormalizeDependencyName, MatchDependency, MatchProject,
// Suggest).
//
// Cobre:
//   - NormalizeDependencyName: scopes npm, versões/ranges, case, caminhos Go
//   - MatchDependency: nil→missing, validated→verified, stale→stale,
//     manifest/none→partial
//   - MatchProject com PackageStore temporária (prisma validated, zod
//     manifest, gin ausente) + fallback para nome cru
//   - Suggest: textos pt-BR da ação
//
// Não toca em items/laws/evidence/discovery existentes — no-regression.
//

package knowledge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// NormalizeDependencyName
// =============================================================================

func TestNormalizeDependencyName(t *testing.T) {
	t.Parallel()

	tests := []struct{ in, want string }{
		{"prisma", "prisma"},
		{"prisma@6", "prisma"},
		{"prisma@^6.1.0", "prisma"},
		{"@types/react", "react"}, // scope npm best-effort
		{"@babel/core", "core"},   // scope npm com subpacote
		{"Express", "express"},    // case
		{"github.com/gin-gonic/gin", "gin"},
		{"github.com/jackc/pgx/v5", "pgx"}, // sufixo de versão de módulo Go
		{"github.com/foo/bar/v2", "bar"},
		{"google.golang.org/grpc", "grpc"},
		{"gopkg.in/yaml.v3", "yaml"},
		{"react@^18.2", "react"},
		{"requests>=2.0", "requests"}, // range de pip
		{"fastapi[all]", "fastapi"},   // extras de pip
		{"github:pgx/pgx", "pgx"},
		{"", ""},
		{"   ", ""},
	}

	for _, tt := range tests {
		if got := NormalizeDependencyName(tt.in); got != tt.want {
			t.Errorf("NormalizeDependencyName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// =============================================================================
// MatchDependency
// =============================================================================

func TestMatchDependency(t *testing.T) {
	t.Parallel()

	// nil → missing.
	assert.Equal(t, MatchMissing, MatchDependency("prisma", nil))

	validated := KnowledgePackage{
		ID: "prisma", Status: PackageStatusValidated, KnowledgeLevel: PackageKnowledgeValidated,
	}
	// Status validated OU knowledge_level validated → verified.
	assert.Equal(t, MatchVerified, MatchDependency("prisma", &validated))

	validatedStatusOnly := KnowledgePackage{
		ID: "zod", Status: PackageStatusValidated, KnowledgeLevel: PackageKnowledgeNone,
	}
	assert.Equal(t, MatchVerified, MatchDependency("zod", &validatedStatusOnly))

	validatedLevelOnly := KnowledgePackage{
		ID: "gin", Status: PackageStatusManifest, KnowledgeLevel: PackageKnowledgeValidated,
	}
	assert.Equal(t, MatchVerified, MatchDependency("gin", &validatedLevelOnly))

	// stale (aging marcou STALE) → stale, tanto em status quanto em level.
	staleStatus := KnowledgePackage{
		ID: "tokio", Status: "stale", KnowledgeLevel: PackageKnowledgeValidated,
	}
	assert.Equal(t, MatchStale, MatchDependency("tokio", &staleStatus))

	staleLevel := KnowledgePackage{
		ID: "serde", Status: PackageStatusValidated, KnowledgeLevel: "STALE",
	}
	assert.Equal(t, MatchStale, MatchDependency("serde", &staleLevel))

	// manifesto existe mas sem validação → partial.
	manifest := KnowledgePackage{
		ID: "zod", Status: PackageStatusManifest, KnowledgeLevel: PackageKnowledgeNone,
	}
	assert.Equal(t, MatchPartial, MatchDependency("zod", &manifest))

	partialLevel := KnowledgePackage{
		ID: "fiber", Status: PackageStatusAcquiring, KnowledgeLevel: PackageKnowledgePartial,
	}
	assert.Equal(t, MatchPartial, MatchDependency("fiber", &partialLevel))
}

// =============================================================================
// MatchProject
// =============================================================================

func TestMatchProject(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := NewPackageStore(root)

	// prisma validated → verified; zod manifest → partial; gin ausente → missing.
	require.NoError(t, store.Add(KnowledgePackage{
		ID: "prisma", Kind: PackageKindLibrary, Ecosystem: "typescript",
		Versions: []string{"6.x"}, Sources: []string{"official-docs"},
		KnowledgeLevel: PackageKnowledgeValidated, Status: PackageStatusValidated,
	}))
	require.NoError(t, store.Add(KnowledgePackage{
		ID: "zod", Kind: PackageKindLibrary, Ecosystem: "typescript",
		Versions: []string{"3.x"}, Sources: []string{"official-docs"},
		KnowledgeLevel: PackageKnowledgeNone, Status: PackageStatusManifest,
	}))

	matches, err := MatchProject([]string{"prisma", "zod", "gin"}, store, nil)
	require.NoError(t, err)
	require.Len(t, matches, 3)

	byDep := map[string]DependencyMatch{}
	for _, m := range matches {
		byDep[m.Dependency] = m
	}

	assert.Equal(t, MatchVerified, byDep["prisma"].Status)
	assert.Equal(t, "prisma", byDep["prisma"].PackageID)
	assert.Equal(t, PackageKnowledgeValidated, byDep["prisma"].KnowledgeLevel)
	assert.Equal(t, "conhecimento verificado", byDep["prisma"].Note)

	assert.Equal(t, MatchPartial, byDep["zod"].Status)
	assert.Equal(t, "zod", byDep["zod"].PackageID)
	assert.Equal(t, PackageKnowledgeNone, byDep["zod"].KnowledgeLevel)
	assert.Equal(t, "manifesto existe, sem validação", byDep["zod"].Note)

	assert.Equal(t, MatchMissing, byDep["gin"].Status)
	assert.Empty(t, byDep["gin"].PackageID)
	assert.Equal(t, PackageKnowledgeNone, byDep["gin"].KnowledgeLevel)
	assert.Equal(t, "sem conhecimento — ofereça aquisição", byDep["gin"].Note)
}

func TestMatchProject_NormalizedAndRawLookup(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := NewPackageStore(root)
	require.NoError(t, store.Add(KnowledgePackage{
		ID: "react", Kind: PackageKindLibrary, Ecosystem: "typescript",
		KnowledgeLevel: PackageKnowledgeValidated, Status: PackageStatusValidated,
	}))

	// Normalização resolve "@types/react" → "react" e "react@18" → "react".
	matches, err := MatchProject([]string{"@types/react", "react@18"}, store, nil)
	require.NoError(t, err)
	require.Len(t, matches, 2)
	for _, m := range matches {
		assert.Equal(t, MatchVerified, m.Status, "dep %q", m.Dependency)
		assert.Equal(t, "react", m.PackageID, "dep %q", m.Dependency)
	}

	// Lista vazia/nil → sem matches, sem erro.
	empty, err := MatchProject(nil, store, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)

	// Store nil → erro.
	if _, err := MatchProject([]string{"prisma"}, nil, nil); err == nil {
		t.Fatal("MatchProject com PackageStore nil deveria falhar")
	}
}

// =============================================================================
// Suggest
// =============================================================================

func TestSuggest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		m    DependencyMatch
		want string
	}{
		{name: "verified", m: DependencyMatch{Dependency: "prisma", Status: MatchVerified}, want: ""},
		{name: "stale", m: DependencyMatch{Dependency: "tokio", Status: MatchStale}, want: "revalide: cosca knowledge revalidate"},
		{name: "partial", m: DependencyMatch{Dependency: "zod", Status: MatchPartial}, want: "complete a aquisição"},
		{name: "missing", m: DependencyMatch{Dependency: "gin", Status: MatchMissing}, want: "ofereça: cosca knowledge add gin"},
		{name: "missing normalized", m: DependencyMatch{Dependency: "@types/react", Status: MatchMissing}, want: "ofereça: cosca knowledge add react"},
		{name: "missing versioned", m: DependencyMatch{Dependency: "prisma@6", Status: MatchMissing}, want: "ofereça: cosca knowledge add prisma"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.m.Suggest(); got != tt.want {
				t.Errorf("Suggest() = %q, want %q", got, tt.want)
			}
		})
	}
}
