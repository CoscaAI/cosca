//
// `cosca db` — inspeção e governança dos bancos de dados do Cosca.
//
// Implementa o gate de tamanho da Decisão 1 do ADR-013 (§2.2.1): TODO banco do
// sistema (o Core/fonte da verdade e cada módulo) deve permanecer abaixo de
// 100 MB. Este comando mede o tamanho on-disk real de cada banco SQLite via
// `PRAGMA page_count * page_size` (a métrica determinística prescrita pelo ADR)
// e reporta o % do teto + o status (ok | warn | fail).
//
// Subcomandos:
//   check                  Lista todos os bancos, tamanhos, % do teto e status.
//   check --gate           Roda o mesmo cálculo MAS: warn ao cruzar ~80% do
//                          teto; fail (exit != 0) quando algum banco cruza 100%.
//
// O gate é 100% READ-ONLY: nunca escreve, nunca migra, nunca apaga. Apenas
// mede e reporta.
//

package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/dbhealth"
)

// NewDBCommand cria a árvore de comandos `cosca db`.
func NewDBCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Inspeção e governança dos bancos de dados do Cosca (gate de 100 MB — ADR-013)",
		Long: `Governança do tamanho dos bancos de dados do Cosca (ADR-013, Decisão 1).

Cada banco do sistema — o Core (fonte da verdade) e cada módulo — deve manter-se
abaixo de 100 MB. Este comando mede o tamanho on-disk real de cada banco SQLite
(PRAGMA page_count * page_size, o padrão determinístico do ADR) e reporta o % do
teto + status.

O gate é READ-ONLY: mede e reporta, nunca escreve, migra ou apaga.

Subcomandos:
  check              Lista todos os bancos, tamanhos, % do teto e status.
  check --gate       Warn ao cruzar ~80% do teto; fail (exit != 0) ao cruzar 100%.`,
		Example: `  cosca db check
  cosca db check --gate
  cosca db check --json
  cosca db check --gate --limit-mb 0.001`,
	}

	cmd.AddCommand(NewDBCheckCommand())
	return cmd
}

// NewDBCheckCommand cria `cosca db check` (e `cosca db check --gate`).
func NewDBCheckCommand() *cobra.Command {
	var (
		limitMB float64
		warnMB  float64
		gate    bool
	)

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Mede o tamanho on-disk de cada banco e reporta o % do teto de 100 MB",
		Long: `Mede o tamanho on-disk de cada banco de dados do Cosca e reporta o % do
teto (ADR-013, Decisão 1). Lê .cosca/*.db (todos os bancos encontrados) e os
módulos alvo (knowledge, memory/index, core, events, projects, graph, vector,
fts — os ausentes são reportados como "não encontrado", sem quebrar).

Sem --gate, é apenas listagem (sempre exit 0).
Com --gate:
  • warn  quando algum banco cruza ~80% do teto (~80 MB por default);
  • fail  (exit != 0) quando algum banco cruza 100% do teto.

O gate é READ-ONLY: nunca escreve no banco, nunca migra, nunca apaga.`,
		Example: `  cosca db check
  cosca db check --gate
  cosca db check --json
  cosca db check --gate --limit-mb 0.001
  cosca db check --gate --warn-mb 0.001`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getwd: %w", err)
			}
			coscaDir := filepath.Join(cwd, ".cosca")

			// Limites a partir das flags (MB binário). --warn-mb <= 0 → 80% do
			// teto; --limit-mb <= 0 → 100 MB (default).
			limits := dbhealth.FromConfigMB(limitMB, warnMB)

			res, err := dbhealth.Check(dbhealth.Options{
				CoscaDir:   coscaDir,
				Limits:     limits,
				IncludeAll: true, // lista o .cosca inteiro (runtime + alvos)
			})
			if err != nil {
				return fmt.Errorf("db check: %w", err)
			}
			res.Gate = gate

			if useJSON {
				return printJSON(cmd, res)
			}

			printDBCheckText(f, res)

			// Gate: só o fail (exceder 100%) bloqueia com exit != 0; o warn é
			// apenas um alerta (exit 0), conforme a Decisão 1.
			if gate && res.AnyFail {
				return ExitCodeError{Code: 1}
			}
			return nil
		},
	}

	cmd.Flags().Float64Var(&limitMB, "limit-mb", 100, "teto (limite) por banco em MB (default 100)")
	cmd.Flags().Float64Var(&warnMB, "warn-mb", 0, "limiar de alerta em MB (0 = automático: 80%% do --limit-mb)")
	cmd.Flags().BoolVar(&gate, "gate", false, "modo gate: exit != 0 quando algum banco cruza 100% do teto")

	return cmd
}

