// repair_test.go — fluxo completo de Repair (critérios de aceite 2, 4, 5 do
// ADR-043 §8) com recipe runner injetável (nunca executa o binário real em
// teste unitário).
package staterepair

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// repairFixture monta um projeto temporário com um `.cosca` e um session.db
// doente (sqlite válido corrompido no meio). Devolve (coscaDir, sessionDB).
func repairFixture(t *testing.T, corrupt bool) (string, string) {
	t.Helper()
	proj := t.TempDir()
	root := filepath.Join(proj, ".cosca")
	require.NoError(t, os.MkdirAll(root, 0o755))
	db := filepath.Join(root, "session.db")
	seedSQLite(t, db)
	if corrupt {
		corruptSQLiteMidFile(t, db)
	}
	return root, db
}

// fakeRecipeRecreate devolve um recipe runner que "regenera" o session.db com
// um SQLite válido novo (simula `cosca session index`).
func fakeRecipeRecreate(t *testing.T) recipeRunner {
	return func(_ context.Context, projectRoot string, _ Recipe) (string, error) {
		seedSQLite(t, filepath.Join(projectRoot, ".cosca", "session.db"))
		return "fake session index ok", nil
	}
}

// TestRepair_DryRunWritesNothing prova o critério de aceite 5: dry-run reporta
// o plano e NÃO escreve NENHUM arquivo (nem backup, nem quarentena, nem
// ledger, nem sidecar).
func TestRepair_DryRunWritesNothing(t *testing.T) {
	root, db := repairFixture(t, true)

	before := treeSnapshot(t, root)
	fpBefore, err := Fingerprint(db)
	require.NoError(t, err)

	mgr := NewManager(root)
	reports, repErr := mgr.Repair(KindSession, Options{DryRun: true})
	require.NoError(t, repErr)
	require.Len(t, reports, 1)

	r := reports[0]
	require.Equal(t, StatusDryRun, r.Status)
	require.Equal(t, fpBefore, r.Fingerprint, "dry-run lê o fingerprint sem alterar o arquivo")
	require.NotEmpty(t, r.Recipe, "dry-run reporta o plano (recipe)")

	after := treeSnapshot(t, root)
	require.Equal(t, before, after, "dry-run NÃO pode escrever nenhum arquivo")
	assertFileMissing(t, filepath.Join(root, "backups"))
	assertFileMissing(t, filepath.Join(root, "quarantine"))
	assertFileMissing(t, LedgerPath(db))
}

// TestRepair_CheckReportsSickSemEscrever prova que o modo check (sem apply e
// sem dry-run) relata o banco doente sem tocar em nada.
func TestRepair_CheckReportsSickSemEscrever(t *testing.T) {
	root, _ := repairFixture(t, true)

	before := treeSnapshot(t, root)
	mgr := NewManager(root)
	reports, repErr := mgr.Repair(KindSession, Options{})
	require.NoError(t, repErr)
	require.Len(t, reports, 1)
	require.Equal(t, StatusSick, reports[0].Status)

	after := treeSnapshot(t, root)
	require.Equal(t, before, after, "modo check não pode escrever")
}

// TestRepair_ApplySuccessRegenerates prova o critério de aceite 4: banco
// derivado corrompido → recipe regenera → Health pós limpo → RecordSuccess.
func TestRepair_ApplySuccessRegenerates(t *testing.T) {
	root, db := repairFixture(t, true)

	mgr := NewManager(root)
	mgr.runRecipe = fakeRecipeRecreate(t)

	reports, repErr := mgr.Repair(KindSession, Options{Apply: true})
	require.NoError(t, repErr)
	require.Len(t, reports, 1)

	r := reports[0]
	require.Equal(t, StatusRepaired, r.Status, "recipe + health pós ok deveria resultar em reparado")
	require.NotEmpty(t, r.BackupPath, "repair de sucesso deve ter backup forense")

	// Banco regenerado está saudável.
	ok, detail, hErr := Health(db)
	require.NoError(t, hErr)
	require.True(t, ok, "health pós deveria estar limpo; detail=%s", detail)

	// Ledger limpo (RecordSuccess idempotente removeu o sidecar).
	assertFileMissing(t, LedgerPath(db))

	// Backup forense com dedupe: 1 backup do arquivo doente.
	backupDir := filepath.Join(root, "backups", "staterepair")
	require.Len(t, existingBackups(backupDir, "session"), 1)

	// Quarentena limpa após sucesso.
	qdir := quarantineDir(db)
	entries, _ := os.ReadDir(qdir)
	require.Empty(t, entries, "quarentena deve ser limpa após repair bem-sucedido")
}

