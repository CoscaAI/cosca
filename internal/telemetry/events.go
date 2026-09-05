// Package telemetry provides event type definitions for the Cosca
// telemetry system. It defines all standard event types and their
// associated metadata.
package telemetry

import (
	"strconv"
	"time"
)

// =============================================================================
// Event Type Constants
// =============================================================================

// EventType identifies the category of a telemetry event.
type EventType string

const (
	// EventCommandExecuted is emitted when a CLI command is executed.
	EventCommandExecuted EventType = "command_executed"
	// EventIndexCompleted is emitted when an index operation completes.
	EventIndexCompleted EventType = "index_completed"
	// EventSearchPerformed is emitted when a search is performed.
	EventSearchPerformed EventType = "search_performed"
	// EventContextBuilt is emitted when a context is built.
	EventContextBuilt EventType = "context_built"
	// EventMemoryStored is emitted when a memory record is stored.
	EventMemoryStored EventType = "memory_stored"
	// EventPluginInstalled is emitted when a plugin is installed.
	EventPluginInstalled EventType = "plugin_installed"
	// EventErrorOccurred is emitted when an error occurs.
	EventErrorOccurred EventType = "error_occurred"
	// EventRuntimeStarted is emitted when the runtime starts.
	EventRuntimeStarted EventType = "runtime_started"
	// EventRuntimeStopped is emitted when the runtime stops.
	EventRuntimeStopped EventType = "runtime_stopped"
	// EventProviderCalled is emitted when an AI provider is called.
	EventProviderCalled EventType = "provider_called"
	// EventConfigChanged is emitted when configuration is changed.
	EventConfigChanged EventType = "config_changed"
	// EventUpdateChecked is emitted when an update check occurs.
	EventUpdateChecked EventType = "update_checked"
	// EventSyncCompleted is emitted when a sync operation completes.
	EventSyncCompleted EventType = "sync_completed"
)

// =============================================================================
// Event
// =============================================================================

