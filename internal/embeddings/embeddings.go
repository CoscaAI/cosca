// Package embeddings provides the embedding provider interface for the Cosca Knowledge Engine.
// It defines the abstraction for generating vector embeddings from text content.
package embeddings

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
)

// EmbeddingResult holds the result of a single embedding operation.
type EmbeddingResult struct {
	Vector     []float64 `json:"vector"`
	Model      string    `json:"model"`
	Dimensions int       `json:"dimensions"`
	TokensUsed int       `json:"tokens_used"`
}

// Provider defines the interface for embedding generation providers.
type Provider interface {
	// GenerateEmbedding generates an embedding vector for a single text string.
	GenerateEmbedding(ctx context.Context, text string) (*EmbeddingResult, error)

	// GenerateEmbeddings generates embedding vectors for a batch of text strings.
	GenerateEmbeddings(ctx context.Context, texts []string) ([]*EmbeddingResult, error)

	// Model returns the name of the embedding model being used.
	Model() string

	// Dimensions returns the dimensionality of the generated embeddings.
	Dimensions() int

	// Name returns the provider name.
	Name() string

	// Close cleans up any provider resources.
	Close() error
}

// BatchConfig defines configuration for batch embedding generation.
type BatchConfig struct {
	// BatchSize is the maximum number of texts to embed in a single batch.
	BatchSize int

	// MaxRetries is the maximum number of retry attempts per batch.
	MaxRetries int

	// ShowProgress enables progress reporting during batch processing.
	ShowProgress bool
}

// DefaultBatchConfig returns sensible default batch configuration.
func DefaultBatchConfig() BatchConfig {
	return BatchConfig{
		BatchSize:    20,
		MaxRetries:   3,
		ShowProgress: false,
	}
}

// EmbeddingStats tracks embedding usage statistics.
type EmbeddingStats struct {
	mu              sync.RWMutex
	TotalRequests   int                       `json:"total_requests"`
	TotalTokens     int                       `json:"total_tokens"`
	TotalDimensions int64                     `json:"total_dimensions"`
	Errors          int                       `json:"errors"`
	ProviderStats   map[string]*ProviderStats `json:"provider_stats"`
}

// ProviderStats tracks per-provider statistics.
type ProviderStats struct {
	Requests int `json:"requests"`
	Tokens   int `json:"tokens"`
	Errors   int `json:"errors"`
}

// NewEmbeddingStats creates a new embedding stats tracker.
func NewEmbeddingStats() *EmbeddingStats {
	return &EmbeddingStats{
		ProviderStats: make(map[string]*ProviderStats),
	}
}

// RecordRequest records a successful embedding request.
func (s *EmbeddingStats) RecordRequest(providerName string, tokens int, dimensions int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.TotalRequests++
	s.TotalTokens += tokens
	s.TotalDimensions += int64(dimensions)

	if _, ok := s.ProviderStats[providerName]; !ok {
		s.ProviderStats[providerName] = &ProviderStats{}
	}
	s.ProviderStats[providerName].Requests++
	s.ProviderStats[providerName].Tokens += tokens
}

// RecordError records a failed embedding request.
func (s *EmbeddingStats) RecordError(providerName string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Errors++
	if _, ok := s.ProviderStats[providerName]; !ok {
		s.ProviderStats[providerName] = &ProviderStats{}
	}
	s.ProviderStats[providerName].Errors++
}

// Snapshot returns a consistent, independent copy of the statistics.
//
// The returned value can be read without synchronization and its provider map
// may be safely retained by the caller. This is also the preferred way to
// inspect stats while requests are being recorded.
func (s *EmbeddingStats) Snapshot() *EmbeddingStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := &EmbeddingStats{
		TotalRequests:   s.TotalRequests,
		TotalTokens:     s.TotalTokens,
		TotalDimensions: s.TotalDimensions,
		Errors:          s.Errors,
		ProviderStats:   make(map[string]*ProviderStats, len(s.ProviderStats)),
	}
	for provider, stats := range s.ProviderStats {
		if stats == nil {
			snapshot.ProviderStats[provider] = nil
			continue
		}
		providerSnapshot := *stats
		snapshot.ProviderStats[provider] = &providerSnapshot
	}
	return snapshot
}

// MarshalJSON preserves the existing JSON representation while taking a
// consistent read of the mutable statistics.
func (s *EmbeddingStats) MarshalJSON() ([]byte, error) {
	snapshot := s.Snapshot()
	type embeddingStatsJSON EmbeddingStats
	return json.Marshal((*embeddingStatsJSON)(snapshot))
}

// ValidateEmbedding checks that an embedding result is valid.
func ValidateEmbedding(result *EmbeddingResult) error {
	if result == nil {
		return fmt.Errorf("embedding result is nil")
	}
	if len(result.Vector) == 0 {
		return fmt.Errorf("embedding vector is empty")
	}
	if result.Model == "" {
		return fmt.Errorf("model name is empty")
	}
	if result.Dimensions <= 0 {
		return fmt.Errorf("invalid dimensions: %d", result.Dimensions)
	}
	if len(result.Vector) != result.Dimensions {
		return fmt.Errorf("vector length %d does not match dimensions %d", len(result.Vector), result.Dimensions)
	}
	for i, value := range result.Vector {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("vector contains non-finite value at index %d", i)
		}
	}
	return nil
}

// CosineSimilarity computes the cosine similarity between two vectors.
func CosineSimilarity(a, b []float64) (float64, error) {
	if len(a) != len(b) {
		return 0, fmt.Errorf("vector dimension mismatch: %d vs %d", len(a), len(b))
	}

	var dotProduct, normA, normB float64
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0, fmt.Errorf("zero vector encountered")
	}

	return dotProduct / (sqrt(normA) * sqrt(normB)), nil
}

// sqrt computes the square root using Newton's method.
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

// AverageVectors computes the element-wise average of multiple vectors.
func AverageVectors(vectors [][]float64) ([]float64, error) {
	if len(vectors) == 0 {
		return nil, fmt.Errorf("no vectors to average")
	}

	dims := len(vectors[0])
	for _, v := range vectors {
		if len(v) != dims {
			return nil, fmt.Errorf("inconsistent dimensions: %d vs %d", dims, len(v))
		}
	}

	result := make([]float64, dims)
	for _, v := range vectors {
		for i := 0; i < dims; i++ {
			result[i] += v[i]
		}
	}

	n := float64(len(vectors))
	for i := 0; i < dims; i++ {
		result[i] /= n
	}

	return result, nil
}

// ensure Context is used
var _ = context.Background
