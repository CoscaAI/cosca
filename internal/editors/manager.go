//
// Editor Manager: detects, sets up, validates, and manages
// all supported editor adapters.

package editors

import (
	"fmt"
	"sort"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/CoscaAI/cosca/internal/editors/claude"
	"github.com/CoscaAI/cosca/internal/editors/codex"
	"github.com/CoscaAI/cosca/internal/editors/cursor"
	genericmcp "github.com/CoscaAI/cosca/internal/editors/generic_mcp"
	"github.com/CoscaAI/cosca/internal/editors/neovim"
	"github.com/CoscaAI/cosca/internal/editors/opencode"
	"github.com/CoscaAI/cosca/internal/editors/types"
	"github.com/CoscaAI/cosca/internal/editors/vscode"
	"github.com/CoscaAI/cosca/internal/editors/windsurf"
	"github.com/CoscaAI/cosca/internal/editors/zed"
)

// =============================================================================
// Manager
// =============================================================================

// Manager discovers, configures, and manages editor adapters.
// It maintains a registry of all available editors and provides
// methods to detect which editors are present on the system.
type Manager struct {
	mu sync.RWMutex

	// adapters maps editor name -> Editor adapter.
	adapters map[string]types.Editor

	// config is the default configuration for new setups.
	config types.EditorConfig

	// detected caches the latest detection results.
	detected map[string]types.EditorInfo
}

// NewManager creates a new editor manager with all built-in adapters.
func NewManager(config types.EditorConfig) *Manager {
	m := &Manager{
		adapters: make(map[string]types.Editor),
		config:   config,
		detected: make(map[string]types.EditorInfo),
	}

	// Register all built-in editor adapters
	m.registerBuiltin()

	return m
}

// registerBuiltin registers all built-in editor adapters.
func (m *Manager) registerBuiltin() {
	// Order matters for detection priority
	adapters := []types.Editor{
		opencode.NewAdapter(),
		claude.NewAdapter(),
		codex.NewAdapter(),
		cursor.NewAdapter(),
		windsurf.NewAdapter(),
		zed.NewAdapter(),
		vscode.NewAdapter(),
		neovim.NewAdapter(),
		genericmcp.NewAdapter(),
	}

	for _, adapter := range adapters {
		m.adapters[adapter.Name()] = adapter
	}

	log.Debug().Int("count", len(adapters)).Msg("editor adapters registered")
}

// =============================================================================
// Detection
// =============================================================================

// Detect runs detection on all registered editors and returns info
// for the first one that is detected, using a priority order.
func (m *Manager) Detect() (types.EditorInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Priority order for detection
	priority := []string{
		"opencode",
		"codex",
		"cursor",
		"windsurf",
		"zed",
		"claude",
		"vscode",
		"neovim",
		"generic_mcp",
	}

	for _, name := range priority {
		adapter, exists := m.adapters[name]
		if !exists {
			continue
		}

		detected, err := adapter.Detect()
		if err != nil {
			log.Debug().Err(err).Str("editor", name).Msg("editor detection error")
			continue
		}

		if detected {
			info, err := adapter.Info()
			if err != nil {
				log.Warn().Err(err).Str("editor", name).Msg("editor info error")
				continue
			}

			info.Detected = true
			// Check if setup is needed
			validateErr := adapter.Validate()
			info.SetupComplete = validateErr == nil
			info.SetupRequired = !info.SetupComplete

			m.detected[name] = info
			log.Info().Str("editor", name).Bool("setup_complete", info.SetupComplete).
				Msg("editor detected")

			return info, nil
		}
	}

	return types.EditorInfo{}, fmt.Errorf("nenhum editor encontrado. Editors suportados: opencode, vscode, cursor, claude, neovim, zed, windsurf. Tente: cosca editor setup opencode")
}

// DetectAll runs detection on all registered editors.
func (m *Manager) DetectAll() []types.EditorInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.detected = make(map[string]types.EditorInfo)
	var results []types.EditorInfo

	for name, adapter := range m.adapters {
		detected, err := adapter.Detect()
		if err != nil {
			log.Debug().Err(err).Str("editor", name).Msg("editor detection error")
			continue
		}

		info, err := adapter.Info()
		if err != nil {
			continue
		}

		info.Detected = detected
		if detected {
			validateErr := adapter.Validate()
			info.SetupComplete = validateErr == nil
			info.SetupRequired = !info.SetupComplete
		}

		m.detected[name] = info
		results = append(results, info)
	}

	// Sort by name for deterministic output
	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	return results
}

// =============================================================================
// Setup
// =============================================================================

// Setup configures the specified editor for Cosca integration.
func (m *Manager) Setup(editor string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	adapter, exists := m.adapters[editor]
	if !exists {
		return fmt.Errorf("unknown editor: %q", editor)
	}

	detected, err := adapter.Detect()
	if err != nil {
		return fmt.Errorf("detect %q: %w", editor, err)
	}
	if !detected {
		return fmt.Errorf("setup %q: %w", editor, types.ErrEditorNotDetected)
	}

	config := m.config
	config.AutoSetup = true

	if err := adapter.Setup(config); err != nil {
		return &types.ErrEditorSetupFailed{
			Editor: editor,
			Err:    err,
		}
	}

	log.Info().Str("editor", editor).Msg("editor setup complete")
	return nil
}

