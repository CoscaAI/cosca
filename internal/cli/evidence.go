//
// `cosca evidence` — Aquisição de evidência externa (ACQUIRE → VERIFY/HASH →
// QUARANTINE).
//
// Pipeline (da conversa do Don):
//
//	INTERNET → External Sources (GitHub, GitLab, Docs, RFCs, registries)
//	→ ACQUISITION → NORMALIZATION → FINGERPRINT → PROVENANCE → QUARANTINE
//	→ EXTRACTION → FTS5/VECTOR → EVIDENCE GRAPH.
//
// Regra de ouro: EXTERNAL DATA ≠ TRUSTED DATA. Tudo o que vem da internet
// começa como UNTRUSTED — a proveniência de todo artefato A-XXXX nasce
// "UNTRUSTED" e nada é promovido automaticamente. Busca é uma coisa; execução
// é outra fronteira: conteúdo externo NUNCA é executado.
//
// Subcomandos:
//   fetch <url> [--allow-remote]   Busca, hasheia (SHA-256), persiste o
//                                   artefato A-XXXX e cria a proposal Q-XXXX
//                                   (external, status pending) na quarentena
//   list                           Tabela dos artefatos adquiridos
//   show <id>                      Detalhe completo de um artefato
//   promote <id> --item K-XX       Promoção MANUAL: marca a proposal promovida
//                                   e adiciona a evidência ao item (P2/P4).
//                                   Requer --item explícito — nunca auto-promove
//
// O corpo do artefato vive em .cosca/quarantine/artifacts/A-XXXX (fora do
// conhecimento). O hardening espelha o guard de SSRF do skills.go:
// fail-closed (--allow-remote), timeout 5s, teto de 3 redirects, 10 MiB por
// corpo e rejeição de esquemas não-http(s) + IPs privados/loopback/link-local.
//

package cli

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/acquisition"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/quarantine"
)

// resolveArtifactStore devolve a store de artefatos do diretório de trabalho
// atual (<projeto>/.cosca/quarantine/artifacts).
func resolveArtifactStore() *acquisition.ArtifactStore {
	dir, err := os.Getwd()
	if err != nil {
		return acquisition.NewArtifactStore(".")
	}
	return acquisition.NewArtifactStore(dir)
}

// allowPrivateArtifacts é o opt-in explícito para destinos privados/loopback
// (mock servers locais, LANs). O default é fail-closed; a flag espelha a
// disciplina COSCA_SKILLS_ALLOW_REMOTE_SOURCES do skills.go.
func allowPrivateArtifacts() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("COSCA_ACQUISITION_ALLOW_PRIVATE"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// NewEvidenceCommand cria a árvore de comandos `cosca evidence`.
func NewEvidenceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "evidence",
		Short: "Aquisição de evidência externa — ACQUIRE → VERIFY/HASH → QUARANTINE (UNTRUSTED até promoção manual)",
		Long: `Aquisição de evidência externa (ACQUIRE → VERIFY/HASH → QUARANTINE).

Busca conteúdo na internet (GitHub, GitLab, Docs, RFCs, registries), calcula o
SHA-256, persiste o artefato A-XXXX e o quarentena como proposal Q-XXXX
(external, status pending). O corpo NUNCA entra em knowledge/laws/memory —
fica em .cosca/quarantine/artifacts/A-XXXX.

REGRAS DO DON:
  * EXTERNAL DATA ≠ TRUSTED DATA — todo artefato nasce "UNTRUSTED".
  * Nada é promovido automaticamente: promote exige --item explícito.
  * Conteúdo externo nunca é executado durante a aquisição.
  * SSRF fail-closed: --allow-remote explícito; apenas http/https; IPs
    privados/loopback/link-local são rejeitados.

Subcomandos:
  fetch <url> [--allow-remote]   Busca, hasheia e quarentena (A-XXXX + Q-XXXX)
  list                           Tabela dos artefatos adquiridos
  show <id>                      Detalhe completo de um artefato
  promote <id> --item K-XX       Promoção manual → evidência no item (P2/P4)`,
		Example: `  cosca evidence fetch https://raw.githubusercontent.com/CoscaAI/cosca/main/README.md --allow-remote
  cosca evidence list
  cosca evidence show A-0001
  cosca evidence promote A-0001 --item K-01`,
	}

	cmd.AddCommand(
		NewEvidenceFetchCommand(),
		NewEvidenceListCommand(),
		NewEvidenceShowCommand(),
		NewEvidencePromoteCommand(),
	)
	return cmd
}

