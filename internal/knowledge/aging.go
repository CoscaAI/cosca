//
// Envelhecimento do conhecimento via hash — revalidação determinística.
//
// Conforme a conversa do Don: "Você pode descobrir quando conhecimento
// envelheceu. O Cosca aprendeu K-442 'API X funciona dessa maneira'. Meses
// depois, GitHub → nova release → arquivo alterado → hash diferente. O
// sistema detecta: K-442 STATUS: STALE."
//
// O mecanismo é local e determinístico: compara o SHA-256 corrente do arquivo
// em Path contra o SHA256 registrado na evidência (P4/P5). Se diferem → a
// fonte mudou → STALE. O envelhecimento NUNCA apaga nem promove/demote nível
// — apenas marca o estado epistemológico STALE: o sistema sabe quando NÃO
// sabe ("Esse conhecimento precisa ser revalidado.").
//

package knowledge

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AgingResult é o resultado da revalidação de uma única evidência: o hash
// registrado vs. o hash corrente do arquivo local, e se a fonte mudou.
type AgingResult struct {
	ItemID       string    `json:"item_id"`
	EvidenceID   string    `json:"evidence_id"`
	Path         string    `json:"path"`
	RecordedHash string    `json:"recorded_hash"`
	CurrentHash  string    `json:"current_hash"`
	Changed      bool      `json:"changed"`
	Skipped      bool      `json:"skipped,omitempty"`
	LastVerified time.Time `json:"last_verified,omitempty"`
}

// HashFile computa o SHA-256 do conteúdo do arquivo em path. Arquivo
// inexistente (ou ilegível) → hash vazio + erro.
func HashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// RevalidateEvidence re-hash o arquivo local em ev.Path e compara com o
// ev.SHA256 registrado. Evidência sem SHA256 ou sem Path é pulada
// (Skipped=true, Changed=false) SEM erro — a revalidação local não tem como
// operar sem a referência de fonte. Se o arquivo local não existe,
// HashFile devolve erro (propagado: a fonte sumiu é informação relevante).
func RevalidateEvidence(item *KnowledgeItem, ev *Evidence, coscaDir string) (AgingResult, error) {
	res := AgingResult{
		ItemID:       item.ID,
		EvidenceID:   ev.ID,
		Path:         ev.Path,
		RecordedHash: ev.SHA256,
		LastVerified: item.LastVerified,
	}
	if strings.TrimSpace(ev.SHA256) == "" || strings.TrimSpace(ev.Path) == "" {
		res.Skipped = true
		return res, nil
	}

	p := ev.Path
	if !filepath.IsAbs(p) && coscaDir != "" {
		p = filepath.Join(coscaDir, p)
	}
	cur, err := HashFile(p)
	if err != nil {
		return res, fmt.Errorf("knowledge: revalidate evidência %s (%s): %w", ev.ID, p, err)
	}
	res.CurrentHash = cur
	res.Changed = cur != ev.SHA256
	return res, nil
}

// RevalidateItem revalida todas as evidências de um item, na ordem em que
// aparecem. O primeiro erro (ex.: arquivo de origem desaparecido) interrompe
// a revalidação do item.
func RevalidateItem(item *KnowledgeItem, coscaDir string) ([]AgingResult, error) {
	if item == nil {
		return nil, errors.New("knowledge: cannot revalidate a nil item")
	}
	results := make([]AgingResult, 0, len(item.Evidence))
	for i := range item.Evidence {
		res, err := RevalidateEvidence(item, &item.Evidence[i], coscaDir)
		if err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	return results, nil
}

// ApplyAging aplica o resultado da revalidação ao item. Toda evidência com
// Changed=true marca o item como STALE. NUNCA apaga nada e NUNCA promove ou
// demote nível — apenas o estado epistemológico (CKL). Se ao menos uma
// evidência foi de fato verificada (não pulada), LastVerified é atualizado
// para agora (refresh). Retorna true se algum status mudou para STALE.
func ApplyAging(item *KnowledgeItem, results []AgingResult) (changed bool) {
	if item == nil {
		return false
	}
	verified := false
	for _, res := range results {
		if res.Skipped {
			continue
		}
		verified = true
		if res.Changed {
			item.Status = StatusStale
			changed = true
		}
	}
	if changed || verified {
		item.LastVerified = time.Now()
	}
	return changed
}
