package dbhealth

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// createTestDB cria um banco SQLite real no caminho dado, com uma tabela e
// algumas linhas, para que page_count*page_size > 0 (tamanho on-disk real).
func createTestDB(t *testing.T, absPath string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	db, err := sql.Open("sqlite", absPath)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS t (id INTEGER PRIMARY KEY, payload TEXT NOT NULL) STRICT`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	for i := 0; i < 300; i++ {
		if _, err := db.Exec(`INSERT INTO t (payload) VALUES (?)`, strings.Repeat("x", 128)); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
}

// createEmptyCosca cria o diretório `.cosca` (e um subdir memory/) vazios.
func createEmptyCosca(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	coscaDir := filepath.Join(dir, ".cosca")
	if err := os.MkdirAll(filepath.Join(coscaDir, "memory"), 0o755); err != nil {
		t.Fatalf("mkdir cosca dir: %v", err)
	}
	return coscaDir
}

// byName localiza o relatório de um módulo pelo nome.
func byName(t *testing.T, r *Result, name string) *Report {
	t.Helper()
	for i := range r.Databases {
		if r.Databases[i].Name == name {
			return &r.Databases[i]
		}
	}
	t.Fatalf("relatório não encontrado para o módulo %q", name)
	return nil
}

// byRel localiza o relatório de um banco pelo caminho relativo.
func byRel(t *testing.T, r *Result, rel string) *Report {
	t.Helper()
	for i := range r.Databases {
		if r.Databases[i].RelPath == rel {
			return &r.Databases[i]
		}
	}
	t.Fatalf("relatório não encontrado para o caminho %q", rel)
	return nil
}

func TestMeasure_RealSQLite(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "knowledge.db")
	createTestDB(t, dbPath)

	rep := measure(dbPath, "knowledge", dir, DefaultLimits())

	if !rep.Found {
		t.Fatalf("esperava Found=true, got %v", rep.Found)
	}
	if rep.PageCount <= 0 {
		t.Errorf("esperava page_count > 0, got %d", rep.PageCount)
	}
	if rep.PageSize <= 0 {
		t.Errorf("esperava page_size > 0, got %d", rep.PageSize)
	}
	if rep.DBSizeBytes <= 0 {
		t.Errorf("esperava db_size_bytes > 0, got %d", rep.DBSizeBytes)
	}
	if rep.DBSizeBytes != rep.PageCount*rep.PageSize {
		t.Errorf("db_size_bytes = %d, esperava page_count*page_size = %d", rep.DBSizeBytes, rep.PageCount*rep.PageSize)
	}
	if rep.Status != StatusOK {
		t.Errorf("esperava status ok com limites default, got %q", rep.Status)
	}
	if rep.PercentOf100MB != float64(rep.DBSizeBytes)/float64(ReferenceCeiling) {
		t.Errorf("percent_of_100mb = %v, esperava %v", rep.PercentOf100MB, float64(rep.DBSizeBytes)/float64(ReferenceCeiling))
	}
	if rep.Path != dbPath {
		t.Errorf("path = %q, esperava %q", rep.Path, dbPath)
	}
}

func TestCheck_TargetsAreReportedEvenWhenMissing(t *testing.T) {
	coscaDir := createEmptyCosca(t)

	res, err := Check(Options{CoscaDir: coscaDir, IncludeAll: false})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if len(res.Databases) != len(DefaultTargets) {
		t.Fatalf("esperava %d alvos, got %d", len(DefaultTargets), len(res.Databases))
	}
	for _, d := range res.Databases {
		if d.Found {
			t.Errorf("módulo %q inesperadamente encontrado em cosca vazio", d.Name)
		}
		if d.Status != StatusNotFound {
			t.Errorf("módulo %q: esperava not_found, got %q", d.Name, d.Status)
		}
	}
	if res.Passed != true {
		t.Errorf("esperava Passed=true (nenhum banco), got %v", res.Passed)
	}
	if res.AnyFail || res.AnyWarn {
		t.Errorf("esperava AnyFail/AnyWarn false em cosca vazio, got fail=%v warn=%v", res.AnyFail, res.AnyWarn)
	}
}

func TestCheck_IncludeAll_FindsRuntimeDBs(t *testing.T) {
	coscaDir := createEmptyCosca(t)
	createTestDB(t, filepath.Join(coscaDir, "knowledge.db"))
	createTestDB(t, filepath.Join(coscaDir, "audit.db"))

	res, err := Check(Options{CoscaDir: coscaDir, IncludeAll: true})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	// O alvo knowledge + o runtime audit.db + mais os alvos ausentes.
	knowledge := byRel(t, res, "knowledge.db")
	if !knowledge.Found {
		t.Errorf("knowledge.db deveria estar encontrado")
	}
	runtime := byRel(t, res, "audit.db")
	if !runtime.Found {
		t.Errorf("audit.db (runtime) deveria estar encontrado pelo IncludeAll")
	}
	if runtime.Name != "audit" {
		t.Errorf("runtime db name = %q, esperava 'audit'", runtime.Name)
	}
	// Verifica que ao menos dois bancos existem no disco.
	found := 0
	for _, d := range res.Databases {
		if d.Found {
			found++
		}
	}
	if found < 2 {
		t.Errorf("esperava ao menos 2 bancos no disco, got %d", found)
	}
}

func TestCheck_Gate_Warn_WhenCrossesAlert(t *testing.T) {
	coscaDir := createEmptyCosca(t)
	createTestDB(t, filepath.Join(coscaDir, "knowledge.db"))

	l := Limits{LimitBytes: DefaultLimitBytes, WarnBytes: 1, FailBytes: DefaultLimitBytes}
	res, err := Check(Options{CoscaDir: coscaDir, IncludeAll: false, Limits: l})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	knowledge := byRel(t, res, "knowledge.db")
	if knowledge.Status != StatusWarn {
		t.Errorf("esperava status warn (db cruzou alerta de 1 byte), got %q", knowledge.Status)
	}
	if !res.AnyWarn {
		t.Errorf("esperava AnyWarn=true")
	}
	if res.AnyFail {
		t.Errorf("esperava AnyFail=false (db abaixo do teto)")
	}
	if !res.Passed {
		t.Errorf("esperava Passed=true (nenhum banco no teto)")
	}
}

func TestCheck_Gate_Fail_WhenExceedsCeiling(t *testing.T) {
	coscaDir := createEmptyCosca(t)
	createTestDB(t, filepath.Join(coscaDir, "knowledge.db"))

	// Teto de 1 byte: qualquer banco com conteúdo real excede → fail.
	l := Limits{LimitBytes: 1, WarnBytes: 1, FailBytes: 1}
	res, err := Check(Options{CoscaDir: coscaDir, IncludeAll: false, Limits: l})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	knowledge := byRel(t, res, "knowledge.db")
	if knowledge.Status != StatusFail {
		t.Errorf("esperava status fail (db excedeu teto de 1 byte), got %q (size=%d)", knowledge.Status, knowledge.DBSizeBytes)
	}
	if !res.AnyFail {
		t.Errorf("esperava AnyFail=true")
	}
	if res.Passed {
		t.Errorf("esperava Passed=false (gate bloqueando)")
	}
}

func TestClassify_Boundaries(t *testing.T) {
	const warn = int64(80 * mb)
	const fail = int64(100 * mb)

	cases := []struct {
		name string
		size int64
		want Status
	}{
		{"abaixo do alerta", 10 * mb, StatusOK},
		{"exatamente no alerta", warn, StatusWarn},
		{"acima do alerta, abaixo do teto", 90 * mb, StatusWarn},
		{"exatamente no teto", fail, StatusWarn}, // no teto ≈ alerta (não cruzou)
		{"acima do teto", fail + 1, StatusFail},
		{"bem acima do teto", 257 * mb, StatusFail},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Classify(tc.size, warn, fail); got != tc.want {
				t.Errorf("Classify(%d, %d, %d) = %q, esperava %q", tc.size, warn, fail, got, tc.want)
			}
		})
	}
}

func TestPercentOfLimit_UsesConfiguredLimit(t *testing.T) {
	coscaDir := createEmptyCosca(t)
	createTestDB(t, filepath.Join(coscaDir, "knowledge.db"))

	l := Limits{LimitBytes: 2 * DefaultLimitBytes, WarnBytes: 1, FailBytes: 2 * DefaultLimitBytes}
	res, err := Check(Options{CoscaDir: coscaDir, IncludeAll: false, Limits: l})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	knowledge := byRel(t, res, "knowledge.db")
	if knowledge.PercentOfLimit == 0 {
		t.Errorf("esperava PercentOfLimit > 0, got %v", knowledge.PercentOfLimit)
	}
	if knowledge.PercentOfLimit != float64(knowledge.DBSizeBytes)/float64(l.LimitBytes) {
		t.Errorf("PercentOfLimit = %v, esperava %v", knowledge.PercentOfLimit, float64(knowledge.DBSizeBytes)/float64(l.LimitBytes))
	}
}

func TestFromConfigMB(t *testing.T) {
	// Teto positivo: comportamento histórico preservado.
	if got := FromConfigMB(100, 0); got.LimitBytes != DefaultLimitBytes {
		t.Errorf("FromConfigMB(100,0).LimitBytes = %d, esperava %d", got.LimitBytes, DefaultLimitBytes)
	}
	if got := FromConfigMB(100, 0); got.WarnBytes != int64(float64(DefaultLimitBytes)*DefaultWarnRatio) {
		t.Errorf("FromConfigMB(100,0).WarnBytes = %d, esperava %d", got.WarnBytes, int64(float64(DefaultLimitBytes)*DefaultWarnRatio))
	}
	if got := FromConfigMB(100, 0); got.FailBytes != DefaultLimitBytes {
		t.Errorf("FromConfigMB(100,0).FailBytes = %d, esperava %d", got.FailBytes, DefaultLimitBytes)
	}
	// warn explícito prevalece sobre o automático.
	if got := FromConfigMB(100, 50); got.WarnBytes != 50*mb {
		t.Errorf("FromConfigMB(100,50).WarnBytes = %d, esperava %d", got.WarnBytes, 50*mb)
	}
	// teto 0 → sem teto (gate de tamanho desarmado): nenhum limite armado.
	got := FromConfigMB(0, 0)
	if got.LimitBytes != 0 || got.WarnBytes != 0 || got.FailBytes != 0 {
		t.Errorf("FromConfigMB(0,0) = %+v, esperava sem teto (0/0/0)", got)
	}
	// teto negativo → sem teto.
	if got := FromConfigMB(-10, 0); got.LimitBytes != 0 {
		t.Errorf("FromConfigMB(-10,0).LimitBytes = %d, esperava 0 (sem teto)", got.LimitBytes)
	}
	// sem limite armado, warn explícito é ignorado (sem warn por tamanho).
	if got := FromConfigMB(0, 50); got.WarnBytes != 0 {
		t.Errorf("FromConfigMB(0,50).WarnBytes = %d, esperava 0 (sem teto ignora warn)", got.WarnBytes)
	}
}

func TestCheck_NoLimit_NeverFailsBySize(t *testing.T) {
	coscaDir := createEmptyCosca(t)
	createTestDB(t, filepath.Join(coscaDir, "knowledge.db"))

	// Limits zero (sem teto) → banco encontrado é sempre ok; nunca warn/fail
	// por tamanho, mesmo com um banco real maior que o alerta padrão.
	res, err := Check(Options{CoscaDir: coscaDir, IncludeAll: false, Limits: Limits{}})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if res.LimitBytes != 0 || res.WarnBytes != 0 || res.FailBytes != 0 {
		t.Fatalf("sem teto deveria reportar limites 0/0/0, got %d/%d/%d", res.LimitBytes, res.WarnBytes, res.FailBytes)
	}
	knowledge := byRel(t, res, "knowledge.db")
	if knowledge.Status != StatusOK {
		t.Errorf("esperava status ok sem teto armado, got %q (size=%d)", knowledge.Status, knowledge.DBSizeBytes)
	}
	if res.AnyWarn || res.AnyFail {
		t.Errorf("esperava AnyWarn/AnyFail false sem teto, got warn=%v fail=%v", res.AnyWarn, res.AnyFail)
	}
	if !res.Passed {
		t.Errorf("esperava Passed=true sem teto")
	}
}

func TestMeasure_NoLimit_PercentUsesReferenceCeiling(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "knowledge.db")
	createTestDB(t, dbPath)

	rep := measure(dbPath, "knowledge", dir, Limits{})

	if !rep.Found {
		t.Fatalf("esperava Found=true, got %v", rep.Found)
	}
	if rep.Status != StatusOK {
		t.Errorf("esperava status ok sem teto armado, got %q", rep.Status)
	}
	want := float64(rep.DBSizeBytes) / float64(ReferenceCeiling)
	if rep.PercentOfLimit != want {
		t.Errorf("PercentOfLimit = %v, esperava referência de 100 MB (%v)", rep.PercentOfLimit, want)
	}
	if rep.PercentOfLimit != rep.PercentOf100MB {
		t.Errorf("sem teto, PercentOfLimit deveria ser idêntico a PercentOf100MB, got %v vs %v", rep.PercentOfLimit, rep.PercentOf100MB)
	}
}

func TestMeasure_NotFound_DoesNotBreakGate(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, ".cosca", "vector.db")

	rep := measure(missing, "vector", filepath.Join(dir, ".cosca"), DefaultLimits())
	if rep.Found {
		t.Errorf("esperava Found=false para arquivo ausente")
	}
	if rep.Status != StatusNotFound {
		t.Errorf("esperava status not_found, got %q", rep.Status)
	}
	if rep.Error != "" {
		t.Errorf("esperava sem erro, got %q", rep.Error)
	}
}
