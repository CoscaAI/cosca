package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/aitask"
)

// NewTaskCommand creates the `cosca task` command group — the AI Task Engine
// (§4 do manifesto Creative/Scientific/Media, Fase 1 etapa 1.3).
func NewTaskCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "AI Task Engine — 18 tarefas canônicas (§4)",
		Long: `AI Task Engine — o catálogo canônico das 18 tarefas de IA do manifesto §4.

O Task Engine NORMALIZA as tarefas (text_generation, image_generation,
speech_to_text, segmentation, upscale, ...) com metadados de modalidade,
hardware e executores candidatos. É a espinha dorsal sobre a qual o Model
Registry e o Node Graph plugam executores — a execução real acontece nos
pipelines (cosca-media, providers), não aqui.

Subcommands:
  list         List all 18 canonical tasks
  info <type>  Show a task's contract (input/output/hardware/executors)`,
		Example: `  cosca task list
  cosca task list --json
  cosca task info speech_to_text`,
	}
	cmd.AddCommand(
		NewTaskListCommand(),
		NewTaskInfoCommand(),
	)
	return cmd
}

// NewTaskListCommand creates `cosca task list`.
func NewTaskListCommand() *cobra.Command {
	var byHardware string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the 18 canonical AI tasks",
		Example: `  cosca task list
  cosca task list --hardware gpu
  cosca task list --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			types := aitask.SortedTypes()
			var tasks []aitask.Task
			for _, t := range types {
				task := aitask.MustGet(t)
				if byHardware != "" && string(task.Hardware) != byHardware {
					continue
				}
				tasks = append(tasks, task)
			}

			if useJSON {
				return printJSON(cmd, tasks)
			}

			if len(tasks) == 0 {
				formatter.Warning(fmt.Sprintf("No tasks for hardware %q (valid: cpu, gpu, any, remote)", byHardware))
				return nil
			}

			formatter.Header(fmt.Sprintf("AI Tasks (%d of %d)", len(tasks), len(aitask.AllTypes)))
			formatter.Println("")

			rows := make([][]string, 0, len(tasks))
			for _, task := range tasks {
				rows = append(rows, []string{
					string(task.Type),
					string(task.Input),
					string(task.Output),
					string(task.Hardware),
				})
			}
			formatter.Table([]string{"Task", "Input", "Output", "Hardware"}, rows)
			return nil
		},
	}

	cmd.Flags().StringVar(&byHardware, "hardware", "", "Filter by hardware class: cpu, gpu, any, remote")
	return cmd
}

// NewTaskInfoCommand creates `cosca task info <type>`.
func NewTaskInfoCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info <type>",
		Short: "Show a task's contract",
		Example: `  cosca task info speech_to_text
  cosca task info segmentation`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			typ := aitask.Type(args[0])
			task, ok := aitask.Get(typ)
			if !ok {
				return fmt.Errorf("unknown task %q (valid: %s)", args[0], aitask.TypesList())
			}

			if useJSON {
				return printJSON(cmd, task)
			}

			formatter.Header(fmt.Sprintf("Task %s", task.Type))
			formatter.KeyValue("Description", task.Description)
			formatter.KeyValue("Input", string(task.Input))
			formatter.KeyValue("Output", string(task.Output))
			formatter.KeyValue("Hardware", string(task.Hardware))
			formatter.KeyValue("Requires Model", fmt.Sprint(task.RequiresModel))
			if len(task.Executors) > 0 {
				formatter.Println("")
				formatter.Println("  Executors:")
				for _, e := range task.Executors {
					note := ""
					if e.Notes != "" {
						note = " — " + e.Notes
					}
					formatter.Bullet(fmt.Sprintf("%s [%s]%s", e.Name, e.Kind, note))
				}
			}
			return nil
		},
	}
	return cmd
}
