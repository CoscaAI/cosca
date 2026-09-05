// Package filesystem provides a safe, atomic filesystem abstraction layer
// with backup support, hash verification, and crash-safe write operations.
// It wraps standard os/filepath operations with additional safety guarantees.
package filesystem

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/CoscaAI/cosca/internal/safe"
	"github.com/rs/zerolog"
)

// =============================================================================
// FileInfo
// =============================================================================

// FileInfo holds metadata about a file.
type FileInfo struct {
	Path       string      `json:"path" yaml:"path"`
	Name       string      `json:"name" yaml:"name"`
	Size       int64       `json:"size" yaml:"size"`
	Mode       os.FileMode `json:"mode" yaml:"mode"`
	IsDir      bool        `json:"is_dir" yaml:"is_dir"`
	ModTime    time.Time   `json:"mod_time" yaml:"mod_time"`
	SHA256Hash string      `json:"sha256,omitempty" yaml:"sha256,omitempty"`
}

// =============================================================================
// Filesystem
// =============================================================================

// Filesystem provides atomic, safe filesystem operations with backup and
// verification support.
type Filesystem struct {
	mu      sync.Mutex
	logger  zerolog.Logger
	backups bool // whether to create backups before overwriting
}

// Option configures the filesystem.
type Option func(*Filesystem)

// WithLogger sets the logger for the filesystem.
func WithLogger(logger zerolog.Logger) Option {
	return func(fs *Filesystem) {
		fs.logger = logger
	}
}

// WithBackups enables or disables automatic backup creation before overwrites.
func WithBackups(enabled bool) Option {
	return func(fs *Filesystem) {
		fs.backups = enabled
	}
}

// New creates a new Filesystem instance.
func New(opts ...Option) *Filesystem {
	fs := &Filesystem{
		logger:  zerolog.Nop(),
		backups: true,
	}
	for _, opt := range opts {
		opt(fs)
	}
	return fs
}

// =============================================================================
// Read Operations
// =============================================================================

// ReadFile reads the entire contents of a file.
func (fs *Filesystem) ReadFile(path string) ([]byte, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}
	fs.logger.Trace().
		Str("path", path).
		Int("bytes", len(data)).
		Msg("file read")
	return data, nil
}

// ReadFile is a package-level convenience function.
func ReadFile(path string) ([]byte, error) {
	return New().ReadFile(path)
}

// =============================================================================
// Write Operations
// =============================================================================

// WriteFile atomically writes data to a file. It writes to a temporary file
// first, then renames to the target path. If backups are enabled, the
// original file is backed up before overwriting.
func (fs *Filesystem) WriteFile(path string, data []byte) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	// Create a backup if the file exists and backups are enabled
	if fs.backups {
		if FileExists(path) {
			if err := fs.createBackup(path); err != nil {
				fs.logger.Warn().Err(err).Str("path", path).Msg("failed to create backup")
			}
		}
	}

	// Write to a temporary file first for atomicity. The temp file is opened
	// with write access so that Sync() works on Windows: FlushFileBuffers
	// requires a writable handle, and opening O_RDONLY returned
	// ERROR_ACCESS_DENIED ("Acesso negado").
	tmpPath := path + ".tmp." + fmt.Sprintf("%d", time.Now().UnixNano())
	tmpFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		safe.Remove(tmpPath) // Clean up temp file on error
		return fmt.Errorf("write temp file %s: %w", tmpPath, err)
	}
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		safe.Remove(tmpPath)
		return fmt.Errorf("write temp file %s: %w", tmpPath, err)
	}
	// Sync the temp file to disk before closing so the rename is crash-safe.
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		safe.Remove(tmpPath)
		return fmt.Errorf("sync temp file %s: %w", tmpPath, err)
	}
	if err := tmpFile.Close(); err != nil {
		safe.Remove(tmpPath)
		return fmt.Errorf("close temp file %s: %w", tmpPath, err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		safe.Remove(tmpPath)
		return fmt.Errorf("rename %s -> %s: %w", tmpPath, path, err)
	}

	fs.logger.Debug().
		Str("path", path).
		Int("bytes", len(data)).
		Msg("file written atomically")
	return nil
}

