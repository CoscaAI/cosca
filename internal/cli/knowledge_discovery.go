//
// `cosca knowledge discovery` — o Hall da Fama do Conhecimento via CLI.
//
// Subcomandos:
//   list          — tabela de todas as descobertas (ID | Nome | Impacto ★ |
//                   Projetos | Risco ↓ | Relacionado); na primeira execução
//                   semeia as 5 descobertas reais (seed) quando
//                   hall-of-fame.json não existe
//   show <id>     — descoberta completa (descrição, impacto, redução de
//                   risco, sessões validadas, economia, origem, lei ligada)
//   add           — registrar nova descoberta (flags: --name --impact
//                   --risk-reduction --origin --law [+ --desc --projects
//                   --sessions --economy-percent --economy-usd])
//
// O registro vive em internal/knowledge (HallOfFame) com persistência JSON.
// O CLI lê/escreve em .cosca/knowledge/hall-of-fame.json (runtime,
// gitignored). Load no início de cada comando, Save após mutação.
// NUNCA toca em .cosca/framework.
//

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/knowledge"
)

// hallOfFameFileName é o arquivo runtime do Hall da Fama (gitignored).
const hallOfFameFileName = "hall-of-fame.json"

// resolveHallOfFamePath devolve o caminho do arquivo runtime do Hall da
// Fama para o diretório de trabalho atual (o Don roda o CLI no projeto):
// <projeto>/.cosca/knowledge/hall-of-fame.json.
func resolveHallOfFamePath() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	return filepath.Join(dir, ".cosca", "knowledge", hallOfFameFileName), nil
}

// loadHallOfFame carrega o HallOfFame do arquivo runtime. Um arquivo
// inexistente não é erro: resulta em Hall da Fama vazio (nenhuma
// descoberta).
func loadHallOfFame(path string) (*knowledge.HallOfFame, error) {
	h := knowledge.NewHallOfFame()
	if err := h.Load(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return h, nil
		}
		return nil, err
	}
	return h, nil
}

// nextDiscoveryID devolve o próximo ID livre no formato D-XX: o maior
// sufixo numérico existente + 1 (D-01..D-05 no seed → D-06, D-07, ...).
func nextDiscoveryID(h *knowledge.HallOfFame) string {
	max := 0
	for _, d := range h.All() {
		var n int
		if _, err := fmt.Sscanf(d.ID, "D-%d", &n); err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("D-%02d", max+1)
}

// NewKnowledgeDiscoveryCommand cria o comando `cosca knowledge discovery`.
func NewKnowledgeDiscoveryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "discovery",
		Short: "Hall da Fama do conhecimento — as maiores descobertas do runtime",
		Long: `Registra as maiores descobertas do runtime Cosca, com impacto medido.

Cada descoberta responde "o que isso mudou?": impacto em estrelas (1-5),
projetos afetados, redução de risco, sessões validadas e economia. A
origem é rastreável (auditoria, red team, conversa do Don) e pode ligar-se
a uma lei do CKL via RelatedLaw (ex: D-01 → K-01).

Impacto em estrelas é uma escala de valor (1-5) — NÃO é a confiança do
CKL (0-1). Uma descoberta é o irmão operacional da lei.

Persistência: .cosca/knowledge/hall-of-fame.json (runtime, gitignored).
`,
		Example: `  cosca knowledge discovery list
  cosca knowledge discovery show D-01
  cosca knowledge discovery add --name "Descoberta X" --impact 4 --risk-reduction 55 --origin "auditoria #52" --law K-01`,
	}

	cmd.AddCommand(
		NewKnowledgeDiscoveryListCommand(),
		NewKnowledgeDiscoveryShowCommand(),
		NewKnowledgeDiscoveryAddCommand(),
	)
	return cmd
}

