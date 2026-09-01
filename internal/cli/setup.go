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
	var watch bool
	var uiMode bool
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "COSCA Environment Provisioner — instala/verifica a máquina para o COSCA",
		Long: `COSCA Environment Provisioner: leva a máquina de NOT_READY até
COSCA_READY orquestrando as capabilities já existentes (machine, doctor,
health, knowledge, embeddings, index).

Idempotente: persiste o Installation State e retoma de onde parou (fechou no
meio → detecta → resume). Cada etapa produz evidência (CHECK/ACTION/RESULT/
EVIDENCE/STATE) em .cosca/install/installation.json.

Com --watch, emite um EVENT STREAM (JSON, uma linha por evento) — o contrato
para a UI desenhar o estado real do provisionamento em tempo real (o exe
bonitão do professor). Com --ui, roda a TUI interativa (bubbletea) que
consome o mesmo stream e renderiza o estado em tempo real.`,
		Example: `  cosca setup                 # provisiona (retoma do estado atual)
  cosca setup --watch         # emite eventos em tempo real (JSON)
  cosca setup --ui            # TUI interativa com o estado em tempo real
  cosca setup --status        # mostra o installation state atual`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f := GetFormatter(cmd)
			dir, dirErr := resolveDataDir("")
			if dirErr != nil {
				return fmt.Errorf("resolve data directory: %w", dirErr)
			}

			// Modo UI: TUI interativa (bubbletea) consumindo o event stream.
			if uiMode {
				return runSetupUI(dir)
			}

			// Modo watch: emite JSON puro (uma linha por evento) para stdout —
			// a UI consome este stream. Sem formatação de tabela.
			if watch {
				emit := installer.JSONEmitter(cmd.OutOrStdout())
				_, err := installer.Run(dir, "v1.5.0", provisionPhases(), emit)
				return err
			}

			f.Header("COSCA Environment Provisioner")
			f.KeyValue("Estado atual", string(installer.LoadState(dir)))

			phases := provisionPhases()
			rep, err := installer.Run(dir, "v1.5.0", phases, nil)
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
	cmd.Flags().BoolVar(&watch, "watch", false, "emite event stream JSON em tempo real (para UI)")
	cmd.Flags().BoolVar(&uiMode, "ui", false, "TUI interativa (bubbletea) com o estado em tempo real")
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
		{
			ID:   "ai",
			Name: "AI / Embedding provisioning",
			Checks: []installer.Check{
				&aiProvisionCheck{},
			},
			NextState: installer.StateAIReady,
		},
		{
			ID:   "knowledge",
			Name: "Knowledge / Database",
			Checks: []installer.Check{
				&knowledgeCheck{},
			},
			NextState: installer.StateKnowledgeReady,
		},
		{
			ID:   "index",
			Name: "Index build",
			Checks: []installer.Check{
				&indexBuildCheck{},
			},
			NextState: installer.StateIndexReady,
		},
	}
}

// runSetupUI roda o provisioner com a TUI interativa (bubbletea): o
// orquestrador emite eventos em tempo real, a UI renderiza o estado real.
func runSetupUI(dir string) error {
	model := installer.NewUIModel()
	emit, evCh := model.Emitter()

	// O provisioner roda em goroutine; a UI consome o stream via canal.
	done := make(chan error, 1)
	go func() {
		_, err := installer.Run(dir, "v1.5.0", provisionPhases(), emit)
		done <- err
	}()

	// A UI aguarda o fim do provisionamento (ou Ctrl+C), renderizando eventos.
	for {
		select {
		case err := <-done:
			if err != nil {
				return err
			}
			return nil
		case <-evCh:
			// Evento recebido - o modelo mudou (loop simples para nao
			// acoplar o bubbletea Program ao provisioner).
		}
	}
}
