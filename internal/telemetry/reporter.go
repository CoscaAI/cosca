package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/CoscaAI/cosca/internal/safe"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// =============================================================================
// Reporter
// =============================================================================

// Reporter handles periodic batched reporting of telemetry events.
// Events are queued locally and sent in batches to a configurable endpoint.
type Reporter struct {
	mu            sync.RWMutex
	logger        zerolog.Logger
	telemetry     *Telemetry
	endpoint      string
	batchSize     int
	flushInterval time.Duration
	client        *http.Client
	apiKey        string

	// Offline queue
	queue     []Event
	queueMu   sync.Mutex
	queueSize int

	// Lifecycle
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool
}

// ReporterConfig configures the telemetry reporter.
type ReporterConfig struct {
	// Endpoint is the URL to send telemetry data to.
	Endpoint string `json:"endpoint" yaml:"endpoint"`
	// BatchSize is the number of events to send in each batch.
	BatchSize int `json:"batch_size" yaml:"batch_size"`
	// FlushInterval is how often to flush the event queue.
	FlushInterval time.Duration `json:"flush_interval" yaml:"flush_interval"`
	// APIKey is an optional API key for the telemetry endpoint.
	APIKey string `json:"api_key" yaml:"api_key"`
	// Timeout is the HTTP request timeout.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

// DefaultReporterConfig returns a default reporter configuration.
func DefaultReporterConfig() ReporterConfig {
	return ReporterConfig{
		Endpoint:      "https://telemetry.cosca.enterprise/v1/events",
		BatchSize:     100,
		FlushInterval: 5 * time.Minute,
		Timeout:       30 * time.Second,
	}
}

// NewReporter creates a new telemetry reporter.
func NewReporter(telemetry *Telemetry, cfg ReporterConfig) *Reporter {
	ctx, cancel := context.WithCancel(context.Background())

	return &Reporter{
		logger:        log.With().Str("component", "telemetry-reporter").Logger(),
		telemetry:     telemetry,
		endpoint:      cfg.Endpoint,
		batchSize:     cfg.BatchSize,
		flushInterval: cfg.FlushInterval,
		apiKey:        cfg.APIKey,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start begins the periodic reporting loop.
func (r *Reporter) Start() {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return
	}
	r.running = true
	r.mu.Unlock()

	r.wg.Add(1)
	go r.reportLoop()

	r.logger.Info().
		Str("endpoint", r.endpoint).
		Int("batch_size", r.batchSize).
		Dur("interval", r.flushInterval).
		Msg("telemetry reporter started")
}

// Stop stops the reporter and flushes remaining events.
func (r *Reporter) Stop() {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return
	}
	r.running = false
	r.mu.Unlock()

	r.cancel()
	r.wg.Wait()

	// Flush remaining events on shutdown
	r.flush()
}

// Enqueue adds an event to the offline queue.
func (r *Reporter) Enqueue(event Event) {
	r.queueMu.Lock()
	defer r.queueMu.Unlock()

	r.queue = append(r.queue, event)
	r.queueSize++

	// Flush immediately if we've reached the batch size
	if r.queueSize >= r.batchSize {
		r.queueMu.Unlock()
		r.flush()
		r.queueMu.Lock()
	}
}

// QueueSize returns the current number of queued events.
func (r *Reporter) QueueSize() int {
	r.queueMu.Lock()
	defer r.queueMu.Unlock()
	return r.queueSize
}

// reportLoop periodically flushes the event queue.
func (r *Reporter) reportLoop() {
	defer r.wg.Done()

	ticker := time.NewTicker(r.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.flush()
		case <-r.ctx.Done():
			return
		}
	}
}

// flush sends all queued events to the telemetry endpoint.
func (r *Reporter) flush() {
	r.queueMu.Lock()
	if len(r.queue) == 0 {
		r.queueMu.Unlock()
		return
	}

	events := r.queue
	r.queue = make([]Event, 0, r.batchSize)
	r.queueSize = 0
	r.queueMu.Unlock()

	if err := r.sendEvents(events); err != nil {
		r.logger.Warn().
			Err(err).
			Int("count", len(events)).
			Msg("failed to send telemetry events, requeueing")

		// Re-queue events that failed to send
		r.queueMu.Lock()
		r.queue = append(events, r.queue...)
		r.queueSize = len(r.queue)
		r.queueMu.Unlock()
		return
	}

	r.logger.Debug().
		Int("count", len(events)).
		Str("endpoint", r.endpoint).
		Msg("telemetry events sent")
}

// sendEvents sends a batch of events to the telemetry endpoint.
func (r *Reporter) sendEvents(events []Event) error {
	if r.endpoint == "" || len(events) == 0 {
		return nil
	}

	// Guard against nil telemetry
	instanceID := "unknown"
	version := "0.0.0"
	if r.telemetry != nil {
		instanceID = r.telemetry.InstanceID()
		version = r.telemetry.Version()
	}

	// Build the payload
	payload := map[string]interface{}{
		"instance_id": instanceID,
		"version":     version,
		"events":      events,
		"sent_at":     time.Now().UTC().Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal telemetry payload: %w", err)
	}

	req, err := http.NewRequestWithContext(r.ctx, http.MethodPost, r.endpoint, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("create telemetry request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "cosca-telemetry/1.0")
	if r.apiKey != "" {
		req.Header.Set("X-API-Key", r.apiKey)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("send telemetry request: %w", err)
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telemetry server returned status %d", resp.StatusCode)
	}

	return nil
}
