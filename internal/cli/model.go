package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/aitask"
	"github.com/CoscaAI/cosca/internal/modelreg"
)

// NewModelCommand creates the `cosca model` command group — the Model Registry
// (§18 do manifesto Creative/Scientific/Media, Fase 1 etapa 1.4).
//
// Diferente de `cosca models` (catálogo models.dev com context/pricing), o
// `cosca model` registra MODELOS CONCRETOS do projeto: formato, quantização,
// VRAM, capabilities (quais das 18 tasks §4 executa) e licença (§35).
func NewModelCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "model",
		Short: "Model Registry — modelos concretos das 18 tasks (§18)",
		Long: `Model Registry — registra os modelos concretos que o projeto usa.

Cada modelo declara: provider, id, version, formato (safetensors/gguf/onnx/
torch), quantização (fp16/int8/int4), requisito de VRAM, as TASKS do AI Task
Engine que executa (§4/§18) e a licença (§35).

O registry vive em .cosca/models/index.yaml e liga modelos → tarefas: para
qualquer task, o engine sabe qual modelo do projeto a executa.

Subcommands:
  add        Register a model (provider/id/version + capabilities)
  list       List registered models [--task <type>] [--kind local|remote]
  info <key> Show a model (key: provider/id/version, prefix aceito)
  remove     Remove a model by key`,
		Example: `  cosca model add whisper --provider openai --version large-v3 --task speech_to_text
  cosca model add tesseract --provider local --version 5.3 --task ocr --format unknown
  cosca model list
  cosca model list --task segmentation
  cosca model info openai/whisper/large-v3`,
	}
	cmd.AddCommand(
		NewModelAddCommand(),
		NewModelListCommand(),
		NewModelInfoCommand(),
		NewModelRemoveCommand(),
	)
	return cmd
}

// resolveModelRegistry abre o registry do projeto atual (como o asset).
func resolveModelRegistry() (*modelreg.Registry, string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, "", fmt.Errorf("getwd: %w", err)
	}
	root, err := findProjectRoot(wd)
	if err != nil {
		return nil, "", err
	}
	r, err := modelreg.Open(root)
	if err != nil {
		return nil, "", err
	}
	return r, root, nil
}

// NewModelAddCommand creates `cosca model add`.
func NewModelAddCommand() *cobra.Command {
	var provider, version, formatStr, quantStr, kindStr, license, source string
	var vramMB int64
	var tasks []string

	cmd := &cobra.Command{
		Use:   "add <id>",
		Short: "Register a model",
		Long: `Register a model in the Model Registry (.cosca/models/index.yaml).

The key is provider/id/version. Capabilities (--task, repeatable) declare
which of the 18 AI tasks this model executes — the link to the AI Task Engine.`,
		Example: `  cosca model add whisper --provider openai --version large-v3 --task speech_to_text --license MIT
  cosca model add rembg --provider local --version 2.0.78 --task segmentation --format onnx --kind local
  cosca model add tesseract --provider local --version 5.3 --task ocr --format unknown --license Apache-2.0`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			r, root, err := resolveModelRegistry()
			if err != nil {
				return err
			}

			format := modelreg.Format(formatStr)
			if !format.Valid() {
				return fmt.Errorf("invalid format %q (valid: safetensors, gguf, onnx, torch, tensorrt, unknown)", formatStr)
			}
			quant := modelreg.Quantization(quantStr)
			if !quant.Valid() {
				return fmt.Errorf("invalid quantization %q (valid: fp16, fp32, int8, int4, none)", quantStr)
			}
			kind := modelreg.Kind(kindStr)
			if kind != modelreg.KindLocal && kind != modelreg.KindRemote {
				return fmt.Errorf("invalid kind %q (valid: local, remote)", kindStr)
			}
			if len(tasks) == 0 {
				return fmt.Errorf("at least one --task is required (valid: %s)", aitask.TypesList())
			}

			caps := make([]aitask.Type, 0, len(tasks))
			for _, t := range tasks {
				typ := aitask.Type(t)
				if !typ.Valid() {
					return fmt.Errorf("unknown task %q (valid: %s)", t, aitask.TypesList())
				}
				caps = append(caps, typ)
			}

			m, err := modelreg.New(args[0], provider, version, format, quant, vramMB<<20, kind, license, caps)
			if err != nil {
				return err
			}
			if source != "" {
				m.Source = source
			}
			if err := r.Register(m); err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, m)
			}

			formatter.Success(fmt.Sprintf("Model registered: %s", m.Key()))
			formatter.KeyValue("Format", string(m.Format))
			formatter.KeyValue("Quantization", string(m.Quantization))
			formatter.KeyValue("Kind", string(m.Kind))
			formatter.KeyValue("VRAM", formatSize(vramMB<<20))
			formatter.KeyValue("Tasks", tasksString(caps))
			formatter.KeyValue("License", m.License)
			formatter.KeyValue("Registry", filepath.Join(root, modelreg.DefaultDir))
			return nil
		},
	}

	cmd.Flags().StringVar(&provider, "provider", "local", "Model provider (default: local)")
	cmd.Flags().StringVar(&version, "version", "1.0", "Model version")
	cmd.Flags().StringVar(&formatStr, "format", "unknown", "Format: safetensors, gguf, onnx, torch, tensorrt, unknown")
	cmd.Flags().StringVar(&quantStr, "quantization", "none", "Quantization: fp16, fp32, int8, int4, none")
	cmd.Flags().Int64Var(&vramMB, "vram", 0, "VRAM requirement in MB")
	cmd.Flags().StringVar(&kindStr, "kind", "local", "Kind: local (runs on machine) or remote (API)")
	cmd.Flags().StringVar(&license, "license", "", "Model license (§35)")
	cmd.Flags().StringVar(&source, "source", "", "Provenance source (§33/§35)")
	cmd.Flags().StringSliceVar(&tasks, "task", nil, "Task capability (repeatable): "+aitask.TypesList())
	return cmd
}

