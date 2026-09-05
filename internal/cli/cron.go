//
// `cosca cron` — Agendamento determinístico (scheduler.cron, KERNEL.md STABLE).
//
// Jobs cron da Cosca: comandos determinísticos (cosca CLI ou shell) que o Don
// define explicitamente e executa conforme uma agenda em linguagem natural.
//
// REGRA DO DON: nada roda sozinho. O runner só executa jobs quando
// `cosca cron daemon` está rodando em foreground (bloqueando) E o job foi
// adicionado pelo Don. Sem daemon, adicionar/listar/rodar manualmente são as
// únicas operações — o daemon é a única porta de execução automática.
//
// Subcomandos:
//   add --name <nome> --schedule <agenda> --cmd <comando>   Cria C-XXXX
//   list                                     Tabela (id, nome, agenda, habilitado, próxima)
//   remove <id>                              Remove o job
//   run <id>                                 Executa o job UMA vez agora (o Don inicia)
//   daemon                                   Roda o runner em foreground até Ctrl+C
//
// Agenda em linguagem natural (pt-BR/EN):
//   "every 30m", "every 2h", "every 15s", "every 5m30s"   → intervalo
//   "daily at 9am", "diariamente às 9:00", "every day at 08:30" → diária
//   "0 9 * * *"                                            → cron de 5 campos
//
// Jobs vivem em .cosca/cron.db (SQLite).
//

package cli

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/scheduler"
)

// resolveCronStore abre a base de jobs do projeto atual
// (<projeto>/.cosca/cron.db).
func resolveCronStore() (*scheduler.Store, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getwd: %w", err)
	}
	return scheduler.NewStore(filepath.Join(dir, ".cosca", "cron.db"))
}

// NewCronCommand cria a árvore de comandos `cosca cron`.
func NewCronCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cron",
		Short: "Agendamento determinístico (scheduler.cron) — jobs cron em linguagem natural",
		Long: `Agendamento determinístico da Cosca (scheduler.cron, KERNEL.md STABLE).

Jobs são comandos determinísticos (cosca CLI ou shell) que o Don define
explicitamente e executa conforme uma agenda em linguagem natural.

  every 30m / every 2h / a cada 30 minutos   → intervalo fixo
  daily at 9am / diariamente às 9:00          → diária em hora fixa
  0 9 * * *                                   → cron de 5 campos (subconjunto)

REGRA DO DON — nada roda sozinho: o runner executa jobs SOMENTE quando
"cosca cron daemon" está rodando (foreground, até Ctrl+C) E o job foi
adicionado pelo Don. Sem daemon, nada é executado automaticamente.

Subcomandos:
  add --name <nome> --schedule <agenda> --cmd <comando>   Cria o job C-XXXX
  list                                    Tabela (id, nome, agenda, habilitado, próxima)
  remove <id>                             Remove o job
  run <id>                                Executa o job UMA vez agora (o Don inicia)
  daemon                                  Roda o runner em foreground até Ctrl+C`,
		Example: `  cosca cron add --name "backup" --schedule "every 30m" --cmd "cosca cv snapshot"
  cosca cron add --name "daily report" --schedule "diariamente às 9:00" --cmd "cosca status"
  cosca cron list
  cosca cron run C-0001
  cosca cron daemon`,
	}

	cmd.AddCommand(
		NewCronAddCommand(),
		NewCronListCommand(),
		NewCronRemoveCommand(),
		NewCronRunCommand(),
		NewCronDaemonCommand(),
	)
	return cmd
}

// NewCronAddCommand cria `cosca cron add --name --schedule --cmd`.
func NewCronAddCommand() *cobra.Command {
	var (
		name     string
		schedule string
		command  string
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Cria um job cron C-XXXX em .cosca/cron.db",
		Long: `Cria um job cron: agenda em linguagem natural + comando determinístico
que o Don define. O job nasce habilitado; a primeira execução só ocorre
quando "cosca cron daemon" estiver rodando E o NextRun vencer.

Agendas (pt-BR/EN):
  "every 30m", "every 2h", "every 5m30s"          → intervalo
  "daily at 9am", "diariamente às 9:00"           → diária
  "0 9 * * *"                                     → cron de 5 campos`,
		Example: `  cosca cron add --name "backup" --schedule "every 30m" --cmd "cosca cv snapshot"
  cosca cron add --name "daily" --schedule "diariamente às 9:00" --cmd "cosca status"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			sch, err := scheduler.ParseNL(schedule)
			if err != nil {
				return err
			}

			store, err := resolveCronStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			id, err := store.Add(scheduler.Job{
				Name:     name,
				Command:  command,
				Schedule: *sch,
				Enabled:  true,
			})
			if err != nil {
				return err
			}
			job, err := store.Get(id)
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, job)
			}
			formatter.Success(fmt.Sprintf("Job %s criado — %q (%s)", job.ID, job.Name, job.Schedule.Human()))
			formatter.KeyValue("ID", job.ID)
			formatter.KeyValue("Nome", job.Name)
			formatter.KeyValue("Comando", job.Command)
			formatter.KeyValue("Agenda", job.Schedule.Human())
			formatter.KeyValue("Próxima execução", formatNextRun(job.NextRun))
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "nome do job (ex.: backup)")
	cmd.Flags().StringVar(&schedule, "schedule", "", "agenda em linguagem natural (ex.: every 30m, daily at 9am, 0 9 * * *)")
	cmd.Flags().StringVar(&command, "cmd", "", "comando determinístico (sh -c) que o Don define (ex.: cosca cv snapshot)")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("schedule")
	_ = cmd.MarkFlagRequired("cmd")
	return cmd
}

// NewCronListCommand cria `cosca cron list`.
func NewCronListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Lista os jobs cron (id, nome, agenda, habilitado, próxima execução)",
		Long: `Lista os jobs registrados em .cosca/cron.db em uma tabela
(id, nome, agenda, habilitado, próxima execução). Apenas leitura.`,
		Example: `  cosca cron list
  cosca cron list --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			store, err := resolveCronStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			jobs, err := store.List()
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, jobs)
			}
			if len(jobs) == 0 {
				formatter.Warning("Nenhum job cron — rode \"cosca cron add --name <nome> --schedule <agenda> --cmd <comando>\".")
				return nil
			}

			rows := make([][]string, 0, len(jobs))
			for _, j := range jobs {
				rows = append(rows, []string{
					j.ID,
					j.Name,
					j.Schedule.Human(),
					boolLabel(j.Enabled),
					formatNextRun(j.NextRun),
				})
			}
			formatter.Header(fmt.Sprintf("Jobs cron — %d job(s)", len(jobs)))
			formatter.Table([]string{"ID", "Nome", "Agenda", "Habilitado", "Próxima execução"}, rows)
			return nil
		},
	}
}

