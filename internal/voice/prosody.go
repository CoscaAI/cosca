package voice

// ── Prosódia (prosody.go) ────────────────────────────────────────────────
// Diretriz do Professor: "uma voz convincente responde 'quanto tempo?',
// 'com que entonação?', 'onde respirar?', 'qual palavra enfatizar?'".
//
// Este arquivo é o ProsodyPlanner: converte a sequência fonêmica + o
// estilo de voz em contornos de duração/energia/pitch por fonema.
// A engine de síntese NÃO decide prosódia — o planner entrega tudo.

import (
	"math"
	"strings"
)

// ── VoiceStyle: a personalidade vocal, SEPARADA da engine ───────────────
type VoiceStyle struct {
	PitchMean      float64 // Hz base
	PitchRange     float64 // variação em semitons
	SpeakingRate   float64 // sílabas por segundo
	Energy         float64 // ganho base (0..1)
	Warmth         float64 // 0..1 — suavidade dos formantes (filtro)
	PauseFactor    float64 // multiplicador de pausas
	Expressiveness float64 // 0..1 — amplitude do contorno
}

// Estilos prontos da casa.
var (
	StyleCoscaExecutive = VoiceStyle{PitchMean: 110, PitchRange: 4, SpeakingRate: 4.2, Energy: 0.85, Warmth: 0.6, PauseFactor: 1.0, Expressiveness: 0.5}
	StyleCoscaCalm      = VoiceStyle{PitchMean: 105, PitchRange: 3, SpeakingRate: 3.4, Energy: 0.7, Warmth: 0.8, PauseFactor: 1.3, Expressiveness: 0.35}
	StyleCoscaCinema    = VoiceStyle{PitchMean: 100, PitchRange: 6, SpeakingRate: 3.0, Energy: 0.9, Warmth: 0.7, PauseFactor: 1.5, Expressiveness: 0.8}
	StyleCoscaAssistant = VoiceStyle{PitchMean: 118, PitchRange: 5, SpeakingRate: 4.6, Energy: 0.8, Warmth: 0.5, PauseFactor: 0.8, Expressiveness: 0.45}
)

// ── Intenção da frase: o Professor manda interpretar, não ler ───────────
type Intencao int

const (
	IntencaoAfirmacao Intencao = iota
	IntencaoPergunta
	IntencaoComando
	IntencaoEnumeracao
)

// detectarIntencao: pontuação e palavra inicial definem o contorno.
func detectarIntencao(texto string) Intencao {
	t := strings.TrimSpace(texto)
	switch {
	case strings.HasSuffix(t, "?"):
		return IntencaoPergunta
	case strings.HasPrefix(t, "chef,") || strings.HasPrefix(t, "chefe,") ||
		strings.HasPrefix(t, "don,") || strings.HasPrefix(t, "don "):
		return IntencaoComando
	}
	return IntencaoAfirmacao
}

// ── ProsodicWord: palavra com atributos prosódicos ──────────────────────
type ProsodicWord struct {
	Text     string
	Phonemes []Phoneme
	Stress   float64 // peso da ênfase (0..1)
	Pitch    float64 // desvio de pitch (semitons, +/−)
	Energy   float64 // ganho relativo (0.5..1.5)
	Duration float64 // multiplicador de duração (0.6..1.6)
}

// ── ProsodyPlan: a saída do planner ─────────────────────────────────────
type ProsodyPlan struct {
	Words    []ProsodicWord
	Pauses   []int   // índice do fonema onde entra pausa (ms)
	Contour  []float64 // pitch em Hz por fonema
	Duration []float64 // duração por fonema (ms)
	Energy   []float64 // energia por fonema (0..1)
}

