package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/knowledge"
)

// ─────────────────────────────────────────────────────────────────────────────
// Fase C (ADR-030 §3.3) — Snapshot copy-on-write + refs (CLI).
//
// `cosca knowledge snapshot` cria um snapshot do conhecimento
// content-addressable (objects/) e avança o ref (`main` por default).
// `cosca knowledge snapshot list` lista os snapshots.
// `cosca knowledge snapshot diff <refA> <refB>` mostra o diff cognitivo.
// `cosca knowledge checkout <ref>` troca o HEAD para um snapshot/ref.
// `cosca knowledge refs` lista os refs nomeados + o HEAD.
//
// O modelo é o padrão CAS do Asset Registry (asset.go): cada objeto é
// content-addressable por sha256; os snapshots são copy-on-write (referenciam
// hashes, nunca duplicam); os refs vivem em .cosca/knowledge/refs/.
// ─────────────────────────────────────────────────────────────────────────────

// knowledgeSource é um arquivo de conhecimento declarativo (a "fonte").
type knowledgeSource struct {
	abs   string
	rel   string
	class knowledge.KnowledgeEpistemic
}

// collectKnowledgeSources coleta os arquivos de conhecimento declarativo do
// projeto (.cosca/knowledge/{laws.json,hall-of-fame.json,manifest.yaml,
// lock.yaml,acquired/**,packages/*.json} + .cosca/provenance.yaml). É a fonte
// que vira objeto content-addressable. Escopo deliberadamente LIMITADO: NÃO
// entra em objects/ (CAS), snapshots/ ou refs/ — senão o snapshot referenciaria
// a si mesmo.
func collectKnowledgeSources(projectRoot string) []knowledgeSource {
	var out []knowledgeSource
	add := func(rel string, class knowledge.KnowledgeEpistemic) {
		if rel == "" {
			return
		}
		abs := filepath.Join(projectRoot, filepath.FromSlash(rel))
		if _, err := os.Stat(abs); err == nil {
			out = append(out, knowledgeSource{abs: abs, rel: rel, class: class})
		}
	}
	addAll := func(dirRel string, class knowledge.KnowledgeEpistemic) {
		dirAbs := filepath.Join(projectRoot, filepath.FromSlash(dirRel))
		if _, err := os.Stat(dirAbs); err != nil {
			return
		}
		// Diretórios específicos (nunca recursivo genérico sobre .cosca).
		_ = filepath.Walk(dirAbs, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			rel, rErr := filepath.Rel(projectRoot, path)
			if rErr != nil {
				return nil
			}
			out = append(out, knowledgeSource{abs: path, rel: filepath.ToSlash(rel), class: class})
			return nil
		})
	}

	// Arquivos top-level do DNA (ADR-029 §2.2/§2.3).
	add("knowledge/laws.json", knowledge.EpistemicFACT)
	add("knowledge/hall-of-fame.json", knowledge.EpistemicFACT)
	add("knowledge/manifest.yaml", knowledge.EpistemicFACT)
	add("knowledge/lock.yaml", knowledge.EpistemicDECISION)
	// Ledger de proveniência (fora de knowledge/, em .cosca/).
	add("provenance.yaml", knowledge.EpistemicEVIDENCE)
	// Diretórios declarativos.
	addAll("knowledge/acquired", knowledge.EpistemicFACT)
	addAll("knowledge/packages", knowledge.EpistemicDECISION)

	sort.Slice(out, func(i, j int) bool { return out[i].rel < out[j].rel })
	return out
}

// snapshotKnowledge cria/abre a store e devolve os hashes dos objetos coletados.
func snapshotKnowledge(projectRoot string, fs *knowledge.KnowledgeFS) ([]string, error) {
	sources := collectKnowledgeSources(projectRoot)
	hashes := make([]string, 0, len(sources))
	for _, s := range sources {
		o, err := fs.AddFile(s.abs, s.class, s.rel)
		if err != nil {
			return nil, err
		}
		hashes = append(hashes, o.Hash)
	}
	return hashes, nil
}

// resolveKnowledgeFS resolve a store .cosca/knowledge a partir do cwd.
func resolveKnowledgeFS() (*knowledge.KnowledgeFS, string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, "", fmt.Errorf("getwd: %w", err)
	}
	fs, err := knowledge.NewKnowledgeFS(cwd)
	if err != nil {
		return nil, "", err
	}
	return fs, cwd, nil
}

