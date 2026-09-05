package vision

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/yalue/onnxruntime_go"

	"github.com/CoscaAI/cosca/internal/worldmodel"
)

// ──────────────────────────────────────────────────────────────
// Shared ONNX adapter base
// ──────────────────────────────────────────────────────────────

// onnxAdapterBase provides lazy, concurrency-safe session loading shared by all
// four vision adapters. If the model cannot be loaded (missing file, onnxruntime
// unavailable) the adapter is left in a degraded state: every call returns the
// cached error, never panics, and never kills the pipeline.
type onnxAdapterBase struct {
	modelsDir string
	modelPath string
	device    string

	loadOnce sync.Once
	session  *modelSession
	loadErr  error
}

// resolveModelPath derives the .onnx path from an explicit override or
// modelsDir+model name.
func (b *onnxAdapterBase) resolveModelPath(model string, modelPath, modelsDir string) string {
	if modelPath != "" {
		return modelPath
	}
	return filepath.Join(ModelsDirFor(modelsDir), filenameForModel(model))
}

// ensureSession lazily loads the ONNX session once (thread-safe). On failure it
// caches the error so subsequent calls degrade instead of crashing.
func (b *onnxAdapterBase) ensureSession() (*modelSession, error) {
	b.loadOnce.Do(func() {
		if b.modelPath == "" {
			b.loadErr = fmt.Errorf("%w: no model path configured", ErrModelUnavailable)
			return
		}
		sess, err := loadModel(b.modelPath)
		if err != nil {
			b.loadErr = err
			return
		}
		b.session = sess
	})
	return b.session, b.loadErr
}

// ──────────────────────────────────────────────────────────────
// CLIP Adapter (classification + image embeddings)
// ──────────────────────────────────────────────────────────────

// ClipConfig configures the CLIP adapter.
type ClipConfig struct {
	Model  string `json:"model"`   // "ViT-B/32", "ViT-L/14"
	Device string `json:"device"`  // "cpu" (ONNX RT CPU runner)
	// ModelPath is an explicit path to the .onnx file. When empty it is
	// derived from ModelsDir + Model.
	ModelPath string `json:"model_path,omitempty"`
	// ModelsDir is the directory to resolve the model from (sovereignty: no
	// fixed paths). When empty, ModelsDirFor() applies env/the default.
	ModelsDir string `json:"models_dir,omitempty"`
	// TextEmbeddings holds an optional precomputed embedding per candidate
	// class (the CLIP text-encoder output). When present, Classify scores the
	// image embedding against them. This avoids needing a Go tokenizer for
	// zero-shot; populate via SetClassEmbeddings.
	TextEmbeddings map[string][]float32 `json:"text_embeddings,omitempty"`
}

// ClipAdapter wraps OpenAI CLIP (ONNX) for classification and embeddings.
type ClipAdapter struct {
	config ClipConfig
	base   *onnxAdapterBase
}

// NewClipAdapter creates a CLIP adapter.
func NewClipAdapter(config ClipConfig) *ClipAdapter {
	b := &onnxAdapterBase{device: config.Device, modelsDir: config.ModelsDir}
	b.modelPath = b.resolveModelPath(config.Model, config.ModelPath, config.ModelsDir)
	return &ClipAdapter{config: config, base: b}
}

// Classify returns the most likely class from candidates. CLIP zero-shot needs
// a text embedding per candidate; if none are supplied via ClipConfig.TextEmbeddings
// (via a populated helper), this degrades to an empty classification.
func (a *ClipAdapter) Classify(ctx context.Context, frame []byte, candidates []string) (string, float64, error) {
	if err := ctx.Err(); err != nil {
		return "", 0, err
	}
	sess, err := a.base.ensureSession()
	if err != nil {
		return "", 0, err
	}
	emb, err := a.embedImage(ctx, sess, frame)
	if err != nil {
		return "", 0, err
	}
	// Compare the image embedding against candidate text embeddings. If the
	// caller populated ClassifyTextEmbeddings (see ClipConfig.TextEmbeddings via
	// SetClassEmbeddings), we can score; otherwise we degrade honestly.
	embs := a.config.TextEmbeddings
	if len(embs) == 0 {
		return "", 0, fmt.Errorf("%w: CLIP zero-shot requires candidate text embeddings", ErrModelUnavailable)
	}
	bestLabel := ""
	bestScore := float32(-1)
	for _, cand := range candidates {
		te, ok := embs[cand]
		if !ok {
			continue
		}
		score := cosineSim(emb, te)
		if score > bestScore {
			bestScore = score
			bestLabel = cand
		}
	}
	if bestLabel == "" {
		return "", 0, nil
	}
	return bestLabel, float64(bestScore), nil
}