// WriteFile is a package-level convenience function.
func WriteFile(path string, data []byte) error {
	return New().WriteFile(path, data)
}

// SafeWrite writes data to a file with automatic backup of the existing file.
// The backup is saved as <path>.bak.<timestamp>.
func (fs *Filesystem) SafeWrite(path string, data []byte) error {
	if FileExists(path) {
		backupPath := fmt.Sprintf("%s.bak.%d", path, time.Now().Unix())
		if err := CopyFile(path, backupPath); err != nil {
			return fmt.Errorf("create safe-write backup: %w", err)
		}
		fs.logger.Debug().
			Str("path", path).
			Str("backup", backupPath).
			Msg("created pre-write backup")
	}
	return fs.WriteFile(path, data)
}

// SafeWrite is a package-level convenience function.
func SafeWrite(path string, data []byte) error {
	return New().SafeWrite(path, data)
}

// =============================================================================
// Copy and Move Operations
// =============================================================================

// CopyFile copies a file from src to dst. It creates directories in the
// destination path if they don't exist.
func (fs *Filesystem) CopyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source %s: %w", src, err)
	}
	defer func() { _ = srcFile.Close() }()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("stat source %s: %w", src, err)
	}

	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create destination directory %s: %w", dir, err)
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("create destination %s: %w", dst, err)
	}
	defer func() { _ = dstFile.Close() }()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copy %s -> %s: %w", src, dst, err)
	}

	// Sync the destination to disk
	if err := dstFile.Sync(); err != nil {
		return fmt.Errorf("sync destination %s: %w", dst, err)
	}

	fs.logger.Debug().
		Str("src", src).
		Str("dst", dst).
		Int64("size", srcInfo.Size()).
		Msg("file copied")
	return nil
}

// CopyFile is a package-level convenience function.
func CopyFile(src, dst string) error {
	return New().CopyFile(src, dst)
}

// MoveFile moves (renames) a file from src to dst. Falls back to copy+delete
// if the rename fails (e.g., across filesystem boundaries).
func (fs *Filesystem) MoveFile(src, dst string) error {
	// Try atomic rename first
	if err := os.Rename(src, dst); err == nil {
		fs.logger.Debug().
			Str("src", src).
			Str("dst", dst).
			Msg("file moved via rename")
		return nil
	}

	// Fall back to copy + delete
	if err := fs.CopyFile(src, dst); err != nil {
		return fmt.Errorf("move file (copy failed): %w", err)
	}

	if err := os.Remove(src); err != nil {
		// Attempt to clean up the destination
		safe.Remove(dst)
		return fmt.Errorf("move file (remove source failed): %w", err)
	}

	fs.logger.Debug().
		Str("src", src).
		Str("dst", dst).
		Msg("file moved via copy+delete")
	return nil
}

// MoveFile is a package-level convenience function.
func MoveFile(src, dst string) error {
	return New().MoveFile(src, dst)
}

// =============================================================================
// Directory Operations
// =============================================================================

// EnsureDir creates a directory and all parent directories if they don't exist.
// It's equivalent to mkdir -p.
func (fs *Filesystem) EnsureDir(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("ensure directory %s: %w", path, err)
	}
	return nil
}

// EnsureDir is a package-level convenience function.
func EnsureDir(path string) error {
	return New().EnsureDir(path)
}

// ListFiles returns all files in a directory matching the given glob pattern.
// The pattern is matched against filenames only (not the full path).
func (fs *Filesystem) ListFiles(dir string, pattern string) ([]string, error) {
	if !DirExists(dir) {
		return nil, fmt.Errorf("directory does not exist: %s", dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("list directory %s: %w", dir, err)
	}

	var matches []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if pattern == "" {
			matches = append(matches, filepath.Join(dir, entry.Name()))
			continue
		}
		matched, err := filepath.Match(pattern, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("pattern match %q: %w", pattern, err)
		}
		if matched {
			matches = append(matches, filepath.Join(dir, entry.Name()))
		}
	}

	return matches, nil
}

