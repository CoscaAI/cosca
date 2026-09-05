//
// `cosca cv` — Cognitive Snapshot / Cognitive Version (CV).
//
// O CV congela o "estado cognitivo" da Cosca: um ledger imutável de hashes
// SHA-256 (documentação + conhecimento + leis + memória + prompts + políticas
// + schemas + configuração). NÃO armazena blobs — cada manifest
// (.cosca/cv/CV-XXXX.json) é um livro-razão de verificação.
//
// Rollback é "diagnóstico + guia, não reescrita automática" (regra do Don):
// NUNCA sobrescrevemos arquivos sem comparar antes.
//
// Subcomandos:
//   snapshot            Cria CV-XXXX e imprime a tabela (componente, hash, arquivos)
//   list                Lista os CVs existentes
//   verify [CV-XXXX]    Verifica o estado atual contra um CV (padrão: último)
//   rollback [CV-XXXX]  Diagnóstico + guia de restauração (git revert), nunca reescreve
//

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/cognitive"
)

// cvComponentHeaders são as colunas da tabela de componentes.
var cvComponentHeaders = []string{"Component", "SHA-256", "Files"}

// NewCVCommand cria a árvore de comandos `cosca cv`.
func NewCVCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cv",
		Short: "Cognitive Snapshot / Cognitive Version (CV) — ledger de verificação do estado cognitivo",
		Long: `Cognitive Snapshot / Cognitive Version (CV).

Congela o estado cognitivo da Cosca (documentação, conhecimento, leis, memória,
prompts, políticas, schemas, configuração) em um manifest imutável de hashes
SHA-256 em .cosca/cv/CV-XXXX.json. É um livro-razão de verificação — NÃO
armazena conteúdo dos arquivos.

Rollback é diagnóstico + guia, não reescrita automática: "cosca cv rollback"
nunca sobrescreve arquivos; ele compara o estado atual com o CV alvo, registra
o estado atual como novo snapshot e instrui a restauração via git.

Subcomandos:
  snapshot            Cria um novo CV e imprime a tabela (componente, hash, arquivos)
  list                Lista os CVs existentes
  verify [CV-XXXX]    Verifica o estado atual contra um CV (padrão: o mais recente)
  rollback [CV-XXXX]  Diagnóstico + guia de restauração (padrão: o mais recente)`,
		Example: `  cosca cv snapshot
  cosca cv list
  cosca cv verify CV-0003
  cosca cv rollback CV-0003`,
	}

	cmd.AddCommand(
		NewCVSnapshotCommand(),
		NewCVListCommand(),
		NewCVVerifyCommand(),
		NewCVRollbackCommand(),
	)
	return cmd
}

// NewCVSnapshotCommand cria `cosca cv snapshot`.
func NewCVSnapshotCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "snapshot",
		Short: "Criar um Cognitive Snapshot (CV) do estado atual",
		Long: `Cria um manifest CV-XXXX em .cosca/cv/ com o SHA-256 de cada componente
cognitivo (kernel-docs, knowledge, laws, memory, agents, skills, config).
O versionamento é automático (CV-0001, CV-0002, ...). Operação somente leitura
dos arquivos cognitivos — apenas grava o novo manifest.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("falha ao obter o diretório de trabalho: %w", err)
			}
			formatter := GetFormatter(cmd)

			snap, err := cognitive.CreateSnapshot(cwd)
			if err != nil {
				return err
			}

			if IsJSONOutput(cmd) {
				return printJSON(cmd, snap)
			}

			formatter.Success(fmt.Sprintf("COGNITIVE SNAPSHOT %s criado — %d arquivos", snap.Version, snap.FilesAffected))
			rows := make([][]string, 0, len(snap.Components))
			for _, comp := range cognitive.DefaultComponents {
				rec := snap.Components[comp.Name]
				rows = append(rows, []string{comp.Name, rec.Hash, fmt.Sprintf("%d", len(rec.Files))})
			}
			formatter.Table(cvComponentHeaders, rows)
			formatter.KeyValue("Manifest", snap.ManifestPath)
			return nil
		},
	}
}

// NewCVListCommand cria `cosca cv list`.
func NewCVListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Listar os Cognitive Versions (CVs) existentes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("falha ao obter o diretório de trabalho: %w", err)
			}
			formatter := GetFormatter(cmd)

			snaps, err := cognitive.ListSnapshots(cwd)
			if err != nil {
				return err
			}
			if IsJSONOutput(cmd) {
				return printJSON(cmd, snaps)
			}
			if len(snaps) == 0 {
				formatter.Warning("Nenhum CV encontrado — rode \"cosca cv snapshot\" primeiro.")
				return nil
			}
			rows := make([][]string, 0, len(snaps))
			for _, s := range snaps {
				rows = append(rows, []string{
					s.Version,
					s.CreatedAt.Format("2006-01-02 15:04:05"),
					fmt.Sprintf("%d", s.FilesAffected),
					s.ManifestPath,
				})
			}
			formatter.Table([]string{"CV", "Criado em", "Arquivos", "Manifest"}, rows)
			return nil
		},
	}
}

// NewCVVerifyCommand cria `cosca cv verify [CV-XXXX]`.
func NewCVVerifyCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "verify [CV-XXXX]",
		Short: "Verificar o estado atual contra um CV (padrão: o mais recente)",
		Long: `Re-computa o SHA-256 dos componentes cognitivos atuais e compara com o
