// Package local provides a zero-dependency TF-IDF-based embedding provider.
// It serves as the fallback when no external embedding provider is available.
package local

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/CoscaAI/cosca/internal/embeddings"
	"github.com/rs/zerolog/log"
)

// Config defines configuration for the local embedding provider.
type Config struct {
	// Dimensions is the output vector dimensionality (default: 128).
	Dimensions int

	// MaxFeatures is the maximum number of features in the vocabulary (default: 10000).
	MaxFeatures int

	// MinDocFrequency is the minimum document frequency for a term (default: 1).
	MinDocFrequency int

	// EnableNormalization normalizes vectors to unit length (default: true).
	EnableNormalization bool
}

// DefaultConfig returns sensible defaults for local embeddings.
func DefaultConfig() Config {
	return Config{
		Dimensions:          128,
		MaxFeatures:         10000,
		MinDocFrequency:     1,
		EnableNormalization: true,
	}
}

// Provider implements the embeddings.Provider interface using TF-IDF.
type Provider struct {
	cfg         Config
	mu          sync.RWMutex
	vocabulary  map[string]int     // term -> index
	idf         map[string]float64 // term -> IDF value
	docCount    int
	termDocFreq map[string]int
}

// New creates a new local embedding provider.
func New(cfg Config) (*Provider, error) {
	if cfg.Dimensions <= 0 {
		cfg.Dimensions = 128
	}
	if cfg.MaxFeatures <= 0 {
		cfg.MaxFeatures = 10000
	}
	if cfg.MinDocFrequency <= 0 {
		cfg.MinDocFrequency = 1
	}

	p := &Provider{
		cfg:         cfg,
		vocabulary:  make(map[string]int),
		idf:         make(map[string]float64),
		termDocFreq: make(map[string]int),
	}

	log.Debug().
		Int("dimensions", cfg.Dimensions).
		Int("max_features", cfg.MaxFeatures).
		Msg("local embedding provider initialized")

	return p, nil
}

// Name returns the provider name.
func (p *Provider) Name() string { return "local" }

// Model returns a descriptive model name.
func (p *Provider) Model() string {
	return fmt.Sprintf("tf-idf-local-v%d", p.cfg.Dimensions)
}

// Dimensions returns the embedding dimensionality.
func (p *Provider) Dimensions() int { return p.cfg.Dimensions }

// Close cleans up provider resources.
func (p *Provider) Close() error { return nil }

// GenerateEmbedding generates a TF-IDF-based embedding for a single text.
func (p *Provider) GenerateEmbedding(ctx context.Context, text string) (*embeddings.EmbeddingResult, error) {
	results, err := p.GenerateEmbeddings(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no embedding result")
	}
	return results[0], nil
}

// GenerateEmbeddings generates TF-IDF-based embeddings for multiple texts.
func (p *Provider) GenerateEmbeddings(_ context.Context, texts []string) ([]*embeddings.EmbeddingResult, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("no texts provided")
	}

	results := make([]*embeddings.EmbeddingResult, len(texts))
	for i, text := range texts {
		if text == "" {
			// Return zero vector for empty text
			vector := make([]float64, p.cfg.Dimensions)
			results[i] = &embeddings.EmbeddingResult{
				Vector:     vector,
				Model:      p.Model(),
				Dimensions: p.cfg.Dimensions,
				TokensUsed: 0,
			}
			continue
		}

		// Build term frequency for this document
		tf := p.termFrequency(text)

		// Build feature vector
		vector := make([]float64, p.cfg.Dimensions)

		// Apply feature hashing with sign to reduce collisions
		for term, freq := range tf {
			hash1 := p.hashString("pos:" + term)
			hash2 := p.hashString("neg:" + term)
			dim := int(hash1 % uint64(p.cfg.Dimensions))
			sign := 1.0
			if hash2%2 == 0 {
				sign = -1.0
			}
			vector[dim] += sign * float64(freq) * p.estimateIDF(term)
		}

		// Normalize to unit length
		if p.cfg.EnableNormalization {
			vector = embeddings.NormalizeVector(vector)
		}

		results[i] = &embeddings.EmbeddingResult{
			Vector:     vector,
			Model:      p.Model(),
			Dimensions: p.cfg.Dimensions,
			TokensUsed: estimateTokens(text),
		}
	}

	return results, nil
}

// termFrequency computes term frequency for a text.
func (p *Provider) termFrequency(text string) map[string]float64 {
	tf := make(map[string]float64)
	terms := tokenize(text)

	for _, term := range terms {
		tf[term]++
	}

	// Normalize by max frequency
	maxFreq := 0.0
	for _, f := range tf {
		if f > maxFreq {
			maxFreq = f
		}
	}
	if maxFreq > 0 {
		for term, freq := range tf {
			tf[term] = freq / maxFreq
		}
	}

	return tf
}

