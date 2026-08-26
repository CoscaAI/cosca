package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/modlink"
)

// NewRoutesCommand creates the `cosca routes` command and its subcommands (the
// FASE 1 routing/scope inspection surface). It exposes the deterministic route
// registry (modlink.DefaultRoutes) so an operator can validate and review the
// modules the modular search router may select.
func NewRoutesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "routes",
		Short: "Inspect the modular route registry (modlink routing/scope)",
		Long: `Inspecta e valida o registry de rotas do roteador determinístico (modlink),
usado pelo modo modular de busca (ADR-013 §3.2). Cada rota mapeia um gatilho
(frase de agente) ao módulo que OWNS uma capability. Este comando NÃO altera
rotas: é leitura + validação.`,
		Example: `  cosca routes lint   # valida DefaultRoutes() e imprime o registry`,
		Args:    cobra.NoArgs,
		RunE:    func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	cmd.AddCommand(NewRoutesLintCommand())
	return cmd
}

// NewRoutesLintCommand creates `cosca routes lint`: valida o registry de rotas
// DefaultRoutes() (nenhum Module/Capability/Trigger vazio, Trigger único) e
// imprime o registry. Um registry inválido NUNCA é ignorado — retorna erro.
func NewRoutesLintCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "lint",
		Short: "Validate DefaultRoutes() and print the route registry",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			routes := modlink.DefaultRoutes()

			if err := modlink.ValidateRoutes(routes); err != nil {
				if useJSON {
					_ = printJSON(cmd, map[string]interface{}{
						"valid": false,
						"error": err.Error(),
					})
				} else {
					formatter.Warning(fmt.Sprintf("Route registry inválido: %v", err))
				}
				return fmt.Errorf("route registry inválido: %w", err)
			}

			if useJSON {
				return printJSON(cmd, map[string]interface{}{
					"valid":  true,
					"routes": routes,
				})
			}

			formatter.Header("Route Registry (DefaultRoutes)")
			formatter.KeyValue("Valid", "true")
			formatter.KeyValue("Count", fmt.Sprintf("%d", len(routes)))
			formatter.Println("")

			for _, rt := range routes {
				suffix := "priority=" + fmt.Sprintf("%d", rt.Priority)
				formatter.Printf("%-14s %-24s %-22s %s\n", rt.Module, rt.Capability, rt.Trigger, suffix)
			}

			formatter.Success("Route registry válido.")
			return nil
		},
	}
}
