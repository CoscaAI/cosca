package sensor

import (
	"testing"
	"time"
)

// TestNewMeasured verifica que New produz uma evidência MEASURED forte com
// Source/Modality/Content/Confidence corretos — a confiança de um sensor que
// leu de verdade.
func TestNewMeasured(t *testing.T) {
	o := New(ModalityText, "line", "Punta Cana", 0.93, "ocr")
	if o.Epistemic != EpistemicMEASURED {
		t.Errorf("esperava MEASURED, got %q", o.Epistemic)
	}
	if o.Content != "Punta Cana" {
		t.Errorf("content errado: %q", o.Content)
	}
	if o.Source != "ocr" {
		t.Errorf("source errado: %q", o.Source)
	}
	if o.Modality != ModalityText {
		t.Errorf("modality errado: %q", o.Modality)
	}
	if o.Kind != "line" {
		t.Errorf("kind errado: %q", o.Kind)
	}
	if float64(o.Confidence) != 0.93 {
		t.Errorf("confianca errada: %f", o.Confidence)
	}
	if o.Timestamp.IsZero() {
		t.Errorf("timestamp nao deve ser zero")
	}
}

// TestInferred verifica que Inferred deduz (classe fraca) e nao mede.
func TestInferred(t *testing.T) {
	o := Inferred(ModalityVisual, "entity", "botao-voltar", 0.62, "tracker")
	if o.Epistemic != EpistemicINFERRED {
		t.Errorf("esperava INFERRED, got %q", o.Epistemic)
	}
	// INFERRED nao pode ser tratado como fato pelo kernel.
	if o.Epistemic == EpistemicMEASURED {
		t.Errorf("inferido nao pode virar medido")
	}
}

// TestCorroborated verifica a classe mais forte: dois sensores concordaram.
func TestCorroborated(t *testing.T) {
	o := Corroborated(ModalityVisual, "entity", "pomar", 0.98, "clip+ocr")
	if o.Epistemic != EpistemicEVIDENCE {
		t.Errorf("esperava EVIDENCE, got %q", o.Epistemic)
	}
}

// TestChaining verifica o encadeamento fluente (WithRegion/WithTrace/WithID)
// sem quebrar a observação original.
func TestChaining(t *testing.T) {
	base := New(ModalityText, "line", "ola", 0.9, "ocr")
	o := base.
		WithRegion(Region{X: 10, Y: 20, W: 100, H: 30, Kind: "text"}).
		WithTrace("TRACE-1").
		WithID("obs-42")

	if o.Region == nil {
		t.Fatal("region deve estar presente")
	}
	if o.Region.X != 10 || o.Region.Y != 20 || o.Region.W != 100 || o.Region.H != 30 {
		t.Errorf("region errada: %+v", o.Region)
	}
	if o.TraceID != "TRACE-1" {
		t.Errorf("trace errado: %q", o.TraceID)
	}
	if o.ID != "obs-42" {
		t.Errorf("id errado: %q", o.ID)
	}

	// A base deve permanecer intacta (imutabilidade do chaining).
	if base.Region != nil {
		t.Errorf("base nao deve ter region apos chain")
	}
	if base.TraceID != "" {
		t.Errorf("base nao deve ter trace")
	}
	if base.ID != "" {
		t.Errorf("base nao deve ter id")
	}
}

// TestIsTrustworthy verifica a semente do GATE: abaixo do limiar o kernel
// deve saber que NAO e confiavel (escalar/corroborar), nao que e falso.
func TestIsTrustworthy(t *testing.T) {
	strong := New(ModalityWeb, "answer", "previsao de chuva amanha", 0.85, "web")
	weak := Inferred(ModalityText, "line", "texto tao pequeno que talvez seja ruido", 0.33, "ocr")

	if !strong.IsTrustworthy(0.6) {
		t.Errorf("evidencia forte (0.85) deveria passar limiar 0.6")
	}
	if weak.IsTrustworthy(0.6) {
		t.Errorf("evidencia fraca (0.33) NAO deveria passar limiar 0.6")
	}
	// Confianca alta e limiar baixo -> passa. Teste de borda.
	if !New(ModalityState, "size", "x", 0.50, "fs").IsTrustworthy(0.5) {
		t.Errorf("limiar igual a confianca deve passar (>=)")
	}
}

// TestTimestampUTC verifica que o timestamp nasce em UTC.
func TestTimestampUTC(t *testing.T) {
	o := New(ModalityVisual, "embedding", "frame", 0.91, "clip")
	loc := o.Timestamp.Location()
	if loc.String() != "UTC" {
		t.Errorf("timestamp deveria ser UTC, got %q", loc)
	}
}

// TestEpistemicLadder verifica a ordem NUNCA inversa da confiança epistêmica:
// EVIDENCE (2 sensores) >= MEASURED (1 sensor leu) > INFERRED (deduziu).
func TestEpistemicLadder(t *testing.T) {
	o := New(ModalityVisual, "entity", "x", 0.9, "clip")
	if o.Epistemic == EpistemicINFERRED {
		t.Errorf("New nunca deveria ser INFERRED")
	}
	if Inferred(ModalityVisual, "entity", "x", 0.9, "t").Epistemic == EpistemicMEASURED {
		t.Errorf("Inferred nunca deveria ser MEASURED")
	}
	_ = time.Now // mantém time importado para futuros testes de janela
}
