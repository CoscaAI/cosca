// Package vector — AUTOPSY #3 · EXPERIMENTO BLOCKING × WORKERS (nada de produção).
//
// Hipótese a validar: o throughput do InMemoryIndex (regime ~12,4 Mvec/s,
// ~38 GB/s, próximo do piso de leitura) pode mudar via BLOCKING/TILING do
// slab float32 contíguo e/ou pelo nº de workers no pool de score.
//
// Estes kernels são CÓPIAS de benchmark que consomem o MESMO snapshot do
// índice (gen.vecs SoA contíguo) e a MESMA query do caminho de produção.
// Nenhuma linha de produção é alterada; a matemática é preservada EXATA
// (mesma ordem de somas por linha, acumulação float64 na mesma ordem) →
// paridade bit-exact com o oracle (gen.score).
//
// Matriz: Block B ∈ {32,64,128,256,512} × Workers W ∈ {4,8,12,16} = 20
// configurações, mais o baseline (scoreParallel de produção, W=NumCPU=16).
//
// Estrutura do kernel de blocking: loop EXTERNO por bloco de B vetores,
// loop do meio por vetor no bloco, loop interno por dimensão. O ponto é a
// localidade do bloco (L1/L2/L3); a aritmética por linha é bit-identica a
// scoreRange (index.go).
package vector

import (
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"sort"
	"sync"
	"testing"
)

// blockSizeCandidates é a dimensão B da matriz (blocos de vetores).
var blockSizeCandidates = []int{32, 64, 128, 256, 512}

// blockWorkerCandidates é a dimensão W da matriz (goroutines no pool).
var blockWorkerCandidates = []int{4, 8, 12, 16}

// blockBytesPerVec é o tamanho de uma linha do slab (dim×4B) — usado para o
// GB/s efetivo (Mvec/s × 3072 B), igual ao relatório do baseline.
const blockBytesPerVec = 3072 // 768 dims × 4 bytes float32

// blockScoreRange scores rows [lo,hi) serialmente iterando o dataset em
// blocos de `block` vetores: loop externo por bloco, do meio por vetor, mais
// interno por dimensão. A aritmética por linha é a MESMA de scoreRange
// (mesma ordem de somas) → bit-exact; só o aninhamento de loops muda.
func blockScoreRange(g *indexGeneration, query []float64, lo, hi, limit, block int, sqrtNormA float64) []scoredRow {
	dim := g.dim
	vecs := g.vecs
	ids := g.ids
	top := make([]scoredRow, 0, limit)
	for bs := lo; bs < hi; bs += block {
		be := bs + block
		if be > hi {
			be = hi
		}
		for i := bs; i < be; i++ {
			base := i * dim
			var dot, nb float64
			for d := 0; d < dim; d++ {
				v := float64(vecs[base+d])
				dot += query[d] * v
				nb += v * v
			}
			// Paridade com dotFromBytes: linhas de norma zero pontuam 0.0.
			score := 0.0
			if nb != 0 && sqrtNormA != 0 {
				score = dot / (sqrtNormA * math.Sqrt(nb))
			}
			if math.IsNaN(score) {
				continue
			}
			sr := scoredRow{id: ids[i], score: score}
			if len(top) < limit {
				top = insertScored(top, sr)
			} else if sr.score > top[len(top)-1].score {
				top = insertScored(top, sr)
			}
		}
	}
	return top
}

// blockScoreParallel replica a arquitetura de scoreParallel de produção
// (chunking do dataset entre W workers + janela top-K por worker + merge por
// sort descendente), com cada worker usando o kernel de blocking. W=0 →
// NumCPU. Mesma matemática e mesmo merge → top-K global preservado.
func blockScoreParallel(g *indexGeneration, query []float64, limit, workers, block int, sqrtNormA float64) []scoredRow {
	n := g.n
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers > n {
		workers = n
	}
	chunk := (n + workers - 1) / workers

	var mu sync.Mutex
	collected := make([]scoredRow, 0, limit*workers)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		lo := w * chunk
		if lo >= n {
			break
		}
		hi := lo + chunk
		if hi > n {
			hi = n
		}
		wg.Add(1)
		go func(lo, hi int) {
			defer wg.Done()
			local := blockScoreRange(g, query, lo, hi, limit, block, sqrtNormA)
			mu.Lock()
			collected = append(collected, local...)
			mu.Unlock()
		}(lo, hi)
	}
	wg.Wait()

	if len(collected) == 0 {
		return nil
	}
	sort.Slice(collected, func(a, b int) bool { return collected[a].score > collected[b].score })
	if len(collected) > limit {
		collected = collected[:limit]
	}
	return collected
}