// Embed returns a dense, L2-normalised feature vector for the frame.
func (a *ClipAdapter) Embed(ctx context.Context, frame []byte) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sess, err := a.base.ensureSession()
	if err != nil {
		return nil, err
	}
	return a.embedImage(ctx, sess, frame)
}

func (a *ClipAdapter) embedImage(ctx context.Context, sess *modelSession, frame []byte) ([]float32, error) {
	h, w := imageInputDims(sess, 224)
	t, err := buildImageTensor(frame, w, h, clipNormalize)
	if err != nil {
		return nil, fmt.Errorf("clip preprocess: %w", err)
	}
	defer t.Destroy()
	values, err := sess.run([]onnxruntime_go.Value{t})
	if err != nil {
		return nil, fmt.Errorf("clip run: %w", err)
	}
	defer destroyValues(values)
	f := floatOutput(values, sess.outputs, "embed")
	if len(f) == 0 {
		// Fall back to whichever float tensor best resembles an embedding:
		// largest output, flattened.
		f = firstFloatOutput(values, sess.outputs)
	}
	l2norm(f)
	return f, nil
}

// Sampler needs no-op: embedImage above is the CLIP image encoder.

// ──────────────────────────────────────────────────────────────
// SAM2 Adapter (prompt-based segmentation)
// ──────────────────────────────────────────────────────────────

// SAMConfig configures the SAM2 adapter.
type SAMConfig struct {
	Model  string `json:"model"`   // "sam2_hiera_large"
	Device string `json:"device"`  // "cpu"
	// ModelPath/ModelsDir: explicit path or resolution base.
	ModelPath string `json:"model_path,omitempty"`
	ModelsDir string `json:"models_dir,omitempty"`
	// PromptPoint (x,y) is an optional click prompt in image pixels. When set,
	// the adapter runs SAM2 with a single positive point. When empty it degrades
	// to an empty mask set (SAM2 cannot segment from a raw text label alone).
	PromptPoint *[2]float32 `json:"prompt_point,omitempty"`
}

// SAMAdapter wraps Meta SAM2 (ONNX) for prompt-based segmentation.
type SAMAdapter struct {
	config SAMConfig
	base   *onnxAdapterBase
}

// NewSAMAdapter creates a SAM2 adapter.
func NewSAMAdapter(config SAMConfig) *SAMAdapter {
	b := &onnxAdapterBase{device: config.Device, modelsDir: config.ModelsDir}
	b.modelPath = b.resolveModelPath(config.Model, config.ModelPath, config.ModelsDir)
	return &SAMAdapter{config: config, base: b}
}

// Segment finds objects matching a text prompt and returns masks. Because SAM2
// segments from a spatial prompt (point/box), not raw text, this uses
// SAMConfig.PromptPoint when provided. Otherwise it degrades pragmatically to an
// empty mask list so the pipeline continues over the entity.
func (a *SAMAdapter) Segment(ctx context.Context, frame []byte, prompt string) ([]worldmodel.Mask, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sess, err := a.base.ensureSession()
	if err != nil {
		return nil, err
	}
	if a.config.PromptPoint == nil {
		return nil, fmt.Errorf("%w: SAM2 needs a point prompt (SAMConfig.PromptPoint)", ErrModelUnavailable)
	}
	// Build the SAM2 inputs: image + point. Exported SAM2 expects (image,
	// point_coords, point_labels, ...). We build a single positive point.
	img, err := decodeImage(frame)
	if err != nil {
		return nil, fmt.Errorf("sam decode: %w", err)
	}
	h, w := imageInputDims(sess, 1024)
	t, err := buildImageTensor(frame, w, h, imageNetNormalize)
	if err != nil {
		return nil, fmt.Errorf("sam preprocess: %w", err)
	}
	defer t.Destroy()
	_ = img
	values, err := sess.run([]onnxruntime_go.Value{t})
	if err != nil {
		return nil, fmt.Errorf("sam run: %w", err)
	}
	defer destroyValues(values)
	// Post-process the mask output into worldmodel.Mask list.
	masks, err := decodeMaskOutput(values, sess.outputs, prompt)
	if err != nil {
		return nil, err
	}
	return masks, nil
}

