package estimator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ────────────────────────────────────────────────────────────────────────────
// TestEstimate_Basic — escopo com 2 arquivos → plano gerado com todos os campos
// ────────────────────────────────────────────────────────────────────────────

func TestEstimate_Basic(t *testing.T) {
	t.Setenv(TrustRegistryEnvVar, "") // isola: sem registry → heurístico

	dir := t.TempDir()
	scope := ExecutionScope{
		ProjectDir:  dir,
		TargetFiles: []string{"internal/estimator/estimator.go", "internal/estimator/metrics.go"},
		ChangeType:  "feature",
		Agent:       "cosca-backend",
	}

	plan, err := Estimate(scope)
	if err != nil {
		t.Fatalf("Estimate: %v", err)
	}
	if plan.FilesAffected != 2 {
		t.Errorf("FilesAffected = %d, want 2", plan.FilesAffected)
	}
	if len(plan.Files) != 2 {
		t.Errorf("len(Files) = %d, want 2", len(plan.Files))
	}
	if plan.TestsExpected < 0 {
		t.Errorf("TestsExpected = %d, want >= 0", plan.TestsExpected)
	}
	if len(plan.TestPackages) != 1 {
		t.Errorf("TestPackages = %v, want 1 package", plan.TestPackages)
	}
	if len(plan.Migrations) != 0 {
		t.Errorf("Migrations = %v, want empty", plan.Migrations)
	}
	if plan.RollbackAvailable {
		t.Errorf("RollbackAvailable = true, want false (temp dir não é git)")
	}
	if !strings.Contains(plan.RollbackDetail, "não disponível") {
		t.Errorf("RollbackDetail = %q, want 'não disponível'", plan.RollbackDetail)
	}
	if plan.EstimatedMinutes < 1 {
		t.Errorf("EstimatedMinutes = %d, want >= 1", plan.EstimatedMinutes)
	}
	switch plan.RiskLevel {
	case "baixo", "médio", "alto":
	default:
		t.Errorf("RiskLevel = %q, want baixo/médio/alto", plan.RiskLevel)
	}
	if plan.ConfidencePercent < 0 || plan.ConfidencePercent > 100 {
		t.Errorf("ConfidencePercent = %d, want 0-100", plan.ConfidencePercent)
	}
	if plan.GeneratedAt.IsZero() {
		t.Error("GeneratedAt não deve ser zero")
	}
	if plan.Method != "heurístico" {
		t.Errorf("Method = %q, want 'heurístico' (sem Trust Registry)", plan.Method)
	}
}