manifest do CV informado (ou o mais recente). Imprime os componentes alterados,
removidos e adicionados. Não modifica nada.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("falha ao obter o diretório de trabalho: %w", err)
			}
			formatter := GetFormatter(cmd)

			version, err := resolveCVArg(cwd, args)
			if err != nil {
				return err
			}

			matches, report, err := cognitive.VerifySnapshot(cwd, version)
			if err != nil {
				return err
			}
			if IsJSONOutput(cmd) {
				return printJSON(cmd, report)
			}

			if matches {
				formatter.Success(fmt.Sprintf("%s confere com o estado atual — estado cognitivo íntegro.", report.Version))
				return nil
			}
			formatter.Error(fmt.Sprintf("%s NÃO confere com o estado atual — divergências:", report.Version))
			for _, comp := range cognitive.DefaultComponents {
				st := report.Components[comp.Name]
				if st.Match {
					continue
				}
				formatter.Header(comp.Name)
				for _, rel := range st.Changed {
					formatter.Bullet(fmt.Sprintf("mudou:      %s", filepath.ToSlash(rel)))
				}
				for _, rel := range st.Removed {
					formatter.Bullet(fmt.Sprintf("removido:   %s", filepath.ToSlash(rel)))
				}
				for _, rel := range st.Added {
					formatter.Bullet(fmt.Sprintf("adicionado: %s", filepath.ToSlash(rel)))
				}
			}
			return nil
		},
	}
}

// NewCVRollbackCommand cria `cosca cv rollback [CV-XXXX]`.
func NewCVRollbackCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "rollback [CV-XXXX]",
		Short: "Diagnóstico + guia de rollback do estado cognitivo (nunca reescreve)",
		Long: `Rollback = diagnóstico + guia, não reescrita automática (regra do Don: nunca
sobrescrever sem comparar). Verifica o estado atual contra o CV alvo, registra
o estado atual como um NOVO snapshot (marcador de segurança) e imprime o diff
com as instruções de restauração via git revert / git checkout. Nenhum arquivo
é alterado por este comando.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("falha ao obter o diretório de trabalho: %w", err)
			}
			formatter := GetFormatter(cmd)

			version, err := resolveCVArg(cwd, args)
			if err != nil {
				return err
			}

			err = cognitive.RollbackSnapshot(cwd, version)
			if err == nil {
				formatter.Success("Nenhuma ação de rollback necessária.")
				return nil
			}
			var rerr *cognitive.RollbackError
			if !errors.As(err, &rerr) || rerr.Guide == nil {
				return err
			}

			guide := rerr.Guide
			if IsJSONOutput(cmd) {
				return printJSON(cmd, guide)
			}

			formatter.Header(fmt.Sprintf("ROLLBACK %s — diagnóstico", guide.Version))
			formatter.KeyValue("Resumo", guide.Summary)
			if guide.Snapshot != nil {
				formatter.KeyValue("Marcador do estado atual", guide.Snapshot.Version+" (snapshot criado antes do rollback)")
			}

			formatter.Header("Divergências por componente")
			if guide.Report != nil {
				for _, comp := range cognitive.DefaultComponents {
					st := guide.Report.Components[comp.Name]
					if st.Match {
						continue
					}
					formatter.Bullet(fmt.Sprintf("%s: %d mudado(s), %d removido(s), %d adicionado(s)",
						comp.Name, len(st.Changed), len(st.Removed), len(st.Added)))
				}
			}

			formatter.Header("Guia de restauração (git)")
			for _, c := range guide.GitCommands {
				formatter.Bullet(c)
			}

			if len(guide.ManualSteps) > 0 {
				formatter.Header("Arquivos afetados")
				for _, step := range guide.ManualSteps {
					formatter.Bullet(step)
				}
			}
			return nil
		},
	}
}

// resolveCVArg devolve a versão CV do argumento (normalizada) ou, na ausência,
// a versão do snapshot mais recente.
func resolveCVArg(cwd string, args []string) (string, error) {
	if len(args) > 0 && args[0] != "" {
		return cognitive.NormalizeVersion(args[0])
	}
	snaps, err := cognitive.ListSnapshots(cwd)
	if err != nil {
		return "", err
	}
	if len(snaps) == 0 {
		return "", fmt.Errorf("nenhum CV encontrado — rode \"cosca cv snapshot\" primeiro")
	}
	return snaps[len(snaps)-1].Version, nil
}
