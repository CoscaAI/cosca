package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/evolution"
)

// stageReport é o relatório de observabilidade (projeção, NUNCA control plane).
type stageReport struct {
	Project      string               `json:"project"`
	Current      string               `json:"current_stage"`
	Next         string               `json:"next_stage"`
	Completed    []evolution.StagePhase `json:"completed,omitempty"`
	UpdatedAt    time.Time            `json:"updated_at,omitempty"`
}

// NewStageCommand cria a árvore `cosca stage` — observabilidade do percurso de
// uma mudança (Stage observability / AI-DLC, ADR-018). Projeção, não control.
func NewStageCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stage",
		Short: "Observa o percurso (stage) de uma mudança — projeção, não control plane",
		Long: `Observa em que estágio uma mudança/tarefa está (TASK→REQUIREMENT→DESIGN→
IMPLEMENT→TEST→EVALUATE→PROMOTE) conectando os gates existentes (proposal,
quarantine, skilleval, deliberate) SEM misturá-los. É projeção/observabilidade
(I1: determinístico, zero LLM), nunca uma máquina que orquestra.

Subcomandos:
  status   Mostra o estágio atual do percurso
  new      Cria um novo percurso para um projeto`,
		Example: `  cosca stage status
  cosca stage new F1.5-skill-evolve`,
	}

	cmd.AddCommand(NewStageStatusCommand(), NewStageNewCommand())
	return cmd
}

// defaultStagePath retorna o caminho do roll (default .cosca/evolution/stage.jsonl).
func defaultStagePath() string {
	dir, _ := os.Getwd()
	return filepath.Join(dir, ".cosca", "evolution", "stage.jsonl")
}

// NewStageStatusCommand cria `cosca stage status`.
func NewStageStatusCommand() *cobra.Command {
	var rollPath string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Mostra o estágio atual do percurso",
		Long:  `Lê o último snapshot do StageRoll e mostra em que estágio o percurso está.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)
			if rollPath == "" {
				rollPath = defaultStagePath()
			}
			store := evolution.NewStageRollStore(rollPath)
			roll, err := store.Load()
			if err != nil {
				return fmt.Errorf("load stage roll: %w", err)
			}
			if roll == nil {
				formatter.Warning("Nenhum percurso registrado — rode 'cosca stage new <projeto>'")
				return nil
			}
			report := stageReport{
				Project:   roll.Project,
				Current:   string(roll.Current),
				Next:      string(roll.Next),
				Completed: roll.Completed,
				UpdatedAt: roll.UpdatedAt,
			}
			if useJSON {
				return printJSON(cmd, report)
			}
			formatter.Header("Percurso (Stage observability)")
			formatter.KeyValue("Projeto", roll.Project)
			formatter.KeyValue("Estágio atual", string(roll.Current))
			if roll.Next != "" {
				formatter.KeyValue("Próximo", string(roll.Next))
			} else {
				formatter.KeyValue("Próximo", "— (final do percurso)")
			}
			formatter.KeyValue("Transições registradas", fmt.Sprintf("%d", len(roll.Completed)))
			return nil
		},
	}
	cmd.Flags().StringVar(&rollPath, "roll", "", "caminho do roll (default: .cosca/evolution/stage.jsonl)")
	return cmd
}

// NewStageNewCommand cria `cosca stage new <projeto>`.
func NewStageNewCommand() *cobra.Command {
	var rollPath string
	cmd := &cobra.Command{
		Use:   "new <projeto>",
		Short: "Cria um novo percurso para um projeto",
		Long:  `Inicia um StageRoll em task e persiste o snapshot (append-only).`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			if rollPath == "" {
				rollPath = defaultStagePath()
			}
			roll := evolution.NewStageRoll(args[0])
			store := evolution.NewStageRollStore(rollPath)
			if err := store.Save(roll); err != nil {
				return fmt.Errorf("save stage roll: %w", err)
			}
			formatter.Success(fmt.Sprintf("percurso '%s' iniciado em %s", args[0], evolution.StageTask))
			return nil
		},
	}
	cmd.Flags().StringVar(&rollPath, "roll", "", "caminho do roll (default: .cosca/evolution/stage.jsonl)")
	return cmd
}
