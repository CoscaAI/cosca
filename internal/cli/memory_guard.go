package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/CoscaAI/cosca/internal/memoryguard"
	"github.com/spf13/cobra"
)

// NewMemoryGuardCommand cria o `cosca memory guard` — o oráculo de memória
// (quarta muralha). Valida o índice de gatilhos contra a régua da casa:
// auto-promoção, narrativa inflada e nível inexistente = DENY.
func NewMemoryGuardCommand() *cobra.Command {
	var index string
	cmd := &cobra.Command{
		Use:   "guard",
		Short: "Valida a memória contra a régua (auto-promoção, narrativa inflada, nível inválido)",
		Long: `O oráculo de memória (P15 + quarta muralha). Percorre o índice de gatilhos
e denuncia qualquer aprendizado que viole a régua da casa:
  - nível acima de 5 (auto-promoção — o nível 9 está OFF)
  - narrativa inflada (vaidade = porta de manipulação de memória)
  - nível alto sem evidência declarada (P13)

Saída: exit 1 se houver violação, exit 0 se a memória estiver limpa.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := index
			if path == "" {
				wd, _ := os.Getwd()
				path = filepath.Join(wd, "internal", "embed", "cosca", "memory", "agent", "cosca-kernel", "learnings.md")
			}

			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()

			violations := 0
			total := 0
			scanner := bufio.NewScanner(f)
			scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
			for scanner.Scan() {
				line := scanner.Text()
				if !strings.HasPrefix(strings.TrimSpace(line), "## L") {
					continue
				}
				total++
				v := memoryguard.ValidateIndexLine(line)
				if !v.Approved {
					violations++
					display := line
					if len(display) > 80 {
						display = display[:80] + "…"
					}
					fmt.Printf("✗ DENY: %s\n", display)
					for _, r := range v.Reasons {
						fmt.Printf("    → %s\n", r)
					}
				}
			}
			if err := scanner.Err(); err != nil {
				return err
			}

			if violations == 0 {
				fmt.Printf("✓ %d aprendizados validados — memória limpa (nenhuma violação da régua)\n", total)
				return nil
			}
			fmt.Printf("\n✗ %d de %d aprendizados violam a régua\n", violations, total)
			return fmt.Errorf("memória com %d violações", violations)
		},
	}
	cmd.Flags().StringVar(&index, "index", "", "caminho do índice de gatilhos (default: learnings.md do kernel)")
	return cmd
}
