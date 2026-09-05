//go:build !tts_sherpa

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newVoiceSpeakCommand returns a `cosca voice speak` subcommand that degrades
// gracefully in the default (non-cgo) build: the native-Go sherpa TTS engine is
// not compiled in, so it reports "TTS desabilitado" instead of crashing.
func newVoiceSpeakCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "speak \"<texto>\"",
		Short: "Falar um texto (TTS) — builds sem a tag tts_sherpa reportam TTS desabilitado",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(),
				"TTS nativo desabilitado (build sem -tags tts_sherpa) — o COSCA ainda não fala neste binário.\n")
			return nil
		},
	}
}
