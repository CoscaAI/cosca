package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/knowledge"
)

// NewKnowledgeIngestCommand cria `cosca knowledge ingest <blockPath>`.
//
// É o gatilho explícito (CLI) do caminho de ingestão de um learning block da
// memória do agente: classifica (fail-closed) → verifica idempotência por
// SHA256 → indexa com a proveniência semântica (scope/origin/kind/agent)
// mesclada em metadata_json. O mesmo caminho é acionado automaticamente após
// `cosca memory register` (ver memory_register.go).
func NewKnowledgeIngestCommand() *cobra.Command {
	var agent string

	cmd := &cobra.Command{
		Use:   "ingest <blockPath>",
		Short: "Ingere um learning block da memória no Knowledge Base (proveniência + idempotência)",
		Long: `Ingere um learning block da memória do agente no Knowledge Base local.

Antes de indexar, roda o CLASSIFICADOR determinístico (fail-closed): se o bloco
não for conhecimento persistente (transiente/baixa confiança), NÃO indexa. Se
um documento com o MESMO SHA256 já existir, NÃO duplica (idempotência por
conteúdo; o nome do bloco é <hash>.md = hash do conteúdo).

O scope é decidido pelo classificador (path = proveniência física; scope =
jurisdição semântica): origem de projeto → project; cérebro embarcado
(internal/embed/cosca/**) → global. A ingestão NUNCA promove project→global.`,
		Example: `  cosca knowledge ingest internal/embed/cosca/memory/agent/cosca-kernel/blocks/<hash>.md
  cosca knowledge ingest .opencode/cosca/memory/agent/cosca-uiux/blocks/<hash>.md --agent cosca-uiux`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, err := os.Getwd()
			if err != nil {
				return err
			}
			kbDir := filepath.Join(dir, ".cosca")

			ke, kErr := openKnowledgeEngine(kbDir, false, dir)
			if kErr != nil {
				return fmt.Errorf("knowledge engine not available: %w", kErr)
			}
			if iErr := ke.Init(); iErr != nil {
				return fmt.Errorf("knowledge engine init failed: %w", iErr)
			}
			defer ke.Close()

			res, iErr := ke.IngestLearningBlock(cmd.Context(), args[0], agent)
			if iErr != nil {
				return fmt.Errorf("ingest failed: %w", iErr)
			}

			if useJSON {
				return printJSON(cmd, res)
			}

			switch res.Status {
			case knowledge.IngestStatusIngested:
				formatter.Success(fmt.Sprintf("Ingerido (%s) — hash %.16s… %s/%s", res.Status, res.Hash, res.Scope, res.Kind))
				if res.Document != "" {
					formatter.Bullet("doc: " + res.Document)
				}
			case knowledge.IngestStatusAlreadyIngested:
				formatter.Warning(fmt.Sprintf("Já estava ingerido (idempotência por SHA256) — hash %.16s…", res.Hash))
			case knowledge.IngestStatusSkippedNotPersistent:
				formatter.Warning(fmt.Sprintf("NÃO indexado (fail-closed): %s", res.Reason))
			default:
				formatter.Warning("status desconhecido: " + string(res.Status))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&agent, "agent", "", "Agent dono do bloco (metadata/proveniência)")

	return cmd
}