// ── FASE 1 · PARIDADE (obrigatória) ─────────────────────────────────────────
//
// O top-K de cada configuração (B×W) deve dar os MESMOS ids, na MESMA ordem,
// com os MESMOS scores (bit-exact) do caminho de produção (oracle: gen.score).

func TestBlockingParityFullMatrix(t *testing.T) {
	store := autopsyStore(t, autopsyDim, 100000)
	store.indexEnabled.Store(true)
	q := autopsyQuery(autopsyDim)
	if _, err := store.Search(q, 10); err != nil {
		t.Fatalf("warm: %v", err)
	}
	gen := store.index.gen.Load()
	var normA float64
	for _, v := range q {
		normA += v * v
	}
	sqrtNormA := math.Sqrt(normA)
	oracle := gen.score(q, 10)
	if len(oracle) != 10 {
		t.Fatalf("oracle: len=%d want 10", len(oracle))
	}

	for _, block := range blockSizeCandidates {
		for _, workers := range blockWorkerCandidates {
			got := blockScoreParallel(gen, q, 10, workers, block, sqrtNormA)
			if len(got) != len(oracle) {
				t.Fatalf("B=%d W=%d: len=%d want %d", block, workers, len(got), len(oracle))
			}
			for i := range oracle {
				if got[i].id != oracle[i].id || got[i].score != oracle[i].score {
					t.Fatalf("B=%d W=%d[%d]: (%s,%v) want (%s,%v)", block, workers, i, got[i].id, got[i].score, oracle[i].id, oracle[i].score)
				}
			}
		}
	}
}

// TestBlockingParitySmallSerial cobre o caminho n<256 (scoreRange serial de
// produção) com um dataset pequeno de dim reduzida — paridade dos kernels de
// blocking também nesse regime.
func TestBlockingParitySmallSerial(t *testing.T) {
	store := idxStore(t, 8, 200) // n<256 → produção usa scoreRange serial
	q := idxRandVec(rand.New(rand.NewSource(99)), 8)
	store.indexEnabled.Store(true)
	if _, err := store.Search(q, 10); err != nil {
		t.Fatalf("warm: %v", err)
	}
	gen := store.index.gen.Load()
	var normA float64
	for _, v := range q {
		normA += v * v
	}
	sqrtNormA := math.Sqrt(normA)
	oracle := gen.score(q, 10)

	for _, block := range blockSizeCandidates {
		// workers > n é clampado a n dentro do kernel; cobre o caminho serial.
		got := blockScoreParallel(gen, q, 10, 4, block, sqrtNormA)
		if len(got) != len(oracle) {
			t.Fatalf("B=%d: len=%d want %d", block, len(got), len(oracle))
		}
		for i := range oracle {
			if got[i].id != oracle[i].id || got[i].score != oracle[i].score {
				t.Fatalf("B=%d[%d]: (%s,%v) want (%s,%v)", block, i, got[i].id, got[i].score, oracle[i].id, oracle[i].score)
			}
		}
	}
}

// ── FASE 2 · MATRIZ DE BENCHMARK (5×4 + baseline) ───────────────────────────
//
// Todos os kernels consomem o MESMO snapshot e a MESMA query (warm). Medidas
// por configuração: ns/op (default), B/op, allocs/op (benchmem), Mvec/s e
// GB/s (ReportMetric).

func BenchmarkBlockingMatrix_N100000_Dim768_L10(b *testing.B) {
	store := autopsyStore(b, autopsyDim, 100000)
	store.indexEnabled.Store(true)
	q := autopsyQuery(autopsyDim)
	if _, err := store.Search(q, 10); err != nil {
		b.Fatalf("warm: %v", err)
	}
	gen := store.index.gen.Load()
	var normA float64
	for _, v := range q {
		normA += v * v
	}
	sqrtNormA := math.Sqrt(normA)

	report := func(b *testing.B) {
		b.ReportMetric(float64(100000)*float64(b.N)/b.Elapsed().Seconds()/1e6, "Mvec/s")
		b.ReportMetric(float64(100000)*blockBytesPerVec*float64(b.N)/b.Elapsed().Seconds()/1e9, "GB/s")
	}

	// Baseline: scoreParallel de produção (W=NumCPU=16), sob as MESMAS condições.
	b.Run("baseline_prod", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			r := gen.score(q, 10)
			if len(r) != 10 {
				b.Fatalf("results %d want 10", len(r))
			}
		}
		report(b)
	})

	for _, block := range blockSizeCandidates {
		for _, workers := range blockWorkerCandidates {
			block, workers := block, workers
			b.Run(fmt.Sprintf("B%d_W%d", block, workers), func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					r := blockScoreParallel(gen, q, 10, workers, block, sqrtNormA)
					if len(r) != 10 {
						b.Fatalf("results %d want 10", len(r))
					}
				}
				report(b)
			})
		}
	}
}
