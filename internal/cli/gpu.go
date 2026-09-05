package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/aitask"
	"github.com/CoscaAI/cosca/internal/compute"
	"github.com/CoscaAI/cosca/internal/sched"
)

// NewGPUCommand creates the `cosca gpu` command group — GPU Engine (§17) e
// Scheduler de execução (§28), Fase 1 etapa 1.5.
func NewGPUCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gpu",
		Short: "GPU Engine — probe hardware + scheduler de execução (§17/§28)",
		Long: `GPU Engine — detecta a capacidade de computação da máquina e decide
onde cada task roda (local CPU, local GPU, remoto).

O probe é READ-ONLY (sem sudo, sem instalar driver) e reporta: vendor, modelo,
VRAM, driver, API surface (ROCm/Vulkan/VAAPI/OpenCL/CUDA), compute target
(gfx1030, sm_80) e CUs. O scheduler cruza a task com o hardware e decide o
destino (§28: performance/custo/disponibilidade/qualidade).

Subcommands:
  probe   Print the full GPU capability table
  plan    Decide where a task runs (local-cpu, local-gpu, remote)`,
		Example: `  cosca gpu probe
  cosca gpu probe --json
  cosca gpu plan segmentation
  cosca gpu plan speech_to_text --vram 8192`,
	}
	cmd.AddCommand(
		NewGPUProbeCommand(),
		NewGPUPlanCommand(),
	)
	return cmd
}

// NewGPUProbeCommand creates `cosca gpu probe`.
func NewGPUProbeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "probe",
		Short: "Print the GPU capability table (§17)",
		Example: `  cosca gpu probe
  cosca gpu probe --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			info := compute.ProbeGPU()
			if useJSON {
				return printJSON(cmd, info)
			}

			formatter.Header("GPU Probe")
			formatter.KeyValue("Vendor", string(info.Vendor))
			if info.Model != "" {
				formatter.KeyValue("Model", info.Model)
			}
			if info.VRAMGB > 0 {
				formatter.KeyValue("VRAM", fmt.Sprintf("%d GiB", info.VRAMGB))
			}
			if info.Driver != "" {
				formatter.KeyValue("Driver", info.Driver)
			}
			if info.Compute != "" {
				formatter.KeyValue("Compute Target", info.Compute)
			}
			if info.ROCmVersion != "" {
				formatter.KeyValue("ROCm Version", info.ROCmVersion)
			}
			if info.ComputeUnits > 0 {
				formatter.KeyValue("Compute Units", fmt.Sprint(info.ComputeUnits))
			}
			if info.MaxClockMHz > 0 {
				formatter.KeyValue("Max Clock", fmt.Sprintf("%d MHz", info.MaxClockMHz))
			}
			formatter.Println("")
			formatter.KeyValue("ROCm", yesNo(info.HasROCm))
			formatter.KeyValue("Vulkan", yesNo(info.HasVulkan))
			formatter.KeyValue("VA-API (vídeo)", yesNo(info.HasVAAPI))
			formatter.KeyValue("OpenCL", yesNo(info.HasOpenCL))
			formatter.KeyValue("CUDA", yesNo(info.HasCUDA))
			return nil
		},
	}
	return cmd
}

// NewGPUPlanCommand creates `cosca gpu plan <task>`.
func NewGPUPlanCommand() *cobra.Command {
	var vramMB int64

	cmd := &cobra.Command{
		Use:   "plan <task>",
		Short: "Decide where a task runs (§28)",
		Long: `Decide where an AI task runs: local-cpu, local-gpu or remote.

The scheduler crosses the task's hardware class (§4) with the real GPU probe:
tasks that want GPU run locally when the GPU (ROCm/Vulkan/VAAPI) is present
and has enough VRAM; otherwise they fall back to CPU or remote (§28).

Pass --vram to simulate a model VRAM requirement from the Model Registry.`,
		Example: `  cosca gpu plan segmentation
  cosca gpu plan speech_to_text
  cosca gpu plan image_generation --vram 20480`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			typ := aitask.Type(args[0])
			if !typ.Valid() {
				return fmt.Errorf("unknown task %q (valid: %s)", args[0], aitask.TypesList())
			}

			s := sched.New()
			d := s.Decide(typ, vramMB<<20)

			if useJSON {
				out := map[string]any{
					"task":   args[0],
					"target": d.Target,
					"reason": d.Reason,
					"gpu":    d.GPU,
				}
				b, _ := json.MarshalIndent(out, "", "  ")
				formatter.Println(string(b))
				return nil
			}

			formatter.Header(fmt.Sprintf("Scheduler — %s", args[0]))
			formatter.KeyValue("Target", string(d.Target))
			formatter.KeyValue("Reason", string(d.Reason))
			if d.GPU.Vendor != compute.GPUNone {
				formatter.KeyValue("GPU", fmt.Sprintf("%s (%s, %d GiB)", d.GPU.Model, d.GPU.Compute, d.GPU.VRAMGB))
			} else {
				formatter.KeyValue("GPU", "none")
			}
			if vramMB > 0 {
				formatter.KeyValue("VRAM Required", fmt.Sprintf("%d MiB", vramMB))
			}
			return nil
		},
	}

	cmd.Flags().Int64Var(&vramMB, "vram", 0, "Model VRAM requirement in MB (from Model Registry)")
	return cmd
}
