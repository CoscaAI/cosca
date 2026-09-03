package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/screen"
)

// screenDisplay é o índice do monitor a capturar (0 = primário).
var screenDisplay int

// screenJSON altera o output modo. Via flag -j do root, use --json (o padrão).
// screenBeautyScore expõe ou não o score de beleza sintético.
var screenBeautyScore bool

// useOCR indica se o comando tenta o sensor de texto (OCR) — se não houver
// motor plugado, a percepção visual segue (degradação graciosa).
var useOCR bool

// NewScreenCommand cria o comando `cosca screen` — a percepção visual da tela
// por significado. O Cosca captura a tela, embebe via CLIP (mesma base semântica
// do texto) e calcula métricas estéticas objetivas, para o Kernel ENTENDER a
// beleza/design da tela — sem depender de um modelo generativo que só descreve.
func NewScreenCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "screen",
		Short: "Percepção visual da tela — o Cosca VÊ e ENTENDE (CLIP + métricas estéticas)",
		Long: `Percepção visual da tela por significado (doutrina do Don): a casa entende
por representação semântica, não por índice. Para o texto é o embedding
(nomic→vetor→cosseno); para a IMAGEM é o CLIP (imagem→vetor→cosseno) — a MESMA
língua.

cosca screen captura a tela, embebe via CLIP image-encoder (ONNX nativo,
local) e calcula métricas estéticas determinísticas (luminância, saturação,
contraste, harmonia de cor, simetria, complexidade). O resultado é o "significado"
da tela + o caráter visual — o Kernel usa isso para julgar beleza/design.

Saída:
  Texto  → resumo legível do caráter da tela + métricas.
  JSON   → embedding CLIP + métricas (para agentes/consumo por código).

Exemplos:
  cosca screen                     Captura e entende a tela principal
  cosca screen --display 1         Monitor secundário
  cosca screen --json              Embedding + métricas em JSON
  cosca screen --beauty            Exibe o score de beleza (composto)`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			// Feedback animado (modo texto).
			var spinner *Spinner
			if !useJSON && !globalFlags.Quiet {
				spinner = formatter.Spinner("Olhando a tela…")
				spinner.Start()
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			// Percepção multimodal: CLIP (semântica) + regiões (visor de texto)
			// + OCR opcional. Se não houver motor OCR, o screen ainda entende.
			var ocr screen.OCRProvider
			if useOCR {
				ocr = screen.NewWinRTOCR() // OCR nativo do Windows (WinRT)
			}
			res, err := screen.AnalyzeMultimodal(ctx, screenDisplay, ocr)
			if spinner != nil {
				if err != nil {
					spinner.Fail("percepção falhou")
				} else {
					spinner.Stop(fmt.Sprintf("tela %dx%d entendida (%d regiões)", res.Width, res.Height, len(res.Regions)))
				}
			}
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, map[string]any{
					"width":      res.Width,
					"height":     res.Height,
					"regions":    res.Regions,
					"has_text":   res.HasText,
					"ocr_error":  res.OCRError,
					"embedding":  res.VisualEmbedding,
					"aesthetic":  res.Aesthetic,
					"beauty":     beautyScore(res.Aesthetic),
					"understood": screenUnderstood(res),
				})
			}

			formatter.Header(fmt.Sprintf("Tela %dx%d (%s)", res.Width, res.Height, screenVerdict(res)))
			formatter.KeyValue("modelo CLIP", yesno(len(res.VisualEmbedding) > 0))
			formatter.KeyValue("beauty", fmt.Sprintf("%.2f", beautyScore(res.Aesthetic)))
			formatter.KeyValue("brightness", fmt.Sprintf("%.2f", res.Aesthetic.Brightness))
			formatter.KeyValue("saturation", fmt.Sprintf("%.2f", res.Aesthetic.Saturation))
			formatter.KeyValue("contrast", fmt.Sprintf("%.2f", res.Aesthetic.Contrast))
			formatter.KeyValue("color_harmony", fmt.Sprintf("%.2f", res.Aesthetic.ColorHarmony))
			formatter.KeyValue("symmetry", fmt.Sprintf("%.2f", res.Aesthetic.Symmetry))
			formatter.KeyValue("complexity", fmt.Sprintf("%.2f", res.Aesthetic.Complexity))
			formatter.KeyValue("regiões", fmt.Sprintf("%d", len(res.Regions)))
			if len(res.Regions) > 0 {
				formatter.Println("")
				for i, r := range res.Regions {
					if r.Kind == screen.RegionText {
						label := r.Text
						if label == "" {
							label = "(texto não lido)"
						}
						formatter.Bullet(fmt.Sprintf("região [%d] %s em (%d,%d %dx%d): %s", i+1, r.Kind, r.BBox.X, r.BBox.Y, r.BBox.W, r.BBox.H, label))
					}
				}
			}
			formatter.Println("")
			formatter.Bullet("entendimento: " + screenUnderstood(res))
			return nil
		},
	}

	cmd.Flags().IntVar(&screenDisplay, "display", 0, "Índice do monitor a capturar (0 = primário)")
	cmd.Flags().BoolVar(&screenBeautyScore, "beauty", false, "Exibir o score de beleza (composto) no texto")
	cmd.Flags().BoolVar(&useOCR, "ocr", false, "Tenta o sensor de texto (OCR) — se não houver motor, a visão segue")
	return cmd
}

// beautyScore compõe as métricas estéticas em um score único (0..1) de "beleza"
// percebida. Usa pesos balanceados: harmonia, contraste e simetria dominam.
func beautyScore(a screen.Aesthetic) float64 {
	score := 0.45*a.ColorHarmony + 0.25*a.Contrast + 0.20*a.Symmetry + 0.10*a.Complexity
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	return score
}

// screenVerdict resume o caráter da tela a partir do veredito da métrica.
func screenVerdict(res *screen.Screen) string {
	if res == nil {
		return "desconhecido"
	}
	return res.Aesthetic.Verdict
}

// screenUnderstood gera uma frase de entendimento (o Kernel interpretando a
// percepção multimodal: estética + regiões + texto se houver OCR).
func screenUnderstood(res *screen.Screen) string {
	if res == nil {
		return "não foi possível entender a tela"
	}
	a := res.Aesthetic
	desc := "tela "
	switch a.Verdict {
	case "harmonioso_e_rico":
		desc += "harmoniosa e rica em conteúdo"
	case "harmonioso":
		desc += "com paleta coesa e equilibrada"
	case "contraste_alto":
		desc += "de alto contraste — clara a ênfase visual"
	case "minimalista":
		desc += "minimalista — limpa e direta"
	default:
		desc += "simples"
	}
	if a.Symmetry > 0.7 {
		desc += ", composição bem equilibrada"
	}
	// Percepção de texto: se o detector achou regiões, a tela tem conteúdo
	// textual (ainda que o OCR não o tenha lido).
	if res.HasText {
		desc += fmt.Sprintf(", com %d região(ões) de conteúdo", len(res.Regions))
		// Se alguma região tem texto lido pelo OCR, cita.
		if t := firstRegionText(res.Regions); t != "" {
			desc += fmt.Sprintf(" (ex: %q)", t)
		}
	}
	return desc
}

// firstRegionText devolve o primeiro texto lido por OCR, se houver.
func firstRegionText(regions []screen.Region) string {
	for _, r := range regions {
		if r.Text != "" {
			return r.Text
		}
	}
	return ""
}

// yesno converte bool para "sim"/"não".
func yesno(v bool) string {
	if v {
		return "sim"
	}
	return "não"
}