// PlanejarProsodia: texto + fonemas + estilo → plano prosódico completo.
func PlanejarProsodia(texto string, fonemas []Phoneme, estilo VoiceStyle) ProsodyPlan {
	intencao := detectarIntencao(texto)
	plan := ProsodyPlan{}

	// 1) agrupa em palavras
	var atual []Phoneme
	var palavras []ProsodicWord
	textoPalavras := Tokenize(texto)
	wi := 0
	for _, f := range fonemas {
		if f.Symbol == " " {
			if len(atual) > 0 {
				word := ProsodicWord{Phonemes: atual}
				if wi < len(textoPalavras) {
					word.Text = textoPalavras[wi]
				}
				wi++
				palavras = append(palavras, word)
				atual = nil
			}
			continue
		}
		atual = append(atual, f)
	}
	if len(atual) > 0 {
		palavras = append(palavras, ProsodicWord{Phonemes: atual})
	}
	plan.Words = palavras

	// 2) ênfase: palavra com vogal tônica mais longa = foco (heurística
	// simples: última palavra de conteúdo, ou palavra após "não"/"sim")
	// Distribui pelo tamanho da palavra (palavras maiores = mais peso).
	n := len(palavras)
	for i := range palavras {
		peso := 0.25 + 0.15*float64(len(palavras[i].Phonemes))
		if i == n-1 {
			peso += 0.2 // fim de frase
		}
		palavras[i].Stress = math.Min(1.0, peso)
	}

	// 3) contornos por intenção
	//   afirmação: declinação suave (F0 cai no fim)
	//   pergunta:  subida final (F0 sobe no fim)
	//   comando:   platô alto + queda forte no fim
	//   enumeração: ondas regulares
	plano := &plan
	plano.Contour = make([]float64, len(fonemas))
	plano.Duration = make([]float64, len(fonemas))
	plano.Energy = make([]float64, len(fonemas))

	// percorre o slice REAL de fonemas (pausas contam como posições)
	wi = 0
	for i, f := range fonemas {
		if f.Symbol == " " {
			// pausa: silêncio com duração do planner
			plano.Contour[i] = estilo.PitchMean
			plano.Duration[i] = 90 * estilo.PauseFactor
			plano.Energy[i] = 0
			wi++ // avança para a próxima palavra
			continue
		}
		// palavra atual = wi (palavras[wi].Phonemes contém f)
		progresso := 0.0
		if n > 1 {
			progresso = float64(wi) / float64(n-1)
		}
		stressPalavra := 0.25 + 0.15*float64(len(palavras[wi].Phonemes))
		if wi == n-1 {
			stressPalavra += 0.2
		}
		stressPalavra = math.Min(1.0, stressPalavra)
		palavras[wi].Stress = stressPalavra

		base := estilo.PitchMean
		switch intencao {
		case IntencaoPergunta:
			base += estilo.PitchRange * progresso * 0.9
		case IntencaoComando:
			base += estilo.PitchRange * 0.4 * math.Sin(progresso*math.Pi)
		case IntencaoEnumeracao:
			base += estilo.PitchRange * 0.5 * math.Sin(progresso*math.Pi*2)
		default: // afirmação: declinação
			base -= estilo.PitchRange * 0.5 * progresso
		}
		// ênfase da palavra
		base += stressPalavra * estilo.PitchRange * 0.35
		plano.Contour[i] = base

		// duração por classe
		ms := duracaoBase(f)
		ms *= 1.0 + stressPalavra*0.3
		ms *= estilo.SpeakingRate / 4.0
		plano.Duration[i] = ms

		// energia
		e := estilo.Energy * (0.85 + 0.3*stressPalavra)
		if f.Stress {
			e *= 1.15
		}
		plano.Energy[i] = math.Min(1.0, e)
	}
	return *plano
}

// duracaoBase: duração típica (ms) por classe de fonema — o Professor:
// "não use valores fixos, o contexto decide". Aqui é a BASE; o planner
// multiplica por ênfase/taxa.
func duracaoBase(f Phoneme) float64 {
	switch f.Dur {
	case DurVowelLong:
		return 110
	case DurVowelShort:
		return 70
	case DurNasal:
		return 105
	case DurStop:
		return 55
	case DurFricative:
		return 80
	case DurNasalCons:
		return 75
	case DurLiquid:
		return 60
	case DurGlide:
		return 55
	case DurPause:
		return 90
	}
	return 70
}

func maxi(a, b int) int {
	if a > b {
		return a
	}
	return b
}