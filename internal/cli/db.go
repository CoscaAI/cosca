//
// `cosca db` — inspeção e governança dos bancos de dados do Cosca.
//
// Mede o tamanho on-disk real de cada banco SQLite via
// `PRAGMA page_count * page_size` (a métrica determinística do ADR-013) e
// reporta o % da referência de 100 MB + o status (ok | warn | fail).
//
// Desde 2026-09-08 (Decisão 1 do ADR-013 alterada pelo Don) os bancos são
// DERIVADOS e regeneráveis (cosca index rebuild / knowledge index / db build),
// não versionados no git — portanto o gate de tamanho NÃO falha por default:
// `cosca db check` (mesmo com --gate) é um relatório informativo (exit 0) e o
// bloqueio por tamanho só ocorre quando o operador arma um limite com
// `--limit-mb N` (N > 0).
//
// Subcomandos:
//   check                  Lista todos os bancos, tamanhos, % do teto e status.
//   check --gate           Modo gate: exit != 0 SOMENTE quando um limite de
//                          tamanho está armado (--limit-mb) e algum banco o
//                          cruza. Sem --limit-mb é apenas relatório.
//
// O comando é 100% READ-ONLY: nunca escreve, nunca migra, nunca apaga. Apenas
// mede e reporta.
//

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/dbhealth"
	"github.com/CoscaAI/cosca/internal/vectoragg"
)

// NewDBCommand cria a árvore de comandos `cosca db`.
func NewDBCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Inspeção e governança dos bancos de dados do Cosca (relatório de tamanho — ADR-013)",
		Long: `Governança do tamanho dos bancos de dados do Cosca (ADR-013, Decisão 1,
revisada em 2026-09-08).

Os bancos são derivados e regeneráveis (cosca index rebuild / knowledge index /
db build), não mais versionados no git — por isso o teto de 100 MB deixou de
ser uma regra de enforcement default. Este comando mede o tamanho on-disk real
de cada banco SQLite (PRAGMA page_count * page_size) e reporta o % da
referência de 100 MB + status como RELATÓRIO INFORMATIVO (exit 0) — o gate de
tamanho é opt-in via --limit-mb.

O comando é READ-ONLY: mede e reporta, nunca escreve, migra ou apaga.

Subcomandos:
  check              Lista todos os bancos, tamanhos, % do teto e status.
  check --gate       Gate de tamanho OPT-IN: com --limit-mb N (N > 0) alerta
                     em ~80% de N e falha (exit != 0) quando um banco cruza N;
                     sem --limit-mb é apenas relatório informativo (exit 0).`,
		Example: `  cosca db check
  cosca db check --gate
  cosca db check --json
  cosca db check --gate --limit-mb 100
  cosca db check --gate --limit-mb 100 --warn-mb 80
  cosca db check --gate --limit-mb 0.001   # teto minúsculo: prova o fail`,
	}

	cmd.AddCommand(NewDBCheckCommand())
	cmd.AddCommand(NewDBMirrorCommand())
	cmd.AddCommand(NewDBBuildCommand())
	cmd.AddCommand(NewDBVerifyCommand())
	cmd.AddCommand(NewDBMigrateCommand())
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
		Short: "Mede o tamanho on-disk de cada banco e reporta o % da referência de 100 MB",
		Long: `Mede o tamanho on-disk de cada banco de dados do Cosca e reporta o % da
referência de 100 MB (ADR-013, Decisão 1 — métrica informativa). Lê .cosca/*.db
(todos os bancos encontrados) e os módulos alvo (knowledge, memory/index, core,
events, projects, graph, vector, fts — os ausentes são reportados como
"não encontrado", sem quebrar).

Sem --limit-mb, é apenas relatório informativo (sempre exit 0, mesmo com
--gate): nenhum banco falha por tamanho (Decisão 1 alterada em 2026-09-08 —
bancos derivados/regeneráveis, não versionados).
Com --gate e --limit-mb N (N > 0), o gate de tamanho é armado:
  • warn  quando algum banco cruza ~80% de N;
  • fail  (exit != 0) quando algum banco cruza N.
--warn-mb W ajusta o limiar de alerta (default: 80% do --limit-mb); sem
--limit-mb armado, --warn-mb é ignorado.

O comando é READ-ONLY: nunca escreve no banco, nunca migra, nunca apaga.`,
		Example: `  cosca db check
  cosca db check --gate
  cosca db check --json
  cosca db check --gate --limit-mb 100
  cosca db check --gate --limit-mb 100 --warn-mb 80
  cosca db check --gate --limit-mb 0.001   # teto minúsculo: prova o fail`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getwd: %w", err)
			}
			coscaDir := filepath.Join(cwd, ".cosca")

			// Limites a partir das flags (MB binário). Semântica da Decisão 1
			// alterada em 2026-09-08: --limit-mb <= 0 = "sem teto" (gate de
			// tamanho desarmado — relatório informativo); --limit-mb N > 0
			// arma o teto, com alerta automático (~80% de N) ou explícito via
			// --warn-mb.
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

			// Gate: o fail por tamanho só existe com um limite armado
			// (--limit-mb N > 0) — exceder N bloqueia com exit != 0. Sem limite
			// armado nenhum banco falha por tamanho (relatório informativo). O
			// warn é apenas um alerta (exit 0).
			if gate && res.AnyFail {
				return ExitCodeError{Code: 1}
			}
			return nil
		},
	}

	cmd.Flags().Float64Var(&limitMB, "limit-mb", 0, "teto por banco em MB (0 = sem teto: relatório informativo; N > 0 arma o gate de tamanho)")
	cmd.Flags().Float64Var(&warnMB, "warn-mb", 0, "limiar de alerta em MB (0 = automático: 80%% do --limit-mb; ignorado sem --limit-mb)")
	cmd.Flags().BoolVar(&gate, "gate", false, "modo gate: exit != 0 quando algum banco cruza o teto armado via --limit-mb")

	return cmd
}

