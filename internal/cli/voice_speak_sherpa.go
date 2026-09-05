//go:build tts_sherpa

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

// newVoiceSpeakCommand returns a `cosca voice speak "<texto>"` subcommand that
// synthesizes PT-BR speech natively in Go (sherpa-onnx) and writes a .wav. This
// one is built ONLY behind the `tts_sherpa` tag (the noop twin has the same
// signature for the default build).
func newVoiceSpeakCommand() *cobra.Command {
	var out string
	var sid int
	var speed float64
	var quiet bool

	cmd := &cobra.Command{
		Use:   "speak \"<texto>\"",
		Short: "Sintetizar fala PT-BR em Go nativo (sherpa-onnx) e gravar um .wav",
		Long: `Sintetiza o texto em fala a partir de um modelo local (VITS/piper/kokoro)
carregado de perception.audio.tts.model_dir e grava um .wav em --out (default
~/.cosca/out/tts_<ts>.wav). Tudo local — sem internet, sem Python.

O critério de aceite: um arquivo .wav de áudio PT-BR real é gerado.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			text := args[0]
			if out == "" {
				out = defaultTTSOutPath()
			}
			// Carrega a config para resolver perception.audio.tts (opt-in).
			cfg, err := LoadConfigForVoice()
			if err != nil {
				return err
			}
			spk, err := buildTtsSpeaker(cfg.Perception, newVoiceLogger())
			if err != nil {
				return err
			}
			if spk == nil {
				return fmt.Errorf("TTS nativo desabilitado: defina perception.audio.tts.provider=sherpa (e compile com -tags tts_sherpa)")
			}
			defer spk.Close()

			res, err := spk.SpeakToFile(text, out, sid, speed)
			if err != nil {
				return fmt.Errorf("síntese falhou: %w", err)
			}
			if !quiet {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"✓ COSCA falou %q → %s (%.1f s, %d samples @ %d Hz)\n",
					text, out, res.Duration.Seconds(), len(res.Samples), res.SampleRate)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&out, "out", "", "caminho do .wav (default ~/.cosca/out/tts_<ts>.wav)")
	cmd.Flags().IntVar(&sid, "sid", 0, "id do speaker/voz (default 0)")
	cmd.Flags().Float64Var(&speed, "speed", 1.0, "velocidade da fala (1.0 = normal; <1 mais rápido, >1 mais lento)")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "não imprimir o resumo")
	return cmd
}

// defaultTTSOutPath returns ~/.cosca/out/tts_<timestamp>.wav (creating the dir).
func defaultTTSOutPath() string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".cosca", "out")
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, "tts_"+time.Now().Format("20060102-150405")+".wav")
}

func newVoiceLogger() zerolog.Logger { return zerolog.New(zerolog.Nop()) }
