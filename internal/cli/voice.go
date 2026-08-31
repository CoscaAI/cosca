package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/config"
)

// voiceProjectRoot is the cosca-voice project (desacoplado do root — L210).
// Overridable via COSCA_VOICE_ROOT.
func voiceProjectRoot() string {
	if p := os.Getenv("COSCA_VOICE_ROOT"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Documents", "projects", "cosca-voice")
}

// voicePython resolves the venv python of the cosca-voice project.
func voicePython(root string) string {
	return filepath.Join(root, "venv", "bin", "python")
}

// voiceServiceName is the systemd user unit for the voice assistant.
const voiceServiceName = "cosca-voice.service"

// NewVoiceCommand creates the `cosca voice` command group: start/stop/status
// of the independent cosca-voice assistant project (wake word "cosca"). The
// voice is deliberately NOT always-on — it is a heavy pipeline (torch +
// faster-whisper + kokoro) that must only run when the Don wants to talk.
func NewVoiceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "voice",
		Short: "Assistente de voz cosca (projeto independente)",
		Long: `Gerencia o assistente de voz cosca (wake word "cosca"), projeto independente
em ~/Documents/projects/cosca-voice.

O voice é um pipeline pesado (torch + faster-whisper + kokoro) e NÃO roda
sempre: ele é desabilitado no boot e liga só quando você quiser falar.

  cosca voice start    # inicia o assistente (wake word ativo)
  cosca voice stop     # para o assistente (libera CPU/RAM)
  cosca voice status   # mostra o estado atual`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "start",
			Short: "Iniciar o assistente de voz",
			RunE: func(cmd *cobra.Command, _ []string) error {
				return voiceControl(cmd, "start")
			},
		},
		&cobra.Command{
			Use:   "stop",
			Short: "Parar o assistente de voz",
			RunE: func(cmd *cobra.Command, _ []string) error {
				return voiceControl(cmd, "stop")
			},
		},
		&cobra.Command{
			Use:   "status",
			Short: "Estado do assistente de voz",
			RunE: func(cmd *cobra.Command, _ []string) error {
				return voiceControl(cmd, "status")
			},
		},
		newVoiceSpeakCommand(),
		// FASE D: real-time microphone capture → live STT transcription. This is
		// the native-Go, sovereign "COSCA ouve em tempo real" loop. It is built
		// only behind `-tags stt_sherpa` (see voice_listen_sherpa.go); without it
		// the noop twin (voice_listen_noop.go) reports STT desabilitado.
		newVoiceListenCommand(),
		newVoiceDevicesCommand(),
	)
	return cmd
}

// LoadConfigForVoice loads the COSCA config (for the native TTS voice speak
// path). It reuses the CLI's config loader so the resolver honours the same
// project/home resolution as the serve path.
func LoadConfigForVoice() (*config.Config, error) {
	return loadConfig()
}

// voiceControl runs systemctl --user on the voice unit, with a clear error
// when the project is missing.
func voiceControl(cmd *cobra.Command, action string) error {
	root := voiceProjectRoot()
	if _, err := os.Stat(root); err != nil {
		return fmt.Errorf("projeto cosca-voice nao encontrado em %s (defina COSCA_VOICE_ROOT): %w", root, err)
	}

	out, err := exec.Command("systemctl", "--user", action, voiceServiceName).CombinedOutput()
	if err != nil && !strings.Contains(string(out), "inactive") {
		return fmt.Errorf("systemctl %s %s: %w (%s)", action, voiceServiceName, err, strings.TrimSpace(string(out)))
	}

	switch action {
	case "start":
		fmt.Fprintf(cmd.OutOrStdout(), "✓ assistente de voz iniciado (wake word \"cosca\") — parar com: cosca voice stop\n")
	case "stop":
		fmt.Fprintf(cmd.OutOrStdout(), "✓ assistente de voz parado — CPU/RAM liberadas\n")
	case "status":
		fmt.Fprintf(cmd.OutOrStdout(), "estado do voice: %s\n", strings.TrimSpace(string(out)))
	}
	return nil
}
