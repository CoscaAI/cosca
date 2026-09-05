// Package vector provides the vector store interface for the Cosca Knowledge Engine.
// It defines the abstraction for storing and searching vector embeddings.
package vector

import (
	"database/sql"
	"time"
)

// VectorRecord represents a single vector record in the store.
//
//nolint:revive // Stutter name preserved for API compatibility — used as vector.VectorRecord externally.
type VectorRecord struct {
	// ID is the unique identifier for this vector.
	ID string `json:"id"`

	// Vector is the embedding vector as float64 values.
	Vector []float64 `json:"vector"`

	// Metadata is arbitrary key-value metadata.
	Metadata map[string]string `json:"metadata,omitempty"`

	// DocumentID is the source document ID.
	DocumentID string `json:"document_id,omitempty"`

	// ChunkID is the source chunk ID, if applicable.
	ChunkID string `json:"chunk_id,omitempty"`

	// EntityID is the source entity ID, if applicable.
	EntityID string `json:"entity_id,omitempty"`

	// Content is the original text content (for snippet generation).
	Content string `json:"content,omitempty"`
}

// SearchResult represents a single result from a vector search.
type SearchResult struct {
	// ID is the unique identifier of the matched record.
	ID string `json:"id"`

	// Score is the similarity score (higher = more similar).
	Score float64 `json:"score"`

	// Metadata is the metadata of the matched record.
	Metadata map[string]string `json:"metadata,omitempty"`

	// DocumentID is the source document ID.
	DocumentID string `json:"document_id,omitempty"`

	// ChunkID is the source chunk ID, if applicable.
	ChunkID string `json:"chunk_id,omitempty"`

	// EntityID is the source entity ID, if applicable.
	EntityID string `json:"entity_id,omitempty"`

	// Content is the original text content.
	Content string `json:"content,omitempty"`

	// Vector is the actual embedding (if requested).
	Vector []float64 `json:"vector,omitempty"`
}

// VectorStats provides statistics about the vector store.
//
//nolint:revive // Stutter name preserved for API compatibility — used as vector.VectorStats externally.
type VectorStats struct {
	// TotalVectors is the total number of stored vectors.
	TotalVectors int `json:"total_vectors"`

	// Dimensions is the dimensionality of stored vectors.
	Dimensions int `json:"dimensions"`

	// IndexType describes the index type (e.g., "flat", "hnsw").
	IndexType string `json:"index_type"`

	// MemoryUsage is the estimated memory usage in bytes.
	MemoryUsage int64 `json:"memory_usage_bytes"`

	// DeletedCount is the number of soft-deleted vectors.
	DeletedCount int `json:"deleted_count"`
}

// Store defines the interface for vector storage and retrieval.
type Store interface {
	// Store stores vectors in the database.
	// dimension specifies the expected dimensionality.
	Store(dimension int, vectors []VectorRecord) error

	// Search finds the closest vectors to the query vector.
	// Returns up to limit results sorted by similarity (descending).
	Search(query []float64, limit int) ([]SearchResult, error)

	// SearchWithFilter finds closest vectors with metadata filtering.
	SearchWithFilter(query []float64, limit int, filter map[string]string) ([]SearchResult, error)

	// Delete removes vectors by their IDs.
	Delete(ids []string) error

	// DeleteByDocument removes all vectors associated with a document.
	DeleteByDocument(documentID string) error

	// DeleteByEntity removes all vectors associated with an entity.
	DeleteByEntity(entityID string) error

	// Rebuild rebuilds the entire vector index from scratch.
	Rebuild() error

	// Stats returns current vector store statistics.
	Stats() (VectorStats, error)

	// Dimension returns the expected vector dimensionality.
	Dimension() int

	// Count returns the total number of stored vectors.
	Count() (int, error)

	// Close cleans up vector store resources.
	Close() error
}

// CandidateSearcher is implemented by stores that can restrict vector
// similarity to an explicit set of candidate IDs (plus a bounded pool of
// recent vectors) instead of scanning the full store. Layered search uses it
// to turn the vector layer from a full O(N) brute-force scan into a bounded
// scan over the lexical candidates already found by L1/L2.
type CandidateSearcher interface {
	// SearchWithCandidates returns the closest vectors among candidateIDs
	// plus the most-recentPool recently inserted vectors. Filter applies to
	// both sets. When both candidateIDs and recentPool are empty it degrades
	// to SearchWithFilter (full scan).
	SearchWithCandidates(query []float64, limit int, candidateIDs []string, recentPool int, filter map[string]string) ([]SearchResult, error)
}

