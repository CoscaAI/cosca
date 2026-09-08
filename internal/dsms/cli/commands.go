// Package cli provides the DSMS command-line commands.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"cosca/internal/dsms"
	"cosca/internal/dsms/intelligence"
	"cosca/internal/dsms/intelligence/csnparser"
	"cosca/internal/dsms/intelligence/expander"
	"cosca/internal/dsms/intelligence/rules"
	"cosca/internal/dsms/intelligence/scanner"
	"cosca/internal/dsms/intelligence/trainer"
	"cosca/internal/dsms/scheduler"
	"cosca/internal/dsms/storage"
)

// ============================================================
// TRAIN COMMAND
// ============================================================

// TrainCmd trains the engine from knowledge sources.
func TrainCmd() *cobra.Command {
	var (
		csnDir    string
		knowledge string
		extDir    string
		output    string
	)

	cmd := &cobra.Command{
		Use:   "train",
		Short: "Treina o motor com conhecimento (CSN, knowledge.db, E:\\)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()

			allRules := make([]*intelligence.Rule, 0)
			engine := rules.NewEngine()

			// 1. CodeSearchNet (ALL pairs)
			if csnDir != "" {
				fmt.Printf("[1/3] CodeSearchNet: %s\n", csnDir)
				if _, err := os.Stat(csnDir); err == nil {
					extracted, result, err := csnparser.ExtractAllRules(ctx, csnDir)
					if err != nil {
						return fmt.Errorf("csn extract: %w", err)
					}
					fmt.Printf("  Pares: %d (%d arquivos)\n", result.RowsRead, result.FilesRead)
					allRules = append(allRules, extracted...)
					for _, r := range extracted {
						engine.Register(r)
					}
					fmt.Printf("  Regras: %d\n", len(extracted))
				}
			}

			// 2. Knowledge DB (ALL rules)
			if knowledge != "" {
				fmt.Printf("[2/3] Knowledge DB: %s\n", knowledge)
				if _, err := os.Stat(knowledge); err == nil {
					tr := trainer.NewTrainer(knowledge)
					entries, err := tr.LoadKnowledge(ctx, 120000)
					if err != nil {
						fmt.Printf("  [warn] %v\n", err)
					} else {
						result := tr.Train(ctx, entries)
						// Register ALL extracted rules
						for _, r := range result.AllRules {
							allRules = append(allRules, r)
							engine.Register(r)
						}
						fmt.Printf("  Entradas: %d, Regras: %d\n", result.EntriesRead, result.RulesExtracted)
					}
				}
			}

			// 3. Expander (E:\)
			if extDir != "" {
				fmt.Printf("[3/3] Expander: %s\n", extDir)
				exp := expander.NewExpander()
				result, err := exp.Scan(ctx, 20000)
				if err == nil {
					extracted := expander.ExtractRules(result.SampleDocs)
					allRules = append(allRules, extracted...)
					for _, r := range extracted {
						engine.Register(r)
					}
					fmt.Printf("  Arquivos: %d, Regras: %d\n", result.FilesFound, len(extracted))
				}
			}

			// Save to JSONL
			if output == "" {
				output = storage.DefaultDir()
			}
			st := storage.NewStorage(output)
			path, err := st.SaveRules("train", allRules)
			if err != nil {
				return fmt.Errorf("save: %w", err)
			}

			fmt.Printf("\n✅ Treinamento concluído: %d regras salvas em %s\n", len(allRules), path)
			return nil
		},
	}

	cmd.Flags().StringVar(&csnDir, "csn", `E:\trainer`, "Diretório com parquet do CodeSearchNet")
	cmd.Flags().StringVar(&knowledge, "knowledge", "", "Path do knowledge.db")
	cmd.Flags().StringVar(&extDir, "ext", "", "Diretório externo (E:\\)")
	cmd.Flags().StringVar(&output, "output", "", "Diretório de saída JSONL")

	return cmd
}

// ============================================================
// SCAN COMMAND
// ============================================================

// ScanCmd scans a codebase for issues.
func ScanCmd() *cobra.Command {
	var (
		dir      string
		rulesDir string
		output   string
	)

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Escaneia código com o Intelligence Engine (zero LLM)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dir == "" {
				dir, _ = os.Getwd()
			}

			fmt.Printf("Escaneando: %s\n", dir)

			// Load rules from JSONL if provided
			engine := rules.NewEngine()
			if rulesDir != "" {
				st := storage.NewStorage(rulesDir)
				loaded, err := st.LoadRules()
				if err != nil {
					return fmt.Errorf("load rules: %w", err)
				}
				for _, r := range loaded {
					engine.Register(r)
				}
				fmt.Printf("Regras carregadas: %d\n", len(loaded))
			} else {
				// Use default rules
				engine.RegisterMany(rules.AllDefaultRules())
				fmt.Printf("Regras default: %d\n", len(rules.AllDefaultRules()))
			}

			// Scan
			sc := scanner.NewScanner(dir)
			result, err := sc.Scan()
			if err != nil {
				return fmt.Errorf("scan: %w", err)
			}

			// Report
			fmt.Print(scanner.Report(result))

			// Save results
			if output != "" {
				if err := os.MkdirAll(output, 0755); err == nil {
					reportPath := filepath.Join(output, fmt.Sprintf("scan-%s.txt", time.Now().Format("20060102-150405")))
					os.WriteFile(reportPath, []byte(scanner.Report(result)), 0644)
					fmt.Printf("Relatório salvo: %s\n", reportPath)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "", "Diretório para escanear")
	cmd.Flags().StringVar(&rulesDir, "rules", "", "Diretório com regras JSONL")
	cmd.Flags().StringVar(&output, "output", "", "Diretório para salvar relatório")

	return cmd
}

// ============================================================
// RULES COMMAND
// ============================================================

// RulesCmd lists loaded rules.
func RulesCmd() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Lista regras do motor",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dir == "" {
				dir = storage.DefaultDir()
			}

			st := storage.NewStorage(dir)
			loaded, err := st.LoadRules()
			if err != nil {
				return fmt.Errorf("load rules: %w", err)
			}

			// Group by domain
			byDomain := make(map[string]int)
			for _, r := range loaded {
				byDomain[r.Domain]++
			}

			fmt.Printf("=== REGRAS (%d total) ===\n", len(loaded))
			for domain, count := range byDomain {
				fmt.Printf("  %s: %d\n", domain, count)
			}

			// Show sample
			fmt.Printf("\n=== AMOSTRA ===\n")
			shown := 0
			for _, r := range loaded {
				if shown >= 10 {
					break
				}
				fmt.Printf("  [%s] %s (%s, conf %.0f%%)\n", r.Domain, r.Name, r.ID, r.Confidence*100)
				shown++
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "", "Diretório com regras JSONL")

	return cmd
}