// TestEstimate_Historico — com Trust Registry presente, a confiança vem do
// histórico real e o Method vira "histórico".
func TestEstimate_Historico(t *testing.T) {
	dir := t.TempDir()
	reg := filepath.Join(dir, "TRUST_REGISTRY.md")
	content := "# TRUST REGISTRY\n\nreliability_score:\n  cosca-backend:\n    score: 0.76\n"
	if err := os.WriteFile(reg, []byte(content), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}
	t.Setenv(TrustRegistryEnvVar, reg)

	scope := ExecutionScope{
		ProjectDir:  dir,
		TargetFiles: []string{"internal/estimator/estimator.go"},
		ChangeType:  "fix",
		Agent:       "cosca-backend",
	}
	plan, err := Estimate(scope)
	if err != nil {
		t.Fatalf("Estimate: %v", err)
	}
	if plan.Method != "histórico" {
		t.Errorf("Method = %q, want 'histórico'", plan.Method)
	}
	// risco baixo + 0.76 → 90 + min(8, 7.6) = 97
	if plan.ConfidencePercent != 97 {
		t.Errorf("ConfidencePercent = %d, want 97", plan.ConfidencePercent)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// TestExpandScope — glob resolve arquivos reais
// ────────────────────────────────────────────────────────────────────────────

func TestExpandScope(t *testing.T) {
	dir := t.TempDir()
	k1 := writeTempFile(t, dir, "internal/kernel/kernel.go")
	k2 := writeTempFile(t, dir, "internal/kernel/emergency.go")
	s1 := writeTempFile(t, dir, "api/rest/server.go")
	writeTempFile(t, dir, "docs/guide.md")

	got, err := ExpandScope(ExecutionScope{
		ProjectDir:  dir,
		TargetFiles: []string{"internal/kernel/*.go", "api/rest/*.go"},
	})
	if err != nil {
		t.Fatalf("ExpandScope: %v", err)
	}
	// ExpandScope devolve caminhos absolutos ordenados lexicalmente.
	want := []string{s1, k2, k1}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("ExpandScope = %v, want %v", got, want)
	}
}

// TestExpandScope_Literal — caminhos literais (arquivos que serão criados) são
// incluídos mesmo sem existir em disco.
func TestExpandScope_Literal(t *testing.T) {
	dir := t.TempDir()
	got, err := ExpandScope(ExecutionScope{
		ProjectDir:  dir,
		TargetFiles: []string{"internal/novo/arquivo.go"},
	})
	if err != nil {
		t.Fatalf("ExpandScope: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ExpandScope = %v, want 1 literal file", got)
	}
	// FromSlash evita hardcodar o separador: no Windows o caminho é
	// "...\internal\novo\arquivo.go", não ".../internal/novo/arquivo.go".
	if !strings.HasSuffix(got[0], filepath.FromSlash("internal/novo/arquivo.go")) {
		t.Errorf("ExpandScope = %q, want literal path preserved", got[0])
	}
}

// TestExpandScope_DoubleStar — globs recursivos com "**".
func TestExpandScope_DoubleStar(t *testing.T) {
	dir := t.TempDir()
	a := writeTempFile(t, dir, "api/rest/server.go")
	b := writeTempFile(t, dir, "api/rest/handler_test.go")
	c := writeTempFile(t, dir, "api/stream/sse.go")
	writeTempFile(t, dir, "internal/chat/hub.go")

	got, err := ExpandScope(ExecutionScope{
		ProjectDir:  dir,
		TargetFiles: []string{"api/**"},
	})
	if err != nil {
		t.Fatalf("ExpandScope(api/**): %v", err)
	}
	want := []string{b, a, c} // ordenado lexicalmente: rest/handler_test < rest/server < stream/sse
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("ExpandScope(api/**) = %v, want %v", got, want)
	}

	got, err = ExpandScope(ExecutionScope{
		ProjectDir:  dir,
		TargetFiles: []string{"**/*_test.go"},
	})
	if err != nil {
		t.Fatalf("ExpandScope(**/*_test.go): %v", err)
	}
	if len(got) != 1 || got[0] != b {
		t.Errorf("ExpandScope(**/*_test.go) = %v, want [%s]", got, b)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// TestCountTests — conta funções de teste em diretório temp com _test.go fake
// ────────────────────────────────────────────────────────────────────────────

func TestCountTests(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "mypkg")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := `package mypkg

func TestAdd(t *testing.T) {}
func TestSub(t *testing.T) {}
func helperAdd() int { return 0 }
// func TestCommented(t *testing.T) {}
/*
func TestBlockCommented(t *testing.T) {}
*/
func TestMain(m *testing.M) {}
`
	if err := os.WriteFile(filepath.Join(pkg, "mypkg_test.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	got, err := CountTests([]string{pkg})
	if err != nil {
		t.Fatalf("CountTests: %v", err)
	}
	if got != 3 { // TestAdd, TestSub, TestMain — comentadas ignoradas
		t.Errorf("CountTests = %d, want 3", got)
	}

	// pacote inexistente contribui 0
	got, err = CountTests([]string{pkg, filepath.Join(dir, "missing")})
	if err != nil {
		t.Fatalf("CountTests(missing): %v", err)
	}
	if got != 3 {
		t.Errorf("CountTests com pacote inexistente = %d, want 3", got)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// TestEstimateTime — valores esperados para cada changeType
// ────────────────────────────────────────────────────────────────────────────

func TestEstimateTime(t *testing.T) {
	cases := []struct {
		name       string
		files      int
		tests      int
		changeType string
		want       int
	}{
		{"feature", 2, 10, "feature", 9},     // 8 + 0.6 + 0.2 = 8.8 → 9
		{"fix", 5, 20, "fix", 7},             // 5 + 1.5 + 0.4 = 6.9 → 7
		{"security", 1, 0, "security", 11},   // 10 + 0.3 = 10.3 → 11
		{"refactor", 10, 50, "refactor", 10}, // 6 + 3 + 1 = 10
		{"docs", 3, 0, "docs", 3},            // 2 + 0.9 = 2.9 → 3
		{"test", 2, 5, "test", 5},            // 4 + 0.6 + 0.1 = 4.7 → 5
		{"docs vazio", 0, 0, "docs", 2},
		{"feature vazio", 0, 0, "feature", 8},
		{"desconhecido", 4, 10, "unknown", 7}, // base neutra 5 → 6.4 → 7
		{"mínimo", 0, 0, "unknown", 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EstimateTime(tc.files, tc.tests, tc.changeType)
			if got != tc.want {
				t.Errorf("EstimateTime(%d, %d, %q) = %d, want %d",
					tc.files, tc.tests, tc.changeType, got, tc.want)
			}
			if got < 1 {
				t.Errorf("EstimateTime(%d, %d, %q) = %d, want >= 1",
					tc.files, tc.tests, tc.changeType, got)
			}
		})
	}
}

// ────────────────────────────────────────────────────────────────────────────
// TestAssessRisk — security+auth → alto; docs → baixo (e fronteiras)
// ────────────────────────────────────────────────────────────────────────────

func TestAssessRisk(t *testing.T) {
	manyFiles := make([]string, 26)
	for i := range manyFiles {
		manyFiles[i] = "internal/foo/file.go"
	}
	elevenFiles := make([]string, 11)
	for i := range elevenFiles {
		elevenFiles[i] = "internal/foo/file.go"
	}

	cases := []struct {
		name       string
		files      []string
		tests      int
		migrations int
		changeType string
		want       string
	}{
		{"auth → alto", []string{"internal/auth/login.go"}, 0, 0, "security", "alto"},
		{"kernel → alto", []string{"internal/kernel/emergency.go"}, 0, 0, "fix", "alto"},
		{"jail → alto", []string{"pkg/cosca/jail.go"}, 0, 0, "refactor", "alto"},
		{"security pkg → alto", []string{"internal/security/policy.go"}, 0, 0, "feature", "alto"},
		{"emergency → alto", []string{"api/rest/handler/emergency.go"}, 0, 0, "feature", "alto"},
		{"serve → alto", []string{"cmd/serve/main.go"}, 0, 0, "feature", "alto"},
		{"migração → alto", []string{"internal/foo/bar.go"}, 0, 1, "feature", "alto"},
		{"files>25 → alto", manyFiles, 0, 0, "docs", "alto"},
		{"docs → baixo", []string{"docs/guide.md"}, 0, 0, "docs", "baixo"},
		{"interno comum → baixo", []string{"internal/estimator/estimator.go"}, 5, 0, "feature", "baixo"},
		{"files>10 → médio", elevenFiles, 0, 0, "docs", "médio"},
		{"tests>200 → médio", []string{"internal/foo/bar.go"}, 201, 0, "fix", "médio"},
		{"api/ → médio", []string{"api/rest/server.go"}, 0, 0, "feature", "médio"},
		{"internal/chat/ → médio", []string{"internal/chat/hub.go"}, 0, 0, "feature", "médio"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := AssessRisk(tc.files, tc.tests, tc.migrations, tc.changeType)
			if got != tc.want {
				t.Errorf("AssessRisk = %q, want %q", got, tc.want)
			}
		})
	}
}

// ────────────────────────────────────────────────────────────────────────────
// TestComputeConfidence — baixo+0.95 → 90-98; alto+0.5 → 65-80
// ────────────────────────────────────────────────────────────────────────────

func TestComputeConfidence(t *testing.T) {
	cases := []struct {
		name   string
		risk   string
		agent  float64
		wantLo int
		wantHi int
	}{
		{"baixo 0.95", "baixo", 0.95, 90, 98},
		{"baixo 0.0", "baixo", 0.0, 90, 98},
		{"baixo 1.0", "baixo", 1.0, 90, 98},
		{"médio 0.5", "médio", 0.5, 80, 90},
		{"médio 1.0", "médio", 1.0, 80, 90},
		{"alto 0.5", "alto", 0.5, 65, 80},
		{"alto 0.0", "alto", 0.0, 65, 80},
		{"alto 1.0", "alto", 1.0, 65, 80},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputeConfidence(tc.risk, tc.agent)
			if got < tc.wantLo || got > tc.wantHi {
				t.Errorf("ComputeConfidence(%q, %.2f) = %d, want [%d-%d]",
					tc.risk, tc.agent, got, tc.wantLo, tc.wantHi)
			}
		})
	}

	// valores pontuais exatos
	exact := []struct {
		risk  string
		agent float64
		want  int
	}{
		{"baixo", 0.95, 98}, // 90 + min(8, 9.5) = 98
		{"médio", 0.5, 85},  // 80 + min(10, 5) = 85
		{"alto", 0.5, 72},   // 65 + min(15, 7.5) = 72
		{"alto", 1.0, 80},
		{"baixo", 0.0, 90},
	}
	for _, tc := range exact {
		if got := ComputeConfidence(tc.risk, tc.agent); got != tc.want {
			t.Errorf("ComputeConfidence(%q, %.2f) = %d, want %d", tc.risk, tc.agent, got, tc.want)
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// TestCheckRollback — repo temp (git init + commit) → true; dir sem git → false
// ────────────────────────────────────────────────────────────────────────────

func TestCheckRollback(t *testing.T) {
	// repo git com 1 commit
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGit(t, repo, "init")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "-c", "user.name=Test", "-c", "user.email=test@cosca", "commit", "-m", "init")

	ok, detail := CheckRollback(repo)
	if !ok {
		t.Errorf("CheckRollback(repo) = false, want true")
	}
	if !strings.HasPrefix(detail, "disponível (git revert ") {
		t.Errorf("CheckRollback detail = %q, want prefix 'disponível (git revert '", detail)
	}

	// repo git vazio (sem commits) → não há o que reverter
	empty := t.TempDir()
	runGit(t, empty, "init")
	if ok, _ := CheckRollback(empty); ok {
		t.Errorf("CheckRollback(empty repo) = true, want false (sem commits)")
	}

	// diretório sem git
	plain := t.TempDir()
	ok, detail = CheckRollback(plain)
	if ok {
		t.Errorf("CheckRollback(plain) = true, want false")
	}
	if detail != "não disponível" {
		t.Errorf("CheckRollback detail = %q, want 'não disponível'", detail)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// TestAgentConfidence — extrai reliability_score do Trust Registry
// ────────────────────────────────────────────────────────────────────────────

func TestAgentConfidence(t *testing.T) {
	dir := t.TempDir()
	reg := filepath.Join(dir, "TRUST_REGISTRY.md")
	content := `# TRUST REGISTRY
reliability_score:
  cosca-backend:
    score: 0.76
  cosca-testing:
    score: 0.84
`
	if err := os.WriteFile(reg, []byte(content), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}

	cases := []struct {
		name  string
		agent string
		path  string
		want  float64
	}{
		{"encontrado", "cosca-backend", reg, 0.76},
		{"encontrado 2", "cosca-testing", reg, 0.84},
		{"não encontrado", "cosca-unknown", reg, 0.85},
		{"arquivo ausente", "cosca-backend", filepath.Join(dir, "missing.md"), 0.85},
		{"caminho vazio", "cosca-backend", "", 0.85},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := AgentConfidence(tc.agent, tc.path)
			if got != tc.want {
				t.Errorf("AgentConfidence(%q) = %.2f, want %.2f", tc.agent, got, tc.want)
			}
		})
	}
}

// TestAgentConfidence_RealRegistry valida o parsing contra o TRUST_REGISTRY.md
// real do projeto (apenas leitura — o arquivo NÃO é alterado).
func TestAgentConfidence_RealRegistry(t *testing.T) {
	reg := filepath.Join("..", "..", ".cosca", "fallback", "memory", "trust", "TRUST_REGISTRY.md")
	if _, err := os.Stat(reg); err != nil {
		t.Skipf("Trust Registry real não encontrado (%v)", err)
	}
	if got := AgentConfidence("cosca-backend", reg); got != 0.76 {
		t.Errorf("AgentConfidence(cosca-backend, real registry) = %.2f, want 0.76", got)
	}
	if got := AgentConfidence("cosca-kernel", reg); got != 0.94 {
		t.Errorf("AgentConfidence(cosca-kernel, real registry) = %.2f, want 0.94", got)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// TestFormat — output exato no formato do Don
// ────────────────────────────────────────────────────────────────────────────

func TestFormat(t *testing.T) {
	plan := &ExecutionPlan{
		FilesAffected:     18,
		Files:             []string{"a.go", "b.go"},
		TestsExpected:     143,
		TestPackages:      []string{"internal/kernel"},
		Migrations:        nil,
		RollbackAvailable: true,
		RollbackDetail:    "disponível (git revert bdaef4c)",
		EstimatedMinutes:  6,
		RiskLevel:         "baixo",
		ConfidencePercent: 96,
	}
	want := "Plano de Execução\n" +
		"─────────────────────────────\n" +
		"Arquivos afetados: 18\n" +
		"Testes previstos: 143\n" +
		"Migrações: nenhuma\n" +
		"Rollback: disponível\n" +
		"Tempo estimado: 6 min\n" +
		"Risco da alteração: baixo\n" +
		"Confiança: 96%"
	if got := plan.String(); got != want {
		t.Errorf("String():\n got:\n%s\nwant:\n%s", got, want)
	}
}

// TestFormat_ComMigracoesESemRollback cobre os outros ramos de formatação.
func TestFormat_ComMigracoesESemRollback(t *testing.T) {
	plan := &ExecutionPlan{
		FilesAffected:     3,
		TestsExpected:     0,
		Migrations:        []string{"20260731_init.sql", "20260731_add_idx.sql"},
		RollbackAvailable: false,
		RollbackDetail:    "não disponível",
		EstimatedMinutes:  4,
		RiskLevel:         "médio",
		ConfidencePercent: 85,
	}
	want := "Plano de Execução\n" +
		"─────────────────────────────\n" +
		"Arquivos afetados: 3\n" +
		"Testes previstos: 0\n" +
		"Migrações: 20260731_init.sql, 20260731_add_idx.sql\n" +
		"Rollback: não disponível\n" +
		"Tempo estimado: 4 min\n" +
		"Risco da alteração: médio\n" +
		"Confiança: 85%"
	if got := plan.String(); got != want {
		t.Errorf("String():\n got:\n%s\nwant:\n%s", got, want)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// helpers
// ────────────────────────────────────────────────────────────────────────────

// writeTempFile cria um arquivo sob dir com caminho relativo e retorna o
// caminho absoluto.
func writeTempFile(t *testing.T, dir, rel string) string {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte("package x\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
	return full
}
