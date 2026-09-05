package vision

import (
	"fmt"
	"math"

	"github.com/yalue/onnxruntime_go"

	"github.com/CoscaAI/cosca/internal/worldmodel"
)

// ──────────────────────────────────────────────────────────────
// Post-processing (ONNX outputs → worldmodel types)
// ──────────────────────────────────────────────────────────────
//
// Each decoder reads the exported model's output tensors by NAME (heuristic
// substring match, case-insensitive), so differently-named exports are still
// tolerated. If the expected output is absent, the decoder returns an empty
// result (degraded) rather than panicking — the pipeline keeps running.

// decodeDetectionOutput turns GroundingDINO-style (boxes + scores/logits)
// outputs into worldmodel.Detection. Boxes are expected in normalized
// cxcywh order [x_center, y_center, width, height] in [0,1], scaled to the
// image; scores in [0,1] after sigmoid/softmax. This is the most common
// exporter convention; per-export quirks are documented in MODELS.md.
func decodeDetectionOutput(values []onnxruntime_go.Value, outs []onnxruntime_go.InputOutputInfo, labels []string, threshold float32) ([]worldmodel.Detection, error) {
	boxes := floatOutput(values, outs, "box")
	scores := floatOutput(values, outs, "score")
	if scores == nil {
		// Some exports name the class head "logits"; treat it as scores.
		logits := floatOutput(values, outs, "logit")
		if logits != nil {
			scores = sigmoid(logits)
		}
	}
	if boxes == nil || scores == nil {
		return nil, fmt.Errorf("%w: GroundingDINO box/score outputs not found", ErrModelUnavailable)
	}
	// Resolve the box score shape to detect num_queries. A boxes tensor of
	// [1, q, 4] has q = len/4; scores [1, q, c] or [1, q] (or [q]).
	q := len(boxes) / 4
	detections := make([]worldmodel.Detection, 0, q)
	for i := 0; i < q; i++ {
		// score at this query: if scores is [1,q,c], take the max class; if
		// [1,q] or [q], take the scalar.
		score := queryScore(scores, q, i)
		if score < threshold {
			continue
		}
		cx := boxes[4*i+0]
		cy := boxes[4*i+1]
		bw := boxes[4*i+2]
		bh := boxes[4*i+3]
		if bw <= 0 || bh <= 0 {
			continue
		}
		x1 := cx - bw/2
		y1 := cy - bh/2
		x2 := cx + bw/2
		y2 := cy + bh/2
		label := "object"
		if len(labels) > 0 {
			label = labels[i%len(labels)]
		}
		detections = append(detections, worldmodel.Detection{
			BoundingBox: worldmodel.AABB{
				Min: worldmodel.Vec3{X: float64(x1), Y: float64(y1), Z: 0},
				Max: worldmodel.Vec3{X: float64(x2), Y: float64(y2), Z: 0},
			},
			Label:      label,
			Confidence: float64(score),
		})
	}
	return detections, nil
}

