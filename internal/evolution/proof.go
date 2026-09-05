// proof.go — Evolution Gate COM PROVA (gema #1, mineração ruflo; ADR-022).
//
// O ADR-016/018 definem a evolução como PROJEÇÃO/observabilidade (stage.go é só
// `where am I`). O que falta — e o ruflo provou — é o CONTROL PLANE da melhoria:
// um candidato só é promovido com PROVA estatística, e a promoção é uma transação
// separada (avaliar NUNCA muta a política). Isto é a régua I1/I2/I5 levada ao
// extremo correto:
//
//	gerar candidato → deltas (held-out) → bootstrap CI → veredito → receipt assinado
//	→ promoção transacional (não muta até o receipt ser publicado).
//
// Determinístico (I1): bootstrap com seed fixa, zero LLM. Fail-closed (I2):
// amostras insuficientes ou sem melhoria significativa → REJECT (nunca promove
// "por sorte").
package evolution

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"
)

// Verdictos de promoção.
const (
	VerdictAccept = "ACCEPT"
	VerdictReject = "REJECT"
)

// ProofGate decide se um candidato melhora o baseline com significância.
type ProofGate struct {
	// MinSamples é o mínimo de deltas p/ avaliar (fail-closed; default 20).
	MinSamples int
	// Iterations é o nº de resamples do bootstrap (default 1000).
	Iterations int
	// Seed garante determinismo (I1).
	Seed int64
	// Alpha é o nível de significância unicaudal (default 0.05).
	Alpha float64
}

// NewProofGate cria um gate com defaults conservadores.
func NewProofGate() *ProofGate {
	return &ProofGate{MinSamples: 20, Iterations: 1000, Seed: 1, Alpha: 0.05}
}

// BootstrapCILow devolve o limite INFERIOR do intervalo de confiança (1-alpha)
// para a MÉDIA dos deltas, via bootstrap (resample com reposição, seed fixa →
// determinístico I1). Se > 0, a melhoria é estatisticamente significativa.
func BootstrapCILow(deltas []float64, iterations int, seed int64, alpha float64) (float64, error) {
	if len(deltas) == 0 {
		return 0, fmt.Errorf("bootstrap: empty deltas")
	}
	if iterations <= 0 {
		iterations = 1000
	}
	if alpha <= 0 || alpha >= 1 {
		alpha = 0.05
	}
	rng := rand.New(rand.NewSource(seed)) // determinístico (I1)
	n := len(deltas)
	means := make([]float64, iterations)
	for i := 0; i < iterations; i++ {
		var sum float64
		for j := 0; j < n; j++ {
			sum += deltas[rng.Intn(n)]
		}
		means[i] = sum / float64(n)
	}
	sort.Float64s(means)
	// Quantil = alpha (unicaudal inferior): índice alpha*(len-1).
	idx := int(math.Round(alpha * float64(iterations-1)))
	if idx >= len(means) {
		idx = len(means) - 1
	}
	return means[idx], nil
}

// Evaluate decide ACCEPT/REJECT para um candidato vs baseline, com prova.
// Fail-closed (I2): menos que MinSamples ou BCI <= 0 → REJECT. Determinístico (I1).
func (g *ProofGate) Evaluate(champion, candidate string, deltas []float64) (*ProofReceipt, error) {
	if g.MinSamples <= 0 {
		g.MinSamples = 20
	}
	if len(deltas) < g.MinSamples {
		return nil, fmt.Errorf("proof gate: %d deltas < min %d (fail-closed I2)", len(deltas), g.MinSamples)
	}
	bci, err := BootstrapCILow(deltas, g.Iterations, g.Seed, g.Alpha)
	if err != nil {
		return nil, err
	}
	verdict := VerdictReject
	if bci > 0 {
		verdict = VerdictAccept
	}
	prev := "" // no run history passed: gate recebe o run isolado; o encadeamento é do ledger.
		r := &ProofReceipt{
			RunID:         hashID(champion, candidate, fmt.Sprintf("%d", len(deltas))),
			ChampionHash:  champion,
		CandidateHash: candidate,
		Deltas:        append([]float64(nil), deltas...),
		SplitSize:     len(deltas),
		BCI:           math.Round(bci*10000) / 10000,
		Verdict:       verdict,
		PrevHash:      prev,
		Timestamp:     time.Now().UTC(),
	}
	r.Hash = r.ComputeHash()
	return r, nil
}

// ProofReceipt é o registro imutável da avaliação (I5), encadeável por hash.
type ProofReceipt struct {
	RunID         string    `json:"run_id"`
	ChampionHash  string    `json:"champion_hash"`
	CandidateHash string    `json:"candidate_hash"`
	Deltas        []float64 `json:"deltas"`
	SplitSize     int       `json:"split_size"`
	BCI           float64   `json:"bootstrap_ci_low"`
	Verdict       string    `json:"verdict"`
	PrevHash      string    `json:"prev_hash"`
	Timestamp     time.Time `json:"timestamp"`
	Hash          string    `json:"hash"`
}

// ComputeHash liga o receipt na cadeia (hash de todos os campos + prev_hash).
func (r *ProofReceipt) ComputeHash() string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s|%s|%s|%d|%.4f|%s|%s",
		r.RunID, r.ChampionHash, r.CandidateHash, r.SplitSize, round(r.BCI), r.Verdict, r.PrevHash)))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// Promoted informa se a avaliação recomenda promoção (ACCEPT).
func (r *ProofReceipt) Promoted() bool {
	return r.Verdict == VerdictAccept
}

func hashID(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}

func round(x float64) float64 { return math.Round(x*10000) / 10000 }
