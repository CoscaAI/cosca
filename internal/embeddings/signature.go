package embeddings

import (
	"strconv"
	"strings"
)

// Este arquivo implementa o C2/invariante de espaço único de embedding
// (ADR-011 §3.4): uma assinatura canônica "model:dim" que identifica o espaço
// vetorial de um conjunto de embeddings. Serve para (1) chavear o cache de
// embedding por `sig|text` (evitando servir embedding de outro modelo no mesmo
// cache) e (2) quarentenar vetores persistidos cuja assinatura diverge da
// ativa (reuso vetado — como o `_load_prior_vector_map` do C2).
//
// É puramente ADITIVO: uma assinatura vazia (model ou dim ausente) significa
// "sem invariante" — o modo atual do texto, sem mudança de comportamento.

// Signature devolve a assinatura canônica "model:dim". Uma assinatura vazia
// (model == "" ou dim == "") devolve "" — o modo atual (sem invariante).
//
// Exemplo: Signature("nomic-embed-text", "768") → "nomic-embed-text:768".
func Signature(model string, dim string) string {
	model = strings.TrimSpace(model)
	dim = strings.TrimSpace(dim)
	if model == "" || dim == "" {
		return ""
	}
	return model + ":" + dim
}

// SignatureInt é a variante conveniente que aceita a dimensão como inteiro,
// convertendo internamente via Signature(model, strconv.Itoa(dim)).
func SignatureInt(model string, dim int) string {
	if dim <= 0 {
		return ""
	}
	return Signature(model, strconv.Itoa(dim))
}

// FromProvider deriva a assinatura da interface Provider (Model + Dimensions),
// que é exatamente como o registry expõe o embedding ativo.
func FromProvider(p Provider) string {
	if p == nil {
		return ""
	}
	return SignatureInt(p.Model(), p.Dimensions())
}

// GetArtifactSignature devolve a assinatura de um "artefato" de embedding —
// um par model+dim. Mantém o contrato do ADR (§3.4) de registrar a assinatura
// em VectorRecord/cache; as variantes int e string convergem para o mesmo
// formato canônico.
func GetArtifactSignature(model string, dim int) string {
	return SignatureInt(model, dim)
}

// EmbeddingSpaceInvariant decide se um artefato persistido pode ser reusado
// dado o embedding ativo: a assinatura persistida DEVE ser igual à ativa para
// reusar. Uma das duas vazia (ou ambas) significa "sem invariante" e o reuso é
// permitido (modo atual). É o filtro de quarentena: divergência de assinatura
// ⇒ mismatch ⇒ reuso vetado (e o chamador deve re-embedd).
func EmbeddingSpaceInvariant(persisted, active string) bool {
	persisted = strings.TrimSpace(persisted)
	active = strings.TrimSpace(active)
	if persisted == "" || active == "" {
		return true // sem invariante — modo atual, reuso ok
	}
	return persisted == active
}
