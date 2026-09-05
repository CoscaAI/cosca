package vision

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
	"testing"
	"time"
)

// ──────────────────────────────────────────────────────────────
// ONNX adapters — graceful degradation (no Python, no model required)
// ──────────────────────────────────────────────────────────────
//
// These tests exercise the "best-effort" contract: when the .onnx model file is
// not present on disk, the adapter must degrade with a clear (non-fatal) error
// and never crash/call the C runtime. Since no real models are checked in, the
// exact ONNX inference path is not exercised here (it would t.Skip anyway);
// these verify the loader + degradation path is wired correctly.

func TestClipAdapterClassify_ModelMissing(t *testing.T) {
	a := NewClipAdapter(ClipConfig{ModelPath: "testdata/does-not-exist_clip.onnx"})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, _, err := a.Classify(ctx, []byte("frame"), []string{"cat", "dog"})
	if err == nil {
		t.Fatal("Classify should degrade with an error on missing model, got nil")
	}
	assertDegradeErr(t, err)
}

func TestClipAdapterEmbed_ModelMissing(t *testing.T) {
	a := NewClipAdapter(ClipConfig{ModelPath: "testdata/does-not-exist_clip.onnx"})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := a.Embed(ctx, []byte("frame"))
	if err == nil {
		t.Fatal("Embed should degrade with an error on missing model, got nil")
	}
	assertDegradeErr(t, err)
}

func TestSAMAdapterSegment_ModelMissing(t *testing.T) {
	a := NewSAMAdapter(SAMConfig{ModelPath: "testdata/does-not-exist_sam.onnx"})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := a.Segment(ctx, []byte("frame"), "esteira")
	if err == nil {
		t.Fatal("Segment should degrade with an error on missing model, got nil")
	}
	assertDegradeErr(t, err)
}

func TestGroundingAdapterDetect_ModelMissing(t *testing.T) {
	a := NewGroundingAdapter(GroundingConfig{ModelPath: "testdata/does-not-exist_grounding.onnx"})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := a.Detect(ctx, []byte("frame"))
	if err == nil {
		t.Fatal("Detect should degrade with an error on missing model, got nil")
	}
	assertDegradeErr(t, err)
}

func TestDepthAdapterEstimateDepth_ModelMissing(t *testing.T) {
	a := NewDepthAdapter(DepthConfig{ModelPath: "testdata/does-not-exist_depth.onnx"})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := a.EstimateDepth(ctx, []byte("frame"))
	if err == nil {
		t.Fatal("EstimateDepth should degrade with an error on missing model, got nil")
	}
	assertDegradeErr(t, err)
}

func assertDegradeErr(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, ErrModelNotFound) && !errors.Is(err, ErrModelUnavailable) {
		t.Errorf("expected a graceful model error (ErrModelNotFound/ErrModelUnavailable), got: %v", err)
	}
}

// ──────────────────────────────────────────────────────────────
// Model path resolution
// ──────────────────────────────────────────────────────────────

func TestFilenameForModel(t *testing.T) {
	cases := map[string]string{
		"ViT-B/32": ClipModelFile,
		"clip":     ClipModelFile,
		"sam2":     SAMModelFile,
		"groundingdino": GroundingModelFile,
		"depth":    DepthModelFile,
		"custom":   "custom.onnx",
	}
	for in, want := range cases {
		if got := filenameForModel(in); got != want {
			t.Errorf("filenameForModel(%q) = %q, want %q", in, got, want)
		}
	}
}

// ──────────────────────────────────────────────────────────────
// Image preprocessing helpers
// ──────────────────────────────────────────────────────────────

func TestDecodeImage_PNG(t *testing.T) {
	buf := encodePNG(t, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	img, err := decodeImage(buf)
	if err != nil {
		t.Fatalf("decodeImage: %v", err)
	}
	if img.Bounds().Dx() != 4 || img.Bounds().Dy() != 4 {
		t.Fatalf("unexpected decoded size: %v", img.Bounds())
	}
}

func TestDecodeImage_Unsupported(t *testing.T) {
	_, err := decodeImage([]byte("not an image"))
	if err == nil {
		t.Fatal("decodeImage should error on unsupported/unparseable bytes")
	}
}

func TestResizeBilinear(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 2, 2))
	src.Set(0, 0, color.RGBA{255, 0, 0, 255})
	src.Set(1, 0, color.RGBA{0, 255, 0, 255})
	src.Set(0, 1, color.RGBA{0, 0, 255, 255})
	src.Set(1, 1, color.RGBA{255, 255, 255, 255})

	dst := resizeBilinear(src, 4, 4)
	if dst.Bounds().Dx() != 4 || dst.Bounds().Dy() != 4 {
		t.Fatalf("resize to 4x4 failed: %v", dst.Bounds())
	}
}

func TestToCHWFloat_Shape(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 3, 3))
	data := toCHWFloat(img, 3, 3, imageNetNormalize)
	if len(data) != 3*3*3 {
		t.Fatalf("CHW buffer size = %d, want %d", len(data), 27)
	}
}

func TestSoftmax(t *testing.T) {
	out := softmax([]float32{0, 0})
	if len(out) != 2 {
		t.Fatalf("softmax len = %d, want 2", len(out))
	}
	sum := out[0] + out[1]
	if math.Abs(float64(sum)-1.0) > 1e-4 {
		t.Fatalf("softmax does not sum to 1: %v", out)
	}
}

func TestL2Norm(t *testing.T) {
	v := []float32{3, 4}
	norm := l2norm(v)
	if math.Abs(float64(norm)-5.0) > 1e-4 {
		t.Fatalf("l2norm = %v, want 5", norm)
	}
	if math.Abs(float64(v[0])-0.6) > 1e-4 || math.Abs(float64(v[1])-0.8) > 1e-4 {
		t.Fatalf("normalized vector wrong: %v", v)
	}
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png encode: %v", err)
	}
	return buf.Bytes()
}
