package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeFile creates a file (and parent dirs) under root with the given content.
func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}

// writeManifest generates the canonical snapshot for the current live tree and
// writes it to root/catalog.manifest.
func writeManifest(t *testing.T, root string) {
	t.Helper()
	text, err := Generate(root)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, ManifestFileName), []byte(text), 0o644))
}

// validPrompt is a PROMPT.md with valid frontmatter and a self-contained link.
const validPrompt = "---\nname: cosca-ai\ndescription: Test agent.\nlevel: 3\n---\n\n[INDEX](INDEX.md)\n"

// noFrontmatter is a SKILL.md without any YAML frontmatter.
const noFrontmatter = "> **Version**: 1.0.0\n\n# SKILL\n"

// TestCheckDrift_Scenarios is the table-driven suite for the generate-and-diff
// DRIFT check (manifest vs live tree). It must NOT evaluate invariants A/B/C.
func TestCheckDrift_Scenarios(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T, root string)
		wantPass bool
		wantDrift bool
		wantKind string // non-empty => must appear in violations
	}{
		{
			name: "sem snapshot => drift (manda rodar --generate)",
			setup: func(t *testing.T, root string) {
				writeFile(t, root, "agents/cosca-ai/INDEX.md", "# ai\n")
			},
			wantPass:  false,
			wantDrift: true,
			wantKind:  KindManifestDrift,
		},
		{
			name: "snapshot igual ao vivo => passa",
			setup: func(t *testing.T, root string) {
				writeFile(t, root, "agents/cosca-ai/INDEX.md", "# ai\n")
				writeFile(t, root, "engines/audit/INDEX.md", "# audit\n")
				writeManifest(t, root)
			},
			wantPass:  true,
			wantDrift: false,
			wantKind:  "",
		},
		{
			name: "coluna nova no vivo => drift",
			setup: func(t *testing.T, root string) {
				writeFile(t, root, "agents/cosca-ai/INDEX.md", "# ai\n")
				writeManifest(t, root)
				writeFile(t, root, "agents/cosca-new/PROMPT.md", validPrompt)
			},
			wantPass:  false,
			wantDrift: true,
			wantKind:  KindManifestDrift,
		},
		{
			name: "coluna removida do vivo => drift",
			setup: func(t *testing.T, root string) {
				writeFile(t, root, "agents/cosca-ai/INDEX.md", "# ai\n")
				writeFile(t, root, "agents/cosca-b/INDEX.md", "# b\n")
				writeManifest(t, root)
				require.NoError(t, os.RemoveAll(filepath.Join(root, "agents", "cosca-b")))
			},
			wantPass:  false,
			wantDrift: true,
			wantKind:  KindManifestDrift,
		},
		{
			name: "nome canonico mudou => drift",
			setup: func(t *testing.T, root string) {
				writeFile(t, root, "agents/cosca-ai/INDEX.md", "# ai\n")
				writeFile(t, root, "agents/cosca-ai/PROMPT.md",
					"---\nname: foo-a\ndescription: d\n---\n")
				writeManifest(t, root)
				writeFile(t, root, "agents/cosca-ai/PROMPT.md",
					"---\nname: foo-b\ndescription: d\n---\n")
			},
			wantPass:  false,
			wantDrift: true,
			wantKind:  KindManifestDrift,
		},
		{
			name: "INDEX ausente NAO e drift (e invariante A, audit)",
			setup: func(t *testing.T, root string) {
				writeFile(t, root, "agents/cosca-ai/PROMPT.md", validPrompt)
				writeManifest(t, root)
			},
			wantPass:  true,
			wantDrift: false,
			wantKind:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			tt.setup(t, root)

			rep, err := CheckDrift(root)
			require.NoError(t, err)
			assert.Equal(t, ModeCheck, rep.Mode, "mode mismatch")
			assert.Equal(t, tt.wantPass, rep.Pass, "pass mismatch")
			assert.Equal(t, tt.wantDrift, rep.Drift, "drift mismatch")
			if tt.wantKind != "" {
				found := false
				for _, v := range rep.Violations {
					if v.Kind == tt.wantKind {
						found = true
						break
					}
				}
				assert.True(t, found, "esperava violação do tipo %q, obtido %+v", tt.wantKind, rep.Violations)
			}
		})
	}
}

