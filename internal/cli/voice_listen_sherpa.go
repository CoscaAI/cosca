//go:build stt_sherpa

package cli

// This file compiles only with `-tags stt_sherpa` (native-Go sherpa STT). It
// implements `cosca voice listen` — the live proof that COSCA "ouve em tempo
// real": it opens the microphone (native Windows winmm/WaveIn), streams the PCM
// into the sherpa streaming recognizer, and prints partial/final transcriptions
// to stdout until Ctrl-C.

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/worldmodel/audio/mic"
)

// newVoiceListenCommand returns `cosca voice listen`, the live microphone →
// stream transcription command.
func newVoiceListenCommand() *cobra.Command {
	var device int
	var duration time.Duration
	var quiet bool

	cmd := &cobra.Command{
		Use:   "listen",
		Short: "Ouvir o microfone e transcrever ao vivo (STT streaming local)",
		Long: `Abre o microfone, captura PCM em tempo real (backend nativo Windows
winmm/WaveIn, zero dependência externa) e alimenta o STT streaming sherpa-onnx.
Imprime a transcrição PARCIAL no stdout conforme o Don fala, e a FINAL quando o
endpoint detecta o fim da fala. É a prova viva do loop fechado:

  mic → PCM → stt.AudioSource.PushPCM → sherpa → bus.AudioSample (parcial/final)

Para usar, compile com -tags stt_sherpa e configure perception.audio.stt.provider
=sherpa (com model_dir apontando para um modelo ASR streaming local). Ctrl-C para.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := LoadConfigForVoice()
			if err != nil {
				return err
			}
			if cfg.Perception.Audio.STT.Provider != "sherpa" {
				return fmt.Errorf("STT nativo desabilitado: defina perception.audio.stt.provider=sherpa (e compile com -tags stt_sherpa)")
			}

			logger := zerolog.New(zerolog.Nop())
			if !quiet {
				listDevices(cmd)
			}

			sttSrc, err := newSherpaSTTSource(cfg.Perception, logger)
			if err != nil {
				return err
			}
			if sttSrc == nil {
				return fmt.Errorf("STT desabilitado (sherpa não configurado)")
			}
			defer sttSrc.Close()

			mc, err := mic.NewCapture(mic.Config{
				Device:     device,
				SampleRate: cfg.Perception.Audio.STT.SampleRate,
				Channels:   1,
				ChunkMS:    cfg.Perception.Audio.Mic.ChunkMS,
			})
			if err != nil {
				return fmt.Errorf("abrir microfone falhou: %w", err)
			}
			defer mc.Close()

			src := mic.NewMicrophoneAudioSource(mc, sttSrc, logger)
			defer src.Close()

			fmt.Fprintln(cmd.OutOrStdout(), "COSCA está ouvindo (Ctrl-C para parar)…")

			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
			defer cancel()

			samples := src.Stream()
			var timer *time.Timer
			if duration > 0 {
				timer = time.NewTimer(duration)
				defer timer.Stop()
			}

			for {
				select {
				case <-ctx.Done():
					return nil
				default:
				}
				if timer != nil {
					select {
					case <-timer.C:
						return nil
					default:
					}
				}
				select {
				case <-ctx.Done():
					return nil
				case <-timer.C:
					return nil
				case sample, ok := <-samples:
					if !ok {
						return nil
					}
					text := sample.Payload.Text
					if text == "" {
						continue
					}
					if sample.Payload.IsFinal {
						fmt.Fprintf(cmd.OutOrStdout(), "\r[final]  %s\n", text)
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "\r[parcial] %s", text)
					}
				}
			}
		},
	}

	cmd.Flags().IntVar(&device, "device", -1, "índice do dispositivo de captura (default -1 = dispositivo padrão do sistema)")
	cmd.Flags().DurationVar(&duration, "duration", 0, "parar após esta duração (ex. 10s; default = até Ctrl-C)")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "não listar dispositivos de entrada")
	return cmd
}

// listDevices prints the available capture devices (diagnostic helper).
func listDevices(cmd *cobra.Command) {
	devs, err := mic.ListDevices()
	if err != nil || len(devs) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "aviso: nenhum dispositivo de captura encontrado (%v)\n", err)
		return
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Dispositivos de captura:\n")
	for _, d := range devs {
		fmt.Fprintf(cmd.OutOrStdout(), "  [%d] %s (máx %d canais)\n", d.Index, d.Name, d.MaxChannels)
	}
}
