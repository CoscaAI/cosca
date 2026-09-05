package chat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
)

// HotReload watches configuration sources and re-selects the chat
// provider when API keys or model preferences change. It uses
// fsnotify for file-based config and periodic polling for
// environment variable changes.
type HotReload struct {
	registry *ChatRegistry
	config   HotReloadConfig
	watcher  *fsnotify.Watcher
	mu       sync.Mutex
	running  bool
	stopCh   chan struct{}
	lastHash string
	ctx      context.Context
	cancel   context.CancelFunc
}

// HotReloadConfig configures the hot-reload watcher.
type HotReloadConfig struct {
	// ConfigPaths lists config file paths to watch.
	ConfigPaths []string

	// PollInterval is how often to check for env var changes.
	PollInterval time.Duration // default: 30s

	// AutoDetect enables auto-detection on reload.
	AutoDetect bool // default: true
}

// DefaultHotReloadConfig returns sensible defaults.
func DefaultHotReloadConfig() HotReloadConfig {
	home, _ := os.UserHomeDir()
	return HotReloadConfig{
		ConfigPaths: []string{
			filepath.Join(".cosca", "config.yaml"),
			filepath.Join(home, ".cosca", "config.yaml"),
		},
		PollInterval: 30 * time.Second,
		AutoDetect:   true,
	}
}

// NewHotReload creates a hot-reload watcher for the chat registry.
func NewHotReload(registry *ChatRegistry, cfg HotReloadConfig) (*HotReload, error) {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 30 * time.Second
	}

	hr := &HotReload{
		registry: registry,
		config:   cfg,
		stopCh:   make(chan struct{}),
	}

	// Try to create file watcher.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Warn().Err(err).Msg("failed to create config watcher, using poll-only mode")
	} else {
		hr.watcher = watcher

		// Watch config paths that exist.
		for _, path := range cfg.ConfigPaths {
			if _, err := os.Stat(path); err == nil {
				dir := filepath.Dir(path)
				if err := watcher.Add(dir); err != nil {
					log.Warn().Err(err).Str("dir", dir).Msg("failed to watch config directory")
				} else {
					log.Debug().Str("dir", dir).Msg("watching for config changes")
				}
			}
		}
	}

	// Compute initial hash.
	hr.lastHash = hr.computeConfigHash()

	return hr, nil
}

// Start begins watching for configuration changes.
func (hr *HotReload) Start() {
	hr.mu.Lock()
	if hr.running {
		hr.mu.Unlock()
		return
	}
	hr.running = true
	hr.ctx, hr.cancel = context.WithCancel(context.Background())
	hr.mu.Unlock()

	go hr.watchLoop()
	log.Info().Msg("chat provider hot-reload started")
}

// Stop stops the hot-reload watcher.
func (hr *HotReload) Stop() {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	if !hr.running {
		return
	}
	hr.running = false
	close(hr.stopCh)

	if hr.cancel != nil {
		hr.cancel()
	}

	if hr.watcher != nil {
		_ = hr.watcher.Close()
	}

	log.Info().Msg("chat provider hot-reload stopped")
}

// ─── Watch Loop ──────────────────────────────────────────────────────────────

func (hr *HotReload) watchLoop() {
	pollTicker := time.NewTicker(hr.config.PollInterval)
	defer pollTicker.Stop()

	// Debounce timer — avoid multiple reloads for rapid changes.
	var debounceTimer *time.Timer
	const debounceInterval = 2 * time.Second

	for {
		select {
		case <-hr.stopCh:
			return

		case event, ok := <-hr.watcher.Events:
			if !ok {
				return
			}
			// Only react to write and create events on config files.
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				if hr.isConfigFile(event.Name) {
					log.Debug().Str("file", event.Name).Str("op", event.Op.String()).Msg("config file changed")
					if debounceTimer != nil {
						debounceTimer.Stop()
					}
					debounceTimer = time.AfterFunc(debounceInterval, hr.reloadIfChanged)
				}
			}

		case err, ok := <-hr.watcher.Errors:
			if !ok {
				return
			}
			log.Warn().Err(err).Msg("config watcher error")

		case <-pollTicker.C:
			// Periodic poll for env var changes.
			hr.reloadIfChanged()
		}
	}
}

// ─── Reload Logic ────────────────────────────────────────────────────────────

// reloadIfChanged checks if the configuration has changed and re-selects
// the provider if needed.
//
// It is invoked from two goroutines — the debounce time.AfterFunc callback
// and the periodic poll ticker — so it is serialized with hr.mu: lastHash
// and the registry.Select call are otherwise subject to a data race.
func (hr *HotReload) reloadIfChanged() {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	newHash := hr.computeConfigHash()
	if newHash == hr.lastHash {
		return // No change.
	}

	log.Info().Str("old_hash", hr.lastHash[:8]).Str("new_hash", newHash[:8]).Msg("configuration changed, reloading chat provider")

	hr.lastHash = newHash

	// Environment variables are re-read on next Select() call
	// via each provider's DefaultConfig() which reads from env.

	// Re-select the chat provider.
	cfg := DefaultChatRegistryConfig()
	cfg.AutoDetect = hr.config.AutoDetect

	if err := hr.registry.Select(hr.ctx, cfg); err != nil {
		log.Warn().Err(err).Msg("failed to reload chat provider")
		return
	}

	log.Info().Str("provider", hr.registry.Name()).Str("model", hr.registry.Model()).Msg("chat provider reloaded")
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// computeConfigHash creates a hash of the current configuration state.
// This includes API keys and model preferences that affect provider selection.
func (hr *HotReload) computeConfigHash() string {
	return computeProviderHash()
}

// computeProviderHash creates a hash from current env vars that affect
// provider selection. Changes to API keys trigger a reload.
func computeProviderHash() string {
	// Hash the values, rather than retaining credentials in the reload state.
	// The digest is only used for change detection and is never a credential.
	h := sha256.New()
	for _, name := range []string{
		"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "DEEPSEEK_API_KEY",
		"GOOGLE_API_KEY", "GROQ_API_KEY", "MISTRAL_API_KEY",
		"AZURE_OPENAI_API_KEY", "AWS_ACCESS_KEY_ID", "OLLAMA_HOST",
	} {
		_, _ = h.Write([]byte(name))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(os.Getenv(name)))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// isConfigFile returns true if the path is a watched config file.
func (hr *HotReload) isConfigFile(path string) bool {
	base := filepath.Base(path)
	for _, cfgPath := range hr.config.ConfigPaths {
		if filepath.Base(cfgPath) == base {
			return true
		}
	}
	// Also match .yaml/.yml files in watched directories.
	ext := filepath.Ext(path)
	return ext == ".yaml" || ext == ".yml"
}