// TestAuditInvariants_Scenarios exercises the three invariants (A/B/C) as
// NON-BLOCKING debt — findings are reported, no manifest is required.
func TestAuditInvariants_Scenarios(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T, root string)
		wantPass bool
		wantKind string // non-empty => must appear in violations
	}{
		{
			name: "INDEX faltando => achado",
			setup: func(t *testing.T, root string) {
				writeFile(t, root, "agents/cosca-ai/PROMPT.md", validPrompt)
			},
			wantPass: false,
			wantKind: KindIndexMissing,
		},
		{
			name: "frontmatter invalido => achado",
			setup: func(t *testing.T, root string) {
				writeFile(t, root, "engines/audit/SKILL.md", noFrontmatter)
			},
			wantPass: false,
			wantKind: KindFrontmatter,
		},
		{
			name: "dangling ref => achado",
			setup: func(t *testing.T, root string) {
				writeFile(t, root, "agents/cosca-ai/INDEX.md", "# cosca-ai\n")
				writeFile(t, root, "agents/cosca-ai/PROMPT.md",
					"---\nname: cosca-ai\ndescription: Test agent.\n---\n\n[missing](does-not-exist.md)\n")
			},
			wantPass: false,
			wantKind: KindDanglingLink,
		},
		{
			name: "tudo valido => nenhum achado",
			setup: func(t *testing.T, root string) {
				writeFile(t, root, "agents/cosca-ai/INDEX.md", "# cosca-ai\n")
				writeFile(t, root, "agents/cosca-ai/PROMPT.md", validPrompt)
			},
			wantPass: true,
			wantKind: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			tt.setup(t, root)

			rep, err := AuditInvariants(root)
			require.NoError(t, err)
			assert.Equal(t, ModeAudit, rep.Mode, "mode mismatch")
			assert.Equal(t, tt.wantPass, rep.Pass, "pass mismatch")
			if tt.wantKind != "" {
				found := false
				for _, v := range rep.Violations {
					if v.Kind == tt.wantKind {
						found = true
						break
					}
				}
				assert.True(t, found, "esperava violação do tipo %q, obtido %+v", tt.wantKind, rep.Violations)
			}
		})
	}
}

// TestManifest_RoundTrip verifies generate -> render -> load is stable.
func TestManifest_RoundTrip(t *testing.T) {
	root := t.TempDir()
	// Two columns across two collections, plus one whitelisted dir to ignore.
	writeFile(t, root, "agents/cosca-ai/INDEX.md", "# ai\n")
	writeFile(t, root, "engines/audit/INDEX.md", "# audit\n")
	writeFile(t, root, "engines/.cosca-scaffold/x.txt", "scaffold\n")

	manifest, err := BuildManifest(root)
	require.NoError(t, err)
	require.Len(t, manifest.Entries, 2)
	assert.Equal(t, ManifestVersion, manifest.Version)

	text := renderManifest(manifest)
	// Deterministic: rendering twice yields identical bytes.
	assert.Equal(t, text, renderManifest(manifest))

	// Round-trip through LoadManifest.
	manPath := filepath.Join(root, ManifestFileName)
	require.NoError(t, os.WriteFile(manPath, []byte(text), 0o644))

	loaded, err := LoadManifest(root)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Len(t, loaded.Entries, len(manifest.Entries))
	for i := range manifest.Entries {
		assert.Equal(t, manifest.Entries[i].Collection, loaded.Entries[i].Collection)
		assert.Equal(t, manifest.Entries[i].Name, loaded.Entries[i].Name)
		assert.Equal(t, manifest.Entries[i].IndexPath, loaded.Entries[i].IndexPath)
	}
}

// TestGenerate_Deterministic verifies Generate output is stable and sorted.
func TestGenerate_Deterministic(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "agents/cosca-b/INDEX.md", "# b\n")
	writeFile(t, root, "agents/cosca-a/INDEX.md", "# a\n")

	got1, err := Generate(root)
	require.NoError(t, err)
	got2, err := Generate(root)
	require.NoError(t, err)
	assert.Equal(t, got1, got2)

	// Sorted: cosca-a before cosca-b.
	assert.Less(t, stringsIndex(got1, "agents/cosca-a"), stringsIndex(got1, "agents/cosca-b"))
}

// TestCleanLinkDest exercises the link normalizer.
func TestCleanLinkDest(t *testing.T) {
	tests := []struct {
		raw    string
		want   string
		wantOK bool
	}{
		{"INDEX.md", "INDEX.md", true},
		{"docs/foo.md#anchor", "docs/foo.md", true},
		{`docs/foo.md "Title"`, "docs/foo.md", true},
		{"https://example.com", "", false},
		{"mailto:x@y.z", "", false},
		{"#section", "", false},
		{"", "", false},
		{"../../up.md", "../../up.md", true},
	}
	for _, tt := range tests {
		got, ok := cleanLinkDest(tt.raw)
		assert.Equal(t, tt.want, got, "dest mismatch for %q", tt.raw)
		assert.Equal(t, tt.wantOK, ok, "ok mismatch for %q", tt.raw)
	}
}

// TestReadFrontmatter verifies frontmatter extraction.
func TestReadFrontmatter(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "ok.md", "---\nname: foo-bar\ndescription: hi\nlevel: 2\n---\nbody\n")
	writeFile(t, root, "none.md", "no frontmatter here\n")
	writeFile(t, root, "agent.md", "---\nagent: foo\ndescription: d\n---\n")

	fm, ok := readFrontmatter(filepath.Join(root, "ok.md"))
	assert.True(t, ok)
	assert.Equal(t, "foo-bar", fm["name"])
	assert.Equal(t, "2", fm["level"])

	_, ok = readFrontmatter(filepath.Join(root, "none.md"))
	assert.False(t, ok)

	am, ok := readFrontmatter(filepath.Join(root, "agent.md"))
	assert.True(t, ok)
	assert.Equal(t, "foo", am["agent"])
}

func stringsIndex(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
