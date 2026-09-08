// health_test.go — Health detecta banco corrompido sintético (critério de
// aceite 4 do ADR-043 §8).
package staterepair

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestHealth_OkOnValidSQLite prova que um SQLite válido é saudável.
func TestHealth_OkOnValidSQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ok.db")
	seedSQLite(t, path)

	ok, detail, err := Health(path)
	require.NoError(t, err)
	require.True(t, ok, "banco válido deveria passar no quick_check; detail=%s", detail)
}

// TestHealth_SickOnCorruptSQLite prova o critério central: um sqlite válido
// com bytes lixo no MEIO é detectado como doente.
func TestHealth_SickOnCorruptSQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corrupt.db")
	seedSQLite(t, path)
	corruptSQLiteMidFile(t, path)

	ok, detail, err := Health(path)
	require.NoError(t, err)
	require.False(t, ok, "sqlite corrompido no meio deveria ser detectado")
	require.NotEmpty(t, detail)
}

// TestHealth_SickOnNonSQLite prova que um arquivo que não é SQLite é doente.
func TestHealth_SickOnNonSQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "garbage.db")
	require.NoError(t, os.WriteFile(path, []byte("isto não é um banco sqlite — apenas texto"), 0o600))

	ok, detail, err := Health(path)
	require.NoError(t, err)
	require.False(t, ok, "arquivo que não é SQLite deveria ser doente")
	require.NotEmpty(t, detail)
}

// TestHealth_MissingFile prova que arquivo ausente é reportado como não ok
// (mas sem erro de infraestrutura).
func TestHealth_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ausente.db")

	ok, detail, err := Health(path)
	require.NoError(t, err)
	require.False(t, ok)
	require.Contains(t, detail, "não existe")
}

// TestHealth_ReadOnlyDoesNotCreateSidecars prova que o healthcheck read-only
// não cria arquivos ao lado do banco (modo check é 100% read-only).
func TestHealth_ReadOnlyDoesNotCreateSidecars(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ro.db")
	seedSQLite(t, path)

	before := treeSnapshot(t, dir)
	ok, _, err := Health(path)
	require.NoError(t, err)
	require.True(t, ok)
	after := treeSnapshot(t, dir)
	require.Equal(t, before, after, "healthcheck não pode criar/alterar arquivos")
}
