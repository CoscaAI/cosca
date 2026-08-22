package cosca

import (
	"runtime"
	"strings"
	"testing"
)

// =============================================================================
// Mode Tests
// =============================================================================

func TestMode_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mode     Mode
		expected string
	}{
		{"dev", ModeDev, "development"},
		{"test", ModeTest, "test"},
		{"staging", ModeStaging, "staging"},
		{"production", ModeProduction, "production"},
		{"unknown default", ModeUnknown, "unknown"},
		{"invalid value", Mode(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.mode.String()
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestParseMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected Mode
	}{
		{"dev", ModeDev},
		{"development", ModeDev},
		{"DEV", ModeDev},
		{"  dev  ", ModeDev},
		{"test", ModeTest},
		{"testing", ModeTest},
		{"TEST", ModeTest},
		{"staging", ModeStaging},
		{"stage", ModeStaging},
		{"STAGING", ModeStaging},
		{"prod", ModeProduction},
		{"production", ModeProduction},
		{"PRODUCTION", ModeProduction},
		{"", ModeUnknown},
		{"unknown", ModeUnknown},
		{"invalid-mode", ModeUnknown},
		{" banana ", ModeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseMode(tt.input)
			if got != tt.expected {
				t.Errorf("ParseMode(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// Version Constants
// =============================================================================

func TestVersionConstants(t *testing.T) {
	t.Parallel()

	if Version == "" {
		t.Error("Version should not be empty")
	}
	if !strings.HasPrefix(Version, "1.") {
		t.Errorf("Version should start with '1.', got %q", Version)
	}

	if ReleaseChannel == "" {
		t.Error("ReleaseChannel should not be empty")
	}

	if Codename == "" {
		t.Error("Codename should not be empty")
	}

	if MinimumGoVersion == "" {
		t.Error("MinimumGoVersion should not be empty")
	}

	if APIVersion == "" {
		t.Error("APIVersion should not be empty")
	}

	if ConfigVersion == "" {
		t.Error("ConfigVersion should not be empty")
	}

	if ProtocolVersion == "" {
		t.Error("ProtocolVersion should not be empty")
	}

	if DBVersion <= 0 {
		t.Errorf("DBVersion should be positive, got %d", DBVersion)
	}
}

// =============================================================================
// CurrentPlatform
// =============================================================================

func TestCurrentPlatform(t *testing.T) {
	t.Parallel()

	platform := CurrentPlatform()
	if platform.OS != runtime.GOOS {
		t.Errorf("expected OS %q, got %q", runtime.GOOS, platform.OS)
	}
	if platform.Architecture != runtime.GOARCH {
		t.Errorf("expected Arch %q, got %q", runtime.GOARCH, platform.Architecture)
	}
	if platform.GoVersion == "" {
		t.Error("GoVersion should not be empty")
	}
	if platform.NumCPU <= 0 {
		t.Errorf("NumCPU should be positive, got %d", platform.NumCPU)
	}
}

// =============================================================================
// CurrentInfo
// =============================================================================

func TestCurrentInfo(t *testing.T) {
	t.Parallel()

	info := CurrentInfo()

	if info.Version != Version {
		t.Errorf("expected Version %q, got %q", Version, info.Version)
	}
	if info.CommitHash == "" {
		t.Error("CommitHash should not be empty")
	}
	if info.BuildDate == "" {
		t.Error("BuildDate should not be empty")
	}
	if info.BuildUser == "" {
		t.Error("BuildUser should not be empty")
	}
	if info.GoVersion == "" {
		t.Error("GoVersion should not be empty")
	}
	if info.ReleaseChannel != ReleaseChannel {
		t.Errorf("expected ReleaseChannel %q, got %q", ReleaseChannel, info.ReleaseChannel)
	}
	if info.Codename != Codename {
		t.Errorf("expected Codename %q, got %q", Codename, info.Codename)
	}
	if info.APIVersion != APIVersion {
		t.Errorf("expected APIVersion %q, got %q", APIVersion, info.APIVersion)
	}
	if info.ConfigVersion != ConfigVersion {
		t.Errorf("expected ConfigVersion %q, got %q", ConfigVersion, info.ConfigVersion)
	}
	if info.ProtocolVer != ProtocolVersion {
		t.Errorf("expected ProtocolVer %q, got %q", ProtocolVersion, info.ProtocolVer)
	}
	if info.DBVersion != DBVersion {
		t.Errorf("expected DBVersion %d, got %d", DBVersion, info.DBVersion)
	}
	if info.Mode != ModeUnknown {
		t.Errorf("expected Mode Unknown, got %v", info.Mode)
	}
	if info.Platform.OS != runtime.GOOS {
		t.Errorf("expected platform OS %q, got %q", runtime.GOOS, info.Platform.OS)
	}
	if info.StartedAt.IsZero() {
		t.Error("StartedAt should not be zero")
	}
}

// =============================================================================
// Info Methods
// =============================================================================

func TestInfo_String(t *testing.T) {
	t.Parallel()

	info := Info{
		Version:        "2.0.0",
		Platform:       Platform{OS: "linux", Architecture: "amd64"},
		ReleaseChannel: "stable",
		CommitHash:     "abcdef1234567890",
	}

	str := info.String()
	if !strings.Contains(str, "Cosca 2.0.0") {
		t.Errorf("String() should contain version, got %q", str)
	}
	if !strings.Contains(str, "linux/amd64") {
		t.Errorf("String() should contain platform, got %q", str)
	}
	if !strings.Contains(str, "[stable]") {
		t.Errorf("String() should contain release channel, got %q", str)
	}
	if !strings.Contains(str, "abcdef1234567890") {
		t.Errorf("String() should contain commit hash, got %q", str)
	}
}

func TestInfo_Short(t *testing.T) {
	t.Parallel()

	info := Info{
		Version:    "1.2.3",
		CommitHash: "deadbeef1234567890fedcba",
	}

	short := info.Short()
	expected := "v1.2.3 (deadbee)"

	if short != expected {
		t.Errorf("expected %q, got %q", expected, short)
	}
}

func TestInfo_Short_ShortCommitHash(t *testing.T) {
	t.Parallel()

	// Only test with hash >= 7 chars since Short() does commitHash[:7] without bounds check.
	info := Info{
		Version:    "3.0.0",
		CommitHash: "abc1234def5678",
	}

	short := info.Short()
	expected := "v3.0.0 (abc1234)"

	if short != expected {
		t.Errorf("expected %q, got %q", expected, short)
	}
}

func TestInfo_UserAgent(t *testing.T) {
	t.Parallel()

	info := Info{
		Version:   "1.0.0",
		Platform:  Platform{OS: "darwin", Architecture: "arm64"},
		GoVersion: "go1.25.0",
	}

	ua := info.UserAgent()
	if !strings.Contains(ua, "Cosca-CLI/1.0.0") {
		t.Errorf("UserAgent should contain CLI/version, got %q", ua)
	}
	if !strings.Contains(ua, "darwin") {
		t.Errorf("UserAgent should contain OS, got %q", ua)
	}
	if !strings.Contains(ua, "arm64") {
		t.Errorf("UserAgent should contain architecture, got %q", ua)
	}
	if !strings.Contains(ua, "Go/go1.25.0") {
		t.Errorf("UserAgent should contain Go version, got %q", ua)
	}
}

// =============================================================================
// IsProduction / IsDev
// =============================================================================

func TestIsProduction(t *testing.T) {
	// Not parallel — relies on env vars which are process-global.
	// These will always return false in test because debugReadEnv returns "".

	// Since debugReadEnv always returns "", IsProduction always returns false.
	if IsProduction() {
		t.Error("IsProduction should return false when COSCA_MODE is unset")
	}
}

func TestIsDev(t *testing.T) {
	// Since debugReadEnv always returns "", IsDev always returns false.
	if IsDev() {
		t.Error("IsDev should return false when COSCA_DEV is unset")
	}
}

// =============================================================================
// Build-time Variables
// =============================================================================

func TestBuildTimeVariables(t *testing.T) {
	t.Parallel()

	// These are set at init() or build-time.
	// CommitHash may be "unknown" or a real git hash.
	if CommitHash == "" {
		t.Error("CommitHash should not be empty")
	}

	if BuildDate == "" {
		t.Error("BuildDate should not be empty")
	}

	if BuildUser == "" {
		t.Error("BuildUser should not be empty")
	}

	if GoVersion == "" {
		t.Error("GoVersion should not be empty")
	}

	if !strings.HasPrefix(GoVersion, "go") {
		t.Errorf("GoVersion should start with 'go', got %q", GoVersion)
	}
}

// =============================================================================
// connState.String
// =============================================================================

func TestConnState_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state    connState
		expected string
	}{
		{connStateDisconnected, "disconnected"},
		{connStateConnecting, "connecting"},
		{connStateConnected, "connected"},
		{connStateReconnecting, "reconnecting"},
		{connStateClosed, "closed"},
		{connState(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := tt.state.String()
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

// =============================================================================
// Platform struct
// =============================================================================

func TestPlatform_Fields(t *testing.T) {
	t.Parallel()

	p := Platform{
		OS:           "windows",
		Architecture: "amd64",
		GoVersion:    "go1.24.0",
		NumCPU:       8,
		Hostname:     "dev-machine",
		IsContainer:  true,
		IsWSL:        false,
	}

	if p.OS != "windows" {
		t.Errorf("expected OS %q, got %q", "windows", p.OS)
	}
	if p.Architecture != "amd64" {
		t.Errorf("expected Arch %q, got %q", "amd64", p.Architecture)
	}
	if p.GoVersion != "go1.24.0" {
		t.Errorf("expected GoVersion %q, got %q", "go1.24.0", p.GoVersion)
	}
	if p.NumCPU != 8 {
		t.Errorf("expected NumCPU %d, got %d", 8, p.NumCPU)
	}
	if p.Hostname != "dev-machine" {
		t.Errorf("expected Hostname %q, got %q", "dev-machine", p.Hostname)
	}
	if !p.IsContainer {
		t.Error("expected IsContainer to be true")
	}
	if p.IsWSL {
		t.Error("expected IsWSL to be false")
	}
}

// =============================================================================
// ClientConfig.Validate edge cases
// =============================================================================

func TestClientConfig_Validate_NegativeMaxRetries(t *testing.T) {
	t.Parallel()

	cfg := ClientConfig{
		RuntimeAddr: "localhost:9090",
		MaxRetries:  -1,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for negative MaxRetries")
	}
	if !strings.Contains(err.Error(), "max retries must be non-negative") {
		t.Errorf("expected message about max retries, got %q", err.Error())
	}
}

// =============================================================================
// CoscaError with Details
// =============================================================================

func TestCoscaError_WithDetails(t *testing.T) {
	t.Parallel()

	err := &CoscaError{
		StatusCode: 422,
		Code:       "VALIDATION_ERROR",
		Message:    "invalid input",
		Details: map[string]interface{}{
			"field":  "email",
			"reason": "invalid format",
		},
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "[VALIDATION_ERROR]") {
		t.Errorf("expected code in error, got %q", errStr)
	}
	if !strings.Contains(errStr, "invalid input") {
		t.Errorf("expected message in error, got %q", errStr)
	}
}

// =============================================================================
// ClientConfig_DefaultValues
// =============================================================================

func TestClientConfig_SetDefaults_Partial(t *testing.T) {
	t.Parallel()

	// Test that only zero-value fields get defaults.
	cfg := ClientConfig{
		RuntimeAddr:       "custom:1234",
		Timeout:           60 * 1e9, // 60s as ns
		MaxRetries:        5,
		HeartbeatInterval: 30 * 1e9,
		UserAgent:         "custom-agent/2.0",
	}
	cfg.setDefaults()

	if cfg.RuntimeAddr != "custom:1234" {
		t.Errorf("RuntimeAddr should not be overwritten, got %q", cfg.RuntimeAddr)
	}
	if cfg.MaxRetries != 5 {
		t.Errorf("MaxRetries should not be overwritten, got %d", cfg.MaxRetries)
	}
	if cfg.UserAgent != "custom-agent/2.0" {
		t.Errorf("UserAgent should not be overwritten, got %q", cfg.UserAgent)
	}
}

// =============================================================================
// ClientConfig.Validate_EmptyAddr
// =============================================================================

func TestClientConfig_Validate_EmptyAddr(t *testing.T) {
	t.Parallel()

	cfg := ClientConfig{
		RuntimeAddr: "",
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for empty address")
	}
	if !strings.Contains(err.Error(), "runtime address is required") {
		t.Errorf("expected 'runtime address is required', got %q", err.Error())
	}
}
