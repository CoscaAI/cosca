package engine

// pending_helpers.go — Pending Resolution helpers (Don + professor, 2026-09-01).
// Projeções do estado do AgentEngine para a inspeção de pendências.

// engineObservations projeta as observações do loop do engine (o último
// conteúdo gerado) para a inspeção de pendências — a evidência do estado que
// decide se a continuação é resolvível. Sem conteúdo, a observação é vazia
// (a inspeção decide honestamente).
func engineObservations(lastContent string) []string {
	if lastContent == "" {
		return nil
	}
	return []string{"last_content: " + lastContent}
}
