package integrity

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// WatchConfig holds the watchdog configuration.
type WatchConfig struct {
	Root     string        // Cosca project root
	Interval time.Duration // Polling fallback interval (e.g., 30s)
}

// TamperDetail holds forensic info about a tampered file.
type TamperDetail struct {
	Path string // File path
	Diff string // git diff showing what changed
}

// WatchResult is emitted for each watchdog check cycle.
type WatchResult struct {
	Time      time.Time
	Valid     bool
	Blocks    int
	Files     int
	Errors    []string
	Tampers   []TamperDetail // Forensic details for each tampered file
	GitAnchor string         // git anchoring status of the latest block
	Message   string
}

// Watch runs a real-time integrity watchdog using fsnotify.
// It monitors the chain file, keys directory, and embed files for changes.
// On any change (debounced), it runs a full integrity check and reports.
// The callback is called with each result. Returns on error or when the
// done channel is closed.
func Watch(cfg WatchConfig, onResult func(WatchResult), done <-chan struct{}) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	defer watcher.Close()

	// Watch targets
	targets := []string{
		filepath.Join(cfg.Root, ".cosca", "family_chain.dat"),
		filepath.Join(cfg.Root, ".cosca", "keys"),
		filepath.Join(cfg.Root, "internal", "embed", "cosca"),
	}

	for _, t := range targets {
		// For directories, add recursive watch
		info, err := os.Stat(t)
		if err != nil {
			// Target doesn't exist yet — that's fine, check on next poll
			continue
		}
		if info.IsDir() {
			if err := watcher.Add(t); err != nil {
				return fmt.Errorf("watch %s: %w", t, err)
			}
			// Add subdirectories
			filepath.Walk(t, func(path string, fi os.FileInfo, err error) error {
				if err != nil || !fi.IsDir() {
					return nil
				}
				if path == t {
					return nil
				}
				// Skip hidden dirs and deep nesting (>3 levels from embed)
				rel, _ := filepath.Rel(t, path)
				if strings.HasPrefix(filepath.Base(path), ".") {
					return filepath.SkipDir
				}
				if strings.Count(rel, string(filepath.Separator)) > 5 {
					return filepath.SkipDir
				}
				watcher.Add(path)
				return nil
			})
		} else {
			// Single file — watch its parent directory
			watcher.Add(filepath.Dir(t))
		}
	}

	// Polling ticker for periodic full check
	pollInterval := cfg.Interval
	if pollInterval <= 0 {
		pollInterval = 30 * time.Second
	}
	pollTicker := time.NewTicker(pollInterval)
	defer pollTicker.Stop()

	// Debounce timer
	var debounceTimer *time.Timer
	const debounceDelay = 500 * time.Millisecond

	// Run initial check
	runCheck(cfg.Root, onResult)

	fmt.Fprintf(os.Stderr, "👁️  Watchdog active — monitoring %d targets every %s\n", len(targets), pollInterval)
	fmt.Fprintf(os.Stderr, "   .cosca/family_chain.dat\n")
	fmt.Fprintf(os.Stderr, "   .cosca/keys/\n")
	fmt.Fprintf(os.Stderr, "   internal/embed/cosca/\n\n")

	for {
		select {
		case <-done:
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			// Only care about writes and creates
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}

			// Debounce: reset timer on each event, fire after quiet period
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(debounceDelay, func() {
				runCheck(cfg.Root, onResult)
			})

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(os.Stderr, "⚠️  Watchdog error: %v\n", err)

		case <-pollTicker.C:
			// Periodic full check as fallback
			runCheck(cfg.Root, onResult)
		}
	}
}

// runCheck executes a full integrity check and calls the callback with the result.
func runCheck(root string, onResult func(WatchResult)) {
	info, err := Check(root)

	result := WatchResult{
		Time: time.Now(),
	}

	if err != nil {
		result.Valid = false
		result.Errors = []string{err.Error()}
		result.Message = fmt.Sprintf("🔴 INTEGRITY FATAL: %v", err)
		onResult(result)
		return
	}

	result.Valid = info.Valid
	result.Blocks = info.Blocks
	result.Files = info.Files
	result.Errors = info.Errors
	result.GitAnchor = GitAnchorStatus(root)

	// Capture forensic diffs for tampered files
	if !info.Valid {
		for _, e := range info.Errors {
			// Errors look like: "Block N: TAMPERED — path/to/file"
			if strings.Contains(e, "TAMPERED — ") {
				parts := strings.SplitN(e, "TAMPERED — ", 2)
				if len(parts) == 2 {
					filePath := strings.TrimSpace(parts[1])
					diff := captureDiff(root, filePath)
					result.Tampers = append(result.Tampers, TamperDetail{
						Path: filePath,
						Diff: diff,
					})
				}
			}
			// Also capture MISSING files
			if strings.Contains(e, "FILE MISSING — ") {
				parts := strings.SplitN(e, "FILE MISSING — ", 2)
				if len(parts) == 2 {
					filePath := strings.TrimSpace(parts[1])
					result.Tampers = append(result.Tampers, TamperDetail{
						Path: filePath,
						Diff: "⚠️  FILE DELETED — was present in chain manifest",
					})
				}
			}
		}
		result.Message = fmt.Sprintf("🔴 BREACH DETECTED — %d files tampered", len(info.Errors))
	} else if len(info.Warnings) > 0 {
		result.Message = fmt.Sprintf("⚠️  Chain valid (%d blocks, %d files) — %s | %s", info.Blocks, info.Files, result.GitAnchor, strings.Join(info.Warnings, "; "))
	} else {
		result.Message = fmt.Sprintf("✅ Chain valid (%d blocks, %d files) — %s", info.Blocks, info.Files, result.GitAnchor)
	}
	onResult(result)
}

// captureDiff runs git diff on a specific file to show what was changed.
func captureDiff(root, filePath string) string {
	cmd := exec.Command("git", "-C", root, "diff", "--", filePath)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat")
	out, err := cmd.Output()
	if err != nil {
		// git diff may fail (file not in git, or no repo)
		// Fallback: show file stats
		fullPath := filepath.Join(root, filePath)
		if stat, statErr := os.Stat(fullPath); statErr == nil {
			return fmt.Sprintf("   Size: %d bytes | Modified: %s", stat.Size(), stat.ModTime().Format(time.RFC3339))
		}
		return "   (could not capture diff)"
	}
	diff := strings.TrimSpace(string(out))
	if diff == "" {
		return "   (no changes tracked by git — file may be untracked)"
	}
	// Truncate long diffs
	lines := strings.Split(diff, "\n")
	if len(lines) > 20 {
		diff = strings.Join(lines[:20], "\n") + fmt.Sprintf("\n   ... (%d more lines)", len(lines)-20)
	}
	return diff
}
