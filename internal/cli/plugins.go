package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/pipeline"
)

// NewPluginsCommand creates the `cosca plugins` command.
func NewPluginsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugins",
		Short: "Manage pipeline plugins",
		Long: `List, enable, and inspect pipeline plugins.

Plugins hook into the pipeline at specific ExtensionPoints:
  step.pre_execute   — before a step runs
  step.post_execute  — after a step completes (build, test, lint)
  step.on_failure    — when a step fails
  pipeline.start     — pipeline begins
  pipeline.end       — pipeline ends

Built-in plugins:
  build      go build after each step
  test       go test after each step
  lint       go vet after each step
  checkpoint auto-save progress after each step`,
	}
	cmd.AddCommand(newPluginsListCommand())
	return cmd
}

func newPluginsListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List registered pipeline plugins",
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)

			// Create a local registry with built-in plugins for display
			reg := pipeline.NewPluginRegistry()
			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")
			historyDir := filepath.Join(coscaDir, "history")
			cpDir := filepath.Join(historyDir, "checkpoints")

			var cp *pipeline.CheckpointStore
			if s, err := pipeline.NewCheckpointStore(cpDir); err == nil {
				cp = s
			}
			pipeline.RegisterBuiltinPlugins(reg, cp)

			names := reg.List()
			if len(names) == 0 {
				formatter.Warning("Nenhum plugin registrado.")
				return nil
			}

			formatter.Header(fmt.Sprintf("Pipeline Plugins (%d)", len(names)))

			headers := []string{"Plugin", "Priority", "Extension Points"}
			rows := make([][]string, 0, len(names))

			// Build a map of extension points per plugin name
			extPoints := map[ExtensionPoint]string{
				pipeline.ExtStepPreExecute:  "step.pre",
				pipeline.ExtStepPostExecute: "step.post",
				pipeline.ExtStepOnFailure:   "step.fail",
				pipeline.ExtPipelineStart:   "pipeline.start",
				pipeline.ExtPipelineEnd:     "pipeline.end",
			}

			for _, name := range names {
				pts := []string{}
				for ext, label := range extPoints {
					for _, p := range reg.PluginsFor(ext) {
						if p == name {
							pts = append(pts, label)
						}
					}
				}
				sort.Strings(pts)

				// Priority is hardcoded for built-ins
				prio := "—"
				switch name {
				case "build":
					prio = "10"
				case "test":
					prio = "20"
				case "lint":
					prio = "30"
				case "checkpoint":
					prio = "100"
				}

				rows = append(rows, []string{name, prio, strings.Join(pts, ", ")})
			}

			formatter.Table(headers, rows)
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintln(cmd.OutOrStdout(), "  ExtensionPoints disponiveis:")
			fmt.Fprintln(cmd.OutOrStdout(), "    step.pre_execute, step.post_execute, step.on_failure")
			fmt.Fprintln(cmd.OutOrStdout(), "    pipeline.start, pipeline.end")
			return nil
		},
	}
}

// ExtensionPoint is a local type alias for display.
type ExtensionPoint = pipeline.ExtensionPoint
