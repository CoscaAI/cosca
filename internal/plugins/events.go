// Package plugins provides the plugin system for extending Cosca.
// It defines event types, the plugin manager lifecycle, and the plugin
// manifest schema used for discovery and validation.
package plugins

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// =============================================================================
// Event Types
// =============================================================================

// Predefined event type constants used throughout the Cosca platform.
const (
	// EventIndexStarted is published when indexing begins.
	EventIndexStarted = "index.started"
	// EventIndexCompleted is published when indexing finishes.
	EventIndexCompleted = "index.completed"
	// EventIndexError is published when indexing encounters an error.
	EventIndexError = "index.error"

	// EventSearchStarted is published when a search begins.
	EventSearchStarted = "search.started"
	// EventSearchCompleted is published when a search completes.
	EventSearchCompleted = "search.completed"
	// EventSearchError is published when a search fails.
	EventSearchError = "search.error"

	// EventContextBuilt is published when context is built.
	EventContextBuilt = "context.built"

	// EventMemoryStored is published when memory is persisted.
	EventMemoryStored = "memory.stored"
	// EventMemoryRetrieved is published when memory is retrieved.
	EventMemoryRetrieved = "memory.retrieved"

	// EventPluginStarted is published when a plugin starts.
	EventPluginStarted = "plugin.started"
	// EventPluginStopped is published when a plugin stops.
	EventPluginStopped = "plugin.stopped"
	// EventPluginError is published when a plugin encounters an error.
	EventPluginError = "plugin.error"

	// EventError is a generic error event.
	EventError = "system.error"

	// EventConfigChanged is published when configuration changes.
	EventConfigChanged = "config.changed"

	// EventSystemShutdown is published when the system is shutting down.
	EventSystemShutdown = "system.shutdown"
)

// =============================================================================
// Event
// =============================================================================

