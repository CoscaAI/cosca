package bus

import (
	"time"

	"github.com/CoscaAI/cosca/internal/worldmodel"
	"github.com/CoscaAI/cosca/internal/worldmodel/vision"
)

// ──────────────────────────────────────────────────────────────
// Modality
// ──────────────────────────────────────────────────────────────

// Modality identifies the sensory channel of an Observation.
type Modality string

const (
	// ModalityVision is the vision channel (native Go ONNX pipeline).
	ModalityVision Modality = "vision"
	// ModalityAudio is the audio channel (speech-to-text; Fase B sherpa).
	ModalityAudio Modality = "audio"
)

// String implements fmt.Stringer.
func (m Modality) String() string { return string(m) }

// ──────────────────────────────────────────────────────────────
// Observation
// ──────────────────────────────────────────────────────────────

// Observation is a single, immutable multimodal event captured by the bus. It
// is the atomic unit the ring buffer stores and the synchroniser matches.
type Observation struct {
	// Timestamp is the monotonic time at which the observation entered the bus
	// (for vision: the instant the bus ingested the frame; for audio: the
	// source-provided capture-start timestamp — see AudioSample). All fields are
	// on a single monotonic clock domain.
	Timestamp MonotonicTime `json:"timestamp"`
	// Modality is the sensory channel ("vision" or "audio").
	Modality Modality `json:"modality"`
	// Duration is the observation's temporal extent in wall-clock terms
	// (vision processing time / audio segment length). Zero = instantaneous.
	Duration time.Duration `json:"duration"`
	// Sequence is a process-global monotonic counter assigned by the bus. It
	// is the stable handle for a specific observation (used by WindowRef).
	Sequence uint64 `json:"sequence"`
	// Confidence is the detection/interpolation confidence in [0,1].
	Confidence float64 `json:"confidence"`
	// Payload is the typed sensor payload (a nullable union).
	Payload Payload `json:"payload"`
	// Meta carries free-form, non-serial critical metadata (may be nil).
	Meta map[string]string `json:"meta,omitempty"`
}

// ──────────────────────────────────────────────────────────────
// Payload (typed, nil-safe union)
// ──────────────────────────────────────────────────────────────

// Payload is a nil-safe union of the two sensor payloads. Exactly one of
// Vision/Audio is non-nil for any Observation (enforced at construction).
type Payload struct {
	// Vision is the vision Observation (non-nil iff Modality == ModalityVision).
	Vision *vision.Observation `json:"vision,omitempty"`
	// Audio is the audio payload (non-nil iff Modality == ModalityAudio).
	Audio *AudioPayload `json:"audio,omitempty"`
}

// Visual returns the vision payload, or nil when the observation is not vision.
func (p Payload) Visual() *vision.Observation { return p.Vision }

// AudioPayloadView returns the audio payload, or nil when not audio.
func (p Payload) AudioPayloadView() *AudioPayload { return p.Audio }

// ──────────────────────────────────────────────────────────────
// Audio payload
// ──────────────────────────────────────────────────────────────

// AudioPayload is the speech/audio recognition result carried by an audio
// Observation. It is the transport for Fase B (sherpa) ASR; in Fase A it is a
// no-op source, so it stays structurally ready but rarely populated.
type AudioPayload struct {
	// Text is the full transcript (empty when only partial/tokens are known).
	Text string `json:"text,omitempty"`
	// Tokens are the word-level timings (offset relative to the segment start).
	Tokens []Token `json:"tokens,omitempty"`
	// IsFinal is true for a finalised segment, false for a partial result.
	IsFinal bool `json:"is_final"`
	// SegmentID is a source-given segment identifier (stable across partials).
	SegmentID uint64 `json:"segment_id"`
	// Language is the detected language BCP-47 code ("" = unknown).
	Language string `json:"language,omitempty"`
	// SegmentDuration is the audio segment length (for overlap matching). When
	// zero the bus estimates it from the tokens (see Observation.duration).
	SegmentDuration time.Duration `json:"segment_duration,omitempty"`
}

// Token is a word-level ASR token with a time offset from the segment start.
type Token struct {
	// Word is the recognised word.
	Word string `json:"word"`
	// Offset is the offset of the word from the segment start.
	Offset time.Duration `json:"offset"`
	// Confidence is the recogniser confidence for this word in [0,1].
	Confidence float64 `json:"confidence"`
}

// duration estimates the audio segment length from the payload: the source
// declares it explicitly (SegmentDuration) or, falling back, the last token
// offset plus a per-word estimate. Zero -> a conservative default.
func (a *AudioPayload) duration() time.Duration {
	if a == nil {
		return 0
	}
	if a.SegmentDuration > 0 {
		return a.SegmentDuration
	}
	var last time.Duration
	for i := range a.Tokens {
		off := a.Tokens[i].Offset
		if off > last {
			last = off
		}
	}
	if last > 0 {
		// A rough per-word extent (~140ms) so a lone token still has a span.
		return last + 140*time.Millisecond
	}
	return 0
}

