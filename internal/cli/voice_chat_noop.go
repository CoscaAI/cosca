//go:build !stt_sherpa

package cli

// This file compiles in the default (non-cgo) build. The native-Go sherpa STT
// (and its TTS partner) are not compiled in, so `cosca voice chat` degrades
// gracefully to a clear "desabilitado" message instead of crashing. The real
// loop lives in voice_chat_sherpa.go (built with -tags stt_sherpa).

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVoiceChatCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "chat",
		Short: "Diálogo ao vivo (builds sem a tag stt_sherpa reportam STT/TTS desabilitado)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(),
				"STT/TTS nativo desabilitado (build sem a tag stt_sherpa/tts_sherpa) — o COSCA ainda não tem o diálogo ao vivo (vê+ouve+fala) neste binário.\n")
			return nil
		},
	}
}
