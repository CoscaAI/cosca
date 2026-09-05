package contextrouter

import "testing"

// TestDecideLevel_ByConfidence: os thresholds do ADR-032 decidem a camada.
func TestDecideLevel_ByConfidence(t *testing.T) {
	cases := []struct {
		conf float64
		want Level
	}{
		{0.90, Level0}, // EmitOK → L0 (Kernel decide sem LLM)
		{0.70, Level0}, // fronteira EmitOK → L0
		{0.60, Level1}, // reservations → L1
		{0.50, Level1}, // fronteira reservations → L1
		{0.40, Level2}, // escalate → L2
		{0.10, Level2}, // escalate → L2
	}
	for _, c := range cases {
		got := DecideLevel(Confidence{Final: c.conf})
		if got != c.want {
			t.Errorf("conf %.2f → %s, esperava %s", c.conf, got, c.want)
		}
	}
}

// TestDecideLevelWithInfo_ConfiancaAltaSemEvidencia: confiança alta MAS sem
// conhecimento → L1 (o Kernel não inventa; precisa de contexto).
func TestDecideLevelWithInfo_ConfiancaAltaSemEvidencia(t *testing.T) {
	got := DecideLevelWithInfo(Confidence{Final: 0.90}, false, false)
	if got != Level1 {
		t.Fatalf("confiança alta sem evidência → %s, esperava L1 (não inventa)", got)
	}
}

// TestDecideLevelWithInfo_ConfiancaAltaComEvidencia: confiança alta + evidência
// → L0 (o Kernel decide sem LLM).
func TestDecideLevelWithInfo_ConfiancaAltaComEvidencia(t *testing.T) {
	got := DecideLevelWithInfo(Confidence{Final: 0.90}, true, false)
	if got != Level0 {
		t.Fatalf("confiança alta com evidência → %s, esperava L0", got)
	}
}

// TestDecideLevelWithInfo_ConfiancaBaixa: confiança baixa → L2 mesmo com
// conhecimento (a decisão é incerta — contexto completo).
func TestDecideLevelWithInfo_ConfiancaBaixa(t *testing.T) {
	got := DecideLevelWithInfo(Confidence{Final: 0.30}, true, true)
	if got != Level2 {
		t.Fatalf("confiança baixa → %s, esperava L2", got)
	}
}

// TestEmit_SemLLM: o Kernel emite SEM LLM apenas em L0 com confiança alta E
// evidência (modo determinístico). Qualquer outra combinação → LLM.
func TestEmit_SemLLM(t *testing.T) {
	cases := []struct {
		name        string
		level       Level
		conf        float64
		hasKnowledge bool
		want        bool
	}{
		{"L0 confiança alta com evidência → emite", Level0, 0.90, true, true},
		{"L0 confiança alta sem evidência → NÃO emite", Level0, 0.90, false, false},
		{"L0 confiança baixa → NÃO emite", Level0, 0.40, true, false},
		{"L1 → NÃO emite (precisa LLM)", Level1, 0.90, true, false},
		{"L2 → NÃO emite (precisa LLM)", Level2, 0.90, true, false},
	}
	for _, c := range cases {
		got := Emit(c.level, Confidence{Final: c.conf}, c.hasKnowledge)
		if got != c.want {
			t.Errorf("%s → %v, esperava %v", c.name, got, c.want)
		}
	}
}

// TestLevel_String: nomes legíveis para logs.
func TestLevel_String(t *testing.T) {
	if Level0.String() != "L0" || Level1.String() != "L1" || Level2.String() != "L2" {
		t.Fatal("nomes de nível errados")
	}
}