// SetupResult reports the outcome of installing Cosca integration into a
// single editor during a bulk SetupAll. A nil Err means the editor was
// configured successfully.
type SetupResult struct {
	Editor string
	Err    error
}

// SetupAll configures ALL registered editors for Cosca integration,
// tolerating individual failures. Unlike Setup, it does NOT require an
// editor to be detected first: every registered adapter is configured so
// that `cosca install --all` works on fresh projects. Results are returned
// sorted by editor name.
func (m *Manager) SetupAll() []SetupResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	names := make([]string, 0, len(m.adapters))
	for name := range m.adapters {
		names = append(names, name)
	}
	sort.Strings(names)

	results := make([]SetupResult, 0, len(names))
	for _, name := range names {
		adapter := m.adapters[name]

		config := m.config
		config.AutoSetup = true

		err := adapter.Setup(config)
		if err == nil {
			log.Info().Str("editor", name).Msg("editor setup complete")
		} else {
			log.Warn().Str("editor", name).Err(err).Msg("editor setup failed")
		}

		results = append(results, SetupResult{Editor: name, Err: err})
	}

	return results
}

// SetupForce configures a single editor for Cosca integration without
// requiring the editor to be detected first. It is used when the user
// explicitly names an editor (e.g. `cosca editor setup --editor claude`).
func (m *Manager) SetupForce(editor string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	adapter, exists := m.adapters[editor]
	if !exists {
		return fmt.Errorf("unknown editor: %q", editor)
	}

	config := m.config
	config.AutoSetup = true

	if err := adapter.Setup(config); err != nil {
		return &types.ErrEditorSetupFailed{
			Editor: editor,
			Err:    err,
		}
	}

	log.Info().Str("editor", editor).Msg("editor setup complete")
	return nil
}

// =============================================================================
// Validation
// =============================================================================

// Validate checks whether Cosca integration is properly configured.
func (m *Manager) Validate(editor string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	adapter, exists := m.adapters[editor]
	if !exists {
		return fmt.Errorf("unknown editor: %q", editor)
	}

	return adapter.Validate()
}

// =============================================================================
// Listing
// =============================================================================

// List returns information about all registered editors.
func (m *Manager) List() []types.EditorInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []types.EditorInfo
	for _, adapter := range m.adapters {
		info, err := adapter.Info()
		if err != nil {
			continue
		}
		// Merge with cached detection data
		if cached, ok := m.detected[adapter.Name()]; ok {
			info.Detected = cached.Detected
			info.SetupComplete = cached.SetupComplete
			info.SetupRequired = cached.SetupRequired
		}
		result = append(result, info)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// Names returns the sorted names of all registered editors.
func (m *Manager) Names() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.adapters))
	for name := range m.adapters {
		names = append(names, name)
	}

	sort.Strings(names)
	return names
}

// =============================================================================
// Adapter Access
// =============================================================================

// GetAdapter returns the editor adapter for the given name.
func (m *Manager) GetAdapter(name string) (types.Editor, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	adapter, exists := m.adapters[name]
	if !exists {
		return nil, fmt.Errorf("editor adapter %q not found", name)
	}
	return adapter, nil
}

// =============================================================================
// Teardown
// =============================================================================

// Teardown removes Cosca integration from the specified editor.
func (m *Manager) Teardown(editor string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	adapter, exists := m.adapters[editor]
	if !exists {
		return fmt.Errorf("unknown editor: %q", editor)
	}

	if err := adapter.Teardown(); err != nil {
		return fmt.Errorf("teardown %q: %w", editor, err)
	}

	delete(m.detected, editor)
	log.Info().Str("editor", editor).Msg("editor teardown complete")
	return nil
}

// =============================================================================
// Convenience
// =============================================================================

// AutoDetectAndSetup detects the active editor and sets up Cosca integration.
func (m *Manager) AutoDetectAndSetup() (*types.EditorInfo, error) {
	info, err := m.Detect()
	if err != nil {
		return nil, fmt.Errorf("auto-detect: %w", err)
	}

	if info.SetupComplete {
		log.Info().Str("editor", info.Name).Msg("Cosca integration already configured")
		return &info, nil
	}

	if err := m.Setup(info.Name); err != nil {
		return nil, fmt.Errorf("auto-setup: %w", err)
	}

	// Re-validate after setup
	if err := m.Validate(info.Name); err != nil {
		return nil, fmt.Errorf("auto-setup validation: %w", err)
	}

	info.SetupComplete = true
	info.SetupRequired = false

	log.Info().Str("editor", info.Name).Msg("Cosca integration auto-configured")
	return &info, nil
}
