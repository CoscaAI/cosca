package audio

import (
	"context"
	"testing"

	"github.com/CoscaAI/cosca/internal/worldmodel"
)

// ──────────────────────────────────────────────────────────────
// Pipeline tests
// ──────────────────────────────────────────────────────────────

func TestPipelineConfigDefaults(t *testing.T) {
	config := DefaultPipelineConfig()

	if config.Whisper == nil {
		t.Error("Whisper config should not be nil")
	}
	if config.DiffFields == nil {
		t.Error("DiffFields config should not be nil")
	}
	if config.CoquiTTS == nil {
		t.Error("CoquiTTS config should not be nil")
	}
	if config.AudioCraft == nil {
		t.Error("AudioCraft config should not be nil")
	}
}

func TestPipelineNew(t *testing.T) {
	config := DefaultPipelineConfig()
	p := NewPipeline(config)

	if p == nil {
		t.Fatal("NewPipeline returned nil")
	}
	if p.whisper == nil {
		t.Error("whisper adapter not initialized")
	}
	if p.diff == nil {
		t.Error("diff fields adapter not initialized")
	}
	if p.tts == nil {
		t.Error("coqui adapter not initialized")
	}
	if p.craft == nil {
		t.Error("audiocraft adapter not initialized")
	}
}

func TestPipelineNewMinimal(t *testing.T) {
	config := PipelineConfig{}
	p := NewPipeline(config)

	if p.whisper != nil {
		t.Error("whisper should be nil when not configured")
	}
	if p.diff != nil {
		t.Error("diff should be nil when not configured")
	}
	if p.tts != nil {
		t.Error("tts should be nil when not configured")
	}
	if p.craft != nil {
		t.Error("craft should be nil when not configured")
	}
}

// ──────────────────────────────────────────────────────────────
// Perception tests
// ──────────────────────────────────────────────────────────────

func TestPerceiveNoAdapters(t *testing.T) {
	config := PipelineConfig{}
	p := NewPipeline(config)
	ctx := context.Background()

	// With no adapters, pipeline returns empty result (no error)
	result, err := p.Perceive(ctx, []byte{0x01, 0x02}, 16000, 1)
	if err != nil {
		t.Errorf("Perceive without adapters should not fail: %v", err)
	}
	if result == nil {
		t.Error("result should not be nil")
	}
}

func TestInferEvents(t *testing.T) {
	config := PipelineConfig{}
	p := NewPipeline(config)

	// Text only
	events := p.inferEvents("hello world", nil)
	if len(events) != 1 {
		t.Errorf("text-only events: got %d, want 1", len(events))
	}
	if events[0].Type != "speech" {
		t.Errorf("event type: got %v, want speech", events[0].Type)
	}

	// Spatial only (high intensity)
	spatial := &worldmodel.SpatialAudio{Intensity: 0.8}
	events = p.inferEvents("", spatial)
	if len(events) != 1 {
		t.Errorf("spatial-only events: got %d, want 1", len(events))
	}
	if events[0].Type != "environment" {
		t.Errorf("event type: got %v, want environment", events[0].Type)
	}

	// Both
	events = p.inferEvents("hello", spatial)
	if len(events) != 2 {
		t.Errorf("both events: got %d, want 2", len(events))
	}

	// Low intensity spatial (no event)
	spatialLow := &worldmodel.SpatialAudio{Intensity: 0.2}
	events = p.inferEvents("", spatialLow)
	if len(events) != 0 {
		t.Errorf("low intensity events: got %d, want 0", len(events))
	}
}

// ──────────────────────────────────────────────────────────────
// Generation tests
// ──────────────────────────────────────────────────────────────

func TestGenerateTTSNoAdapter(t *testing.T) {
	config := PipelineConfig{}
	p := NewPipeline(config)
	ctx := context.Background()

	_, err := p.GenerateTTS(ctx, "hello", "default")
	if err == nil {
		t.Error("GenerateTTS without adapter should fail")
	}
}

func TestGenerateSFXNoAdapter(t *testing.T) {
	config := PipelineConfig{}
	p := NewPipeline(config)
	ctx := context.Background()

	_, err := p.GenerateSFX(ctx, "explosion", 2.0)
	if err == nil {
		t.Error("GenerateSFX without adapter should fail")
	}
}

// ──────────────────────────────────────────────────────────────
// Adapter config tests
// ──────────────────────────────────────────────────────────────

func TestWhisperConfig(t *testing.T) {
	config := WhisperConfig{Model: "large", Device: "cuda"}
	adapter := NewWhisperAdapter(config)

	if adapter.config.Model != "large" {
		t.Errorf("model: got %v, want large", adapter.config.Model)
	}
	if adapter.config.Script == "" {
		t.Error("script should have default value")
	}
}

func TestDiffFieldsConfig(t *testing.T) {
	config := DiffFieldsConfig{Device: "cuda"}
	adapter := NewDiffFieldsAdapter(config)

	if adapter.config.Device != "cuda" {
		t.Errorf("device: got %v, want cuda", adapter.config.Device)
	}
}

func TestCoquiConfig(t *testing.T) {
	config := CoquiTTSConfig{Model: "custom", Device: "mps"}
	adapter := NewCoquiAdapter(config)

	if adapter.config.Model != "custom" {
		t.Errorf("model: got %v, want custom", adapter.config.Model)
	}
}

func TestAudioCraftConfig(t *testing.T) {
	config := AudioCraftConfig{Device: "cuda"}
	adapter := NewAudioCraftAdapter(config)

	if adapter.config.Device != "cuda" {
		t.Errorf("device: got %v, want cuda", adapter.config.Device)
	}
}

// ──────────────────────────────────────────────────────────────
// Result structure tests
// ──────────────────────────────────────────────────────────────

func TestPerceptionResultStructure(t *testing.T) {
	result := &PerceptionResult{
		Text: "hello world",
		Events: []worldmodel.AudioEvent{
			{ID: "e1", Type: "speech", Source: worldmodel.Vec3{}, Volume: 1.0, Confidence: 0.9},
		},
	}

	if result.Text != "hello world" {
		t.Errorf("text: got %v, want 'hello world'", result.Text)
	}
	if len(result.Events) != 1 {
		t.Errorf("events: got %d, want 1", len(result.Events))
	}
}

func TestGenerateResultStructure(t *testing.T) {
	result := &GenerateResult{
		AudioData:  []byte{0x01, 0x02, 0x03},
		SampleRate: 24000,
	}

	if len(result.AudioData) != 3 {
		t.Errorf("audio data: got %d bytes, want 3", len(result.AudioData))
	}
	if result.SampleRate != 24000 {
		t.Errorf("sample rate: got %v, want 24000", result.SampleRate)
	}
}
