package knowledge

// KnowledgeEpistemic é a CLASSE EPISTÊMICA de um ITEM DE CONHECIMENTO
// armazenado — a natureza do conhecimento: é um fato? uma medição? uma
// inferência? uma regra? uma decisão? um perfil?
//
// É uma dimensão ORTOGONAL ao EpistemicStatus (epistemic.go): `status` é a
// MATURIDADE/confiança ao longo do tempo (KNOWN, SUPPORTED, UNCERTAIN,
// CONFLICTING, UNKNOWN, STALE); `class` é a NATUREZA do que se sabe. Um
// item pode ser um FACT com status STALE (era fato, ficou desatualizado), ou
// um INFERRED com status KNOWN.
//
// Importante NÃO confundir com ClaimKind (thinking.go): ClaimKind classifica
// AFIRMAÇÕES feitas DURANTE o raciocínio (dinâmico). KnowledgeEpistemic
// classifica o CONHECIMENTO ARMAZENADO (estático). Por isso são tipos
// distintos — misturá-los corromperia o IsTrustworthy de claims.
//
// Semântica de confiança (a régua que impede INFERRED de virar FACT):
//   - FACT / MEASURED  → podem ser apresentados como fato (medido com incerteza).
//   - EVIDENCE        → observação registrada, ainda não validada (provisório).
//   - INFERRED        → derivado de outro conhecimento; NUNCA apresentado como
//     fato — a regra de ouro da Fase 4.
//   - RULE / DECISION → autoritativo por natureza (normativo / escolha), mas
//     NÃO são medição-fato: apresentados com seu próprio rótulo.
//   - PROFILE         → capacidade/atributo declarado.
type KnowledgeEpistemic string

const (
	// EpistemicFACT é estabelecido com evidência validada. "É assim."
	EpistemicFACT KnowledgeEpistemic = "FACT"
	// EpistemicMEASURED é medido com incerteza (benchmark/medida). Nunca vira
	// FACT — a medição é uma observação quantitativa com margem.
	EpistemicMEASURED KnowledgeEpistemic = "MEASURED"
	// EpistemicEVIDENCE é uma observação registrada, ainda não validada.
	EpistemicEVIDENCE KnowledgeEpistemic = "EVIDENCE"
	// EpistemicINFERRED é derivado de outro conhecimento. Nunca apresentado
	// como fato.
	EpistemicINFERRED KnowledgeEpistemic = "INFERRED"
	// EpistemicRULE é uma regra normativa: "deve ser".
	EpistemicRULE KnowledgeEpistemic = "RULE"
	// EpistemicDECISION é uma decisão registrada: "escolhemos X".
	EpistemicDECISION KnowledgeEpistemic = "DECISION"
	// EpistemicPROFILE é uma capacidade/atributo declarado.
	EpistemicPROFILE KnowledgeEpistemic = "PROFILE"
)

// Valid devolve true quando a classe é uma das sete do vocabulário.
func (e KnowledgeEpistemic) Valid() bool {
	switch e {
	case EpistemicFACT, EpistemicMEASURED, EpistemicEVIDENCE,
		EpistemicINFERRED, EpistemicRULE, EpistemicDECISION, EpistemicPROFILE:
		return true
	default:
		return false
	}
}

// Trustworthy devolve true quando a classe pode ser apresentada como fato
// estabelecido (FACT) ou como medição (MEASURED). EVIDENCE/INFERRED são
// provisórios (não confiáveis como fato); RULE/DECISION/PROFILE são
// autoritativos na sua própria natureza, mas NÃO são medição-fato.
func (e KnowledgeEpistemic) Trustworthy() bool {
	switch e {
	case EpistemicFACT, EpistemicMEASURED:
		return true
	default:
		return false
	}
}

// Label devolve o prefixo de exibição do item no contexto do agente —
// a forma como a classe é apresentada (ex.: "[FACT]", "[INFERRED]").
func (e KnowledgeEpistemic) Label() string {
	if !e.Valid() {
		return "[UNKNOWN]"
	}
	return "[" + string(e) + "]"
}

// epistemicForKinds mapeia a categoria de conteúdo (kind, da Fase 2) para uma
// classe epistêmica determinística. Quando o autor não declara a classe, esta
// é a inferência default por tipo de conteúdo.
var epistemicForKinds = map[string]KnowledgeEpistemic{
	"decision":     EpistemicDECISION,
	"adr":          EpistemicDECISION,
	"rule":         EpistemicRULE,
	"architecture": EpistemicDECISION, // decisão arquitetural
	"profile":      EpistemicPROFILE,
	"capability":   EpistemicPROFILE,
	"learning":     EpistemicINFERRED, // aprendizado consolidado = derivação
	"pattern":      EpistemicINFERRED,
	"technique":    EpistemicINFERRED,
}

// EpistemicFor devolve a classe epistêmica de um item pelo seu kind (categoria
// de conteúdo da Fase 2). Não possui mapeamento → default EVIDENCE (observação
// registrada, a classe mais honesta quando não sabemos se foi validada).
func EpistemicFor(kind string) KnowledgeEpistemic {
	if e, ok := epistemicForKinds[kind]; ok {
		return e
	}
	return EpistemicEVIDENCE
}
