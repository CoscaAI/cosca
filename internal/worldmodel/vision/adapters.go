package vision

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/CoscaAI/cosca/internal/worldmodel"
)

// ──────────────────────────────────────────────────────────────
// CLIP Adapter
// ──────────────────────────────────────────────────────────────

// ClipConfig configures the CLIP adapter.
type ClipConfig struct {
	Model  string `json:"model"`   // "ViT-B/32", "ViT-L/14"
	Device string `json:"device"`  // "cpu", "cuda", "mps"
	Script string `json:"script"`  // path to Python script
}

// ClipAdapter wraps OpenAI CLIP for classification and embeddings.
type ClipAdapter struct {
	config ClipConfig
}

// NewClipAdapter creates a CLIP adapter.
func NewClipAdapter(config ClipConfig) *ClipAdapter {
	if config.Script == "" {
		config.Script = "adapters/vision/clip.py"
	}
	return &ClipAdapter{config: config}
}

// Classify returns the most likely class from candidates.
func (a *ClipAdapter) Classify(ctx context.Context, frame []byte, candidates []string) (string, float64, error) {
	payload, _ := json.Marshal(map[string]any{
		"image":      frame,
		"candidates": candidates,
	})

	data, err := runSubprocess(ctx, a.config.Script, subprocessRequest{
		Command: "classify",
		Payload: payload,
	}, 30*time.Second)
	if err != nil {
		return "", 0, fmt.Errorf("clip classify: %w", err)
	}

	var result struct {
		Label      string  `json:"label"`
		Confidence float64 `json:"confidence"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", 0, err
	}

	return result.Label, result.Confidence, nil
}

// Embed returns a dense feature vector for the frame.
func (a *ClipAdapter) Embed(ctx context.Context, frame []byte) ([]float32, error) {
	payload, _ := json.Marshal(map[string]any{
		"image": frame,
	})

	data, err := runSubprocess(ctx, a.config.Script, subprocessRequest{
		Command: "embed",
		Payload: payload,
	}, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("clip embed: %w", err)
	}

	var result struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result.Embedding, nil
}

// ──────────────────────────────────────────────────────────────
// SAM2 Adapter
// ──────────────────────────────────────────────────────────────

// SAMConfig configures the SAM2 adapter.
type SAMConfig struct {
	Model  string `json:"model"`   // "sam2_hiera_large"
	Device string `json:"device"`  // "cpu", "cuda"
	Script string `json:"script"`  // path to Python script
}

// SAMAdapter wraps Meta SAM2 for prompt-based segmentation.
type SAMAdapter struct {
	config SAMConfig
}

// NewSAMAdapter creates a SAM2 adapter.
func NewSAMAdapter(config SAMConfig) *SAMAdapter {
	if config.Script == "" {
		config.Script = "adapters/vision/sam.py"
	}
	return &SAMAdapter{config: config}
}

// Segment finds objects matching a text prompt and returns masks.
func (a *SAMAdapter) Segment(ctx context.Context, frame []byte, prompt string) ([]worldmodel.Mask, error) {
	payload, _ := json.Marshal(map[string]any{
		"image":  frame,
		"prompt": prompt,
	})

	data, err := runSubprocess(ctx, a.config.Script, subprocessRequest{
		Command: "segment",
		Payload: payload,
	}, 60*time.Second)
	if err != nil {
		return nil, fmt.Errorf("sam segment: %w", err)
	}

	var result struct {
		Masks []struct {
			BoundingBox worldmodel.AABB `json:"bounding_box"`
			Area        int            `json:"area"`
			Label       string         `json:"label"`
		} `json:"masks"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	masks := make([]worldmodel.Mask, len(result.Masks))
	for i, m := range result.Masks {
		masks[i] = worldmodel.Mask{
			BoundingBox: m.BoundingBox,
			Area:        m.Area,
			Label:       m.Label,
		}
	}

	return masks, nil
}

// ──────────────────────────────────────────────────────────────
// GroundingDINO Adapter
// ──────────────────────────────────────────────────────────────

// GroundingConfig configures the GroundingDINO adapter.
type GroundingConfig struct {
	Model  string `json:"model"`   // "groundingdino_swint"
	Device string `json:"device"`  // "cpu", "cuda"
	Script string `json:"script"`  // path to Python script
}

// GroundingAdapter wraps GroundingDINO for text-prompted detection.
type GroundingAdapter struct {
	config GroundingConfig
}

// NewGroundingAdapter creates a GroundingDINO adapter.
func NewGroundingAdapter(config GroundingConfig) *GroundingAdapter {
	if config.Script == "" {
		config.Script = "adapters/vision/groundingdino.py"
	}
	return &GroundingAdapter{config: config}
}

// Detect finds objects in a frame using automatic class detection.
func (a *GroundingAdapter) Detect(ctx context.Context, frame []byte) ([]worldmodel.Detection, error) {
	payload, _ := json.Marshal(map[string]any{
		"image": frame,
	})

	data, err := runSubprocess(ctx, a.config.Script, subprocessRequest{
		Command: "detect",
		Payload: payload,
	}, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("groundingdino detect: %w", err)
	}

	var result struct {
		Detections []struct {
			BoundingBox worldmodel.AABB `json:"bounding_box"`
			Label       string         `json:"label"`
			Confidence  float64        `json:"confidence"`
		} `json:"detections"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	detections := make([]worldmodel.Detection, len(result.Detections))
	for i, d := range result.Detections {
		detections[i] = worldmodel.Detection{
			BoundingBox: d.BoundingBox,
			Label:       d.Label,
			Confidence:  d.Confidence,
		}
	}

	return detections, nil
}

// ──────────────────────────────────────────────────────────────
// Depth Anything V2 Adapter
// ──────────────────────────────────────────────────────────────

// DepthConfig configures the Depth Anything V2 adapter.
type DepthConfig struct {
	Model  string `json:"model"`   // "depth_anything_v2_vitl"
	Device string `json:"device"`  // "cpu", "cuda"
	Script string `json:"script"`  // path to Python script
}

// DepthAdapter wraps Depth Anything V2 for monocular depth estimation.
type DepthAdapter struct {
	config DepthConfig
}

// NewDepthAdapter creates a Depth Anything V2 adapter.
func NewDepthAdapter(config DepthConfig) *DepthAdapter {
	if config.Script == "" {
		config.Script = "adapters/vision/depth.py"
	}
	return &DepthAdapter{config: config}
}

// EstimateDepth returns a per-pixel depth map in meters.
func (a *DepthAdapter) EstimateDepth(ctx context.Context, frame []byte) ([][]float32, error) {
	payload, _ := json.Marshal(map[string]any{
		"image": frame,
	})

	data, err := runSubprocess(ctx, a.config.Script, subprocessRequest{
		Command: "depth",
		Payload: payload,
	}, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("depth estimate: %w", err)
	}

	var result struct {
		DepthMap [][]float32 `json:"depth_map"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result.DepthMap, nil
}