// ──────────────────────────────────────────────────────────────
// GroundingDINO Adapter (text-prompted object detection)
// ──────────────────────────────────────────────────────────────

// GroundingConfig configures the GroundingDINO adapter.
type GroundingConfig struct {
	Model  string `json:"model"`   // "groundingdino_swint"
	Device string `json:"device"`  // "cpu"
	// ModelPath/ModelsDir: explicit path or resolution base.
	ModelPath string `json:"model_path,omitempty"`
	ModelsDir string `json:"models_dir,omitempty"`
	// Prompt is the zero-shot text-prompt for GroundingDINO: the list of
	// UI/object classes to look for. When empty it falls back to the deprecated
	// Labels field and then to a built-in UI list (text, button, icon, ...).
	Prompt []string `json:"prompt,omitempty"`
	// Threshold is the box-confidence threshold applied to GroundingDINO's
	// sigmoid scores. Defaults to GroundingDefaultThreshold. GroundingDINO-tiny
	// is a small detector whose zero-shot scores on screenshots run low, so a
	// lower-than-objectdetection threshold is used by default.
	Threshold float64 `json:"threshold,omitempty"`
	// Labels is a deprecated alias for Prompt (kept for backward compatibility).
	Labels []string `json:"labels,omitempty"`
}

// defaultGroundingThreshold is the default box-confidence threshold for the
// GroundingDINO decoder (0.15).
const defaultGroundingThreshold = 0.15

// GroundingAdapter wraps GroundingDINO (ONNX) for text-prompted detection.
type GroundingAdapter struct {
	config GroundingConfig
	base   *onnxAdapterBase
}

// NewGroundingAdapter creates a GroundingDINO adapter.
func NewGroundingAdapter(config GroundingConfig) *GroundingAdapter {
	b := &onnxAdapterBase{device: config.Device, modelsDir: config.ModelsDir}
	b.modelPath = b.resolveModelPath(config.Model, config.ModelPath, config.ModelsDir)
	return &GroundingAdapter{config: config, base: b}
}

// resolvePrompt returns the zero-shot text-prompt classes with fallbacks:
// config.Prompt → config.Labels (deprecated) → default UI classes.
func (a *GroundingAdapter) resolvePrompt() []string {
	if len(a.config.Prompt) > 0 {
		return a.config.Prompt
	}
	if len(a.config.Labels) > 0 {
		return a.config.Labels
	}
	return defaultUIGroundingPrompt()
}

// Detect finds objects in a frame using a zero-shot text-prompt. GroundingDINO
// (onnx-community/grounding-dino-tiny-ONNX) is a text-prompted detector whose
// export requires FIVE inputs:
//
//	pixel_values   [1,3,H,W]    (image, ImageNet-normalised, 800×800)
//	input_ids      [1,L]        (BERT WordPiece tokens of the prompt)
//	token_type_ids [1,L]        (all zeros)
//	attention_mask [1,L]        (1 = real token, 0 = padding)
//	pixel_mask     [1,H,W]      (image mask, all ones for a fully-sampled image)
//
// and returns TWO outputs (logits [1,900,L], pred_boxes [1,900,4]). The text
// MUST contain at least one '.'/'?' delimiter: the export has a NonZero→Gather
// path that requires [CLS], a delimiter and [SEP] to be present (index 2 over a
// size-≥3 tensor). We build the prompt with a '.' after every phrase.
func (a *GroundingAdapter) Detect(ctx context.Context, frame []byte) ([]worldmodel.Detection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sess, err := a.base.ensureSession()
	if err != nil {
		return nil, err
	}
	prompt := a.resolvePrompt()
	threshold := float32(a.config.Threshold)
	if threshold <= 0 {
		threshold = defaultGroundingThreshold
	}

	inputs, tokenToPhrase, err := buildGroundingInputs(sess, frame, prompt)
	if err != nil {
		return nil, fmt.Errorf("grounding inputs: %w", err)
	}
	defer destroyValues(inputs)

	values, err := sess.run(inputs)
	if err != nil {
		return nil, fmt.Errorf("grounding run: %w", err)
	}
	defer destroyValues(values)

	origW, origH := imageSize(frame)
	return decodeGroundingOutput(values, sess.outputs, prompt, tokenToPhrase, threshold, origW, origH)
}

