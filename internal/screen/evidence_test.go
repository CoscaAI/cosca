package screen

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/sensor"
)

// approx compara dois float64 com tolerância de 1e-4 (evita falsos positivos
// por precisão de ponto flutuante ao converter float32→float64).
func approx(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1e-4
}

// TestEvidence_Measured verifica que uma região de texto LIDA pelo OCR vira
// uma observação MEASURED com ModalityText e a região anexada.
func TestEvidence_Measured(t *testing.T) {
	s := &Screen{Regions: []Region{
		{BBox: Rect{X: 10, Y: 20, W: 100, H: 30}, Kind: RegionText, Text: "Punta Cana", TextConfidence: 0.93},
	}}
	obs := s.Evidence("ocr")
	if len(obs) != 1 {
		t.Fatalf("esperava 1 observacao, got %d", len(obs))
	}
	o := obs[0]
	if o.Epistemic != sensor.EpistemicMEASURED {
		t.Errorf("texto lido deveria ser MEASURED, got %q", o.Epistemic)
	}
	if o.Modality != sensor.ModalityText {
		t.Errorf("modality deveria ser text, got %q", o.Modality)
	}
	if o.Content != "Punta Cana" {
		t.Errorf("content errado: %q", o.Content)
	}
	if o.Source != "ocr" {
		t.Errorf("source errado: %q", o.Source)
	}
	if !approx(float64(o.Confidence), 0.93) {
		t.Errorf("confianca errada: %f", o.Confidence)
	}
	if o.Region == nil || o.Region.X != 10 || o.Region.Y != 20 || o.Region.W != 100 || o.Region.H != 30 {
		t.Errorf("region errada: %+v", o.Region)
	}
	if o.Kind != "line" {
		t.Errorf("kind errado: %q", o.Kind)
	}
}

// TestEvidence_Inferred verifica que uma região de texto detectada mas NÃO
// lida vira INFERRED — o kernel não pode tratá-la como conteúdo lido.
func TestEvidence_Inferred(t *testing.T) {
	s := &Screen{Regions: []Region{
		{BBox: Rect{X: 0, Y: 0, W: 1920, H: 1080}, Kind: RegionText, Confidence: 0.62},
	}}
	obs := s.Evidence("ocr")
	if len(obs) != 1 {
		t.Fatalf("esperava 1 observacao, got %d", len(obs))
	}
	o := obs[0]
	if o.Epistemic != sensor.EpistemicINFERRED {
		t.Errorf("regiao sem texto deveria ser INFERRED, got %q", o.Epistemic)
	}
	if o.Modality != sensor.ModalityVisual {
		t.Errorf("regiao deduzida deveria ser visual, got %q", o.Modality)
	}
	if o.Content != "região de texto detectada" {
		t.Errorf("content errado: %q", o.Content)
	}
	if !approx(float64(o.Confidence), 0.62) {
		t.Errorf("confianca (do detector) errada: %f", o.Confidence)
	}
}

// TestEvidence_IgnoresNonText verifica que regiões que não são de texto
// (gráfico/desconhecida) NÃO geram observação — o sensor de texto não fala
// sobre o que não é texto.
func TestEvidence_IgnoresNonText(t *testing.T) {
	s := &Screen{Regions: []Region{
		{BBox: Rect{X: 0, Y: 0, W: 50, H: 50}, Kind: RegionGraphic},
		{BBox: Rect{X: 0, Y: 0, W: 50, H: 50}, Kind: RegionUnknown},
		{BBox: Rect{X: 0, Y: 0, W: 50, H: 50}, Kind: RegionText, Text: "ola", TextConfidence: 0.8},
	}}
	obs := s.Evidence("ocr")
	if len(obs) != 1 {
		t.Fatalf("esperava apenas a regiao de texto (1), got %d", len(obs))
	}
	if obs[0].Content != "ola" {
		t.Errorf("conteudo errado: %q", obs[0].Content)
	}
}

// TestEvidence_NilScreen verifica que um Screen nil não panica.
func TestEvidence_NilScreen(t *testing.T) {
	var s *Screen
	obs := s.Evidence("ocr")
	if obs != nil {
		t.Errorf("Screen nil deveria devolver nil, got %d obs", len(obs))
	}
}

// TestEvidence_Empty verifica que um Screen sem regiões devolve vazio.
func TestEvidence_Empty(t *testing.T) {
	s := &Screen{}
	obs := s.Evidence("ocr")
	if len(obs) != 0 {
		t.Errorf("tela sem regioes deveria devolver 0 obs, got %d", len(obs))
	}
}
