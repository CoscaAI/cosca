package perception

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/draw"
	"image/png"

	"github.com/kbinani/screenshot"
)

// ──────────────────────────────────────────────────────────────
// Screen capture (Windows / cross-platform)
// ──────────────────────────────────────────────────────────────
//
// The screen-capture is the "missing piece" of the Perception Loop. It is the
// source that turns "capability pontual" into "presença contínua": the Cosca
// looks at the real screen and feeds the frame to the vision pipeline.
//
// Choice — library vs. native GDI:
//
//	`github.com/kbinani/screenshot` (pure Go, multi-platform, popular).
//
// Decision: USE THE LIBRARY. Rationale:
//   - It is pure Go and cross-platform (Windows GDI, Linux X11/wayland,
//     macOS) — no cgo, no Python, no third-party engine.
//   - It is the recommended option in the task and is battle-tested on
//     Windows, where our runtime lives.
//   - Native GDI/BitBlt via syscall was the "total sovereignty" alternative,
//     but it is far more code (DIBSECTION, BITMAPINFOHEADER, GetDC/SelectObject
//     plumbing, pixel-format wrangling) and far more fragile than a maintained
//     library. The sovereignty concern (no hard dependency) is mitigated by
//     the injectable Captor seam (see capture.go / perception.go) and the
//     graceful-degradation contract: when the library cannot capture (e.g. a
//     headless / session-0 context), the loop emits a Degraded state instead
//     of crashing.
//
// Note: `screenshot` captures the active desktop/display. It works for the
// primary use-case (a desk with a real Windows session). RDP-disconnected or
// locked sessions degrade to an empty/failed capture — handled gracefully.

// Captor captures a screen frame (PNG/JPEG bytes + dimensions) for the
// perception loop. It is the injectable seam so tests can supply a
// deterministic fake without touching the real screen or the OS capture API.
type Captor interface {
	// Capture returns the encoded frame bytes (PNG or JPEG) plus its width and
	// height. On failure it returns a non-nil error; the caller degrades
	// gracefully instead of crashing.
	Capture(ctx context.Context) ([]byte, int, int, error)
}

// ──────────────────────────────────────────────────────────────
// Display stitching
// ──────────────────────────────────────────────────────────────

// CaptureScreen captures the whole virtual desktop (all active displays,
// composited) and returns PNG bytes plus the total width/height. It is the
// default captor used by the production Perception Loop.
func CaptureScreen(ctx context.Context) ([]byte, int, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, 0, err
	}

	n := screenshot.NumActiveDisplays()
	if n < 1 {
		return nil, 0, 0, fmt.Errorf("no active display to capture")
	}

	// Resolve every display's bounds and compute the union rectangle that
	// spans the whole virtual desktop (handles multi-monitor layouts).
	bounds := make([]image.Rectangle, n)
	for i := 0; i < n; i++ {
		bounds[i] = screenshot.GetDisplayBounds(i)
	}
	union := bounds[0]
	for i := 1; i < n; i++ {
		union = union.Union(bounds[i])
	}
	if union.Dx() <= 0 || union.Dy() <= 0 {
		return nil, 0, 0, fmt.Errorf("invalid virtual screen bounds %v", union)
	}

	full := image.NewRGBA(union)
	for i, b := range bounds {
		if err := ctx.Err(); err != nil {
			return nil, 0, 0, err
		}
		img, err := screenshot.CaptureRect(b)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("capture display %d: %w", i, err)
		}
		// Place this display's image at its offset within the union.
		offset := image.Pt(b.Min.X-union.Min.X, b.Min.Y-union.Min.Y)
		draw.Draw(
			full,
			image.Rect(offset.X, offset.Y, offset.X+img.Bounds().Dx(), offset.Y+img.Bounds().Dy()),
			img,
			image.Point{},
			draw.Src,
		)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, full); err != nil {
		return nil, 0, 0, fmt.Errorf("encode screen capture: %w", err)
	}
	return buf.Bytes(), union.Dx(), union.Dy(), nil
}

// ──────────────────────────────────────────────────────────────
// ScreenCaptor — the default Captor backed by CaptureScreen.
// ──────────────────────────────────────────────────────────────

// ScreenCaptor captures the real screen. Zero-value is ready to use.
type ScreenCaptor struct{}

// Capture implements Captor.
func (ScreenCaptor) Capture(ctx context.Context) ([]byte, int, int, error) {
	return CaptureScreen(ctx)
}
