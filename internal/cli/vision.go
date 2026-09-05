package cli

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/worldmodel"
	"github.com/CoscaAI/cosca/internal/worldmodel/vision"
)

// ──────────────────────────────────────────────────────────────
// Vision engine CLI (native Go ONNX — no Python, no PyTorch)
// ──────────────────────────────────────────────────────────────

// NewVisionCommand creates `cosca vision` — the native Go ONNX vision engine
// group. It mirrors `cosca model vision` (status) and adds the operational
// subcommand `cosca vision infer <image>` that actually runs the perception
// pipeline over a PNG/JPEG frame via the real binary.
func NewVisionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vision",
		Short: "Native Go ONNX vision engine (run perception on images)",
		Long: `Run the native Go vision engine directly — no Python, no PyTorch,
no external runtime. The engine loads the .onnx models (CLIP/SAM2/GroundingDINO/
Depth) through onnxruntime and builds a structured Observation.

Subcommands:
  infer    Run the full perception pipeline over a PNG/JPEG image`,
		Example: `  cosca vision infer ~/Downloads/photo.png
  cosca vision infer screenshot.png --json`,
	}
	cmd.AddCommand(NewVisionInferCommand())
	return cmd
}

// NewVisionInferCommand creates `cosca vision infer <image>`. It reads a PNG or
// JPEG file off disk, runs the perception pipeline (detect → classify → embed
// → depth → relations) and prints the resulting semantic understanding. It also
// validates that the CLIP image encoder actually runs (the feature vector),
// independent of the regression where object *detection* needs GroundingDINO.
func NewVisionInferCommand() *cobra.Command {
	var dir string
	var engine string
	var promptStr string

	cmd := &cobra.Command{
		Use:   "infer <image>",
		Short: "Run the vision pipeline on a PNG/JPEG image",
		Long: `Load a PNG/JPEG image and run the native Go vision pipeline over it.

The observation is "best effort": if a model (GroundingDINO for detection, SAM2
for segmentation, DepthAnything for depth) is not downloaded yet, that stage
degrades gracefully to a non-fatal warning and the rest of the pipeline still
runs. The CLIP ViT-B/32 image encoder is always attempted so you can confirm
the engine produces a real embedding for the frame.

The models directory is resolved in this order:
  1. $COSCA_MODELS
  2. ~/.cosca/models/vision/
  3. ./.cosca/models/vision

Use --dir to point at another directory.`,
		Example: `  cosca vision infer ~/Downloads/photo.png
  cosca vision infer screenshot.png --json
  cosca vision infer frame.png --dir ~/.cosca/models/vision`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			path := args[0]
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read image %q: %w", path, err)
			}

			contentType := vision.DetectImageContentType(data)
			if contentType == "" {
				contentType = contentTypeFromExt(path)
			}
			if !vision.IsImageData(data) && !vision.IsImageContentType(contentType) {
				return fmt.Errorf("unsupported image %q (expected PNG or JPEG)", path)
			}

			modelsDir := vision.ModelsDirFor(dir)

			// 1) Full perception pipeline (detect → classify → embed → depth).
			// A custom --prompt (comma-separated classes) overrides the default
			// UI zero-shot prompt; otherwise the built-in UI list is used.
			var prompt []string
			if promptStr != "" {
				for _, p := range strings.Split(promptStr, ",") {
					if p = strings.TrimSpace(p); p != "" {
						prompt = append(prompt, p)
					}
				}
			}
			obs, err := vision.DetectImageAndRunVisionWithPrompt(cmd.Context(), data, contentType, prompt)
			if err != nil {
				return fmt.Errorf("vision pipeline: %w", err)
			}

			// 2) Direct CLIP image-encoder validation: the pipeline only invokes
			// CLIP when detection produced entities, so when GroundingDINO is
			// absent we still want to prove the CLIP session loads and runs.
			clipEmb, clipErr := runClipEmbed(cmd.Context(), engine, dir, data)

			return renderVisionInfer(cmd, formatter, useJSON, path, contentType,
				modelsDir, obs, clipEmb, clipErr)
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "", "Models directory (default: resolve from $COSCA_MODELS / ~/.cosca/models/vision)")
	cmd.Flags().StringVar(&engine, "engine", "cpu", "ONNX runtime execution provider (cpu)")
	cmd.Flags().StringVar(&promptStr, "prompt", "", "Comma-separated zero-shot classes (default: UI list: text,button,icon,logo,chart,graph,image,heading,label,box,table,menu,window,title)")
	return cmd
}

