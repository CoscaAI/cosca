package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/dflow"
)

// NewFlowCommand creates the `cosca flow` command — Durable Workflow demo
// (§40 do manifesto Creative/Scientific/Media), Fase 1 etapa 1.9.
//
// O comando executa o pipeline de exemplo do §21 (extract_audio → whisper →
// subtitle) como um workflow DURÁVEL: activities com retry, histórico
// persistido, e replay determinístico (retomada não re-executa effects).
func NewFlowCommand() *cobra.Command {
	var historyPath string
	var simulateFailure uint

	cmd := &cobra.Command{
		Use:   "flow",
		Short: "Durable Workflow demo — replay + retry (§40)",
		Long: `Durable Workflow demo (§40) — pipeline de exemplo do §21
(extract_audio → whisper → subtitle) executado como workflow DURÁVEL.

Padrão Temporal:
  - Activities = efeitos colaterais com RETRY (backoff exponencial)
  - Histórico = estado append-only persistido em disco
  - Replay = retomada determinística: effects já feitos NÃO re-executam

Flags:
  --history <path>     Persist/load o histórico (resumível §40)
  --fail <n>           Simula falha nas primeiras n tentativas (testa retry)
  --retry              Re-executa com o histórico existente (replay)`,
		Example: `  cosca flow
  cosca flow --history .cosca/history.json
  cosca flow --history .cosca/history.json --fail 2
  cosca flow --history .cosca/history.json --retry`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			// Carrega histórico pré-existente (retomada §40).
			var hist *dflow.History
			if historyPath != "" {
				if data, err := os.ReadFile(historyPath); err == nil {
					var h dflow.History
					if err := json.Unmarshal(data, &h); err != nil {
						return fmt.Errorf("parse history %s: %w", historyPath, err)
					}
					hist = &h
					if hist.ActivityOutcomes == nil {
						hist.ActivityOutcomes = map[dflow.ActivityKey]dflow.ActivityOutcome{}
					}
				}
			}

			// Activities do pipeline §21.
			var whisperFails int
			remainingFails := int(simulateFailure)
			extract := dflow.ActivityFunc{ID: "extract_audio", Fn: func(_ context.Context) (any, error) {
				return "audio.wav (extraído)", nil
			}}
			whisper := dflow.ActivityFunc{ID: "whisper", Fn: func(ctx context.Context) (any, error) {
				if remainingFails > 0 {
					remainingFails--
					whisperFails++
					return nil, fmt.Errorf("whisper falhou (tentativa %d de %d)", whisperFails, simulateFailure)
				}
				return "legendas geradas pelo whisper", nil
			}}
			subtitle := dflow.ActivityFunc{ID: "subtitle", Fn: func(_ context.Context) (any, error) {
				return "final.srt (legendado)", nil
			}}
			acts := []dflow.Activity{extract, whisper, subtitle}

			wf := func(ctx context.Context, wc *dflow.WorkflowCtx) (any, error) {
				if _, err := wc.ExecActivity(ctx, extract); err != nil {
					return nil, err
				}
				text, err := wc.ExecActivity(ctx, whisper)
				if err != nil {
					return nil, err
				}
				if _, err := wc.ExecActivity(ctx, subtitle); err != nil {
					return nil, err
				}
				return text, nil
			}

			runner := dflow.NewRunner()
			res, err := runner.Run(context.Background(), wf, acts, nil, &dflow.RunOptions{History: hist})
			if err != nil {
				return err
			}

			// Persiste o histórico (resumível §40).
			if historyPath != "" {
				data, err := json.MarshalIndent(res.History, "", "  ")
				if err != nil {
					return err
				}
				if err := os.WriteFile(historyPath, data, 0o644); err != nil {
					return err
				}
			}

			if useJSON {
				return printJSON(cmd, map[string]any{
					"output":   res.Output,
					"executed": res.Executed,
					"replayed": res.Replayed,
					"history":  historyPath,
				})
			}

			formatter.Header("Durable Workflow — pipeline §21")
			formatter.KeyValue("Output", fmt.Sprint(res.Output))
			formatter.KeyValue("Executed (nesta rodada)", fmt.Sprint(res.Executed))
			formatter.KeyValue("Replayed (do histórico)", fmt.Sprint(res.Replayed))
			if whisperFails > 0 {
				formatter.KeyValue("Retries (whisper)", fmt.Sprint(whisperFails))
			}
			if historyPath != "" {
				formatter.KeyValue("History", historyPath)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&historyPath, "history", "", "Persist/load workflow history (resumible §40)")
	cmd.Flags().UintVar(&simulateFailure, "fail", 0, "Simulate failure on the first N whisper attempts (tests retry)")
	return cmd
}
