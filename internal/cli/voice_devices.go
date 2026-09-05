package cli

// `cosca voice devices` lists the available input (capture) audio devices so the
// Don can pick a device index for perception.audio.mic.device. On non-Windows it
// reports that capture enumeration is unsupported (the mic backend is winmm).

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/worldmodel/audio/mic"
)

// newVoiceDevicesCommand returns `cosca voice devices`.
func newVoiceDevicesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "devices",
		Short: "Listar os dispositivos de captura de áudio disponíveis",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			devs, err := mic.ListDevices()
			if err != nil {
				// On non-Windows (or missing mic) this is ErrUnsupported; report
				// it cleanly instead of crashing.
				return fmt.Errorf("enumerar dispositivos: %w", err)
			}
			if len(devs) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Nenhum dispositivo de captura encontrado.")
				return nil
			}
			for _, d := range devs {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[%d] %s (máx %d canais)\n", d.Index, d.Name, d.MaxChannels)
			}
			return nil
		},
	}
}
