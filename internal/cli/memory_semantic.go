// Package cli — `cosca memory semantic`: o retrieval semântico de memória.
//
// Fecha o gap §4.3 do INDEX.md C2 ("No indexed memory retrieval — retrieval
// still path-first"): busca memória por SIGNIFICADO, não por caminho.
//
//	cosca memory semantic index          # embebe os registros
//	cosca memory semantic search "query" # busca por similaridade
//	cosca memory semantic status         # quantos vetores indexados
package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/embeddings"
	"github.com/CoscaAI/cosca/internal/memory"
	"github.com/CoscaAI/cosca/internal/providers/ollama"
)

// NewMemorySemanticCommand cria `cosca memory semantic`.
func NewMemorySemanticCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "semantic",
		Short: "Retrieval semântico de memória — busca por significado (gap §4.3 C2)",
		Long: `Retrieval semântico de memória: embebe os registros (nomic-embed-text)
e busca por similaridade de cosseno — "search by meaning, not path".

Subcomandos:
  index    Embebe os registros de memória no índice semântico
  search   Busca memória por significado (top-K por cosseno)
  status   Quantos vetores estão indexados`,
	}
	cmd.AddCommand(newMemorySemanticIndexCommand())
	cmd.AddCommand(newMemorySemanticSearchCommand())
	cmd.AddCommand(newMemorySemanticStatusCommand())
	return cmd
}

// semanticOpen abre o índice semântico com o provider de embeddings real
// (ollama + nomic-embed-text — o mesmo caminho do setup).
func semanticOpen() (*memory.SemanticMemory, error) {
	dir, err := resolveDataDir("")
	if err != nil {
		return nil, fmt.Errorf("resolve data directory: %w", err)
	}
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
	prov, ok := reg.Get("ollama")
	if !ok {
		return nil, fmt.Errorf("provider ollama não disponível")
	}
	return memory.OpenSemanticMemory(dir, prov)
}

func newMemorySemanticIndexCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "index",
		Short: "Embebe os registros de memória no índice semântico",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f := GetFormatter(cmd)
			dir, _ := resolveDataDir("")
			sem, err := semanticOpen()
			if err != nil {
				return err
			}
			defer sem.Close()

			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
			defer cancel()

			start := time.Now()
			indexed := 0
			// Varre os LAYERS de memória real (episodic/session/long/workspace)
			// e embebe cada arquivo .md no índice semântico. O FileStore de
			// cada layer resolve o diretório correto.
			layers := []memory.MemoryLayer{
				memory.LayerEpisodic,
				memory.LayerSession,
				memory.LayerLong,
				memory.LayerProject,
			}
			for _, layer := range layers {
				fs, err := memory.NewFileStore(dir, layer, zerolog.Nop())
				if err != nil {
					continue
				}
				recs, err := fs.Search(ctx, "", memory.SearchOptions{Limit: 5000})
				_ = fs.Close()
				if err != nil {
					continue // layer sem conteúdo — segue
				}
				for _, rec := range recs {
					if rec.Content == "" {
						continue
					}
					if err := sem.Index(ctx, rec); err != nil {
						continue // uma falha não derruba a indexação
					}
					indexed++
				}
			}
			n, _ := sem.Count()
			f.Success(fmt.Sprintf("Indexação semântica concluída: %d novos, %d vetores no total (%.0fs)",
				indexed, n, time.Since(start).Seconds()))
			return nil
		},
	}
}

func newMemorySemanticSearchCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Busca memória por significado (similaridade de cosseno)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f := GetFormatter(cmd)
			sem, err := semanticOpen()
			if err != nil {
				return err
			}
			defer sem.Close()

			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			results, err := sem.SearchSemantic(ctx, args[0], 5, "", "")
			if err != nil {
				return fmt.Errorf("busca semântica: %w", err)
			}
			f.Header(fmt.Sprintf("Memória semântica — %q", args[0]))
			if len(results) == 0 {
				f.Print("Nenhum resultado. Rode 'cosca memory semantic index' para indexar.")
				return nil
			}
			for _, r := range results {
				content := r.Content
				if len(content) > 80 {
					content = content[:80] + "..."
				}
				f.Printf("  [%.3f] %s (%s)\n", r.Score, content, r.Layer)
			}
			return nil
		},
	}
}

func newMemorySemanticStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Quantos vetores de memória estão indexados",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f := GetFormatter(cmd)
			sem, err := semanticOpen()
			if err != nil {
				return err
			}
			defer sem.Close()
			n, _ := sem.Count()
			f.Header("Memória semântica")
			f.KeyValue("Vetores indexados", fmt.Sprintf("%d", n))
			return nil
		},
	}
}
