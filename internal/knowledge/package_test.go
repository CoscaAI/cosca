//
// Tests for internal/knowledge/package.go — Knowledge Package manifesto.
//
// Cobre:
//   - DetectPackageInfo (heurística LOCAL, sem rede): "prisma" →
//     typescript; "github:prisma/prisma" → id + repository; "pgx" → go;
//     "tokio" → rust; desconhecido → best-effort default ("unknown")
//   - Validate: id/kind/ecosystem obrigatórios com erro pt-BR
//   - PackageStore: Add/Get/List/Exists round-trip, sem rede
//
// Não toca em items/laws/evidence existentes — no-regression.
//

package knowledge

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// DetectPackageInfo — descoberta heurística
// =============================================================================

func TestDetectPackageInfo_Known(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		wantID     string
		wantEco    string
		wantRepo   string
		wantStatus string
	}{
		{name: "prisma plain", input: "prisma", wantID: "prisma", wantEco: "typescript", wantStatus: PackageStatusManifest},
		{name: "prisma via repo", input: "github:prisma/prisma", wantID: "prisma", wantEco: "typescript", wantRepo: "prisma/prisma", wantStatus: PackageStatusManifest},
		{name: "pgx go", input: "pgx", wantID: "pgx", wantEco: "go", wantStatus: PackageStatusManifest},
		{name: "tokio rust", input: "tokio", wantID: "tokio", wantEco: "rust", wantStatus: PackageStatusManifest},
		{name: "zod typescript", input: "zod", wantID: "zod", wantEco: "typescript", wantStatus: PackageStatusManifest},
		{name: "github.com form", input: "github.com/org/pgx", wantID: "pgx", wantEco: "go", wantRepo: "org/pgx", wantStatus: PackageStatusManifest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg, err := DetectPackageInfo(tt.input)
			require.NoError(t, err, "DetectPackageInfo(%q)", tt.input)
			assert.Equal(t, tt.wantID, pkg.ID, "id")
			assert.Equal(t, PackageKindLibrary, pkg.Kind, "kind default library")
			assert.Equal(t, tt.wantEco, pkg.Ecosystem, "ecosystem")
			assert.Equal(t, tt.wantRepo, pkg.Repository, "repository")
			assert.Equal(t, PackageKnowledgeNone, pkg.KnowledgeLevel, "knowledge_level default")
			assert.Equal(t, tt.wantStatus, pkg.Status, "status")
		})
	}
}

func TestDetectPackageInfo_JavaEcosystem(t *testing.T) {
	t.Parallel()

	libs := []string{
		"spring", "spring-boot", "spring-mvc", "spring-security", "spring-data", "spring-cloud",
		"springboot", "quarkus", "micronaut", "vertx", "vaadin", "grails",
		"hibernate", "jpa", "mybatis", "flyway", "liquibase", "jooq", "jdbc",
		"junit", "junit5", "testng", "mockito", "assertj", "cucumber",
		"maven", "gradle", "lombok", "mapstruct", "jackson", "gson",
		"slf4j", "log4j", "logback",
		"kafka-clients", "apache-kafka", "redis-client", "jedis", "lettuce",
		"guava", "apache-commons", "rxjava", "reactor", "netty", "opencsv",
	}
	for _, lib := range libs {
		pkg, err := DetectPackageInfo(lib)
		require.NoError(t, err, "DetectPackageInfo(%q)", lib)
		assert.Equal(t, lib, pkg.ID, "id de %q", lib)
		assert.Equal(t, "java", pkg.Ecosystem, "ecosystem de %q", lib)
	}
}

func TestDetectPackageInfo_RustEcosystem(t *testing.T) {
	t.Parallel()

	libs := []string{
		"tokio", "async-std", "axum", "actix-web", "hyper",
		"serde", "clap", "structopt", "sqlx", "diesel", "sea-orm",
		"tracing", "anyhow", "thiserror", "uuid",
	}
	for _, lib := range libs {
		pkg, err := DetectPackageInfo(lib)
		require.NoError(t, err, "DetectPackageInfo(%q)", lib)
		assert.Equal(t, lib, pkg.ID, "id de %q", lib)
		assert.Equal(t, "rust", pkg.Ecosystem, "ecosystem de %q", lib)
	}
}