// ListFiles is a package-level convenience function.
func ListFiles(dir string, pattern string) ([]string, error) {
	return New().ListFiles(dir, pattern)
}

// =============================================================================
// Existence Checks
// =============================================================================

// FileExists returns true if the path exists and is a regular file.
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// DirExists returns true if the path exists and is a directory.
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// =============================================================================
// Hash and Info
// =============================================================================

// GetFileHash returns the SHA-256 hash of a file as a hex-encoded string.
func (fs *Filesystem) GetFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file for hash %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash file %s: %w", path, err)
	}

	hash := hex.EncodeToString(h.Sum(nil))
	fs.logger.Trace().
		Str("path", path).
		Str("sha256", hash).
		Msg("file hash computed")
	return hash, nil
}

// GetFileHash is a package-level convenience function.
func GetFileHash(path string) (string, error) {
	return New().GetFileHash(path)
}

// GetFileInfo returns metadata about a file, including its SHA-256 hash.
func (fs *Filesystem) GetFileInfo(path string) (FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return FileInfo{}, fmt.Errorf("stat file %s: %w", path, err)
	}

	fi := FileInfo{
		Path:    path,
		Name:    filepath.Base(path),
		Size:    info.Size(),
		Mode:    info.Mode(),
		IsDir:   info.IsDir(),
		ModTime: info.ModTime(),
	}

	// Compute hash for regular files
	if !info.IsDir() {
		hash, err := fs.GetFileHash(path)
		if err != nil {
			fs.logger.Warn().Err(err).Str("path", path).Msg("failed to compute file hash")
		} else {
			fi.SHA256Hash = hash
		}
	}

	return fi, nil
}

// GetFileInfo is a package-level convenience function.
func GetFileInfo(path string) (FileInfo, error) {
	return New().GetFileInfo(path)
}

// =============================================================================
// Backup and Restore
// =============================================================================

// createBackup creates a timestamped backup of a file before overwriting it.
func (fs *Filesystem) createBackup(path string) error {
	backupDir := filepath.Join(filepath.Dir(path), ".backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}

	timestamp := time.Now().UTC().Format("20060102T150405Z")
	baseName := filepath.Base(path)
	backupName := fmt.Sprintf("%s.%s.bak", baseName, timestamp)
	backupPath := filepath.Join(backupDir, backupName)

	if err := fs.CopyFile(path, backupPath); err != nil {
		return fmt.Errorf("create backup: %w", err)
	}

	fs.logger.Debug().
		Str("path", path).
		Str("backup", backupPath).
		Msg("backup created before overwrite")
	return nil
}

// =============================================================================
// Global utility functions
// =============================================================================

// HashBytes returns the SHA-256 hash of the given data as a hex string.
func HashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// UniqueTempPath returns a unique temporary file path in the system temp dir.
// A random suffix is appended because the system clock alone is not a reliable
// uniqueness source: on Windows the timer granularity can make two consecutive
// calls with the same prefix collide.
func UniqueTempPath(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fallback: timestamp only, still far more likely to be unique than
		// not; the random source is unavailable in exotic environments.
		return filepath.Join(os.TempDir(), fmt.Sprintf("%s.%d", prefix, time.Now().UnixNano()))
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("%s.%d.%x", prefix, time.Now().UnixNano(), b))
}

// IsAbsPath returns true if the given path is absolute.
func IsAbsPath(path string) bool {
	return filepath.IsAbs(path)
}

// ResolvePath resolves a path relative to base, expanding ~ and env vars.
func ResolvePath(path, base string) string {
	// Handle both ~/ (Unix) and ~\ (Windows) for home directory expansion.
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[2:])
		}
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(base, path))
}