// NewEvidenceFetchCommand cria `cosca evidence fetch <url>`.
func NewEvidenceFetchCommand() *cobra.Command {
	var allowRemote bool

	cmd := &cobra.Command{
		Use:   "fetch <url>",
		Short: "Busca uma evidência externa, hasheia (SHA-256) e a quarentena",
		Long: `Busca a URL, calcula o SHA-256, persiste o artefato A-XXXX em
.cosca/quarantine/artifacts/ e cria a proposal Q-XXXX (external, status
pending) na quarentena.

A proveniência é SEMPRE "UNTRUSTED" — evidência externa ≠ dado confiável, e
nada é promovido automaticamente. O corpo fica em
.cosca/quarantine/artifacts/A-XXXX, fora do conhecimento.

Hardening (espelho do skills.go): fetch exige --allow-remote explícito
(fail-closed); apenas http/https; IPs privados/loopback/link-local rejeitados
(SSRF — ex.: 169.254.169.254); timeout 5s; teto de 3 redirects; 10 MiB por
corpo. Para mock servers locais (testes), defina
COSCA_ACQUISITION_ALLOW_PRIVATE=true.`,
		Example: `  cosca evidence fetch https://raw.githubusercontent.com/CoscaAI/cosca/main/README.md --allow-remote`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			client := acquisition.NewClient(
				acquisition.DefaultTimeout,
				acquisition.DefaultMaxRedirects,
				acquisition.DefaultMaxBodyBytes,
			)
			client.AllowRemote = allowRemote
			client.AllowLoopback = allowPrivateArtifacts()

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			art, body, err := client.FetchAll(ctx, args[0])
			if err != nil {
				return err
			}

			// 1) Persiste o artefato A-XXXX + corpo (fora do conhecimento).
			astore := resolveArtifactStore()
			artID, err := astore.Add(art, body)
			if err != nil {
				return err
			}

			// 2) Quarentena: proposal Q-XXXX (external, pending) — referência
			//    apenas, nunca o dump completo.
			qstore, err := resolveQuarantineStore()
			if err != nil {
				return err
			}
			qID, err := acquisition.QuarantineArtifact(qstore, art, "adquirido via cosca evidence fetch")
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, map[string]interface{}{
					"artifact":   art,
					"proposal":   qID,
					"status":     quarantine.StatusPending,
					"provenance": art.Provenance,
				})
			}

			formatter.Success(fmt.Sprintf(
				"Artefato %s adquirido — SHA256 %s, %d bytes",
				artID, abbreviateSHA(art.SHA256), art.SizeBytes))
			formatter.KeyValue("URL", art.URL)
			formatter.KeyValue("SHA256", art.SHA256)
			formatter.KeyValue("Tamanho", fmt.Sprintf("%d bytes", art.SizeBytes))
			formatter.KeyValue("Content-Type", art.ContentType)
			formatter.KeyValue("Proveniência", art.Provenance)
			formatter.KeyValue("Quarentena", qID+" (external, "+quarantine.StatusPending+")")
			formatter.Warning("UNTRUSTED — em quarentena, nada promovido automaticamente")
			return nil
		},
	}

	cmd.Flags().BoolVar(&allowRemote, "allow-remote", false, "habilita a busca remota (fail-closed — default desabilitado)")
	return cmd
}

// NewEvidenceListCommand cria `cosca evidence list`.
func NewEvidenceListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Lista os artefatos de evidência externa adquiridos",
		Long: `Lista os artefatos adquiridos (A-XXXX) numa tabela: id, url, SHA256
abreviado, tamanho e data de recuperação. Todos nascem "UNTRUSTED".`,
		Example: `  cosca evidence list
  cosca evidence list --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			arts, err := resolveArtifactStore().List()
			if err != nil {
				return err
			}
			if useJSON {
				return printJSON(cmd, arts)
			}
			if len(arts) == 0 {
				formatter.Warning("Nenhum artefato adquirido — rode \"cosca evidence fetch <url> --allow-remote\".")
				return nil
			}
			formatter.Header(fmt.Sprintf("Evidências externas adquiridas — %d artefato(s)", len(arts)))
			rows := make([][]string, 0, len(arts))
			for _, a := range arts {
				rows = append(rows, []string{
					a.ID,
					a.URL,
					abbreviateSHA(a.SHA256),
					fmt.Sprintf("%d", a.SizeBytes),
					a.RetrievedAt.Format("2006-01-02 15:04:05"),
					a.Provenance,
				})
			}
			formatter.Table([]string{"ID", "URL", "SHA256", "Tamanho", "Recuperado em", "Proveniência"}, rows)
			return nil
		},
	}
}

// NewEvidenceShowCommand cria `cosca evidence show <id>`.
func NewEvidenceShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Mostra o detalhe completo de um artefato adquirido",
		Long: `Mostra o detalhe completo do artefato A-XXXX: URL, content-type,