// Event represents a single telemetry event.
type Event struct {
	// ID is a unique identifier for the event.
	ID string `json:"id" yaml:"id"`
	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp" yaml:"timestamp"`
	// Type is the category of event.
	Type EventType `json:"type" yaml:"type"`
	// Duration is how long the operation took, if applicable.
	DurationMs int64 `json:"duration_ms,omitempty" yaml:"duration_ms,omitempty"`
	// Success indicates whether the operation succeeded.
	Success bool `json:"success" yaml:"success"`
	// Metadata holds event-specific data (no PII).
	Metadata map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// =============================================================================
// Event Constructors
// =============================================================================

// NewCommandExecutedEvent creates an event for a CLI command execution.
func NewCommandExecutedEvent(command string, durationMs int64, success bool) Event {
	return Event{
		Type:       EventCommandExecuted,
		Timestamp:  time.Now(),
		DurationMs: durationMs,
		Success:    success,
		Metadata: map[string]string{
			"command": command,
		},
	}
}

// NewIndexCompletedEvent creates an event for an index operation.
func NewIndexCompletedEvent(source string, filesCount int, durationMs int64, success bool) Event {
	meta := map[string]string{
		"source":      source,
		"files_count": fmtInt(filesCount),
	}
	return Event{
		Type:       EventIndexCompleted,
		Timestamp:  time.Now(),
		DurationMs: durationMs,
		Success:    success,
		Metadata:   meta,
	}
}

// NewSearchPerformedEvent creates an event for a search operation.
func NewSearchPerformedEvent(queryType string, resultCount int, durationMs int64, success bool) Event {
	meta := map[string]string{
		"query_type":   queryType,
		"result_count": fmtInt(resultCount),
	}
	return Event{
		Type:       EventSearchPerformed,
		Timestamp:  time.Now(),
		DurationMs: durationMs,
		Success:    success,
		Metadata:   meta,
	}
}

// NewContextBuiltEvent creates an event for a context build.
func NewContextBuiltEvent(contextType string, tokens int, durationMs int64, success bool) Event {
	meta := map[string]string{
		"context_type": contextType,
		"tokens":       fmtInt(tokens),
	}
	return Event{
		Type:       EventContextBuilt,
		Timestamp:  time.Now(),
		DurationMs: durationMs,
		Success:    success,
		Metadata:   meta,
	}
}

// NewMemoryStoredEvent creates an event for a memory store operation.
func NewMemoryStoredEvent(layer string, memoryType string, success bool) Event {
	meta := map[string]string{
		"layer":       layer,
		"memory_type": memoryType,
	}
	return Event{
		Type:      EventMemoryStored,
		Timestamp: time.Now(),
		Success:   success,
		Metadata:  meta,
	}
}

// NewPluginInstalledEvent creates an event for a plugin installation.
func NewPluginInstalledEvent(pluginName string, version string, success bool) Event {
	meta := map[string]string{
		"plugin":  pluginName,
		"version": version,
	}
	return Event{
		Type:      EventPluginInstalled,
		Timestamp: time.Now(),
		Success:   success,
		Metadata:  meta,
	}
}

// NewErrorOccurredEvent creates an event for an error.
func NewErrorOccurredEvent(operation string, errorCode string) Event {
	meta := map[string]string{
		"operation":  operation,
		"error_code": errorCode,
	}
	return Event{
		Type:      EventErrorOccurred,
		Timestamp: time.Now(),
		Success:   false,
		Metadata:  meta,
	}
}

// NewRuntimeStartedEvent creates an event for runtime startup.
func NewRuntimeStartedEvent(version string, mode string) Event {
	meta := map[string]string{
		"version": version,
		"mode":    mode,
	}
	return Event{
		Type:      EventRuntimeStarted,
		Timestamp: time.Now(),
		Success:   true,
		Metadata:  meta,
	}
}

// NewRuntimeStoppedEvent creates an event for runtime shutdown.
func NewRuntimeStoppedEvent(uptimeSeconds int64, reason string) Event {
	meta := map[string]string{
		"uptime_seconds": fmtInt(int(uptimeSeconds)),
		"reason":         reason,
	}
	return Event{
		Type:      EventRuntimeStopped,
		Timestamp: time.Now(),
		Success:   true,
		Metadata:  meta,
	}
}

// NewProviderCalledEvent creates an event for an AI provider call.
func NewProviderCalledEvent(provider string, model string, durationMs int64, tokens int, success bool) Event {
	meta := map[string]string{
		"provider": provider,
		"model":    model,
		"tokens":   fmtInt(tokens),
	}
	return Event{
		Type:       EventProviderCalled,
		Timestamp:  time.Now(),
		DurationMs: durationMs,
		Success:    success,
		Metadata:   meta,
	}
}

// NewConfigChangedEvent creates an event for a config change.
func NewConfigChangedEvent(key string, source string) Event {
	meta := map[string]string{
		"key":    key,
		"source": source,
	}
	return Event{
		Type:      EventConfigChanged,
		Timestamp: time.Now(),
		Success:   true,
		Metadata:  meta,
	}
}

// NewUpdateCheckedEvent creates an event for an update check.
func NewUpdateCheckedEvent(currentVersion string, latestVersion string, updateAvailable bool) Event {
	meta := map[string]string{
		"current_version":  currentVersion,
		"latest_version":   latestVersion,
		"update_available": fmtBool(updateAvailable),
	}
	return Event{
		Type:      EventUpdateChecked,
		Timestamp: time.Now(),
		Success:   true,
		Metadata:  meta,
	}
}

// NewSyncCompletedEvent creates an event for a sync operation.
func NewSyncCompletedEvent(syncType string, items int, durationMs int64, success bool) Event {
	meta := map[string]string{
		"sync_type": syncType,
		"items":     fmtInt(items),
	}
	return Event{
		Type:       EventSyncCompleted,
		Timestamp:  time.Now(),
		DurationMs: durationMs,
		Success:    success,
		Metadata:   meta,
	}
}

// =============================================================================
// Helpers
// =============================================================================

func fmtInt(v int) string {
	return strconv.Itoa(v)
}

func fmtBool(v bool) string {
	return strconv.FormatBool(v)
}
