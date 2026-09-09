package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
)

// NewKnowledgeSemanticDiscoveryCommand cria `cosca knowledge discover-semantic <query>`.
//
// FASE 5.4 â€” descobrimento semÃ¢ntico ENTRE agentes: busca por SIGNIFICADO em
// todo o corpus (cobertura completa apÃ³s 5.3) e anota cada resultado com o
// agente dono. Um console semÃ¢ntico que atravessa a famÃ­lia, sem confinar a um
// agente/caminho.
func NewKnowledgeSemanticDiscoveryCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "discover-semantic <query>",
		Short: "Busca por significado em toda a famÃ­lia (atravessa agentes por sentido)",
		Long: `Busca por SIGNIFICADO em todo o corpus de conhecimento (cobertura completa apÃ³s a
5.3) e anota cada resultado com o agente dono (.cosca/memory/agent/<nome>).
Provando a recuperaÃ§Ã£o ENTRE agentes: uma consulta semÃ¢ntica atravessa a famÃ­lia,
nÃ£o um Ãºnico agente/caminho. Ã‰ o modo "perguntar a famÃ­lia inteira por sentido".`,
		Example: `  cosca knowledge discover-semantic "seguranca de identidade da familia"
  cosca knowledge discover-semantic "como evitar erro no codigo"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, err := os.Getwd()
			if err != nil {
				return err
			}
			kbDir := filepath.Join(dir, ".cosca")
			ke, kErr := openKnowledgeEngine(kbDir, false, dir)
			if kErr != nil {
				return fmt.Errorf("knowledge engine not available: %w", kErr)
			}
			if iErr := ke.Init(); iErr != nil {
				return fmt.Errorf("knowledge engine init failed: %w", iErr)
			}
			defer ke.Close()

			res, dErr := ke.SemanticDiscovery(cmd.Context(), args[0], 20)
			if dErr != nil {
				return fmt.Errorf("semantic discovery: %w", dErr)
			}

			if useJSON {
				return printJSON(cmd, res)
			}

			formatter.Header(fmt.Sprintf("Descobrimento semÃ¢ntico â€” %d hits de %d agentes", len(res.Hits), res.DistinctAgents))
			keys := make([]string, 0, len(res.ByAgent))
			for a := range res.ByAgent {
				keys = append(keys, a)
			}
			sort.Strings(keys)
			for _, a := range keys {
				formatter.Bullet(fmt.Sprintf("agente %-28s %d", a, res.ByAgent[a]))
			}
			for _, h := range res.Hits {
				formatter.Bullet(fmt.Sprintf("[%s] score=%.3f  %s", h.Agent, h.Score, h.Snippet))
			}
			return nil
		},
	}
}
