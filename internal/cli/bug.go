//
// `cosca bug` — Bug Fingerprint: agrupar bugs iguais por campos estruturados.
//
// Quando o mesmo tipo de bug aparecer novamente, o Cosca NÃO deve depender da
// descrição humana para reconhecê-lo. Cada bug carrega um fingerprint
// estruturado (component, error, path, phase) que é a sua identidade real:
//
//	BUG-FP: runtime.backup, error=permission_denied, path=/backups, phase=snapshot
//
// BUG-184, BUG-201 e BUG-244 podem assim ser reconhecidos como a mesma família
// de fingerprint mesmo que as mensagens sejam diferentes. O registro vive em
// .cosca/bug.db (SQLite): o MESMO fingerprint reaparece → TimesSeen++ (nunca
// um registro duplicado).
//
// Subcomandos:
//   register --component <c> --error <e> [--path] [--phase] [--title]
//                               Registra o bug (BUG-XXXX) ou incrementa a família
//   match --component <c> --error <e> [--path] [--phase]
//                               Lista bugs da mesma família (path/phase só se ambos)
//   family <key>               Todos os bugs da família component:error
//   list                       Tabela (id, família, component:error, vezes, status)
//

package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/kernel"
)

// resolveBugStore abre a base de bugs do projeto atual
// (<projeto>/.cosca/bug.db).
func resolveBugStore() (*kernel.BugStore, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getwd: %w", err)
	}
	return kernel.NewBugStore(filepath.Join(dir, ".cosca", "bug.db"))
}

// NewBugCommand cria a árvore de comandos `cosca bug`.
func NewBugCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bug",
		Short: "Bug Fingerprint — agrupar bugs iguais por campos estruturados (não pela descrição)",
		Long: `Bug Fingerprint — agrupar bugs iguais por campos estruturados.

Quando o mesmo tipo de bug aparecer novamente, o Cosca NÃO depende da descrição
humana para reconhecê-lo: cada bug carrega um fingerprint estruturado
(component, error, path, phase) que é a sua identidade real.

  BUG-FP: runtime.backup, error=permission_denied, path=/backups, phase=snapshot

BUG-184, BUG-201 e BUG-244 podem ser reconhecidos como a mesma família de
fingerprint mesmo que as mensagens sejam diferentes. O registro vive em
.cosca/bug.db (SQLite): o MESMO fingerprint reaparece → TimesSeen++ e LastSeen
atualizado, nunca um registro duplicado.

Subcomandos:
  register --component <c> --error <e> [--path <p>] [--phase <ph>] [--title <t>]
                              Registra o bug (BUG-XXXX) ou incrementa a família
  match --component <c> --error <e> [--path <p>] [--phase <ph>]
                              Lista bugs da mesma família (path/phase só se ambos)
  family <key>                Todos os bugs da família component:error
  list                        Tabela (id, família, component:error, vezes, status)`,
		Example: `  cosca bug register --component runtime.backup --error permission_denied --path /backups --phase snapshot --title "backup falhou"
  cosca bug match --component runtime.backup --error permission_denied
  cosca bug family runtime.backup:permission_denied
  cosca bug list`,
	}

	cmd.AddCommand(
		NewBugRegisterCommand(),
		NewBugMatchCommand(),
		NewBugFamilyCommand(),
		NewBugListCommand(),
	)
	return cmd
}

// NewBugRegisterCommand cria `cosca bug register`.
func NewBugRegisterCommand() *cobra.Command {
	var component, errorCode, path, phase, title, status string

	cmd := &cobra.Command{
		Use:   "register",
		Short: "Registra um bug pelo fingerprint — novo BUG-XXXX ou incrementa a família existente",
		Long: `Registra um bug identificado pelo fingerprint estruturado. Se o
fingerprint (component:error:path:phase) já existir em .cosca/bug.db, NÃO
duplica: incrementa TimesSeen e atualiza LastSeen, devolvendo o BUG-XXXX
original — é assim que o Cosca reconhece a reincidência sem depender da
descrição humana.

  --component <c>  componente (ex.: runtime.backup) — obrigatório
  --error <e>      código do erro (ex.: permission_denied) — obrigatório
  --path <p>       caminho do recurso (ex.: /backups) — opcional
  --phase <ph>     fase do ciclo (ex.: snapshot) — opcional
  --title <t>      título descritivo (ajuda, não identidade) — opcional`,
		Example: `  cosca bug register --component runtime.backup --error permission_denied --path /backups --phase snapshot --title "backup falhou"`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			store, err := resolveBugStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			id, err := store.Register(kernel.BugRecord{
				Title: title,
				Fingerprint: kernel.BugFingerprint{
					Component: component,
					Error:     errorCode,
					Path:      path,
					Phase:     phase,
				},
				Status: status,
			})
			if err != nil {
				return err
			}

			rec, err := store.Get(id)
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, rec)
			}
			if rec.TimesSeen > 1 {
				formatter.Warning(fmt.Sprintf("Fingerprint já conhecido — %s reincidência #%d (sem duplicar)", id, rec.TimesSeen))
			} else {
				formatter.Success(fmt.Sprintf("Bug %s registrado", id))
			}
			formatter.KeyValue("ID", rec.ID)
			formatter.KeyValue("Fingerprint", rec.Fingerprint.Key())
			formatter.KeyValue("Família", rec.Family)
			formatter.KeyValue("Título", rec.Title)
			formatter.KeyValue("Visto", fmt.Sprintf("%dx", rec.TimesSeen))
			formatter.KeyValue("Status", rec.Status)
			return nil
		},
	}

	cmd.Flags().StringVar(&component, "component", "", "componente do bug (ex.: runtime.backup)")
	cmd.Flags().StringVar(&errorCode, "error", "", "código do erro (ex.: permission_denied)")
	cmd.Flags().StringVar(&path, "path", "", "caminho do recurso (ex.: /backups)")
	cmd.Flags().StringVar(&phase, "phase", "", "fase do ciclo (ex.: snapshot)")
	cmd.Flags().StringVar(&title, "title", "", "título descritivo do bug")
	cmd.Flags().StringVar(&status, "status", "", "status inicial (open, fixed, archived); padrão: open")
	_ = cmd.MarkFlagRequired("component")
	_ = cmd.MarkFlagRequired("error")
	return cmd
}