// TestRepair_ApplyFailureRestoresOriginalAndRecords prova o "nunca
// meio-repair": recipe falha → original doente é RESTAURADO da quarentena,
// ledger registra a falha e o relatório traz instruções manuais.
func TestRepair_ApplyFailureRestoresOriginalAndRecords(t *testing.T) {
	root, db := repairFixture(t, true)
	original := readFileBytes(t, db)

	mgr := NewManager(root)
	mgr.runRecipe = func(context.Context, string, Recipe) (string, error) {
		return "fake recipe output", os.ErrPermission // falha simulada
	}

	reports, repErr := mgr.Repair(KindSession, Options{Apply: true})
	require.NoError(t, repErr)
	require.Len(t, reports, 1)

	r := reports[0]
	require.Equal(t, StatusRecipeFailed, r.Status)
	require.Contains(t, r.ManualInstructions, "RESTAURADO")
	require.Contains(t, r.Error, "recipe falhou")
	require.Contains(t, r.RecipeOutput, "fake recipe output")

	// Original doente restaurado byte a byte — nada foi perdido.
	require.Equal(t, original, readFileBytes(t, db), "falha de recipe deve restaurar o original intacto")

	// Ledger registrou 1 falha do fingerprint atual.
	entries := ReadAttempts(db)
	require.Len(t, entries, 1)
	fp, err := Fingerprint(db)
	require.NoError(t, err)
	require.Equal(t, 1, entries[fp].Attempts)

	// Quarentena vazia (original restaurado); backup forense preservado.
	qdir := quarantineDir(db)
	qEntries, _ := os.ReadDir(qdir)
	require.Empty(t, qEntries, "quarentena deve esvaziar após restauração")
	require.Len(t, existingBackups(filepath.Join(root, "backups", "staterepair"), "session"), 1)
}

// TestRepair_ApplyExhaustedLedgerRefuses prova o critério de aceite 2 no nível
// do fluxo: 3 falhas no MESMO fingerprint → o Repair recusa nova cirurgia sem
// tocar no arquivo (sem backup novo, sem quarentena).
func TestRepair_ApplyExhaustedLedgerRefuses(t *testing.T) {
	root, db := repairFixture(t, true)
	original := readFileBytes(t, db)

	fp, err := Fingerprint(db)
	require.NoError(t, err)
	for i := 0; i < MaxAttempts; i++ {
		require.NoError(t, RecordFailure(db, fp))
	}
	require.True(t, AttemptsExhausted(db, fp))

	mgr := NewManager(root)
	mgr.runRecipe = fakeRecipeRecreate(t)

	reports, repErr := mgr.Repair(KindSession, Options{Apply: true})
	require.NoError(t, repErr)
	require.Len(t, reports, 1)

	r := reports[0]
	require.Equal(t, StatusAttemptsExhausted, r.Status)
	require.Empty(t, r.BackupPath, "ledger exausto recusa ANTES do backup")
	require.Contains(t, r.ManualInstructions, "Recuperação manual")

	// Arquivo intacto no lugar — nenhuma cirurgia foi tentada.
	require.Equal(t, original, readFileBytes(t, db))
	assertFileMissing(t, filepath.Join(root, "backups"))
	assertFileMissing(t, filepath.Join(root, "quarantine"))
}

// TestRepair_MissingFileNadaAReparar prova que arquivo ausente não dispara
// cirurgia (reporta "missing") — mesmo com --apply.
func TestRepair_MissingFileNadaAReparar(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".cosca")
	require.NoError(t, os.MkdirAll(root, 0o755))

	mgr := NewManager(root)
	mgr.runRecipe = fakeRecipeRecreate(t)

	reports, repErr := mgr.Repair(KindSession, Options{Apply: true})
	require.NoError(t, repErr)
	require.Len(t, reports, 1)
	require.Equal(t, StatusMissing, reports[0].Status)

	assertFileMissing(t, filepath.Join(root, "session.db"))
	assertFileMissing(t, filepath.Join(root, "backups"))
	assertFileMissing(t, filepath.Join(root, "quarantine"))
}

