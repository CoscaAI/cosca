package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadFromFile_DeliberationEnabled verifies that a YAML config with
// orchestration.deliberation.enabled: true propagates Enabled=true and the
// configured thresholds intact.
func TestLoadFromFile_DeliberationEnabled(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	yamlContent := `
version: "1.0"
orchestration:
  deliberation:
    enabled: true
    emit_threshold: 0.80
    reservation_threshold: 0.60
    max_evidence: 7
    max_chars_per_evidence: 500
    min_score: 0.40
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := LoadFromFile(cfgPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	d := cfg.Orchestration.Deliberation
	if !d.Enabled {
		t.Error("Deliberation.Enabled should be true when explicitly set")
	}
	if d.EmitThreshold != 0.80 {
		t.Errorf("EmitThreshold = %f, want %f", d.EmitThreshold, 0.80)
	}
	if d.ReservationThreshold != 0.60 {
		t.Errorf("ReservationThreshold = %f, want %f", d.ReservationThreshold, 0.60)
	}
	if d.MaxEvidence != 7 {
		t.Errorf("MaxEvidence = %d, want %d", d.MaxEvidence, 7)
	}
	if d.MaxCharsPerEvidence != 500 {
		t.Errorf("MaxCharsPerEvidence = %d, want %d", d.MaxCharsPerEvidence, 500)
	}
	if d.MinScore != 0.40 {
		t.Errorf("MinScore = %f, want %f", d.MinScore, 0.40)
	}
}

// TestLoadFromFile_DeliberationAbsent_FailClosed verifies the LEI DO COFRE:
// when the orchestration.deliberation section is absent, Enabled stays false.
func TestLoadFromFile_DeliberationAbsent_FailClosed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	yamlContent := `
version: "1.0"
provider:
  name: ollama
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := LoadFromFile(cfgPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	d := cfg.Orchestration.Deliberation
	if d.Enabled {
		t.Error("Deliberation.Enabled must be false when the section is absent (fail-closed)")
	}
	// Thresholds hold the A4 defaults even when the section is absent.
	if d.EmitThreshold != 0.70 {
		t.Errorf("EmitThreshold default = %f, want %f", d.EmitThreshold, 0.70)
	}
	if d.ReservationThreshold != 0.50 {
		t.Errorf("ReservationThreshold default = %f, want %f", d.ReservationThreshold, 0.50)
	}
	if d.MaxEvidence != 5 {
		t.Errorf("MaxEvidence default = %d, want %d", d.MaxEvidence, 5)
	}
	if d.MaxCharsPerEvidence != 300 {
		t.Errorf("MaxCharsPerEvidence default = %d, want %d", d.MaxCharsPerEvidence, 300)
	}
	if d.MinScore != 0.50 {
		t.Errorf("MinScore default = %f, want %f", d.MinScore, 0.50)
	}
}

// TestLoadFromFile_DeliberationPartial verifies that setting only enabled:true
// keeps the A4 threshold defaults for the rest.
func TestLoadFromFile_DeliberationPartial(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	yamlContent := `
version: "1.0"
orchestration:
  deliberation:
    enabled: true
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := LoadFromFile(cfgPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	d := cfg.Orchestration.Deliberation
	if !d.Enabled {
		t.Error("Deliberation.Enabled should be true")
	}
	// Unset thresholds fall back to the A4 defaults.
	if d.EmitThreshold != 0.70 {
		t.Errorf("EmitThreshold default = %f, want %f", d.EmitThreshold, 0.70)
	}
	if d.MaxEvidence != 5 {
		t.Errorf("MaxEvidence default = %d, want %d", d.MaxEvidence, 5)
	}
	if d.MinScore != 0.50 {
		t.Errorf("MinScore default = %f, want %f", d.MinScore, 0.50)
	}
}

// TestDefaultConfig_DeliberationFailClosed verifies the in-memory default is
// fail-closed (Enabled=false with A4 thresholds).
func TestDefaultConfig_DeliberationFailClosed(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	d := cfg.Orchestration.Deliberation
	if d.Enabled {
		t.Error("DefaultConfig Deliberation.Enabled must be false (fail-closed)")
	}
	if d.EmitThreshold != 0.70 {
		t.Errorf("EmitThreshold default = %f, want %f", d.EmitThreshold, 0.70)
	}
	if d.ReservationThreshold != 0.50 {
		t.Errorf("ReservationThreshold default = %f, want %f", d.ReservationThreshold, 0.50)
	}
	if d.MaxEvidence != 5 {
		t.Errorf("MaxEvidence default = %d, want %d", d.MaxEvidence, 5)
	}
	if d.MaxCharsPerEvidence != 300 {
		t.Errorf("MaxCharsPerEvidence default = %d, want %d", d.MaxCharsPerEvidence, 300)
	}
	if d.MinScore != 0.50 {
		t.Errorf("MinScore default = %f, want %f", d.MinScore, 0.50)
	}
}

// TestValidate_DeliberationThresholds verifies the sanity checks applied when
// the deliberation stage is enabled.
func TestValidate_DeliberationThresholds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		modify     func(cfg *Config)
		wantErr    bool
		errContain string
	}{
		{
			name: "disabled ignores garbage thresholds (inert)",
			modify: func(cfg *Config) {
				cfg.Orchestration.Deliberation.Enabled = false
				cfg.Orchestration.Deliberation.EmitThreshold = 2.5
			},
			wantErr: false,
		},
		{
			name: "valid thresholds pass",
			modify: func(cfg *Config) {
				cfg.Orchestration.Deliberation.Enabled = true
			},
			wantErr: false,
		},
		{
			name: "emit_threshold too high",
			modify: func(cfg *Config) {
				cfg.Orchestration.Deliberation.Enabled = true
				cfg.Orchestration.Deliberation.EmitThreshold = 1.5
			},
			wantErr:    true,
			errContain: "emit_threshold",
		},
		{
			name: "emit_threshold zero",
			modify: func(cfg *Config) {
				cfg.Orchestration.Deliberation.Enabled = true
				cfg.Orchestration.Deliberation.EmitThreshold = 0
			},
			wantErr:    true,
			errContain: "emit_threshold",
		},
		{
			name: "reservation_threshold >= 1",
			modify: func(cfg *Config) {
				cfg.Orchestration.Deliberation.Enabled = true
				cfg.Orchestration.Deliberation.ReservationThreshold = 1.0
			},
			wantErr:    true,
			errContain: "reservation_threshold",
		},
		{
			name: "max_evidence zero",
			modify: func(cfg *Config) {
				cfg.Orchestration.Deliberation.Enabled = true
				cfg.Orchestration.Deliberation.MaxEvidence = 0
			},
			wantErr:    true,
			errContain: "max_evidence",
		},
		{
			name: "max_chars_per_evidence zero",
			modify: func(cfg *Config) {
				cfg.Orchestration.Deliberation.Enabled = true
				cfg.Orchestration.Deliberation.MaxCharsPerEvidence = 0
			},
			wantErr:    true,
			errContain: "max_chars_per_evidence",
		},
		{
			name: "min_score zero",
			modify: func(cfg *Config) {
				cfg.Orchestration.Deliberation.Enabled = true
				cfg.Orchestration.Deliberation.MinScore = 0
			},
			wantErr:    true,
			errContain: "min_score",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modify(cfg)
			err := cfg.Validate()
			if tt.wantErr {
				if err == nil {
					t.Fatal("Expected validation error, got nil")
				}
				verr, ok := err.(*ValidationError)
				if !ok {
					t.Fatalf("Expected ValidationError, got %T", err)
				}
				found := false
				for _, e := range verr.Errors {
					if contains(e, tt.errContain) {
						found = true
					}
				}
				if !found {
					t.Errorf("Error %v should contain %q", verr, tt.errContain)
				}
			} else if err != nil {
				t.Errorf("Unexpected validation error: %v", err)
			}
		})
	}
}

// TestLoadFromEnv_DeliberationEnabled verifies COSCA_ORCHESTRATION__DELIBERATION__*
// env vars override the config (following the COSCA_SECTION__KEY pattern).
func TestLoadFromEnv_DeliberationEnabled(t *testing.T) {
	t.Setenv("COSCA_ORCHESTRATION__DELIBERATION__ENABLED", "true")
	t.Setenv("COSCA_ORCHESTRATION__DELIBERATION__EMIT_THRESHOLD", "0.85")
	t.Setenv("COSCA_ORCHESTRATION__DELIBERATION__MAX_EVIDENCE", "9")

	cfg := DefaultConfig()
	cfg.loadFromEnv()

	d := cfg.Orchestration.Deliberation
	if !d.Enabled {
		t.Error("COSCA_ORCHESTRATION__DELIBERATION__ENABLED=true should enable deliberation")
	}
	if d.EmitThreshold != 0.85 {
		t.Errorf("EmitThreshold = %f, want %f (env override)", d.EmitThreshold, 0.85)
	}
	if d.MaxEvidence != 9 {
		t.Errorf("MaxEvidence = %d, want %d (env override)", d.MaxEvidence, 9)
	}
}

// TestLoadFromEnv_DeliberationDisabled verifies COSCA_ORCHESTRATION__DELIBERATION__ENABLED=false
// keeps the stage off (fail-closed).
func TestLoadFromEnv_DeliberationDisabled(t *testing.T) {
	t.Setenv("COSCA_ORCHESTRATION__DELIBERATION__ENABLED", "false")

	cfg := DefaultConfig()
	cfg.loadFromEnv()

	if cfg.Orchestration.Deliberation.Enabled {
		t.Error("COSCA_ORCHESTRATION__DELIBERATION__ENABLED=false should keep deliberation disabled")
	}
}
