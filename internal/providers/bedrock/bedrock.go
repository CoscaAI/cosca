// Package bedrock provides embedding generation via AWS Bedrock Titan Embeddings API.
package bedrock

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/embeddings"
	"github.com/CoscaAI/cosca/internal/providers"
	"github.com/CoscaAI/cosca/internal/safe"
	"github.com/rs/zerolog/log"
)

// Default model and dimensions.
const (
	DefaultModel  = "amazon.titan-embed-text-v2:0"
	DefaultDim    = 1024
	DefaultRegion = "us-east-1"
	ServiceName   = "bedrock"
)

// Config defines configuration for the AWS Bedrock embedding provider.
type Config struct {
	// AccessKeyID is the AWS access key ID.
	AccessKeyID string

	// SecretAccessKey is the AWS secret access key.
	SecretAccessKey string

	// Region is the AWS region.
	Region string

	// Model is the embedding model ID.
	Model string

	// Dimensions is the output dimensionality.
	Dimensions int

	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int

	// Timeout is the HTTP request timeout.
	Timeout time.Duration

	// BatchSize is the maximum items per batch request.
	BatchSize int
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() Config {
	return Config{
		AccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
		Region:          getEnvOrDefault("AWS_REGION", DefaultRegion),
		Model:           DefaultModel,
		Dimensions:      DefaultDim,
		MaxRetries:      3,
		Timeout:         120 * time.Second, // Bedrock can be slower
		BatchSize:       10,                // Titan supports batching
	}
}

// Provider implements the embeddings.Provider interface for AWS Bedrock.
type Provider struct {
	cfg         Config
	client      *http.Client
	rateLimiter *providers.RateLimiter
	modelDims   int
}

// rateLimiter implements a simple token bucket rate limiter.

// New creates a new AWS Bedrock embedding provider.
func New(cfg Config) (*Provider, error) {
	if cfg.AccessKeyID == "" {
		cfg.AccessKeyID = os.Getenv("AWS_ACCESS_KEY_ID")
	}
	if cfg.AccessKeyID == "" {
		return nil, fmt.Errorf("AWS_ACCESS_KEY_ID not set")
	}
	if cfg.SecretAccessKey == "" {
		cfg.SecretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	}
	if cfg.SecretAccessKey == "" {
		return nil, fmt.Errorf("AWS_SECRET_ACCESS_KEY not set")
	}
	if cfg.Region == "" {
		cfg.Region = getEnvOrDefault("AWS_REGION", DefaultRegion)
	}
	if cfg.Model == "" {
		cfg.Model = DefaultModel
	}
	if cfg.Dimensions <= 0 {
		cfg.Dimensions = DefaultDim
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 120 * time.Second
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 10
	}

	p := &Provider{
		cfg:         cfg,
		client:      providers.SharedHTTPClient(cfg.Timeout),
		rateLimiter: providers.NewRateLimiter(500000), // 500k TPM for Bedrock
		modelDims:   cfg.Dimensions,
	}

	log.Debug().
		Str("model", cfg.Model).
		Str("region", cfg.Region).
		Int("dims", p.modelDims).
		Msg("bedrock embedding provider initialized")

	return p, nil
}

// Name returns the provider name.
func (p *Provider) Name() string { return "bedrock" }

// Model returns the model ID.
func (p *Provider) Model() string { return p.cfg.Model }

// Dimensions returns the embedding dimensionality.
func (p *Provider) Dimensions() int { return p.modelDims }

// Close cleans up provider resources.
func (p *Provider) Close() error { return nil }

// GenerateEmbedding generates an embedding for a single text string.
func (p *Provider) GenerateEmbedding(ctx context.Context, text string) (*embeddings.EmbeddingResult, error) {
	results, err := p.GenerateEmbeddings(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no embedding result returned")
	}
	return results[0], nil
}

// GenerateEmbeddings generates embeddings for a batch of texts.
func (p *Provider) GenerateEmbeddings(ctx context.Context, texts []string) ([]*embeddings.EmbeddingResult, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("no texts provided")
	}

	// Process in batches
	var allResults []*embeddings.EmbeddingResult
	for i := 0; i < len(texts); i += p.cfg.BatchSize {
		end := i + p.cfg.BatchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		batchResults, err := p.embedBatch(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("batch %d: %w", i/p.cfg.BatchSize, err)
		}
		allResults = append(allResults, batchResults...)
	}

	return allResults, nil
}

