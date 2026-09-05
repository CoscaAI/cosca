// F3 do ADR-019 — SignalSet: os 3 sinais de um arquivo, pré-computados, para a
// busca de código NÃO re-ler o arquivo nem recalcular a cada consulta.
//
// Uma SignalSet guarda o embedding unigram, o bigram e a assinatura MinHash de
// um trecho de código. Index pre-computa por arquivo; a busca só computa a
// SignalSet da QUERY (uma vez) e compõe com cada arquivo via Fuse. Determinístico
// (I1), zero LLM/zero rede.
package codeembed

// SignalSet agrega os sinais de similaridade de um trecho de código.
type SignalSet struct {
	Unigram []float64
	Bigram  []float64
	MinHash []uint32
}

// Signals computa os 3 sinais de um trecho de código (dimensão `dim`).
func Signals(code string, dim int) SignalSet {
	if dim <= 0 {
		dim = DefaultDim
	}
	return SignalSet{
		Unigram: Embed(code, dim),
		Bigram:  BigramEmbed(code, dim),
		MinHash: MinHashSketch(code),
	}
}

// Fuse compõe dois SignalSet em uma pontuação ponderada [0,1].
// weights (opcional, 3 entradas) default 0.4/0.3/0.3 — mesmo do FuseSimilarity.
func Fuse(a, b SignalSet, weights ...float64) float64 {
	s1 := Similarity(a.Unigram, b.Unigram)
	s2 := Similarity(a.Bigram, b.Bigram)
	s3 := Jaccard(a.MinHash, b.MinHash)

	w := [3]float64{0.4, 0.3, 0.3}
	if len(weights) >= 3 {
		copy(w[:], weights[:3])
	}
	return clamp01(s1*w[0] + s2*w[1] + s3*w[2])
}
