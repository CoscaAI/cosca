//
// `cosca machine` — Capability Profile versionado da máquina.
//
// O Machine Profile congela as capacidades de inferência da máquina
// (.cosca/machine/profile.json) com um hash SHA-256 estável. Se o hardware
// mudar (ex.: trocar a GPU), `cosca machine profile`/`diff` detectam
// OLD PROFILE → hardware diff → NEW PROFILE e avisam que o backend de
// inferência precisa ser reavaliado — evitando reinstalar tudo.
//
// Regra do Don: `cosca machine profile` só grava quando o diff de capacidade
// detecta mudança real (ou quando não existe perfil salvo).
//
// Subcomandos:
//   probe    Imprime a tabela de capacidades (cpu/mem/gpu/capabilities + hash)
//   profile  Salva o perfil atual em .cosca/machine/profile.json (com diff)
//   diff     Compara o perfil salvo com a capacidade atual
//

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/compute"
)

// NewMachineCommand cria a árvore de comandos `cosca machine`.
func NewMachineCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "machine",
		Short: "Capability Profile versionado da máquina (hardware diff)",
		Long: `Capability Profile versionado da máquina.

Congela as capacidades de inferência da máquina (CPU, memória, GPU e
capabilities) em .cosca/machine/profile.json com um hash SHA-256 estável.
Quando o hardware muda (ex.: troca de GPU), o diff OLD PROFILE → NEW PROFILE
é detectado e o backend de inferência precisa ser reavaliado — sem reinstalar
tudo.

Subcomandos:
  probe    Imprime a tabela de capacidades da máquina
  profile  Salva o perfil atual (grava apenas se a capacidade mudou)
  diff     Compara o perfil salvo com a capacidade atual`,
		Example: `  cosca machine probe
  cosca machine profile
  cosca machine diff`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				return fmt.Errorf("unknown subcommand: %s (valid: probe, profile, diff)", args[0])
			}
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		NewMachineProbeCommand(),
		NewMachineProfileCommand(),
		NewMachineDiffCommand(),
		NewMachineCapabilityCommand(),
	)
	return cmd
}

// NewMachineProbeCommand cria `cosca machine probe`.
func NewMachineProbeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "probe",
		Short: "Imprimir a tabela de capacidades da máquina",
		Long: `Executa o probe de hardware (ProbeHardware) e deriva o perfil de
capacidade (BuildCapabilityProfile), imprimindo CPU, memória, GPU,
capabilities e o hash SHA-256 estável. Não grava nada.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			profile := compute.BuildCapabilityProfile(compute.ProbeHardware())

			if IsJSONOutput(cmd) {
				return printJSON(cmd, profile)
			}
			printCapabilityProfile(formatter, profile)
			return nil
		},
	}
}

// NewMachineProfileCommand cria `cosca machine profile`.
func NewMachineProfileCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "profile",
		Short: "Salvar o perfil de capacidade atual em .cosca/machine/profile.json",
		Long: `Probeia a capacidade atual e compara com o perfil salvo
(.cosca/machine/profile.json). Se não houver mudança, imprime "perfil
inalterado". Se mudou (ou não existe perfil), grava o novo perfil de forma
atômica e imprime o diff de capacidade + o aviso de reavaliação do backend de
inferência.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			store, err := machineStoreFromCWD()
			if err != nil {
				return err
			}
			profile := compute.BuildCapabilityProfile(compute.ProbeHardware())

			existing, err := store.Load()
			if err != nil {
				return err
			}
			if existing == nil {
				if err := store.Save(profile); err != nil {
					return err
				}
				if IsJSONOutput(cmd) {
					return printJSON(cmd, profile)
				}
				formatter.Success(fmt.Sprintf("Perfil de capacidade salvo — hash %s", shortHash(profile.Hash)))
				printCapabilityProfile(formatter, profile)
				return nil
			}

			changed, summary := compute.DiffProfiles(*existing, profile)
			if !changed {
				if IsJSONOutput(cmd) {
					return printJSON(cmd, machineStatusReport{Status: "unchanged", Hash: profile.Hash})
				}
				formatter.Success(fmt.Sprintf("Perfil inalterado ✓ — hash %s", shortHash(profile.Hash)))
				return nil
			}

			if err := store.Save(profile); err != nil {
				return err
			}
			if IsJSONOutput(cmd) {
				return printJSON(cmd, machineStatusReport{Status: "changed", Diff: summary, Hash: profile.Hash})
			}
			printMachineDiff(formatter, summary)
			formatter.Warning(compute.CapabilityChangedWarning)
			formatter.Success(fmt.Sprintf("Perfil atualizado — hash %s", shortHash(profile.Hash)))
			return nil
		},
	}
}

