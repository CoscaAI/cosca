package grounding

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Qrels é o "gabarito" de uma única query de baseline: quais chunks são
// relevantes e qual é o chunk que deve aparecer na primeira posição. É o
// contrato congelado contra o qual o RecallGate mede não-regressão de recall.
type Qrels struct {
	// Query é a pergunta autoral (a mesmíssima passada ao retriever).
	Query string `json:"query"`
	// RelevantChunkIDs são os chunk_ids considerados relevantes (gold).
	RelevantChunkIDs []string `json:"relevant_chunk_ids"`
	// FirstRelevantChunkID é o chunk que DEVE estar no topo (expected top-1).
	FirstRelevantChunkID string `json:"first_relevant_chunk_id"`
}

// QrelsFile é o layout no disco de um arquivo qrels-baseline.json. Suporta
// um schema com version/embedding (paridade com o ADR §3.4) e a lista de
// queries.
type QrelsFile struct {
	// Version é o schema do arquivo (1).
	Version int `json:"version"`
	// Embedding registra a assinatura model:dim do baseline (C2). Sempre que
	// o modelo de embedding mudar, o baseline deve ser regenerado (--generate).
	Embedding EmbeddingSig `json:"embedding,omitempty"`
	// Queries é a lista de gabaritos.
	Queries []Qrels `json:"queries"`
}

// EmbeddingSig é a assinatura de espaço de embedding (model:dim) do baseline.
type EmbeddingSig struct {
	Model string `json:"model"`
	Dim   string `json:"dim"`
}

// Signature devolve a assinatura canônica "model:dim" do baseline.
func (e EmbeddingSig) Signature() string {
	if e.Model == "" || e.Dim == "" {
		return ""
	}
	return e.Model + ":" + e.Dim
}

// LoadQrels lê um arquivo qrels-baseline.json e devolve a lista de Qrels.
// Aceita tanto o layout envolto ({version, embedding, queries:[...]}) quanto
// um array puro ([...]). Depois de ler, chama ValidateQrels; um Qrels inválido
// é um erro (fail-closed — baseline corrompido não pode passar o gate).
func LoadQrels(path string) ([]Qrels, error) {
	file, err := LoadQrelsFile(path)
	if err != nil {
		return nil, err
	}
	return file.Queries, nil
}

// LoadQrelsFile lê um arquivo qrels-baseline.json e devolve o QrelsFile completo
// (layout envolto com version/embedding), preservando a assinatura de embedding
// do baseline. Aceita tanto o layout envolto quanto o array puro (neste caso o
// QrelsFile devolvido tem Version=1 e Embedding vazio). Valida (ValidateQrels)
// antes de devolver. É a forma de expor o `embedding.model:dim` para a
// persistência de observabilidade da campanha.
func LoadQrelsFile(path string) (*QrelsFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("grounding: load qrels %s: %w", path, err)
	}

	// Tentar primeiro o array puro.
	var bare []Qrels
	if err := json.Unmarshal(data, &bare); err == nil {
		if err := ValidateQrels(bare); err != nil {
			return nil, fmt.Errorf("grounding: qrels %s: %w", path, err)
		}
		return &QrelsFile{Version: 1, Queries: bare}, nil
	}

	// Senão o layout envolto.
	var file QrelsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("grounding: parse qrels %s: %w", path, err)
	}
	if err := ValidateQrels(file.Queries); err != nil {
		return nil, fmt.Errorf("grounding: qrels %s: %w", path, err)
	}
	return &file, nil
}

// ValidateQrels valida a lista de gabaritos. Regras (fail-closed):
//   - a lista não pode ser vazia;
//   - cada Query não pode ser vazia;
//   - cada RelevantChunkIDs deve ter ao menos 1 elemento (queryset vazio
//     torna recall indefinido — e seria uma query "fácil de descartar" para
//     inflar a nota, exatamente o que o ADR §6 pune);
//   - cada ChunkID não pode ser vazio;
//   - o FirstRelevantChunkID, quando presente, deve estar em RelevantChunkIDs.
//
// Returns um erro com o índice do primeiro gabarito inválido.
func ValidateQrels(qrels []Qrels) error {
	if len(qrels) == 0 {
		return fmt.Errorf("grounding: qrels vazio — não há gabarito para o gate")
	}
	for i, q := range qrels {
		if strings.TrimSpace(q.Query) == "" {
			return fmt.Errorf("grounding: qrels[%d] com query vazia", i)
		}
		if len(q.RelevantChunkIDs) == 0 {
			return fmt.Errorf("grounding: qrels[%d] (%q) sem relevant_chunk_ids", i, q.Query)
		}
		for j, id := range q.RelevantChunkIDs {
			if strings.TrimSpace(id) == "" {
				return fmt.Errorf("grounding: qrels[%d] (%q) relevant_chunk_ids[%d] vazio", i, q.Query, j)
			}
		}
		if q.FirstRelevantChunkID != "" {
			found := false
			for _, id := range q.RelevantChunkIDs {
				if id == q.FirstRelevantChunkID {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("grounding: qrels[%d] (%q) first_relevant_chunk_id %q não está em relevant_chunk_ids", i, q.Query, q.FirstRelevantChunkID)
			}
		}
	}
	return nil
}
