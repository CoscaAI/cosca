//
// Tests for internal/knowledge/resolver.go — Knowledge Resolver + Gap
// Detection (aquisição adaptativa).
//
// Cobre:
//   - ResolveTask: dep verified + versão coberta → sufficient; dep verified mas
//     versão ausente (6.0 ✓ / 6.1 ✗) → gap; dep ausente → gap com next_action
//     "adquirir"
//   - parseDepVersion/coversVersion: extração e cobertura de versão
//   - SuggestSources: cascata de 8 níveis em ordem
//   - VoiceSummary: suficiente → "", lacuna → "Encontrei uma lacuna de
//     conhecimento: ...", follow-up que resolve → "validado. continuei a
//     implementação."
//
// Não toca em matching/package/diff existentes — no-regression.
//

package knowledge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// parseDepVersion
// =============================================================================

func TestParseDepVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in, want string
		ok       bool
	}{
		{"prisma@6.1", "6.1", true},
		{"prisma@^6.1.0", "^6.1.0", true},
		{"prisma@6", "6", true},
		{"@types/react@^5.0", "^5.0", true},
		{"github.com/foo/bar@v1.2.3", "v1.2.3", true},
		{"prisma@*", "*", true},
		{"prisma", "", false},
		{"@types/react", "", false}, // scope sem versão
		{"", "", false},
	}
	for _, tt := range tests {
		got, ok := parseDepVersion(tt.in)
		assert.Equal(t, tt.ok, ok, "parseDepVersion(%q) ok", tt.in)
		assert.Equal(t, tt.want, got, "parseDepVersion(%q) versão", tt.in)
	}
}

// =============================================================================
// coversVersion
// =============================================================================

func TestCoversVersion(t *testing.T) {
	t.Parallel()

	pkg := KnowledgePackage{
		ID: "prisma", Versions: []string{"6.0"},
	}
	assert.False(t, pkg.coversVersion("6.1"), "6.0 não cobre 6.1")
	assert.True(t, pkg.coversVersion("6.0"), "6.0 cobre 6.0")
	assert.True(t, pkg.coversVersion("6.0.5"), "6.0 cobre 6.0.5")

	wild := KnowledgePackage{ID: "prisma", Versions: []string{"6.x"}}
	assert.True(t, wild.coversVersion("6.1"), "6.x cobre 6.1")
	assert.True(t, wild.coversVersion("^6.2.0"), "6.x cobre ^6.2.0")
	assert.False(t, wild.coversVersion("7.0"), "6.x não cobre 7.0")

	all := KnowledgePackage{ID: "prisma", Versions: []string{"*"}}
	assert.True(t, all.coversVersion("6.1"), "* cobre 6.1")

	// VersionDiffs ("6.0->6.1") também cobrem a versão pedida.
	diffed := KnowledgePackage{
		ID: "prisma", Versions: []string{"6.0"},
		VersionDiffs: map[string][]DiffEntry{
			"6.0->6.1": {{Kind: DiffKindNew, Symbol: "transaction", Detail: "nova API"}},
		},
	}
	assert.True(t, diffed.coversVersion("6.1"), "VersionDiffs 6.0->6.1 cobre 6.1")
}

// =============================================================================
// ResolveTask
// =============================================================================

// resolverStore monta uma PackageStore temporária com prisma: versões e status
// controlados pelo caller. KnowledgeLevel segue o status (validated → validated,
// senão none) para que MatchDependency classifique como esperado.
func resolverStore(t *testing.T, prismaVersions []string, prismaStatus string) *PackageStore {
	t.Helper()
	store := NewPackageStore(t.TempDir())
	level := PackageKnowledgeNone
	if prismaStatus == PackageStatusValidated {
		level = PackageKnowledgeValidated
	}
	require.NoError(t, store.Add(KnowledgePackage{
		ID: "prisma", Kind: PackageKindLibrary, Ecosystem: "typescript",
		Versions:       prismaVersions,
		Sources:        []string{"official-docs"},
		KnowledgeLevel: level,
		Status:         prismaStatus,
	}))
	return store
}