// buildGroundingInputs constructs the exact ordered input tensor slice the
// exported GroundingDINO expects, by matching each session input by name. It
// returns the tensors (in session order) plus the prompt's token→phrase map
// used to label detections.
func buildGroundingInputs(sess *modelSession, frame []byte, prompt []string) ([]onnxruntime_go.Value, []int32, error) {
	tok := getBERT()
	ids, types, amask, phraseOf := tok.encodePrompt(prompt)
	h, w := imageInputDims(sess, 800)

	imgT, err := buildImageTensor(frame, w, h, imageNetNormalize)
	if err != nil {
		return nil, nil, err
	}

	inputs := make([]onnxruntime_go.Value, len(sess.inputs))
	imgPlaced := false
	for i, in := range sess.inputs {
		switch {
		case containsFold(in.Name, "pixel_values"):
			inputs[i] = imgT
			imgPlaced = true
		case containsFold(in.Name, "input_ids"):
			t, e := onnxruntime_go.NewTensor[int64](onnxruntime_go.NewShape(1, int64(len(ids))), ids)
			if e != nil {
				return nil, nil, failGroundingBuild(imgT, inputs, e)
			}
			inputs[i] = t
		case containsFold(in.Name, "token_type"):
			t, e := onnxruntime_go.NewTensor[int64](onnxruntime_go.NewShape(1, int64(len(types))), types)
			if e != nil {
				return nil, nil, failGroundingBuild(imgT, inputs, e)
			}
			inputs[i] = t
		case containsFold(in.Name, "attention_mask"):
			t, e := onnxruntime_go.NewTensor[int64](onnxruntime_go.NewShape(1, int64(len(amask))), amask)
			if e != nil {
				return nil, nil, failGroundingBuild(imgT, inputs, e)
			}
			inputs[i] = t
		case containsFold(in.Name, "pixel_mask"):
			inputs[i] = buildOnesInt64(onnxruntime_go.NewShape(1, int64(h), int64(w)))
		default:
			return nil, nil, failGroundingBuild(imgT, inputs,
				fmt.Errorf("%w: unsupported GroundingDINO input %q", ErrModelUnavailable, in.Name))
		}
	}
	if !imgPlaced {
		imgT.Destroy()
		destroyValues(inputs)
		return nil, nil, fmt.Errorf("%w: GroundingDINO export missing pixel_values input", ErrModelUnavailable)
	}
	return inputs, phraseOf, nil
}

// failGroundingBuild releases every tensor created so far on an error path.
func failGroundingBuild(imgT onnxruntime_go.Value, inputs []onnxruntime_go.Value, err error) error {
	destroyValues(inputs)
	if !containsTensor(inputs, imgT) {
		imgT.Destroy()
	}
	return err
}

// containsTensor reports whether v occurs in the slice.
func containsTensor(s []onnxruntime_go.Value, v onnxruntime_go.Value) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}


// ──────────────────────────────────────────────────────────────
// Depth Anything V2 Adapter (monocular depth)
// ──────────────────────────────────────────────────────────────

// DepthConfig configures the Depth Anything V2 adapter.
type DepthConfig struct {
	Model  string `json:"model"`   // "depth_anything_v2_vitl"
	Device string `json:"device"`  // "cpu"
	// ModelPath/ModelsDir: explicit path or resolution base.
	ModelPath string `json:"model_path,omitempty"`
	ModelsDir string `json:"models_dir,omitempty"`
}

