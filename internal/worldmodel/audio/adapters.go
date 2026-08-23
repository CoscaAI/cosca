package audio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/CoscaAI/cosca/internal/worldmodel"
)

// ──────────────────────────────────────────────────────────────
// Subprocess helper
// ──────────────────────────────────────────────────────────────

type subprocessRequest struct {
	Command string          `json:"command"`
	Payload json.RawMessage `json:"payload"`
}

type subprocessResponse struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error string          `json:"error,omitempty"`
}

func runSubprocess(ctx context.Context, scriptPath string, req subprocessRequest, timeout time.Duration) (json.RawMessage, error) {
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	reqBytes, _ := json.Marshal(req)
	cmd := exec.CommandContext(ctx, "python3", scriptPath)
	cmd.Stdin = bytes.NewReader(reqBytes)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("subprocess failed: %w, stderr: %s", err, stderr.String())
	}

	var resp subprocessResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if !resp.OK {
		return nil, fmt.Errorf("subprocess error: %s", resp.Error)
	}
	return resp.Data, nil
}

// ──────────────────────────────────────────────────────────────
// Whisper Adapter (Speech to Text)
// ──────────────────────────────────────────────────────────────

// WhisperConfig configures the Whisper adapter.
type WhisperConfig struct {
	Model  string `json:"model"`  // "tiny", "base", "small", "medium", "large"
	Device string `json:"device"` // "cpu", "cuda", "mps"
	Script string `json:"script"`
}

// WhisperAdapter wraps whisper.cpp for speech-to-text.
type WhisperAdapter struct {
	config WhisperConfig
}

// NewWhisperAdapter creates a Whisper adapter.
func NewWhisperAdapter(config WhisperConfig) *WhisperAdapter {
	if config.Script == "" {
		config.Script = "adapters/audio/whisper.py"
	}
	return &WhisperAdapter{config: config}
}

// Transcribe converts audio bytes to text.
func (a *WhisperAdapter) Transcribe(ctx context.Context, audioData []byte, sampleRate, channels int) (string, error) {
	payload, _ := json.Marshal(map[string]any{
		"audio":       audioData,
		"sample_rate": sampleRate,
		"channels":    channels,
	})

	data, err := runSubprocess(ctx, a.config.Script, subprocessRequest{
		Command: "transcribe",
		Payload: payload,
	}, 30*time.Second)
	if err != nil {
		return "", fmt.Errorf("whisper transcribe: %w", err)
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}

	return result.Text, nil
}

// ──────────────────────────────────────────────────────────────
// Diffusion Fields Adapter (Spatial Audio Analysis)
// ──────────────────────────────────────────────────────────────

// DiffFieldsConfig configures the Diffusion Fields adapter.
type DiffFieldsConfig struct {
	Device string `json:"device"`
	Script string `json:"script"`
}

// DiffFieldsAdapter wraps Diffusion Fields for spatial audio analysis.
type DiffFieldsAdapter struct {
	config DiffFieldsConfig
}

// NewDiffFieldsAdapter creates a Diffusion Fields adapter.
func NewDiffFieldsAdapter(config DiffFieldsConfig) *DiffFieldsAdapter {
	if config.Script == "" {
		config.Script = "adapters/audio/difffields.py"
	}
	return &DiffFieldsAdapter{config: config}
}

// Analyze analyzes spatial properties of audio.
func (a *DiffFieldsAdapter) Analyze(ctx context.Context, audioData []byte, sampleRate, channels int) (worldmodel.SpatialAudio, error) {
	payload, _ := json.Marshal(map[string]any{
		"audio":       audioData,
		"sample_rate": sampleRate,
		"channels":    channels,
	})

	data, err := runSubprocess(ctx, a.config.Script, subprocessRequest{
		Command: "analyze",
		Payload: payload,
	}, 20*time.Second)
	if err != nil {
		return worldmodel.SpatialAudio{}, fmt.Errorf("difffields analyze: %w", err)
	}

	var result struct {
		Direction    [3]float64 `json:"direction"`
		Distance     float64    `json:"distance"`
		Intensity    float64    `json:"intensity"`
		Spatializer  string     `json:"spatializer"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return worldmodel.SpatialAudio{}, err
	}

	return worldmodel.SpatialAudio{
		Direction:   worldmodel.Vec3{X: result.Direction[0], Y: result.Direction[1], Z: result.Direction[2]},
		Distance:    result.Distance,
		Intensity:   result.Intensity,
		Spatializer: result.Spatializer,
	}, nil
}

// ──────────────────────────────────────────────────────────────
// Coqui TTS Adapter (Text to Speech)
// ──────────────────────────────────────────────────────────────

// CoquiTTSConfig configures the Coqui TTS adapter.
type CoquiTTSConfig struct {
	Model  string `json:"model"`  // XTTS model
	Device string `json:"device"` // "cpu", "cuda", "mps"
	Script string `json:"script"`
}

// CoquiAdapter wraps Coqui TTS for text-to-speech.
type CoquiAdapter struct {
	config CoquiTTSConfig
}

// NewCoquiAdapter creates a Coqui TTS adapter.
func NewCoquiAdapter(config CoquiTTSConfig) *CoquiAdapter {
	if config.Script == "" {
		config.Script = "adapters/audio/coqui_tts.py"
	}
	return &CoquiAdapter{config: config}
}

// Synthesize converts text to audio bytes.
func (a *CoquiAdapter) Synthesize(ctx context.Context, text string, voice string) ([]byte, int, error) {
	payload, _ := json.Marshal(map[string]any{
		"text":  text,
		"voice": voice,
	})

	data, err := runSubprocess(ctx, a.config.Script, subprocessRequest{
		Command: "synthesize",
		Payload: payload,
	}, 30*time.Second)
	if err != nil {
		return nil, 0, fmt.Errorf("coqui synthesize: %w", err)
	}

	var result struct {
		Audio      []byte `json:"audio"`
		SampleRate int    `json:"sample_rate"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, 0, err
	}

	return result.Audio, result.SampleRate, nil
}

// ──────────────────────────────────────────────────────────────
// AudioCraft Adapter (Sound Effects)
// ──────────────────────────────────────────────────────────────

// AudioCraftConfig configures the AudioCraft adapter.
type AudioCraftConfig struct {
	Device string `json:"device"`
	Script string `json:"script"`
}

// AudioCraftAdapter wraps AudioCraft for sound effect generation.
type AudioCraftAdapter struct {
	config AudioCraftConfig
}

// NewAudioCraftAdapter creates an AudioCraft adapter.
func NewAudioCraftAdapter(config AudioCraftConfig) *AudioCraftAdapter {
	if config.Script == "" {
		config.Script = "adapters/audio/audiocraft.py"
	}
	return &AudioCraftAdapter{config: config}
}

// Generate creates a sound effect from a description.
func (a *AudioCraftAdapter) Generate(ctx context.Context, description string, duration float64) ([]byte, int, error) {
	payload, _ := json.Marshal(map[string]any{
		"description": description,
		"duration":    duration,
	})

	data, err := runSubprocess(ctx, a.config.Script, subprocessRequest{
		Command: "generate",
		Payload: payload,
	}, 60*time.Second)
	if err != nil {
		return nil, 0, fmt.Errorf("audiocraft generate: %w", err)
	}

	var result struct {
		Audio      []byte `json:"audio"`
		SampleRate int    `json:"sample_rate"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, 0, err
	}

	return result.Audio, result.SampleRate, nil
}