// NewKnowledgeSnapshotCommand cria o grupo `cosca knowledge snapshot`.
//
// Invocado sem subcomando, cria um snapshot do estado atual (advancing `main`
// ou o ref de `--name`). Subcomandos: `list` e `diff <refA> <refB>`.
func NewKnowledgeSnapshotCommand() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Cria e lista snapshots copy-on-write do conhecimento (Fase C, ADR-030)",
		Long: `Cria e lista snapshots copy-on-write do conhecimento em .cosca/knowledge/.

O conhecimento declarativo (laws.json, hall-of-fame.json, manifest.yaml,
lock.yaml, acquired/, packages/, provenance.yaml) é armazenado como objetos
content-addressable (objects/) — o MESMO padrão do Asset Registry. Um snapshot
REFERENCIA os hashes dos objetos ativos: criar um snapshot NUNCA duplica os
objetos existentes (copy-on-write), e os snapshots formam a genealogia do
conhecimento (checkout/diff).

O snapshot avança o ref nomeado (default "main") e o HEAD para ele.

Sem subcomando, cria o snapshot. Subcomandos:
  list                    Lista os snapshots + HEAD/refs
  diff <refA> <refB>      Diff cognitivo entre dois snapshots/refs`,
		Example: `  cosca knowledge snapshot                # cria o snapshot atual (ref main)
  cosca knowledge snapshot --name experimental
  cosca knowledge snapshot --json
  cosca knowledge snapshot list
  cosca knowledge snapshot diff main experimental`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			fs, root, err := resolveKnowledgeFS()
			if err != nil {
				return err
			}
			hashes, err := snapshotKnowledge(root, fs)
			if err != nil {
				return err
			}
			snap, err := fs.Snapshot(name, hashes)
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, snap)
			}

			formatter.Success(fmt.Sprintf("Snapshot de conhecimento criado (%s)", snap.ID))
			formatter.KeyValue("Ref", name)
			formatter.KeyValue("Objetos", fmt.Sprintf("%d", len(snap.Objects)))
			formatter.KeyValue("Parent", snap.Parent)
			formatter.KeyValue("Registro", filepath.Join(root, ".cosca", "knowledge", "snapshots"))
			if len(snap.Objects) == 0 {
				formatter.Warning("Nenhum objeto de conhecimento declarativo encontrado — o snapshot registra o estado vazio.")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "main", "Ref nomeado a avançar para este snapshot (default: main)")

	cmd.AddCommand(
		NewKnowledgeSnapshotListCommand(),
		NewKnowledgeSnapshotDiffCommand(),
	)
	return cmd
}

// NewKnowledgeSnapshotListCommand cria `cosca knowledge snapshot list`.
func NewKnowledgeSnapshotListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lista os snapshots de conhecimento",
		Example: `  cosca knowledge snapshot list
  cosca knowledge snapshot list --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			fs, _, err := resolveKnowledgeFS()
			if err != nil {
				return err
			}

			snaps, lErr := fs.ListSnapshots()
			if lErr != nil {
				return lErr
			}

			if useJSON {
				if err := printJSON(cmd, map[string]any{
					"snapshots": snaps,
					"head":      headInfo(fs),
				}); err != nil {
					return err
				}
				return nil
			}

			if len(snaps) == 0 {
				formatter.Warning("Nenhum snapshot de conhecimento — crie um com 'cosca knowledge snapshot'.")
				return nil
			}

			formatter.Header(fmt.Sprintf("Snapshots de conhecimento (%d)", len(snaps)))
			if hi := headInfo(fs); hi != "" {
				formatter.KeyValue("HEAD", hi)
			}

			rows := make([][]string, 0, len(snaps))
			for _, s := range snaps {
				rows = append(rows, []string{
					s.ID,
					s.Name,
					formatTimeShort(s.CreatedAt),
					fmt.Sprintf("%d", len(s.Objects)),
					s.Parent,
				})
			}
			formatter.Table([]string{"ID", "Ref", "Criado", "Objetos", "Parent"}, rows)
			return nil
		},
	}
	return cmd
}

// NewKnowledgeSnapshotDiffCommand cria `cosca knowledge snapshot diff <a> <b>`.
func NewKnowledgeSnapshotDiffCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff <refA> <refB>",
		Short: "Diff cognitivo entre dois snapshots de conhecimento",
		Long: `Diff cognitivo entre dois snapshots/refs (Fase C, ADR-030 §3.3).

Compara o conjunto de objetos content-addressable reachable por cada snapshot e
reporta as mudanças por proveniência (path) e classe epistêmica — não por
"diff de arquivo":
  + ADDED      objeto novo no snapshot B
  ~ CHANGED    mesmo path com conteúdo (hash) diferente
  - REMOVED    objeto presente no snapshot A mas não no B

