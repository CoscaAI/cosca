package watcher

import (
	"context"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog"
)

// EventType represents the type of file system event.
type EventType string

const (
	// EventCreate is emitted when a file or directory is created.
	EventCreate EventType = "create"
	// EventModify is emitted when a file or directory is modified.
	EventModify EventType = "modify"
	// EventDelete is emitted when a file or directory is deleted.
	EventDelete EventType = "delete"
	// EventRename is emitted when a file or directory is renamed.
	EventRename EventType = "rename"
	// EventChmod is emitted when file permissions change.
	EventChmod EventType = "chmod"
)

// FileEvent represents a file system change event.
type FileEvent struct {
	Type      EventType `json:"type"`
	Path      string    `json:"path"`
	OldPath   string    `json:"old_path,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	IsDir     bool      `json:"is_dir"`
	Size      int64     `json:"size,omitempty"`
}

// EventHandler is a function that processes file events.
type EventHandler func(ctx context.Context, event FileEvent) error

// Watcher monitors the file system for changes.
type Watcher struct {
	mu              sync.RWMutex
	fsWatcher       *fsnotify.Watcher
	logger          zerolog.Logger
	handlers        map[EventType][]EventHandler
	debounce        time.Duration
	pending         map[string]*debounceEntry
	debounceMu      sync.Mutex
	ignoreDirs      []string
	ignoreFiles     []string
	extensions      []string
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	watchCounter    int
	eventBufSize    int
	shutdownTimeout time.Duration
}

// debounceEntry represents a debounced event.
type debounceEntry struct {
	event     FileEvent
	timer     *time.Timer
	createdAt time.Time
}

// Option configures the watcher.
type Option func(*Watcher)

// WithLogger sets the logger for the watcher.
func WithLogger(logger zerolog.Logger) Option {
	return func(w *Watcher) {
		w.logger = logger
	}
}

// WithDebounce sets the debounce interval for events.
func WithDebounce(d time.Duration) Option {
	return func(w *Watcher) {
		w.debounce = d
	}
}

// WithIgnoreDirs sets directory patterns to ignore.
func WithIgnoreDirs(dirs ...string) Option {
	return func(w *Watcher) {
		w.ignoreDirs = append(w.ignoreDirs, dirs...)
	}
}

// WithIgnoreFiles sets file patterns to ignore.
func WithIgnoreFiles(files ...string) Option {
	return func(w *Watcher) {
		w.ignoreFiles = append(w.ignoreFiles, files...)
	}
}

// WithExtensions sets the file extensions to watch (empty means all).
func WithExtensions(exts ...string) Option {
	return func(w *Watcher) {
		w.extensions = exts
	}
}

// WithEventBufferSize sets the size of the event buffer.
func WithEventBufferSize(size int) Option {
	return func(w *Watcher) {
		w.eventBufSize = size
	}
}

// WithShutdownTimeout bounds the wait for watcher workers during Close.
func WithShutdownTimeout(timeout time.Duration) Option {
	return func(w *Watcher) {
		w.shutdownTimeout = timeout
	}
}

// New creates a new file watcher.
func New(opts ...Option) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	w := &Watcher{
		fsWatcher:       fsWatcher,
		logger:          zerolog.Nop(),
		handlers:        make(map[EventType][]EventHandler),
		debounce:        500 * time.Millisecond,
		pending:         make(map[string]*debounceEntry),
		ignoreDirs:      []string{"node_modules", ".git", "dist", ".cosca", ".next", "build", "target", "__pycache__", ".cache", ".venv", "vendor"},
		ignoreFiles:     []string{".DS_Store", "Thumbs.db", "*.swp", "*.swx", "*.bak", "~"},
		ctx:             ctx,
		cancel:          cancel,
		eventBufSize:    100,
		shutdownTimeout: 10 * time.Second,
	}

	for _, opt := range opts {
		opt(w)
	}

	return w, nil
}

// Watch starts watching a directory tree recursively.
func (w *Watcher) Watch(root string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}

	w.logger.Info().Str("root", root).Msg("starting recursive watch")

	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			w.logger.Warn().Err(err).Str("path", path).Msg("error walking path")
			return nil // Skip paths with errors
		}

		if d.IsDir() {
			// Check if this directory should be ignored
			base := filepath.Base(path)
			for _, ignore := range w.ignoreDirs {
				if base == ignore {
					return filepath.SkipDir
				}
			}

			// Add directory to watcher
			if err := w.fsWatcher.Add(path); err != nil {
				w.logger.Warn().Err(err).Str("path", path).Msg("failed to watch directory")
				return nil
			}
			w.watchCounter++
		}

		return nil
	})

	if err != nil {
		return err
	}

	w.logger.Info().Int("directories", w.watchCounter).Msg("recursive watch started")

	// Start processing events
	w.wg.Add(1)
	go w.processEvents()

	return nil
}

// Unwatch stops watching a directory tree.
func (w *Watcher) Unwatch(root string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}

	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			_ = w.fsWatcher.Remove(path)
			w.watchCounter--
		}
		return nil
	})
}

// On registers an event handler for a specific event type.
func (w *Watcher) On(eventType EventType, handler EventHandler) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.handlers[eventType] = append(w.handlers[eventType], handler)
}

// OnCreate registers a handler for file creation events.
func (w *Watcher) OnCreate(handler EventHandler) {
	w.On(EventCreate, handler)
}

// OnModify registers a handler for file modification events.
func (w *Watcher) OnModify(handler EventHandler) {
	w.On(EventModify, handler)
}

// OnDelete registers a handler for file deletion events.
func (w *Watcher) OnDelete(handler EventHandler) {
	w.On(EventDelete, handler)
}

// OnRename registers a handler for file rename events.
func (w *Watcher) OnRename(handler EventHandler) {
	w.On(EventRename, handler)
}

// OnChmod registers a handler for permission change events.
func (w *Watcher) OnChmod(handler EventHandler) {
	w.On(EventChmod, handler)
}

// Close stops the watcher and releases resources.
func (w *Watcher) Close() error {
	w.cancel()
	timeout := w.shutdownTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		w.logger.Warn().
			Dur("timeout", timeout).
			Msg("watcher shutdown wait timed out; returning with workers still running")
	}
	return w.fsWatcher.Close()
}

// WatchedCount returns the number of watched directories.
func (w *Watcher) WatchedCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.watchCounter
}

// processEvents is the main event processing loop.
func (w *Watcher) processEvents() {
	defer w.wg.Done()

	for {
		select {
		case <-w.ctx.Done():
			return
		case fsEvent, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}
			// Panic boundary: a panic while processing one event must not
			// kill the loop (and with it the daemon) — the next event keeps
			// being processed.
			func() {
				defer w.recoverPanic("processEvents")
				w.handleFSEvent(fsEvent)
			}()
		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			w.logger.Warn().Err(err).Msg("file watcher error")
		}
	}
}

// handleFSEvent processes a single fsnotify event.
func (w *Watcher) handleFSEvent(fsEvent fsnotify.Event) {
	// Check if file should be ignored
	if w.shouldIgnore(fsEvent.Name) {
		return
	}

	// Check extension filter
	if !w.matchesExtension(fsEvent.Name) {
		return
	}

	// Map fsnotify event to our event type
	var eventTypes []EventType
	if fsEvent.Has(fsnotify.Create) {
		eventTypes = append(eventTypes, EventCreate)
		// If a new directory is created, watch it
		if info, err := os.Stat(fsEvent.Name); err == nil && info.IsDir() {
			w.watchNewDir(fsEvent.Name)
		}
	}
	if fsEvent.Has(fsnotify.Write) {
		eventTypes = append(eventTypes, EventModify)
	}
	if fsEvent.Has(fsnotify.Remove) {
		eventTypes = append(eventTypes, EventDelete)
	}
	if fsEvent.Has(fsnotify.Rename) {
		eventTypes = append(eventTypes, EventRename)
	}
	if fsEvent.Has(fsnotify.Chmod) {
		eventTypes = append(eventTypes, EventChmod)
	}

	for _, eventType := range eventTypes {
		event := FileEvent{
			Type:      eventType,
			Path:      fsEvent.Name,
			Timestamp: time.Now(),
			IsDir:     false,
		}

		// Check if it's a directory
		if info, err := os.Stat(fsEvent.Name); err == nil {
			event.IsDir = info.IsDir()
			if !info.IsDir() {
				event.Size = info.Size()
			}
		}

		// Handle rename old path
		if fsEvent.Has(fsnotify.Rename) {
			event.OldPath = fsEvent.Name
		}

		w.debounceEvent(event)
	}
}

// watchNewDir starts watching a newly created directory.
func (w *Watcher) watchNewDir(path string) {
	err := filepath.WalkDir(path, func(subPath string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := filepath.Base(subPath)
			for _, ignore := range w.ignoreDirs {
				if base == ignore {
					return filepath.SkipDir
				}
			}
			_ = w.fsWatcher.Add(subPath)
			w.mu.Lock()
			w.watchCounter++
			w.mu.Unlock()
		}
		return nil
	})
	if err != nil {
		w.logger.Warn().Err(err).Str("path", path).Msg("failed to watch new directory")
	}
}

// debounceEvent debounces rapid successive events for the same path.
func (w *Watcher) debounceEvent(event FileEvent) {
	w.debounceMu.Lock()
	defer w.debounceMu.Unlock()

	key := event.Path + ":" + string(event.Type)
	if existing, ok := w.pending[key]; ok {
		existing.timer.Stop()
		existing.event = event
		existing.timer.Reset(w.debounce)
		return
	}

	entry := &debounceEntry{
		event:     event,
		createdAt: time.Now(),
	}
	entry.timer = time.AfterFunc(w.debounce, func() {
		w.dispatchEvent(entry.event)
		w.debounceMu.Lock()
		delete(w.pending, key)
		w.debounceMu.Unlock()
	})
	w.pending[key] = entry
}

// recoverPanic logs a recovered panic with its stack trace. It is meant to
// be used as a deferred call inside a panic boundary so a single bad event or
// handler never crashes the daemon.
func (w *Watcher) recoverPanic(where string) {
	if rec := recover(); rec != nil {
		w.logger.Error().
			Interface("panic", rec).
			Str("stack", string(debug.Stack())).
			Str("where", where).
			Msg("watcher panic recovered")
	}
}

// dispatchEvent sends the event to all registered handlers.
func (w *Watcher) dispatchEvent(event FileEvent) {
	w.mu.RLock()
	handlers := w.handlers[event.Type]
	w.mu.RUnlock()

	if len(handlers) == 0 {
		w.logger.Trace().
			Str("type", string(event.Type)).
			Str("path", event.Path).
			Msg("no handlers for event type")
		return
	}

	w.logger.Debug().
		Str("type", string(event.Type)).
		Str("path", event.Path).
		Int("handlers", len(handlers)).
		Msg("dispatching event")

	for _, handler := range handlers {
		handlerCtx, cancel := context.WithTimeout(w.ctx, 30*time.Second)
		// Panic boundary per handler: one panicking handler must not stop
		// the remaining handlers nor kill the dispatch goroutine.
		func() {
			defer w.recoverPanic("event handler")
			if err := handler(handlerCtx, event); err != nil {
				w.logger.Warn().
					Err(err).
					Str("type", string(event.Type)).
					Str("path", event.Path).
					Msg("event handler error")
			}
		}()
		cancel()
	}
}

// shouldIgnore checks if a path matches ignore patterns.
func (w *Watcher) shouldIgnore(path string) bool {
	base := filepath.Base(path)
	name := base

	// Check exact matches
	for _, ignore := range w.ignoreFiles {
		if matched, _ := filepath.Match(ignore, name); matched {
			return true
		}
	}

	// Check directory name patterns
	rel := path
	for _, ignore := range w.ignoreDirs {
		if strings.Contains(rel, string(filepath.Separator)+ignore+string(filepath.Separator)) ||
			strings.HasPrefix(rel, ignore+string(filepath.Separator)) {
			return true
		}
	}

	return false
}

// matchesExtension checks if the file extension matches the watch filter.
func (w *Watcher) matchesExtension(path string) bool {
	if len(w.extensions) == 0 {
		return true // No filter means watch all
	}
	ext := filepath.Ext(path)
	if ext == "" {
		return true // Files without extension are always watched
	}
	for _, allowed := range w.extensions {
		if strings.EqualFold(ext, allowed) {
			return true
		}
	}
	return false
}
