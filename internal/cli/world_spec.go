package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/engineadapter"
	"github.com/CoscaAI/cosca/internal/worldloop"
	"github.com/CoscaAI/cosca/internal/worldspec"
)

// NewWorldBuildCommand materializa um WorldSpec no corpo (Rojo) — ADR-024.
func NewWorldBuildCommand() *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:   "build <spec.json>",
		Short: "Materializar um WorldSpec num projeto Rojo (ADR-024)",
		Long: `Consome um WorldSpec (contrato canonico) e materializa num projeto Rojo
que builds/valida (AuthorityMode=Server, DataStore anti-dupe, RemoteEvent validation).

Example:
  cosca world build spec.json --out rojo-out`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			spec, err := loadSpec(args[0])
			if err != nil {
				return err
			}
			adapter := engineadapter.NewRobloxAdapter(out)
			handle, err := adapter.Materialize(cmd.Context(), *spec)
			if err != nil {
				return err
			}
			formatter.Header(fmt.Sprintf("Materialized WorldSpec '%s' (type=%s, hash=%s)",
				spec.Name, spec.WorldType, handle.WorldHash))
			formatter.KeyValue("Corpo", handle.Target)
			formatter.KeyValue("Arquivos", fmt.Sprintf("%d", len(handle.Files)))
			for _, f := range handle.Files {
				formatter.KeyValue("  ", f)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&out, "out", "rojo-out", "diretório de saída do projeto Rojo")
	return cmd
}

// NewWorldLoopCommand roda o loop cognitivo de mundos (SIMULATE->OBSERVE->EVALUATE->MODIFY).
func NewWorldLoopCommand() *cobra.Command {
	var maxIter int
	cmd := &cobra.Command{
		Use:   "loop <spec.json>",
		Short: "Rodar o loop cognitivo de mundos sobre um WorldSpec (ADR-024)",
		Long: `Roda SIMULATE->OBSERVE->EVALUATE->MODIFY sobre um WorldSpec, medindo
acessibilidade e propondo correcao estrutural ate passar no gate (evalgo).

Example:
  cosca world loop spec.json --max-iter 3`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			spec, err := loadSpec(args[0])
			if err != nil {
				return err
			}
			rep, err := worldloop.Run(*spec, worldloop.AccessibilitySimulator{}, worldloop.ConnectIsolated{}, worldloop.DefaultGate(), maxIter)
			if err != nil {
				return err
			}
			formatter.Header(fmt.Sprintf("Loop convergiu em %d iteracao(es)", rep.Iterations))
			verdict := "NÃO CONVERGIU"
			if rep.Verdict.Promoted {
				verdict = "CONVERGIU"
			}
			formatter.KeyValue("Veredito", verdict)
			formatter.KeyValue("Resumo", rep.Verdict.Summary)
			if len(rep.Clusters) > 0 {
				rows := make([][]string, 0, len(rep.Clusters))
				for _, c := range rep.Clusters {
					rows = append(rows, []string{c.Signature, fmt.Sprintf("%d", c.Count)})
				}
				formatter.Table([]string{"Assinatura", "Casos"}, rows)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&maxIter, "max-iter", 5, "máximo de iterações do loop")
	return cmd
}

// loadSpec carrega e valida um WorldSpec a partir de um arquivo JSON (fail-closed I2).
func loadSpec(path string) (*worldspec.WorldSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read spec: %w", err)
	}
	var spec worldspec.WorldSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parse spec: %w", err)
	}
	if err := spec.Validate(); err != nil {
		return nil, fmt.Errorf("validate spec: %w", err)
	}
	return &spec, nil
}
