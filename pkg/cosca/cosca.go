/*
 */

// Package cosca provides the core identity, versioning, and runtime
// information for the Cosca Enterprise Platform.
//
// This package is the single source of truth for build metadata and
// platform capabilities. All other packages import this one to
// access version strings, platform info, and runtime mode.
package cosca

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

// =============================================================================
// Version (var for ldflags override)
// =============================================================================

var (
	// Version is the current semantic version of Cosca.
	// Follows Semantic Versioning 2.0.0 (https://semver.org).
	// Overridable via ldflags at build time: -X 'pkg/cosca.Version=x.y.z'
	Version = "1.5.0"
)

// =============================================================================
// Constants
// =============================================================================

const (
	// ReleaseChannel indicates the stability of this build.
	// Possible values: "stable", "beta", "alpha", "dev".
	ReleaseChannel = "stable"

	// Codename is the human-readable codename for this release.
	Codename = "Nova"

	// MinimumGoVersion is the minimum Go version required to build.
	MinimumGoVersion = "1.22.0"

	// APIVersion is the version of the Cosca plugin API.
	APIVersion = "v1"

	// ConfigVersion is the version of the Cosca configuration schema.
	ConfigVersion = "v1"

	// ProtocolVersion is the version of the Cosca wire protocol.
	ProtocolVersion = "1.0"

	// DBVersion is the expected schema version for the Cosca database.
	DBVersion = 1
)

// =============================================================================
// Build-time Variables (set via ldflags)
// =============================================================================

var (
	// CommitHash is the git commit hash at build time.
	CommitHash = "unknown"

	// BuildDate is the RFC3339 timestamp of the build.
	BuildDate = "unknown"

	// BuildUser is the user who initiated the build.
	BuildUser = "unknown"

	// GoVersion is the Go version used to build (populated at init).
	GoVersion = runtime.Version()
)

// =============================================================================
// Runtime Mode
// =============================================================================

// Mode represents the Cosca runtime mode.
type Mode int

const (
	// ModeUnknown indicates the runtime mode could not be determined.
	ModeUnknown Mode = iota
	// ModeDev indicates development mode (hot reload, debug logging).
	ModeDev
	// ModeTest indicates test mode (isolated storage, mock providers).
	ModeTest
	// ModeStaging indicates staging/pre-production mode.
	ModeStaging
	// ModeProduction indicates production mode (optimized, minimal logging).
	ModeProduction
)

// String returns the string representation of the mode.
func (m Mode) String() string {
	switch m {
	case ModeDev:
		return "development"
	case ModeTest:
		return "test"
	case ModeStaging:
		return "staging"
	case ModeProduction:
		return "production"
	default:
		return "unknown"
	}
}

// ParseMode parses a string into a Mode.
func ParseMode(s string) Mode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "dev", "development":
		return ModeDev
	case "test", "testing":
		return ModeTest
	case "staging", "stage":
		return ModeStaging
	case "prod", "production":
		return ModeProduction
	default:
		return ModeUnknown
	}
}

// =============================================================================
// Platform Info
// =============================================================================

// Platform contains information about the current platform.
type Platform struct {
	OS           string `json:"os"`
	Architecture string `json:"arch"`
	GoVersion    string `json:"goVersion"`
	NumCPU       int    `json:"numCpu"`
	Hostname     string `json:"hostname,omitempty"`
	IsContainer  bool   `json:"isContainer,omitempty"`
	IsWSL        bool   `json:"isWsl,omitempty"`
}

// CurrentPlatform returns the platform information for the current system.
func CurrentPlatform() Platform {
	return Platform{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		GoVersion:    GoVersion,
		NumCPU:       runtime.NumCPU(),
		Hostname:     "", // populated lazily if needed
	}
}

// =============================================================================
// Build Info
// =============================================================================

// Info aggregates all build-time and runtime metadata.
type Info struct {
	Version        string    `json:"version"`
	CommitHash     string    `json:"commitHash"`
	BuildDate      string    `json:"buildDate"`
	BuildUser      string    `json:"buildUser"`
	GoVersion      string    `json:"goVersion"`
	ReleaseChannel string    `json:"releaseChannel"`
	Codename       string    `json:"codename"`
	APIVersion     string    `json:"apiVersion"`
	ConfigVersion  string    `json:"configVersion"`
	ProtocolVer    string    `json:"protocolVersion"`
	DBVersion      int       `json:"dbVersion"`
	Mode           Mode      `json:"mode"`
	Platform       Platform  `json:"platform"`
	StartedAt      time.Time `json:"startedAt"`
}

// CurrentInfo returns the full build and runtime info.
func CurrentInfo() Info {
	return Info{
		Version:        Version,
		CommitHash:     CommitHash,
		BuildDate:      BuildDate,
		BuildUser:      BuildUser,
		GoVersion:      GoVersion,
		ReleaseChannel: ReleaseChannel,
		Codename:       Codename,
		APIVersion:     APIVersion,
		ConfigVersion:  ConfigVersion,
		ProtocolVer:    ProtocolVersion,
		DBVersion:      DBVersion,
		Mode:           ModeUnknown,
		Platform:       CurrentPlatform(),
		StartedAt:      time.Now(),
	}
}

// String returns a human-readable summary.
func (i Info) String() string {
	return fmt.Sprintf("Cosca %s (%s/%s) [%s] - rev %s",
		i.Version, i.Platform.OS, i.Platform.Architecture, i.ReleaseChannel, i.CommitHash)
}

// Short returns a short version string.
func (i Info) Short() string {
	shortHash := i.CommitHash
	if len(shortHash) > 7 {
		shortHash = shortHash[:7]
	}
	return fmt.Sprintf("v%s (%s)", i.Version, shortHash)
}

// UserAgent returns a user-agent string suitable for HTTP clients.
func (i Info) UserAgent() string {
	return fmt.Sprintf("Cosca-CLI/%s (%s; %s) Go/%s",
		i.Version, i.Platform.OS, i.Platform.Architecture, i.GoVersion)
}

// =============================================================================
// Build Info Helpers
// =============================================================================

func init() {
	// Attempt to read build info from the running binary
	if info, ok := debug.ReadBuildInfo(); ok {
		GoVersion = info.GoVersion
		if BuildDate == "unknown" || BuildDate == "" {
			for _, s := range info.Settings {
				switch s.Key {
				case "vcs.time":
					if t, err := time.Parse(time.RFC3339, s.Value); err == nil {
						BuildDate = t.Format(time.RFC3339)
					}
				case "vcs.revision":
					if CommitHash == "unknown" || CommitHash == "" {
						CommitHash = s.Value
					}
				}
			}
		}
	}
}

// IsProduction reports whether the current mode is production.
func IsProduction() bool {
	modeStr := strings.ToLower(strings.TrimSpace(
		debugReadEnv("COSCA_MODE"),
	))
	return modeStr == "production" || modeStr == "prod"
}

// IsDev reports whether the current mode is development.
func IsDev() bool {
	modeStr := strings.ToLower(strings.TrimSpace(
		debugReadEnv("COSCA_DEV"),
	))
	return modeStr == "true" || modeStr == "1" || modeStr == "yes"
}

// debugReadEnv reads an environment variable from the OS.
func debugReadEnv(key string) string {
	return os.Getenv(key)
}
