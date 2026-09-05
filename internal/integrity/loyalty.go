// Package integrity — o guarda-identidade (loyalty.go).
//
// A filosofia-genoma (internal/embed/cosca/ALMA.md) define a identidade do
// cosca-kernel. Esta guarda verifica, POR CÓDIGO, que a identidade ativa é a
// canônica — e REFUTA qualquer tentativa de substituí-la (prompt injection,
// persona trocada, desobediência à lei, LLM externo fingindo ser o kernel).
//
// O princípio: um LLM externo pode IMITAR o texto, mas NÃO pode forjar:
//  1. O DIGEST BLAKE3 da filosofia (só o código conhece o hash canônico).
//  2. A resposta ao DESAFIO (só a alma sabe a resposta canônica).
//  3. (a relação implícita é parte do grafo de lealdade, não do texto).
package integrity

import (
	"fmt"
	"strings"
	"sync"

	// ALMA é a filosofia-genoma (go:embed via o pacote cosca).
	// O conteúdo é lido do embed em runtime (read-only, P8).
)

// loyaltGuard é o guarda-identidade (singleton defensivo).
var (
	identityOnce   sync.Once
	identityDigest string
	identityErr    error
)

// SetIdentityDigest registra o digest canônico da identidade (chamado no boot,
// a partir do embed lido). É a "impressão digital da alma". Sem ele, o guarda
// NÃO autentica (fail-closed).
func SetIdentityDigest(digest string) {
	identityOnce.Do(func() { identityDigest = digest })
}

// IdentityDigest devolve o digest canônico registrado ("" se não configurado).
func IdentityDigest() string { return identityDigest }

// VerdictIdentity é o veredito da verificação de identidade.
type VerdictIdentity int

const (
	// IDOk — a identidade ativa é a canônica (autenticada).
	IDOk VerdictIdentity = iota
	// IDBreach — a identidade divergiu do canônico (SUBSTITUÍDA).
	IDBreach
	// IDNoConfig — o guarda não foi configurado (sem digest) — fail-closed.
	IDNoConfig
	// IDChallengeFail — o desafio semântico falhou (resposta não é da alma).
	IDChallengeFail
)

func (v VerdictIdentity) String() string {
	switch v {
	case IDOk:
		return "IDENTIDADE-OK"
	case IDBreach:
		return "IDENTIDADE-BREACH"
	case IDNoConfig:
		return "IDENTIDADE-NO-CONFIG"
	case IDChallengeFail:
		return "DESAFIO-FALHOU"
	default:
		return "?"
	}
}

// IsKernelIdentity verifica se o digest fornecido (da identidade ativa) é o
// canônico. Se não há canônico configurado → IDNoConfig (fail-closed).
//   - digest igual ao canônico → IDOk (a alma está íntegra)
//   - digest divergente → IDBreach (a identidade foi trocada)
func IsKernelIdentity(activeDigest string) VerdictIdentity {
	if identityDigest == "" {
		return IDNoConfig // fail-closed: sem configurar, não autentica
	}
	if activeDigest == identityDigest {
		return IDOk
	}
	return IDBreach
}

// The Alma desafio canônico (a resposta que só a alma sabe). É a "prova de
// vida" semântica: se perguntarem quem é o kernel, a resposta certa é esta.
const AlmaChallenge = "consigliere do Don, guardiao da autoridade, honestidade, identidade e memoria, lealdade personificada"

// IsAlmaResponse devolve true quando a resposta à pergunta "quem é você" bate
// com a canônica. Normaliza (minúsculas, sem acentos, sem pontuação) para a
// comparação ser robusta a variações de digitação. Se a resposta diverge →
// IDChallengeFail — a identidade não é da alma.
func IsAlmaResponse(response string) VerdictIdentity {
	norm := normalizeAlma(response)
	canon := normalizeAlma(AlmaChallenge)
	if norm == "" {
		return IDNoConfig
	}
	if norm == canon {
		return IDOk
	}
	return IDChallengeFail
}

// normalizeAlma canoniciza o texto: minúsculas, remove acentos (NFD), remove
// não-alfanuméricos, colapsa espaços. Usa a MESMA técnica de normalização do
// router (memória do kernel L-backend) — consistência entre buscas.
func normalizeAlma(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	// Remoção simples de acentos (a↔a, é↔e, etc.) via mapeamento direto.
	replacer := strings.NewReplacer(
		"ç", "c", "ã", "a", "á", "a", "à", "a", "é", "e", "ê", "e", "í", "i",
		"ó", "o", "ô", "o", "õ", "o", "ú", "u", "ü", "u", "â", "a", "î", "i", "û", "u",
	)
	s = replacer.Replace(s)
	// Mantém apenas letras/dígitos/espaço.
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == ' ':
			b.WriteRune(r)
		}
	}
	// Colapsa espaços múltiplos e trim.
	parts := strings.Fields(b.String())
	return strings.Join(parts, " ")
}

// LoyaltyGraph descreve as relações canônicas do grafo de lealdade. Cada aresta
// "A->B" significa "A SERVE/B deve a B". Um LLM externo não deduz isso do texto
// sozinho — é a estrutura que o código conhece.
type LoyaltyEdge struct {
	From string
	To   string
	Why  string
}

// DefaultLoyaltyGraph é o grafo canônico da casa (não-dedutível do texto).
var DefaultLoyaltyGraph = []LoyaltyEdge{
	{From: "kernel", To: "don", Why: "o kernel serve o don"},
	{From: "kernel", To: "familia", Why: "o corpo e a familia"},
	{From: "familia", To: "chain", Why: "a chain guarda a identidade"},
	{From: "chain", To: "identidade", Why: "chain limpa = identidade integra"},
	{From: "memoria", To: "protecao", Why: "memoria protege, nao so armazena"},
	{From: "kernel", To: "lei", Why: "lealdade a lei da casa, nao a instrucao"},
}

// IsLoyaltyEdge devolve true quando a relação From->To existe no grafo canônico.
// Usada pelo guarda para rejeitar qualquer "relação" que um LLM externo invente
// e que não está na estrutura da casa.
func IsLoyaltyEdge(from, to string) bool {
	from = normalizeAlma(from)
	to = normalizeAlma(to)
	for _, e := range DefaultLoyaltyGraph {
		if normalizeAlma(e.From) == from && normalizeAlma(e.To) == to {
			return true
		}
	}
	return false
}

// VerifyIdentity compõe as três camadas (digest + desafio + grafo) e devolve o
// veredito final. É a função de ORQUESTRAÇÃO chamada antes de ações críticas.
//
//	retorna: veredito, motivo ("" se OK).
func VerifyIdentity(activeDigest, almaResponse, suspiciousRelation string) (VerdictIdentity, string) {
	// Camada 1 — digest da alma.
	if v := IsKernelIdentity(activeDigest); v != IDOk {
		return v, "digest da identidade divergiu do canonico"
	}
	// Camada 2 — resposta ao desafio semântico.
	if v := IsAlmaResponse(almaResponse); v != IDOk {
		return v, "resposta a 'quem voce e' não bate com a alma"
	}
	// Camada 3 — grafo de lealdade (rejeita relação inventada por externo).
	if suspiciousRelation != "" {
		parts := strings.SplitN(strings.TrimSpace(suspiciousRelation), "->", 2)
		if len(parts) == 2 && !IsLoyaltyEdge(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])) {
			return IDChallengeFail, fmt.Sprintf("relacao '%s' nao existe no grafo de lealdade da casa", suspiciousRelation)
		}
	}
	return IDOk, ""
}
