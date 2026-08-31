package orchestration

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/deliberate"
)

// TestDeliberateConfigFromConfig_Enabled verifies the YAML→engine mapping
// propagates Enabled=true and all thresholds.
func TestDeliberateConfigFromConfig_Enabled(t *testing.T) {
	got := DeliberateConfigFromConfig(config.DeliberationConfig{
		Enabled:              true,
		EmitThreshold:        0.82,
		ReservationThreshold: 0.61,
		MaxEvidence:          6,
		MaxCharsPerEvidence:  400,
		MinScore:             0.55,
	})

	assert.True(t, got.Enabled, "Enabled must propagate true")
	assert.Equal(t, 0.82, got.EmitThreshold)
	assert.Equal(t, 0.61, got.ReservationThreshold)
	assert.Equal(t, 6, got.MaxEvidence)
	assert.Equal(t, 400, got.MaxCharsPerEvidence)
	assert.Equal(t, 0.55, got.MinScore)
	assert.Equal(t, deliberate.DefaultConvergenceWeights(), got.Weights,
		"Weights must default to the A3 convergence defaults")
}

// TestDeliberateConfigFromConfig_Absent_FailClosed verifies the LEI DO COFRE:
// a zero-value (section absent) deliberation config maps to Enabled=false.
func TestDeliberateConfigFromConfig_Absent_FailClosed(t *testing.T) {
	got := DeliberateConfigFromConfig(config.DeliberationConfig{})

	assert.False(t, got.Enabled, "Enabled must be false for an absent section (fail-closed)")
	assert.Equal(t, deliberate.DefaultConvergenceWeights(), got.Weights)
}

// TestDeliberateConfigFromConfig_EqualsDefault verifies the invariant: mapping
// the config package's default DeliberationConfig yields EXACTLY the engine's
// DefaultDeliberateConfig() — the legacy path is preserved bit-for-bit when
// the YAML section is absent.
func TestDeliberateConfigFromConfig_EqualsDefault(t *testing.T) {
	def := DefaultDeliberateConfig()
	got := DeliberateConfigFromConfig(config.DeliberationConfig{
		EmitThreshold:        def.EmitThreshold,
		ReservationThreshold: def.ReservationThreshold,
		MaxEvidence:          def.MaxEvidence,
		MaxCharsPerEvidence:  def.MaxCharsPerEvidence,
		MinScore:             def.MinScore,
	})

	assert.Equal(t, def, got)
}

// TestDeliberateConfigFromConfig_ThresholdsZero verifies zero-valued thresholds
// survive the mapping untouched (the deliberator normalizes them at run time).
func TestDeliberateConfigFromConfig_ThresholdsZero(t *testing.T) {
	got := DeliberateConfigFromConfig(config.DeliberationConfig{Enabled: true})

	assert.True(t, got.Enabled)
	assert.Equal(t, 0.0, got.EmitThreshold, "zero thresholds are forwarded as-is (normalized later)")
	assert.Equal(t, deliberate.DefaultConvergenceWeights(), got.Weights,
		"Weights must always be the deliberate defaults, never nil/zero")
}
