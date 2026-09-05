package screen

import "github.com/CoscaAI/cosca/internal/sensor"

// RegionKind classifica o tipo de uma região percebida na tela. Um detector
// nativo (visão computacional clássica) classifica; o OCR posteriormente
// preenche o texto. Isto NÃO é um "OCR disfarçado" — é apenas "aqui provavelmente
// existe texto".
type RegionKind string

const (
	// RegionText indica uma área onde provavelmente há linhas de texto.
	RegionText RegionKind = "text"
	// RegionGraphic indica uma área visual (gráfico/imagem) sem texto.
	RegionGraphic RegionKind = "graphic"
	// RegionUnknown indica uma região não classificada (baixa confiança).
	RegionUnknown RegionKind = "unknown"
)

// Rect é um retângulo em coordenadas absolutas de imagem (pixels).
type Rect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// Region é uma região percebida da tela. É a unidade básica da percepção
// multimodal: cada região carrega o bounding box, o tipo, e (quando o sensor
// CLIP rodar) seu embedding visual. O sensor OCR preenche Text/TextConfidence
// depois — região e OCR são independentes.
type Region struct {
	// BBox é a posição/tamanho da região na imagem.
	BBox Rect `json:"bbox"`
	// Kind é a classificação (text/graphic/unknown).
	Kind RegionKind `json:"kind"`
	// Confidence é a confiança do DETECTOR (não do OCR) em 0..1.
	Confidence float32 `json:"confidence"`
	// VisualEmbedding é o embedding CLIP desta região (se computado).
	VisualEmbedding []float32 `json:"visual_embedding,omitempty"`
	// Text é preenchido pelo sensor OCR (opcional).
	Text string `json:"text,omitempty"`
	// TextConfidence é a confiança do OCR para o texto (0..1).
	TextConfidence float32 `json:"text_confidence,omitempty"`
}

// Screen é a percepção multimodal completa de uma captura.
type Screen struct {
	// Width/Height da imagem.
	Width  int `json:"width"`
	Height int `json:"height"`
	// Regions são as regiões percebidas (texto/gráfico).
	Regions []Region `json:"regions"`
	// Aesthetic é o julgamento estético global (objetivo/CLIP).
	Aesthetic Aesthetic `json:"aesthetic"`
	// VisualEmbedding é o embedding CLIP global da tela.
	VisualEmbedding []float32 `json:"visual_embedding,omitempty"`
	// HasText indica se o sensor de texto (detector) rodou.
	HasText bool `json:"has_text"`
	// OCRError indica se o sensor OCR falhou (degradação graciosa).
	OCRError string `json:"ocr_error,omitempty"`
	// Observations são as evidências canônicas (internal/sensor) derivadas
	// desta percepção — a ponte do sensor de tela para o DTO normalizado.
	// Preenchido por AnalyzeMultimodal via Screen.Evidence().
	Observations []sensor.Observation `json:"-"`
}
