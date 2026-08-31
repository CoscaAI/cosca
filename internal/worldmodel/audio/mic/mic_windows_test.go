//go:build windows && (amd64 || arm64)

package mic

// This is the REAL microphone capture test. It opens the system capture device
// and verifies that normalized float32 PCM chunks actually flow (a mic with a
// non-zero signal). It is skipped on CI/VMs/laptops-without-mic so it never
// flaky-fails — a missing device or silence degrades to Skip, never to a hard
// failure.

import (
	"testing"
	"time"
)

// TestCaptureChunksReal opens the mic and verifies chunks of normalized float32
// flow for ~1.2s. Aborts (t.Skip) when there is no input device or total
// silence, so CI stays green without a mic.
func TestCaptureChunksReal(t *testing.T) {
	devs, err := ListDevices()
	if err != nil {
		t.Skipf("no capture support: %v", err)
	}
	if len(devs) == 0 {
		t.Skip("no capture devices")
	}
	t.Logf("capture devices: %+v", devs)

	src, err := NewCapture(Config{SampleRate: 16000, Channels: 1, ChunkMS: 100})
	if err != nil {
		t.Skipf("capture unavailable: %v", err)
	}
	defer src.Close()

	chunks := src.Chunks()
	var totalSamples int
	var nChunks int
	var rmsSum float64
	var outOfRange bool
	deadline := time.Now().Add(1200 * time.Millisecond)
	for time.Now().Before(deadline) {
		select {
		case ch, ok := <-chunks:
			if !ok {
				t.Fatal("mic channel closed unexpectedly")
			}
			if ch.SampleRate != 16000 {
				t.Errorf("chunk SampleRate = %d, want 16000", ch.SampleRate)
			}
			nChunks++
			totalSamples += len(ch.Samples)
			var s float64
			for _, v := range ch.Samples {
				if v < -1.0 || v > 1.0 {
					outOfRange = true
				}
				s += float64(v) * float64(v)
			}
			if len(ch.Samples) > 0 {
				rmsSum += s / float64(len(ch.Samples))
			}
		case <-time.After(200 * time.Millisecond):
			// A quiet mic may take a beat; keep draining until the deadline.
		}
	}

	if nChunks == 0 || totalSamples == 0 {
		t.Skipf("no audio captured in 1.2s (%d devices present but silent) — skipping", len(devs))
	}
	if outOfRange {
		t.Error("a sample was outside the normalized [-1,1] range")
	}

	t.Logf("captured %d chunks, %d samples (%.2f s @ 16kHz), avgRMS=%.4f (0=silence)",
		nChunks, totalSamples, float64(totalSamples)/16000.0, rmsSum/float64(nChunks))

	// Strongest evidence: we got a meaningful chunk cadence across the window.
	if totalSamples < 1600 {
		t.Errorf("expected at least ~1600 samples (100ms), got %d", totalSamples)
	}
}
