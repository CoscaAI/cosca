package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/worldmodel"
	"github.com/CoscaAI/cosca/internal/worldmodel/vision"
)

// tinyPNG is a valid 1×1 transparent PNG as base64. It is only used so the
// data-URI decodes; the injected fake vision runner never reads the pixels.
const tinyPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="

// fakeVisionObs returns a deterministic Observation with entities so the test
// asserts the injected semantic understanding without real ONNX inference.
func fakeVisionObs() *vision.Observation {
	return &vision.Observation{
		Entities: []worldmodel.WorldEntity{
			{ID: "e0", Type: worldmodel.EntityObject, Label: "button", Confidence: 0.93, Depth: 1.5},
			{ID: "e1", Type: worldmodel.EntityObject, Label: "text", Confidence: 0.88, Depth: 2.0},
		},
		Relations: []worldmodel.SpatialRelation{
			{Subject: "e0", Object: "e1", Relation: "right_of", Distance: 1.0, Confidence: 0.7},
		},
	}
}

// imageMessage builds a user message carrying an image data-URI attachment.
func imageMessage(uri string) chat.Message {
	return chat.Message{
		Role: chat.RoleUser,
		ContentParts: []chat.ContentPart{
			{Type: "image_url", ImageURL: &chat.ImageURL{URL: uri, Detail: "auto"}},
			{Type: "text", Text: "what is in this screenshot?"},
		},
	}
}

// allTextParts joins the Text of every text content part for easy assertion.
func allTextParts(m chat.Message) string {
	var b strings.Builder
	for _, p := range m.ContentParts {
		if p.Type == "text" {
			b.WriteString(p.Text)
			b.WriteString(" ")
		}
	}
	return b.String()
}

func TestEnrichMessagesWithVision_WithImage_AppendsSummary(t *testing.T) {
	runner := func(_ context.Context, _ []byte, _ string) (*vision.Observation, error) {
		return fakeVisionObs(), nil
	}

	msgs := []chat.Message{imageMessage("data:image/png;base64," + tinyPNG)}
	enriched := enrichMessagesWithVision(context.Background(), msgs, true, runner)

	if len(enriched) != len(msgs) {
		t.Fatalf("expected %d messages, got %d", len(msgs), len(enriched))
	}
	got := allTextParts(enriched[0])
	if !strings.Contains(got, "[auto-vision]") {
		t.Errorf("message missing auto-vision marker: %q", got)
	}
	if !strings.Contains(got, "Vision observation") {
		t.Errorf("message missing vision summary header: %q", got)
	}
	if !strings.Contains(got, "button") {
		t.Errorf("message missing detected entity label: %q", got)
	}
	if !strings.Contains(got, "right_of") {
		t.Errorf("message missing spatial relation: %q", got)
	}
	// The original text part must be preserved.
	if !strings.Contains(got, "what is in this screenshot?") {
		t.Errorf("original text part was lost: %q", got)
	}
}

func TestEnrichMessagesWithVision_Disabled_Unchanged(t *testing.T) {
	// Even with an image and a nominal runner, the hook is off → no enrichment.
	msgs := []chat.Message{imageMessage("data:image/png;base64," + tinyPNG)}
	enriched := enrichMessagesWithVision(context.Background(), msgs, false, fakeVisionRunnerUnused(t))

	if got := allTextParts(enriched[0]); strings.Contains(got, "Vision observation") {
		t.Errorf("disabled hook should not enrich, got: %q", got)
	}
	if len(enriched[0].ContentParts) != 2 {
		t.Errorf("disabled hook should keep parts unchanged: got %d parts", len(enriched[0].ContentParts))
	}
}

func fakeVisionRunnerUnused(t *testing.T) visionRunner {
	t.Helper()
	return func(_ context.Context, _ []byte, _ string) (*vision.Observation, error) {
		return fakeVisionObs(), nil
	}
}

func TestEnrichMessagesWithVision_NoImage_Unchanged(t *testing.T) {
	msgs := []chat.Message{
		{Role: chat.RoleUser, Content: "just text, no image"},
	}
	enriched := enrichMessagesWithVision(context.Background(), msgs, true, fakeVisionRunnerUnused(t))

	if len(enriched) != 1 {
		t.Fatalf("expected 1 message, got %d", len(enriched))
	}
	if len(enriched[0].ContentParts) != 0 {
		t.Errorf("text-only message should not gain content parts: %d", len(enriched[0].ContentParts))
	}
	if enriched[0].Content != "just text, no image" {
		t.Errorf("message content changed: %q", enriched[0].Content)
	}
}

func TestEnrichMessagesWithVision_VisionFails_Unchanged(t *testing.T) {
	runner := func(_ context.Context, _ []byte, _ string) (*vision.Observation, error) {
		return nil, context.DeadlineExceeded
	}

	msgs := []chat.Message{imageMessage("data:image/png;base64," + tinyPNG)}
	enriched := enrichMessagesWithVision(context.Background(), msgs, true, runner)

	if got := allTextParts(enriched[0]); strings.Contains(got, "Vision observation") {
		t.Errorf("failed vision should not enrich, got: %q", got)
	}
	if len(enriched[0].ContentParts) != 2 {
		t.Errorf("failed vision should keep parts unchanged: got %d parts", len(enriched[0].ContentParts))
	}
}

func TestEnrichMessagesWithVision_NonDataURI_Unchanged(t *testing.T) {
	// A remote http(s) URL cannot be resolved without a network fetch — the
	// hook skips it (only data:image URIs are handled) and leaves the message
	// unchanged.
	msgs := []chat.Message{imageMessage("https://example.com/shot.png")}
	enriched := enrichMessagesWithVision(context.Background(), msgs, true, fakeVisionRunnerUnused(t))

	if got := allTextParts(enriched[0]); strings.Contains(got, "Vision observation") {
		t.Errorf("http URL should not enrich, got: %q", got)
	}
	if len(enriched[0].ContentParts) != 2 {
		t.Errorf("http URL should keep parts unchanged: got %d parts", len(enriched[0].ContentParts))
	}
}

func TestEnrichMessagesWithVision_MultipleImages_Combined(t *testing.T) {
	runner := func(_ context.Context, _ []byte, _ string) (*vision.Observation, error) {
		return fakeVisionObs(), nil
	}

	// A single message carrying two attached images.
	msg := chat.Message{
		Role: chat.RoleUser,
		ContentParts: []chat.ContentPart{
			{Type: "image_url", ImageURL: &chat.ImageURL{URL: "data:image/png;base64," + tinyPNG}},
			{Type: "image_url", ImageURL: &chat.ImageURL{URL: "data:image/jpeg;base64," + tinyPNG}},
		},
	}
	enriched := enrichMessagesWithVision(context.Background(), []chat.Message{msg}, true, runner)

	// Each image yields one summary → two appended text parts.
	count := 0
	for _, p := range enriched[0].ContentParts {
		if p.Type == "text" && strings.Contains(p.Text, "[auto-vision]") {
			count++
		}
	}
	if count != 2 {
		t.Errorf("expected 2 auto-vision text parts, got %d", count)
	}
}