// NewCronRemoveCommand cria `cosca cron remove <id>`.
func NewCronRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove o job cron (C-XXXX)",
		Long: `Remove definitivamente o job do .cosca/cron.db. Use com cuidado —
a remoção é irreversível (o job deixa de ser executado).`,
		Example: `  cosca cron remove C-0001`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)

			store, err := resolveCronStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			norm, err := scheduler.NormalizeID(args[0])
			if err != nil {
				return err
			}
			if err := store.Remove(norm); err != nil {
				return err
			}
			formatter.Success(fmt.Sprintf("Job %s removido de .cosca/cron.db", norm))
			return nil
		},
	}
}

// NewCronRunCommand cria `cosca cron run <id>`.
func NewCronRunCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "run <id>",
		Short: "Executa o job UMA vez imediatamente (o Don inicia)",
		Long: `Executa o comando do job uma única vez agora, via sh -c, com timeout
de 5 minutos (DefaultTimeout). É a forma explícita do Don iniciar a execução —
não depende do daemon. Registra LastRun/Runs e recalcula a próxima ocorrência
no .cosca/cron.db.`,
		Example: `  cosca cron run C-0001`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			store, err := resolveCronStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			norm, err := scheduler.NormalizeID(args[0])
			if err != nil {
				return err
			}
			job, err := store.Get(norm)
			if err != nil {
				return err
			}
			if job == nil {
				return fmt.Errorf("job %s não encontrado em .cosca/cron.db", norm)
			}

			now := time.Now()
			out, runErr := scheduler.RunCommand(cmd.Context(), job.Command, scheduler.DefaultTimeout)

			next, nextErr := job.Schedule.NextAfter(now)
			if nextErr != nil {
				next = now.Add(scheduler.DefaultInterval)
			}
			_ = store.RecordRun(job.ID, now, next, job.Runs+1)

			if useJSON {
				type runResult struct {
					ID      string    `json:"id"`
					Command string    `json:"command"`
					RanAt   time.Time `json:"ran_at"`
					Output  string    `json:"output"`
					Err     string    `json:"error,omitempty"`
				}
				res := runResult{ID: job.ID, Command: job.Command, RanAt: now, Output: out}
				if runErr != nil {
					res.Err = runErr.Error()
				}
				return printJSON(cmd, res)
			}

			formatter.Header(fmt.Sprintf("JOB %s — %q", job.ID, job.Name))
			formatter.KeyValue("Comando", job.Command)
			if out != "" {
				formatter.KeyValue("Saída", out)
			}
			if runErr != nil {
				formatter.Error(fmt.Sprintf("job falhou: %v", runErr))
				return runErr
			}
			formatter.Success(fmt.Sprintf("Job %s executado com sucesso", job.ID))
			return nil
		},
	}
}

// NewCronDaemonCommand cria `cosca cron daemon`.
func NewCronDaemonCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Roda o runner em foreground até Ctrl+C — a ÚNICA porta de execução automática",
		Long: `Inicia o runner do cron em foreground: a cada 30s ele processa os jobs
habilitados cujas próxima execução já venceu. Bloqueia até Ctrl+C.

REGRA DO DON: nada roda sozinho. Nada é executado automaticamente a menos
que (a) este daemon esteja rodando E (b) o job tenha sido adicionado e
habilitado pelo Don. Sem este comando, "cosca cron run <id>" é a única forma
de executar um job.`,
		Example: `  cosca cron daemon`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)

			store, err := resolveCronStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			runner := scheduler.NewRunner(store, log.Logger)

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			runner.Start(ctx)
			defer runner.Stop()

			formatter.Println("cron daemon rodando (tick 30s) — press Ctrl+C to stop")
			formatter.Warning("NADA roda automaticamente a menos que este daemon esteja rodando E o job tenha sido adicionado pelo Don.")
			formatter.KeyValue("Cron db", store.DBPath())

			<-ctx.Done()
			formatter.Println("cron daemon parado")
			return nil
		},
	}
}

// formatNextRun formata o NextRun para tabelas ("" para zero).
func formatNextRun(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("2006-01-02 15:04")
}

// boolLabel traduz bool para a tabela.
func boolLabel(b bool) string {
	if b {
		return "sim"
	}
	return "não"
}