// TestRepair_KindAllCoversAllSupported prova que `all` expande para todas as
// classes concretas reparáveis (e o glob vector sem matches não gera report).
func TestRepair_KindAllCoversAllSupported(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".cosca")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "memory"), 0o755))
	// knowledge.db saudável apenas — as demais ausentes.
	seedSQLite(t, filepath.Join(root, "knowledge.db"))

	mgr := NewManager(root)
	reports, repErr := mgr.Repair(KindAll, Options{})
	require.NoError(t, repErr)
	// session + index + knowledge (vector sem matches não gera report).
	require.Len(t, reports, 3)

	got := map[DBKind]Status{}
	for _, r := range reports {
		got[r.Kind] = r.Status
	}
	require.Equal(t, StatusOK, got[KindKnowledge])
	require.Equal(t, StatusMissing, got[KindSession])
	require.Equal(t, StatusMissing, got[KindIndex])

	// O glob vector-*.db sem matches expande para zero arquivos.
	vecPaths, err := mgr.DBPaths(KindVector)
	require.NoError(t, err)
	require.Empty(t, vecPaths)
}

// TestRepair_VectorGlobExpandsPartitions prova que a classe vector expande o
// glob vector-*.db em um report por partição existente.
func TestRepair_VectorGlobExpandsPartitions(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".cosca")
	require.NoError(t, os.MkdirAll(root, 0o755))
	seedSQLite(t, filepath.Join(root, "vector-code.db"))
	seedSQLite(t, filepath.Join(root, "vector-docs.db"))

	mgr := NewManager(root)
	paths, err := mgr.DBPaths(KindVector)
	require.NoError(t, err)
	require.Len(t, paths, 2)
	require.Contains(t, paths[0], "vector-code.db")
	require.Contains(t, paths[1], "vector-docs.db")

	reports, repErr := mgr.Repair(KindVector, Options{})
	require.NoError(t, repErr)
	require.Len(t, reports, 2)
	for _, r := range reports {
		require.Equal(t, StatusOK, r.Status)
	}
}

// TestRepair_AmbiguousModesRejected prova que Apply+DryRun juntos é erro.
func TestRepair_AmbiguousModesRejected(t *testing.T) {
	mgr := NewManager(filepath.Join(t.TempDir(), ".cosca"))
	_, err := mgr.Repair(KindSession, Options{Apply: true, DryRun: true})
	require.Error(t, err)
}

// TestSupportedKinds_NeverChainOrEmbed prova o critério de aceite 6: a lista
// de classes reparáveis NÃO contém chain (family_chain.dat), embed
// (internal/embed/cosca) nem identidade — e nenhuma recipe os referencia.
func TestSupportedKinds_NeverChainOrEmbed(t *testing.T) {
	for _, k := range SupportedKinds() {
		require.NotEqual(t, "chain", k.String())
		require.NotEqual(t, "embed", k.String())
		require.NotEqual(t, "identity", k.String())
	}

	recipe, err := recipeFor(SupportedKinds()[0])
	require.NoError(t, err)
	require.NotContains(t, recipe.Label, "chain")

	for _, kind := range SupportedKinds() {
		r, rErr := recipeFor(kind)
		require.NoError(t, rErr)
		for _, s := range r.Steps {
			joined := strings.Join(s.Args, " ")
			require.NotContains(t, joined, "family_chain")
			require.NotContains(t, joined, "embed")
			require.NotContains(t, joined, ".dat")
		}
		require.NotContains(t, r.ManualInstructions, "internal/embed/cosca")
	}

	require.Len(t, SupportedKinds(), 4)
}

// TestParseKind_ValidaSeletores cobre o parsing da CLI (--db).
func TestParseKind_ValidaSeletores(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want DBKind
	}{
		{"", KindAll},
		{"all", KindAll},
		{"session", KindSession},
		{"index", KindIndex},
		{"knowledge", KindKnowledge},
		{"vector", KindVector},
	} {
		got, err := ParseKind(tc.in)
		require.NoError(t, err)
		require.Equal(t, tc.want, got, "ParseKind(%q)", tc.in)
	}

	_, err := ParseKind("chain")
	require.Error(t, err, "chain não é classe reparável")
	_, err = ParseKind("family_chain.dat")
	require.Error(t, err)
}
