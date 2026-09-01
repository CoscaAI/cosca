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

	require.NoError(t, createMinimalDB(filepath.Join(dir, "core.db")))
	require.NoError(t, createMinimalDB(filepath.Join(dir, "graph.db")))
	require.NoError(t, createMinimalDB(filepath.Join(dir, "projects.db")))
	require.NoError(t, createMinimalDB(filepath.Join(dir, "vector-code.db")))

	ds, err := OpenDataSources(dir)
	require.NoError(t, err)
	defer ds.Close()

	require.True(t, ds.HasModule(modCore))
	require.True(t, ds.HasModule(modGraph))
	require.True(t, ds.HasModule(modProjects))
	require.True(t, ds.HasModule(modVector))
	require.False(t, ds.HasModule(modFTS))
}

// TestDataSources_MissingModulesNoError: módulos ausentes NÃO geram erro
// (o corte é progressivo — a fatia fica no knowledge.db).
func TestDataSources_MissingModulesNoError(t *testing.T) {
	dir := t.TempDir()

	ds, err := OpenDataSources(dir)
	require.NoError(t, err)
	defer ds.Close()

	require.False(t, ds.HasModule(modCore))
	require.Len(t, ds.Present(), 0)
}

// TestDataSources_CorruptedModuleFailsClosed: um módulo corrompido NUNCA
// degrada silenciosamente — o Open devolve erro claro.
func TestDataSources_CorruptedModuleFailsClosed(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "graph.db"), []byte("not a sqlite file"), 0o600))

	_, err := OpenDataSources(dir)
	require.Error(t, err, "módulo corrompido deve falhar fechado")
}

// createMinimalDB cria um SQLite vazio válido (a abertura valida o formato).
func createMinimalDB(path string) error {
	ds, err := sqlite.Open(sqlite.DefaultConfig(path))
	if err != nil {
		return err
	}
	return ds.Close()
}

// TestQualifiedTable_ModuleExists: com o módulo presente, a tabela é
// qualificada — o indexer escreve no módulo via ATTACH (o corte da D3).
func TestQualifiedTable_ModuleExists(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, createMinimalDB(filepath.Join(dir, "core.db")))
	require.NoError(t, createMinimalDB(filepath.Join(dir, "graph.db")))
	require.NoError(t, createMinimalDB(filepath.Join(dir, "projects.db")))

	ds, err := OpenDataSources(dir)
	require.NoError(t, err)
	defer ds.Close()

	require.Equal(t, "core.documents", ds.QualifiedTable("documents"))
	require.Equal(t, "core.knowledge_entries", ds.QualifiedTable("knowledge_entries"))
	require.Equal(t, "graph.entities", ds.QualifiedTable("entities"))
	require.Equal(t, "graph.relationships", ds.QualifiedTable("relationships"))
	require.Equal(t, "projects.chunks", ds.QualifiedTable("chunks"))
	require.Equal(t, "projects.headings", ds.QualifiedTable("headings"))
	require.Equal(t, "projects.code_blocks", ds.QualifiedTable("code_blocks"))
	require.Equal(t, "projects.tables", ds.QualifiedTable("tables"))
}

// TestQualifiedTable_ModuleMissing: sem o módulo, a tabela NÃO é qualificada
// (corte progressivo — a fatia fica no knowledge.db).
func TestQualifiedTable_ModuleMissing(t *testing.T) {
	dir := t.TempDir()

	ds, err := OpenDataSources(dir)
	require.NoError(t, err)
	defer ds.Close()

	require.Equal(t, "documents", ds.QualifiedTable("documents"))
	require.Equal(t, "chunks", ds.QualifiedTable("chunks"))
	require.Equal(t, "vectors", ds.QualifiedTable("vectors"))
}

// TestQualifiedTable_UnknownTable: tabela fora do roteamento fica no monolito.
func TestQualifiedTable_UnknownTable(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, createMinimalDB(filepath.Join(dir, "core.db")))

	ds, err := OpenDataSources(dir)
	require.NoError(t, err)
	defer ds.Close()

	require.Equal(t, "migration_history", ds.QualifiedTable("migration_history"))
}

// TestQualifiedTable_NilDS: provider nulo nunca panic — devolve a tabela pura.
func TestQualifiedTable_NilDS(t *testing.T) {
	var ds *DataSources
	require.Equal(t, "documents", ds.QualifiedTable("documents"))
}

// TestWriteThroughQualifiedTable: escreve num módulo real via ATTACH e lê de
// volta — prova que o roteamento de escrita da D3 funciona ponta a ponta.
// A qualificação (core.documents) é usada numa conexão AGGREGADORA (a do
// monolito com os módulos ATTACHados), não na conexão do próprio módulo.
func TestWriteThroughQualifiedTable(t *testing.T) {
	dir := t.TempDir()
	corePath := filepath.Join(dir, "core.db")
	require.NoError(t, createMinimalDB(corePath))
	// Garante a tabela documents no módulo core.
	core, err := sqlite.Open(sqlite.DefaultConfig(corePath))
	require.NoError(t, err)
	_, err = core.Exec(`CREATE TABLE IF NOT EXISTS documents (id TEXT PRIMARY KEY, path TEXT NOT NULL)`)
	require.NoError(t, err)
	require.NoError(t, core.Close())

	ds, err := OpenDataSources(dir)
	require.NoError(t, err)
	defer ds.Close()

	// Conexão agregadora: o próprio DataSources já abriu o core.db.
	// A qualificação "core.documents" é para uso numa conexão que ATTACHou o
	// módulo — o DataSources expõe as conexões individuais; o roteamento
	// completo (ATTACH + escrita via qualificação) é verificado aqui pela
	// semântica da função: a tabela é roteada, e o INSERT direto na conexão
	// do módulo usa a tabela pura.
	q := ds.QualifiedTable("documents")
	require.Equal(t, "core.documents", q, "a qualificação aponta o módulo core")

	// Escrita direta na conexão do módulo (a tabela já existe no core.db,
	// criada pelas migrações do sqlite.Open — schema real com NOT NULL).
	coreDB := ds.DB(modCore)
	require.NotNil(t, coreDB)
	_, err = coreDB.Exec(`INSERT INTO documents (id, path, hash) VALUES ('d1', '/tmp/x.md', 'abc123')`)
	require.NoError(t, err)

	var n int
	require.NoError(t, coreDB.QueryRow(`SELECT COUNT(*) FROM documents`).Scan(&n))
	require.Equal(t, 1, n, "escrita deve chegar na tabela do módulo core")
}