// NewMachineDiffCommand cria `cosca machine diff`.
func NewMachineDiffCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "diff",
		Short: "Comparar o perfil salvo com a capacidade atual",
		Long: `Carrega o perfil salvo (.cosca/machine/profile.json), probeia a
capacidade atual e imprime o diff OLD PROFILE → NEW PROFILE. Não grava nada.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			store, err := machineStoreFromCWD()
			if err != nil {
				return err
			}
			existing, err := store.Load()
			if err != nil {
				return err
			}
			if existing == nil {
				return fmt.Errorf("nenhum perfil salvo — rode \"cosca machine profile\" primeiro")
			}

			current := compute.BuildCapabilityProfile(compute.ProbeHardware())
			changed, summary := compute.DiffProfiles(*existing, current)

			if IsJSONOutput(cmd) {
				return printJSON(cmd, machineDiffReport{
					Changed: changed,
					Diff:    summary,
					OldHash: existing.Hash,
					NewHash: current.Hash,
				})
			}
			if !changed {
				formatter.Success("Sem mudanças — capacidade da máquina inalterada.")
				return nil
			}
			formatter.Header("Diff de capacidade")
			for _, line := range summary {
				formatter.Bullet(line)
			}
			return nil
		},
	}
}

// machineStatusReport é o relatório JSON de `cosca machine profile`.
type machineStatusReport struct {
	Status string   `json:"status"`
	Hash   string   `json:"hash"`
	Diff   []string `json:"diff,omitempty"`
}

// machineDiffReport é o relatório JSON de `cosca machine diff`.
type machineDiffReport struct {
	Changed bool     `json:"changed"`
	Diff    []string `json:"diff,omitempty"`
	OldHash string   `json:"old_hash"`
	NewHash string   `json:"new_hash"`
}

// printCapabilityProfile imprime a tabela de capacidades (cpu/mem/gpu/
// capabilities + hash) no formatter de texto.
func printCapabilityProfile(formatter *OutputFormatter, p compute.CapabilityProfile) {
	formatter.Header("CPU")
	formatter.KeyValue("Cores", strconv.Itoa(p.CPU.Cores))
	formatter.KeyValue("Threads", strconv.Itoa(p.CPU.Threads))

	formatter.Header("Memória")
	formatter.KeyValue("Total", fmt.Sprintf("%.1f GB", p.Memory.TotalGB))
	formatter.KeyValue("Disponível", fmt.Sprintf("%.1f GB", p.Memory.AvailableGB))

	formatter.Header("GPU")
	formatter.KeyValue("Vendor", string(p.GPU.Vendor))
	if p.GPU.Model != "" {
		formatter.KeyValue("Model", p.GPU.Model)
	}
	if p.GPU.VRAMGB > 0 {
		formatter.KeyValue("VRAM", fmt.Sprintf("%d GB", p.GPU.VRAMGB))
	}
	if p.GPU.Driver != "" {
		formatter.KeyValue("Driver", p.GPU.Driver)
	}
	formatter.KeyValue("ROCm", yesNo(p.GPU.HasROCm))
	formatter.KeyValue("Vulkan", yesNo(p.GPU.HasVulkan))
	formatter.KeyValue("CUDA", yesNo(p.GPU.HasCUDA))
	if p.GPU.Compute != "" {
		formatter.KeyValue("Compute", p.GPU.Compute)
	}

	formatter.Header("Capabilities")
	formatter.KeyValue("Lista", strings.Join(p.Capabilities, ", "))
	formatter.KeyValue("Hash", p.Hash)
}

// printMachineDiff imprime as linhas do diff de capacidade. A linha de aviso
// (CapabilityChangedWarning) é impressa como warning; as demais como bullets.
func printMachineDiff(formatter *OutputFormatter, summary []string) {
	formatter.Header("Diff de capacidade")
	for _, line := range summary {
		if line == compute.CapabilityChangedWarning {
			formatter.Warning(line)
			continue
		}
		formatter.Bullet(line)
	}
}

// machineStoreFromCWD resolve o MachineStore para o .cosca do diretório atual.
func machineStoreFromCWD() (*compute.MachineStore, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("falha ao obter o diretório de trabalho: %w", err)
	}
	return compute.NewMachineStore(filepath.Join(cwd, ".cosca")), nil
}

// shortHash abrevia um SHA-256 hex para os primeiros 8 caracteres.
func shortHash(hash string) string {
	if len(hash) > 8 {
		return hash[:8]
	}
	return hash
}

// NewMachineCapabilityCommand cria o `cosca machine capability` — o Capability
// Model da F7 (a estrutura central, L319): a matriz de capabilities com
// proveniência (status/source/evidence/confidence/environment/timestamp).
func NewMachineCapabilityCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "capability",
		Short: "Capability Model (F7) — matriz de capacidades com proveniência",
		Long: `Capability Model (F7 do Hardware Brain).

Deriva a matriz de capabilities do snapshot de hardware (F1-F6), onde CADA
capability carrega: status (reported/visible/usable/measured), source,
evidence, confidence, environment e timestamp. Nada é inventado — tudo deriva
do que os probes observaram (L319/L320).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			model := compute.BuildCapabilityModel(compute.ProbeHardware())

			if IsJSONOutput(cmd) {
				return printJSON(cmd, model)
			}

			formatter.Header("Capability Model (F7)")
			formatter.KeyValue("Environment", model.Environment)
			formatter.KeyValue("Hash", shortHash(model.Hash))
			if model.HardwareFingerprint != "" {
				formatter.KeyValue("Hardware", model.HardwareFingerprint)
			}

			// Tabela: name | status | r/v/u/m | confidence | evidence
			rows := make([][]string, 0, len(model.Capabilities))
			for _, name := range model.Names() {
				c := model.Capabilities[name]
				rows = append(rows, []string{
					name,
					string(c.Status),
					rvum(c),
					c.Confidence,
					shorten(c.Evidence, 40),
				})
			}
			formatter.Table([]string{"Capability", "Status", "R/V/U/M", "Conf", "Evidence"}, rows)
			return nil
		},
	}
}

// rvum formata a cadeia reported/visible/usable/measured como "1/0/0/0".
func rvum(c compute.Capability) string {
	return b1(c.Reported) + "/" + b1(c.Visible) + "/" + b1(c.Usable) + "/" + b1(c.Measured)
}

func b1(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// shorten é o helper local de abreviação de evidência (o truncate do pacote
// tem outra assinatura — ver context.go).
func shorten(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
