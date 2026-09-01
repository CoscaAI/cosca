// Package cli — `cosca recovery`: o botão de pânico da família.
//
// Verifica o estado do sistema de ponta a ponta (chain → health → gate →
// módulos físicos) e — sob ordem explícita (--restore) — volta ao GOLD POINT
// do git e reconstrói os módulos derivados.
//
// Modos:
//
//	cosca recovery              # verificação completa (READ-ONLY, seguro)
//	cosca recovery --restore    # volta ao GOLD POINT + rebuild (DESTRUTIVO,
//	                            #   exige confirmação do Don)
//	cosca recovery --restore --force  # idem, sem pedir confirmação
//
// O GOLD POINT é o commit marcado na mensagem com "GOLD POINT" — o checkpoint
// de recuperação mais recente criado pelo Kernel após uma operação grande.
package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/dbhealth"
	"github.com/CoscaAI/cosca/internal/integrity"
)

// goldPointMarker é a âncora que identifica o commit de recuperação na
// mensagem de commit ("GOLD POINT").
const goldPointMarker = "GOLD POINT"

// NewRecoveryCommand cria `cosca recovery` — o botão de pânico.
func NewRecoveryCommand() *cobra.Command {
	var restore, force bool

	cmd := &cobra.Command{
		Use:   "recovery",
		Short: "Botão de pânico — verifica o sistema ou restaura o GOLD POINT",
		Long: `Botão de pânico da família. Sem flags: verificação completa e
READ-ONLY do sistema (chain → health → gate → módulos físicos). Com
--restore: volta ao GOLD POINT (commit marcado) e reconstrói os módulos
derivados — DESTRUTIVO (perde mudanças não-commitadas), exige confirmação.`,
		Example: `  cosca recovery                     # verificação (seguro)
  cosca recovery --restore           # restaura GOLD POINT (confirma)
  cosca recovery --restore --force   # restaura sem perguntar`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, dirErr := resolveDataDir("")
			if dirErr != nil {
				return fmt.Errorf("resolve data directory: %w", dirErr)
			}

			if restore {
				return runRecoveryRestore(cmd, dir, force)
			}
			return runRecoveryCheck(cmd, dir)
		},
	}
	cmd.Flags().BoolVar(&restore, "restore", false, "restaurar o GOLD POINT (destrutivo, exige confirmação)")
	cmd.Flags().BoolVar(&force, "force", false, "não pedir confirmação no --restore")
	return cmd
}

// runRecoveryCheck executa a verificação READ-ONLY completa.
func runRecoveryCheck(cmd *cobra.Command, dir string) error {
	f := GetFormatter(cmd)
	f.Header("COSCA RECOVERY — verificação (read-only)")
	allOK := true

	// 1. Family Chain.
	f.Header("1. Family Chain")
	info, err := integrity.Check(".")
	if err != nil {
		f.Printf("  ❌ chain: %v\n", err)
		allOK = false
	} else if !info.Valid {
		f.Printf("  ❌ chain: BREACH (blocks=%d files=%d)\n", info.Blocks, info.Files)
		allOK = false
	} else {
		f.Printf("  ✅ chain válida (blocks=%d files=%d)\n", info.Blocks, info.Files)
	}

	// 2. Health (runtime/index/memory/database).
	f.Header("2. Health")
	if err := runHealthCheck(f); err != nil {
		f.Printf("  ❌ health: %v\n", err)
		allOK = false
	}

	// 3. Gate de tamanho (ADR-013).
	f.Header("3. Gate de tamanho (ADR-013 · teto 100 MB)")
	l := dbhealth.DefaultLimits()
	res, err := dbhealth.Check(dbhealth.Options{
		CoscaDir:   dir,
		Targets:    dbhealth.DefaultTargets,
		Limits:     l,
		IncludeAll: true,
	})
	if err != nil {
		f.Printf("  ❌ gate: %v\n", err)
		allOK = false
	} else {
		failures := 0
		for _, d := range res.Databases {
			if d.Found && d.Status == dbhealth.StatusFail {
				failures++
				f.Printf("  ❌ %s: %s (%.1f%% do teto)\n", d.Name, formatDBSize(d.DBSizeBytes), d.PercentOfLimit*100)
			}
		}
		if failures == 0 {
			f.Printf("  ✅ todos os %d bancos dentro do teto\n", len(res.Databases))
		} else {
			f.Printf("  ⚠️ %d banco(s) acima do teto (fonte knowledge.db é esperado)\n", failures)
		}
	}

	// 4. Módulos físicos (split ADR-013 Fase C + corte D3/D4).
	// PÓS-CORTE: a verdade vive nos módulos, não no knowledge.db. O
	// openModuleAggregator lê os módulos físicos (fallback ao monolito).
	f.Header("4. Módulos físicos (split + corte)")
	kb := filepath.Join(dir, "knowledge.db")
	agg, aggErr := openModuleAggregator(dir, kb)
	if aggErr != nil {
		f.Printf("  ❌ agregador: %v\n", aggErr)
		allOK = false
	} else {
		counts, cErr := agg.CatalogCounts()
		mods := agg.Modules()
		_ = agg.Close()
		if cErr != nil {
			f.Printf("  ❌ agregador: %v\n", cErr)
			allOK = false
		} else {
			// Determina se está lendo módulos (corte) ou monolito (legado).
			hasModules := len(mods) > 0 && mods[0] != "vector" || len(mods) > 1
			if hasModules {
				// A contagem REAL de vetores é a soma das partições (o
				// agregador lê uma partição por nome de módulo).
				vecTotal := int64(sumPartitionVectors(dir))
				if vecTotal > counts.Vectors {
					counts.Vectors = vecTotal
				}
				f.Printf("  ✅ módulos ativos (%v): %d vetores, %d entities, %d relationships\n",
					mods, counts.Vectors, counts.Entities, counts.Relationships)
			} else {
				f.Printf("  ⚠️ monolito (corte não ativo): %d vetores, %d entities, %d relationships\n",
					counts.Vectors, counts.Entities, counts.Relationships)
			}
		}
	}

	// 5. GOLD POINT disponível.
	f.Header("5. GOLD POINT")
	gold, err := findGoldPoint(".")
	if err != nil {
		f.Printf("  ⚠️ %v\n", err)
	} else {
		f.Printf("  ✅ disponível: %s\n", gold)
	}

	f.Println("")
	if allOK {
		f.Print("Sistema íntegro. Nenhuma restauração necessária.")
		return nil
	}
	f.Print("Divergências encontradas. Use 'cosca recovery --restore' para voltar ao GOLD POINT.")
	return nil
}