// NewModelListCommand creates `cosca model list`.
func NewModelListCommand() *cobra.Command {
	var taskFilter, kindFilter string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List registered models",
		Example: `  cosca model list
  cosca model list --task segmentation
  cosca model list --kind local
  cosca model list --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			r, root, err := resolveModelRegistry()
			if err != nil {
				return err
			}

			var models []*modelreg.Model
			if taskFilter != "" {
				typ := aitask.Type(taskFilter)
				if !typ.Valid() {
					return fmt.Errorf("unknown task %q (valid: %s)", taskFilter, aitask.TypesList())
				}
				models = r.FindForTask(typ)
			} else {
				models = r.List()
			}
			if kindFilter != "" {
				var filtered []*modelreg.Model
				for _, m := range models {
					if string(m.Kind) == kindFilter {
						filtered = append(filtered, m)
					}
				}
				models = filtered
			}

			if useJSON {
				return printJSON(cmd, models)
			}

			if len(models) == 0 {
				formatter.Warning("No models registered — add one with 'cosca model add <id> --task <task>'")
				return nil
			}

			formatter.Header(fmt.Sprintf("Models (%d)", len(models)))
			formatter.KeyValue("Registry", filepath.Join(root, modelreg.DefaultDir))
			formatter.Println("")

			rows := make([][]string, 0, len(models))
			for _, m := range models {
				rows = append(rows, []string{
					m.Key(),
					string(m.Format),
					string(m.Quantization),
					string(m.Kind),
					shortTasks(m.Capabilities),
				})
			}
			formatter.Table([]string{"Model", "Format", "Quant", "Kind", "Tasks"}, rows)
			return nil
		},
	}

	cmd.Flags().StringVar(&taskFilter, "task", "", "Filter by task capability")
	cmd.Flags().StringVar(&kindFilter, "kind", "", "Filter by kind: local, remote")
	return cmd
}

// NewModelInfoCommand creates `cosca model info <key>`.
func NewModelInfoCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info <key>",
		Short: "Show a model's details",
		Long: `Show details of a registered model.

The key is provider/id/version. A prefix of the key is accepted when it
uniquely matches one model.`,
		Example: `  cosca model info openai/whisper/large-v3`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			r, _, err := resolveModelRegistry()
			if err != nil {
				return err
			}

			key, err := resolveModelKey(r, args[0])
			if err != nil {
				return err
			}
			m, ok := r.Get(key)
			if !ok {
				return fmt.Errorf("model %q not found", args[0])
			}

			if useJSON {
				return printJSON(cmd, m)
			}

			formatter.Header(m.Key())
			formatter.KeyValue("Provider", m.Provider)
			formatter.KeyValue("ID", m.ID)
			formatter.KeyValue("Version", m.Version)
			formatter.KeyValue("Format", string(m.Format))
			formatter.KeyValue("Quantization", string(m.Quantization))
			formatter.KeyValue("Kind", string(m.Kind))
			if m.VRAM > 0 {
				formatter.KeyValue("VRAM", formatSize(m.VRAM))
			}
			formatter.KeyValue("Tasks", tasksString(m.Capabilities))
			if m.License != "" {
				formatter.KeyValue("License", m.License)
			}
			if m.Source != "" {
				formatter.KeyValue("Source", m.Source)
			}
			formatter.KeyValue("Added", m.AddedAt)
			return nil
		},
	}
	return cmd
}

// NewModelRemoveCommand creates `cosca model remove <key>`.
func NewModelRemoveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove <key>",
		Short:   "Remove a model by key",
		Example: `  cosca model remove openai/whisper/large-v3`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)

			r, _, err := resolveModelRegistry()
			if err != nil {
				return err
			}
			key, err := resolveModelKey(r, args[0])
			if err != nil {
				return err
			}
			if err := r.Remove(key); err != nil {
				return err
			}
			formatter.Success(fmt.Sprintf("Model %s removed", key))
			return nil
		},
	}
	return cmd
}

// resolveModelKey resolve uma chave completa a partir de um prefixo.
func resolveModelKey(r *modelreg.Registry, prefix string) (string, error) {
	// Chave completa exata.
	if _, ok := r.Get(prefix); ok {
		return prefix, nil
	}
	// Prefixo de chave (provider/id/version parcial).
	var matches []string
	for _, m := range r.List() {
		if strings.HasPrefix(m.Key(), prefix) {
			matches = append(matches, m.Key())
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("model %q not found", prefix)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("prefix %q is ambiguous (%d matches)", prefix, len(matches))
	}
}

// tasksString formata a lista de tasks.
func tasksString(caps []aitask.Type) string {
	parts := make([]string, len(caps))
	for i, c := range caps {
		parts[i] = string(c)
	}
	return strings.Join(parts, ", ")
}

// shortTasks formata até 3 tasks para tabela.
func shortTasks(caps []aitask.Type) string {
	if len(caps) <= 3 {
		return tasksString(caps)
	}
	return tasksString(caps[:3]) + fmt.Sprintf(" +%d", len(caps)-3)
}
