package oracle

import "strings"

// ─── O Pacote Semântico ────────────────────────────────────────────────────────
//
// O Cosca externo não entrega apenas um resultado (ORACLE_PROTOCOL §4). Ele
// entrega um PACOTE: intenção, resultado, fonte, proveniência, evidência,
// contexto, confiança. O Oráculo valida o pacote inteiro — nunca só a forma.

// Confidence é o grau de certeza declarado. O Oráculo NUNCA aceita um
// Confidence alto sem evidência que o sustente.
type Confidence string

const (
	ConfHigh   Confidence = "HIGH"
	ConfMedium Confidence = "MEDIUM"
	ConfLow    Confidence = "LOW"
)

// SemanticPackage é o que o Cosca externo entrega ao Oráculo.
type SemanticPackage struct {
	// Intent — a intenção original (o que se queria alcançar).
	Intent string `json:"intent"`
	// Result — o que foi encontrado/produzido.
	Result string `json:"result"`
	// Source — de onde veio.
	Source string `json:"source"`
	// Provenance — origem verificável (commit, hash, arquivo, URL).
	Provenance string `json:"provenance"`
	// Evidence — o que sustenta o resultado (comando, teste, medição).
	Evidence string `json:"evidence"`
	// Context — o cenário (produção, teste, investigação).
	Context string `json:"context"`
	// Actions — o que foi feito/alterado.
	Actions string `json:"actions"`
	// Changes — mudanças concretas (arquivos, configs).
	Changes string `json:"changes"`
	// Confidence — HIGH/MEDIUM/LOW declarado.
	Confidence Confidence `json:"confidence"`
}

// ─── Decisão do Oráculo ────────────────────────────────────────────────────────

// Decision é o veredito do Oráculo (ORACLE_PROTOCOL §14).
type Decision string

const (
	// Accept — contrato satisfeito + semântica coerente + proveniência +
	// evidência adequada.
	Accept Decision = "ACCEPT"
	// AcceptWithCaveat — resultado útil, mas com incerteza conhecida.
	AcceptWithCaveat Decision = "ACCEPT_WITH_CAVEAT"
	// Reject — contrato incompatível, semântica incompatível, ou
	// proveniência insuficiente para o risco.
	Reject Decision = "REJECT"
	// Inconclusive — não há evidência suficiente para decidir. NÃO é
	// falha: significa "ainda não sabemos".
	Inconclusive Decision = "INCONCLUSIVE"
)

// Verdict é a decisão + a justificativa estruturada (o feedback ao Cosca
// externo — §20: nunca responder só "invalid").
type Verdict struct {
	Decision Decision `json:"decision"`
	// ContractGaps são as lacunas de contrato (ex.: "evidência causal ausente").
	ContractGaps []string `json:"contract_gaps,omitempty"`
	// SemanticGaps são as lacunas de significado (ex.: "correlação ≠ causa").
	SemanticGaps []string `json:"semantic_gaps,omitempty"`
	// Required é o que falta para avançar (ex.: "teste reproduzível").
	Required []string `json:"required,omitempty"`
	// Questions são perguntas que o Oráculo faz ao Cosca externo (§19).
	Questions []string `json:"questions,omitempty"`
	// Reason é o resumo da decisão em linguagem natural.
	Reason string `json:"reason"`
}

// ─── O BLOQUEIO (o guard do Oráculo) ───────────────────────────────────────────
//
// O Don ordenou: "tem um bloqueio pra so permitir o que eh valido". O Oráculo
// NÃO valida pela palavra do Cosca externo — valida o PACOTE contra regras
// determinísticas (camada 1) e contra a memória assinada (camada 2, quando
// conectada). Fail-closed: o que não prova, não passa.

// Gate é o bloqueio determinístico do Oráculo. Camada 1: regras de contrato
// avaliadas por código, independentes do LLM (mesmo princípio do internal/policy).
type Gate struct {
	// MemoryCheck é uma função opcional que valida o resultado contra a
	// memória assinada (raízes verificadas, L418). Quando nil, o gate
	// opera apenas com as regras determinísticas de contrato.
	MemoryCheck func(pkg SemanticPackage) (contradiction string, ok bool)
}

// NewGate cria um bloqueio com as regras padrão.
func NewGate() *Gate { return &Gate{} }

