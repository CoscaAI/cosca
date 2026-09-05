// Package fusion — Peça (2) da arquitetura: FUSÃO + DETECÇÃO DE CONTRADIÇÃO.
//
// Doutrina (professor+Don, 2026-09-02): o valor do COSCA não está em cada
// sensor, mas no kernel decidir no que confiar, combinar as evidências e saber
// quando uma resposta NÃO é confiável. A fusão é o passo que transforma um
// conjunto de observações dispersas em CONSENSO ou CONFLITO.
//
// Dois eixos são separados (correção dos exemplos mal-ajustados):
//   - REFERENTE (a identidade: "Las Vegas", "botão-voltar", "frame 847") —
//     é o que pode ser corroborado e contradito.
//   - PREDICADO (o atributo: "é destination", "é text", "é clickable") —
//     outra dimensão, não colide com o referente.
//
// Confiança de consenso = COMBINAÇÃO DE EVIDÊNCIA INDEPENDENTE (log-odds),
// não média. Dois sensores concordando SOBEM; uma discordância forte derruba.
//
// CONTRADIÇÃO vive entre REFERENTES CONCORRENTES no MESMO SLOT (mesma região/
// posição), não dentro de um grupo. "Las Vegas" vs "Los Angeles" na mesma
// região são hipóteses mutuamente exclusivas — o kernel NÃO escolhe um
// vencedor silenciosamente; a fusão sinaliza o conflito.
package fusion

import (
	"math"
	"strings"

	"github.com/CoscaAI/cosca/internal/sensor"
)

// Referent é a identidade de um grupo de observações sobre a MESMA coisa.
// É a chave de fusão: o que está sendo afirmado, não o atributo.
type Referent string

// Predicate é o atributo/declaração sobre o referente (ex: "destination",
// "text", "clickable").
type Predicate string

// Slot é a posição/região onde referentes concorrentes disputam. Duas
// observações no mesmo slot com conteúdos diferentes = contradição.
type Slot string

// Result é o resultado da fusão: consensos (grupos por referente) +
// contradições entre referentes concorrentes no mesmo slot.
type Result struct {
	// Consensus são os grupos fundidos por referente (consenso ou uniparental).
	Consensus []Fused `json:"consensus"`
	// Contradictions são os SLOTS onde há hipóteses mutuamente exclusivas.
	// O kernel deve escalar (Peça 3), não escolher vencedor.
	Contradictions []Contradiction `json:"contradictions,omitempty"`
}

// Fused é o consenso de um referente, após fundir as observações que falam
// sobre ele.
type Fused struct {
	// Referent é a identidade do grupo ("las vegas", "botão-voltar").
	Referent Referent `json:"referent"`
	// Predicate é o atributo comum mais forte do grupo ("" se não aplicável).
	Predicate Predicate `json:"predicate,omitempty"`
	// Support é o número de fontes independentes que corroboram.
	Support int `json:"support"`
	// Confidence é a confiança do CONSENSO em [0,1], combinada por evidência
	// independente (log-odds). NUNCA é a média das confianças.
	Confidence float64 `json:"confidence"`
	// Epistemic é a classe mais forte entre os membros do grupo.
	Epistemic sensor.EpistemicState `json:"epistemic"`
	// Members são as observações que formam o consenso (defensive copy).
	Members []sensor.Observation `json:"-"`
}

// Contradiction é um conjunto de referentes mutuamente exclusivos num slot.
type Contradiction struct {
	// Slot é a posição/região onde os referentes disputam.
	Slot Slot `json:"slot"`
	// Referents são as hipóteses concorrentes (ex: ["las vegas","los angeles"]).
	Referents []Referent `json:"referents"`
	// Confidence é a confiança média das hipóteses (todas críveis).
	Confidence float64 `json:"confidence"`
}

// Fuse agrupa observações por referente e detecta contradições entre
// referentes concorrentes no mesmo slot. Usa estratégia padrão de derivação
// de referente e slot (content normalizado; região para pré-posição).
func Fuse(obs []sensor.Observation) Result {
	return FuseWith(obs, defaultReferent, defaultSlot)
}

// FuseWith agrupa por referente usando refOf e detecta conflitos por slot
// usando slotOf. Permite regras de identidade/posição específicas.
func FuseWith(obs []sensor.Observation, refOf func(sensor.Observation) Referent, slotOf func(sensor.Observation) Slot) Result {
	if len(obs) == 0 {
		return Result{}
	}

	// Agrupa por referente.
	groups := make(map[Referent][]sensor.Observation)
	var order []Referent
	for _, o := range obs {
		r := refOf(o)
		if _, ok := groups[r]; !ok {
			order = append(order, r)
		}
		groups[r] = append(groups[r], o)
	}

	consensus := make([]Fused, 0, len(groups))
	for _, r := range order {
		consensus = append(consensus, fuseGroup(r, groups[r]))
	}

	// Detecta contradição entre referentes concorrentes no mesmo slot.
	contradictions := detectContradictions(obs, refOf, slotOf)

	return Result{Consensus: consensus, Contradictions: contradictions}
}

// fuseGroup funde um grupo de observações sobre um único referente.
func fuseGroup(ref Referent, members []sensor.Observation) Fused {
	f := Fused{Referent: ref, Members: append([]sensor.Observation(nil), members...)}

	f.Predicate = dominantPredicate(members)
	f.Epistemic = strongestEpistemic(members)
	f.Support = independentSources(members)
	f.Confidence = combineConfidence(members)

	return f
}

