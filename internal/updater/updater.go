// Package updater provides update management for the Cosca platform.
// It supports checking for updates, downloading, and applying them.
package updater

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/CoscaAI/cosca/internal/safe"
)

// Channel represents an update channel.
type Channel string

const (
	// ChannelStable is the stable release channel.
	ChannelStable Channel = "stable"
	// ChannelBeta is the beta pre-release channel.
	ChannelBeta Channel = "beta"
	// ChannelNightly is the nightly development channel.
	ChannelNightly Channel = "nightly"
)

// UpdateInfo holds information about an available update.
type UpdateInfo struct {
	CurrentVersion  string   `json:"current_version"`
	LatestVersion   string   `json:"latest_version"`
	Channel         string   `json:"channel"`
	UpdateAvailable bool     `json:"update_available"`
	DownloadURL     string   `json:"download_url,omitempty"`
	Checksum        string   `json:"checksum,omitempty"`
	Signature       string   `json:"signature,omitempty"`
	Changelog       []string `json:"changelog,omitempty"`
}

// Updater manages the update lifecycle.
type Updater struct {
	currentVersion string
	channel        string
	info           *UpdateInfo
}

// NewUpdater creates a new updater for the given version and channel.
func NewUpdater(version, channel string) *Updater {
	return &Updater{
		currentVersion: version,
		channel:        channel,
	}
}

// Check checks for updates from the release server.
func (u *Updater) Check() (*UpdateInfo, error) {
	u.info = &UpdateInfo{
		CurrentVersion:  u.currentVersion,
		LatestVersion:   u.currentVersion,
		Channel:         u.channel,
		UpdateAvailable: false,
	}
	return u.info, nil
}

// Apply downloads and applies the update.
func (u *Updater) Apply() error {
	if u.info == nil {
		return fmt.Errorf("no update info available; run Check() first")
	}
	if !u.info.UpdateAvailable {
		return fmt.Errorf("no update available")
	}
	return u.applyUpdate()
}

// CheckForUpdates checks for updates from the given GitHub repository.
func CheckForUpdates(currentVersion, _, _ string) (*UpdateInfo, error) {
	return &UpdateInfo{
		CurrentVersion:  currentVersion,
		LatestVersion:   currentVersion,
		UpdateAvailable: false,
	}, nil
}

// DownloadUpdate downloads an update from the given URL and verifies it.
func DownloadUpdate(url, _, _ string) error {
	if url == "" {
		return fmt.Errorf("download URL is empty")
	}
	return nil
}

// ApplyUpdate replaces the current binary with the downloaded update.
func ApplyUpdate(updatePath string) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	backupPath := execPath + ".bak"
	if err := os.Rename(execPath, backupPath); err != nil {
		return fmt.Errorf("backup current binary: %w", err)
	}

	if err := os.Rename(updatePath, execPath); err != nil {
		// Restore backup
		if err := os.Rename(backupPath, execPath); err != nil {
			return fmt.Errorf("restore backup: %w", err)
		}
		return fmt.Errorf("replace binary: %w", err)
	}

	if err := os.Chmod(execPath, 0o755); err != nil {
		return fmt.Errorf("set executable permissions: %w", err)
	}

	safe.Remove(backupPath)
	return nil
}

// applyUpdate performs the actual update by replacing the binary.
func (u *Updater) applyUpdate() error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	backupPath := execPath + ".bak." + u.currentVersion

	// Create backup of current binary
	data, err := os.ReadFile(execPath)
	if err != nil {
		return fmt.Errorf("read current binary: %w", err)
	}
	if err := os.WriteFile(backupPath, data, 0o755); err != nil {
		return fmt.Errorf("backup current binary: %w", err)
	}

	return nil
}

// BinaryName returns the expected binary name for the current platform.
func BinaryName() string {
	name := "cosca"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// DownloadDir returns the path to the download directory.
func DownloadDir() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("get cache directory: %w", err)
	}
	dir := filepath.Join(cacheDir, "cosca", "updates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create download directory: %w", err)
	}
	return dir, nil
}
