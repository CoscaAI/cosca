package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"cosca/internal/dsms/intelligence"
	"cosca/internal/dsms/intelligence/codeanalyzer"
	"cosca/internal/dsms/oracle"
	"cosca/internal/dsms/storage"
)

// AskCmd adds the "ask" command — consult the Oracle.
func AskCmd() *cobra.Command {
	var (
		rulesDir string
		classify string
		file     string
	)

	cmd := &cobra.Command{
		Use:   "ask",
		Short: "Consulta o Oráculo (classifica, analisa, responde com evidência)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if rulesDir == "" {
				rulesDir = storage.DefaultDir()
			}

			o, err := oracle.NewOracle(rulesDir)
			if err != nil {
				return fmt.Errorf("init oracle: %w", err)
			}

			// Mode 1: classify an order
			if classify != "" {
				cls, err := o.Classify("cli", classify)
				if err != nil {
					return err
				}
				fmt.Printf("=== ORÁCULO — CLASSIFICAÇÃO ===\n")
				fmt.Printf("  Ordem: %s\n", classify)
				fmt.Printf("  Domínio: %s\n", cls.Domain)
				fmt.Printf("  Agente: %s\n", cls.Agent)
				fmt.Printf("  Confiança: %.0f%%\n", cls.Confidence*100)
				fmt.Printf("  Motivo: %s\n", cls.Reason)
				return nil
			}

			// Mode 2: analyze a file (pre-flight)
			if file != "" {
				code, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("read file: %w", err)
				}
				analyzer := codeanalyzer.NewAnalyzer()
				analysis, aErr := analyzer.AnalyzeGo(file, string(code))
				if aErr != nil {
					return fmt.Errorf("analyze: %w", aErr)
				}

				var patterns []string
				for _, p := range analysis.Patterns {
					patterns = append(patterns, p.Pattern)
				}

				ctx := &intelligence.Context{
					Language: analysis.Language,
					FilePath: analysis.FilePath,
					Code:     string(code),
					Metrics:  analysis.Metrics,
					Patterns: patterns,
				}

				ans, err := o.Ask("cli", "analyze:"+file, ctx)
				if err != nil {
					return err
				}

				fmt.Printf("=== ORÁCULO — ANÁLISE ===\n")
				fmt.Printf("  Arquivo: %s\n", file)
				fmt.Printf("  Linhas: %d\n", analysis.Lines)
				if ans.Unknown {
					fmt.Printf("  Resultado: nenhum problema conhecido detectado\n")
				} else {
					fmt.Printf("  Resultado: %d finding(s)\n", len(ans.Evidence))
					for _, f := range ans.Evidence {
						fmt.Printf("    [%s] %s: %s (conf %.0f%%)\n", f.Severity, f.RuleName, f.Message, f.Confidence*100)
					}
				}
				return nil
			}

			return fmt.Errorf("use --classify \"ordem\" ou --file \"caminho\"")
		},
	}

	cmd.Flags().StringVar(&rulesDir, "rules", "", "Diretório com regras JSONL")
	cmd.Flags().StringVar(&classify, "classify", "", "Classifica uma ordem/tarefa")
	cmd.Flags().StringVar(&file, "file", "", "Analisa um arquivo")

	return cmd
}