SHA-256, tamanho, data de recuperação, proveniência (UNTRUSTED) e o caminho do
corpo (fora do conhecimento).`,
		Example: `  cosca evidence show A-0001
  cosca evidence show A-0001 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			artID, err := acquisition.NormalizeArtifactID(args[0])
			if err != nil {
				return err
			}
			art, err := resolveArtifactStore().Get(artID)
			if err != nil {
				return err
			}
			if useJSON {
				return printJSON(cmd, art)
			}
			formatter.Header(fmt.Sprintf("Artefato %s", art.ID))
			formatter.KeyValue("URL", art.URL)
			formatter.KeyValue("Content-Type", art.ContentType)
			formatter.KeyValue("SHA256", art.SHA256)
			formatter.KeyValue("Tamanho", fmt.Sprintf("%d bytes", art.SizeBytes))
			formatter.KeyValue("Recuperado em", art.RetrievedAt.Format(time.RFC3339))
			formatter.KeyValue("Proveniência", art.Provenance)
			formatter.KeyValue("Corpo", filepath.Join(".cosca", filepath.FromSlash(acquisition.ArtifactsDir), art.ID))
			if strings.TrimSpace(art.Notes) != "" {
				formatter.Header("Trecho (normalização)")
				formatter.Bullet(art.Notes)
			}
			return nil
		},
	}
}

