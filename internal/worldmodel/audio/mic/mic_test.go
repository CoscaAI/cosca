// Plateform-independent mic package tests: defaults resolution and the Source
// contract. Real-capture tests live in mic_windows_test.go (windows) and the
// unsupported-platform path is asserted by mic_other_test.go (!windows).
package mic

import (
	"errors"
	"testing"
)

// TestConfigResolve ensures defaults are applied for a zero-value Config.
func TestConfigResolve(t *testing.T) {
	c := Config{}.resolve()
	if c.SampleRate != 16000 {
		t.Errorf("resolve(): SampleRate = %d, want 16000", c.SampleRate)
	}
	if c.Channels != 1 {
		t.Errorf("resolve(): Channels = %d, want 1", c.Channels)
	}
	if c.Device != -1 {
		t.Errorf("resolve(): Device = %d, want -1 (WAVE_MAPPER default)", c.Device)
	}
	if c.ChunkMS != 100 {
		t.Errorf("resolve(): ChunkMS = %d, want 100", c.ChunkMS)
	}
}

// TestConfiguredValuesKept ensures non-zero values are preserved, not clamped.
func TestConfiguredValuesKept(t *testing.T) {
	c := Config{
		Device:     2,
		SampleRate: 48000,
		Channels:   1,
		ChunkMS:    50,
	}.resolve()
	if c.Device != 2 || c.SampleRate != 48000 || c.Channels != 1 || c.ChunkMS != 50 {
		t.Errorf("resolve() clobbered explicit values: %+v", c)
	}
}

// TestErrUnsupported is a sentinel contract: on unsupported platforms NewCapture
// must return ErrUnsupported (never a panic) so the pipeline degrades gracefully.
func TestErrUnsupported(t *testing.T) {
	// This is validated per-platform (mic_other_test.go asserts the real return
	// on !windows). Here we only pin the sentinel identity.
	if errors.Is(ErrUnsupported, nil) {
		t.Fatal("ErrUnsupported should be a distinct sentinel error")
	}
}