// NewBugMatchCommand cria `cosca bug match`.
func NewBugMatchCommand() *cobra.Command {
	var component, errorCode, path, phase string

	cmd := &cobra.Command{
		Use:   "match",
		Short: "Lista bugs da mesma família do fingerprint dado (component+error; path/phase só se ambos)",
		Long: `Lista os bugs da mesma família do fingerprint informado. A família é
definida por component+error (sempre exigidos); path e phase só participam da
comparação quando AMBOS os lados estiverem preenchidos — um lado vazio é
curinga. Mensagens humanas não participam da comparação.`,
		Example: `  cosca bug match --component runtime.backup --error permission_denied
  cosca bug match --component runtime.backup --error permission_denied --path /backups --phase snapshot`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			store, err := resolveBugStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			fp := kernel.BugFingerprint{
				Component: component,
				Error:     errorCode,
				Path:      path,
				Phase:     phase,
			}
			records, err := store.Match(fp)
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, records)
			}
			if len(records) == 0 {
				formatter.Warning(fmt.Sprintf("Nenhum bug na família %q — rode \"cosca bug register --component %s --error %s ...\".",
					fp.FamilyKey(), component, errorCode))
				return nil
			}
			formatter.Header(fmt.Sprintf("Família %s — %d bug(s)", fp.FamilyKey(), len(records)))
			printBugTable(formatter, records)
			return nil
		},
	}

	cmd.Flags().StringVar(&component, "component", "", "componente do bug (ex.: runtime.backup)")
	cmd.Flags().StringVar(&errorCode, "error", "", "código do erro (ex.: permission_denied)")
	cmd.Flags().StringVar(&path, "path", "", "caminho do recurso (ex.: /backups)")
	cmd.Flags().StringVar(&phase, "phase", "", "fase do ciclo (ex.: snapshot)")
	_ = cmd.MarkFlagRequired("component")
	_ = cmd.MarkFlagRequired("error")
	return cmd
}

// NewBugFamilyCommand cria `cosca bug family <key>`.
func NewBugFamilyCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "family <key>",
		Short: "Todos os bugs da família de fingerprint component:error",
		Long: `Lista todos os bugs que pertencem à família de chave canônica
component:error (ex.: runtime.backup:permission_denied). A chave é normalizada
antes da consulta — case e whitespace não importam.`,
		Example: `  cosca bug family runtime.backup:permission_denied
  cosca bug family runtime.backup:permission_denied --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			store, err := resolveBugStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			records, err := store.Family(args[0])
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, records)
			}
			if len(records) == 0 {
				formatter.Warning(fmt.Sprintf("Nenhum bug na família %q.", args[0]))
				return nil
			}
			formatter.Header(fmt.Sprintf("Família %s — %d bug(s)", args[0], len(records)))
			printBugTable(formatter, records)
			return nil
		},
	}
}

// NewBugListCommand cria `cosca bug list`.
func NewBugListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Lista todos os bugs (id, família, component:error, vezes visto, status)",
		Long: `Lista os bugs registrados em .cosca/bug.db em uma tabela
(id, família, component:error, vezes visto, status). Apenas leitura.`,
		Example: `  cosca bug list
  cosca bug list --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			store, err := resolveBugStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			records, err := store.List()
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, records)
			}
			if len(records) == 0 {
				formatter.Warning("Nenhum bug registrado ainda — rode \"cosca bug register --component <c> --error <e> ...\".")
				return nil
			}
			formatter.Header(fmt.Sprintf("Bugs — %d registro(s)", len(records)))
			printBugTable(formatter, records)
			return nil
		},
	}
}

// printBugTable imprime a tabela comum de bugs (id, família, component:error,
// vezes visto, status) via formatter injetado.
func printBugTable(formatter *OutputFormatter, records []kernel.BugRecord) {
	rows := make([][]string, 0, len(records))
	for _, r := range records {
		rows = append(rows, []string{
			r.ID,
			r.Family,
			r.Fingerprint.Key(),
			fmt.Sprintf("%dx", r.TimesSeen),
			r.Status,
			r.LastSeen.Format("2006-01-02 15:04:05"),
		})
	}
	formatter.Table([]string{"ID", "Família", "Fingerprint", "Visto", "Status", "Último em"}, rows)
}
