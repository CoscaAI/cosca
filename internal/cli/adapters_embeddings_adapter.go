package cli

// Embeddings Adapter
// =============================================================================

// embeddingGeneratorAdapter wraps embedding generation. Placeholder.
type embeddingGeneratorAdapter struct {
	dir string
}

// newEmbeddingGeneratorAdapter creates a new embedding generator adapter.
func newEmbeddingGeneratorAdapter(dir string) *embeddingGeneratorAdapter {
	return &embeddingGeneratorAdapter{dir: dir}
}

// Generate generates embeddings. Placeholder for install flow.
func (a *embeddingGeneratorAdapter) Generate() error {
	return nil
}

// Update updates embeddings with changes. Placeholder for sync flow.
func (a *embeddingGeneratorAdapter) Update(_ interface{}) error {
	return nil
}

// =============================================================================
