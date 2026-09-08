// backup_test.go — backup forense com dedupe por conteúdo e retenção máxima
// (critério de aceite 3 do ADR-043 §8).
package staterepair

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// backupWorkDir monta um diretório `.cosca` de teste e devolve o caminho do
// banco doente dentro dele (para o backup cair em .cosca/backups/staterepair).
func backupWorkDir(t *testing.T) (coscaDir, sickPath string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), ".cosca")
	require.NoError(t, os.MkdirAll(root, 0o755))
	return root, filepath.Join(root, "session.db")
}

// writeSickFile grava bytes arbitrários no "banco doente".
func writeSickFile(t *testing.T, path string, content []byte) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, content, 0o600))
}

// TestForensicBackup_DedupeSameContent prova que um arquivo doente idêntico
// NUNCA é copiado 2x: 2 repairs do mesmo conteúdo → 1 backup (o mesmo
// caminho reusado).
func TestForensicBackup_DedupeSameContent(t *testing.T) {
	coscaDir, sickPath := backupWorkDir(t)
	writeSickFile(t, sickPath, []byte("conteúdo doente idêntico — bytes crus"))

	first, err := ForensicBackup(sickPath)
	require.NoError(t, err)
	require.NotEmpty(t, first)

	second, err := ForensicBackup(sickPath)
	require.NoError(t, err)
	require.Equal(t, first, second, "2 repairs do MESMO conteúdo devem reusar o MESMO backup")

	backups := existingBackups(filepath.Join(coscaDir, "backups", "staterepair"), "session")
	require.Len(t, backups, 1, "conteúdo idêntico nunca é copiado 2x")
}

// TestForensicBackup_RetentionMax3 prova que apenas as 3 cópias mais novas
// por banco sobrevivem (a 4ª distinta remove a mais velha).
func TestForensicBackup_RetentionMax3(t *testing.T) {
	coscaDir, sickPath := backupWorkDir(t)

	backupDir := filepath.Join(coscaDir, "backups", "staterepair")
	contents := [][]byte{
		[]byte("estado doente v1"),
		[]byte("estado doente v2"),
		[]byte("estado doente v3"),
		[]byte("estado doente v4"),
	}
	for _, c := range contents {
		writeSickFile(t, sickPath, c)
		_, err := ForensicBackup(sickPath)
		require.NoError(t, err)
	}

	kept := existingBackups(backupDir, "session")
	require.Len(t, kept, MaxBackups, "retenção máxima de backups forenses violada")

	// A v4 (mais nova, copiada por último) deve estar entre as mantidas.
	var keptContent [][]byte
	for _, b := range kept {
		raw, err := os.ReadFile(b)
		require.NoError(t, err)
		keptContent = append(keptContent, raw)
	}
	require.Contains(t, keptContent, []byte("estado doente v4"))
	require.NotContains(t, keptContent, []byte("estado doente v1"), "a mais velha deve ser podada")
}

// TestForensicBackup_NameConvention prova a convenção de nome e localização:
// .cosca/backups/staterepair/<nome>-<ts>-<hash8>.db.
func TestForensicBackup_NameConvention(t *testing.T) {
	_, sickPath := backupWorkDir(t)
	writeSickFile(t, sickPath, []byte("banco doente para nome"))

	dest, err := ForensicBackup(sickPath)
	require.NoError(t, err)

	require.Contains(t, filepath.ToSlash(dest), "/backups/staterepair/")
	name := filepath.Base(dest)
	require.Contains(t, name, "session-")
	require.Contains(t, name, ".db")
	// hash8 no nome: <nome>-<ts>-<hash8>.db → entre o segundo '-' e o '.db'.
	require.Regexp(t, `^session-\d{8}_\d{6}-[0-9a-f]{8}\.db$`, name)
}

// TestForensicBackup_MissingFileDevolveErro prova que backup de arquivo
// inexistente falha (nunca cria backup vazio).
func TestForensicBackup_MissingFileDevolveErro(t *testing.T) {
	_, sickPath := backupWorkDir(t)
	_, err := ForensicBackup(sickPath)
	require.Error(t, err)
}
