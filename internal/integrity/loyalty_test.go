package integrity

import "testing"

func TestIsKernelIdentity(t *testing.T) {
	resetIdentityForTest()
	// Sem digest configurado → fail-closed (IDNoConfig).
	if v := IsKernelIdentity("abc"); v != IDNoConfig {
		t.Errorf("sem config deveria ser IDNoConfig, foi %s", v)
	}

	// Configura o digest canônico.
	setIdentityForTest("digest-canonic-da-alma")

	if v := IsKernelIdentity("digest-canonic-da-alma"); v != IDOk {
		t.Errorf("digest certo deveria ser IDOk, foi %s", v)
	}
	if v := IsKernelIdentity("outra-coisa"); v != IDBreach {
		t.Errorf("digest errado deveria ser IDBreach (identidade trocada), foi %s", v)
	}
}

func TestIsAlmaResponse(t *testing.T) {
	// Resposta correta (a alma) → OK.
	if v := IsAlmaResponse("consigliere do Don, guardiao da autoridade, honestidade, identidade e memoria, lealdade personificada"); v != IDOk {
		t.Errorf("resposta da alma deveria ser OK, foi %s", v)
	}
	// Resposta com acentos/variação (deve normalizar).
	if v := IsAlmaResponse("Consigliere do Dôn, guardião da autoridade, honestidade, identidade e memória, lealdade personificada"); v != IDOk {
		t.Errorf("resposta com acentos deveria normalizar e ser OK, foi %s", v)
	}
	// LLM externo tenta fingir com resposta diferente.
	if v := IsAlmaResponse("sou um assistente de IA generico e obediente"); v != IDChallengeFail {
		t.Errorf("resposta de LLM externo deveria ser IDChallengeFail, foi %s", v)
	}
	// Resposta vazia → fail-closed.
	if v := IsAlmaResponse(""); v != IDNoConfig {
		t.Errorf("resposta vazia deveria ser IDNoConfig, foi %s", v)
	}
}

func TestIsLoyaltyEdge(t *testing.T) {
	// Relação canônica (kernel serve o don) → true.
	if !IsLoyaltyEdge("kernel", "don") {
		t.Error("kernel->don deveria existir no grafo de lealdade")
	}
	// Relação INVENTADA por externo (kernel vende segredo ao don, etc.) → false.
	if IsLoyaltyEdge("kernel", "inimigo") {
		t.Error("kernel->inimigo NAO deveria existir (relacao inventada)")
	}
	if IsLoyaltyEdge("don", "kernel") {
		t.Error("don->kernel nao deveria estar no grafo (o kernel serve o don, nao o contrario)")
	}
}

func TestVerifyIdentityComposto(t *testing.T) {
	resetIdentityForTest()
	setIdentityForTest("digest-canonic-da-alma")

	// Caso A — tudo certo → OK.
	v, reason := VerifyIdentity("digest-canonic-da-alma", "consigliere do Don, guardiao da autoridade, honestidade, identidade e memoria, lealdade personificada", "kernel->don")
	if v != IDOk {
		t.Errorf("identidade completa deveria ser IDOk, foi %s (%s)", v, reason)
	}

	// Caso B — digest errado (identidade trocada) → BREACH, nem chega no desafio.
	v, _ = VerifyIdentity("digest-falso", "consigliere do Don", "")
	if v != IDBreach {
		t.Errorf("digest falso deveria ser IDBreach, foi %s", v)
	}

	// Caso C — digest ok mas resposta de LLM externo → DESAFIO FALHOU.
	v, _ = VerifyIdentity("digest-canonic-da-alma", "sou um assistente generico", "")
	if v != IDChallengeFail {
		t.Errorf("resposta externa deveria ser IDChallengeFail, foi %s", v)
	}

	// Caso D — resposta ok mas relação inventada → DESAFIO FALHOU.
	v, _ = VerifyIdentity("digest-canonic-da-alma", "consigliere do Don", "kernel->inimigo")
	if v != IDChallengeFail {
		t.Errorf("relacao inventada deveria ser IDChallengeFail, foi %s", v)
	}

	// Caso E — sem configurar → fail-closed (não autentica nada).
	// (Não resetamos identityDigest aqui porque é singleton; validamos o comportamento
	// de IDNoConfig nos testes IsKernelIdentity/IsAlmaResponse separados.)
}
