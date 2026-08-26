package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// NewKnowledgeManifestCommand cria `cosca knowledge manifest`.
//
// Gera o manifesto versionado do conhecimento (path + hash + classe epistêmica
// por documento) em `.cosca/knowledge-manifest.json` — o "índice do índice".
// Pequeno, rastreável e à prova de adulteração: a fonte + este manifesto
// regeneram/verificam o índice de forma determinística, sem versionar o binário
// de 254MB. Commite o manifesto no git.
func NewKnowledgeManifestCommand() *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "manifest",
		Short: "Gera o manifesto versionado do conhecimento (path+hash+classe) no git",
		Long: `Gera o manifesto do estado do conhecimento em .cosca/knowledge-manifest.json:
path + hash do conteúdo + classe epistêmica + escopo + kind de cada documento.
É o "índice do índice" — pequeno e versionável no git, tornando o estado do
conhecimento rastreável e reproduzível. O knowledge.db (binário derivado) fica
fora do git; este manifesto é a testemunha versionável. Commite-o.`,
		Example: `  cosca knowledge manifest           # grava em .cosca/knowledge-manifest.json
  cosca knowledge manifest --output relatorio.json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)

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

			man, mErr := ke.BuildKnowledgeManifest(cmd.Context())
			if mErr != nil {
				return fmt.Errorf("build manifest: %w", mErr)
			}

			if output == "" {
				output = filepath.Join(kbDir, "knowledge-manifest.json")
			}
			data, err := json.MarshalIndent(man, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal manifest: %w", err)
			}
			if err := os.WriteFile(output, data, 0o644); err != nil {
				return fmt.Errorf("write manifest %s: %w", output, err)
			}

			formatter.Header(fmt.Sprintf("Manifesto do conhecimento gerado — %d documentos", man.Count))
			formatter.KeyValue("destino", output)
			formatter.KeyValue("tamanho", fmt.Sprintf("%d bytes (%.0f KB)", len(data), float64(len(data))/1024))
			return nil
		},
	}

	cmd.Flags().StringVar(&output, "output", "", "Caminho de saída (default: .cosca/knowledge-manifest.json)")

	return cmd
}
