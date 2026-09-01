package engine

// concatEpistemic prefixa o conteúdo de um resultado de conhecimento com a sua
// classe epistêmica (FASE 4) — ex.: "[INFERRED] ...", "[FACT] ...". Assim o
// agente vê a NATUREZA do conhecimento no contexto, e um item INFERRED nunca é
// lido como fato. Sem classe → conteúdo puro (compatível).
//
// Função de APRESENTAÇÃO do AgentEngine (não é adapter) — o contrato
// KnowledgeSearchResult vem de orchestration (porta única).
func concatEpistemic(r KnowledgeSearchResult) string {
	if r.Epistemic == "" {
		return r.Content
	}
	return "[" + r.Epistemic + "] " + r.Content
}