// runClipEmbed builds a CLIP adapter and embeds the whole frame, returning the
// L2-normalised feature vector. It is the direct proof that the onnxruntime
// session (and thus the loaded .onnx model) is healthy.
func runClipEmbed(ctx context.Context, engine, dir string, frame []byte) ([]float32, error) {
	cfg := vision.ClipConfig{Model: "ViT-B/32", Device: engine, ModelsDir: dir}
	adapter := vision.NewClipAdapter(cfg)
	return adapter.Embed(ctx, frame)
}

// renderVisionInfer prints the observation in text or JSON form.
//
// It deliberately never returns an error for a *degraded* model — a missing
// GroundingDINO/SAM2/Depth model is a warning, not a crash. Only a genuinely
// unrecoverable rendering problem returns an error.
func renderVisionInfer(cmd *cobra.Command, formatter *OutputFormatter, useJSON bool,
	path, contentType, modelsDir string, obs *vision.Observation,
	clipEmb []float32, clipErr error) error {

	res := buildVisionInferResult(path, contentType, modelsDir, obs, clipEmb, clipErr)

	if useJSON {
		return printJSON(cmd, res)
	}

	formatter.Header("Vision infer")
	formatter.KeyValue("Image", path)
	formatter.KeyValue("Type", contentType)
	formatter.KeyValue("ModelsDir", modelsDir)
	formatter.Println("")

	// Semantic understanding summary (entities + relations).
	formatter.Header("Semantic understanding")
	formatter.Println(obs.SummaryText())
	formatter.Println("")

	// Detailed entities (label, bbox, confidence, depth).
	if len(obs.Entities) > 0 {
		formatter.Header("Entities")
		rows := make([][]string, 0, len(obs.Entities))
		for _, e := range obs.Entities {
			rows = append(rows, []string{
				strings.TrimSpace(e.Label),
				string(e.Type),
				fmt.Sprintf("%.2f", e.Confidence),
				fmt.Sprintf("%.2fm", e.Depth),
				bboxString(e.BoundingBox),
			})
		}
		formatter.Table([]string{"Label", "Type", "Conf", "Depth", "BBox"}, rows)
		formatter.Println("")
	}

	// Spatial relations.
	if len(obs.Relations) > 0 {
		formatter.Header("Relations")
		rows := make([][]string, 0, len(obs.Relations))
		for _, r := range obs.Relations {
			rows = append(rows, []string{
				shortEntityID(r.Subject), shortEntityID(r.Object),
				r.Relation, fmt.Sprintf("%.2f", r.Distance),
				fmt.Sprintf("%.2f", r.Confidence),
			})
		}
		formatter.Table([]string{"Subject", "Object", "Relation", "Dist", "Conf"}, rows)
		formatter.Println("")
	}

	// CLIP image-encoder result (the actual inference health check).
	formatter.Header("CLIP image encoder")
	if clipErr != nil {
		if isDegradationErr(clipErr) {
			formatter.Warning("CLIP model unavailable: " + clipErr.Error())
			formatter.KeyValue("Status", "degraded (no CLIP session)")
		} else {
			formatter.Error("CLIP error: " + clipErr.Error())
			formatter.KeyValue("Status", "error")
		}
	} else {
		formatter.KeyValue("Status", "ok")
		formatter.KeyValue("Dimensions", fmt.Sprintf("%d", len(clipEmb)))
		formatter.KeyValue("L2 Norm", fmt.Sprintf("%.4f", vecNorm(clipEmb)))
		formatter.KeyValue("Head", summarizeHead(clipEmb, 8))
	}
	formatter.Println("")

	if len(obs.Entities) == 0 && clipErr == nil {
		formatter.Warning("No entities detected: object detection needs GroundingDINO (only the CLIP image encoder ran).")
	}
	return nil
}