// printDBCheckText renderiza o relatório do gate em texto (tabela + resumo).
func printDBCheckText(f *OutputFormatter, res *dbhealth.Result) {
	f.Header("DB Check — Tamanho por banco (ADR-013 · Decisão 1 · referência 100 MB)")
	f.Printf("Diretório: %s\n", res.CoscaDir)
	if res.LimitBytes > 0 {
		f.Printf("Teto armado: %s  ·  Alerta: %s  ·  Bloqueio: %s\n",
			formatDBSize(res.LimitBytes), formatDBSize(res.WarnBytes), formatDBSize(res.FailBytes))
	} else {
		f.Printf("Teto: nenhum armado — relatório informativo (gate de tamanho opt-in via --limit-mb; 100 MB é a referência da métrica)\n")
	}
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

	if res.LimitBytes > 0 {
		if res.AnyWarn {
			f.Warning(fmt.Sprintf("Atenção: %d banco(s) cruzaram o alerta de %s (%.0f%% do teto armado).", warnCount(res), formatDBSize(res.WarnBytes), percent(res.WarnBytes, res.LimitBytes)))
		}
		if res.AnyFail {
			f.Error(fmt.Sprintf("BLOQUEIO: %d banco(s) cruzaram o teto de %s — o gate recusa (exit != 0).", failCount(res), formatDBSize(res.LimitBytes)))
		}
		return
	}
	if res.AnyWarn {
		// Sem teto armado, um status warn só reflete problema de leitura/estado
		// do arquivo (nunca tamanho) — informativo, não bloqueia.
		f.Warning(fmt.Sprintf("Atenção: %d banco(s) em estado de alerta (leitura/estado) — sem teto armado, nenhum bloqueio por tamanho.", warnCount(res)))
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

// NewDBMirrorCommand cria `cosca db mirror` — o espelho de leitura do ADR-013
// (§6.3, Fase B): abre os módulos lógicos (vector/graph/fts/projects) via
// ATTACH read-only sobre o knowledge.db atual e prova que o agregador lê as
// projeções tipadas SEM escrever, migrar ou tocar em nada.
//
// 100% READ-ONLY por construção (vectoragg abre tudo com mode=ro). Uma escrita
// acidental falharia no nível SQLite — o espelho é verificação, não migração.
func NewDBMirrorCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mirror",
		Short: "Espelho de leitura (ADR-013 Fase B) — agrega os módulos lógicos sobre o knowledge.db sem escrever",
		Long: `Espelho de leitura (split-de-leitura) do ADR-013 §6.3.

Os módulos lógicos (vector, graph, fts, projects) ainda vivem num único
knowledge.db. Este comando abre o agregador read-only (vectoragg) com o
catálogo-espelho (todos os módulos apontando para o mesmo arquivo), e reporta
as contagens de cada projeção tipada — provando que o agregador lê de módulos
separados SEM escrever, migrar ou alterar nada.

Isso é a verificação da Fase B: o dia em que os módulos virarem arquivos
físicos, o mesmo agregador lerá sem mudança de contrato.

O comando é 100% READ-ONLY: mode=ro em todas as conexões, nenhuma escrita,
nenhum migrate, nenhum drop.`,
		Example: `  cosca db mirror
  cosca db mirror --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			useJSON := IsJSONOutput(cmd)
			f := GetFormatter(cmd)

			dir, dirErr := resolveDataDir("")
			if dirErr != nil {
				return fmt.Errorf("resolve data directory: %w", dirErr)
			}
			kb := filepath.Join(dir, "knowledge.db")

			// PÓS-CORTE (Plano D): se os módulos físicos existem, o espelho
			// lê DELES (a verdade). O monolito é usado só quando o corte não
			// está ativo (legado) — verificar contra a fonte drenada seria
			// comparar contra um alvo esvaziado.
			agg, err := openModuleAggregator(dir, kb)
			if err != nil {
				return err
			}
			defer agg.Close()

			counts, countErr := agg.CatalogCounts()
			if countErr != nil {
				return fmt.Errorf("contar módulos: %w", countErr)
			}

			type mirrorReport struct {
				KnowledgeDB string           `json:"knowledge_db"`
				Modules     []string         `json:"modules"`
				Counts      vectoragg.Counts `json:"counts"`
				ReadOnly    bool             `json:"read_only"`
			}
			rep := mirrorReport{
				KnowledgeDB: kb,
				Modules:     agg.Modules(),
				Counts:      counts,
				ReadOnly:    true,
			}

			if useJSON {
				b, _ := json.MarshalIndent(rep, "", "  ")
				f.Println(string(b))
				return nil
			}

			f.Header("Espelho de leitura (ADR-013 Fase B) — read-only")
			f.KeyValue("knowledge.db", kb)
			f.KeyValue("Módulos lógicos", fmt.Sprintf("%v", rep.Modules))
			f.Println("")
			f.Header("Projeções tipadas (contagens)")
			f.KeyValue("vectors", fmt.Sprintf("%d", counts.Vectors))
			f.KeyValue("entities", fmt.Sprintf("%d", counts.Entities))
			f.KeyValue("relationships", fmt.Sprintf("%d", counts.Relationships))
			f.KeyValue("chunks_fts", fmt.Sprintf("%d", counts.ChunksFTS))
			f.KeyValue("entities_fts", fmt.Sprintf("%d", counts.EntitiesFTS))
			f.Println("")
			f.Print("O espelho lê os módulos lógicos sem escrever (mode=ro).")
			return nil
		},
	}
	return cmd
}

// sumPartitionVectors soma os vetores de todas as partições vector-*.db —
// o agregador lê por nome de módulo (uma partição); a contagem real do
// conhecimento é a SOMA (o corte particionou por domínio).
func sumPartitionVectors(dir string) int {
	parts, err := filepath.Glob(filepath.Join(dir, "vector-*.db"))
	if err != nil {
		return 0
	}
	total := 0
	for _, p := range parts {
		db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(p)+"?mode=ro")
		if err != nil {
			continue
		}
		var n int
		if err := db.QueryRow("SELECT COUNT(*) FROM vectors").Scan(&n); err == nil {
			total += n
		}
		_ = db.Close()
	}
	return total
}

// openModuleAggregator abre o agregador read-only sobre os MÓDULOS físicos
// (Plano D, pós-corte): o catálogo aponta para core/graph/projects/vector-*.
// Fallback ao monolito (MirrorCatalog) apenas quando os módulos não existem
// (legado / corte não ativo).
func openModuleAggregator(dir, kb string) (*vectoragg.Aggregator, error) {
	// 1. Tenta os módulos físicos (corte ativo).
	catalog := make(vectoragg.ModuleCatalog)
	// O vector é particionado: o agregador lê a soma via um catálogo que
	// aponta para CADA partição como um módulo "vector-<domínio>" — mas o
	// vectoragg agrega por nome de módulo. Solução: o catálogo mapeia a
	// primeira partição para "vector" (a leitura de todas usa o glob real).
	parts, err := filepath.Glob(filepath.Join(dir, "vector-*.db"))
	if err == nil && len(parts) > 0 {
		catalog[vectoragg.ModuleVector] = parts[0]
	}
	if _, statErr := os.Stat(filepath.Join(dir, "graph.db")); statErr == nil {
		catalog[vectoragg.ModuleGraph] = filepath.Join(dir, "graph.db")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "projects.db")); statErr == nil {
		catalog[vectoragg.ModuleProjects] = filepath.Join(dir, "projects.db")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "fts.db")); statErr == nil {
		catalog[vectoragg.ModuleFTS] = filepath.Join(dir, "fts.db")
	}
	if len(catalog) > 0 {
		return vectoragg.Open(catalog)
	}

	// 2. Fallback: monolito (legado).
	if _, statErr := os.Stat(kb); statErr == nil {
		return vectoragg.Open(vectoragg.MirrorCatalog(kb))
	}
	return nil, fmt.Errorf("nem módulos físicos nem knowledge.db encontrados em %s", dir)
}

// guarda de compilação: o comando db segue o padrão cobra.
var (
	_ *cobra.Command = NewDBCommand()
	_ *cobra.Command = NewDBCheckCommand()
	_ *cobra.Command = NewDBMirrorCommand()
)