// decodeGroundingOutput post-processes the GroundingDINO text-prompted export
// (onnx-community/grounding-dino-tiny-ONNX) whose outputs are:
//
//	logits     [1, num_queries, num_text_tokens]  (class logits per query per text token)
//	pred_boxes [1, num_queries, 4]                (cxcywh, normalized [0,1])
//
// For every query the score is sigmoid(max over the prompt's token logits,
// ignoring [CLS]/[SEP]/delimiter/padding tokens) and the label is the prompt
// phrase owning the argmax token (via tokenToPhrase). Boxes are converted from
// cxcywh→xyxy and scaled to the original image pixels when imgW/imgH>0, else
// left normalized. A confidence-thresholded, IoU-based NMS then dedupes
// overlapping queries.
func decodeGroundingOutput(values []onnxruntime_go.Value, outs []onnxruntime_go.InputOutputInfo, phrases []string, tokenToPhrase []int32, threshold float32, imgW, imgH int) ([]worldmodel.Detection, error) {
	logits := floatOutput(values, outs, "logit")
	boxes := floatOutput(values, outs, "box")
	if logits == nil || boxes == nil {
		return nil, fmt.Errorf("%w: GroundingDINO logits/pred_boxes outputs not found", ErrModelUnavailable)
	}
	nq := len(boxes) / 4
	if nq <= 0 {
		return nil, fmt.Errorf("%w: GroundingDINO empty pred_boxes", ErrModelUnavailable)
	}
	tokLen := len(logits) / nq
	if tokLen <= 0 {
		return nil, fmt.Errorf("%w: GroundingDINO empty logits", ErrModelUnavailable)
	}

	detections := make([]worldmodel.Detection, 0, nq)
	for i := 0; i < nq; i++ {
		score, label, ok := queryScoreFromLogits(logits, i, tokLen, tokenToPhrase, phrases)
		if !ok || score < threshold {
			continue
		}
		cx := boxes[4*i+0]
		cy := boxes[4*i+1]
		bw := boxes[4*i+2]
		bh := boxes[4*i+3]
		if bw <= 0 || bh <= 0 {
			continue
		}
		x1 := cx - bw/2
		y1 := cy - bh/2
		x2 := cx + bw/2
		y2 := cy + bh/2
		if imgW > 0 && imgH > 0 {
			x1 *= float32(imgW)
			x2 *= float32(imgW)
			y1 *= float32(imgH)
			y2 *= float32(imgH)
		}
		detections = append(detections, worldmodel.Detection{
			BoundingBox: worldmodel.AABB{
				Min: worldmodel.Vec3{X: float64(x1), Y: float64(y1), Z: 0},
				Max: worldmodel.Vec3{X: float64(x2), Y: float64(y2), Z: 0},
			},
			Label:      label,
			Confidence: float64(score),
		})
	}
	return nmsDetections(detections, 0.5), nil
}

// queryScoreFromLogits returns the (score, label) for query q by taking the
// sigmoid of the max logit over the prompt's real-token positions. ok is false
// when no prompt token is available for that query.
func queryScoreFromLogits(logits []float32, q, tokLen int, tokenToPhrase []int32, phrases []string) (float32, string, bool) {
	best := float32(-1e9)
	bestTok := -1
	start := q * tokLen
	for k := 0; k < tokLen; k++ {
		if k >= len(tokenToPhrase) || tokenToPhrase[k] < 0 {
			continue
		}
		v := logits[start+k]
		if v > best {
			best = v
			bestTok = k
		}
	}
	if bestTok < 0 {
		return 0, "", false
	}
	score := float32(1.0 / (1.0 + math.Exp(-float64(best))))
	label := "object"
	if int(tokenToPhrase[bestTok]) >= 0 && int(tokenToPhrase[bestTok]) < len(phrases) {
		label = phrases[tokenToPhrase[bestTok]]
	}
	return score, label, true
}

// nmsDetections greedily keeps highest-confidence detections, suppressing
// lower ones whose IoU with a kept box exceeds iouThresh. It sorts descending
// by confidence first.
func nmsDetections(in []worldmodel.Detection, iouThresh float64) []worldmodel.Detection {
	if len(in) <= 1 {
		return in
	}
	// Sort descending by confidence (stable-ish via simple insertion).
	for i := 1; i < len(in); i++ {
		for j := i; j > 0 && in[j].Confidence > in[j-1].Confidence; j-- {
			in[j], in[j-1] = in[j-1], in[j]
		}
	}
	keep := make([]worldmodel.Detection, 0, len(in))
	for _, d := range in {
		dup := false
		for _, k := range keep {
			if iouAABB(d.BoundingBox, k.BoundingBox) > iouThresh {
				dup = true
				break
			}
		}
		if !dup {
			keep = append(keep, d)
		}
	}
	return keep
}

// iouAABB computes the Intersection-over-Union of two axis-aligned boxes.
func iouAABB(a, b worldmodel.AABB) float64 {
	ix1 := math.Max(a.Min.X, b.Min.X)
	iy1 := math.Max(a.Min.Y, b.Min.Y)
	ix2 := math.Min(a.Max.X, b.Max.X)
	iy2 := math.Min(a.Max.Y, b.Max.Y)
	iw := ix2 - ix1
	ih := iy2 - iy1
	if iw <= 0 || ih <= 0 {
		return 0
	}
	inter := iw * ih
	areaA := (a.Max.X - a.Min.X) * (a.Max.Y - a.Min.Y)
	areaB := (b.Max.X - b.Min.X) * (b.Max.Y - b.Min.Y)
	return inter / (areaA + areaB - inter)
}

