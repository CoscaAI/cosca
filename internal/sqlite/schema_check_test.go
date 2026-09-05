package sqlite

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

func TestDDLGeneration(t *testing.T) {
	t.Parallel()

	s := NewSchema()
	ddl := s.DDL()
	require.NotEmpty(t, ddl, "DDL should not be empty")
	assert.Contains(t, ddl, "CREATE TABLE IF NOT EXISTS documents")
	assert.Contains(t, ddl, "CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts")
	assert.Contains(t, ddl, "CREATE TRIGGER IF NOT EXISTS documents_ai")
}

func TestNewSchema(t *testing.T) {
	t.Parallel()

	s := NewSchema()
	assert.NotNil(t, s)
	assert.Equal(t, 1, s.Version())
	assert.NotEmpty(t, s.Statements())
	assert.Greater(t, len(s.Statements()), 40, "should have many DDL statements")
}

func TestTableNames(t *testing.T) {
	t.Parallel()

	names := TableNames()
	assert.Len(t, names, 18)
	assert.Contains(t, names, "documents")
	assert.Contains(t, names, "chunks")
	assert.Contains(t, names, "entities")
	assert.Contains(t, names, "documents_fts")
	assert.Contains(t, names, "chunks_fts")
	assert.Contains(t, names, "migration_history")
}

func TestEntityTypes(t *testing.T) {
	t.Parallel()

	types := EntityTypes()
	assert.NotEmpty(t, types)
	assert.Contains(t, types, "agent")
	assert.Contains(t, types, "skill")
	assert.Contains(t, types, "prompt")
	assert.Contains(t, types, "workflow")
	assert.Contains(t, types, "template")
}

func TestRelationshipTypes(t *testing.T) {
	t.Parallel()

	types := RelationshipTypes()
	assert.NotEmpty(t, types)
	assert.Contains(t, types, "depends_on")
	assert.Contains(t, types, "extends")
	assert.Contains(t, types, "implements")
	assert.Contains(t, types, "related_to")
}

func TestSectionTypes(t *testing.T) {
	t.Parallel()

	types := SectionTypes()
	assert.NotEmpty(t, types)
	assert.Contains(t, types, "text")
	assert.Contains(t, types, "code")
	assert.Contains(t, types, "table")
}

func TestDocumentTypes(t *testing.T) {
	t.Parallel()

	types := DocumentTypes()
	assert.NotEmpty(t, types)
	assert.Contains(t, types, "markdown")
	assert.Contains(t, types, "go")
	assert.Contains(t, types, "typescript")
	assert.Contains(t, types, "yaml")
}

func TestSchemaHash(t *testing.T) {
	t.Parallel()

	s := NewSchema()
	hash := s.SchemaHash()
	assert.Equal(t, "cosca-kg-v1", hash)
}

func TestSchemaVersion(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 1, SchemaVersion)
}

func TestSchemaDDLNotEmpty(t *testing.T) {
	t.Parallel()

	s := NewSchema()
	ddl := s.DDL()
	assert.Contains(t, ddl, "documents")
	assert.Contains(t, ddl, "chunks")
	assert.Contains(t, ddl, "headings")
	assert.Contains(t, ddl, "code_blocks")
	assert.Contains(t, ddl, "tables")
	assert.Contains(t, ddl, "symbols")
	assert.Contains(t, ddl, "entities")
	assert.Contains(t, ddl, "relationships")
	assert.Contains(t, ddl, "cache")
	assert.Contains(t, ddl, "snapshots")
}