// buildVisionInferResult assembles a fully JSON-serialisable result.
func buildVisionInferResult(path, contentType, modelsDir string, obs *vision.Observation,
	clipEmb []float32, clipErr error) map[string]any {
	res := map[string]any{
		"image":         path,
		"content_type":  contentType,
		"models_dir":    modelsDir,
		"model_status":  modelStatusList(modelsDir),
		"entity_count":  len(obs.Entities),
		"relation_count": len(obs.Relations),
		"entities":      entityList(obs.Entities),
		"relations":     relationList(obs.Relations),
		"warnings":      append([]string{}, obs.Warnings...),
		"latency":       obs.Latency.Round(time.Millisecond).String(),
	}

	if clipErr != nil {
		res["clip"] = map[string]any{
			"ok":    false,
			"error": clipErr.Error(),
		}
	} else {
		res["clip"] = map[string]any{
			"ok":          true,
			"dimensions":  len(clipEmb),
			"l2_norm":     vecNorm(clipEmb),
			"head":        clipEmb[:min(len(clipEmb), 8)],
		}
	}
	return res
}

// modelStatusList reports the presence of each canonical .onnx model in a dir.
func modelStatusList(dir string) map[string]any {
	files := []struct {
		name     string
		desc     string
	}{
		{vision.ClipModelFile, "CLIP ViT-B/32 (classification + embeddings)"},
		{vision.SAMModelFile, "SAM2 (segmentation)"},
		{vision.GroundingModelFile, "GroundingDINO (text-prompted detection)"},
		{vision.DepthModelFile, "Depth Anything V2 (monocular depth)"},
	}
	out := make(map[string]any)
	for _, f := range files {
		p := filepath.Join(dir, f.name)
		status := map[string]any{"present": false, "path": p}
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			status["present"] = true
			status["size_bytes"] = st.Size()
			status["size"] = formatSize(st.Size())
		}
		out[f.name] = status
	}
	return out
}

// entityList converts world entities to a compact JSON-friendly form.
func entityList(entities []worldmodel.WorldEntity) []map[string]any {
	out := make([]map[string]any, 0, len(entities))
	for _, e := range entities {
		entry := map[string]any{
			"id":         e.ID,
			"label":      e.Label,
			"type":       string(e.Type),
			"confidence": e.Confidence,
			"depth":      e.Depth,
			"bbox": map[string]any{
				"x": (e.BoundingBox.Min.X + e.BoundingBox.Max.X) / 2,
				"y": (e.BoundingBox.Min.Y + e.BoundingBox.Max.Y) / 2,
				"w": e.BoundingBox.Max.X - e.BoundingBox.Min.X,
				"h": e.BoundingBox.Max.Y - e.BoundingBox.Min.Y,
			},
		}
		if len(e.Embedding) > 0 {
			entry["embedding_dim"] = len(e.Embedding)
		}
		out = append(out, entry)
	}
	return out
}

// relationList converts spatial relations to a compact JSON-friendly form.
func relationList(relations []worldmodel.SpatialRelation) []map[string]any {
	out := make([]map[string]any, 0, len(relations))
	for _, r := range relations {
		out = append(out, map[string]any{
			"subject":    r.Subject,
			"object":     r.Object,
			"relation":   r.Relation,
			"distance":   r.Distance,
			"confidence": r.Confidence,
		})
	}
	return out
}

// bboxString renders an AABB as a compact "cx,cy w×h" line.
func bboxString(b worldmodel.AABB) string {
	return fmt.Sprintf("%.0f,%.0f %.0f×%.0f",
		(b.Min.X+b.Max.X)/2, (b.Min.Y+b.Max.Y)/2,
		b.Max.X-b.Min.X, b.Max.Y-b.Min.Y)
}

// shortEntityID shows the tail of an entity id for compact relation tables.
func shortEntityID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[len(id)-12:]
}

// summarizeHead returns the first n floats of a vector, comma-separated.
func summarizeHead(v []float32, n int) string {
	if len(v) == 0 {
		return ""
	}
	if n > len(v) {
		n = len(v)
	}
	parts := make([]string, n)
	for i := 0; i < n; i++ {
		parts[i] = fmt.Sprintf("%.3f", v[i])
	}
	return strings.Join(parts, ", ")
}

// vecNorm computes the L2 norm of a float32 vector.
func vecNorm(v []float32) float64 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	return math.Sqrt(sum)
}

// contentTypeFromExt maps a file extension to a MIME type as a last resort.
func contentTypeFromExt(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	default:
		return ""
	}
}

// isDegradationErr reports whether err is a "model unavailable / not found"
// degradation (non-fatal) rather than a hard session/runtime error.
func isDegradationErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "model unavailable") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "model file not found")
}