// NewKnowledgeDiscoveryListCommand cria o comando
// `cosca knowledge discovery list`.
func NewKnowledgeDiscoveryListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Lista as descobertas do Hall da Fama (impacto desc)",
		Long: `Lista todas as descobertas do Hall da Fama, ordenadas por impacto
(estrelas ★) decrescente — a maior descoberta primeiro.

Na primeira execução (quando .cosca/knowledge/hall-of-fame.json ainda não
existe), semeia as 5 descobertas REAIS que o Cosca já produziu nas ondas de
segurança e no CKL — cada uma com impacto medido de evidências reais
(testes da jail, auditorias red team, commits). O seed é idempotente:
nunca sobrescreve um arquivo existente.`,
		Example: `  cosca knowledge discovery list
  cosca knowledge discovery list --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			path, err := resolveHallOfFamePath()
			if err != nil {
				return err
			}

			// Primeira execução: semear as 5 descobertas reais
			// (idempotente — um arquivo existente nunca é sobrescrito).
			seeded, err := ensureSeededHallOfFame(path)
			if err != nil {
				return err
			}

			h, err := loadHallOfFame(path)
			if err != nil {
				return err
			}
			items := h.All()

			if seeded {
				formatter.Verbose(fmt.Sprintf(
					"Primeira execução: %d descobertas seed criadas em %s (runtime, gitignored)",
					len(items), path))
			}

			if useJSON {
				return printJSON(cmd, items)
			}

			if len(items) == 0 {
				formatter.Warning("Nenhuma descoberta registrada ainda — use \"cosca knowledge discovery add --name ... --impact ...\".")
				return nil
			}

			formatter.Header(fmt.Sprintf("Hall da Fama do Conhecimento (%d)", len(items)))
			rows := make([][]string, 0, len(items))
			for _, d := range items {
				rows = append(rows, []string{
					d.ID,
					d.Name,
					d.Stars(),
					fmt.Sprintf("%d", d.ProjectsAffected),
					fmt.Sprintf("-%.0f%%", d.RiskReductionPercent),
					d.RelatedLaw,
				})
			}
			formatter.Table([]string{"ID", "Nome", "Impacto", "Projetos", "Risco ↓", "Relacionado"}, rows)
			return nil
		},
	}
}

// NewKnowledgeDiscoveryShowCommand cria o comando
// `cosca knowledge discovery show <id>`.
func NewKnowledgeDiscoveryShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "show <id>",
		Short:   "Mostra a descoberta completa do Hall da Fama",
		Example: `  cosca knowledge discovery show D-01`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			path, err := resolveHallOfFamePath()
			if err != nil {
				return err
			}
			h, err := loadHallOfFame(path)
			if err != nil {
				return err
			}

			d, ok := h.Get(args[0])
			if !ok {
				return fmt.Errorf("descoberta %q não encontrada em %s", args[0], path)
			}

			if useJSON {
				return printJSON(cmd, d)
			}

			formatter.Header(fmt.Sprintf("Descoberta %s — %s", d.ID, d.Name))
			formatter.KeyValue("Impacto", fmt.Sprintf("%s (%d/5)", d.Stars(), d.Impact))
			formatter.KeyValue("Descrição", d.Description)
			formatter.KeyValue("Projetos afetados", fmt.Sprintf("%d", d.ProjectsAffected))
			formatter.KeyValue("Redução de risco", fmt.Sprintf("-%.0f%%", d.RiskReductionPercent))
			formatter.KeyValue("Sessões validadas", fmt.Sprintf("%d", d.ValidatedSessions))
			if d.EconomyPercent > 0 {
				formatter.KeyValue("Economia (%)", fmt.Sprintf("%.0f%%", d.EconomyPercent))
			}
			if d.EconomyUSD > 0 {
				formatter.KeyValue("Economia (USD)", fmt.Sprintf("US$%.2f", d.EconomyUSD))
			}
			formatter.KeyValue("Origem", d.Origin)
			if d.RelatedLaw != "" {
				formatter.KeyValue("Lei relacionada (CKL)", d.RelatedLaw)
			}
			formatter.KeyValue("Criada em", d.CreatedAt.Format("2006-01-02"))
			return nil
		},
	}
}

// NewKnowledgeDiscoveryAddCommand cria o comando
// `cosca knowledge discovery add`.
func NewKnowledgeDiscoveryAddCommand() *cobra.Command {
	var (
		name           string
		desc           string
		impact         int
		projects       int
		riskReduction  float64
		economyPercent float64
		economyUSD     float64
		sessions       int
		origin         string
		law            string
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Registra uma nova descoberta no Hall da Fama",
		Long: `Registra uma nova descoberta no Hall da Fama do conhecimento.

