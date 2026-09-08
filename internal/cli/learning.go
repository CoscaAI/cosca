package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/embeddings"
	"github.com/CoscaAI/cosca/internal/learning"
	"github.com/CoscaAI/cosca/internal/providers/ollama"
)

// NewLearningCommand creates the `cosca learning` command (ADR-044).
// Learning Vaults por Departamento — bancos enxutos de gatilhos por departamento.
func NewLearningCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "learning",
		Short: "Learning Vaults por departamento (ADR-044)",
		Long: `Learning Vaults por departamento (ADR-044).

Bancos SQLite enxutos, um por departamento, contendo APENAS gatilhos
(trigger + hash + pointer para blocks/{sha256}.md) — nunca conteúdo completo.
O conteúdo imutável vive nos blocos chain-tracked.

Subcomandos:
  rebuild   Reconstrói todos os vaults do zero (chain.dat + blocks)
  search    Busca semântica (FTS5) em um vault
  stats     Estatísticas dos vaults
  list      Lista os departamentos (vaults) e seus agents`,
		Example: `  cosca learning rebuild
  cosca learning search "chain invalida" --vault kernel
  cosca learning stats
  cosca learning list`,
	}

	cmd.AddCommand(
		newLearningRebuildCommand(),
		newLearningEmbedCommand(),
		newLearningSearchCommand(),
		newLearningStatsCommand(),
		newLearningListCommand(),
	)
	return cmd
}

// newLearningEmbedCommand: `cosca learning embed` — vetoriza os gatilhos
// sem embedding (idempotente). Requer o provider local (ollama nomic).
// --force re-embeda todos (útil após melhorar o texto embedado).
func newLearningEmbedCommand() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "embed",
		Short: "Vetoriza gatilhos sem embedding (idempotente, busca semântica)",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultDir, err := learningVaultDir()
			if err != nil {
				return err
			}
			reg, err := newEmbeddingRegistry()
			if err != nil {
				return err
			}
			defer reg.Close()

			if force {
				if err := learning.ClearEmbeddings(vaultDir); err != nil {
					return err
				}
			}

			n, err := learning.EmbedAll(cmd.Context(), reg, vaultDir, learningAgentsRoot())
			if err != nil {
				return err
			}
			formatter := GetFormatter(cmd)
			formatter.Success(fmt.Sprintf("Embeddings gerados: %d gatilhos vetorizados", n))
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "re-embeda todos (apaga embeddings existentes)")
	return cmd
}

// newEmbeddingRegistry abre o registry de embeddings com o provider local
// (ollama + nomic-embed-text — mesmo caminho do memory semantic).
func newEmbeddingRegistry() (*embeddings.ProviderRegistry, error) {
	reg := embeddings.GetRegistry()
	ollama.Register()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := reg.Select(ctx, embeddings.ProviderRegistryConfig{
		Primary: "ollama",
		Model:   "nomic-embed-text",
	}); err != nil {
		return nil, fmt.Errorf("selecionar provider de embedding: %w", err)
	}
	return reg, nil
}

// learningVaultDir resolves the vault dir (.cosca/learning).
func learningVaultDir() (string, error) {
	dir, err := resolveDataDir("")
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "learning"), nil
}

// learningAgentsRoot resolves the agents memory root (internal/embed).
func learningAgentsRoot() string {
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, "internal", "embed", "cosca", "memory", "agent")
}

func newLearningRebuildCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "rebuild",
		Short: "Reconstrói todos os vaults do zero (chain.dat + blocks)",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultDir, err := learningVaultDir()
			if err != nil {
				return err
			}
			agentsRoot := learningAgentsRoot()
			if _, err := os.Stat(agentsRoot); err != nil {
				return fmt.Errorf("agents root não encontrado: %w", err)
			}

			res, err := learning.Rebuild(vaultDir, agentsRoot)
			if err != nil {
				return err
			}

			formatter := GetFormatter(cmd)
			formatter.Success(fmt.Sprintf("Learning Vaults reconstruídos: %d triggers, %d agents, %d blocos faltando",
				res.TotalTriggers, res.AgentsScanned, res.MissingBlocks))

			names := make([]string, 0, len(res.Vaults))
			for v := range res.Vaults {
				names = append(names, string(v))
			}
			sort.Strings(names)
			for _, name := range names {
				formatter.Bullet(fmt.Sprintf("%s.db: %d triggers", name, res.Vaults[learning.VaultName(name)]))
			}
			return nil
		},
	}
}

