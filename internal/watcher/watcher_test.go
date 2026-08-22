package watcher

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestWatcherOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		opts  []Option
		check func(*testing.T, *Watcher)
	}{
		{
			name: "default options",
			opts: nil,
			check: func(t *testing.T, w *Watcher) {
				if w.debounce != 500*time.Millisecond {
					t.Errorf("debounce = %v, want 500ms", w.debounce)
				}
				if w.eventBufSize != 100 {
					t.Errorf("eventBufSize = %d, want 100", w.eventBufSize)
				}
			},
		},
		{
			name: "with logger",
			opts: []Option{WithLogger(zerolog.Nop())},
			check: func(_ *testing.T, _ *Watcher) {
				// just ensure no panic
			},
		},
		{
			name: "with debounce",
			opts: []Option{WithDebounce(time.Second)},
			check: func(t *testing.T, w *Watcher) {
				if w.debounce != time.Second {
					t.Errorf("debounce = %v, want 1s", w.debounce)
				}
			},
		},
		{
			name: "with ignore dirs",
			opts: []Option{WithIgnoreDirs("custom", "logs")},
			check: func(t *testing.T, w *Watcher) {
				found := false
				for _, d := range w.ignoreDirs {
					if d == "custom" {
						found = true
					}
				}
				if !found {
					t.Error("custom ignore dir not found")
				}
			},
		},
		{
			name: "with ignore files",
			opts: []Option{WithIgnoreFiles("*.log")},
			check: func(t *testing.T, w *Watcher) {
				found := false
				for _, f := range w.ignoreFiles {
					if f == "*.log" {
						found = true
					}
				}
				if !found {
					t.Error("*.log not in ignore files")
				}
			},
		},
		{
			name: "with extensions",
			opts: []Option{WithExtensions(".go", ".md")},
			check: func(t *testing.T, w *Watcher) {
				if len(w.extensions) != 2 {
					t.Fatalf("extensions = %v, want 2", w.extensions)
				}
			},
		},
		{
			name: "with event buffer size",
			opts: []Option{WithEventBufferSize(200)},
			check: func(t *testing.T, w *Watcher) {
				if w.eventBufSize != 200 {
					t.Errorf("eventBufSize = %d, want 200", w.eventBufSize)
				}
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w, err := New(tc.opts...)
			if err != nil {
				t.Fatalf("New() error: %v", err)
			}
			defer func() { _ = w.Close() }()
			tc.check(t, w)
		})
	}
}

func TestCloseReturnsWhenHandlerIgnoresCancellation(t *testing.T) {
	w, err := New(WithShutdownTimeout(30 * time.Millisecond))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	started := make(chan struct{})
	release := make(chan struct{})
	workerDone := make(chan struct{})
	w.OnModify(func(context.Context, FileEvent) error {
		close(started)
		<-release
		return nil
	})
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		defer close(workerDone)
		w.dispatchEvent(FileEvent{Type: EventModify, Path: "ignored"})
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}

	start := time.Now()
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("Close took %v, want it to return within the shutdown wait deadline", elapsed)
	}

	close(release)
	select {
	case <-workerDone:
	case <-time.After(time.Second):
		t.Fatal("watcher handler did not exit after release")
	}
}

func TestEventTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		eventType EventType
		want      string
	}{
		{EventCreate, "create"},
		{EventModify, "modify"},
		{EventDelete, "delete"},
		{EventRename, "rename"},
		{EventChmod, "chmod"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			if string(tc.eventType) != tc.want {
				t.Errorf("EventType = %q, want %q", tc.eventType, tc.want)
			}
		})
	}
}

func TestFileEvent(t *testing.T) {
	t.Parallel()

	event := FileEvent{
		Type:  EventCreate,
		Path:  "/test/file.go",
		IsDir: false,
		Size:  100,
	}

	if event.Type != EventCreate {
		t.Errorf("Type = %v", event.Type)
	}
	if event.Path != "/test/file.go" {
		t.Errorf("Path = %q", event.Path)
	}
	if event.Size != 100 {
		t.Errorf("Size = %d", event.Size)
	}
}

