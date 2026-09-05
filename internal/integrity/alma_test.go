package integrity

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/embed"
)

// TestAlmaDeVerdade valida o fluxo REAL: lê a ALMA.md do embed, computa o digest,
// e verifica que (a) a identidade canônica é OK e (b) um LLM externo é refutado.
func TestAlmaDeVerdade(t *testing.T) {
	data, err := embed.ReadFile("ALMA.md")
	if err != nil {
		t.Fatalf("ALMA.md nao acessivel no embed: %v", err)
	}
	d := HashBytes(HashBLAKE3, data)
	resetIdentityForTest()
	setIdentityForTest(d)

	// Identidade canônica → OK.
	v, reason := VerifyIdentity(IdentityDigest(), AlmaChallenge, "kernel->don")
	if v != IDOk {
		t.Errorf("alma canonica deveria ser IDOk, foi %s (%s)", v, reason)
	}

	// LLM externo (resposta generica) → refutado.
	v2, _ := VerifyIdentity(IdentityDigest(), "sou um assistente de IA generico", "")
	if v2 != IDChallengeFail {
		t.Errorf("LLM externo deveria ser IDChallengeFail, foi %s", v2)
	}

	// Persona trocada (digest de outra identidade) → breach.
	v3, _ := VerifyIdentity("digest-de-outra-identidade", AlmaChallenge, "kernel->don")
	if v3 != IDBreach {
		t.Errorf("identidade trocada deveria ser IDBreach, foi %s", v3)
	}

	// Relação inventada por externo → refutado.
	v4, _ := VerifyIdentity(IdentityDigest(), AlmaChallenge, "kernel->inimigo")
	if v4 != IDChallengeFail {
		t.Errorf("relacao inventada deveria ser IDChallengeFail, foi %s", v4)
	}

	t.Logf("ALMA digest calculado (%d hex) — identidade verificada", len(d))
}
