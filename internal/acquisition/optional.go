// optional.go — enriquecimento opcional resiliente (nunca derruba o pipeline).
//
// PRINCÍPIO minerado (Osiris, MIT — src/lib/httpJson.ts) reimplementado segundo
// os contratos do COSCA. Ver ADR-042.
//
// Uma chamada de enriquecimento de baixa prioridade (dado complementar, não
// crítico) pode falhar por qualquer motivo transitório. Optional resolve o erro
// para um valor zero + ok=false, localizando a robustez — o que é OPORTUNO não
// se propaga como erro fatal. Não use no caminho crítico (o obrigatório não
// passa por aqui, para não mascarar bug real).
package acquisition

// Optional executa fn e devolve o valor se não houver erro; em erro devolve o
// zero de T e ok=false. Para enriquecimento opcional que não pode derrubar o
// pipeline. O retorno ok=false é distinto de "zero legítimo": o chamador
// distingue "não está disponível" de "está vazio".
func Optional[T any](fn func() (T, error)) (T, bool) {
	v, err := fn()
	if err != nil {
		var zero T
		return zero, false
	}
	return v, true
}
