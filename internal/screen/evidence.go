package screen

import (
	"github.com/CoscaAI/cosca/internal/sensor"
)

// Evidence converte a percepção visual desta tela em observações canônicas
// (internal/sensor.Observation). É a ponte entre o sensor de tela e o DTO
// normalizado — o kernel nunca precisa saber que veio de OCR/CLIP.
//
// Doutrina (professor+Don, 2026-09-02): todo sensor emite o MESMO pacote
// {tipo, conteúdo, confiança, fonte, estilo_epistêmico, timestamp, trace_id}.
// Este método aplica a regra de classe epistêmica:
//   - Texto LIDO pelo OCR → EpistemicMEASURED (o sensor mediu de verdade).
//   - Região de texto detectada mas NÃO lida → EpistemicINFERRED (deduziu que
//     há texto; não é fato). O kernel não pode tratá-la como conteúdo lido.
func (s *Screen) Evidence(source string) []sensor.Observation {
	if s == nil {
		return nil
	}
	var obs []sensor.Observation
	for _, r := range s.Regions {
		if r.Kind != RegionText {
			continue
		}
		region := sensor.Region{X: r.BBox.X, Y: r.BBox.Y, W: r.BBox.W, H: r.BBox.H, Kind: string(r.Kind)}

		switch {
		case r.Text != "":
			// O OCR leu o texto de verdade → MEASURED.
			o := sensor.New(sensor.ModalityText, "line", r.Text, sensor.Confidence(r.TextConfidence), source)
			obs = append(obs, o.WithRegion(region))
		default:
			// Só a região foi detectada → INFERRED (há texto aqui, não sei qual).
			o := sensor.Inferred(sensor.ModalityVisual, "text_region", "região de texto detectada", sensor.Confidence(r.Confidence), source)
			obs = append(obs, o.WithRegion(region))
		}
	}
	return obs
}
