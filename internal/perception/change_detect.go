package perception

import (
	"bytes"
	"image"
	_ "image/jpeg" // register JPEG so image.Decode accepts captured JPEG frames
	_ "image/png"  // register PNG so image.Decode accepts captured PNG frames
)

// ──────────────────────────────────────────────────────────────
// Change detection gate (gocv-learned: only run vision if the screen changed)
// ──────────────────────────────────────────────────────────────
//
// The Don mined `cmd/motion-detect` (BackgroundSubtractorMOG2 + Threshold +
// Dilate + FindContours) and distilled the key learning: run the heavy vision
// inference (CLIP / GroundingDINO / Depth) only when the screen actually
// CHANGED. When the screen is static, COSCA keeps "perceiving" without
// recomputing the universe — cheap presence.
//
// This comparator is pure Go (no OpenCV). It decodes the captured frame
// (PNG/JPEG), downsamples it to a small NxN grid (stride sampling, so a huge
// screenshot is never compared pixel-by-pixel), and computes the **mean
// absolute delta** between the current frame and the reference (the last frame
// that was actually *processed*). If meanDelta exceeds the threshold → the
// screen changed → let the vision pipeline run. Small/noisy deltas below the
// threshold are ignored (mirrors the gocv threshold+dilate noise rejection).
//
// Degradation contract: if the frame cannot be decoded, the detector assumes a
// change (changed = true) so the loop degrades safely and still runs vision
// rather than silently staying blind.

// sampleSize is the target grid resolution for the downsample. 16x16 → 256
// sampled cells. It is enough to catch layout / region / UI changes while
// staying cheap (a few hundred pixel reads, not millions).
const sampleSize = 16

// DefaultChangeDetectionThreshold is the default mean-absolute-delta (in the
// normalised [0,1] range) above which a frame is considered a real change.
// Chosen in the 0.02–0.05 window recommended by the motion-detect learnings:
// sensitive enough to catch meaningful change, tolerant enough to ignore
// anti-aliasing / cursor flicker / compression noise.
const DefaultChangeDetectionThreshold = 0.02

// ChangeDetector compares an incoming captured frame against a reference frame
// and reports whether the screen changed beyond the configured threshold.
//
// It stores only the *downsampled* reference (a few hundred bytes), never the
// full PNG, so a long static period costs memory proportional to the small
// grid, not the screen resolution.
type ChangeDetector struct {
	// threshold is the normalised mean-absolute-delta above which a change is
	// detected (in [0,1]).
	threshold float64
	// reference is the downsampled RGB grid of the last frame that was actually
	// processed. It is only replaced when a change is detected, so slow drift
	// eventually crosses the threshold instead of being silently absorbed.
	reference []uint8
}

// newChangeDetector builds a ChangeDetector with the given threshold.
func newChangeDetector(threshold float64) *ChangeDetector {
	if threshold <= 0 {
		threshold = DefaultChangeDetectionThreshold
	}
	return &ChangeDetector{threshold: threshold}
}

// NewChangeDetector builds a ChangeDetector with the given normalised
// mean-absolute-delta threshold (<=0 falls back to the default). It is the
// exported constructor so other packages (e.g. the perception-by-action tools in
// internal/visionact) can reuse the same pure-Go change-detection comparator
// without re-implementing the downsample + mean-abs-delta logic.
func NewChangeDetector(threshold float64) *ChangeDetector {
	return newChangeDetector(threshold)
}

// Evaluate compares `frame` (encoded PNG/JPEG bytes) against the stored
// reference and returns true when the screen meaningfully changed (or when
// there is no reference yet, i.e. the first frame).
//
// On a detected change (or the first frame) the reference is updated to the
// current downsample; on a non-change the reference is left untouched so that
// slow, incremental drift accumulates until it crosses the threshold.
//
// Decode failure is treated as a change (graceful degradation: run vision
// rather than risk staying blind).
func (d *ChangeDetector) Evaluate(frame []byte, _, _ int) bool {
	img, _, err := image.Decode(bytes.NewReader(frame))
	if err != nil || img == nil {
		// Cannot tell whether the screen changed → assume change (safe default).
		d.reference = nil
		return true
	}

	samp := downsample(img, sampleSize, sampleSize)
	if len(samp) == 0 {
		d.reference = nil
		return true
	}

	// First frame: no reference yet → must process it.
	if len(d.reference) == 0 {
		d.reference = samp
		return true
	}

	delta := meanAbsDelta(d.reference, samp)
	if delta > d.threshold {
		// Meaningful change → adapt the reference to the new scene.
		d.reference = samp
		return true
	}

	// No meaningful change → keep the stale reference so cumulative drift still
	// trips the gate once it is big enough (mirrors MOG2 not adapting during
	// static periods).
	return false
}

// downsample samples `img` into a rows×cols RGB grid. Each cell is the average
// of a small 3×3 patch centred on the cell, which rejects single-pixel noise
// the way the gocv Dilate did after Threshold. Returns a flat []uint8 in RGB
// order (length = rows*cols*3), or nil on a degenerate image.
func downsample(img image.Image, cols, rows int) []uint8 {
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return nil
	}
	width, height := b.Dx(), b.Dy()
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	out := make([]uint8, cols*rows*3)
	for r := 0; r < rows; r++ {
		y := b.Min.Y + r*height/rows + height/(2*rows)
		for c := 0; c < cols; c++ {
			x := b.Min.X + c*width/cols + width/(2*cols)
			var rs, gs, bs, n int
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					px, py := x+dx, y+dy
					if px < b.Min.X || px >= b.Max.X || py < b.Min.Y || py >= b.Max.Y {
						continue
					}
					cr, cg, cb, _ := img.At(px, py).RGBA()
					rs += int(cr >> 8)
					gs += int(cg >> 8)
					bs += int(cb >> 8)
					n++
				}
			}
			if n == 0 {
				n = 1
			}
			idx := (r*cols + c) * 3
			out[idx] = uint8(rs / n)
			out[idx+1] = uint8(gs / n)
			out[idx+2] = uint8(bs / n)
		}
	}
	return out
}

// meanAbsDelta computes the normalised mean absolute difference between two
// equally-sized RGB grids (in [0,1]). Different lengths are treated as a full
// change (1.0).
func meanAbsDelta(a, b []uint8) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 1.0
	}
	var sum int
	for i := range a {
		d := int(a[i]) - int(b[i])
		if d < 0 {
			d = -d
		}
		sum += d
	}
	return float64(sum) / float64(len(a)) / 255.0
}