func TestDetectPackageInfo_PythonEcosystem(t *testing.T) {
	t.Parallel()

	libs := []string{
		"fastapi", "django", "flask", "starlette", "aiohttp",
		"sqlalchemy", "pydantic", "sqlmodel", "pytest", "httpx",
		"boto3", "numpy", "pandas", "celery", "redis-py",
	}
	for _, lib := range libs {
		pkg, err := DetectPackageInfo(lib)
		require.NoError(t, err, "DetectPackageInfo(%q)", lib)
		assert.Equal(t, lib, pkg.ID, "id de %q", lib)
		assert.Equal(t, "python", pkg.Ecosystem, "ecosystem de %q", lib)
	}
}

func TestDetectPackageInfo_AIML(t *testing.T) {
	t.Parallel()

	libs := []string{
		"comfyui", "comfy-ui", "comfy", "stable-diffusion-webui",
		"diffusers", "transformers", "langchain", "ollama",
		"whisper", "faster-whisper", "kokoro", "opencv-python", "torch",
		"openai", "anthropic", "langgraph", "llama-index", "pytorch",
		"tensorflow", "jax", "onnxruntime", "sentence-transformers", "vllm",
		"huggingface-hub", "torchvision", "torchaudio", "accelerate", "peft",
		"datasets", "tokenizers", "xgboost", "lightgbm", "statsmodels",
		"seaborn", "plotly", "tts", "stable-diffusion", "controlnet",
		"pillow", "soundfile", "librosa", "smolagents", "crewai",
		"autogen", "pydantic-ai", "instructor",
	}
	for _, lib := range libs {
		pkg, err := DetectPackageInfo(lib)
		require.NoError(t, err, "DetectPackageInfo(%q)", lib)
		assert.Equal(t, lib, pkg.ID, "id de %q", lib)
		assert.Equal(t, "python", pkg.Ecosystem, "ecosystem de %q", lib)
	}
}

func TestDetectPackageInfo_DotNetEcosystem(t *testing.T) {
	t.Parallel()

	libs := []string{
		"aspnetcore", "mvc", "razor", "blazor", "minimal-api",
		"efcore", "dapper", "npgsql", "serilog", "xunit",
		"polly", "newtonsoft-json",
	}
	for _, lib := range libs {
		pkg, err := DetectPackageInfo(lib)
		require.NoError(t, err, "DetectPackageInfo(%q)", lib)
		assert.Equal(t, lib, pkg.ID, "id de %q", lib)
		assert.Equal(t, "dotnet", pkg.Ecosystem, "ecosystem de %q", lib)
	}
}

func TestDetectPackageInfo_JavaRegression_OtherEcosystems(t *testing.T) {
	t.Parallel()

	tests := []struct{ in, want string }{
		{in: "prisma", want: "typescript"},
		{in: "pgx", want: "go"},
		{in: "tokio", want: "rust"},
		{in: "fastapi", want: "python"},
	}
	for _, tt := range tests {
		pkg, err := DetectPackageInfo(tt.in)
		require.NoError(t, err, "DetectPackageInfo(%q)", tt.in)
		assert.Equal(t, tt.want, pkg.Ecosystem, "ecosystem de %q", tt.in)
	}
}

func TestDetectPackageInfo_UnknownBestEffort(t *testing.T) {
	t.Parallel()

	pkg, err := DetectPackageInfo("some-unknown-lib-xyz")
	require.NoError(t, err)
	assert.Equal(t, "some-unknown-lib-xyz", pkg.ID)
	assert.Equal(t, PackageKindLibrary, pkg.Kind)
	assert.Equal(t, EcosystemUnknown, pkg.Ecosystem, "best-effort default honesto")
	assert.Empty(t, pkg.Repository)
}

func TestDetectPackageInfo_Empty(t *testing.T) {
	t.Parallel()

	if _, err := DetectPackageInfo(""); err == nil {
		t.Fatal("entrada vazia deveria falhar")
	}
	if _, err := DetectPackageInfo("   "); err == nil {
		t.Fatal("entrada em branco deveria falhar")
	}
}

