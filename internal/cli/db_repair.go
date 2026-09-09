// `cosca db repair` — Repair assistido de estado para bancos DERIVADOS e
// regeneráveis (ADR-043 §8, pacote 1 — Camada C do ADR-014 para a classe
// derivado/estado).
//
// Decisão de nomenclatura: o ADR-043 nomeia `cosca state repair`, mas este
// comando é registrado como `cosca db repair` para REUTILIZAR o parent `db`
// (governança existente de bancos em internal/cli/db.go). A função é idêntica
// à especificada; apenas o caminho de comando muda.
//
// Modos (mutuamente exclusivos):
//
//	cosca db repair --db session --check      # healthcheck read-only (PRAGMA quick_check)
//	cosca db repair --db all --dry-run        # plano SEM escrever nada
//	cosca db repair --db vector --apply       # executa o repair (ledger guard → backup
//	                                          # forense → quarentena → recipe → health pós)
//
// Classes (`--db`): session | index | knowledge | vector | all (default all).
// FORA DE ESCOPO por construção: chain (family_chain.dat), embed e
// identidade/chaves — nunca auto (ADR-014 regra 3; SupportedKinds).
//
// Exit: 0 = nada a reparar / reparado; != 0 = banco doente detectado ou
// repair recusado/falho (as instruções manuais aparecem no output).
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/staterepair"
)

// repairStatusText renderiza o Status do staterepair em pt-BR para o output.
func repairStatusText(s staterepair.Status) string {
	switch s {
	case staterepair.StatusOK:
		return "ok"
	case staterepair.StatusSick:
		return "doente"
	case staterepair.StatusMissing:
		return "não encontrado"
	case staterepair.StatusDryRun:
		return "plano (dry-run)"
	case staterepair.StatusRepaired:
		return "reparado"
	case staterepair.StatusAttemptsExhausted:
		return "recusado (ledger exausto)"
	case staterepair.StatusRecipeFailed:
		return "recipe falhou"
	case staterepair.StatusError:
		return "erro"
	default:
		return string(s)
	}
}

// repairNeedsAttention devolve true quando o status exige exit != 0.
func repairNeedsAttention(s staterepair.Status) bool {
	switch s {
	case staterepair.StatusOK, staterepair.StatusMissing, staterepair.StatusRepaired:
		return false
	default:
		return true
	}
}

// NewDBRepairCommand cria `cosca db repair`.
func NewDBRepairCommand() *cobra.Command {
	var (
		dbSel   string
		dataDir string
		check   bool
		dryRun  bool
		apply   bool
	)

	cmd := &cobra.Command{
		Use:   "repair",
		Short: "Repair assistido de bancos derivados/regeneráveis (ADR-043 §8)",
		Long: `Repair assistido de estado para bancos DERIVADOS e regeneráveis
(session.db, memory/index.db, knowledge.db e vector-*.db) — ADR-043 §8,
implementando a Camada C do ADR-014 para a classe derivado/estado.

Mecânica (adaptada de hermes_state_repair.py):
  fingerprint do arquivo doente (tamanho + amostra com header volátil
  mascarado → escrita viva não rearma o ledger) + ledger sidecar
  <db>.repair-attempts.json (recusa cirurgia após 3 falhas no MESMO
  fingerprint) + backup forense deduplicado por conteúdo (retenção 3) +
  quarentena + recipe de rebuild canônica + health pós (PRAGMA quick_check).

FORA DE ESCOPO por construção: chain (family_chain.dat), embed e
identidade/chaves — nunca há caminho automático (ADR-014: sempre --manual + Don).

Modos (mutuamente exclusivos):
  --check     healthcheck read-only: lista ok/doente/não encontrado;
  --dry-run   mostra o plano completo SEM escrever nada (zero arquivos);
  --apply     executa o repair (é a única flag que escreve).

Exit: 0 = tudo ok ou reparado; != 0 = banco doente detectado ou repair
recusado/falho.`,
		Example: `  cosca db repair --check
  cosca db repair --db session --check
  cosca db repair --db all --dry-run
  cosca db repair --db vector --dry-run
  cosca db repair --db session --apply
  cosca db repair --db all --apply
  cosca db repair --json --dry-run`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			// Modo: exatamente um de check/dry-run/apply.
			modes := 0
			for _, on := range []bool{check, dryRun, apply} {
				if on {
					modes++
				}
			}
			if modes == 0 {
				return fmt.Errorf("escolha um modo: --check, --dry-run ou --apply")
			}
			if modes > 1 {
				return fmt.Errorf("modos mutuamente exclusivos: use apenas um de --check, --dry-run, --apply")
			}

			kind, kindErr := staterepair.ParseKind(dbSel)
			if kindErr != nil {
				return kindErr
			}

			dir, dirErr := resolveDataDir(dataDir)
			if dirErr != nil {
				return fmt.Errorf("resolve data directory: %w", dirErr)
			}

			mgr := staterepair.NewManager(dir)
			opts := staterepair.Options{Apply: apply, DryRun: dryRun}

			reports, repErr := mgr.Repair(kind, opts)
			if repErr != nil {
				return fmt.Errorf("db repair: %w", repErr)
			}

			mode := "check"
			if dryRun {
				mode = "dry-run"
			}
			if apply {
				mode = "apply"
			}

			allOK := true
			for _, r := range reports {
				if repairNeedsAttention(r.Status) {
					allOK = false
					break
				}
			}

			if useJSON {
				type repairJSON struct {
					Mode    string                   `json:"mode"`
					Kind    string                   `json:"kind"`
					Cosca   string                   `json:"cosca_dir"`
					AllOK   bool                     `json:"all_ok"`
					Reports []staterepair.PathReport `json:"reports"`
				}
				out, _ := json.MarshalIndent(repairJSON{
					Mode: mode, Kind: string(kind), Cosca: dir, AllOK: allOK, Reports: reports,
				}, "", "  ")
				f.Println(string(out))
			} else {
				printDBRepairText(f, mode, string(kind), dir, reports)
			}

			if !allOK {
				return ExitCodeError{Code: 1}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dataDir, "data-dir", "", "diretório .cosca explícito (bypass da resolução config-first; ex.: para testes)")
	cmd.Flags().StringVar(&dbSel, "db", "all", "classe de banco alvo: session|index|knowledge|vector|all (default all)")
	cmd.Flags().BoolVar(&check, "check", false, "healthcheck read-only (PRAGMA quick_check) — não escreve")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "plano completo SEM escrever nada (zero arquivos)")
	cmd.Flags().BoolVar(&apply, "apply", false, "executa o repair (única flag que escreve: backup, quarentena, recipe)")

	return cmd
}

