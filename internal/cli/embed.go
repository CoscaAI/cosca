package cli

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// classifyEmbedPath classifica um arquivo do embed pela sua função no cérebro.
// Responde às perguntas do checklist: é regra? memória? conhecimento?
// segurança? filosofia? arquitetura? agente? skill? artefato?
func classifyEmbedPath(rel string) string {
	switch {
	case strings.HasPrefix(rel, "memory/"):
		return "MEMÓRIA"
	case strings.HasPrefix(rel, "knowledge/"):
		return "CONHECIMENTO"
	case strings.HasPrefix(rel, "agents/"):
		return "AGENTES"
	case strings.HasPrefix(rel, "skills/"):
		return "SKILLS"
	case strings.HasPrefix(rel, "departments/"):
		return "DEPARTAMENTOS"
	case strings.HasPrefix(rel, "councils/"):
		return "CONSELHOS"
	case strings.HasPrefix(rel, "company/"):
		return "EMPRESA"
	case strings.HasPrefix(rel, "workflows/"):
		return "WORKFLOWS"
	case strings.HasPrefix(rel, "architecture/"):
		return "ARQUITETURA"
	case strings.HasPrefix(rel, "capabilities/"):
		return "CAPACIDADES"
	case strings.HasPrefix(rel, "prompts/"):
		return "PROMPTS"
	case strings.HasPrefix(rel, "shared/"):
		return "COMPARTILHADO"
	case strings.HasPrefix(rel, "keys/"):
		return "CHAVES"
	case strings.HasPrefix(rel, "templates/"):
		return "TEMPLATES"
	case strings.HasPrefix(rel, "evals/"):
		return "AVALIAÇÕES"
	case strings.HasPrefix(rel, "archive/"):
		return "ARQUIVO"
	case strings.HasSuffix(rel, ".go"):
		return "CÓDIGO (self)"
	default:
		// .md / .yaml na raiz são leis, identidade, filosofia, segurança.
		return "LEI/IDENTIDADE"
	}
}

// NewEmbedCommand cria o comando `cosca embed` — arqueologia do cérebro.
func NewEmbedCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "embed",
		Short: "Arqueologia do cérebro — auditoria, inventário e classificação do embed",
		Long:  "Audita o internal/embed/cosca/ (o cérebro) em modo READ-ONLY: inventário, classificação, duplicação, órfãos e provenance. NUNCA modifica nada — o relatório é a evidência para a aprovação do Don (P8).",
	}
	cmd.AddCommand(newEmbedAuditCommand())
	return cmd
}

func newEmbedAuditCommand() *cobra.Command {
	var showDup bool
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Inventário + classificação + duplicação + provenance do embed",
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "internal/embed/cosca"

			counts := map[string]int{}
			totalBytes := int64(0)
			byHash := map[string][]string{} // hash -> paths (para duplicação)
			dups := 0

			err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if info.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(root, path)
				// filepath.Rel usa o separador do SO (\" no Windows), mas
				// classifyEmbedPath e o manifest da chain usam \"/\" canônico.
				rel = filepath.ToSlash(rel)
				cls := classifyEmbedPath(rel)
				counts[cls]++
				totalBytes += info.Size()

				if data, rerr := os.ReadFile(path); rerr == nil {
					h := fmt.Sprintf("%x", sha256.Sum256(data))
					byHash[h] = append(byHash[h], rel)
				}
				return nil
			})
			if err != nil {
				return err
			}

			// Duplicação: hash com mais de 1 arquivo.
			var dupPairs []string
			for _, paths := range byHash {
				if len(paths) > 1 {
					dups += len(paths) - 1
					dupPairs = append(dupPairs, fmt.Sprintf("  [%d×] %s", len(paths), strings.Join(paths, " = ")))
				}
			}
			sort.Strings(dupPairs)

			// Provenance: quantos estão no manifest assinado da chain.
			inChain, outChain := provenanceCount(root)

			// Relatório.
			fmt.Println("════════ ARQUEOLOGIA DO EMBED (read-only) ════════")
			fmt.Println()
			fmt.Println("Inventário por classificação:")
			var keys []string
			for k := range counts {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			total := 0
			for _, k := range keys {
				fmt.Printf("  %-18s %d\n", k, counts[k])
				total += counts[k]
			}
			fmt.Printf("\n  TOTAL: %d arquivos, %.1f MB\n", total, float64(totalBytes)/(1024*1024))
			fmt.Printf("  PROVENANCE: %d na chain, %d fora\n", inChain, outChain)
			fmt.Printf("  DUPLICAÇÃO: %d arquivo(s) redundante(s)\n", dups)
			if showDup && len(dupPairs) > 0 {
				fmt.Println("\n  Pares duplicados:")
				for _, p := range dupPairs {
					fmt.Println(p)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&showDup, "show-dup", false, "exibir pares de arquivos duplicados")
	return cmd
}

// provenanceCount conta quantos arquivos do embed estão no manifest assinado
// da chain (o último bloco de .cosca/family_chain.dat).
func provenanceCount(root string) (inChain, outChain int) {
	manifest := readChainManifest()
	if manifest == nil {
		return 0, 0
	}
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		// Mesmo fix do audit: ToSlash para casar com o manifest (\" canônico).
		rel = filepath.ToSlash(rel)
		if _, ok := manifest["internal/embed/cosca/"+rel]; ok {
			inChain++
		} else {
			outChain++
		}
		return nil
	})
	return inChain, outChain
}

// readChainManifest lê o manifest do último bloco da chain (mapa path→hash).
func readChainManifest() map[string]string {
	wd, _ := os.Getwd()
	data, err := os.ReadFile(filepath.Join(wd, ".cosca", "family_chain.dat"))
	if err != nil {
		return nil
	}
	blocks := strings.Split(string(data), "======= BLOCK")
	last := blocks[len(blocks)-1]
	parts := strings.SplitN(last, "---", 2)
	if len(parts) < 2 {
		return nil
	}
	var entries []struct {
		Path string `json:"path"`
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(parts[1])), &entries); err != nil {
		return nil
	}
	m := make(map[string]string, len(entries))
	for _, e := range entries {
		m[e.Path] = e.Hash
	}
	return m
}