// SearchMetrics descreve o caminho REAL percorrido por uma busca vetorial
// (campanha de performance, FASE 1): quantos vetores existem no store, quantos
// sobreviveram ao filtro de metadata/candidatos e quantos foram de fato
// escaneados — e por qual representação. TotalVectors == ScannedVectors
// significa full-scan; ScannedVectors << TotalVectors significa caminho
// híbrido/bounded (o alvo do L3). SearchWithMetrics coleta; os métodos
// públicos existentes (Search, SearchWithFilter, SearchWithCandidates)
// mantêm o contrato inalterado.
type SearchMetrics struct {
	TotalVectors       int
	MetadataCandidates int
	ScannedVectors     int
	ReturnedK          int
	Int8Enabled        bool
	UsedFloat32        bool
	Latency            time.Duration
}

// MetricsSearcher is the optional instrumented surface used by the campaign
// (FASE 1): same contract as SearchWithCandidates, plus metrics.
type MetricsSearcher interface {
	SearchWithMetrics(query []float64, limit int, candidateIDs []string, recentPool int, filter map[string]string) ([]SearchResult, SearchMetrics, error)
}

// TransactionalStore can stage vectors in a caller-owned SQLite transaction.
// Index replacement uses this optional capability so document rows and their
// vectors commit (or roll back) together. Implementations backed by another
// storage engine may omit it and are not suitable for atomic SQLite replace.
type TransactionalStore interface {
	StoreTx(tx *sql.Tx, dimension int, vectors []VectorRecord) error
}

// StoreTxCommitter is implemented by transactional stores that stage writes in
// a caller-owned transaction (StoreTx) and keep derived state (e.g. an
// in-memory index) consistent with COMMITTED data only. Callers that stage
// vectors via TransactionalStore MUST call StoreTxCommitted after their
// transaction commits — even when the transaction replaces existing rows.
type StoreTxCommitter interface {
	// StoreTxCommitted notifies the store that a StoreTx transaction has
	// committed. Safe to call even when no StoreTx is pending (no-op).
	StoreTxCommitted()
}

// SearchParams defines optional parameters for vector search.
type SearchParams struct {
	// Limit is the maximum number of results (default: 10).
	Limit int

	// Offset is the number of results to skip (default: 0).
	Offset int

	// Filter restricts results to records matching these metadata key-value pairs.
	Filter map[string]string

	// DocumentID restricts results to a specific document.
	DocumentID string

	// EntityType restricts results to a specific entity type.
	EntityType string

	// IncludeVectors includes the actual vector in results (default: false).
	IncludeVectors bool

	// MinScore filters results below this threshold (default: 0.0).
	MinScore float64
}

// DefaultSearchParams returns sensible defaults for vector search.
func DefaultSearchParams() SearchParams {
	return SearchParams{
		Limit:          10,
		Offset:         0,
		IncludeVectors: false,
		MinScore:       0.0,
	}
}

// CosineSimilarity computes the cosine similarity between two vectors.
func CosineSimilarity(a, b []float64) (float64, error) {
	if len(a) != len(b) {
		return 0, nil // mismatched dimensions yield zero similarity
	}

	var dotProduct, normA, normB float64
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0, nil
	}

	return dotProduct / (sqrt(normA) * sqrt(normB)), nil
}

// sqrt computes square root using Newton's method.
func sqrt(x float64) float64 {
	if x == 0 {
		return 0
	}
	z := x / 2
	for i := 0; i < 100; i++ {
		prev := z
		z -= (z*z - x) / (2 * z)
		if z == prev {
			break
		}
	}
	return z
}

// NormalizeVector normalizes a vector to unit length.
func NormalizeVector(v []float64) []float64 {
	var norm float64
	for _, val := range v {
		norm += val * val
	}
	norm = sqrt(norm)
	if norm == 0 {
		return v
	}

	result := make([]float64, len(v))
	for i, val := range v {
		result[i] = val / norm
	}
	return result
}
