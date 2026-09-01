package vision

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"

	"github.com/CoscaAI/cosca/internal/worldmodel"
)

// ──────────────────────────────────────────────────────────────
// Automatic image detection → route to the vision pipeline
// ──────────────────────────────────────────────────────────────
//
// This is the integration point for "vision is recognised automatically": when
// an image (PNG/JPEG) arrives in the agent runtime (attachment, frame, or a
// data:image URI), callers sniff the content type, decode the bytes, and run
// the pipeline. The returned Observation carries the semantic understanding
// (entities + relations) ready to be injected into the agent's context — and
// degrades gracefully (non-fatal warnings) when the .onnx models are absent.

// DetectImageContentType sniffs the MIME type of image bytes from their magic
// bytes. It returns "" if the bytes are not a supported raster format.
func DetectImageContentType(data []byte) string {
	if len(data) >= 8 &&
		data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G' {
		return "image/png"
	}
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}
	return ""
}

// IsImageData reports whether the bytes are a supported PNG/JPEG image.
func IsImageData(data []byte) bool { return DetectImageContentType(data) != "" }

// IsImageContentType reports whether a MIME/content-type string is a supported
// image (PNG/JPEG). It strips parameters (e.g. ";base64").
func IsImageContentType(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(strings.Split(ct, ";")[0]))
	return ct == "image/png" || ct == "image/jpeg"
}

// DecodeDataURI extracts the raw image bytes and MIME type from a
// "data:image/<type>;base64,<payload>" URI. It tolerates standard and
// raw (no-padding) base64.
func DecodeDataURI(uri string) ([]byte, string, error) {
	if !strings.HasPrefix(strings.ToLower(uri), "data:image/") {
		return nil, "", fmt.Errorf("not a data:image URI")
	}
	comma := strings.IndexByte(uri, ',')
	if comma < 0 {
		return nil, "", fmt.Errorf("malformed data URI (missing comma)")
	}
	meta := uri[5:comma] // "image/png;base64"
	contentType := strings.Split(meta, ";")[0]
	raw := uri[comma+1:]

	if data, err := base64.StdEncoding.DecodeString(raw); err == nil {
		return data, contentType, nil
	}
	data, err := base64.RawStdEncoding.DecodeString(raw)
	if err != nil {
		return nil, contentType, fmt.Errorf("invalid base64 payload: %w", err)
	}
	return data, contentType, nil
}

// defaultPipelineOnce guards the process-wide default Pipeline. The Perception
// Loop calls DetectImageAndRunVision once per tick: building a brand-new
// Pipeline (and re-loading the ONNX sessions of every adapter) on every tick is
// a memory/cpu leak vector in a long-running loop — the model graph and
// onnxruntime session would be created, and never destroyed, each frame. We
// therefore build the default pipeline ONCE and reuse it. (onnxruntime sessions
// are safe for concurrent Run calls; the perception loop serialises ticks, so
// there is no concurrent use in the serving path.)
var (
	defaultPipelineOnce sync.Once
	defaultPipelineObj  *Pipeline
)

// defaultPipeline returns the shared, process-wide default vision pipeline
// (DefaultPipelineConfig). It is built lazily once and reused forever. The
// adapters' sessions are loaded once and shared; the pipeline is never rebuilt
// per-frame. A nil return (unreachable) degrades gracefully at the call site.
func defaultPipeline() *Pipeline {
	defaultPipelineOnce.Do(func() {
		defaultPipelineObj = NewPipeline(DefaultPipelineConfig())
	})
	return defaultPipelineObj
}

// DetectImageAndRunVision sniffs the image, runs the vision pipeline over it,
// and returns the resulting Observation. It is deliberately "best effort": if
// no .onnx model is downloaded yet, the pipeline degrades to an observation
// with empty entities plus non-fatal Warnings — the caller/agent never breaks.
//
// contentType is used as a hint when the magic-bytes sniff returns an empty
// string (e.g. it was already decoded upstream); it may be "" to rely on the
// sniff alone.
//
// MEMORY NOTE: this reuse is the corrective for the slow leak the Don observed
// (heap_objects rising monotonically in the perception loop). The default
// pipeline is built once and shared, so the ONNX sessions are never churned per
// frame. Callers that need a bespoke config should build a *Pipeline once via
// NewPipeline and reuse it (DetectImageAndRunVisionWithConfig keeps the
// fresh-per-call behaviour for one-off classification).
func DetectImageAndRunVision(ctx context.Context, data []byte, contentType string) (*Observation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ct := DetectImageContentType(data)
	if ct == "" {
		ct = contentType
	}
	if !IsImageData(data) && !IsImageContentType(ct) {
		return nil, fmt.Errorf("not a supported image (expected PNG or JPEG)")
	}
	return defaultPipeline().Process(ctx, data, worldmodel.Pose6DoF{})
}

// DetectImageAndRunVisionWithPrompt is DetectImageAndRunVision with a caller
// supplied zero-shot prompt (list of classes). When prompt is empty the default
// UI list is used.
func DetectImageAndRunVisionWithPrompt(ctx context.Context, data []byte, contentType string, prompt []string) (*Observation, error) {
	cfg := DefaultPipelineConfig()
	if len(prompt) > 0 && cfg.Grounding != nil {
		cfg.Grounding.Prompt = prompt
	}
	return DetectImageAndRunVisionWithConfig(ctx, data, contentType, cfg)
}

// DetectImageAndRunVisionWithConfig runs the pipeline with an explicit config,
// enabling callers to override the zero-shot prompt / thresholds / models dir.
func DetectImageAndRunVisionWithConfig(ctx context.Context, data []byte, contentType string, cfg PipelineConfig) (*Observation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ct := DetectImageContentType(data)
	if ct == "" {
		ct = contentType
	}
	if !IsImageData(data) && !IsImageContentType(ct) {
		return nil, fmt.Errorf("not a supported image (expected PNG or JPEG)")
	}
	p := NewPipeline(cfg)
	return p.Process(ctx, data, worldmodel.Pose6DoF{})
}

// SummaryText renders the semantic understanding of an Observation as a short
// human/machine-readable string, suitable for injecting into an agent context
// block. It lists entities (label + confidence + depth) and their spatial
// relations.
func (o *Observation) SummaryText() string {
	if o == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Vision observation: %d entity(ies), %d relation(s).",
		len(o.Entities), len(o.Relations)))
	for i, e := range o.Entities {
		b.WriteString(fmt.Sprintf("\n  [%d] %s (conf %.2f, depth %.2fm)",
			i+1, e.Label, e.Confidence, e.Depth))
	}
	for _, r := range o.Relations {
		b.WriteString(fmt.Sprintf("\n  relation %s-%s: %s", r.Subject, r.Object, r.Relation))
	}
	if len(o.Warnings) > 0 {
		b.WriteString(fmt.Sprintf("\n  %d degradation warning(s): %s",
			len(o.Warnings), strings.Join(o.Warnings, "; ")))
	}
	return b.String()
}
