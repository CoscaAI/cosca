// Package watcher provides file system watching capabilities.
package watcher

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// EventHandlerFunc is a convenience type for creating handlers from functions.
type EventHandlerFunc func(ctx context.Context, event FileEvent) error

// Handle satisfies the EventHandler interface via a function.
func (f EventHandlerFunc) Handle(ctx context.Context, event FileEvent) error {
	return f(ctx, event)
}

// QueueProcessor manages a queue of file events for batch processing.
type QueueProcessor struct {
	mu        sync.Mutex
	events    []FileEvent
	queueSize int
	interval  time.Duration
	handler   EventHandler
	logger    zerolog.Logger
	ticker    *time.Ticker
	done      chan struct{}
}

// NewQueueProcessor creates a new queue-based event processor.
func NewQueueProcessor(handler EventHandler, interval time.Duration, logger zerolog.Logger) *QueueProcessor {
	return &QueueProcessor{
		events:   make([]FileEvent, 0, 100),
		interval: interval,
		handler:  handler,
		logger:   logger,
		done:     make(chan struct{}),
	}
}

// Start begins the processing loop.
func (qp *QueueProcessor) Start(ctx context.Context) {
	qp.ticker = time.NewTicker(qp.interval)
	go func() {
		defer qp.ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				qp.Flush(ctx)
				close(qp.done)
				return
			case <-qp.ticker.C:
				qp.Flush(ctx)
			}
		}
	}()
}

// Stop stops the processor.
func (qp *QueueProcessor) Stop() {
	close(qp.done)
}

// Enqueue adds an event to the processing queue.
func (qp *QueueProcessor) Enqueue(event FileEvent) {
	qp.mu.Lock()
	defer qp.mu.Unlock()
	qp.events = append(qp.events, event)
	qp.queueSize++
}

// Flush processes all queued events.
func (qp *QueueProcessor) Flush(ctx context.Context) {
	qp.mu.Lock()
	events := qp.events
	qp.events = make([]FileEvent, 0, 100)
	qp.queueSize = 0
	qp.mu.Unlock()

	if len(events) == 0 {
		return
	}

	qp.logger.Debug().Int("count", len(events)).Msg("flushing event queue")

	// Merge duplicate events (keep the latest for each path)
	merged := qp.mergeEvents(events)

	// Process merged events
	for _, event := range merged {
		if err := qp.handler(ctx, event); err != nil {
			qp.logger.Warn().Err(err).Str("path", event.Path).Msg("queue handler error")
		}
	}
}

// mergeEvents merges duplicate events, keeping the most recent for each path.
func (qp *QueueProcessor) mergeEvents(events []FileEvent) []FileEvent {
	latest := make(map[string]FileEvent)
	order := make([]string, 0)

	for _, event := range events {
		key := event.Path
		if _, exists := latest[key]; !exists {
			order = append(order, key)
		}
		latest[key] = event
	}

	result := make([]FileEvent, 0, len(order))
	for _, key := range order {
		result = append(result, latest[key])
	}

	return result
}

// QueueSize returns the current number of queued events.
func (qp *QueueProcessor) QueueSize() int {
	qp.mu.Lock()
	defer qp.mu.Unlock()
	return qp.queueSize
}

// OnCreateHandler returns a handler that processes file creation events.
func OnCreateHandler(indexer func(ctx context.Context, path string) error) EventHandler {
	return func(ctx context.Context, event FileEvent) error {
		if event.IsDir {
			return nil // Skip directories for indexing
		}
		return indexer(ctx, event.Path)
	}
}

// OnModifyHandler returns a handler that processes file modification events.
func OnModifyHandler(reindexer func(ctx context.Context, path string) error) EventHandler {
	return func(ctx context.Context, event FileEvent) error {
		if event.IsDir {
			return nil
		}
		return reindexer(ctx, event.Path)
	}
}

// OnDeleteHandler returns a handler that processes file deletion events.
func OnDeleteHandler(remover func(ctx context.Context, path string) error) EventHandler {
	return func(ctx context.Context, event FileEvent) error {
		return remover(ctx, event.Path)
	}
}

// OnRenameHandler returns a handler that processes file rename events.
func OnRenameHandler(updater func(ctx context.Context, oldPath, newPath string) error) EventHandler {
	return func(ctx context.Context, event FileEvent) error {
		if event.OldPath != "" {
			return updater(ctx, event.OldPath, event.Path)
		}
		return nil
	}
}

// HandlerChain combines multiple event handlers into one.
type HandlerChain struct {
	handlers []EventHandler
	logger   zerolog.Logger
}

// NewHandlerChain creates a chain of event handlers.
func NewHandlerChain(handlers ...EventHandler) *HandlerChain {
	return &HandlerChain{
		handlers: handlers,
		logger:   zerolog.Nop(),
	}
}

// Handle processes an event through the chain.
func (hc *HandlerChain) Handle(ctx context.Context, event FileEvent) error {
	for _, handler := range hc.handlers {
		if err := handler(ctx, event); err != nil {
			hc.logger.Warn().
				Err(err).
				Str("type", string(event.Type)).
				Str("path", event.Path).
				Msg("handler chain error")
			return err
		}
	}
	return nil
}

// WithLogger sets the logger for the handler chain.
func (hc *HandlerChain) WithLogger(logger zerolog.Logger) *HandlerChain {
	hc.logger = logger
	return hc
}

// BatchHandler creates an event handler that batch-processes multiple events.
func BatchHandler(batchSize int, timeout time.Duration, processor func(ctx context.Context, events []FileEvent) error) EventHandler {
	var mu sync.Mutex
	batch := make([]FileEvent, 0, batchSize)
	timer := time.NewTimer(timeout)
	timer.Stop()

	return func(ctx context.Context, event FileEvent) error {
		mu.Lock()
		batch = append(batch, event)

		if len(batch) >= batchSize {
			events := batch
			batch = make([]FileEvent, 0, batchSize)
			timer.Stop()
			mu.Unlock()

			return processor(ctx, events)
		}

		timer.Reset(timeout)
		mu.Unlock()

		// Wait for batch to fill or timeout
		select {
		case <-timer.C:
			mu.Lock()
			events := batch
			batch = make([]FileEvent, 0, batchSize)
			mu.Unlock()
			return processor(ctx, events)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// FilterHandler creates a handler that only processes events matching a filter.
func FilterHandler(filter func(event FileEvent) bool, next EventHandler) EventHandler {
	return func(ctx context.Context, event FileEvent) error {
		if filter(event) {
			return next(ctx, event)
		}
		return nil
	}
}

// LoggingHandler creates a handler that logs events before passing them on.
func LoggingHandler(logger zerolog.Logger, next EventHandler) EventHandler {
	return func(ctx context.Context, event FileEvent) error {
		logger.Debug().
			Str("type", string(event.Type)).
			Str("path", event.Path).
			Bool("is_dir", event.IsDir).
			Int64("size", event.Size).
			Msg("file event")
		return next(ctx, event)
	}
}
