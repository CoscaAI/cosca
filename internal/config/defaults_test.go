package config

import (
	"testing"
	"time"
)

func TestDefaultConstants(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		value interface{}
		want  interface{}
	}{
		{"DefaultCoscaHome", DefaultCoscaHome, "~/.config/cosca"},
		{"DefaultCoscaProjectDir", DefaultCoscaProjectDir, ".cosca"},
		{"DefaultRuntimeDir", DefaultRuntimeDir, "runtime"},
		{"DefaultDataDir", DefaultDataDir, "data"},
		{"DefaultCacheDir", DefaultCacheDir, "cache"},
		{"DefaultLogsDir", DefaultLogsDir, "logs"},
		{"DefaultTempDir", DefaultTempDir, "tmp"},
		{"DefaultPluginsDir", DefaultPluginsDir, "plugins"},
		{"DefaultBackupDir", DefaultBackupDir, "backups"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != tt.want {
				t.Errorf("%s = %v, want %v", tt.name, tt.value, tt.want)
			}
		})
	}
}

func TestDefaultTimeoutsValues(t *testing.T) {
	t.Parallel()
	if DefaultCommandTimeout != 30*time.Second {
		t.Errorf("DefaultCommandTimeout = %v", DefaultCommandTimeout)
	}
	if DefaultAgentTimeout != 5*time.Minute {
		t.Errorf("DefaultAgentTimeout = %v", DefaultAgentTimeout)
	}
	if DefaultSkillTimeout != 2*time.Minute {
		t.Errorf("DefaultSkillTimeout = %v", DefaultSkillTimeout)
	}
	if DefaultWorkflowTimeout != 10*time.Minute {
		t.Errorf("DefaultWorkflowTimeout = %v", DefaultWorkflowTimeout)
	}
	if DefaultRequestTimeout != 30*time.Second {
		t.Errorf("DefaultRequestTimeout = %v", DefaultRequestTimeout)
	}
}

func TestDefaultCacheValues(t *testing.T) {
	t.Parallel()
	if DefaultCacheSize != 10000 {
		t.Errorf("DefaultCacheSize = %d", DefaultCacheSize)
	}
	if DefaultCacheTTL != 5*time.Minute {
		t.Errorf("DefaultCacheTTL = %v", DefaultCacheTTL)
	}
}

func TestDefaultDBValues(t *testing.T) {
	t.Parallel()
	if DefaultMaxOpenDBConn != 25 {
		t.Errorf("DefaultMaxOpenDBConn = %d", DefaultMaxOpenDBConn)
	}
	if DefaultMaxIdleDBConn != 5 {
		t.Errorf("DefaultMaxIdleDBConn = %d", DefaultMaxIdleDBConn)
	}
	if DefaultDBPageSize != 4096 {
		t.Errorf("DefaultDBPageSize = %d", DefaultDBPageSize)
	}
}

func TestDefaultProviderValues(t *testing.T) {
	t.Parallel()
	if DefaultProvider != "ollama" {
		t.Errorf("DefaultProvider = %q", DefaultProvider)
	}
	if DefaultProviderModel != "qwen2.5-coder:14b-128k" {
		t.Errorf("DefaultProviderModel = %q", DefaultProviderModel)
	}
	if DefaultProviderContextWindow != 131072 {
		t.Errorf("DefaultProviderContextWindow = %d", DefaultProviderContextWindow)
	}
	if DefaultProviderMaxTokens != 16384 {
		t.Errorf("DefaultProviderMaxTokens = %d", DefaultProviderMaxTokens)
	}
	if DefaultProviderTemperature != 0.2 {
		t.Errorf("DefaultProviderTemperature = %f", DefaultProviderTemperature)
	}
}

func TestDefaultEmbeddingValues(t *testing.T) {
	t.Parallel()
	if DefaultEmbeddingModel != "nomic-embed-text" {
		t.Errorf("DefaultEmbeddingModel = %q", DefaultEmbeddingModel)
	}
	if DefaultEmbeddingDimensions != 768 {
		t.Errorf("DefaultEmbeddingDimensions = %d", DefaultEmbeddingDimensions)
	}
}

func TestDefaultEditorValues(t *testing.T) {
	t.Parallel()
	if DefaultEditor != "vim" {
		t.Errorf("DefaultEditor = %q", DefaultEditor)
	}
	if DefaultEditorTheme != "default" {
		t.Errorf("DefaultEditorTheme = %q", DefaultEditorTheme)
	}
	if DefaultEditorFontSize != 12 {
		t.Errorf("DefaultEditorFontSize = %d", DefaultEditorFontSize)
	}
}

func TestDefaultFeatureFlags(t *testing.T) {
	t.Parallel()
	if DefaultEnableMetrics != false {
		t.Error("DefaultEnableMetrics should be false")
	}
	if DefaultEnableTelemetry != true {
		t.Error("DefaultEnableTelemetry should be true")
	}
	if DefaultEnableAutoUpdate != true {
		t.Error("DefaultEnableAutoUpdate should be true")
	}
	if DefaultEnableVectorSearch != true {
		t.Error("DefaultEnableVectorSearch should be true")
	}
	if DefaultEnableWatch != true {
		t.Error("DefaultEnableWatch should be true")
	}
	if DefaultEnablePluginSystem != false {
		t.Error("DefaultEnablePluginSystem should be false")
	}
}

func TestDefaultPerformanceValues(t *testing.T) {
	t.Parallel()
	if DefaultMaxMemoryMB != 512 {
		t.Errorf("DefaultMaxMemoryMB = %d", DefaultMaxMemoryMB)
	}
	if DefaultMaxOpenFiles != 1024 {
		t.Errorf("DefaultMaxOpenFiles = %d", DefaultMaxOpenFiles)
	}
	if DefaultBatchSize != 100 {
		t.Errorf("DefaultBatchSize = %d", DefaultBatchSize)
	}
}

func TestDefaultNetworkValues(t *testing.T) {
	t.Parallel()
	if DefaultAPIPort != 8370 {
		t.Errorf("DefaultAPIPort = %d", DefaultAPIPort)
	}
	if DefaultMetricsPort != 8371 {
		t.Errorf("DefaultMetricsPort = %d", DefaultMetricsPort)
	}
	if DefaultRPCPort != 8372 {
		t.Errorf("DefaultRPCPort = %d", DefaultRPCPort)
	}
	if DefaultHost != "127.0.0.1" {
		t.Errorf("DefaultHost = %q", DefaultHost)
	}
}

func TestDefaultLogValues(t *testing.T) {
	t.Parallel()
	if DefaultLogMaxSizeMB != 100 {
		t.Errorf("DefaultLogMaxSizeMB = %d", DefaultLogMaxSizeMB)
	}
	if DefaultLogMaxBackups != 5 {
		t.Errorf("DefaultLogMaxBackups = %d", DefaultLogMaxBackups)
	}
	if DefaultLogMaxAgeDays != 30 {
		t.Errorf("DefaultLogMaxAgeDays = %d", DefaultLogMaxAgeDays)
	}
}

func TestDefaultSearchValues(t *testing.T) {
	t.Parallel()
	if DefaultSearchResultLimit != 50 {
		t.Errorf("DefaultSearchResultLimit = %d", DefaultSearchResultLimit)
	}
	if DefaultSearchMaxResults != 1000 {
		t.Errorf("DefaultSearchMaxResults = %d", DefaultSearchMaxResults)
	}
}
