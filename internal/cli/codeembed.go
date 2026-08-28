package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/codeembed"
)

// NewCodeEmbedCommand cria a árvore `cosca codeembed` — embedding de código
// determinístico (F1, ADR-019). ZERO LLM, ZERO rede (I1).
func NewCodeEmbedCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "codeembed",
		Short: "Embedding de código determinístico (int8/float), sem LLM (ADR-019)",
		Long: `Embedding de código determinístico (signed feature-hashing). Determinístico
(I1), zero LLM, zero rede — alinhado ao ADR-019 (busca semântica de código sem
modelo).

Subcomandos:
  embed  Produz o vetor de um trecho de código
  sim    Compara a similaridade de dois trechos de código`,
		Example: `  cosca codeembed embed --dim 512 'func main() { fmt.Println("hi") }' --json
  cosca codeembed sim 'parseConfig()' 'parse_config()'`,
	}
	cmd.AddCommand(NewCodeEmbedEmbedCommand(), NewCodeEmbedSimCommand())
	return cmd
}

// NewCodeEmbedEmbedCommand cria `cosca codeembed embed <code>`.
func NewCodeEmbedEmbedCommand() *cobra.Command {
	var dim int
	cmd := &cobra.Command{
		Use:   "embed <code>",
		Short: "Produz o vetor de embedding de um trecho de código",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)
			vec := codeembed.Embed(args[0], dim)
			report := map[string]interface{}{
				"dimension": len(vec),
				"tokens":    codeembed.Tokenize(args[0]),
				"quantized": codeembed.QuantizeInt8(vec),
			}
			if useJSON {
				return printJSON(cmd, report)
			}
			formatter.Header("Embedding determinístico (código)")
			formatter.KeyValue("Dimensão", fmt.Sprintf("%d", len(vec)))
			formatter.KeyValue("Tokens", fmt.Sprintf("%d", len(report["tokens"].([]string))))
			formatter.KeyValue("Vetor (int8)", fmt.Sprintf("%v", report["quantized"].([]int8)))
			return nil
		},
	}
	cmd.Flags().IntVar(&dim, "dim", codeembed.DefaultDim, "dimensão do vetor")
	return cmd
}

// NewCodeEmbedSimCommand cria `cosca codeembed sim <a> <b>`.
func NewCodeEmbedSimCommand() *cobra.Command {
	var dim int
	cmd := &cobra.Command{
		Use:   "sim <a> <b>",
		Short: "Similaridade de cosseno entre dois trechos de código",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)
			sim := codeembed.Similarity(codeembed.Embed(args[0], dim), codeembed.Embed(args[1], dim))
			if useJSON {
				return printJSON(cmd, map[string]interface{}{"similarity": sim, "dim": dim})
			}
			formatter.Header("Similaridade de código (determinístico)")
			formatter.KeyValue("Dimensão", fmt.Sprintf("%d", dim))
			formatter.KeyValue("Similaridade", fmt.Sprintf("%.4f", sim))
			return nil
		},
	}
	cmd.Flags().IntVar(&dim, "dim", codeembed.DefaultDim, "dimensão do vetor")
	return cmd
}