O ID é gerado automaticamente (D-06, D-07, ...) a partir do maior ID
existente. A descoberta persiste em .cosca/knowledge/hall-of-fame.json.

--impact é a escala de estrelas 1-5 (valor da descoberta) — NÃO confundir
com a confiança 0-1 do CKL. --risk-reduction é a redução de risco
operacional em % (0-100). --origin deve ser rastreável (Princípio 2):
auditoria, red team, conversa do Don. --law liga a descoberta a uma lei
do CKL (ex: K-01).`,
		Example: `  cosca knowledge discovery add --name "Descoberta X" --impact 4 --risk-reduction 55 --origin "auditoria #52" --law K-01
  cosca knowledge discovery add --name "RAG otimizado" --impact 3 --economy-percent 22 --sessions 12 --origin "conversa do Don"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			path, err := resolveHallOfFamePath()
			if err != nil {
				return err
			}
			h, err := loadHallOfFame(path)
			if err != nil {
				return err
			}

			// Impacto é escala de estrelas 1-5 (valor), não confidence 0-1.
			if impact < 1 || impact > 5 {
				return fmt.Errorf("--impact deve estar entre 1 e 5 estrelas (recebido %d)", impact)
			}
			if riskReduction < 0 || riskReduction > 100 {
				return fmt.Errorf("--risk-reduction deve estar entre 0 e 100 (recebido %.1f)", riskReduction)
			}
			if economyPercent < 0 || economyPercent > 100 {
				return fmt.Errorf("--economy-percent deve estar entre 0 e 100 (recebido %.1f)", economyPercent)
			}
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("--name é obrigatório para registrar uma descoberta")
			}

			// Origem default rastreável: sem origem, a descoberta não tem
			// rastro (Princípio 2). Quem registra via CLI assina o registro.
			if strings.TrimSpace(origin) == "" {
				origin = "CLI (registro manual)"
			}

			id := nextDiscoveryID(h)
			d := &knowledge.Discovery{
				ID:                   id,
				Name:                 strings.TrimSpace(name),
				Description:          desc,
				Impact:               impact,
				ProjectsAffected:     projects,
				EconomyPercent:       economyPercent,
				EconomyUSD:           economyUSD,
				RiskReductionPercent: riskReduction,
				ValidatedSessions:    sessions,
				Origin:               strings.TrimSpace(origin),
				RelatedLaw:           strings.TrimSpace(law),
			}
			if err := h.Register(d); err != nil {
				return err
			}
			if err := h.Save(path); err != nil {
				return fmt.Errorf("salvar %s: %w", path, err)
			}

			if useJSON {
				return printJSON(cmd, d)
			}

			formatter.Success(fmt.Sprintf(
				"Descoberta %s registrada — %q (impacto %s)", d.ID, d.Name, d.Stars()))
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "nome da descoberta (obrigatório)")
	cmd.Flags().StringVar(&desc, "desc", "", "descrição — o que mudou")
	cmd.Flags().IntVar(&impact, "impact", 0, "impacto em estrelas 1-5 (obrigatório)")
	cmd.Flags().IntVar(&projects, "projects", 0, "projetos afetados")
	cmd.Flags().Float64Var(&riskReduction, "risk-reduction", 0, "redução de risco operacional em % (0-100)")
	cmd.Flags().Float64Var(&economyPercent, "economy-percent", 0, "economia em % (0-100)")
	cmd.Flags().Float64Var(&economyUSD, "economy-usd", 0, "economia em USD")
	cmd.Flags().IntVar(&sessions, "sessions", 0, "sessões que validaram")
	cmd.Flags().StringVar(&origin, "origin", "", "origem rastreável (auditoria, red team, conversa do Don)")
	cmd.Flags().StringVar(&law, "law", "", "lei do CKL relacionada (ex: K-01)")
	return cmd
}