func TestResolveTask_Sufficient(t *testing.T) {
	t.Parallel()

	// dep verified + versão coberta (6.x cobre 6.1) → suficiente.
	store := resolverStore(t, []string{"6.x"}, PackageStatusValidated)
	res, err := ResolveTask("create transaction", []string{"prisma@6.1"}, store, nil)
	require.NoError(t, err)
	assert.True(t, res.Sufficient)
	assert.Empty(t, res.Gaps)
	assert.Equal(t, "continua", res.NextAction)
	require.Len(t, res.Matches, 1)
	assert.Equal(t, MatchVerified, res.Matches[0].Status)

	// Sem pedido de versão ("prisma") → suficiente também.
	res, err = ResolveTask("create transaction", []string{"prisma"}, store, nil)
	require.NoError(t, err)
	assert.True(t, res.Sufficient)
	assert.Empty(t, res.Gaps)
	assert.Equal(t, "continua", res.NextAction)
}

func TestResolveTask_VersionGap(t *testing.T) {
	t.Parallel()

	// dep verified mas versão ausente: conhecimento 6.0 ✓ / 6.1 ✗ → gap.
	store := resolverStore(t, []string{"6.0"}, PackageStatusValidated)
	res, err := ResolveTask("create transaction", []string{"prisma@6.1"}, store, nil)
	require.NoError(t, err)
	assert.False(t, res.Sufficient)
	require.Len(t, res.Gaps, 1)

	g := res.Gaps[0]
	assert.Equal(t, "prisma", g.PackageID)
	assert.Equal(t, "6.1", g.Version)
	assert.Equal(t, "transaction", g.API)
	assert.Equal(t, []GapField{GapVersionCompat}, g.Fields)
	assert.Contains(t, g.Reason, "prisma 6.1 não coberto")
	assert.Contains(t, g.Reason, "6.0")
	assert.Equal(t, []string{"official-docs", "official-repository", "release-notes"}, g.Sources)
	assert.Contains(t, res.NextAction, "adquirir: cosca knowledge add prisma")
	assert.Contains(t, res.VoiceSummary(), "prisma 6.1/transaction")
}

func TestResolveTask_MissingDep(t *testing.T) {
	t.Parallel()

	// dep ausente da PackageStore → gap com next_action "adquirir".
	store := resolverStore(t, []string{"6.x"}, PackageStatusValidated)
	res, err := ResolveTask("create transaction", []string{"gin"}, store, nil)
	require.NoError(t, err)
	assert.False(t, res.Sufficient)
	require.Len(t, res.Gaps, 1)

	g := res.Gaps[0]
	assert.Equal(t, "gin", g.PackageID)
	assert.Empty(t, g.Version)
	assert.Equal(t, []GapField{GapSyntax}, g.Fields)
	assert.Contains(t, g.Reason, "sem conhecimento para gin")
	assert.Equal(t, "adquirir: cosca knowledge add gin", res.NextAction)
}

func TestResolveTask_StaleAndPartial(t *testing.T) {
	t.Parallel()

	store := resolverStore(t, []string{"6.x"}, "stale")
	res, err := ResolveTask("criar schema", []string{"prisma@6.1"}, store, nil)
	require.NoError(t, err)
	assert.False(t, res.Sufficient)
	require.Len(t, res.Gaps, 1)
	assert.Equal(t, []GapField{GapErrorBehavior}, res.Gaps[0].Fields)
	assert.Contains(t, res.Gaps[0].Reason, "envelhecido")

	store = resolverStore(t, []string{"6.x"}, PackageStatusManifest)
	res, err = ResolveTask("criar schema", []string{"prisma@6.1"}, store, nil)
	require.NoError(t, err)
	assert.False(t, res.Sufficient)
	require.Len(t, res.Gaps, 1)
	assert.Equal(t, []GapField{GapOptions}, res.Gaps[0].Fields)
	assert.Contains(t, res.Gaps[0].Reason, "parcial")
}

