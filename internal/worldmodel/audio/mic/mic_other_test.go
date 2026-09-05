//go:build !windows || (windows && !(amd64 || arm64))

package mic

// On unsupported platforms mic capture degrades to ErrUnsupported. This test
// pins that contract so the perception pipeline's graceful fallback is never
// accidentally broken.
import (
	"errors"
	"testing"
)

func TestUnsupportedPlatform(t *testing.T) {
	if _, err := NewCapture(Config{}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("NewCapture on unsupported platform: got %v, want ErrUnsupported", err)
	}
	if _, err := ListDevices(); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("ListDevices on unsupported platform: got %v, want ErrUnsupported", err)
	}
}
