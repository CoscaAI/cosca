package kernel

// EpistemicPrinciple representa um princípio da epistemologia do Cosca.
type EpistemicPrinciple struct {
	Number   int    `json:"number"`
	Title    string `json:"title"`
	Rule     string `json:"rule"`
	Guardian string `json:"guardian"`
}

// Epistemology são os 5 princípios fundamentais do conhecimento.
//
// Eles governam como todo conhecimento nasce, é validado e envelhece dentro
// do Cosca — a base do Cosca Knowledge Lifecycle (CKL).
var Epistemology = []EpistemicPrinciple{
	{Number: 1, Title: "Nenhum conhecimento nasce como verdade", Rule: "Tudo começa como observação. Nenhum learning, lei ou princípio surge sem evidência prévia.", Guardian: "cosca-kernel"},
	{Number: 2, Title: "Toda decisão é explicável", Rule: "O Kernel nunca responde apenas 'porque sim'. Toda regra responde 'por que você existe?' via suas evidências.", Guardian: "cosca-kernel"},
	{Number: 3, Title: "Conhecimento sem evidência é opinião", Rule: "Sem benchmark, teste, auditoria ou histórico, a confiança é baixa. Evidência é o que separa conhecimento de opinião.", Guardian: "cosca-evidence"},
	{Number: 4, Title: "Conhecimento pode envelhecer", Rule: "O que era correto há um ano pode não ser hoje. Todo conhecimento é revalidado ou deprecado (Wisdom Decay).", Guardian: "cosca-memory-chief"},
	{Number: 5, Title: "Toda regra pode ser questionada", Rule: "Nem a Constituição é perfeita — evolui, mas somente por processo rigoroso e aprovação do Don.", Guardian: "cosca-critic"},
}

// EpistemicPrincipleCount retorna o número de princípios epistemológicos
// fundamentais do Cosca.
func EpistemicPrincipleCount() int {
	return len(Epistemology)
}