// textHint returns the transcript (or a slice of it) as the sync text hint.
func (a *AudioPayload) textHint() string {
	if a == nil {
		return ""
	}
	if a.Text != "" {
		return a.Text
	}
	// Build from tokens (fallback for partial results with no full text).
	joined := ""
	for i := range a.Tokens {
		if i > 0 {
			joined += " "
		}
		joined += a.Tokens[i].Word
	}
	if len(joined) > 256 {
		joined = joined[:256] + "…"
	}
	return joined
}

// ──────────────────────────────────────────────────────────────
// WorldState (the bus projection)
// ──────────────────────────────────────────────────────────────

// WorldState is the projected, synchronised world view the bus publishes to
// its subscribers and exposes at /v1/perception/bus/state. It is a snapshot:
// the ring window of recent observations plus the latest vision and audio, tied
// together by overlap relations.
type WorldState struct {
	// Timestamp is the monotonic time of this projection.
	Timestamp MonotonicTime `json:"timestamp"`
	// Version is a monotonic revision counter (fencing: detect stale views).
	Version uint64 `json:"version"`
	// LastSeenAt is the most recent monotonic ingest across all modalities.
	LastSeenAt MonotonicTime `json:"last_seen_at"`
	// Window is a defensive copy of the ring's current observations (sorted by
	// timestamp; may be partial/empty).
	Window []Observation `json:"window,omitempty"`
	// Vision is the most recent vision payload (nil when none yet).
	Vision *vision.Observation `json:"vision,omitempty"`
	// Audio is the most recent audio payload (nil when none yet).
	Audio *AudioPayload `json:"audio,omitempty"`
	// Relations are the audio↔vision overlap bindings ("what was seen when X
	// was heard"). Empty when there is no overlap in the window.
	Relations []MultiRel `json:"relations,omitempty"`
	// Degraded is true when the bus is running but with a degraded signal
	// (e.g. missing source, nil payload, a source that was expected but silent).
	Degraded bool `json:"degraded"`
	// Warnings are non-fatal degradation notes.
	Warnings []string `json:"warnings,omitempty"`
	// Metrics carries bus telemetry (observation counts per modality, window
	// length). Always present on a live update.
	Metrics *Metrics `json:"metrics,omitempty"`
}

// MultiRel is a sync relation: one audio segment bound to the vision
// observations that temporally overlapped it. This is the answer to "what was
// the agent seeing when it heard the word/phrase X".
type MultiRel struct {
	// AudioSeg is the audio segment identifier (from AudioPayload.SegmentID).
	AudioSeg uint64 `json:"audio_seg"`
	// TextHint is the related transcript (full or truncated).
	TextHint string `json:"text_hint"`
	// AudioConf is the audio observation confidence in [0,1].
	AudioConf float64 `json:"audio_conf"`
	// Overlap lists the vision windows that overlapped this audio segment.
	Overlap []WindowRef `json:"overlap,omitempty"`
	// Confidence is the aggregate overlap confidence in [0,1] (fraction of the
	// audio segment covered by the vision window(s)).
	Confidence float64 `json:"confidence"`
	// Window is [before, after] tolerance around the audio segment's midpoint
	// that was used to bind vision (for the "was seeing" query).
	Window [2]time.Duration `json:"window"`
}

// WindowRef references a single vision observation that overlapped an audio
// segment. It carries the entities that were being seen at that instant.
type WindowRef struct {
	// Sequence is the bus-assigned sequence of the vision observation.
	Sequence uint64 `json:"sequence"`
	// Timestamp is the monotonic timestamp of the vision observation.
	Timestamp MonotonicTime `json:"timestamp"`
	// Entities are the world entities detected in that frame (defensive copy).
	Entities []worldmodel.WorldEntity `json:"entities,omitempty"`
}

// Metrics is the bus telemetry snapshot.
type Metrics struct {
	// Observations is the count of observations currently in the ring window.
	Observations uint64 `json:"observations"`
	// Vision is the number of vision observations ingested (process lifetime).
	Vision uint64 `json:"vision"`
	// Audio is the number of audio observations ingested (process lifetime).
	Audio uint64 `json:"audio"`
	// LastVisionAt is the monotonic ingest time of the last vision observation.
	LastVisionAt MonotonicTime `json:"last_vision_at,omitempty"`
	// LastAudioAt is the monotonic ingest time of the last audio observation.
	LastAudioAt MonotonicTime `json:"last_audio_at,omitempty"`
	// WindowSeconds is the configured ring window length in seconds.
	WindowSeconds float64 `json:"window_seconds"`
	// Degraded is the number of degraded observations ingested.
	Degraded uint64 `json:"degraded"`
}
