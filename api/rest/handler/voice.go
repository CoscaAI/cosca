package handler

import (
	"encoding/base64"
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/CoscaAI/cosca/internal/worldmodel/audio/tts"
)

// VoiceHandler exposes the native-Go TTS "speaking" primitive over HTTP. It is
// nil-safe: with no Speaker wired (default build, or TTS not opted-in) every
// /v1/voice/speaks request returns 503. The Speaker is always a valid *tts.TTS
// value (the package compiles in the default build as the disabled stub), so the
// REST layer never needs cgo.
type VoiceHandler struct {
	speaker *tts.Speaker
}

// NewVoiceHandler creates a VoiceHandler. Passing nil is fine — it degrades to
// 503 for every request (TTS not wired).
func NewVoiceHandler(spk *tts.Speaker) *VoiceHandler {
	return &VoiceHandler{speaker: spk}
}

// speakRequest is the POST /v1/voice/speaks body.
type speakRequest struct {
	Text  string  `json:"text"`
	Sid   int     `json:"sid,omitempty"`
	Speed float64 `json:"speed,omitempty"`
}

// spokeResponse is the JSON body for the default (non-WAV) response: the PCM is
// base64-encoded so the consumer can splice it / play it outside the COSCA.
type spokeResponse struct {
	SampleRate int    `json:"sample_rate"`
	Samples    int    `json:"samples"`
	DurationMS int64  `json:"duration_ms"`
	PCMBase64  string `json:"pcm_base64,omitempty"`
	Format     string `json:"format,omitempty"`
}

// Speaks handles POST /v1/voice/speaks:
//
//	{ "text": "Olá, eu sou o COSCA.", "sid": 0, "speed": 1.0 }
//
// Returns:
//   - 503 when the Speaker is not wired (default build / TTS not opted-in).
//   - `audio/wav` bytes when the request has `Accept: audio/wav` or `?format=wav`.
//   - JSON (JSON `{sample_rate, samples, duration_ms, pcm_base64}`) otherwise.
func (v *VoiceHandler) Speaks(w http.ResponseWriter, r *http.Request) {
	if v.speaker == nil || !v.speaker.Enabled() {
		writeVoiceError(w, http.StatusServiceUnavailable, "voice/tts disabled (build with -tags tts_sherpa and set perception.audio.tts.provider=sherpa)")
		return
	}

	var req speakRequest
	if r.Body != nil {
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		if err := dec.Decode(&req); err != nil {
			writeVoiceError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
			return
		}
	}
	req.Text = trimSpace(req.Text)
	if req.Text == "" {
		writeVoiceError(w, http.StatusBadRequest, "text is required")
		return
	}
	if req.Speed <= 0 {
		req.Speed = 1.0
	}

	res, err := v.speaker.Synthesize(req.Text, req.Sid, req.Speed)
	if err != nil {
		writeVoiceError(w, http.StatusInternalServerError, "synthesis failed: "+err.Error())
		return
	}
	if len(res.Samples) == 0 {
		writeVoiceError(w, http.StatusOK, "empty utterance")
		return
	}

	// WAV bytes when requested; otherwise base64 PCM.
	if wantsWAV(r) {
		wavBytes, werr := tts.EncodeWAV16(res.Samples, res.SampleRate)
		if werr != nil {
			writeVoiceError(w, http.StatusInternalServerError, "wav encode failed: "+werr.Error())
			return
		}
		w.Header().Set("Content-Type", "audio/wav")
		w.Header().Set("X-Cosca-Sample-Rate", intString(int64(res.SampleRate)))
		w.Header().Set("X-Cosca-Samples", intString(int64(len(res.Samples))))
		w.Header().Set("X-Cosca-Duration-Ms", intString(res.Duration.Milliseconds()))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(wavBytes)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cosca-Sample-Rate", intString(int64(res.SampleRate)))
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(spokeResponse{
		SampleRate: res.SampleRate,
		Samples:    len(res.Samples),
		DurationMS: res.Duration.Milliseconds(),
		PCMBase64:  base64.StdEncoding.EncodeToString(float32ToBytes(res.Samples)),
		Format:     "pcm_f32le",
	})
}

// wantsWAV reports whether the client wants raw WAV bytes.
func wantsWAV(r *http.Request) bool {
	if r.URL.Query().Get("format") == "wav" {
		return true
	}
	accept := r.Header.Get("Accept")
	return accept == "audio/wav" || accept == "audio/*" || accept == ""
}

func writeVoiceError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// float32ToBytes reinterprets a []float32 as a raw little-endian byte slice.
func float32ToBytes(in []float32) []byte {
	if len(in) == 0 {
		return nil
	}
	b := make([]byte, len(in)*4)
	for i, v := range in {
		u := math.Float32bits(v)
		b[i*4+0] = byte(u)
		b[i*4+1] = byte(u >> 8)
		b[i*4+2] = byte(u >> 16)
		b[i*4+3] = byte(u >> 24)
	}
	return b
}

func trimSpace(s string) string { return strings.TrimSpace(s) }

func intString(n int64) string { return strconv.FormatInt(n, 10) }
