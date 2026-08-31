package vision

import (
	"context"
	"image"
	"strings"
	"testing"
)

func encodeTestPNG(t *testing.T) []byte {
	t.Helper()
	return encodePNG(t, image.NewRGBA(image.Rect(0, 0, 2, 2)))
}

func TestDetectImageContentType(t *testing.T) {
	pngData := encodeTestPNG(t)
	if got := DetectImageContentType(pngData); got != "image/png" {
		t.Errorf("PNG sniff = %q, want image/png", got)
	}
	// Minimal JPEG magic (FF D8 FF).
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}
	if got := DetectImageContentType(jpegData); got != "image/jpeg" {
		t.Errorf("JPEG sniff = %q, want image/jpeg", got)
	}
	if got := DetectImageContentType([]byte("hello")); got != "" {
		t.Errorf("non-image sniff = %q, want empty", got)
	}
}

func TestIsImageContentType(t *testing.T) {
	for _, ct := range []string{"image/png", "image/jpeg", "IMAGE/PNG", "image/png;base64"} {
		if !IsImageContentType(ct) {
			t.Errorf("IsImageContentType(%q) = false, want true", ct)
		}
	}
	for _, ct := range []string{"text/plain", "application/json", ""} {
		if IsImageContentType(ct) {
			t.Errorf("IsImageContentType(%q) = true, want false", ct)
		}
	}
}

func TestDecodeDataURI(t *testing.T) {
	data, ct, err := DecodeDataURI("data:image/png;base64,iVBORw0KGgo=")
	if err != nil {
		t.Fatalf("DecodeDataURI: %v", err)
	}
	if ct != "image/png" {
		t.Errorf("content-type = %q, want image/png", ct)
	}
	if len(data) == 0 {
		t.Error("decoded empty data")
	}
	if _, _, err := DecodeDataURI("data:text/plain;base64,aGk="); err == nil {
		t.Error("DecodeDataURI should reject non-image data URI")
	}
	if _, _, err := DecodeDataURI("not a uri"); err == nil {
		t.Error("DecodeDataURI should reject non-URIs")
	}
}

func TestDetectImageAndRunVision_DegradesWithoutModel(t *testing.T) {
	// No .onnx models are checked in, so this runs the pipeline end-to-end and
	// asserts it DEGRADES gracefully (returns an observation + warnings, not a
	// panic / not a hard failure). This is the "best effort" contract.
	ctx := context.Background()
	data := encodeTestPNG(t)

	obs, err := DetectImageAndRunVision(ctx, data, "image/png")
	if err != nil {
		t.Fatalf("DetectImageAndRunVision should not fail on missing models: %v", err)
	}
	if obs == nil {
		t.Fatal("observation is nil")
	}
	if len(obs.Warnings) == 0 {
		t.Error("expected degradation warnings (no models downloaded)")
	}
	if s := obs.SummaryText(); !strings.Contains(s, "Vision observation") {
		t.Errorf("SummaryText missing header: %q", s)
	}
}

func TestDetectImageAndRunVision_RejectsNonImage(t *testing.T) {
	_, err := DetectImageAndRunVision(context.Background(), []byte("not an image"), "text/plain")
	if err == nil {
		t.Fatal("DetectImageAndRunVision should reject non-image input")
	}
}