// printDBCheckText renderiza o relatório do gate em texto (tabela + resumo).
func printDBCheckText(f *OutputFormatter, res *dbhealth.Result) {
	f.Header("DB Check — Tamanho por banco (ADR-013 · Decisão 1 · teto 100 MB)")
	f.Printf("Diretório: %s\n", res.CoscaDir)
	f.Printf("Teto: %s  ·  Alerta: %s  ·  Bloqueio: %s\n",
		formatDBSize(res.LimitBytes), formatDBSize(res.WarnBytes), formatDBSize(res.FailBytes))
	f.Println("")

	rows := make([][]string, 0, len(res.Databases))
	for _, d := range res.Databases {
		rows = append(rows, []string{
			d.Name,
			d.RelPath,
			dbSizeColumn(d),
			formatPercent(d.PercentOfLimit),
			formatStatus(d),
		})
	}
	f.Table([]string{"Banco", "Caminho", "Tamanho on-disk", "% do teto", "Status"}, rows)

	f.Println("")
	if len(res.Databases) > 0 {
		found := 0
		for _, d := range res.Databases {
			if d.Found {
				found++
			}
		}
		f.Printf("Bancos no disco: %d  ·  Módulos alvo ausentes: %d\n",
			found, len(res.Databases)-found)
	}

	if res.AnyWarn {
		f.Warning(fmt.Sprintf("Atenção: %d banco(s) cruzaram %s (%.0f%% do teto).", warnCount(res), formatDBSize(res.WarnBytes), percent(res.WarnBytes, res.LimitBytes)))
	}
	if res.AnyFail {
		f.Error(fmt.Sprintf("BLOQUEIO: %d banco(s) cruzaram o teto de %s — o gate recusa (exit != 0).", failCount(res), formatDBSize(res.LimitBytes)))
	}
}

// dbSizeColumn formata a coluna "Tamanho on-disk".
func dbSizeColumn(d dbhealth.Report) string {
	if !d.Found {
		return "—"
	}
	return formatDBSize(d.DBSizeBytes)
}

// formatStatus renderiza o status do banco para a tabela.
func formatStatus(d dbhealth.Report) string {
	switch d.Status {
	case dbhealth.StatusNotFound:
		return "não encontrado"
	case dbhealth.StatusWarn:
		return "warn"
	case dbhealth.StatusFail:
		return "fail"
	default:
		return "ok"
	}
}

// formatPercent renderiza a fração do teto como percentual (%).
func formatPercent(f float64) string {
	return fmt.Sprintf("%.1f%%", f*100)
}

// warnCount conta os bancos existentes em status warn.
func warnCount(res *dbhealth.Result) int {
	n := 0
	for _, d := range res.Databases {
		if d.Found && d.Status == dbhealth.StatusWarn {
			n++
		}
	}
	return n
}

// failCount conta os bancos existentes em status fail.
func failCount(res *dbhealth.Result) int {
	n := 0
	for _, d := range res.Databases {
		if d.Found && d.Status == dbhealth.StatusFail {
			n++
		}
	}
	return n
}

// percent devolve a fração a/b como percentual inteiro (0-100+).
func percent(a, b int64) float64 {
	if b <= 0 {
		return 0
	}
	return float64(a) / float64(b) * 100
}

// formatDBSize formata um tamanho em bytes numa unidade binária legível
// (B / KiB / MiB / GiB / TiB).
func formatDBSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	units := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}
	idx := exp
	if idx >= len(units) {
		idx = len(units) - 1
	}
	return fmt.Sprintf("%.1f %s", float64(n)/float64(div), units[idx])
}

// guarda de compilação: o comando db segue o padrão cobra.
var (
	_ *cobra.Command = NewDBCommand()
	_ *cobra.Command = NewDBCheckCommand()
)
