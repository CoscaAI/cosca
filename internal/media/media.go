// Package media implementa a Media Engine (§16 do manifesto
// Creative/Scientific/Media) — Fase 1, etapa 1.8.
//
// Princípio P5 do Blueprint: "I/O e codecs sempre via FFmpeg — nunca
// reescrever parsing de mídia". Esta engine PROBE (ffprobe) e PIPELINE
// (ffmpeg) mídia de forma determinística e observável:
//
//	DECODE → PROCESS → AI → FILTER → ENCODE → VALIDATE (§16)
//
// Cobre vídeo e áudio (imagem/PDF/SVG já estão na cosca-media). Integra com o
// Node Graph (1.6): cada operação é um nó (extract_audio, transcode,
// extract_frame...). Usa aceleração VA-API/OpenCL quando o GPU Probe (1.5)
// detecta suporte (§17).
package media

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// =============================================================================
// Probe — ffprobe
// =============================================================================

// Stream descreve um stream de mídia.
type Stream struct {
	Index     int    `json:"index"`
	CodecType string `json:"codec_type"` // video | audio | subtitle | data
	CodecName string `json:"codec_name"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Duration  string `json:"duration,omitempty"`
	BitRate   string `json:"bit_rate,omitempty"`
	Channels  int    `json:"channels,omitempty"`
	SampleRate string `json:"sample_rate,omitempty"`
}

// Format descreve o container.
type Format struct {
	Name       string `json:"name"`
	Duration   string `json:"duration,omitempty"`
	Size       string `json:"size,omitempty"`
	BitRate    string `json:"bit_rate,omitempty"`
	ProbeScore int    `json:"probe_score,omitempty"`
}

// Info é o resultado do probe de um arquivo de mídia.
type Info struct {
	Path    string   `json:"path"`
	Format  Format   `json:"format"`
	Streams []Stream `json:"streams"`
	// HasVideo/HasAudio são conveniências derivadas.
	HasVideo bool `json:"has_video"`
	HasAudio bool `json:"has_audio"`
}

// VideoStream devolve o primeiro stream de vídeo (nil se não houver).
func (i *Info) VideoStream() *Stream {
	for idx := range i.Streams {
		if i.Streams[idx].CodecType == "video" {
			return &i.Streams[idx]
		}
	}
	return nil
}

// AudioStream devolve o primeiro stream de áudio (nil se não houver).
func (i *Info) AudioStream() *Stream {
	for idx := range i.Streams {
		if i.Streams[idx].CodecType == "audio" {
			return &i.Streams[idx]
		}
	}
	return nil
}

// DurationSeconds devolve a duração em segundos (0 se desconhecida).
func (i *Info) DurationSeconds() float64 {
	d, err := strconv.ParseFloat(i.Format.Duration, 64)
	if err != nil {
		return 0
	}
	return d
}

// ffprobeJSON representa a saída crua do ffprobe.
type ffprobeJSON struct {
	Format struct {
		Name       string `json:"format_name"`
		Duration   string `json:"duration"`
		Size       string `json:"size"`
		BitRate    string `json:"bit_rate"`
		ProbeScore int    `json:"probe_score"`
	} `json:"format"`
	Streams []struct {
		Index      int    `json:"index"`
		CodecType  string `json:"codec_type"`
		CodecName  string `json:"codec_name"`
		Width      int    `json:"width"`
		Height     int    `json:"height"`
		Duration   string `json:"duration"`
		BitRate    string `json:"bit_rate"`
		Channels   int    `json:"channels"`
		SampleRate string `json:"sample_rate"`
	} `json:"streams"`
}

// Probe analisa um arquivo de mídia via ffprobe (JSON). Retorna erro se o
// arquivo não for mídia válida ou o ffprobe não existir.
func Probe(ctx context.Context, path string) (*Info, error) {
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		return nil, fmt.Errorf("media: ffprobe not found: %w", err)
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("media: %w", err)
	}

	args := []string{
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	}
	out, err := exec.CommandContext(ctx, ffprobe, args...).Output()
	if err != nil {
		return nil, fmt.Errorf("media: probe %s: %w", path, err)
	}

	var raw ffprobeJSON
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("media: parse ffprobe output: %w", err)
	}

	info := &Info{Path: path}
	info.Format.Name = raw.Format.Name
	info.Format.Duration = raw.Format.Duration
	info.Format.Size = raw.Format.Size
	info.Format.BitRate = raw.Format.BitRate
	info.Format.ProbeScore = raw.Format.ProbeScore

	for _, s := range raw.Streams {
		st := Stream{
			Index:      s.Index,
			CodecType:  s.CodecType,
			CodecName:  s.CodecName,
			Width:      s.Width,
			Height:     s.Height,
			Duration:   s.Duration,
			BitRate:    s.BitRate,
			Channels:   s.Channels,
			SampleRate: s.SampleRate,
		}
		info.Streams = append(info.Streams, st)
		if s.CodecType == "video" {
			info.HasVideo = true
		}
		if s.CodecType == "audio" {
			info.HasAudio = true
		}
	}
	return info, nil
}

// =============================================================================
// Pipelines — ffmpeg
// =============================================================================

// PipeOptions configura um pipeline ffmpeg.
type PipeOptions struct {
	// HWAccel habilita aceleração VA-API (AMD) quando disponível (§17).
	HWAccel bool
	// Quality é o nível de qualidade (repassado como crf/bitrate).
	Quality string // "preview" | "draft" | "final"
	// ExtraArgs são args extras passados ao ffmpeg.
	ExtraArgs []string
}

// pipeExec roda um pipeline ffmpeg. O stderr é capturado e exposto apenas em
// caso de erro (o banner do ffmpeg não polui a CLI).
func pipeExec(ctx context.Context, args []string) error {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("media: ffmpeg not found: %w", err)
	}
	cmd := exec.CommandContext(ctx, ffmpeg, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return fmt.Errorf("media: ffmpeg: %w: %s", err, msg)
		}
		return fmt.Errorf("media: ffmpeg: %w", err)
	}
	return nil
}

// ExtractAudio extrai a trilha de áudio de um vídeo (pipeline §21:
// VIDEO → EXTRACT_AUDIO → WHISPER...). Saída wav (para ASR) ou mp3/aac.
func ExtractAudio(ctx context.Context, in, out string, o PipeOptions) error {
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return fmt.Errorf("media: mkdir: %w", err)
	}
	ext := strings.ToLower(filepath.Ext(out))
	args := []string{"-y"}
	if o.HWAccel {
		args = append(args, "-hwaccel", "auto")
	}
	args = append(args, "-i", in, "-vn")
	switch ext {
	case ".wav":
		args = append(args, "-c:a", "pcm_s16le")
	case ".mp3":
		args = append(args, "-c:a", "libmp3lame", "-q:a", "2")
	default:
		args = append(args, "-c:a", "aac", "-b:a", "192k")
	}
	args = append(args, o.ExtraArgs...)
	args = append(args, out)
	return pipeExec(ctx, args)
}

// Transcode re-encoda um arquivo (formato/codec). Preserva streams de mídia.
func Transcode(ctx context.Context, in, out string, o PipeOptions) error {
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return fmt.Errorf("media: mkdir: %w", err)
	}
	args := []string{"-y"}
	if o.HWAccel {
		args = append(args, "-hwaccel", "auto")
	}
	args = append(args, "-i", in)
	// Qualidade como contrato (§22): preview/draft/final → crf.
	switch o.Quality {
	case "preview":
		args = append(args, "-crf", "32", "-preset", "ultrafast")
	case "final":
		args = append(args, "-crf", "18", "-preset", "slow")
	default: // draft
		args = append(args, "-crf", "23", "-preset", "medium")
	}
	args = append(args, o.ExtraArgs...)
	args = append(args, out)
	return pipeExec(ctx, args)
}

// ExtractFrame extrai um frame de um vídeo num instante t (thumbnail/still).
func ExtractFrame(ctx context.Context, in, out string, t string, o PipeOptions) error {
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return fmt.Errorf("media: mkdir: %w", err)
	}
	args := []string{"-y"}
	if o.HWAccel {
		args = append(args, "-hwaccel", "auto")
	}
	args = append(args, "-ss", t, "-i", in, "-frames:v", "1", "-q:v", "2")
	args = append(args, o.ExtraArgs...)
	args = append(args, out)
	return pipeExec(ctx, args)
}

// AudioPipeOptions configura um pipeline de áudio.
type AudioPipeOptions struct {
	// Volume em dB (ex.: "2dB") ou fator (ex.: "1.5"). Vazio = sem ajuste.
	Volume string
	// SampleRate de saída (ex.: "44100"). Vazio = mantém.
	SampleRate string
	// Channels de saída (ex.: "2" estéreo, "1" mono). Vazio = mantém.
	Channels string
}

// ConvertAudio converte um arquivo de áudio para outro formato/codec,
// com opções de volume/sample-rate/canais (§9 — DAW básica).
func ConvertAudio(ctx context.Context, in, out string, o AudioPipeOptions) error {
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return fmt.Errorf("media: mkdir: %w", err)
	}
	ext := strings.ToLower(filepath.Ext(out))
	args := []string{"-y", "-i", in}
	switch ext {
	case ".mp3":
		args = append(args, "-c:a", "libmp3lame", "-q:a", "2")
	case ".ogg", ".opus":
		args = append(args, "-c:a", "libopus")
	case ".flac":
		args = append(args, "-c:a", "flac")
	case ".wav":
		args = append(args, "-c:a", "pcm_s16le")
	default:
		args = append(args, "-c:a", "aac", "-b:a", "192k")
	}
	if o.Volume != "" {
		args = append(args, "-af", "volume="+o.Volume)
	}
	if o.SampleRate != "" {
		args = append(args, "-ar", o.SampleRate)
	}
	if o.Channels != "" {
		args = append(args, "-ac", o.Channels)
	}
	args = append(args, out)
	return pipeExec(ctx, args)
}

// =============================================================================
// Validação (§16: ENCODE → VALIDATE)
// =============================================================================

// Validate verifica que um arquivo de mídia é íntegro (probe + presença de
// streams). Retorna erro para arquivos corrompidos ou não-mídia.
func Validate(ctx context.Context, path string) (*Info, error) {
	info, err := Probe(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("media: validate: %w", err)
	}
	if len(info.Streams) == 0 {
		return nil, fmt.Errorf("media: validate %s: no streams (corrupted or not media)", path)
	}
	return info, nil
}