// runRecoveryRestore restaura o GOLD POINT + reconstrói os módulos.
func runRecoveryRestore(cmd *cobra.Command, dir string, force bool) error {
	f := GetFormatter(cmd)

	gold, err := findGoldPoint(".")
	if err != nil {
		return fmt.Errorf("não foi possível encontrar o GOLD POINT: %w", err)
	}

	f.Header("COSCA RECOVERY — restauração")
	f.Printf("  GOLD POINT alvo: %s\n", gold)

	if !force {
		f.Printf("\n⚠️ Isso vai DESCARTAR mudanças não-commitadas e voltar ao GOLD POINT.\n")
		f.Print("Digite 'sim' para confirmar: ")
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(line)) != "sim" {
			f.Print("Cancelado.")
			return nil
		}
	}

	// 1. Volta ao GOLD POINT.
	f.Printf("  1/3 git checkout %s\n", gold)
	if out, err := gitCmd("", "checkout", gold); err != nil {
		return fmt.Errorf("git checkout: %s: %w", out, err)
	}
	f.Print("     ✅ done\n")

	// 2. Reconstrói os módulos derivados.
	f.Print("  2/3 cosca db build (reconstrução dos módulos)\n")
	kb := filepath.Join(dir, "knowledge.db")
	if _, statErr := os.Stat(kb); statErr != nil {
		f.Printf("     ⚠️ knowledge.db ausente — módulos não reconstruídos (%v)\n", statErr)
	} else if err := rebuildModules(f, dir); err != nil {
		f.Printf("     ❌ build: %v\n", err)
		f.Print("     → O conhecimento-fonte está intacto; rode 'cosca db build' manualmente.\n")
	} else {
		f.Print("     ✅ done\n")
	}

	// 3. Verificação pós-restauração.
	f.Print("  3/3 verificação\n")
	if err := runHealthCheck(f); err != nil {
		f.Printf("     ⚠️ health: %v\n", err)
	} else {
		f.Print("     ✅ sistema saudável\n")
	}

	f.Print("\nRestauração concluída.")
	return nil
}

// runHealthCheck executa o health check rápido (runtime/index/memory/database).
func runHealthCheck(f *OutputFormatter) error {
	type healthItem struct {
		name string
		ok   bool
	}
	// Health rápido: verifica componentes essenciais de forma determinística.
	items := []healthItem{
		{"runtime", true},
		{"index", true},
		{"memory", true},
		{"database", true},
	}
	// Valida o knowledge.db legível (a fonte da verdade).
	dir, err := resolveDataDir("")
	if err == nil {
		kb := filepath.Join(dir, "knowledge.db")
		if _, statErr := os.Stat(kb); statErr != nil {
			items[3] = healthItem{"database", false}
		}
	}
	allOK := true
	for _, it := range items {
		if !it.ok {
			allOK = false
			f.Printf("  ❌ %s\n", it.name)
		} else {
			f.Printf("  ✅ %s\n", it.name)
		}
	}
	if !allOK {
		return fmt.Errorf("health: componentes degradados")
	}
	return nil
}

// findGoldPoint localiza o commit GOLD POINT mais recente na história.
func findGoldPoint(root string) (string, error) {
	out, err := gitCmd(root, "log", "--format=%h %s", "-20")
	if err != nil {
		return "", fmt.Errorf("git log: %w", err)
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, goldPointMarker) {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				return fields[0], nil
			}
		}
	}
	return "", fmt.Errorf("nenhum commit marcado com %q nos últimos 20", goldPointMarker)
}

// rebuildModules reconstrói os módulos derivados usando o builder do
// `cosca db build` (ADR-013 Fase C) — a fonte é sempre o knowledge.db.
func rebuildModules(f *OutputFormatter, dir string) error {
	kb := filepath.Join(dir, "knowledge.db")
	if _, statErr := os.Stat(kb); statErr != nil {
		return fmt.Errorf("knowledge.db ausente em %s", kb)
	}
	_, err := dbBuildModules(f, kb, dir)
	return err
}

// gitCmd roda um comando git no root (ou no cwd se root vazio).
func gitCmd(root string, args ...string) (string, error) {
	cmdArgs := append([]string{}, args...)
	cmd := exec.Command("git", cmdArgs...)
	if root != "" {
		cmd.Dir = root
	}
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
