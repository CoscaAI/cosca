package cli

import (
	"context"
	"strings"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/worldmodel/vision"
)

// ──────────────────────────────────────────────────────────────
// Automatic vision recognition in the agent loop
// ──────────────────────────────────────────────────────────────
//
// Plugged into `runAgentToolLoop` (internal/cli/agent.go): just before the
// conversation is sent to `provider.Chat`, the messages are scanned for image
// content. When an image (a PNG/JPEG `data:image` URI wrapped in a
// ContentPart[].image_url) is present, the native Go ONNX vision pipeline runs
// over its decoded bytes and the resulting semantic understanding
// (Observation.SummaryText) is appended to that SAME message — so the model
// receives "image + its textual understanding" in one turn, WITHOUT relying on
// the language model to "see" the image.
//
// The hook is additive and best-effort. It never breaks the tool-calling flow:
//   - vision is disabled → messages pass through untouched;
//   - no image present → untouched;
//   - the pipeline errors (e.g. a missing .onnx model) → the image is skipped
//     and the rest of the conversation is preserved;
//   - a degraded observation (warnings, no entities) still yields a SummaryText,
//     so the model is told the image was seen but perception was degraded.

// visionRunner is the vision pipeline entry point. It is an injectable seam so
// tests can supply a deterministic fake without loading onnxruntime.
type visionRunner func(ctx context.Context, data []byte, contentType string) (*vision.Observation, error)

// defaultVisionRunner runs the real native Go ONNX vision pipeline (best-effort).
func defaultVisionRunner(ctx context.Context, data []byte, contentType string) (*vision.Observation, error) {
	return vision.DetectImageAndRunVision(ctx, data, contentType)
}

// enrichMessagesWithVision is the automatic vision recognition hook. It scans
// every message for embedded image data-URIs and, for each, runs the vision
// pipeline and appends the semantic understanding to the carrying message.
//
// runner may be nil to use the production pipeline; tests inject a fake. When
// enabled is false the messages are returned unchanged (opt-in default).
func enrichMessagesWithVision(ctx context.Context, messages []chat.Message, enabled bool, runner visionRunner) []chat.Message {
	if !enabled {
		return messages
	}
	if runner == nil {
		runner = defaultVisionRunner
	}
	enriched := make([]chat.Message, 0, len(messages))
	for _, m := range messages {
		enriched = append(enriched, enrichMessageWithVision(ctx, m, runner))
	}
	return enriched
}

// enrichMessageWithVision processes a single message. If it carries one or more
// `data:image/...` URIs (PNG/JPEG) as image_url parts, it decodes each, runs
// the vision pipeline, and appends the resulting SummaryText as an extra text
// content part on the SAME message. It never errors: a message with no image,
// a non-data URI, or a failed pipeline is returned unmodified.
func enrichMessageWithVision(ctx context.Context, m chat.Message, runner visionRunner) chat.Message {
	var summaries []string

	for _, part := range m.ContentParts {
		if part.Type != "image_url" || part.ImageURL == nil || part.ImageURL.URL == "" {
			continue
		}
		uri := part.ImageURL.URL
		// Only data:image URIs are decoded here without a network fetch. A
		// plain http(s) URL is a remote reference this hook does not resolve.
		if !strings.HasPrefix(strings.ToLower(uri), "data:image/") {
			continue
		}

		data, contentType, err := vision.DecodeDataURI(uri)
		if err != nil {
			continue
		}

		obs, err := runner(ctx, data, contentType)
		if err != nil || obs == nil {
			// Graceful degradation: skip this image, keep the rest of the flow.
			continue
		}
		if summary := obs.SummaryText(); summary != "" {
			summaries = append(summaries, summary)
		}
	}

	if len(summaries) == 0 {
		return m
	}

	// Append the perception result as a text part on the same message that
	// carried the image. ContentParts takes precedence over Content when the
	// provider maps the message, and AddText preserves any existing parts.
	for _, s := range summaries {
		m.AddText("[auto-vision] " + s)
	}
	return m
}
