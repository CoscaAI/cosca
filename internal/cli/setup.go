// Package cli — `cosca setup`: o COSCA Environment Provisioner.
//
// O maestro que ORQUESTRA as capabilities já existentes do COSCA (machine,
// doctor, health, knowledge, embeddings, index) num ciclo de provisionamento
// idempotente, com Installation State e trilha de evidência (CHECK/ACTION/
// RESULT/EVIDENCE/STATE — o conceito do professor).
//
// Cada etapa segue: Detectar → decidir → executar → validar → registrar.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/installer"
)

// NewSetupCommand cria `cosca setup` — o provisioner.
func NewSetupCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "COSCA Environment Provisioner — instala/verifica a máquina para o COSCA",
		Long: `COSCA Environment Provisioner: leva a máquina de NOT_READY até
COSCA_READY orquestrando as capabilities já existentes (machine, doctor,
health, knowledge, embeddings, index).

Idempotente: persiste o Installation State e retoma de onde parou (fechou no
meio → detecta → resume). Cada etapa produz evidência (CHECK/ACTION/RESULT/
EVIDENCE/STATE) em .cosca/install/installation.json.`,
		Example: `  cosca setup                 # provisiona (retoma do estado atual)
  cosca setup --status        # mostra o installation state atual`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f := GetFormatter(cmd)
			dir, dirErr := resolveDataDir("")
			if dirErr != nil {
				return fmt.Errorf("resolve data directory: %w", dirErr)
			}

			f.Header("COSCA Environment Provisioner")
			f.KeyValue("Estado atual", string(installer.LoadState(dir)))

			phases := provisionPhases()
			rep, err := installer.Run(dir, "v1.5.0", phases)
			if err != nil {
				f.Error(fmt.Sprintf("Provisionamento parou: %v", err))
				return err
			}

			// Relatório: estado + resumo das etapas.
			f.KeyValue("Estado final", string(rep.CurrentState))
			f.KeyValue("Certificado", fmt.Sprintf("%v", rep.Certified))
			f.Header("Etapas")
			for _, s := range rep.Steps {
				mark := "✅"
				if s.Result != installer.ResultPass {
					mark = "⚠️"
				}
				f.Printf("  %s [%s] %-30s %s (%s)\n", mark, s.State, s.Check, s.Result, s.Action)
			}
			if rep.Certified {
				f.Success("\nCOSCA CERTIFIED — READY!")
			}
			return nil
		},
	}
	cmd.AddCommand(newSetupStatusCommand())
	return cmd
}

// newSetupStatusCommand mostra o installation state atual (read-only).
func newSetupStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Mostra o installation state atual",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f := GetFormatter(cmd)
			dir, dirErr := resolveDataDir("")
			if dirErr != nil {
				return fmt.Errorf("resolve data directory: %w", dirErr)
			}
			current := installer.LoadState(dir)
			f.Header("Installation State")
			f.KeyValue("Estado", string(current))
			for _, s := range installer.Order {
				mark := "○"
				if s == current {
					mark = "◉"
				}
				f.Printf("  %s %s\n", mark, s)
			}
			return nil
		},
	}
}

// provisionPhases monta as fases do provisionamento. As Fases 0-1 usam as
// capabilities reais (hardware/machine para preflight; detecção de
// git/go/ollama para dependencies). As fases seguintes são preenchidas
// incrementalmente (auth, cosca, ai, knowledge, index, runtime, certify).
func provisionPhases() []installer.Phase {
	return []installer.Phase{
		{
			ID:   "preflight",
			Name: "Preflight (machine profile)",
			Checks: []installer.Check{
				&machineProfileCheck{},
			},
			NextState: installer.StatePreflightOK,
		},
		{
			ID:   "dependencies",
			Name: "Dependencies",
			Checks: []installer.Check{
				&toolCheck{id: "git.installed", name: "Git", cmd: "git"},
				&toolCheck{id: "go.installed", name: "Go", cmd: "go"},
				&toolCheck{id: "ollama.installed", name: "Ollama", cmd: "ollama"},
			},
			NextState: installer.StateDepsReady,
		},
	}
}
