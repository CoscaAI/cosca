//go:build stt_sherpa

package cli

// This file compiles only with `-tags stt_sherpa` (the native-Go sherpa STT
// engine). It implements `cosca voice chat` — the "teste absurdo do professor":
// the COSCA's first LOOP de voz, the proof of the full architecture, all native
// Go, WITHOUT manually attaching an image:
//
//	ouve (mic→STT streaming) → interpreta (query bus.State() for what vision is
//	seeing NOW) → responde (describe the detected entities) → fala (sherpa TTS)
//
// The recognition + vision are synchronised on a single Perception Bus
// (internal/perception/bus), so the "agora" the COSCA answers about is the same
// multimodal instant it heard the Don. Everything is local and sovereign: no
// Python, no Internet, no torch.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/perception/bus"
	"github.com/CoscaAI/cosca/internal/worldmodel/audio/mic"
	"github.com/CoscaAI/cosca/internal/worldmodel/audio/tts"
)

// newVoiceChatCommand returns `cosca voice chat` — the live dialogue loop. It is
// the stream combination of `voice listen` (mic→STT streaming), the Perception
// Bus (vision+audio sync) and `voice speak` (native TTS), wired into one loop
// that only runs (and speaks) when a real question is detected.
func newVoiceChatCommand() *cobra.Command {
	var device int
	var sid int
	var speed float64
	var play bool
	var quiet bool
	var outDir string

	cmd := &cobra.Command{
		Use:   "chat",
		Short: "Diálogo ao vivo: ouve (mic→STT) → interpreta (bus/visão) → fala (TTS)",
		Long: `O "teste absurdo do professor": o primeiro diálogo ao vivo do COSCA.

Abre o microfone e o STT streaming (sherpa-onnx), mantém um Perception Bus
sincronizando a visão (Perception Loop) com o que está sendo ouvido, e quando o
Don fala algo — o STT emite uma transcrição FINAL (= uma pergunta) — o COSCA
consulta o WorldState do bus: o que está vendo AGORA. Monta uma resposta em PT-BR
descrevendo as entidades detectadas (label + confiança + profundidade) e a FALA
via TTS (sherpa-onnx), gravando um .wav e o reproduzindo pelo player do sistema.

Fluxo (tudo em Go nativo, sem anexar imagem):
  mic → PCM → STT(sherpa).PushPCM → bus.AudioSample(final)
    → bus.State() (Vision.Entities + SummaryText) → respondFromWorldState
    → tts.Speaker.Synthesize → .wav → player padrão
  → volta a ouvir para o próximo turno. Ctrl-C para parar.

Degradação graciosa:
  * perception.enabled=false            → "A visão não está ativa agora."
  * visão ativa sem entidades            → "Não estou vendo objetos claros agora."
  * tts.provider != sherpa (ou build sem -tags tts_sherpa) → responde (texto) sem falar.

Pré-requisitos (como no cosca voice listen/speak): COSCA_ALLOW_NO_ROOT=1, as
DLLs (sherpa-onnx-c-api.dll / sherpa-onnx-cxx-api.dll / onnxruntime.dll) em bin/
(ou no PATH), e perception.audio.{stt,tts,mic} configurados (ver config.yaml).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := LoadConfigForVoice()
			if err != nil {
				return err
			}
			if cfg.Perception.Audio.STT.Provider != "sherpa" {
				return fmt.Errorf("STT nativo desabilitado: defina perception.audio.stt.provider=sherpa (e compile com -tags stt_sherpa)")
			}

			logger := newVoiceChatLogger()
			if !quiet {
				listDevices(cmd)
			}

			// 1) OUVIR: sherpa streaming STT source (opt-in via stt.provider).
			sttSrc, err := newSherpaSTTSource(cfg.Perception, logger)
			if err != nil {
				return err
			}
			if sttSrc == nil {
				return fmt.Errorf("STT desabilitado (sherpa não configurado)")
			}
			defer sttSrc.Close()

			// 2) OUVIR: live microphone → STT (mic → PCM → PushPCM → bus).
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

			micSrc := mic.NewMicrophoneAudioSource(mc, sttSrc, logger)
			defer micSrc.Close()

			// 3) VER: build the Perception Loop (opt-in via perception.enabled).
			// A nil service (perception disabled) means the bus has no vision →
			// the loop degrades gracefully to "A visão não está ativa agora".
			visionSvc := buildPerceptionService(cfg.Perception, logger)

			// 4) SINCRONIZAR: the Perception Bus ties vision (Perception Loop)
			//    + live audio (mic→STT) onto one monotonic clock, so the answer
			//    describes what the COSCA is seeing at the instant it heard you.
			b := bus.NewBus(bus.Config{
				Window:    cfg.Perception.Audio.Window,
				MaxObs:    bus.DefaultMaxObs,
				Tolerance: cfg.Perception.Audio.Tolerance,
			}, visionSvc, micSrc, logger.With().Str("component", "voice-chat/bus").Logger())

			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
			defer cancel()

			if visionSvc != nil && visionSvc.Enabled() {
				visionSvc.Start(ctx)
			}
			b.Start(ctx)
			defer b.Stop()

			// 5) FALAR: build the sherpa TTS speaker (opt-in via tts.provider).
			//    A nil speaker means TTS is disabled — the loop still answers
			//    (text) but does not speak.
			spk, err := buildTtsSpeaker(cfg.Perception, logger)
			if err != nil {
				return err
			}
			if spk != nil {
				defer spk.Close()
			}

			// 6) CÉREBRO: FASE 1 — connect the multimodal perception (what the
			//    COSCa sees/hears/remembers) to the MODEL (qwen3:8b via Ollama)
			//    for the answer, instead of the template. Graceful: a missing
			//    provider just disables deliberation (template fallback).
			brain := buildVoiceBrain(ctx, cfg, logger.With().Str("component", "voice-chat/brain").Logger())
			SetVoiceBrain(brain)
			defer brain.Shutdown()

			// 7) MEMÓRIA: build the episodic memory manager so the loop can
			//    recall what it saw/heard before (FASE D) and feed those hints
			//    into the deliberation. Degrades to nil (no recall) when absent.
			coscaDir := ".cosca"
			if cwd, wErr := os.Getwd(); wErr == nil {
				coscaDir = filepath.Join(cwd, ".cosca")
			}
			mem := NewMemoryManager(coscaDir)
			if mem != nil {
				defer mem.Close()
			}

			fmt.Fprintln(cmd.OutOrStdout(), "COSCA está ouvindo e vendo (Ctrl-C para parar)…")
			if visionSvc == nil || !visionSvc.Enabled() {
				fmt.Fprintln(cmd.OutOrStdout(), "aviso: perception.enabled=false — a visão NÃO está ativa; o COSCA responderá \"A visão não está ativa agora\".")
			}
			if spk == nil {
				fmt.Fprintln(cmd.OutOrStdout(), "aviso: TTS nativo não compilado/percepção.tts.provider != sherpa — o COSCA escreverá a resposta mas NÃO a falará.")
			}

			return runVoiceChatLoop(cmd, b, spk, mem, sid, speed, play, outDir, logger, ctx)
		},
	}

	cmd.Flags().IntVar(&device, "device", -1, "índice do dispositivo de captura (default -1 = dispositivo padrão do sistema)")
	cmd.Flags().IntVar(&sid, "sid", 0, "id da voz TTS (default 0)")
	cmd.Flags().Float64Var(&speed, "speed", 1.0, "velocidade da fala (1.0 = normal; <1 mais rápido, >1 mais lento)")
	cmd.Flags().BoolVar(&play, "play", true, "reproduzir o .wav via player padrão do sistema (default true)")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "não listar dispositivos de entrada")
	cmd.Flags().StringVar(&outDir, "out-dir", "", "diretório dos .wav gerados (default ~/.cosca/out)")
	return cmd
}

// runVoiceChatLoop is the live loop: it subscribes to the Perception Bus,
// reacts to each FINAL transcription (a question), interprets the current
// WorldState (what vision is seeing NOW), and speaks the response. A periodic
// ticker surfaces the "vejo agora" status so the Don can see what the COSCA is
// perceiving even before it speaks.
func runVoiceChatLoop(cmd *cobra.Command, b *bus.Bus, spk *tts.Speaker, mem *memoryManagerAdapter,
	sid int, speed float64, play bool, outDir string,
	logger zerolog.Logger, ctx context.Context) error {

	sub := b.Watch()
	defer b.Unsubscribe(sub)

	// Heartbeat: print the current vision status so the "vê" is visible.
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()

	// lastFinalSeg prevents answering the same finalised segment more than once
	// (a subsequent vision-only WorldState re-carries the same last audio).
	// Segment IDs start at 0, so -1 is a sentinel meaning "none answered yet".
	lastFinalSeg := int64(-1)

	for {
		select {
		case <-ctx.Done():
			fmt.Fprintln(cmd.OutOrStdout(), "\nCOSCA parou.")
			return nil

		case <-tick.C:
			printVoiceChatVisionStatus(cmd, b)

		case ws, ok := <-sub:
			if !ok {
				return nil
			}
			if ws == nil || ws.Audio == nil {
				continue
			}
			a := ws.Audio
			if !a.IsFinal || a.Text == "" {
				continue
			}
			if int64(a.SegmentID) == lastFinalSeg {
				continue
			}
			lastFinalSeg = int64(a.SegmentID)

			utterance := strings.TrimSpace(a.Text)
			fmt.Fprintf(cmd.OutOrStdout(), "\r[ouvi] %s\n", utterance)

			// INTERPRETAR (FASE 1 — o cérebro usa os sentidos): monta o contexto
			// perceptual (visão + áudio + memória episódica) + a pergunta do Don e
			// deixa o MODELO deliberar a resposta inteligente em PT-BR. Degradação
			// graciosa: se o modelo falhar/for ausente, cai no template antigo.
			nowState := b.State()
			memoryHints := buildMemoryHints(ctx, mem, utterance, 4)
			resp, delibErr := respondWithDeliberation(ctx, nowState, utterance, memoryHints)
			if delibErr != nil {
				logger.Warn().Err(delibErr).Msg("voice chat: deliberation degraded — spoke template")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "[COSCA] %s\n", resp)

			// FALAR: synthesize + write a .wav + (optionally) play it.
			if spk == nil {
				fmt.Fprintln(cmd.OutOrStdout(), "(TTS desabilitado — escrevi a resposta acima; para eu falar configure perception.audio.tts.provider=sherpa no binário.)")
				continue
			}
			if err := speakVoiceChat(cmd, spk, resp, sid, speed, play, outDir, logger); err != nil {
				logger.Warn().Err(err).Msg("voice chat: speak failed")
				fmt.Fprintf(cmd.OutOrStdout(), "(fala falhou: %v)\n", err)
			}
		}
	}
}

// printVoiceChatVisionStatus surfaces the live vision status on the heartbeat.
func printVoiceChatVisionStatus(cmd *cobra.Command, b *bus.Bus) {
	st := b.State()
	if st == nil || st.Vision == nil {
		fmt.Fprintln(cmd.OutOrStdout(), "[status] visão: inativa (bus sem WorldState de visão)")
		return
	}
	entities := st.Vision.Entities
	if len(entities) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "[status] visão: ativa — nenhum objeto claro agora")
		return
	}
	labels := make([]string, 0, len(entities))
	for _, e := range entities {
		label := strings.TrimSpace(e.Label)
		if label == "" {
			label = "objeto"
		}
		labels = append(labels, fmt.Sprintf("%s (%.0f%%, %.1fm)", label, e.Confidence*100, e.Depth))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "[status] visão: vejo %d objeto(s): %s\n", len(entities), strings.Join(labels, ", "))
}

// speakVoiceChat synthesizes the response to a .wav directory (or the default
// ~/.cosca/out), prints it, and (when --play) reproduces it via the system
// player. Synthesis + playback are synchronous so the loop waits for the COSCA
// to finish speaking before listening again.
func speakVoiceChat(cmd *cobra.Command, spk *tts.Speaker, text string,
	sid int, speed float64, play bool, outDir string, logger zerolog.Logger) error {

	out := voiceChatOutPath(outDir)
	res, err := spk.SpeakToFile(text, out, sid, speed)
	if err != nil {
		return fmt.Errorf("síntese: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "[falei] %d samples @ %d Hz (%.1fs) → %s\n",
		len(res.Samples), res.SampleRate, res.Duration.Seconds(), out)

	if play {
		if err := playVoiceChatWAV(out); err != nil {
			logger.Warn().Err(err).Msg("voice chat: player unavailable — playback skipped")
			fmt.Fprintf(cmd.OutOrStdout(), "(player indisponível para %s: %v)\n", out, err)
		}
	}
	return nil
}

// voiceChatOutPath returns a .wav path under outDir (or ~/.cosca/out), making
// the directory if needed.
func voiceChatOutPath(outDir string) string {
	if outDir == "" {
		home, _ := os.UserHomeDir()
		outDir = filepath.Join(home, ".cosca", "out")
	}
	_ = os.MkdirAll(outDir, 0o755)
	return filepath.Join(outDir, "tts_"+time.Now().Format("20060102-150405")+".wav")
}

// playVoiceChatWAV reproduces a .wav via the Windows built-in
// System.Media.SoundPlayer (through PowerShell). It plays SYNCHRONOUSLY (blocks
// until the audio ends), so the loop naturally pauses while the COSCA speaks.
// A non-Windows / audio-less host degrades gracefully (returns an error).
func playVoiceChatWAV(path string) error {
	safePath := strings.ReplaceAll(path, "'", "''")
	script := fmt.Sprintf("$p=New-Object System.Media.SoundPlayer '%s'; $p.PlaySync()", safePath)
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("player: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func newVoiceChatLogger() zerolog.Logger { return zerolog.New(zerolog.Nop()) }
