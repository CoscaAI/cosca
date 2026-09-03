package screen

import (
	"context"
	"image"
)

// OCRProvider é o contrato do sensor de texto. O cosca screen não sabe (nem
// precisa saber) qual motor está por trás — WinRT, Tesseract ou outro. É um
// sensor plugável, como o CLIP é o sensor visual.
//
// A assinatura recebe a imagem e as regiões já detectadas pelo detector nativo
// (visor de texto). O OCR preenche o Text/TextConfidence dessas regiões. Assim
// o OCR NÃO é obrigatório e NÃO é o cérebro — é só mais uma modalidade.
type OCRProvider interface {
	// Recognize lê o texto das regiões candidatas. Deve preencher o campo
	// Text/TextConfidence de cada região apontada. Devolve erro se falhar —
	// o chamador degrada graciosamente (visão continua).
	Recognize(ctx context.Context, img image.Image, regions []Region) error
}

// OCRResult é o resultado de reconhecer o texto de uma única região.
type OCRResult struct {
	// BBox é a região onde o texto foi reconhecido.
	BBox Rect `json:"bbox"`
	// Text é o texto reconhecido pela região.
	Text string `json:"text"`
	// Confidence é a confiança do OCR (0..1).
	Confidence float32 `json:"confidence"`
}

// NoOPCR is a no-op OCR provider (sensor ausente). Útil quando nenhum motor
// está disponível — a percepção visual segue sem texto.
type NoOPCR struct{}

// Recognize is a no-op (no text).
func (NoOPCR) Recognize(_ context.Context, _ image.Image, _ []Region) error {
	return nil // silencioso; não preenche texto
}
