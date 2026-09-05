package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/CoscaAI/cosca/internal/integrity"
	"github.com/spf13/cobra"
)

// NewMemoryWatchCommand creates the `cosca memory watch` subcommand — o
// watchdog 24/7 do cofre. Câmera (fsnotify) + cachorro de guarda (chain) +
// audit log persistente (quem entra, quem chega perto, quem sai).
func NewMemoryWatchCommand() *cobra.Command {
	var (
		interval  int
		auditPath string
	)

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Vigiar o cofre 24h (watchdog + audit log)",
		Long: `Vigia o cofre da família em tempo real: internal/embed/cosca (a memória),
a family chain e as chaves. Qualquer escrita/criação/remoção dispara uma
verificação de integridade completa e registra o evento no audit log — quem
chegou perto, o que mudou, quando.

Roda como um cão de guarda: fica de plantão, late a cada movimento suspeito.`,
		Example: `  cosca memory watch
  cosca memory watch --audit .cosca/audit/watchdog.log --interval 15`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return err
			}

			if auditPath == "" {
				auditPath = filepath.Join(dir, ".cosca", "audit", "memory-watch.log")
			}
			if err := os.MkdirAll(filepath.Dir(auditPath), 0o700); err != nil {
				return err
			}
			audit, err := os.OpenFile(auditPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
			if err != nil {
				return fmt.Errorf("abrir audit log: %w", err)
			}
			defer audit.Close()

			done := make(chan struct{})
			cfg := integrity.WatchConfig{
				Root:     dir,
				Interval: time.Duration(interval) * time.Second,
			}

			record := func(line string) {
				fmt.Println(line)
				_, _ = fmt.Fprintln(audit, line)
			}

			fmt.Printf("👁️  Vigilância 24h ativa — audit em %s\n", auditPath)
			return integrity.Watch(cfg, func(r integrity.WatchResult) {
				ts := r.Time.Format(time.RFC3339)
				record(fmt.Sprintf("[%s] %s", ts, r.Message))
				for _, t := range r.Tampers {
					record(fmt.Sprintf("[%s]   📁 %s — %s", ts, t.Path, t.Diff))
				}
			}, done)
		},
	}

	cmd.Flags().IntVar(&interval, "interval", 30, "Intervalo de polling (s) como fallback do fsnotify")
	cmd.Flags().StringVar(&auditPath, "audit", "", "Caminho do audit log (default: .cosca/audit/memory-watch.log)")

	return cmd
}
