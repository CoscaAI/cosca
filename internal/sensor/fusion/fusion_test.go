package fusion

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/sensor"
)

// TestConsensusRises verifica o princípio central: dois sensores concordando
// SOBEM a confiança (evidência independente), NÃO fazem média.
func TestConsensusRises(t *testing.T) {
	obs := []sensor.Observation{
		sensor.New(sensor.ModalityText, "line", "Las Vegas", 0.91, "ocr"),
		sensor.New(sensor.ModalityText, "line", "Las Vegas", 0.91, "ocr-2"),
	}
	res := Fuse(obs)
	if len(res.Consensus) != 1 {
		t.Fatalf("esperava 1 referente, got %d", len(res.Consensus))
	}
	f := res.Consensus[0]
	if f.Confidence <= 0.91 {
		t.Errorf("consenso (%.3f) deve ser > individual (0.91)", f.Confidence)
	}
	if f.Epistemic != sensor.EpistemicMEASURED {
		t.Errorf("epistemic deveria ser MEASURED, got %q", f.Epistemic)
	}
	if f.Support != 2 {
		t.Errorf("support deveria ser 2 (2 fontes), got %d", f.Support)
	}
	if f.Referent != "las vegas" {
		t.Errorf("referent errado: %q", f.Referent)
	}
	if len(res.Contradictions) != 0 {
		t.Errorf("concordância nao deveria gerar contradicao")
	}
}

// TestConfidenceNotAverage verifica que a fusão NÃO é média: 0.99+0.99 deve
// combinar para > 0.99 (muito mais certo que cada um).
func TestConfidenceNotAverage(t *testing.T) {
	obs := []sensor.Observation{
		sensor.New(sensor.ModalityText, "line", "pomar", 0.99, "ocr"),
		sensor.New(sensor.ModalityText, "line", "pomar", 0.99, "ocr-2"),
	}
	f := Fuse(obs).Consensus[0]
	if f.Confidence <= 0.99 {
		t.Errorf("combinacao (%.3f) deveria ser > 0.99, nao media", f.Confidence)
	}
}

// TestSingleSensorKeepsConfidence verifica que um único sensor não infla.
func TestSingleSensorKeepsConfidence(t *testing.T) {
	obs := []sensor.Observation{
		sensor.New(sensor.ModalityText, "line", "sozinho", 0.73, "ocr"),
	}
	f := Fuse(obs).Consensus[0]
	if f.Confidence != 0.73 {
		t.Errorf("sensor unico deveria manter 0.73, got %.3f", f.Confidence)
	}
	if f.Support != 1 {
		t.Errorf("support deveria ser 1, got %d", f.Support)
	}
}

// TestContradictionSameSlot verifica hipóteses mutuamente exclusivas no mesmo
// slot: "Las Vegas" vs "Los Angeles" na MESMA região → contradição, o kernel
// não escolhe vencedor.
func TestContradictionSameSlot(t *testing.T) {
	region := sensor.Region{X: 0, Y: 0, W: 200, H: 30}
	a := sensor.New(sensor.ModalityText, "line", "Las Vegas", 0.91, "ocr").WithRegion(region)
	b := sensor.New(sensor.ModalityText, "line", "Los Angeles", 0.72, "ocr-2").WithRegion(region)
	res := Fuse([]sensor.Observation{a, b})

	if len(res.Contradictions) == 0 {
		t.Fatalf("esperava contradicao no mesmo slot, nao encontrou")
	}
	c := res.Contradictions[0]
	if len(c.Referents) != 2 {
		t.Errorf("contradicao deveria ter 2 referentes concorrentes, got %d", len(c.Referents))
	}
	// Não deve ser tratado como consenso vencedor: grupo fica uniparental.
	if len(res.Consensus) != 2 {
		t.Errorf("esperava 2 grupos (hipoteses distintas), got %d", len(res.Consensus))
	}
}

// TestContradictionDifferentRegion verifica que conteúdos iguais em regiões
// diferentes NÃO conflitam (são coisas distintas no espaço).
func TestContradictionDifferentRegion(t *testing.T) {
	a := sensor.New(sensor.ModalityText, "line", "K10", 0.8, "ocr").WithRegion(sensor.Region{X: 0, Y: 0, W: 10, H: 10})
	b := sensor.New(sensor.ModalityText, "line", "K11", 0.8, "ocr").WithRegion(sensor.Region{X: 100, Y: 100, W: 10, H: 10})
	res := Fuse([]sensor.Observation{a, b})
	if len(res.Contradictions) != 0 {
		t.Errorf("regioes diferentes nao deveriam conflitar")
	}
}

// TestDifferentModalitiesDoNotConflict verifica que modalidades diferentes
// (texto vs visual) são dimensões complementares, não contradição.
func TestDifferentModalitiesDoNotConflict(t *testing.T) {
	region := sensor.Region{X: 0, Y: 0, W: 400, H: 100}
	a := sensor.New(sensor.ModalityText, "line", "Las Vegas", 0.91, "ocr").WithRegion(region)
	b := sensor.New(sensor.ModalityVisual, "entity", "destination", 0.84, "clip").WithRegion(region)
	res := Fuse([]sensor.Observation{a, b})
	if len(res.Contradictions) != 0 {
		t.Errorf("modalidades diferentes sao complementares, nao contradicao")
	}
}

// TestGroupByReferent verifica que referentes diferentes formam grupos separados.
func TestGroupByReferent(t *testing.T) {
	obs := []sensor.Observation{
		sensor.New(sensor.ModalityText, "line", "Las Vegas", 0.91, "ocr"),
		sensor.New(sensor.ModalityText, "line", "Costa Rica", 0.85, "ocr"),
	}
	if len(Fuse(obs).Consensus) != 2 {
		t.Errorf("esperava 2 referentes distintos")
	}
}

// TestLowConfidenceIsNoise verifica que observações de baixa confiança não
// disputam nem conflitam (são ruído, não hipótese).
func TestLowConfidenceIsNoise(t *testing.T) {
	region := sensor.Region{X: 0, Y: 0, W: 100, H: 30}
	a := sensor.New(sensor.ModalityText, "line", "Las Vegas", 0.31, "ocr").WithRegion(region)
	b := sensor.New(sensor.ModalityText, "line", "C.ta Rka", 0.42, "ocr").WithRegion(region)
	res := Fuse([]sensor.Observation{a, b})
	if len(res.Contradictions) != 0 {
		t.Errorf("ruido de baixa confianca nao deveria conflitar")
	}
}

// TestEmpty verifica que fusão de vazio devolve Result vazio.
func TestEmpty(t *testing.T) {
	res := Fuse(nil)
	if res.Consensus != nil || res.Contradictions != nil {
		t.Errorf("Fuse(nil) deveria devolver Result vazio")
	}
}