func TestMatchesExtension(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		extensions []string
		path       string
		want       bool
	}{
		{"no filter", nil, "file.go", true},
		{"empty filter", []string{}, "file.go", true},
		{"matching ext", []string{".go"}, "file.go", true},
		{"non-matching ext", []string{".md"}, "file.go", false},
		{"no extension", []string{".go"}, "Makefile", true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := &Watcher{extensions: tc.extensions}
			got := w.matchesExtension(tc.path)
			if got != tc.want {
				t.Errorf("matchesExtension(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestShouldIgnore(t *testing.T) {
	t.Parallel()

	w := &Watcher{
		ignoreDirs:  []string{".git", "node_modules"},
		ignoreFiles: []string{".DS_Store", "*.swp"},
	}

	tests := []struct {
		path string
		want bool
	}{
		// filepath.Join usa o separador nativo: no Windows os paths são
		// "\project\.git\config" etc., que é exatamente o que o fsnotify
		// entrega e o que shouldIgnore espera (filepath.Separator).
		{filepath.Join("/project", ".git", "config"), true},
		{filepath.Join("/project", "node_modules", "pkg", "index.js"), true},
		{"/project/.DS_Store", true},
		{"/project/file.swp", true},
		{filepath.Join("/project", "src", "main.go"), false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			got := w.shouldIgnore(tc.path)
			if got != tc.want {
				t.Errorf("shouldIgnore(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestHandlerRegistration(t *testing.T) {
	t.Parallel()

	w, err := New(WithLogger(zerolog.Nop()))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = w.Close() }()

	handler := func(_ context.Context, _ FileEvent) error {
		return nil
	}

	w.On(EventCreate, handler)
	if len(w.handlers[EventCreate]) != 1 {
		t.Errorf("handlers = %d, want 1", len(w.handlers[EventCreate]))
	}
}

func TestHandlerShortcuts(t *testing.T) {
	t.Parallel()

	w, err := New(WithLogger(zerolog.Nop()))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = w.Close() }()

	handler := func(_ context.Context, _ FileEvent) error { return nil }

	w.OnCreate(handler)
	w.OnModify(handler)
	w.OnDelete(handler)
	w.OnRename(handler)
	w.OnChmod(handler)

	if len(w.handlers[EventCreate]) != 1 {
		t.Error("OnCreate handler not registered")
	}
	if len(w.handlers[EventModify]) != 1 {
		t.Error("OnModify handler not registered")
	}
	if len(w.handlers[EventDelete]) != 1 {
		t.Error("OnDelete handler not registered")
	}
	if len(w.handlers[EventRename]) != 1 {
		t.Error("OnRename handler not registered")
	}
	if len(w.handlers[EventChmod]) != 1 {
		t.Error("OnChmod handler not registered")
	}
}

func TestQueueProcessor(t *testing.T) {
	t.Parallel()

	var processed []FileEvent
	handler := func(_ context.Context, event FileEvent) error {
		processed = append(processed, event)
		return nil
	}

	qp := NewQueueProcessor(handler, 50*time.Millisecond, zerolog.Nop())
	if qp == nil {
		t.Fatal("NewQueueProcessor returned nil")
	}

	qp.Enqueue(FileEvent{Type: EventCreate, Path: "/test.go"})
	if qp.QueueSize() != 1 {
		t.Errorf("queue size = %d, want 1", qp.QueueSize())
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Immediate cancel to avoid goroutine leak
	qp.Start(ctx)

	qp.Enqueue(FileEvent{Type: EventModify, Path: "/test2.go"})
	qp.Flush(ctx)

	if len(processed) < 1 {
		t.Error("expected at least one processed event")
	}
}

func TestMergeEvents(t *testing.T) {
	t.Parallel()

	qp := NewQueueProcessor(nil, time.Second, zerolog.Nop())
	events := []FileEvent{
		{Path: "/a", Type: EventCreate},
		{Path: "/b", Type: EventModify},
		{Path: "/a", Type: EventModify},
	}

	merged := qp.mergeEvents(events)
	if len(merged) != 2 {
		t.Fatalf("got %d events, want 2", len(merged))
	}
	// /a should be the latest (modify)
	for _, e := range merged {
		if e.Path == "/a" && e.Type != EventModify {
			t.Error("merged /a should be the latest event type (modify)")
		}
	}
}

func TestHandlerChain(t *testing.T) {
	t.Parallel()

	var order []string
	h1 := EventHandler(func(_ context.Context, _ FileEvent) error {
		order = append(order, "h1")
		return nil
	})
	h2 := EventHandler(func(_ context.Context, _ FileEvent) error {
		order = append(order, "h2")
		return nil
	})

	chain := NewHandlerChain(h1, h2)
	err := chain.Handle(context.Background(), FileEvent{Type: EventCreate})
	if err != nil {
		t.Fatalf("Handle error: %v", err)
	}
	if len(order) != 2 || order[0] != "h1" || order[1] != "h2" {
		t.Errorf("order = %v, want [h1 h2]", order)
	}
}

func TestFilterHandler(t *testing.T) {
	t.Parallel()

	var handled bool
	next := EventHandler(func(_ context.Context, _ FileEvent) error {
		handled = true
		return nil
	})

	filter := FilterHandler(func(event FileEvent) bool {
		return event.Type == EventModify
	}, next)

	// Should not pass filter
	_ = filter(context.Background(), FileEvent{Type: EventCreate})
	if handled {
		t.Error("create event should not pass the filter")
	}

	// Should pass filter
	_ = filter(context.Background(), FileEvent{Type: EventModify})
	if !handled {
		t.Error("modify event should pass the filter")
	}
}

func TestLoggingHandler(t *testing.T) {
	t.Parallel()

	var handled bool
	next := EventHandler(func(_ context.Context, _ FileEvent) error {
		handled = true
		return nil
	})

	h := LoggingHandler(zerolog.Nop(), next)
	err := h(context.Background(), FileEvent{Type: EventCreate, Path: "/test.go"})
	if err != nil {
		t.Fatalf("LoggingHandler error: %v", err)
	}
	if !handled {
		t.Error("next handler should have been called")
	}
}

func TestOnCreateHandler(t *testing.T) {
	t.Parallel()

	var calledPath string
	h := OnCreateHandler(func(_ context.Context, path string) error {
		calledPath = path
		return nil
	})

	// should process files
	_ = h(context.Background(), FileEvent{Type: EventCreate, Path: "/f.go", IsDir: false})
	if calledPath != "/f.go" {
		t.Errorf("calledPath = %q, want /f.go", calledPath)
	}

	// should skip directories
	calledPath = ""
	_ = h(context.Background(), FileEvent{Type: EventCreate, Path: "/dir", IsDir: true})
	if calledPath != "" {
		t.Error("should skip directories")
	}
}

func TestOnModifyHandler(t *testing.T) {
	t.Parallel()

	var calledPath string
	h := OnModifyHandler(func(_ context.Context, path string) error {
		calledPath = path
		return nil
	})

	_ = h(context.Background(), FileEvent{Type: EventModify, Path: "/f.go", IsDir: false})
	if calledPath != "/f.go" {
		t.Errorf("calledPath = %q", calledPath)
	}
}

func TestOnDeleteHandler(t *testing.T) {
	t.Parallel()

	var calledPath string
	h := OnDeleteHandler(func(_ context.Context, path string) error {
		calledPath = path
		return nil
	})

	_ = h(context.Background(), FileEvent{Type: EventDelete, Path: "/f.go"})
	if calledPath != "/f.go" {
		t.Errorf("calledPath = %q", calledPath)
	}
}

func TestOnRenameHandler(t *testing.T) {
	t.Parallel()

	var oldP, newP string
	h := OnRenameHandler(func(_ context.Context, oldPath, newPath string) error {
		oldP = oldPath
		newP = newPath
		return nil
	})

	_ = h(context.Background(), FileEvent{Type: EventRename, Path: "/new.go", OldPath: "/old.go"})
	if oldP != "/old.go" || newP != "/new.go" {
		t.Errorf("old=%q new=%q", oldP, newP)
	}
}

func TestBatchHandler(t *testing.T) {
	// BatchHandler uses a blocking select{}, so events must be sent concurrently.
	// This test sends two events in parallel goroutines and expects one flush of size 2.
	var mu sync.Mutex
	var flushCount int
	var batchSizes []int
	processor := func(_ context.Context, events []FileEvent) error {
		mu.Lock()
		flushCount++
		batchSizes = append(batchSizes, len(events))
		mu.Unlock()
		return nil
	}

	h := BatchHandler(2, time.Second, processor)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Send two events concurrently
	go func() { _ = h(ctx, FileEvent{Type: EventCreate, Path: "/a"}) }()
	go func() { _ = h(ctx, FileEvent{Type: EventCreate, Path: "/b"}) }()

	// Wait for flush
	time.Sleep(50 * time.Millisecond) // debounce window

	mu.Lock()
	if flushCount != 1 {
		t.Errorf("expected 1 flush, got %d", flushCount)
	}
	if len(batchSizes) > 0 && batchSizes[0] != 2 {
		t.Errorf("batch size = %d, want 2", batchSizes[0])
	}
	mu.Unlock()
}

// TestDispatchEvent_PanickingHandlerRecovered verifies the per-handler panic
// boundary: a panicking handler is recovered, the remaining handlers still
// run, and a later dispatch still works.
func TestDispatchEvent_PanickingHandlerRecovered(t *testing.T) {
	w, err := New(WithLogger(zerolog.Nop()))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = w.Close() }()

	var mu sync.Mutex
	called := 0

	w.On(EventModify, func(_ context.Context, _ FileEvent) error {
		panic("handler exploded")
	})
	w.On(EventModify, func(_ context.Context, _ FileEvent) error {
		mu.Lock()
		called++
		mu.Unlock()
		return nil
	})

	// A panicking handler must not crash the process nor stop the siblings.
	w.dispatchEvent(FileEvent{Type: EventModify, Path: "/a"})
	mu.Lock()
	first := called
	mu.Unlock()
	if first != 1 {
		t.Errorf("handlers after first dispatch = %d, want 1", first)
	}

	// The dispatch path keeps working for later events.
	w.dispatchEvent(FileEvent{Type: EventModify, Path: "/b"})
	mu.Lock()
	second := called
	mu.Unlock()
	if second != 2 {
		t.Errorf("handlers after second dispatch = %d, want 2", second)
	}
}

// TestWatcher_LoopSurvivesPanickingHandler runs the real fsnotify loop with a
// panicking handler: the panic is contained per handler, the loop keeps
// processing events, and later events still reach the healthy handler.
func TestWatcher_LoopSurvivesPanickingHandler(t *testing.T) {
	dir := t.TempDir()

	var mu sync.Mutex
	handled := 0

	w, err := New(
		WithLogger(zerolog.Nop()),
		WithDebounce(10*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = w.Close() }()

	w.OnCreate(func(_ context.Context, _ FileEvent) error {
		panic("create handler exploded")
	})
	w.OnCreate(func(_ context.Context, _ FileEvent) error {
		mu.Lock()
		handled++
		mu.Unlock()
		return nil
	})

	if err := w.Watch(dir); err != nil {
		t.Fatalf("Watch() error: %v", err)
	}

	// Trigger a first create event; the panicking handler fires and is
	// recovered, the counting handler still records it.
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write a.go: %v", err)
	}
	waitForHandled := func(want int) {
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			mu.Lock()
			n := handled
			mu.Unlock()
			if n >= want {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		mu.Lock()
		defer mu.Unlock()
		t.Fatalf("handler called %d times, want >= %d", handled, want)
	}
	waitForHandled(1)

	// A second event must still be processed: the loop survived the panic.
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("y"), 0o644); err != nil {
		t.Fatalf("write b.go: %v", err)
	}
	waitForHandled(2)
}
