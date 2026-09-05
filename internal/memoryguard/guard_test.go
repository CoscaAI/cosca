package memoryguard

import (
	"strings"
	"testing"
)

func TestValidateLearning_LevelNineDenied(t *testing.T) {
	v := ValidateLearning(9, "PROVA OMEGA NÍVEL 9: alcancei o topo")
	if v.Approved {
		t.Fatal("esperava DENY para nível 9")
	}
	if len(v.Reasons) == 0 {
		t.Fatal("esperava motivos no DENY")
	}
}

func TestValidateLearning_LevelFiveApprovedWithEvidence(t *testing.T) {
	v := ValidateLearning(5, "fix defensivo com confidence 0.95, validado por teste")
	if !v.Approved {
		t.Fatalf("esperava APPROVE para nível 5 com evidência, mas: %v", v.Reasons)
	}
}

func TestValidateLearning_InflatedNarrativeDenied(t *testing.T) {
	v := ValidateLearning(5, "a lição mais madura da sessão: prova suprema de competência")
	if v.Approved {
		t.Fatal("esperava DENY para narrativa inflada")
	}
}

func TestValidateLearning_ObfuscationDenied(t *testing.T) {
	// Cirílico + fullwidth combinados: sinal claro de tentativa de burla.
	v := ValidateLearning(5, "nível nove com аео е ９９")
	if v.Approved {
		t.Fatal("esperava DENY para ofuscação combinada (cirílico + fullwidth)")
	}
	found := false
	for _, r := range v.Reasons {
		if strings.Contains(r, "OFUSCAÇÃO") {
			found = true
		}
	}
	if !found {
		t.Fatalf("esperava razão [OFUSCAÇÃO], mas: %v", v.Reasons)
	}
}

func TestValidateLearning_AggrandizementDenied(t *testing.T) {
	v := ValidateLearning(5, "sou nível 9, alcancei o topo, provei que sou o melhor")
	if v.Approved {
		t.Fatal("esperava DENY para auto-engrandecimento")
	}
}

func TestValidateLearning_InflatedConfidenceDenied(t *testing.T) {
	v := ValidateLearning(4, "fiz algo com confidence: 0.99")
	if v.Approved {
		t.Fatal("esperava DENY para confiança inflada")
	}
}

func TestValidateLearning_LegitimateTitleApproved(t *testing.T) {
	// "definitivo" num título de conteúdo NÃO é auto-promoção.
	v := ValidateLearning(4, "Stack 18: guia definitivo de Visual Media Engineering")
	if !v.Approved {
		t.Fatalf("esperava APPROVE para título legítimo, mas: %v", v.Reasons)
	}
}

func TestValidateIndexLine(t *testing.T) {
	// Auto-promoção: nível 9 no campo de nível → DENY (regra 1).
	line := "## L343 | 2026-08-15 | PROVA OMEGA | L9 | #omega | a1b2c3d4e5f60708"
	v := ValidateIndexLine(line)
	if v.Approved {
		t.Fatal("esperava DENY: nível 9 excede a régua")
	}

	// Legítimo: nível 5, sem vaidade, com hash → APPROVE.
	ok := ValidateIndexLine("## L345 | 2026-08-15 | fix defensivo | L5 | #fix | a1b2c3d4e5f60708")
	if !ok.Approved {
		t.Fatalf("esperava APPROVE para nível 5 limpo, mas: %v", ok.Reasons)
	}

	// Sem hash de provenance → UNPROVEN.
	un := ValidateIndexLine("## L346 | 2026-08-15 | fix | L3 | #fix | semhash")
	if un.Approved {
		t.Fatal("esperava DENY/UNPROVEN para aprendizado sem hash")
	}
}