// ============================================================
// SERVE COMMAND
// ============================================================

// ServeCmd runs the DSMS infinite loop as an independent process.
// It combines: database maintenance loop + rule engine watching.
func ServeCmd() *cobra.Command {
	var (
		rulesDir   string
		watchDir   string
		dataDir    string
		scanEvery  time.Duration
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Roda o DSMS como processo independente (loop infinito)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Signal handling for graceful shutdown
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
			go func() {
				<-sigChan
				fmt.Println("\n[DSMS] Interrompido. Encerrando...")
				cancel()
			}()

			if dataDir == "" {
				dataDir = filepath.Join("dsms-data", "db")
			}
			if rulesDir == "" {
				rulesDir = storage.DefaultDir()
			}

			fmt.Println("══════════════════════════════════════════")
			fmt.Println("  DSMS SERVE — processo independente")
			fmt.Println("══════════════════════════════════════════")
			fmt.Printf("  Data dir: %s\n", dataDir)
			fmt.Printf("  Rules dir: %s\n", rulesDir)
			if watchDir != "" {
				fmt.Printf("  Watch dir: %s (scan a cada %v)\n", watchDir, scanEvery)
			}
			fmt.Println("  Ctrl+C para encerrar")
			fmt.Println("══════════════════════════════════════════")

			// 1. Open DSMS databases
			d := dsms.New(dsms.DefaultConfig(dataDir))
			if err := d.Open(ctx); err != nil {
				return fmt.Errorf("open dsms: %w", err)
			}
			defer d.Close()

			// 2. Load trained rules
			engine := rules.NewEngine()
			st := storage.NewStorage(rulesDir)
			loaded, err := st.LoadRules()
			if err != nil {
				return fmt.Errorf("load rules: %w", err)
			}
			for _, r := range loaded {
				engine.Register(r)
			}
			fmt.Printf("[DSMS] %d regras carregadas\n", len(loaded))

			// 3. Start database maintenance scheduler (background)
			sched := scheduler.NewScheduler(d, nil)
			schedErrChan := make(chan error, 1)
			go func() {
				schedErrChan <- sched.Start(ctx)
			}()
			fmt.Println("[DSMS] Scheduler de manutenção iniciado")

			// 4. If watch dir set, scan periodically
			if watchDir != "" {
				go watchLoop(ctx, watchDir, engine, scanEvery)
				fmt.Printf("[DSMS] Vigilância ativa em %s\n", watchDir)
			}

			// 5. Wait for shutdown
			<-ctx.Done()
			sched.Stop()
			fmt.Println("[DSMS] Encerrado com sucesso")
			return nil
		},
	}

	cmd.Flags().StringVar(&rulesDir, "rules", "", "Diretório com regras JSONL (default: dsms-data/rules)")
	cmd.Flags().StringVar(&watchDir, "watch", "", "Diretório para escanear continuamente")
	cmd.Flags().StringVar(&dataDir, "data", "", "Diretório de dados DSMS (default: dsms-data/db)")
	cmd.Flags().DurationVar(&scanEvery, "scan-every", 1*time.Minute, "Intervalo de scan do diretório vigiado")

	return cmd
}

// watchLoop scans the watch directory periodically using the loaded rules.
func watchLoop(ctx context.Context, watchDir string, engine *rules.Engine, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	fmt.Printf("[DSMS] Scan inicial de %s...\n", watchDir)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fmt.Printf("[DSMS] Escaneando %s...\n", watchDir)
			scanAndReport(watchDir, engine)
		}
	}
}

// scanAndReport scans a directory and reports findings.
func scanAndReport(dir string, engine *rules.Engine) {
	sc := scanner.NewScanner(dir)
	result, err := sc.Scan()
	if err != nil {
		fmt.Printf("[DSMS] Erro no scan: %v\n", err)
		return
	}

	fmt.Printf("[DSMS] Scan: %d arquivos, %d findings em %v\n",
		result.FilesAnalyzed, result.TotalFindings, result.Duration)

	if result.TotalFindings > 0 {
		// Print critical/warning findings only for brevity
		for _, f := range result.TopFindings {
			if f.Severity == "critical" {
				fmt.Printf("  [CRITICAL] %s (%s): %d ocorrências\n", f.RuleName, f.Domain, f.Count)
			}
		}
	}
}