func TestResolveTask_MixedAndDedup(t *testing.T) {
	t.Parallel()

	// gin ausente + prisma com versão fora do conhecimento → 2 gaps; a próxima
	// ação deduplica "cosca knowledge add prisma".
	store := resolverStore(t, []string{"6.0"}, PackageStatusValidated)
	res, err := ResolveTask("criar transações", []string{"prisma@6.1", "gin"}, store, nil)
	require.NoError(t, err)
	assert.False(t, res.Sufficient)
	require.Len(t, res.Gaps, 2)
	assert.Contains(t, res.NextAction, "adquirir:")
	assert.Contains(t, res.NextAction, "cosca knowledge add prisma")
	assert.Contains(t, res.NextAction, "cosca knowledge add gin")
}

func TestResolveTask_NilStore(t *testing.T) {
	t.Parallel()

	if _, err := ResolveTask("create transaction", []string{"prisma"}, nil, nil); err == nil {
		t.Fatal("ResolveTask com PackageStore nil deveria falhar")
	}
}

// =============================================================================
// SuggestSources
// =============================================================================

func TestSuggestSources_CascadeInOrder(t *testing.T) {
	t.Parallel()

	want := []string{
		"local-knowledge",
		"existing-evidence",
		"official-docs",
		"official-repository",
		"release-notes",
		"tests-examples",
		"community",
		"llm-synthesis",
	}
	got := SuggestSources(KnowledgeGap{PackageID: "prisma"})
	assert.Equal(t, want, got, "cascata de 8 níveis na ordem determinística")
	assert.True(t, len(got) > 0 && &got[0] != &want[0],
		"deve devolver uma cópia, nunca o slice interno (ponteiro do 1º elemento difere)")
	got[0] = "mutated"
	assert.Equal(t, "local-knowledge", want[0], "mutações no retorno não afetam a cascata interna")
}

// =============================================================================
// VoiceSummary
// =============================================================================

func TestVoiceSummary(t *testing.T) {
	t.Parallel()

	// Suficiente (primeira resolução) → "".
	sufficient := &ResolveResult{Sufficient: true, NextAction: "continua"}
	assert.Empty(t, sufficient.VoiceSummary())

	// Lacunas → "Encontrei uma lacuna de conhecimento: {pkg} {version}/{api}.
	// Estou validando antes de continuar."
	gap := &ResolveResult{
		Sufficient: false,
		Gaps: []KnowledgeGap{
			{PackageID: "prisma", Version: "6.1", API: "transaction"},
		},
	}
	want := "Encontrei uma lacuna de conhecimento: prisma 6.1/transaction. Estou validando antes de continuar."
	assert.Equal(t, want, gap.VoiceSummary())

	// Lacuna sem versão/API.
	gapNoAPI := &ResolveResult{
		Sufficient: false,
		Gaps:       []KnowledgeGap{{PackageID: "gin"}},
	}
	assert.Equal(t, "Encontrei uma lacuna de conhecimento: gin. Estou validando antes de continuar.", gapNoAPI.VoiceSummary())

	// Follow-up que resolve as lacunas → "validado. continuei a implementação."
	followUp := &ResolveResult{Sufficient: true, NextAction: "continua", ResolvedFollowUp: true}
	assert.Equal(t, "validado. continuei a implementação.", followUp.VoiceSummary())

	// Nil → "".
	var nilRes *ResolveResult
	assert.Empty(t, nilRes.VoiceSummary())
}

// =============================================================================
// detectAPIFromTask
// =============================================================================

func TestDetectAPIFromTask(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "transaction", detectAPIFromTask("create transaction", nil))
	// "prisma" e "6.1" são excluídos (ids de pacote + versão).
	assert.Equal(t, "x", detectAPIFromTask("Implemente X usando Prisma 6.1", map[string]bool{"prisma": true}))
	assert.Empty(t, detectAPIFromTask("create", nil))
}