// NewEvidencePromoteCommand cria `cosca evidence promote <id> --item K-XX`.
func NewEvidencePromoteCommand() *cobra.Command {
	var item string

	cmd := &cobra.Command{
		Use:   "promote <id>",
		Short: "Promoção manual da evidência externa para um item do CKL (exige --item)",
		Long: `Promoção MANUAL da evidência externa A-XXXX: marca a proposal Q-XXXX
como promoted (PromotedTo = item) e adiciona a evidência ao item do CKL com
proveniência P2 (repositório identificável) ou P4 (arquivo-fonte foi buscado),
SHA256 do artefato e Retrieved do artefato.

EXIGE --item explícito: sem ele, o comando RECUSA — evidência externa nunca é
auto-promovida. O corpo do artefato permanece em quarentena
(.cosca/quarantine/artifacts/A-XXXX); apenas a EVIDÊNCIA (referência auditável)
entra no conhecimento.`,
		Example: `  cosca evidence promote A-0001 --item K-01`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			if strings.TrimSpace(item) == "" {
				return fmt.Errorf("promoção exige --item explícito (ex.: --item K-01) — nunca auto-promover evidência externa")
			}

			artID, err := acquisition.NormalizeArtifactID(args[0])
			if err != nil {
				return err
			}

			// Artefato deve existir.
			astore := resolveArtifactStore()
			art, err := astore.Get(artID)
			if err != nil {
				return err
			}

			// Proposal Q-XXXX correspondente deve existir na quarentena.
			qstore, err := resolveQuarantineStore()
			if err != nil {
				return err
			}
			qID, err := findQuarantineProposal(qstore, artID)
			if err != nil {
				return err
			}

			// Transição da máquina de estados: pending → validating → promoted.
			prop, err := qstore.Get(qID)
			if err != nil {
				return err
			}
			switch prop.Status {
			case quarantine.StatusPending:
				if err := qstore.SetStatus(qID, quarantine.StatusValidating); err != nil {
					return err
				}
			case quarantine.StatusDiscarded:
				return fmt.Errorf("proposal %s está descartada — não pode ser promovida", qID)
			case quarantine.StatusPromoted:
				return fmt.Errorf("proposal %s já foi promovida", qID)
			}
			if err := qstore.Promote(qID, item); err != nil {
				return err
			}

			// Item do CKL deve existir (promoção nunca cria item).
			lawsPath, err := resolveLawsPath()
			if err != nil {
				return err
			}
			engine, err := loadLawsEngine(lawsPath)
			if err != nil {
				return err
			}
			if _, ok := engine.Get(item); !ok {
				return fmt.Errorf("item %q não encontrado em %s (use \"cosca knowledge law list\" para ver os IDs)", item, lawsPath)
			}

			// Evidência auditável: repositório = host+path, SHA256/Retrieved do
			// artefato, proveniência P2 (ou P4 se um arquivo-fonte foi buscado).
			prov := provenanceForArtifact(art)
			ev := knowledge.Evidence{
				ID:          art.ID,
				Kind:        "external",
				Source:      "external:" + art.URL,
				Description: fmt.Sprintf("Evidência externa adquirida via cosca evidence — quarentena %s → %s (promoção manual)", art.ID, qID),
				Timestamp:   time.Now(),
				Provenance:  prov,
				Repository:  repositoryFromURL(art.URL),
				Path:        filepath.ToSlash(filepath.Join(".cosca", filepath.FromSlash(acquisition.ArtifactsDir), art.ID)),
				SHA256:      art.SHA256,
				Retrieved:   art.RetrievedAt,
			}
			itemOut, err := engine.AddEvidence(item, ev)
			if err != nil {
				return err
			}
			if err := engine.Save(lawsPath); err != nil {
				return fmt.Errorf("salvar %s: %w", lawsPath, err)
			}

			if useJSON {
				return printJSON(cmd, map[string]interface{}{
					"artifact":       art,
					"proposal":       qID,
					"proposalStatus": quarantine.StatusPromoted,
					"item":           itemOut,
					"evidence":       ev,
				})
			}

			formatter.Success(fmt.Sprintf(
				"Evidência %s promovida manualmente → item %s (proposal %s → %s, provenance %s)",
				art.ID, item, qID, quarantine.StatusPromoted, prov))
			formatter.KeyValue("Repository", ev.Repository)
			formatter.KeyValue("SHA256", art.SHA256)
			formatter.KeyValue("Retrieved", art.RetrievedAt.Format(time.RFC3339))
			formatter.Warning("Corpo permanece em quarentena (.cosca/quarantine/artifacts/" + art.ID + ") — não executado.")
			return nil
		},
	}

	cmd.Flags().StringVar(&item, "item", "", "ID do item de conhecimento (ex.: K-01) — obrigatório; nunca auto-promover")
	_ = cmd.MarkFlagRequired("item")
	return cmd
}

// findQuarantineProposal localiza a proposal Q-XXXX que referencia o artefato
// A-XXXX (criada por QuarantineArtifact com a linha "Artifact: A-XXXX" no
// Content).
func findQuarantineProposal(store *quarantine.Store, artID string) (string, error) {
	props, err := store.List()
	if err != nil {
		return "", err
	}
	marker := "Artifact: " + artID
	for _, p := range props {
		if strings.Contains(p.Content, marker) {
			return p.ID, nil
		}
	}
	return "", fmt.Errorf("nenhuma proposal na quarentena referencia %s — rode \"cosca evidence fetch\" primeiro", artID)
}

// repositoryFromURL devolve o "repositório identificável" da evidência: o
// host+path da URL (ex.: github.com/CoscaAI/cosca/blob/main/README.md).
func repositoryFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return u.Host + u.Path
}

// provenanceForArtifact decide a proveniência da evidência promovida:
// P2 (repositório identificável) para páginas/repositórios; P4
// (código/teste reproduzível) quando um arquivo-fonte específico foi buscado
// (o nome base da URL tem extensão). SHA256 + corpo em quarentena permitem
// verificar "foi exatamente este conteúdo".
func provenanceForArtifact(art *acquisition.AcquiredArtifact) knowledge.ProvenanceLevel {
	u, err := url.Parse(art.URL)
	if err != nil {
		return knowledge.ProvenanceIdentifiableRepo
	}
	base := path.Base(u.Path)
	if base != "." && base != "/" && base != "" && strings.Contains(base, ".") {
		return knowledge.ProvenanceReproducible
	}
	return knowledge.ProvenanceIdentifiableRepo
}