func TestNormalizePackageID(t *testing.T) {
	t.Parallel()

	tests := []struct{ in, want string }{
		{"prisma", "prisma"},
		{"github:prisma/prisma", "prisma"},
		{"github.com/org/prisma", "prisma"},
		{"@scope/lib", "lib"},
		{"PRISMA", "prisma"},
		{"prisma/", "prisma"},
	}
	for _, tt := range tests {
		if got := NormalizePackageID(tt.in); got != tt.want {
			t.Errorf("NormalizePackageID(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// =============================================================================
// Validate
// =============================================================================

func TestKnowledgePackage_Validate(t *testing.T) {
	t.Parallel()

	valid := KnowledgePackage{
		ID: "prisma", Kind: PackageKindLibrary, Ecosystem: "typescript",
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("manifesto válido não deveria falhar: %v", err)
	}

	// ID obrigatório.
	noID := KnowledgePackage{Kind: PackageKindLibrary, Ecosystem: "typescript"}
	err := noID.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "id é obrigatório")

	// Kind no conjunto conhecido.
	badKind := KnowledgePackage{ID: "prisma", Kind: "frameworkx", Ecosystem: "typescript"}
	err = badKind.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kind")

	// Ecosystem obrigatório.
	noEco := KnowledgePackage{ID: "prisma", Kind: PackageKindLibrary}
	err = noEco.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ecosystem é obrigatório")
}

// =============================================================================
// PackageStore round-trip
// =============================================================================

func TestPackageStore_RoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := NewPackageStore(dir)

	// Vazio no início.
	pkgs, err := store.List()
	require.NoError(t, err)
	assert.Empty(t, pkgs)
	assert.False(t, store.Exists("prisma"))

	prisma := KnowledgePackage{
		ID: "prisma", Kind: PackageKindLibrary, Ecosystem: "typescript",
		Versions: []string{"6.x"}, Sources: []string{"official-docs", "official-repository"},
		Repository: "prisma/prisma", KnowledgeLevel: PackageKnowledgeNone,
		Status: PackageStatusManifest,
	}
	require.NoError(t, store.Add(prisma))

	pgx := KnowledgePackage{
		ID: "pgx", Kind: PackageKindLibrary, Ecosystem: "go",
		Versions: []string{"*"}, KnowledgeLevel: PackageKnowledgeNone,
		Status: PackageStatusManifest,
	}
	require.NoError(t, store.Add(pgx))

	// Add valida: manifesto inválido é recusado.
	if err := store.Add(KnowledgePackage{Kind: PackageKindLibrary, Ecosystem: "go"}); err == nil {
		t.Fatal("Add de manifesto sem id deveria falhar")
	}

	assert.True(t, store.Exists("prisma"))
	assert.True(t, store.Exists("pgx"))
	assert.False(t, store.Exists("next"))

	got, err := store.Get("prisma")
	require.NoError(t, err)
	assert.Equal(t, "prisma", got.ID)
	assert.Equal(t, "typescript", got.Ecosystem)
	assert.Equal(t, "prisma/prisma", got.Repository)
	assert.Equal(t, []string{"6.x"}, got.Versions)
	assert.Equal(t, PackageStatusManifest, got.Status)

	// Get de manifesto inexistente → erro.
	if _, err := store.Get("next"); err == nil {
		t.Fatal("Get de pacote inexistente deveria falhar")
	}
	if _, err := store.Get(""); err == nil {
		t.Fatal("Get com id vazio deveria falhar")
	}

	// List ordenada por ID.
	all, err := store.List()
	require.NoError(t, err)
	require.Len(t, all, 2)
	assert.Equal(t, "pgx", all[0].ID)
	assert.Equal(t, "prisma", all[1].ID)

	// Arquivo em .cosca/knowledge/packages/<id>.json com permissões restritas.
	manifestPath := filepath.Join(dir, ".cosca", filepath.FromSlash(PackagesDir), "prisma.json")
	info, err := filepath.Glob(manifestPath)
	require.NoError(t, err)
	assert.Len(t, info, 1, "manifesto deveria existir no caminho padrão")
}

func TestPackageStore_ListOnMissingDir(t *testing.T) {
	t.Parallel()

	store := NewPackageStore(t.TempDir())
	pkgs, err := store.List()
	require.NoError(t, err)
	assert.Empty(t, pkgs, "diretório ausente ⇒ lista vazia, não erro")
}
