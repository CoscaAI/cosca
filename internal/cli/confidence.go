// Package cli — `cosca confidence`: expõe o Agent Confidence Tracker.
//
// ANTES: o Tracker (internal/confidence) era lido apenas pelo rest-cycle (ORC)
// que o recriava do zero a cada execução — os perfis Wave 2 hardcoded nunca
// eram atualizados no fluxo vivo. Este comando:
//
//	cosca confidence            # lista perfis (leitura)
//	cosca confidence summary    # média + resumo por domínio
//	cosca confidence set <agent> <0..1> [domains...]  # registra no fluxo vivo
//
// O Tracker é a fonte de verdade de confiabilidade dos agentes (calibração
// Wave 2 + evolução por uso).
package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/confidence"
)

// NewConfidenceCommand cria `cosca confidence`.
func NewConfidenceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "confidence",
		Short: "Agent Confidence Tracker — perfis de confiabilidade dos agentes",
		Long: `Agent Confidence Tracker: fonte de verdade da confiabilidade dos
agentes (calibração Wave 2 + evolução por uso).

Subcomandos:
  list        Lista todos os perfis de confiança por agente/domínio.
  summary     Média geral + resumo por domínio.
  set         Registra/atualiza a confiança de um agente (fluxo vivo).`,
		Example: `  cosca confidence
  cosca confidence summary
  cosca confidence set cosca-qa 0.65 quality_assurance`,
	}
	cmd.AddCommand(newConfidenceListCommand())
	cmd.AddCommand(newConfidenceSummaryCommand())
	cmd.AddCommand(newConfidenceSetCommand())
	return cmd
}

// newTracker cria o Tracker com os perfis (Wave 2 + registros vivos).
func newTracker() *confidence.Tracker {
	return confidence.NewTracker(zerolog.Nop())
}

func newConfidenceListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Lista os perfis de confiança",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)
			t := newTracker()

			type row struct {
				Agent    string   `json:"agent"`
				Domain   string   `json:"domain"`
				Score    float64  `json:"score"`
				Domains  []string `json:"domains"`
			}
			var rows []row
			for _, name := range t.ListAgents() {
				domains, _ := t.GetAgentDomains(name)
				domList := make([]string, 0, len(domains))
				for _, d := range domains {
					domList = append(domList, string(d))
				}
				rows = append(rows, row{
					Agent:   name,
					Domains: domList,
				})
			}

			if useJSON {
				b, _ := jsonMarshalImpl(map[string]any{"agents": rows})
				f.Println(string(b))
				return nil
			}

			f.Header("Agent Confidence Tracker")
			if len(rows) == 0 {
				f.Print("Nenhum perfil registrado.")
				return nil
			}
			for _, r := range rows {
				avg := 0.0
				for _, d := range r.Domains {
					if s, err := t.GetConfidence(r.Agent, confidence.Domain(d)); err == nil {
						avg += s
					}
				}
				if len(r.Domains) > 0 {
					avg /= float64(len(r.Domains))
				}
				f.Printf("  %-24s confiança=%.2f domínios=%s\n", r.Agent, avg, strings.Join(r.Domains, ","))
			}
			return nil
		},
	}
}

func newConfidenceSummaryCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "summary",
		Short: "Média geral + resumo por domínio",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f := GetFormatter(cmd)
			t := newTracker()

			f.Header("Confidence — resumo")
			f.KeyValue("Agentes", fmt.Sprintf("%d", len(t.ListAgents())))
			f.KeyValue("Confiança média", fmt.Sprintf("%.3f", t.GetAverageConfidence()))

			f.Header("Por domínio")
			for _, d := range t.ListDomains() {
				if s, err := t.GetDomainSummary(d); err == nil {
					f.Printf("  %-24s %.3f (%d agentes)\n", s.Domain, s.AverageConfidence, s.AgentCount)
				}
			}
			return nil
		},
	}
}

func newConfidenceSetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set <agent> <0..1> [domains...]",
		Short: "Registra/atualiza a confiança de um agente (fluxo vivo)",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			f := GetFormatter(cmd)
			agent := args[0]
			score, err := strconv.ParseFloat(args[1], 64)
			if err != nil {
				return fmt.Errorf("confiança inválida %q (use 0..1): %w", args[1], err)
			}
			if score < 0 || score > 1 {
				return fmt.Errorf("confiança fora do intervalo 0..1: %f", score)
			}

			var domains []confidence.Domain
			for _, d := range args[2:] {
				domains = append(domains, confidence.Domain(d))
			}
			if len(domains) == 0 {
				domains = []confidence.Domain{confidence.DomainOperations}
			}

			t := newTracker()
			t.Register(agent, score, domains)

			f.Print(fmt.Sprintf("✅ %s → confiança %.2f (domínios: %v)", agent, score, domains))
			return nil
		},
	}
}