// printDBRepairText renderiza o relatório de repair em texto.
func printDBRepairText(f *OutputFormatter, mode, kind, dir string, reports []staterepair.PathReport) {
	f.Header("DB Repair — bancos derivados/regeneráveis (ADR-043 §8)")
	f.KeyValue("Modo", mode)
	f.KeyValue("Classe", kind)
	f.KeyValue("Diretório", dir)
	f.Println("")

	if len(reports) == 0 {
		f.Print("Nenhum banco encontrado para a classe selecionada (glob sem matches).")
		return
	}

	rows := make([][]string, 0, len(reports))
	for _, r := range reports {
		detail := r.Detail
		if r.Status == staterepair.StatusDryRun && len(r.Recipe) > 0 {
			detail = "plano: " + strings.Join(r.Recipe, " → ")
		}
		rows = append(rows, []string{string(r.Kind), r.Path, repairStatusText(r.Status), detail})
	}
	f.Table([]string{"Classe", "Banco", "Estado", "Detalhe"}, rows)
	f.Println("")

	for _, r := range reports {
		switch r.Status {
		case staterepair.StatusOK:
			f.Success(fmt.Sprintf("%s: saudável (quick_check ok)", r.Path))
		case staterepair.StatusMissing:
			f.Print(fmt.Sprintf("%s: não encontrado — nada a reparar (não é corrupção)", r.Path))
		case staterepair.StatusDryRun:
			f.Warning(fmt.Sprintf("%s: DOENTE — plano acima. Nada foi escrito (dry-run).", r.Path))
			if r.BackupPath == "" && len(r.Recipe) > 0 {
				f.Print("  backup forense → .cosca/backups/staterepair/; quarentena → .cosca/quarantine/staterepair/")
			}
			if r.Fingerprint != "" {
				f.Verbose(fmt.Sprintf("fingerprint: %s", r.Fingerprint))
			}
		case staterepair.StatusSick:
			f.Error(fmt.Sprintf("%s: DOENTE — %s", r.Path, r.Detail))
		case staterepair.StatusRepaired:
			f.Success(fmt.Sprintf("%s: REPARADO — %s", r.Path, r.Detail))
			if r.BackupPath != "" {
				f.KeyValue("Backup forense", r.BackupPath)
			}
		case staterepair.StatusRecipeFailed:
			f.Error(fmt.Sprintf("%s: REPAIR FALHOU — %s", r.Path, r.Error))
			f.Warning("  Original RESTAURADO da quarentena — nada foi perdido, nada foi meio-reparado.")
			if r.BackupPath != "" {
				f.KeyValue("Backup forense", r.BackupPath)
			}
			if r.ManualInstructions != "" {
				f.Bullet(r.ManualInstructions)
			}
		case staterepair.StatusAttemptsExhausted:
			f.Error(fmt.Sprintf("%s: RECUSADO — %s", r.Path, r.Detail))
			if r.ManualInstructions != "" {
				f.Bullet(r.ManualInstructions)
			}
		case staterepair.StatusError:
			f.Error(fmt.Sprintf("%s: ERRO — %s", r.Path, r.Error))
		}
	}
	f.Println("")
	f.Print("Exit != 0 quando há banco doente, repair recusado ou falho. Chain/identidade nunca são reparadas automaticamente (ADR-014).")
}

// guarda de compilação: o comando segue o padrão cobra.
var _ *cobra.Command = NewDBRepairCommand()