// embedBatch sends a batch of texts to the Bedrock API.
// For Titan Embeddings v2, we make individual requests since the API accepts
// one input text per request (v2 supports batching via the model's native API,
// but the invokeModel endpoint is single-input).
func (p *Provider) embedBatch(ctx context.Context, texts []string) ([]*embeddings.EmbeddingResult, error) {
	results := make([]*embeddings.EmbeddingResult, 0, len(texts))

	for _, text := range texts {
		result, err := p.embedOne(ctx, text)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	return results, nil
}

// embedOne sends a single embedding request to the Bedrock API with AWS SigV4.
func (p *Provider) embedOne(ctx context.Context, text string) (*embeddings.EmbeddingResult, error) {
	// Apply rate limiting
	tokenEstimate := len(text) / 4
	if err := p.rateLimiter.Wait(ctx, tokenEstimate); err != nil {
		return nil, fmt.Errorf("rate limit wait: %w", err)
	}

	// Build Titan Embeddings v2 request body
	requestBody := map[string]interface{}{
		"inputText":  text,
		"dimensions": p.modelDims,
		"normalize":  true,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Build endpoint: https://bedrock-runtime.{region}.amazonaws.com/model/{modelId}/invoke
	endpoint := fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com/model/%s/invoke",
		p.cfg.Region, p.cfg.Model)

	// Send request with retry
	var lastErr error
	for attempt := 0; attempt <= p.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * 2 * time.Second
			log.Debug().Int("attempt", attempt+1).Dur("backoff", backoff).Msg("retrying bedrock embedding request")
			timer := time.NewTimer(backoff)
			select {
			case <-timer.C:
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return nil, ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")

		// Sign request with AWS Signature V4
		if err := signRequest(req, body, p.cfg.AccessKeyID, p.cfg.SecretAccessKey, p.cfg.Region, ServiceName); err != nil {
			lastErr = fmt.Errorf("sign request: %w", err)
			continue
		}

		resp, err := p.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("http request: %w", err)
			continue
		}

		respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
		safe.Close(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("read response: %w", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = handleAPIError(resp.StatusCode, string(respBody))
			if isBadRequest(lastErr) || resp.StatusCode == http.StatusForbidden {
				return nil, lastErr
			}
			continue
		}

		var apiResponse struct {
			Embedding           []float64 `json:"embedding"`
			InputTextTokenCount int       `json:"inputTextTokenCount"`
		}

		if err := json.Unmarshal(respBody, &apiResponse); err != nil {
			lastErr = fmt.Errorf("parse response: %w", err)
			continue
		}

		if len(apiResponse.Embedding) == 0 {
			return nil, fmt.Errorf("empty embedding vector in response")
		}

		// Truncate or pad to configured dimensions
		vector := apiResponse.Embedding
		if len(vector) > p.modelDims {
			vector = vector[:p.modelDims]
		} else if len(vector) < p.modelDims {
			padded := make([]float64, p.modelDims)
			copy(padded, vector)
			vector = padded
		}

		result := &embeddings.EmbeddingResult{
			Vector:     vector,
			Model:      p.cfg.Model,
			Dimensions: len(vector),
			TokensUsed: apiResponse.InputTextTokenCount,
		}

		return result, nil
	}

	return nil, fmt.Errorf("bedrock embedding failed after %d retries: %w", p.cfg.MaxRetries, lastErr)
}

// ─── AWS Signature V4 ────────────────────────────────────────────────────────

// signRequest signs an HTTP request with AWS Signature V4.
func signRequest(req *http.Request, body []byte, accessKeyID, secretAccessKey, region, service string) error {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")

	// Set AWS headers
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("Host", req.URL.Host)

	// Compute payload hash
	payloadHash := sha256Hex(body)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	// Gather signed headers
	var headerNames []string
	headerNames = make([]string, 0, len(req.Header))
	for name := range req.Header {
		lowerName := strings.ToLower(name)
		// Skip headers that start with "x-amz-" for the canonical request,
		// but include them in the signature scope
		headerNames = append(headerNames, lowerName)
	}
	sort.Strings(headerNames)

	var signedHeaders strings.Builder
	for i, name := range headerNames {
		if i > 0 {
			signedHeaders.WriteByte(';')
		}
		signedHeaders.WriteString(name)
	}
	signedHeadersStr := signedHeaders.String()

	// Build canonical request
	canonicalURI := req.URL.Path
	if canonicalURI == "" {
		canonicalURI = "/"
	}

	canonicalQueryString := req.URL.RawQuery

	var canonicalHeaders strings.Builder
	for _, name := range headerNames {
		canonicalHeaders.WriteString(name)
		canonicalHeaders.WriteByte(':')
		canonicalHeaders.WriteString(strings.TrimSpace(req.Header.Get(name)))
		canonicalHeaders.WriteByte('\n')
	}

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		req.Method,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders.String(),
		signedHeadersStr,
		payloadHash,
	)

	canonicalRequestHash := sha256Hex([]byte(canonicalRequest))

	// Build string to sign
	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStamp, region, service)
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate,
		credentialScope,
		canonicalRequestHash,
	)

	// Compute signing key
	signingKey := getSignatureKey(secretAccessKey, dateStamp, region, service)
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	// Build authorization header
	authorization := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKeyID, credentialScope, signedHeadersStr, signature)

	req.Header.Set("Authorization", authorization)

	return nil
}

