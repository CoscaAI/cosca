package grounding

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRealBaselineFile_MatchesFixture: o gerador tipado (RealBaselineFile) deve
// ser idêntico ao fixture real em testdata/qrels-baseline.json. É a prova de que
// o baseline congelado NÃO foi escrito à mão (nem é fictício): é derivado do
// ground-truth do benchmark (benchmarkQueries).
func TestRealBaselineFile_MatchesFixture(t *testing.T) {
	t.Parallel()

	gen := RealBaselineFile()
	data, err := os.ReadFile(filepath.Join("testdata", "qrels-baseline.json"))
	require.NoError(t, err)

	var onDisk QrelsFile
	require.NoError(t, json.Unmarshal(data, &onDisk))

	assert.Equal(t, gen.Version, onDisk.Version)
	assert.Equal(t, gen.Embedding, onDisk.Embedding)
	require.Len(t, onDisk.Queries, len(gen.Queries))
	for i := range gen.Queries {
		assert.Equal(t, gen.Queries[i].Query, onDisk.Queries[i].Query)
		assert.ElementsMatch(t, gen.Queries[i].RelevantChunkIDs, onDisk.Queries[i].RelevantChunkIDs)
		assert.Equal(t, gen.Queries[i].FirstRelevantChunkID, onDisk.Queries[i].FirstRelevantChunkID)
	}
}

// TestRealBaselineFile_Valid: o baseline real passa no ValidateQrels (fail-closed
// de contrato do gabarito) — é a garantia de que o gate aceitará o gabarito.
func TestRealBaselineFile_Valid(t *testing.T) {
	t.Parallel()

	err := ValidateQrels(RealBenchmarkQrels())
	assert.NoError(t, err)
}

// TestRealBenchmarkDocuments_NonEmpty: o mapa chunk→document do GT é completo,
// sem chunk vazio ou repetido.
func TestRealBenchmarkDocuments_NonEmpty(t *testing.T) {
	t.Parallel()

	docs := RealBenchmarkDocuments()
	require.Len(t, docs, 6)
	for chunk, doc := range docs {
		assert.NotEmpty(t, chunk)
		assert.NotEmpty(t, doc)
	}
}
