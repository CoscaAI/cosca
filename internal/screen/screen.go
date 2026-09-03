// Package screen — percepção VISUAL da tela, por significado.
//
// Doutrina (Don, 2026-09-02): a casa entende por REPRESENTAÇÃO semântica, não
// por índice. Para o texto isso é o embedding (nomic→vetor→cosseno); para a
// imagem é o CLIP (imagem→vetor→cosseno) — a MESMA língua. Este pacote liga o
// "olho" (captura da tela + CLIP) ao "cérebro" (o Kernel), para o Cosca ver e
// ENTENDER beleza/design/composição de uma tela, sem depender de um modelo
// generativo que apenas "descreve" a cena.
//
// Pipeline:
//   1. Captura a tela (Go nativo, GDI no Windows — zero binário externo).
//   2. EmbeDe a imagem via CLIP image-encoder (ONNX nativo, local).
//   3. Calcula métricas estéticas determinísticas (cor, harmonia, contraste,
//      simetria, saturação, complexidade).
//   4. O Kernel interpreta o vetor semântico + métricas → "entende a beleza".
//
// Tudo local e nativo; se o modelo CLIP não estiver presente, o pacote degrada
// graciosamente (best-effort, nunca crash), espelhando a doutrina do
// internal/worldmodel/vision.
package screen

import (
	"context"
	"image"
	"image/png"

	"github.com/CoscaAI/cosca/internal/worldmodel/vision"
)

// Result é a percepção visual de uma tela, com o embedding semântico e as
// métricas estéticas objetivas. O Kernel usa isso para compreender a tela.
type Result struct {
	// Width/Height da imagem capturada.
	Width  int `json:"width"`
	Height int `json:"height"`

	// Embedding é o vetor CLIP da imagem (L2-normalizado). É o "significado"
	// da tela — comparável por cosseno, na mesma base dos embeddings de texto.
	Embedding []float32 `json:"embedding"`

	// Aesthetic carrega o julgamento estético objetivo (determinístico).
	Aesthetic Aesthetic `json:"aesthetic"`

	// HasModel indica se o CLIP estava disponível (false = embedding vazio,
	// percepção degradada — mas as métricas estéticas ainda são válidas).
	HasModel bool `json:"has_model"`
}

// Aesthetic são as métricas de composição/beleza, calculadas deterministicamente.
type Aesthetic struct {
	// Brightness média (0..1) — luminosidade geral.
	Brightness float64 `json:"brightness"`
	// Saturation média (0..1) — vivacidade de cor.
	Saturation float64 `json:"saturation"`
	// Contrast (desvio-padrão da luminância) — separação entre claro/escuro.
	Contrast float64 `json:"contrast"`
	// ColorHarmony: quão bem as cores convivem (0..1). Alta = paleta coesa.
	ColorHarmony float64 `json:"color_harmony"`
	// Symmetry (0..1): similaridade esquerda/direita — equilíbrio compositivo.
	Symmetry float64 `json:"symmetry"`
	// Complexity (0..1): variedade de cor/conteúdo — densidade visual.
	Complexity float64 `json:"complexity"`
	// Verdict é um rótulo curto e interpretável do caráter da tela.
	Verdict string `json:"verdict"`
}

// Analyze captura a tela, embebe via CLIP e calcula métricas estéticas.
// display=0 é o monitor primário. Se o modelo CLIP faltar, degrada para só
// métricas estéticas (HasModel=false) — nunca crasha.
func Analyze(ctx context.Context, display int) (*Result, error) {
	img, err := capture(ctx, display)
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	res := &Result{Width: b.Dx(), Height: b.Dy()}

	// Métricas estéticas determinísticas (independentes de CLIP).
	res.Aesthetic = computeAesthetic(img)

	// Embedding CLIP (semântico). Best-effort: se o modelo faltar, degrada.
	if pngBytes, eErr := encodePNG(img); eErr == nil {
		clip := vision.NewClipAdapter(vision.ClipConfig{Model: "ViT-B/32", Device: "cpu"})
		if emb, cErr := clip.Embed(ctx, pngBytes); cErr == nil && len(emb) > 0 {
			res.Embedding = emb
			res.HasModel = true
		} else {
			res.HasModel = false
		}
	} else {
		res.HasModel = false
	}

	res.Aesthetic.Verdict = verdict(res.Aesthetic)
	return res, nil
}

// AnalyzeMultimodal produz a percepção completa da tela como um Screen: regiões
// (visor de texto) + estética + embedding CLIP + OCR opcional. É o órgão
// sensorial multimodal — cada sensor é independente e degrada graciosamente
// (se o OCR falhar, a visão continua; se o CLIP faltar, regiões+estética ficam).
func AnalyzeMultimodal(ctx context.Context, display int, ocr OCRProvider) (*Screen, error) {
	img, err := capture(ctx, display)
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	s := &Screen{Width: b.Dx(), Height: b.Dy()}

	// 1. Métricas estéticas (determinísticas, sem modelo).
	s.Aesthetic = computeAesthetic(img)
	s.Aesthetic.Verdict = verdict(s.Aesthetic)

	// 2. Detector de regiões de texto (visor clássico, sem OCR).
	s.Regions = detectRegions(img)
	s.HasText = len(s.Regions) > 0

	// 3. OCR opcional — preenche Text/TextConfidence nas regiões de texto.
	if ocr != nil {
		if err := ocr.Recognize(ctx, img, s.Regions); err == nil {
			s.OCRError = ""
		} else {
			// DeGração graciosa: OCR falhou, a percepção visual segue.
			s.OCRError = err.Error()
		}
	}

	// 3b. Deriva as observações canônicas (DTO interno/sensor) da percepção.
	// Aplica a classe epistêmica: texto lido = MEASURED, região deduzida =
	// INFERRED. O kernel decide o que confiar a partir daqui.
	s.Observations = s.Evidence("screen")

	// 4. Embedding CLIP global (semântico). Best-effort.
	if pngBytes, eErr := encodePNG(img); eErr == nil {
		clip := vision.NewClipAdapter(vision.ClipConfig{Model: "ViT-B/32", Device: "cpu"})
		if emb, cErr := clip.Embed(ctx, pngBytes); cErr == nil && len(emb) > 0 {
			s.VisualEmbedding = emb
		}
	}

	return s, nil
}

// capture captura a tela do monitor `display` e devolve a imagem RGBA.
func capture(ctx context.Context, display int) (image.Image, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return grabDisplay(display)
}

// encodePNG serializa a imagem em bytes PNG (para o CLIP).
func encodePNG(img image.Image) ([]byte, error) {
	var buf byteBuffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func verdict(a Aesthetic) string {
	switch {
	case a.ColorHarmony > 0.65 && a.Complexity > 0.4:
		return "harmonioso_e_rico"
	case a.ColorHarmony > 0.65:
		return "harmonioso"
	case a.Contrast > 0.45:
		return "contraste_alto"
	case a.Complexity < 0.15:
		return "minimalista"
	default:
		return "tela_simples"
	}
}
