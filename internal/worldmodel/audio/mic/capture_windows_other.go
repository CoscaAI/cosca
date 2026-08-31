//go:build windows && !(amd64 || arm64)

package mic

// This stub covers Windows on 32-bit (or otherwise non-8-byte-pointer) arches
// where the winmm struct layout (WAVEHDR/WAVEINCAPS) would not match the go
// struct definitions in capture_winmm_windows.go. Mic capture is constrained to
// windows/amd64+arm64; these builds degrade gracefully with ErrUnsupported.

// newCapture returns ErrUnsupported on 32-bit Windows.
func newCapture(Config) (Source, error) {
	return nil, ErrUnsupported
}

// ListDevices returns ErrUnsupported on 32-bit Windows.
func ListDevices() ([]DeviceInfo, error) {
	return nil, ErrUnsupported
}
