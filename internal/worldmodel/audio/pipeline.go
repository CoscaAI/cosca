// Package audio implements the audio perception and generation pipeline for the Living World.
//
// Pipeline flow (Perception):
//
//	AudioChunk → STT (whisper.cpp) → text
//	AudioChunk → SpatialAnalysis (diffusion fields) → spatial_audio
//	text + spatial → AudioEvent (what, where, who)
//
// Pipeline flow (Generation):
//
//	Text → TTS (Coqui TTS) → audio bytes
//	Event type → Sound Effect (AudioCraft) → audio bytes
//	Spatial relation → Spatial Mix → 3D audio
package audio

import (
	"context"
	"fmt"
	"time"

	"github.com/CoscaAI/cosca/internal/worldmodel"
)

// ──────────────────────────────────────────────────────────────
// Pipeline configuration
// ──────────────────────────────────────────────────────────────

// PipelineConfig configures the audio pipeline.
type PipelineConfig struct {
	Whisper    *WhisperConfig    `json:"whisper,omitempty"`
	DiffFields *DiffFieldsConfig `json:"diff_fields,omitempty"`
	CoquiTTS   *CoquiTTSConfig   `json:"coqui_tts,omitempty"`
	AudioCraft *AudioCraftConfig `json:"audiocraft,omitempty"`
}

// DefaultPipelineConfig returns sensible defaults.
func DefaultPipelineConfig() PipelineConfig {
	return PipelineConfig{
		Whisper:    &WhisperConfig{Model: "base", Device: "cpu"},
		DiffFields: &DiffFieldsConfig{Device: "cpu"},
		CoquiTTS:   &CoquiTTSConfig{Model: "tts_models/multilingual/multi-dataset/xtts_v2", Device: "cpu"},
		AudioCraft: &AudioCraftConfig{Device: "cpu"},
	}
}

// ──────────────────────────────────────────────────────────────
// Pipeline
// ──────────────────────────────────────────────────────────────

// Pipeline orchestrates audio adapters.
type Pipeline struct {
	config   PipelineConfig
	whisper  *WhisperAdapter
	diff     *DiffFieldsAdapter
	tts      *CoquiAdapter
	craft    *AudioCraftAdapter
}

// NewPipeline creates an audio pipeline.
func NewPipeline(config PipelineConfig) *Pipeline {
	p := &Pipeline{config: config}
	if config.Whisper != nil {
		p.whisper = NewWhisperAdapter(*config.Whisper)
	}
	if config.DiffFields != nil {
		p.diff = NewDiffFieldsAdapter(*config.DiffFields)
	}
	if config.CoquiTTS != nil {
		p.tts = NewCoquiAdapter(*config.CoquiTTS)
	}
	if config.AudioCraft != nil {
		p.craft = NewAudioCraftAdapter(*config.AudioCraft)
	}
	return p
}

// ──────────────────────────────────────────────────────────────
// Perception
// ──────────────────────────────────────────────────────────────

// PerceptionResult is the result of audio perception.
type PerceptionResult struct {
	Text      string                `json:"text"`
	Events    []worldmodel.AudioEvent `json:"events"`
	Spatial   *worldmodel.SpatialAudio `json:"spatial,omitempty"`
	Latency   time.Duration         `json:"latency"`
}

// Perceive processes an audio chunk through the perception pipeline.
func (p *Pipeline) Perceive(ctx context.Context, audioData []byte, sampleRate int, channels int) (*PerceptionResult, error) {
	start := time.Now()
	result := &PerceptionResult{}

	// Step 1: Speech to text
	if p.whisper != nil {
		text, err := p.whisper.Transcribe(ctx, audioData, sampleRate, channels)
		if err != nil {
			return nil, fmt.Errorf("whisper transcribe: %w", err)
		}
		result.Text = text
	}

	// Step 2: Spatial audio analysis
	if p.diff != nil {
		spatial, err := p.diff.Analyze(ctx, audioData, sampleRate, channels)
		if err == nil {
			result.Spatial = &spatial
		}
	}

	// Step 3: Infer audio events from text + spatial
	if result.Text != "" || result.Spatial != nil {
		result.Events = p.inferEvents(result.Text, result.Spatial)
	}

	result.Latency = time.Since(start)
	return result, nil
}

// inferEvents creates AudioEvents from text and spatial analysis.
func (p *Pipeline) inferEvents(text string, spatial *worldmodel.SpatialAudio) []worldmodel.AudioEvent {
	var events []worldmodel.AudioEvent

	if text != "" {
		events = append(events, worldmodel.AudioEvent{
			ID:         "speech_0",
			Type:       "speech",
			Source:     worldmodel.Vec3{},
			Content:    text,
			Confidence: 0.9,
			Timestamp:  time.Now(),
		})
	}

	if spatial != nil && spatial.Intensity > 0.5 {
		events = append(events, worldmodel.AudioEvent{
			ID:         "env_0",
			Type:       "environment",
			Source:     worldmodel.Vec3{},
			Volume:     spatial.Intensity,
			Confidence: 0.7,
			Timestamp:  time.Now(),
		})
	}

	return events
}

// ──────────────────────────────────────────────────────────────
// Generation
// ──────────────────────────────────────────────────────────────

// GenerateResult is the result of audio generation.
type GenerateResult struct {
	AudioData []byte        `json:"audio_data"`
	SampleRate int          `json:"sample_rate"`
	Latency   time.Duration `json:"latency"`
}

// GenerateTTS converts text to speech.
func (p *Pipeline) GenerateTTS(ctx context.Context, text string, voice string) (*GenerateResult, error) {
	if p.tts == nil {
		return nil, fmt.Errorf("no TTS adapter configured")
	}

	start := time.Now()
	data, sampleRate, err := p.tts.Synthesize(ctx, text, voice)
	if err != nil {
		return nil, fmt.Errorf("tts synthesize: %w", err)
	}

	return &GenerateResult{
		AudioData:  data,
		SampleRate: sampleRate,
		Latency:    time.Since(start),
	}, nil
}

// GenerateSFX generates a sound effect from a description.
func (p *Pipeline) GenerateSFX(ctx context.Context, description string, duration float64) (*GenerateResult, error) {
	if p.craft == nil {
		return nil, fmt.Errorf("no AudioCraft adapter configured")
	}

	start := time.Now()
	data, sampleRate, err := p.craft.Generate(ctx, description, duration)
	if err != nil {
		return nil, fmt.Errorf("audiocraft generate: %w", err)
	}

	return &GenerateResult{
		AudioData:  data,
		SampleRate: sampleRate,
		Latency:    time.Since(start),
	}, nil
}