// combineConfidence combina as confianças de um grupo por evidência
// independente (log-odds): converte cada confiança em logit, soma, sigmoid.
// Concordância sobe; um único sensor não infla.
func combineConfidence(members []sensor.Observation) float64 {
	if len(members) == 0 {
		return 0
	}
	if len(members) == 1 {
		return clamp01(float64(members[0].Confidence))
	}
	var sum float64
	for _, m := range members {
		sum += logit(clamp01(float64(m.Confidence)))
	}
	return clamp01(sigmoid(sum))
}

// logit é o log-odds de p: log(p/(1-p)). p em (0,1).
func logit(p float64) float64 {
	if p <= 0 {
		return -10 // conservador: evita -Inf
	}
	if p >= 1 {
		return 10 // conservador: evita +Inf
	}
	return math.Log(p / (1 - p))
}

// sigmoid é a função logística 1/(1+e^-x).
func sigmoid(x float64) float64 {
	return 1 / (1 + math.Exp(-x))
}

// clamp01 limita v ao intervalo [0,1].
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// strongestEpistemic devolve a classe epistêmica mais forte do grupo.
func strongestEpistemic(members []sensor.Observation) sensor.EpistemicState {
	rank := map[sensor.EpistemicState]int{
		sensor.EpistemicEVIDENCE: 3,
		sensor.EpistemicMEASURED: 2,
		sensor.EpistemicINFERRED: 1,
		sensor.EpistemicDECISION: 0,
	}
	best := sensor.EpistemicDECISION
	bestRank := -1
	for _, m := range members {
		if r, ok := rank[m.Epistemic]; ok && r > bestRank {
			best = m.Epistemic
			bestRank = r
		}
	}
	return best
}

// independentSources conta as fontes (sensors) distintos que corroboram.
// Dois frames do MESMO sensor contam como 1 fonte.
func independentSources(members []sensor.Observation) int {
	seen := make(map[string]struct{})
	for _, m := range members {
		key := m.Source
		if key == "" {
			key = "__anon_" + m.Content
		}
		seen[key] = struct{}{}
	}
	return len(seen)
}

// dominantPredicate escolhe o Kind mais frequente que descreve o referente.
func dominantPredicate(members []sensor.Observation) Predicate {
	counts := make(map[Predicate]int)
	for _, m := range members {
		if m.Kind != "" {
			counts[Predicate(m.Kind)]++
		}
	}
	best := Predicate("")
	bestN := -1
	for k, n := range counts {
		if n > bestN {
			best = k
			bestN = n
		}
	}
	return best
}

// detectContradictions acha slots com referentes mutuamente exclusivos.
// Duas observações com CONTEÚDOS DIFERENTES no mesmo slot (mesma região) e
// ambas com confiança superior ao limiar de credibilidade = contradição.
// Modalidades diferentes não conflitam (dimensões complementares).
func detectContradictions(obs []sensor.Observation, refOf func(sensor.Observation) Referent, slotOf func(sensor.Observation) Slot) []Contradiction {
	const minConf = 0.5 // abaixo disso é ruído, não hipótese crível

	// Mapa slot -> mapa conteudo -> lista de obs.
	bySlot := make(map[Slot]map[sensor.Modality]map[string][]sensor.Observation)
	var slotOrder []Slot
	for _, o := range obs {
		if float64(o.Confidence) < minConf {
			continue // ruído não disputa
		}
		sl := slotOf(o)
		if _, ok := bySlot[sl]; !ok {
			bySlot[sl] = make(map[sensor.Modality]map[string][]sensor.Observation)
			slotOrder = append(slotOrder, sl)
		}
		if bySlot[sl][o.Modality] == nil {
			bySlot[sl][o.Modality] = make(map[string][]sensor.Observation)
		}
		bySlot[sl][o.Modality][o.Content] = append(bySlot[sl][o.Modality][o.Content], o)
	}

	var out []Contradiction
	for _, sl := range slotOrder {
		// Só há contradição se DENTRO DA MESMA MODALIDADE há conteúdos
		// diferentes no mesmo slot. Modalidades diferentes (texto vs visual)
		// são dimensões complementares, não hipóteses concorrentes.
		for _, byContent := range bySlot[sl] {
			if len(byContent) <= 1 {
				// Uma única afirmação por modalidade → sem conflito.
				continue
			}
			contents := make(map[string]bool)
			maxConf := 0.0
			var refs []Referent
			for c := range byContent {
				if c == "" {
					continue
				}
				contents[c] = true
				first := byContent[c][0]
				for _, o := range byContent[c] {
					if float64(o.Confidence) > maxConf {
						maxConf = float64(o.Confidence)
					}
				}
				refs = append(refs, refOf(first))
			}
			if len(contents) > 1 {
				out = append(out, Contradiction{Slot: sl, Referents: refs, Confidence: maxConf})
			}
		}
	}
	return out
}

// defaultReferent deriva o referente: para texto é o conteúdo normalizado;
// para outros usa conteúdo + região quando a região diferencia.
func defaultReferent(o sensor.Observation) Referent {
	if o.Modality == sensor.ModalityText {
		return Referent(strings.ToLower(strings.TrimSpace(o.Content)))
	}
	if o.Region != nil && o.Content != "" {
		return Referent(strings.ToLower(strings.TrimSpace(o.Content)) + "@" +
			itoa(o.Region.X) + "," + itoa(o.Region.Y))
	}
	return Referent(strings.ToLower(strings.TrimSpace(o.Content)))
}

// defaultSlot deriva o slot (posição) de uma observação. A região define o
// slot; se não houver região, o slot é a modalidade (conservador).
func defaultSlot(o sensor.Observation) Slot {
	if o.Region != nil {
		return Slot(itoa(o.Region.X) + "," + itoa(o.Region.Y) + "," +
			itoa(o.Region.W) + "," + itoa(o.Region.H))
	}
	return Slot("modal_" + string(o.Modality))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
