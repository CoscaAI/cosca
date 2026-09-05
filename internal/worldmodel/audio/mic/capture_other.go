//go:build !windows

package mic

// This stub keeps the mic package buildable on non-Windows platforms (Linux CI,
// macOS dev, cross-builds). Microphone capture is only implemented natively on
// Windows (winmm/WaveIn); elsewhere we degrade gracefully with ErrUnsupported
// so the perception pipeline never crashes and the bus falls back to a no-op
// audio source.

// newCapture returns ErrUnsupported on non-Windows platforms.
func newCapture(Config) (Source, error) {
	return nil, ErrUnsupported
}

// ListDevices returns ErrUnsupported on non-Windows platforms.
func ListDevices() ([]DeviceInfo, error) {
	return nil, ErrUnsupported
}