// getSignatureKey derives the AWS Signature V4 signing key.
func getSignatureKey(secret, dateStamp, region, service string) []byte {
	kSecret := []byte("AWS4" + secret)
	kDate := hmacSHA256(kSecret, []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	return kSigning
}

// hmacSHA256 computes HMAC-SHA256 of data using the given key.
func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

// sha256Hex computes the SHA-256 hex digest of data.
func sha256Hex(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// ─── Error Handling ──────────────────────────────────────────────────────────

// handleAPIError translates HTTP status codes into meaningful errors.
func handleAPIError(statusCode int, body string) error {
	switch statusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("bedrock: invalid credentials (401)")
	case http.StatusForbidden:
		return fmt.Errorf("bedrock: access denied (403): %s", providers.TruncateBody(body))
	case http.StatusTooManyRequests:
		return fmt.Errorf("bedrock: rate limited (429): %s", providers.TruncateBody(body))
	case http.StatusServiceUnavailable:
		return fmt.Errorf("bedrock: service unavailable (503)")
	case http.StatusBadRequest:
		return fmt.Errorf("bedrock: bad request (400): %s", providers.TruncateBody(body))
	default:
		if statusCode >= 500 {
			return fmt.Errorf("bedrock: server error (%d): %s", statusCode, providers.TruncateBody(body))
		}
		return fmt.Errorf("bedrock: unexpected status %d: %s", statusCode, providers.TruncateBody(body))
	}
}

// isBadRequest returns true for non-retryable errors.
func isBadRequest(err error) bool {
	return strings.Contains(err.Error(), "(400)") ||
		strings.Contains(err.Error(), "(401)") ||
		strings.Contains(err.Error(), "(403)")
}

// truncateBody truncates error response bodies for logging.

// getEnvOrDefault returns the environment variable or a default value.
func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// Factory returns an embeddings.ProviderFactory for the Bedrock provider.
func Factory() (string, embeddings.ProviderFactory) {
	return "bedrock", func(_ context.Context, cfg *embeddings.Config) (embeddings.Provider, error) {
		config := DefaultConfig()
		if cfg != nil {
			if cfg.AccessKeyID != "" {
				config.AccessKeyID = cfg.AccessKeyID
			}
			if cfg.SecretAccessKey != "" {
				config.SecretAccessKey = cfg.SecretAccessKey
			}
			if cfg.Region != "" {
				config.Region = cfg.Region
			}
			if cfg.Model != "" {
				config.Model = cfg.Model
			}
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
	registry.Register(name, factory, "AWS Bedrock Titan Embeddings API", 60)
}