func newLearningSearchCommand() *cobra.Command {
	var vault string
	var limit int
	var semantic bool
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Busca em um vault (FTS5 ou semântica com --semantic)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if vault == "" {
				return fmt.Errorf("--vault é obrigatório (ex: --vault kernel; veja 'cosca learning list')")
			}
			vaultDir, err := learningVaultDir()
			if err != nil {
				return err
			}
			db, err := learning.OpenVault(vaultDir, learning.VaultName(vault))
			if err != nil {
				return err
			}
			defer db.Close()

			formatter := GetFormatter(cmd)

			var results []learning.Trigger
			if semantic {
				reg, rErr := newEmbeddingRegistry()
				if rErr != nil {
					return rErr
				}
				defer reg.Close()
				results, err = learning.HybridSearch(cmd.Context(), reg, db, args[0], limit)
			} else {
				q := learning.SanitizeFTS(args[0])
				results, err = learning.Search(db, q, limit)
			}
			if err != nil {
				return err
			}

			if len(results) == 0 {
				formatter.Warning(fmt.Sprintf("Nenhum trigger encontrado no vault %s para: %s", vault, args[0]))
				return nil
			}
			mode := "FTS5"
			if semantic {
				mode = "híbrida (FTS5 + semântica)"
			}
			formatter.Success(fmt.Sprintf("%d resultado(s) no vault %s (busca %s):", len(results), vault, mode))
			for _, t := range results {
				formatter.Bullet(fmt.Sprintf("%s | %s | %s | L%d | %s | %s",
					t.ID, t.Date, t.Title, t.Level, t.Tags, t.Hash16))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&vault, "vault", "", "departamento (vault) para buscar")
	cmd.Flags().IntVar(&limit, "limit", 20, "máximo de resultados")
	cmd.Flags().BoolVar(&semantic, "semantic", false, "busca semântica por entendimento (requer embeddings)")
	return cmd
}

func newLearningStatsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Estatísticas dos vaults",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultDir, err := learningVaultDir()
			if err != nil {
				return err
			}
			formatter := GetFormatter(cmd)

			entries, err := os.ReadDir(vaultDir)
			if err != nil {
				if os.IsNotExist(err) {
					formatter.Warning("Nenhum vault ainda — rode 'cosca learning rebuild'")
					return nil
				}
				return err
			}

			total := 0
			for _, e := range entries {
				if e.IsDir() || filepath.Ext(e.Name()) != ".db" {
					continue
				}
				db, err := learning.OpenVault(vaultDir, learning.VaultName(strings.TrimSuffix(e.Name(), ".db")))
				if err != nil {
					continue
				}
				n, err := learning.Count(db)
				db.Close()
				if err != nil {
					continue
				}
				total += n
				formatter.Bullet(fmt.Sprintf("%s: %d triggers", e.Name(), n))
			}
			formatter.Success(fmt.Sprintf("Total: %d triggers", total))
			return nil
		},
	}
}

func newLearningListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Lista os departamentos (vaults) e seus agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			byVault := make(map[learning.VaultName][]string)
			for agent := range learning.AgentVaults() {
				v := learning.VaultForAgent(agent)
				byVault[v] = append(byVault[v], agent)
			}
			names := make([]string, 0, len(byVault))
			for v := range byVault {
				names = append(names, string(v))
			}
			sort.Strings(names)
			for _, name := range names {
				formatter.Bullet(fmt.Sprintf("%s: %d agents", name, len(byVault[learning.VaultName(name)])))
			}
			return nil
		},
	}
}