// Evaluate aplica o bloqueio ao pacote semântico e produz a decisão.
//
// Regras (fail-closed — na dúvida, não passa):
//  1. Pacote vazio → REJECT (nada a validar).
//  2. Sem intenção → REJECT (o que se queria?).
//  3. Sem resultado → REJECT (o que foi encontrado?).
//  4. Sem proveniência → REJECT se for FACT/MEASURED; INCONCLUSIVE se o
//     resultado é exploratório (não se pode verificar de onde veio).
//  5. Confidence HIGH sem evidência → REJECT (promoção ilegítima).
//  6. Evidence vazia com Confidence HIGH → REJECT.
//  7. Contradição com a memória assinada → ACCEPT_WITH_CAVEAT com a
//     contradição listada (objeto de investigação, §16) — nunca escolher
//     um lado automaticamente.
func (g *Gate) Evaluate(pkg SemanticPackage) Verdict {
	// 1. Pacote vazio.
	if pkg.Intent == "" && pkg.Result == "" && pkg.Provenance == "" {
		return Verdict{
			Decision:     Reject,
			ContractGaps: []string{"pacote vazio — nada foi entregue para validação"},
			Reason:       "O Cosca externo não entregou um pacote semântico válido.",
		}
	}

	// 2. Sem intenção.
	if pkg.Intent == "" {
		return Verdict{
			Decision:     Reject,
			ContractGaps: []string{"intenção ausente"},
			Questions:    []string{"Qual era a intenção original desta investigação?"},
			Reason:       "Sem intenção não há como saber se o resultado satisfaz o que se queria.",
		}
	}

	// 3. Sem resultado.
	if pkg.Result == "" {
		return Verdict{
			Decision:     Reject,
			ContractGaps: []string{"resultado ausente"},
			Reason:       "O Cosca externo afirmou uma intenção mas não entregou resultado.",
		}
	}

	// 3.5 INCONCLUSIVE (quarta invariante, §28): o resultado existe, a
	// intenção existe, MAS não há evidência suficiente para julgar se o
	// resultado satisfaz a intenção. "Ainda não sabemos" — nunca inventar
	// conclusão. NÃO é REJECT: é falta de base para decidir.
	if pkg.Evidence == "" && pkg.Provenance == "" && pkg.Confidence == "" {
		return Verdict{
			Decision: Inconclusive,
			Questions: []string{
				"De onde veio esse resultado?",
				"Como você verificou?",
				"Isso é fato ou hipótese?",
				"Existe evidência independente?",
			},
			Required: []string{"proveniência verificável", "evidência (comando/teste/medição)", "confiança declarada"},
			Reason:   "Resultado apresentado sem evidência, sem proveniência e sem confiança — não há base para aceitar nem rejeitar (§14, §28).",
		}
	}

	// 4/5/6. Evidência vs Confiança.
	if pkg.Confidence == ConfHigh {
		if pkg.Evidence == "" {
			return Verdict{
				Decision:     Reject,
				ContractGaps: []string{"confiança HIGH sem evidência"},
				SemanticGaps: []string{"promoção ilegítima: hipótese → fato sem evidência (§8, §27)"},
				Required:     []string{"evidência mensurável (comando, teste, benchmark)"},
				Reason:       "Confiança HIGH exige evidência compatível com sua importância.",
			}
		}
		if pkg.Provenance == "" {
			return Verdict{
				Decision:     Reject,
				ContractGaps: []string{"confiança HIGH sem proveniência verificável"},
				SemanticGaps: []string{"resultado sem origem não pode ser FACT/MEASURED (§7)"},
				Required:     []string{"origem verificável (commit, hash, arquivo, URL)"},
				Reason:       "Resultado sem proveniência não sustenta confiança alta.",
			}
		}
	}

	// 7. Contradição com a memória assinada.
	if g.MemoryCheck != nil {
		if contradiction, ok := g.MemoryCheck(pkg); ok && contradiction != "" {
			return Verdict{
				Decision:     AcceptWithCaveat,
				SemanticGaps: []string{"contradição com a memória: " + contradiction},
				Questions: []string{
					"Qual versão/timestamp/commit sustenta o novo estado?",
					"Qual evidência independente confirma a mudança?",
				},
				Reason: "O resultado contradiz a memória assinada — vira objeto de investigação, não é aceito nem rejeitado cegamente (§16).",
			}
		}
	}

	// 7.5 RESULTADO ≠ VERDADE (§10): se o resultado afirma CAUSA
	// ("X causou Y") mas a evidência é só correlação/observação, o oráculo
	// não pode aceitar como FACT causal. O Cosca encontrou X — isso não
	// significa que X é a causa.
	if causalClaim(pkg.Result) && !causalEvidence(pkg.Evidence) {
		return Verdict{
			Decision:     AcceptWithCaveat,
			ContractGaps: []string{"evidência causal ausente"},
			SemanticGaps: []string{"o resultado demonstra correlação, mas não demonstra causa (§10)"},
			Required:     []string{"teste reproduzível", "evidência causal (antes/depois controlado, isolamento de variável)"},
			Reason:       "Encontrar X não significa que X é a causa. Correlação ≠ causa — evidência causal necessária antes de promover a afirmação.",
		}
	}

	// Caso nominal: contrato satisfeito.
	var caveats []string
	if pkg.Confidence == ConfLow || pkg.Confidence == "" {
		caveats = append(caveats, "confiança baixa ou não declarada")
	}
	if pkg.Evidence == "" {
		caveats = append(caveats, "sem evidência declarada — resultado aceito como exploração, não como FACT")
	}
	if len(caveats) > 0 {
		return Verdict{
			Decision:     AcceptWithCaveat,
			ContractGaps: caveats,
			Reason:       "Resultado útil com incertezas conhecidas (§14).",
		}
	}

	return Verdict{
		Decision: Accept,
		Reason:   "Contrato satisfeito. Semântica coerente. Proveniência e evidência adequadas.",
	}
}

// causalClaim reporta se o resultado afirma uma relação CAUSAL ("X causou Y",
// "X é a causa de Y", "X provoca Y", "por causa de X", "devido a X").
func causalClaim(result string) bool {
	r := strings.ToLower(result)
	markers := []string{
		" causou ", " causa de ", " causada por ", " causado por ",
		" provoca ", " provoca ", " por causa de ", " devido a ",
		" é a causa ", " é o motivo ", " originou ", " gerou ",
	}
	for _, m := range markers {
		if strings.Contains(r, m) {
			return true
		}
	}
	return false
}

// causalEvidence reporta se a evidência sustenta CAUSA (controle, antes/depois,
// isolamento, comparação) e não apenas correlação.
func causalEvidence(evidence string) bool {
	e := strings.ToLower(evidence)
	markers := []string{
		"antes", "depois", "controle", "isolad", "comparação", "comparacao",
		"benchmark", "a/b", "grupo", "variavel", "variável",
		"reproduz", "causa", "causal",
	}
	for _, m := range markers {
		if strings.Contains(e, m) {
			return true
		}
	}
	return false
}
