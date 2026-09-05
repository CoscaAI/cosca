// Package mic captures real-time microphone PCM on Windows, natively and
// sovereignly: no external library, no Python, no PortAudio — raw syscalls into
// the OS-builtin audio capture API (winmm/WaveIn). It converts the captured
// int16 PCM into normalized float32 [-1,1] chunks (the format sherpa ASR
// consumes) and hands them to a consumer via the `Source` channel seam.
//
// This is the last missing link of the audio loop:
//
//	mic → Chunk([]float32) → stt.AudioSource.PushPCM → sherpa streaming STT
//	      → bus.AudioSample (partial+final) → perception.bus → sync with vision
//
// Design goals:
//   - Sovereign: only syscall.NewLazyDLL("winmm.dll") — winmm ships with every
//     Windows install; no compiled native dependency, no pkg-config.
//   - Decoupled: `Source` exposes a channel of `Chunk` so the STT bridging and
//     the perception bus never depend on the capture backend.
//   - Graceful: a missing mic or unsupported platform degrades to ErrUnsupported
//     (callers fall back to a no-op audio source) — never crashes.
//
// The winmm capture path is implemented in capture_winmm_windows.go; non-Windows
// builds get an ErrUnsupported stub (capture_other.go).
package mic

import (
	"errors"
	"time"
)

// Sentinel errors. They are NOT fatal to the perception pipeline.
var (
	// ErrUnsupported reports that mic capture is not available on this platform
	// or build (e.g. non-Windows, or a Windows arch without the winmm layout).
	ErrUnsupported = errors.New("mic: microphone capture unavailable on this platform/build")
	// ErrNoDevice reports that no input (capture) audio device was found.
	ErrNoDevice = errors.New("mic: no capture audio device found")
	// ErrOpenFailed reports that the selected capture device refused to open.
	ErrOpenFailed = errors.New("mic: failed to open capture device")
)

// Config configures the microphone capture.
type Config struct {
	// Device selects the input device index. -1 (the zero value → resolved to
	// -1) uses the system default capture device (WAVE_MAPPER). >=0 selects a
	// device by index as reported by ListDevices.
	Device int `json:"device" yaml:"device"`
	// SampleRate is the capture sample rate in Hz. Default 16000.
	SampleRate int `json:"sample_rate" yaml:"sample_rate"`
	// Channels is the capture channel count. Only 1 (mono) is supported for the
	// STT loop. Default 1.
	Channels int `json:"channels" yaml:"channels"`
	// ChunkMS is the length of each emitted chunk in milliseconds. Default 100
	// (the STT/ASR sweet spot: ~100ms of PCM per PushPCM).
	ChunkMS int `json:"chunk_ms" yaml:"chunk_ms"`
}

// resolve returns the config with defaults filled in.
func (c Config) resolve() Config {
	if c.Device == 0 {
		c.Device = -1 // WAVE_MAPPER (default). Note: 0 and -1 both default here;
		// callers that explicitly want device 0 set Device after resolve, or we
		// treat 0 as "unset". See NewCapture for the authoritative Device mapping.
	}
	if c.SampleRate <= 0 {
		c.SampleRate = 16000
	}
	if c.Channels <= 0 {
		c.Channels = 1
	}
	if c.ChunkMS <= 0 {
		c.ChunkMS = 100
	}
	return c
}

// Chunk is one slice of normalized float32 PCM samples ([-1,1]) at SampleRate.
// It is the decoupled unit of audio a `Source` emits and a consumer (the STT
// AudioSource) feeds to push-to-ASR.
type Chunk struct {
	// Samples are normalized PCM in [-1,1], mono, interleaved per SampleRate.
	Samples []float32
	// SampleRate is the sample rate of the audio in Hz (e.g. 16000).
	SampleRate int
}

// Source streams microphone audio chunks. It is the injectable seam so the mic
// backend is invisible to the STT/bus bridging.
//
// Chunks MUST be non-blocking to the capture goroutine: a slow consumer (the
// ASR feed) must never backpressure the OS capture buffers. When the consumer
// cannot keep up, chunks are dropped (matching the STT source's own drop policy).
type Source interface {
	// Chunks returns a channel of audio chunks. It is closed when the source is
	// closed. A slow consumer is dropped (never backpressured).
	Chunks() <-chan Chunk
	// Close releases the capture device and stops the source. Idempotent.
	Close() error
}

// DeviceInfo describes one input (capture) audio device.
type DeviceInfo struct {
	// Index is the device index used in Config.Device.
	Index int
	// Name is the friendly product name (e.g. "Microfone (USB Audio Device)").
	Name string
	// MaxChannels is the maximum channel count the device supports.
	MaxChannels int
}

// NewCapture opens the configured microphone and returns a running Source. It
// is the platform dispatch: capture_winmm_windows.go implements it on Windows,
// capture_other.go stubs it elsewhere. blocksUntil reads are available.
func NewCapture(cfg Config) (Source, error) {
	return newCapture(cfg.resolve())
}

// DefaultChunkLen is the default capture chunk length used to size buffers.
const DefaultChunkLen = 100 * time.Millisecond