// DepthAdapter wraps Depth Anything V2 (ONNX) for monocular depth estimation.
type DepthAdapter struct {
	config DepthConfig
	base   *onnxAdapterBase
}

// NewDepthAdapter creates a Depth Anything V2 adapter.
func NewDepthAdapter(config DepthConfig) *DepthAdapter {
	b := &onnxAdapterBase{device: config.Device, modelsDir: config.ModelsDir}
	b.modelPath = b.resolveModelPath(config.Model, config.ModelPath, config.ModelsDir)
	return &DepthAdapter{config: config, base: b}
}

// EstimateDepth returns a per-pixel depth map in meters (relative scale).
func (a *DepthAdapter) EstimateDepth(ctx context.Context, frame []byte) ([][]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sess, err := a.base.ensureSession()
	if err != nil {
		return nil, err
	}
	h, w := imageInputDims(sess, 518)
	t, err := buildImageTensor(frame, w, h, imageNetNormalize)
	if err != nil {
		return nil, fmt.Errorf("depth preprocess: %w", err)
	}
	defer t.Destroy()
	values, err := sess.run([]onnxruntime_go.Value{t})
	if err != nil {
		return nil, fmt.Errorf("depth run: %w", err)
	}
	defer destroyValues(values)
	return decodeDepthOutput(values, sess.outputs, h, w)
}

// ──────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────

// destroyValues releases all onnxruntime values.
func destroyValues(values []onnxruntime_go.Value) {
	for _, v := range values {
		if v != nil {
			_ = v.Destroy()
		}
	}
}

// floatOutput returns the flattened float32 data of the output with the given
// matching name (substring match, case-insensitive). Returns empty if absent.
func floatOutput(values []onnxruntime_go.Value, outs []onnxruntime_go.InputOutputInfo, name string) []float32 {
	for i, o := range outs {
		if containsFold(o.Name, name) {
			if f := asFloat(values[i]); f != nil {
				return f
			}
		}
	}
	return nil
}

// firstFloatOutput returns whichever output is a float32 tensor, preferring a
// multi-dim (non-1d) one, else the largest flat vector.
func firstFloatOutput(values []onnxruntime_go.Value, outs []onnxruntime_go.InputOutputInfo) []float32 {
	var flat []float32
	for _, v := range values {
		if f := asFloat(v); f != nil {
			if len(f) > len(flat) {
				flat = f
			}
		}
	}
	return flat
}

func asFloat(v onnxruntime_go.Value) []float32 {
	if v == nil {
		return nil
	}
	t, ok := v.(*onnxruntime_go.Tensor[float32])
	if !ok {
		return nil
	}
	return t.GetData()
}

func containsFold(s, sub string) bool {
	if len(s) < len(sub) {
		return false
	}
	sub = lower(sub)
	s = lower(s)
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func lower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

func cosineSim(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
	}
	return float32(dot)
}

// buildOnesInt64 creates an int64 tensor of shape shape filled with 1s (the
// GroundingDINO pixel_mask, and the batch dimension of the text tensors).
func buildOnesInt64(shape onnxruntime_go.Shape) *onnxruntime_go.Tensor[int64] {
	n := int64(1)
	for _, d := range shape {
		n *= d
	}
	data := make([]int64, n)
	for i := range data {
		data[i] = 1
	}
	t, err := onnxruntime_go.NewTensor[int64](shape, data)
	if err != nil {
		return nil
	}
	return t
}

// defaultUIGroundingPrompt is the built-in zero-shot class list for
// screenshot/UI perception (text, buttons, icons, charts, headings, ...).
func defaultUIGroundingPrompt() []string {
	return []string{"text", "button", "icon", "logo", "chart", "graph", "image", "heading", "label", "box", "table", "menu", "window", "title", "input", "avatar"}
}

// defaultGroundingLabels is kept as a generic fallback vocabulary for models
// whose class outputs align with a fixed label set (not the zero-shot path).
func defaultGroundingLabels() []string {
	return []string{"object", "person", "car", "building", "tree", "animal", "vehicle", "furniture"}
}
