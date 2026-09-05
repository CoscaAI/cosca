package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/media"
)

// NewMediaCommand creates the `cosca media` command group — Media Engine (§16
// do manifesto Creative/Scientific/Media), Fase 1 etapa 1.8.
//
// NOTA: a cosca-media (Python, visual: imagem/PDF/SVG) já existe. Este comando
// Go cobre VÍDEO e ÁUDIO via ffmpeg/ffprobe (P5: nunca reescrever parsing).
func NewMediaCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "media",
		Short: "Media Engine — vídeo/áudio via ffmpeg (§16)",
		Long: `Media Engine (§16) — probe e pipelines de VÍDEO e ÁUDIO via ffmpeg.

Pipeline: DECODE → PROCESS → AI → FILTER → ENCODE → VALIDATE.
Imagem/PDF/SVG continuam na cosca-media (Python); este comando é a extensão
de vídeo/áudio em Go, pronta para virar nós do Node Graph (extract_audio,
transcode, extract_frame, whisper...).

Subcommands:
  probe <file>         Show media info (ffprobe: streams, codecs, duração)
  extract-audio <in> <out>  Extract the audio track (wav/mp3/aac)
  frame <in> <out> <t> Extract a frame at time t (e.g. 00:00:01.5)
  transcode <in> <out> Re-encode (crf por qualidade)
  validate <file>      Verify media integrity`,
		Example: `  cosca media probe intro.mp4
  cosca media extract-audio intro.mp4 audio.wav
  cosca media frame intro.mp4 frame.png 00:00:01
  cosca media transcode intro.mp4 web.mkv --quality final
  cosca media validate intro.mp4`,
	}
	cmd.AddCommand(
		NewMediaProbeCommand(),
		NewMediaExtractAudioCommand(),
		NewMediaFrameCommand(),
		NewMediaTranscodeCommand(),
		NewMediaValidateCommand(),
	)
	return cmd
}

// NewMediaProbeCommand creates `cosca media probe <file>`.
func NewMediaProbeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "probe <file>",
		Short: "Show media info (ffprobe)",
		Example: `  cosca media probe intro.mp4
  cosca media probe intro.mp4 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			info, err := media.Probe(context.Background(), args[0])
			if err != nil {
				return err
			}
			if useJSON {
				return printJSON(cmd, info)
			}

			formatter.Header(fmt.Sprintf("Media — %s", filepath.Base(args[0])))
			formatter.KeyValue("Format", info.Format.Name)
			if info.Format.Duration != "" {
				formatter.KeyValue("Duration", info.Format.Duration+"s")
			}
			if info.Format.Size != "" {
				formatter.KeyValue("Size", formatSize(mustParseSize(info.Format.Size)))
			}
			formatter.Println("")
			for _, s := range info.Streams {
				kind := "? " + s.CodecType
				switch s.CodecType {
				case "video":
					kind = fmt.Sprintf("video %dx%d (%s)", s.Width, s.Height, s.CodecName)
				case "audio":
					kind = fmt.Sprintf("audio %s ch=%d (%s)", s.CodecName, s.Channels, s.SampleRate)
				}
				formatter.Bullet(kind)
			}
			return nil
		},
	}
	return cmd
}

// NewMediaExtractAudioCommand creates `cosca media extract-audio <in> <out>`.
func NewMediaExtractAudioCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "extract-audio <in> <out>",
		Short:   "Extract the audio track (wav/mp3/aac)",
		Example: `  cosca media extract-audio intro.mp4 audio.wav`,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			if err := media.ExtractAudio(context.Background(), args[0], args[1], media.PipeOptions{}); err != nil {
				return err
			}
			formatter.Success(fmt.Sprintf("Audio extracted → %s", args[1]))
			return nil
		},
	}
	return cmd
}

// NewMediaFrameCommand creates `cosca media frame <in> <out> <t>`.
func NewMediaFrameCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "frame <in> <out> <time>",
		Short:   "Extract a frame at time t",
		Example: `  cosca media frame intro.mp4 frame.png 00:00:01`,
		Args:    cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			if err := media.ExtractFrame(context.Background(), args[0], args[1], args[2], media.PipeOptions{}); err != nil {
				return err
			}
			formatter.Success(fmt.Sprintf("Frame at %s → %s", args[2], args[1]))
			return nil
		},
	}
	return cmd
}

// NewMediaTranscodeCommand creates `cosca media transcode <in> <out>`.
func NewMediaTranscodeCommand() *cobra.Command {
	var quality string
	cmd := &cobra.Command{
		Use:   "transcode <in> <out>",
		Short: "Re-encode (crf por qualidade)",
		Example: `  cosca media transcode intro.mp4 web.mkv
  cosca media transcode intro.mp4 hi.mp4 --quality final`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			if quality != "preview" && quality != "draft" && quality != "final" {
				return fmt.Errorf("invalid quality %q (valid: preview, draft, final)", quality)
			}
			if err := media.Transcode(context.Background(), args[0], args[1], media.PipeOptions{Quality: quality}); err != nil {
				return err
			}
			formatter.Success(fmt.Sprintf("Transcoded (%s) → %s", quality, args[1]))
			return nil
		},
	}
	cmd.Flags().StringVar(&quality, "quality", "draft", "Quality: preview, draft, final")
	return cmd
}

// NewMediaValidateCommand creates `cosca media validate <file>`.
func NewMediaValidateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "validate <file>",
		Short:   "Verify media integrity",
		Example: `  cosca media validate intro.mp4`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			info, err := media.Validate(context.Background(), args[0])
			if err != nil {
				return err
			}
			formatter.Success(fmt.Sprintf("%s is valid media (%d streams, %s)", args[0], len(info.Streams), info.Format.Name))
			return nil
		},
	}
	return cmd
}

// mustParseSize converte bytes (string) para int64, tolerante a falha.
func mustParseSize(s string) int64 {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int64(c-'0')
	}
	return n
}

var _ = os.Stat // reservado
