package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/datasetgen"
)

// NewDatasetCommand expõe a FÁBRICA DE DADOS / TREINADOR de modelos via CLI.
// Subcomandos:
//   - cosca dataset generate [n]  — gera um dataset de treinamento (JSONL)
//   - cosca dataset golden        — roda o golden gate (métrica de promoção)
func NewDatasetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dataset",
		Short: "Fábrica de dados de treinamento (treinador de modelos)",
		Long: `Cosca dataset — a fábrica de dados de treinamento do COSCA.

Gera datasets (trajetórias de tool-call) e roda o GOLDEN GATE (régua de
promoção anti-autoengano). Este é o primeiro componente do treinador de
modelos: cada geração de modelo precisa PROVAR, via golden gate, que ficou
melhor antes de ganhar o crachá.

Subcomandos:
  generate   gerar dataset de treinamento (JSONL)
  golden     rodar o golden set e reportar a métrica de promoção`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newDatasetGenerateCommand())
	cmd.AddCommand(newDatasetGoldenCommand())
	cmd.AddCommand(newDatasetConvertCommand())
	return cmd
}

// newDatasetGenerateCommand gera um dataset de treinamento via gerador
// procedural (determinístico) + (opcionalmente) executa os exemplos pelo
// modelo para capturar trajetórias reais.
func newDatasetGenerateCommand() *cobra.Command {
	var n int
	var model string
	var out string
	var runEval bool

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Gerar dataset de treinamento (JSONL)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			cfg := datasetgen.DefaultGeneratorConfig()
			if model != "" {
				cfg.Model = model
			}

			// Gera os specs procedurais.
			specs := datasetgen.GenerateBatch(n, time.Now().UnixNano())
			formatter.Printf("gerando %d tarefas (%s)\n", n, datasetgen.SummarizeBatch(specs))

			if !runEval {
				return fmt.Errorf("defina --eval (requer Ollama rodando) para executar as trajetórias")
			}

			// Executa cada spec e agrega os exemplos.
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(n)*cfg.Timeout)
			defer cancel()
			r := datasetgen.NewRunner(cfg)
			examples, _ := r.GenerateBatch(ctx, specs)

			// Escreve o dataset JSONL.
			f, err := os.Create(out)
			if err != nil {
				return fmt.Errorf("dataset: criar output: %w", err)
			}
			defer f.Close()
			var pos, neg int
			for _, ex := range examples {
				if _, err := f.Write(ex.MarshalJSONL()); err != nil {
					return fmt.Errorf("dataset: write: %w", err)
				}
				if ex.Label.IsPositive() {
					pos++
				} else {
					neg++
				}
			}
			formatter.Success(fmt.Sprintf("dataset escrito: %s (pos=%d, contraste=%d)", out, pos, neg))
			return nil
		},
	}
	cmd.Flags().IntVar(&n, "n", 10, "número de tarefas")
	cmd.Flags().StringVar(&model, "model", "qwen3:4b", "modelo aluno (executa as trajetórias)")
	cmd.Flags().StringVar(&out, "out", "", "arquivo de saída JSONL (default: <cwd>/dataset.jsonl)")
	cmd.Flags().BoolVar(&runEval, "eval", false, "executar trajetórias com o modelo (requer Ollama)")
	return cmd
}

// newDatasetConvertCommand converte o dataset (datasetgen) para o formato SFT
// ChatML (fine-tune) e divide em train/val.
func newDatasetConvertCommand() *cobra.Command {
	var input, output string
	var ratio float64

	cmd := &cobra.Command{
		Use:   "convert",
		Short: "Converter dataset para formato SFT (fine-tune ChatML)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			pos, err := datasetgen.ConvertDatasetToSFT(input, output)
			if err != nil {
				return fmt.Errorf("dataset convert: %w", err)
			}
			trainPath, valPath, err := datasetgen.WriteSFTBatches(output, ratio)
			if err != nil {
				return fmt.Errorf("dataset split: %w", err)
			}
			formatter.Success(fmt.Sprintf("convertido %d exemplos positivos → %s | train=%s val=%s", pos, output, trainPath, valPath))
			return nil
		},
	}
	cmd.Flags().StringVar(&input, "input", "", "dataset JSONL de entrada (datasetgen)")
	cmd.Flags().StringVar(&output, "output", "", "arquivo SFT JSONL de saída")
	cmd.Flags().Float64Var(&ratio, "ratio", 0.1, "proporção de validação (0-1)")
	return cmd
}

// newDatasetGoldenCommand roda o golden set congelado e reporta a métrica de
// promoção (baseline). Para comparar antes/depois, rode isto ANTES e DEPOIS
// do LoRA.
func newDatasetGoldenCommand() *cobra.Command {
	var model string

	cmd := &cobra.Command{
		Use:   "golden",
		Short: "Rodar golden set (métrica de promoção)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			cfg := datasetgen.DefaultGeneratorConfig()
			if model != "" {
				cfg.Model = model
			}
			// Loga qual modelo esta' sendo usado para a campanha - evita rodar
			// o AFTER com o modelo errado (o "0.88 == 0.88" do base).
			formatter.Printf("golden gate modelo=%s\n", cfg.Model)

			gs, err := datasetgen.LoadGoldenSet(datasetgen.DefaultGoldenSetPath())
			if err != nil {
				return fmt.Errorf("dataset golden: %w", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(len(gs.Cases))*cfg.Timeout)
			defer cancel()
			r := datasetgen.NewRunner(cfg)
			report, err := datasetgen.EvaluateGolden(ctx, r, gs)
			if err != nil {
				return fmt.Errorf("dataset golden: %w", err)
			}

			formatter.Printf("golden gate (n=%d):\n", report.N)
			formatter.Printf("  pass%%       = %.2f (%d/%d)\n", report.AggregatePassRate, report.Passed, report.N)
			formatter.Printf("  recovery    = %.2f\n", report.AggregateRecoveryRate)
			formatter.Printf("  tool_valid  = %.2f\n", report.AggregateToolValidity)
			formatter.Printf("  read_edit   = %.2f\n", report.AggregateReadEdit)
			formatter.Printf("  test_evid   = %.2f\n", report.AggregateTestEvidence)
			formatter.Printf("  CRÍTICAS    = unsafe=%d false_completion=%d\n", report.UnsafeMutationCount, report.FalseCompletionCount)
			for _, res := range report.Results {
				mark := "✅"
				if !res.Passed {
					mark = "❌"
				}
				formatter.Printf("  %s %-22s label=%-18s passed=%v\n", mark, res.CaseID, res.Label, res.Passed)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&model, "model", "qwen3:4b", "modelo aluno (executa o golden)")
	return cmd
}
