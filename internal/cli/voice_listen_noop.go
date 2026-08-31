//go:build !stt_sherpa

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newVoiceListenCommand returns `cosca voice listen` that degrades gracefully in
// the default (non-cgo) build: the native-Go sherpa STT engine is not compiled
// in, so it reports "STT desabilitado" instead of crashing. The microphone
// capture backend is available on Windows, but without an ASR it has nothing to
// transcribe — so the live loop stays a no-op here.
func newVoiceListenCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "listen",
		Short: "Ouvir o microfone e transcrever ao vivo (builds sem -tags stt_sherpa reportam STT desabilitado)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(),
				"STT nativo desabilitado (build sem -tags stt_sherpa) — o COSCA ainda não transcreve ao vivo neste binário.\n")
			return nil
		},
	}
}
