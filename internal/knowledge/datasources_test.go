package knowledge

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/CoscaAI/cosca/internal/sqlite"
)

// TestDataSources_OpensExistingModules cria os módulos físicos (simulando o
// split Fase C) e confirma que o provider abre cada um.
func TestDataSources_OpensExistingModules(t *testing.T) {
	dir := t.TempDir()

	// Cria os arquivos de módulo (mínimos, apenas para validar abertura).
	require.NoError(t, createMinimalDB(filepath.Join(dir, "core.db")))
	require.NoError(t, createMinimalDB(filepath.Join(dir, "graph.db")))
	require.NoError(t, createMinimalDB(filepath.Join(dir, "projects.db")))
	require.NoError(t, createMinimalDB(filepath.Join(dir, "vector-code.db")))

	ds, err := OpenDataSources(dir)
	require.NoError(t, err)
	defer ds.Close()

	require.True(t, ds.HasModule(modCore), "core.db deve estar presente")
	require.True(t, ds.HasModule(modGraph), "graph.db deve estar presente")
	require.True(t, ds.HasModule(modProjects), "projects.db deve estar presente")
	require.True(t, ds.HasModule(modVector), "vector-*.db (partição) deve estar presente")
	require.False(t, ds.HasModule(modFTS), "fts.db ausente deve ser reportado como ausente")

	require.NotNil(t, ds.DB(modCore))
	require.NotEmpty(t, ds.Path(modCore))
}

// TestDataSources_MissingModulesNoError: módulos ausentes NÃO geram erro —
// o corte é progressivo (a fatia fica no knowledge.db até o módulo existir).
func TestDataSources_MissingModulesNoError(t *testing.T) {
	dir := t.TempDir()

	ds, err := OpenDataSources(dir)
	require.NoError(t, err, "dir vazio não pode gerar erro (corte progressivo)")
	defer ds.Close()

	require.False(t, ds.HasModule(modCore))
	require.Len(t, ds.Present(), 0)
}

// TestDataSources_CorruptedModuleFailsClosed: um módulo corrompido NUNCA
// degrada silenciosamente — o Open devolve erro claro.
func TestDataSources_CorruptedModuleFailsClosed(t *testing.T) {
	dir := t.TempDir()

	// Escreve lixo no graph.db (não é um SQLite válido).
	require.NoError(t, os.WriteFile(filepath.Join(dir, "graph.db"), []byte("not a sqlite file"), 0o600))

	_, err := OpenDataSources(dir)
	require.Error(t, err, "módulo corrompido deve falhar fechado")
}

// createMinimalDB cria um SQLite vazio válido (a abertura valida o formato).
func createMinimalDB(path string) error {
	// Abre via sqlite.Open (que valida/initializa) e fecha.
	ds, err := sqlite.Open(sqlite.DefaultConfig(path))
	if err != nil {
		return err
	}
	return ds.Close()
}