Cada alvo pode ser um ref nomeado (main/experimental) ou um id de snapshot
(ks_...). O diff é determinístico — nunca inventa uma mudança.`,
		Example: `  cosca knowledge snapshot diff main experimental
  cosca knowledge snapshot diff ks_20260829_001 ks_20260829_002`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			fs, _, err := resolveKnowledgeFS()
			if err != nil {
				return err
			}
			diffs, dErr := fs.Diff(args[0], args[1])
			if dErr != nil {
				return dErr
			}

			if useJSON {
				return printJSON(cmd, map[string]any{
					"from": args[0],
					"to":   args[1],
					"diffs": diffs,
				})
			}

			formatter.Header(fmt.Sprintf("Diff cognitivo — %s → %s", args[0], args[1]))
			if len(diffs) == 0 {
				formatter.Success("Sem mudanças entre os snapshots.")
				return nil
			}

			sections := []struct {
				label string
				rank  knowledge.ObjectDiffChange
				icon  string
			}{
				{"ADDED (+)", knowledge.DiffAdded, "+"},
				{"CHANGED (~)", knowledge.DiffChanged, "~"},
				{"REMOVED (-)", knowledge.DiffRemoved, "-"},
			}
			for _, sec := range sections {
				shown := false
				for _, d := range diffs {
					if d.Change != sec.rank {
						continue
					}
					if !shown {
						formatter.Header(sec.label)
						shown = true
					}
					line := fmt.Sprintf("  %s %-10s %s", sec.icon, string(d.Class), d.Path)
					formatter.Println(line)
				}
			}
			formatter.KeyValue("Total", fmt.Sprintf("%d", len(diffs)))
			return nil
		},
	}
	return cmd
}

// NewKnowledgeCheckoutCommand cria `cosca knowledge checkout <ref>`.
func NewKnowledgeCheckoutCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "checkout <ref>",
		Short: "Troca o HEAD para um snapshot/ref de conhecimento (Fase C)",
		Long: `Troca o HEAD do conhecimento para um snapshot existente.

<ref> pode ser:
  - um ref nomeado (main, experimental, ...) → HEAD simbólico para o ref;
  - um id de snapshot (ks_...) → HEAD detached (como o git).

O checkout NÃO toca o knowledge.db (materialização) — apenas muda qual snapshot
é o ativo (o gênero do conhecimento "em vigor"). Persiste em
.cosca/knowledge/refs/HEAD.

Exemplo:
  cosca knowledge checkout experimental
  cosca knowledge checkout ks_20260829_001`,
		Example: `  cosca knowledge checkout main
  cosca knowledge checkout experimental
  cosca knowledge checkout ks_20260829_001`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			fs, _, err := resolveKnowledgeFS()
			if err != nil {
				return err
			}
			snap, cErr := fs.Checkout(args[0])
			if cErr != nil {
				return cErr
			}
			head, _ := fs.ReadHEAD()

			if useJSON {
				return printJSON(cmd, map[string]any{
					"checked_out": snap.ID,
					"head":        head,
					"objects":     len(snap.Objects),
				})
			}

			formatter.Success(fmt.Sprintf("Checkout concluído — HEAD → %s (%s)", head, snap.ID))
			formatter.KeyValue("Snapshot", snap.ID)
			formatter.KeyValue("Objetos", fmt.Sprintf("%d", len(snap.Objects)))
			formatter.KeyValue("Criado", formatTimeShort(snap.CreatedAt))
			return nil
		},
	}
	return cmd
}

// NewKnowledgeRefsCommand cria `cosca knowledge refs`.
func NewKnowledgeRefsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "refs",
		Short: "Lista os refs nomeados de conhecimento e o HEAD (Fase C)",
		Example: `  cosca knowledge refs
  cosca knowledge refs --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			fs, _, err := resolveKnowledgeFS()
			if err != nil {
				return err
			}
			refs, rErr := fs.Refs()
			if rErr != nil {
				return rErr
			}
			head, _ := fs.ReadHEAD()

			if useJSON {
				byRef := map[string]string{}
				for _, r := range refs {
					if v, e := fs.RefValue(r); e == nil {
						byRef[r] = v
					}
				}
				return printJSON(cmd, map[string]any{"head": head, "refs": byRef})
			}

			if len(refs) == 0 {
				formatter.Warning("Nenhum ref de conhecimento — crie um com 'cosca knowledge snapshot'.")
				return nil
			}
			formatter.Header(fmt.Sprintf("Refs de conhecimento (%d)", len(refs)))
			formatter.KeyValue("HEAD", head)
			rows := make([][]string, 0, len(refs))
			for _, r := range refs {
				v, _ := fs.RefValue(r)
				marker := ""
				if r == head {
					marker = "*"
				}
				rows = append(rows, []string{marker + r, v})
			}
			formatter.Table([]string{"Ref", "Snapshot"}, rows)
			return nil
		},
	}
	return cmd
}

// headInfo devolve uma descrição curta do HEAD (ref → snapshot) ou "".
func headInfo(fs *knowledge.KnowledgeFS) string {
	head, err := fs.ReadHEAD()
	if err != nil {
		return ""
	}
	if id, e := fs.RefValue(head); e == nil {
		return fmt.Sprintf("%s → %s", head, id)
	}
	return head
}

// formatTimeShort abrevia um RFC3339 para (yyyy-mm-dd hh:mm).
func formatTimeShort(ts string) string {
	if len(ts) >= 19 {
		return strings.Replace(ts[:16], "T", " ", 1)
	}
	return ts
}
