package acquisition

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/quarantine"
)

// QuarantineArtifact registra o artefato adquirido na zona de quarentena
// epistemológica: cria uma Proposal Q-XXXX (status pending) cujo Content é
// apenas uma REFERÊNCIA — A-XXXX + URL + SHA256 + um curto trecho (a
// normalização via Notes) — NUNCA o dump completo do corpo. O corpo do
// artefato vive separado em .cosca/quarantine/artifacts/A-XXXX — jamais no
// conhecimento.
//
// Source = "external:<url>" marca a proposal como evidência externa
// (EXTERNAL DATA ≠ TRUSTED DATA). Nada aqui é promovido automaticamente.
// Retorna o ID da proposal criada (Q-XXXX).
func QuarantineArtifact(store *quarantine.Store, art *AcquiredArtifact, reason string) (string, error) {
	if store == nil {
		return "", fmt.Errorf("acquisition: store de quarentena nil")
	}
	if art == nil {
		return "", fmt.Errorf("acquisition: artefato nil")
	}
	if strings.TrimSpace(art.ID) == "" {
		return "", fmt.Errorf("acquisition: artefato sem ID — persista-o antes de quarentenar")
	}

	host := ""
	if u, err := url.Parse(art.URL); err == nil {
		host = u.Host
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Artefato externo adquirido — EVIDÊNCIA EXTERNA ≠ DADOS CONFIÁVEIS.\n")
	fmt.Fprintf(&b, "Proveniência: %s — em quarentena, NADA é promovido automaticamente.\n", art.Provenance)
	if strings.TrimSpace(reason) != "" {
		fmt.Fprintf(&b, "Motivo: %s\n", reason)
	}
	fmt.Fprintf(&b, "Artifact: %s\n", art.ID)
	fmt.Fprintf(&b, "URL: %s\n", art.URL)
	fmt.Fprintf(&b, "SHA256: %s\n", art.SHA256)
	fmt.Fprintf(&b, "Tamanho: %d bytes\n", art.SizeBytes)
	if strings.TrimSpace(art.ContentType) != "" {
		fmt.Fprintf(&b, "Content-Type: %s\n", art.ContentType)
	}
	fmt.Fprintf(&b, "Recuperado em: %s\n", art.RetrievedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "Corpo completo: .cosca/quarantine/artifacts/%s (fora do conhecimento)\n", art.ID)
	if strings.TrimSpace(art.Notes) != "" {
		fmt.Fprintf(&b, "Trecho: %s\n", art.Notes)
	}

	title := fmt.Sprintf("Evidência externa %s", art.ID)
	if host != "" {
		title += " — " + host
	}

	id, err := store.Add(quarantine.Proposal{
		Title:   title,
		Content: b.String(),
		Source:  "external:" + art.URL,
	})
	if err != nil {
		return "", fmt.Errorf("acquisition: quarentenar %s: %w", art.ID, err)
	}
	return id, nil
}