// estimateIDF estimates the inverse document frequency for a term.
// Uses a global estimate + smoothing.
func (p *Provider) estimateIDF(term string) float64 {
	p.mu.RLock()
	idf, exists := p.idf[term]
	p.mu.RUnlock()

	if exists {
		return idf
	}

	// Estimate IDF based on term length and character distribution
	// Longer, more complex terms tend to be more informative
	length := len(term)
	uniqueChars := 0
	seen := make(map[rune]bool)
	for _, r := range term {
		if !seen[r] {
			seen[r] = true
			uniqueChars++
		}
	}

	// Heuristic: terms with more unique chars and moderate length are more informative
	score := math.Log(1 + float64(uniqueChars))
	score *= math.Log(1 + float64(length))

	// Smooth to avoid zero
	if score < 0.1 {
		score = 0.1
	}

	p.mu.Lock()
	p.idf[term] = score
	p.mu.Unlock()

	return score
}

// hashString computes a 64-bit hash of a string for feature hashing.
func (p *Provider) hashString(s string) uint64 {
	// Hashing must depend only on the feature. A process-global RNG makes the
	// same text produce different vectors after unrelated calls.
	digest := sha256.Sum256([]byte(s))
	var h uint64
	for _, b := range digest[:8] {
		h = h<<8 | uint64(b)
	}
	return h
}

// tokenize splits text into lowercase tokens, removing punctuation.
func tokenize(text string) []string {
	text = strings.ToLower(text)

	// Split on non-alphanumeric characters
	var tokens []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				token := current.String()
				if len(token) >= 2 { // skip very short tokens
					tokens = append(tokens, token)
				}
				current.Reset()
			}
		}
	}

	if current.Len() > 0 {
		token := current.String()
		if len(token) >= 2 {
			tokens = append(tokens, token)
		}
	}

	return tokens
}

// estimateTokens estimates token count (~4 chars per token).
func estimateTokens(text string) int {
	return len(text) / 4
}

// Fit builds a vocabulary from a corpus of texts.
// This improves embedding quality by using corpus-level IDF statistics.
func (p *Provider) Fit(texts []string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	log.Debug().Int("docs", len(texts)).Msg("fitting local embedding model")

	// Reset
	p.vocabulary = make(map[string]int)
	p.termDocFreq = make(map[string]int)
	p.docCount = len(texts)

	// Count document frequency for each term
	for _, text := range texts {
		terms := tokenize(text)
		seen := make(map[string]bool)
		for _, term := range terms {
			if !seen[term] {
				p.termDocFreq[term]++
				seen[term] = true
			}
		}
	}

	// Build vocabulary (sort by frequency, take top MaxFeatures)
	type termFreq struct {
		term string
		freq int
	}
	var sorted []termFreq
	for term, freq := range p.termDocFreq {
		if freq >= p.cfg.MinDocFrequency {
			sorted = append(sorted, termFreq{term, freq})
		}
	}

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].freq > sorted[j].freq
	})

	maxFeatures := p.cfg.MaxFeatures
	if len(sorted) < maxFeatures {
		maxFeatures = len(sorted)
	}

	for i, tf := range sorted[:maxFeatures] {
		p.vocabulary[tf.term] = i
	}

	// Compute IDF for each vocabulary term
	n := float64(p.docCount)
	for term, df := range p.termDocFreq {
		if _, ok := p.vocabulary[term]; ok {
			p.idf[term] = math.Log((n + 1) / (float64(df) + 1))
		}
	}

	log.Debug().
		Int("vocab_size", len(p.vocabulary)).
		Int("total_terms", len(p.termDocFreq)).
		Msg("local embedding model fitted")

	return nil
}

// Reset clears the fitted vocabulary and IDF statistics.
func (p *Provider) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.vocabulary = make(map[string]int)
	p.idf = make(map[string]float64)
	p.termDocFreq = make(map[string]int)
	p.docCount = 0
}

// Factory returns an embeddings.ProviderFactory for the local provider.
func Factory() (string, embeddings.ProviderFactory) {
	return "local", func(_ context.Context, cfg *embeddings.Config) (embeddings.Provider, error) {
		config := DefaultConfig()
		if cfg != nil {
			if cfg.Dimensions > 0 {
				config.Dimensions = cfg.Dimensions
			}
		}
		return New(config)
	}
}

// Register registers this provider with the global embedding provider registry.
func Register() {
	registry := embeddings.GetRegistry()
	name, factory := Factory()
	registry.Register(name, factory, "Local TF-IDF embeddings (zero dependencies)", 100)
}
