// Package grounding tests — Qrels: LoadQrels (layout envolto + array puro) e
// ValidateQrels (fail-closed de contrato do gabarito).
package grounding

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTempJSON escreve `content` em um arquivo dentro de um t.TempDir e
// devolve o caminho absoluto.
func writeTempJSON(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

// TestLoadQrels_WrappedLayout: lê {version, embedding, queries:[...]}.
func TestLoadQrels_WrappedLayout(t *testing.T) {
	t.Parallel()

	path := writeTempJSON(t, "qrels-baseline.json", `{
		"version": 1,
		"embedding": {"model": "nomic-embed-text", "dim": "768"},
		"queries": [
			{"query": "q1", "relevant_chunk_ids": ["c1", "c2"], "first_relevant_chunk_id": "c1"},
			{"query": "q2", "relevant_chunk_ids": ["c3"], "first_relevant_chunk_id": "c3"}
		]
	}`)

	qrels, err := LoadQrels(path)
	require.NoError(t, err)
	require.Len(t, qrels, 2)
	assert.Equal(t, "q1", qrels[0].Query)
	assert.Equal(t, []string{"c1", "c2"}, qrels[0].RelevantChunkIDs)
	assert.Equal(t, "c1", qrels[0].FirstRelevantChunkID)
	assert.Equal(t, "c3", qrels[1].FirstRelevantChunkID)
}

// TestLoadQrels_BareArray: um array puro [...] é aceito como layout alternativo.
func TestLoadQrels_BareArray(t *testing.T) {
	t.Parallel()

	path := writeTempJSON(t, "bare.json", `[
		{"query": "q-a", "relevant_chunk_ids": ["x1"], "first_relevant_chunk_id": "x1"}
	]`)

	qrels, err := LoadQrels(path)
	require.NoError(t, err)
	require.Len(t, qrels, 1)
	assert.Equal(t, "q-a", qrels[0].Query)
	assert.Equal(t, []string{"x1"}, qrels[0].RelevantChunkIDs)
}

// TestLoadQrels_SignatureCanonical: o campo embedding expõe a assinatura
// "model:dim" do baseline (C2).
func TestLoadQrels_SignatureCanonical(t *testing.T) {
	t.Parallel()

	// Reusa o layout envolto para verificar a assinatura via devolução; como
	// LoadQrels só devolve []Qrels, verificamos a assinatura no próprio
	// EmbeddingSig (tipo do QrelsFile) por unidade separada.
	sig := EmbeddingSig{Model: "nomic-embed-text", Dim: "768"}
	assert.Equal(t, "nomic-embed-text:768", sig.Signature())
	assert.Equal(t, "", EmbeddingSig{Model: "m"}.Signature())
}

// TestLoadQrels_MissingFile: arquivo inexistente → erro de carga.
func TestLoadQrels_MissingFile(t *testing.T) {
	t.Parallel()

	_, err := LoadQrels(filepath.Join(t.TempDir(), "nope.json"))
	require.Error(t, err)
	assert.ErrorContains(t, err, "load qrels")
}

// TestLoadQrels_InvalidFile: conteúdo inválido (relevant vazio) → erro de
// validação fail-closed.
func TestLoadQrels_InvalidFile(t *testing.T) {
	t.Parallel()

	path := writeTempJSON(t, "bad.json", `[{"query": "q", "relevant_chunk_ids": []}]`)
	_, err := LoadQrels(path)
	require.Error(t, err)
	assert.ErrorContains(t, err, "relevant_chunk_ids")
}

// TestLoadQrels_ParsingError: JSON inválido → erro de parse.
func TestLoadQrels_ParsingError(t *testing.T) {
	t.Parallel()

	path := writeTempJSON(t, "parse.json", `{{{not json`)
	_, err := LoadQrels(path)
	require.Error(t, err)
}

// TestValidateQrels_Valid: um gabarito válido passa sem erro.
func TestValidateQrels_Valid(t *testing.T) {
	t.Parallel()

	err := ValidateQrels(validBaseline())
	assert.NoError(t, err)
}

// TestValidateQrels_Rejections: regras fail-closed do ADR.
func TestValidateQrels_Rejections(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		qrels   []Qrels
		wantErr string
	}{
		{"vazio", nil, "vazio"},
		{"empty query", []Qrels{{Query: "   ", RelevantChunkIDs: []string{"c1"}}}, "query vazia"},
		{"relevant vazio", []Qrels{{Query: "q", RelevantChunkIDs: nil}}, "sem relevant_chunk_ids"},
		{"relevant vazio (empty slice)", []Qrels{{Query: "q", RelevantChunkIDs: []string{}}}, "sem relevant_chunk_ids"},
		{"chunk id vazio", []Qrels{{Query: "q", RelevantChunkIDs: []string{""}}}, "vazio"},
		{"first_relevant fora do relevant", []Qrels{{Query: "q", RelevantChunkIDs: []string{"c1"}, FirstRelevantChunkID: "c9"}}, "não está em relevant_chunk_ids"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateQrels(tt.qrels)
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// TestLoadQrels_TestdataFixture: o fixture com 6 queries REAIS em testdata é
// válido e carrega 6 gabaritos. As queries e os chunk_ids reais vêm do
// ground-truth pré-registrado do benchmark (internal/search/vectorevidence_
// bench_test.go, benchmarkQueries) — NÃO são IDs fictícios.
func TestLoadQrels_TestdataFixture(t *testing.T) {
	t.Parallel()

	qrels, err := LoadQrels(filepath.Join("testdata", "qrels-baseline.json"))
	require.NoError(t, err)
	require.Len(t, qrels, 6)
	assert.Equal(t, "hot reload atualiza arquivos markdown sem reiniciar o runtime", qrels[0].Query)
	assert.Equal(t, "47659d61-351f-4d91-9889-d8fc5f31e643", qrels[0].FirstRelevantChunkID)
	assert.Equal(t, []string{
		"47659d61-351f-4d91-9889-d8fc5f31e643",
		"b7783a8a-1baf-4eb2-9ca8-dccde79d65b7",
		"c6890d0e-a5df-4be3-9e89-827b2c09b1ae",
	}, qrels[0].RelevantChunkIDs)
}