// Event represents a notification in the event system.
type Event struct {
	// ID is a unique identifier for this event instance.
	ID string `json:"id"`
	// Type is the event type identifier (e.g., "index.started").
	Type string `json:"type"`
	// Source is the ID of the component/plugin that published the event.
	Source string `json:"source"`
	// Timestamp is when the event was published.
	Timestamp time.Time `json:"timestamp"`
	// Data holds the event payload.
	Data interface{} `json:"data,omitempty"`
	// Metadata holds additional contextual key-value pairs.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// NewEvent creates a new event with the given type, source, and data.
func NewEvent(eventType, source string, data interface{}) Event {
	return Event{
		ID:        uuid.New().String(),
		Type:      eventType,
		Source:    source,
		Timestamp: time.Now(),
		Data:      data,
		Metadata:  make(map[string]string),
	}
}

// String returns a summary of the event.
func (e Event) String() string {
	return fmt.Sprintf("Event[%s] type=%s source=%s", e.ID, e.Type, e.Source)
}

// =============================================================================
// EventHandler
// =============================================================================

// EventHandler is a function that processes an event.
type EventHandler func(event Event)

// subscriber represents a registered event subscriber.
type subscriber struct {
	id       string
	pluginID string
	handler  EventHandler
	filter   EventFilter
}

// EventFilter allows subscribers to filter which events they receive.
type EventFilter struct {
	// Types restricts to specific event types (empty = all).
	Types []string
	// Sources restricts to specific sources (empty = all).
	Sources []string
}

// matches checks whether an event matches the subscriber's filter.
func (f EventFilter) matches(event Event) bool {
	// Filter by type
	if len(f.Types) > 0 {
		found := false
		for _, t := range f.Types {
			if t == event.Type {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by source
	if len(f.Sources) > 0 {
		found := false
		for _, s := range f.Sources {
			if s == event.Source {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// =============================================================================
// EventBus
// =============================================================================

// EventBus is a thread-safe publish/subscribe event bus.
// It supports topic-based routing, asynchronous delivery, and
// subscriber filtering.
type EventBus struct {
	mu sync.RWMutex

	// subscribers maps subscriber ID -> subscriber
	subscribers map[string]*subscriber

	// nextID is a counter for unique subscriber IDs.
	nextID int

	// bufferSize is the channel buffer size for async delivery.
	bufferSize int

	// stopped indicates whether the event bus has been stopped.
	stopped bool
}

// NewEventBus creates a new event bus with the given buffer size.
func NewEventBus(bufferSize int) *EventBus {
	if bufferSize <= 0 {
		bufferSize = 100
	}

	return &EventBus{
		subscribers: make(map[string]*subscriber),
		bufferSize:  bufferSize,
	}
}

// Subscribe registers a handler for events matching the given type(s).
// The returned subscription ID can be used to unsubscribe.
func (b *EventBus) Subscribe(eventTypes []string, pluginID string, handler EventHandler) (string, error) {
	if handler == nil {
		return "", fmt.Errorf("event handler cannot be nil")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.stopped {
		return "", fmt.Errorf("event bus is stopped")
	}

	b.nextID++
	subID := fmt.Sprintf("sub_%s_%d", pluginID, b.nextID)

	b.subscribers[subID] = &subscriber{
		id:       subID,
		pluginID: pluginID,
		handler:  handler,
		filter: EventFilter{
			Types: eventTypes,
		},
	}

	log.Debug().Str("subscriber_id", subID).
		Str("plugin", pluginID).
		Strs("types", eventTypes).
		Msg("event subscriber registered")

	return subID, nil
}

// SubscribeWithFilter registers a handler with a custom filter.
func (b *EventBus) SubscribeWithFilter(filter EventFilter, pluginID string, handler EventHandler) (string, error) {
	if handler == nil {
		return "", fmt.Errorf("event handler cannot be nil")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.stopped {
		return "", fmt.Errorf("event bus is stopped")
	}

	b.nextID++
	subID := fmt.Sprintf("sub_%s_%d", pluginID, b.nextID)

	b.subscribers[subID] = &subscriber{
		id:       subID,
		pluginID: pluginID,
		handler:  handler,
		filter:   filter,
	}

	log.Debug().Str("subscriber_id", subID).
		Str("plugin", pluginID).
		Strs("types", filter.Types).
		Strs("sources", filter.Sources).
		Msg("event subscriber registered with filter")

	return subID, nil
}

// Unsubscribe removes a previously registered subscriber.
func (b *EventBus) Unsubscribe(subscriberID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, exists := b.subscribers[subscriberID]; !exists {
		return fmt.Errorf("subscriber %q not found", subscriberID)
	}

	delete(b.subscribers, subscriberID)
	log.Debug().Str("subscriber_id", subscriberID).Msg("event subscriber unregistered")
	return nil
}

// Publish publishes an event to all matching subscribers.
// Delivery is asynchronous; handlers are called in separate goroutines.
func (b *EventBus) Publish(event Event) {
	b.mu.RLock()
	if b.stopped {
		b.mu.RUnlock()
		return
	}

	// Collect matching subscribers
	matching := make([]*subscriber, 0)
	for _, sub := range b.subscribers {
		if sub.filter.matches(event) {
			matching = append(matching, sub)
		}
	}
	b.mu.RUnlock()

	if len(matching) == 0 {
		return
	}

	log.Debug().Str("event_type", event.Type).
		Str("source", event.Source).
		Int("subscribers", len(matching)).
		Msg("publishing event")

	// Deliver asynchronously
	for _, sub := range matching {
		sub := sub // capture for closure
		go func() {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error().
						Str("subscriber_id", sub.id).
						Str("plugin", sub.pluginID).
						Str("event_type", event.Type).
						Interface("panic", rec).
						Msg("event handler panicked — isolated to prevent cascading failure")
				}
			}()
			sub.handler(event)
		}()
	}
}

// PublishSync publishes an event synchronously to all matching subscribers.
// This is useful for critical events where ordering matters.
func (b *EventBus) PublishSync(event Event) {
	b.mu.RLock()
	if b.stopped {
		b.mu.RUnlock()
		return
	}

	matching := make([]*subscriber, 0)
	for _, sub := range b.subscribers {
		if sub.filter.matches(event) {
			matching = append(matching, sub)
		}
	}
	b.mu.RUnlock()

	for _, sub := range matching {
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error().
						Str("subscriber_id", sub.id).
						Str("plugin", sub.pluginID).
						Interface("panic", rec).
						Msg("event handler panicked (sync)")
				}
			}()
			sub.handler(event)
		}()
	}
}

// SubscriberCount returns the number of registered subscribers.
func (b *EventBus) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}

// ClearPluginSubscribers removes all subscribers registered by a plugin.
func (b *EventBus) ClearPluginSubscribers(pluginID string) int {
	b.mu.Lock()
	defer b.mu.Unlock()

	count := 0
	for id, sub := range b.subscribers {
		if sub.pluginID == pluginID {
			delete(b.subscribers, id)
			count++
		}
	}

	if count > 0 {
		log.Debug().Str("plugin", pluginID).Int("count", count).
			Msg("event subscribers cleared for plugin")
	}

	return count
}

// Stop gracefully shuts down the event bus, preventing new subscriptions
// and publications.
func (b *EventBus) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.stopped = true
	b.subscribers = make(map[string]*subscriber)
	log.Info().Msg("event bus stopped")
}

// =============================================================================
// Default Event Bus
// =============================================================================

var (
	defaultEventBus     *EventBus
	defaultEventBusOnce sync.Once
)

// DefaultEventBus returns the package-level default event bus.
func DefaultEventBus() *EventBus {
	defaultEventBusOnce.Do(func() {
		defaultEventBus = NewEventBus(200)
	})
	return defaultEventBus
}

// PublishGlobalEvent publishes an event to the default event bus.
func PublishGlobalEvent(eventType, source string, data interface{}) {
	event := NewEvent(eventType, source, data)
	DefaultEventBus().Publish(event)
}

// SubscribeGlobal subscribes to events on the default event bus.
func SubscribeGlobal(eventTypes []string, pluginID string, handler EventHandler) (string, error) {
	return DefaultEventBus().Subscribe(eventTypes, pluginID, handler)
}