// decodeMaskOutput turns a SAM2-style binary/logit mask output into a list of
// worldmodel.Mask. Only the first mask channel is surfaced (per prompt).
func decodeMaskOutput(values []onnxruntime_go.Value, outs []onnxruntime_go.InputOutputInfo, label string) ([]worldmodel.Mask, error) {
	maskData := floatOutput(values, outs, "mask")
	if maskData == nil {
		// SAM2 exports usually name the mask "output_0" / "masks"; fall back.
		maskData = firstFloatOutput(values, outs)
	}
	if len(maskData) == 0 {
		return nil, fmt.Errorf("%w: SAM2 mask output not found", ErrModelUnavailable)
	}
	// Compute a coarse area from the binarised mask (pixels with value>0.5).
	area := 0
	for _, v := range maskData {
		if v > 0.5 {
			area++
		}
	}
	mask := worldmodel.Mask{
		Area:  area,
		Label: label,
	}
	return []worldmodel.Mask{mask}, nil
}

// decodeDepthOutput reshapes a [H,W] (or [1,H,W]) depth tensor into a [][]float32
// map. A single depth channel is assumed (disparity/normalised in [0,1]).
func decodeDepthOutput(values []onnxruntime_go.Value, outs []onnxruntime_go.InputOutputInfo, h, w int) ([][]float32, error) {
	data := floatOutput(values, outs, "depth")
	if data == nil {
		data = firstFloatOutput(values, outs)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: depth output not found", ErrModelUnavailable)
	}
	// Decide the actual grid. Prefer the depth tensor shape if available.
	if shape, ok := depthShape(values, outs); ok {
		if len(shape) == 4 && shape[2] > 0 && shape[3] > 0 {
			h, w = int(shape[2]), int(shape[3])
		} else if len(shape) == 3 && shape[1] > 0 && shape[2] > 0 {
			h, w = int(shape[1]), int(shape[2])
		}
	}
	if h <= 0 || w <= 0 || len(data) < h*w {
		return nil, fmt.Errorf("%w: depth output size (%d) does not match %dx%d", ErrModelUnavailable, len(data), h, w)
	}
	out := make([][]float32, h)
	for y := 0; y < h; y++ {
		out[y] = make([]float32, w)
		copy(out[y], data[y*w:(y+1)*w])
	}
	return out, nil
}

// depthShape returns the 2D grid dims for a depth-like output tensor, if any.
func depthShape(values []onnxruntime_go.Value, outs []onnxruntime_go.InputOutputInfo) ([]int64, bool) {
	for i, o := range outs {
		if containsFold(o.Name, "depth") {
			if values[i] != nil {
				return values[i].GetShape(), true
			}
		}
	}
	for _, v := range values {
		if v != nil {
			s := v.GetShape()
			if len(s) >= 2 {
				return s, true
			}
		}
	}
	return nil, false
}

// ──────────────────────────────────────────────────────────────
// Vector helpers
// ──────────────────────────────────────────────────────────────

func sigmoid(v []float32) []float32 {
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = 1.0 / (1.0 + float32(math.Exp(-float64(x))))
	}
	return out
}

// queryScore returns the confidence for query i given a scores tensor whose
// length is a multiple of q. Handles [1,q,c], [1,q] and [q] layouts.
func queryScore(scores []float32, q, i int) float32 {
	n := len(scores)
	if n <= 0 {
		return 0
	}
	if n == q {
		return scores[i]
	}
	if n == q*2 {
		// [2,q] or [1,q,2]
		return scores[q+i]
	}
	// Assume [1,q,c] or [q,c]: take the max over the class dim per query.
	if c := n / q; c > 0 && q > 0 {
		best := float32(-1e9)
		for k := 0; k < c; k++ {
			v := scores[i*c+k]
			if v > best {
				best = v
			}
		}
		return best
	}
	// Fallback: clamp to the last available score.
	return scores[i%n]
}


