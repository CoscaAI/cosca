// Package vector — FASE 3/4/5/12 · EXPERIMENTOS A/B no caminho InMemoryIndex.
//
// Nenhum destes kernels altera produção. Cada candidato é benchmarkado contra
// o baseline e só promovido a index.go após: speedup real + paridade bit-exact
// com o caminho SQL (oracle) + race limpo.
//
// Candidatos nesta rodada:
//   - expRangeTopK      → top-K por worker via inserção em janela pequena
//     (elimina a alocação de chunk inteiro + o sort O(chunk log chunk)).
//   - expNorms          → normas pré-computadas por linha (corta metade das
//     FMAs por dimensão; aritmética na MESMA ordem → bit-exact).
//   - expUnroll4        → desenrolamento x4 PRESERVANDO a ordem sequencial das
//     somas (bit-exact por construção).
package vector

import (
	"math"
	"math/rand"
	"runtime"
	"sort"
	"sync"
	"testing"
)

// insertTopK insere sr em `top` (janela descendente por score, cap = limit),
// descartando o pior quando cheia. Retorna a janela resultante.
func insertTopK(top []scoredRow, sr scoredRow) []scoredRow {
	i := len(top)
	for i > 0 && top[i-1].score < sr.score {
		i--
	}
	if i >= cap(top) {
		return top // cheia e sr não entra
	}
	if len(top) < cap(top) {
		top = top[:len(top)+1]
	}
	copy(top[i+1:], top[i:len(top)-1])
	top[i] = sr
	return top
}

// expRangeTopK scores [lo,hi) mantendo só o top-K via janela pequena.
func expRangeTopK(g *indexGeneration, query []float64, lo, hi, limit int, sqrtNormA float64) []scoredRow {
	dim := g.dim
	vecs := g.vecs
	ids := g.ids
	top := make([]scoredRow, 0, limit)
	for i := lo; i < hi; i++ {
		base := i * dim
		var dot, nb float64
		for d := 0; d < dim; d++ {
			v := float64(vecs[base+d])
			dot += query[d] * v
			nb += v * v
		}
		score := 0.0
		if nb != 0 && sqrtNormA != 0 {
			score = dot / (sqrtNormA * math.Sqrt(nb))
		}
		if math.IsNaN(score) {
			continue
		}
		sr := scoredRow{id: ids[i], score: score}
		if len(top) < limit {
			top = insertTopK(top, sr)
		} else if sr.score > top[len(top)-1].score {
			top = insertTopK(top, sr)
		}
	}
	return top
}

// expScoreParallelTopK é a réplica de scoreParallel com top-K por worker.
func expScoreParallelTopK(g *indexGeneration, query []float64, limit, workers int, sqrtNormA float64) []scoredRow {
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
			local := expRangeTopK(g, query, lo, hi, limit, sqrtNormA)
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

// expPrecomputeNorms calcula ‖row‖² de cada linha na MESMA ordem de acumulação
// do caminho SQL → valor bit-exact ao nb computado no loop.
func expPrecomputeNorms(g *indexGeneration) []float64 {
	norms := make([]float64, g.n)
	for i := 0; i < g.n; i++ {
		base := i * g.dim
		var nb float64
		for d := 0; d < g.dim; d++ {
			v := float64(g.vecs[base+d])
			nb += v * v
		}
		norms[i] = nb
	}
	return norms
}

// expRangeNorms como expRangeTopK mas usando normas pré-computadas (só dot no
// loop quente). Score bit-exact: nb pré-computado na mesma ordem.
func expRangeNorms(g *indexGeneration, query []float64, norms []float64, lo, hi, limit int, sqrtNormA float64) []scoredRow {
	dim := g.dim
	vecs := g.vecs
	ids := g.ids
	top := make([]scoredRow, 0, limit)
	for i := lo; i < hi; i++ {
		base := i * dim
		var dot float64
		for d := 0; d < dim; d++ {
			dot += query[d] * float64(vecs[base+d])
		}
		nb := norms[i]
		score := 0.0
		if nb != 0 && sqrtNormA != 0 {
			score = dot / (sqrtNormA * math.Sqrt(nb))
		}
		if math.IsNaN(score) {
			continue
		}
		sr := scoredRow{id: ids[i], score: score}
		if len(top) < limit {
			top = insertTopK(top, sr)
		} else if sr.score > top[len(top)-1].score {
			top = insertTopK(top, sr)
		}
	}
	return top
}

// expRangeUnroll4 desenrola x4 PRESERVANDO a ordem sequencial das somas
// (dot += ... na ordem d, d+1, d+2, d+3) → bit-exact por construção.
func expRangeUnroll4(g *indexGeneration, query []float64, lo, hi, limit int, sqrtNormA float64) []scoredRow {
	dim := g.dim
	vecs := g.vecs
	ids := g.ids
	top := make([]scoredRow, 0, limit)
	for i := lo; i < hi; i++ {
		base := i * dim
		var dot, nb float64
		d := 0
		for ; d+4 <= dim; d += 4 {
			dot += query[d] * float64(vecs[base+d])
			dot += query[d+1] * float64(vecs[base+d+1])
			dot += query[d+2] * float64(vecs[base+d+2])
			dot += query[d+3] * float64(vecs[base+d+3])
			nb += float64(vecs[base+d]) * float64(vecs[base+d])
			nb += float64(vecs[base+d+1]) * float64(vecs[base+d+1])
			nb += float64(vecs[base+d+2]) * float64(vecs[base+d+2])
			nb += float64(vecs[base+d+3]) * float64(vecs[base+d+3])
		}
		for ; d < dim; d++ {
			v := float64(vecs[base+d])
			dot += query[d] * v
			nb += v * v
		}
		score := 0.0
		if nb != 0 && sqrtNormA != 0 {
			score = dot / (sqrtNormA * math.Sqrt(nb))
		}
		if math.IsNaN(score) {
			continue
		}
		sr := scoredRow{id: ids[i], score: score}
		if len(top) < limit {
			top = insertTopK(top, sr)
		} else if sr.score > top[len(top)-1].score {
			top = insertTopK(top, sr)
		}
	}
	return top
}

// expScoreParallelNorms como expScoreParallelTopK com normas + topK.
func expScoreParallelNorms(g *indexGeneration, query []float64, norms []float64, limit, workers int, sqrtNormA float64) []scoredRow {
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
			local := expRangeNorms(g, query, norms, lo, hi, limit, sqrtNormA)
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

// expScoreSerialTopK variante serial (n<256) com top-K por janela.
func expScoreSerialTopK(g *indexGeneration, query []float64, limit int, sqrtNormA float64) []scoredRow {
	return expRangeTopK(g, query, 0, g.n, limit, sqrtNormA)
}

// expScoreParallelTopKUnroll4 é expScoreParallelTopK com o dot desenrolado x4
// (ordem preservada → bit-exact). Serial quando n < 256.
func expScoreParallelTopKUnroll4(g *indexGeneration, query []float64, limit, workers int, sqrtNormA float64) []scoredRow {
	if g.n < 256 {
		return expRangeUnroll4(g, query, 0, g.n, limit, sqrtNormA)
	}
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
			local := expRangeUnroll4(g, query, lo, hi, limit, sqrtNormA)
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

// expScoreParallelNormsUnroll4 combina normas pré-computadas + unroll x4.
func expScoreParallelNormsUnroll4(g *indexGeneration, query []float64, norms []float64, limit, workers int, sqrtNormA float64) []scoredRow {
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
			local := expRangeNormsUnroll4(g, query, norms, lo, hi, limit, sqrtNormA)
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

// expRangeNormsUnroll4 como expRangeNorms com o dot desenrolado x4 (ordem
// preservada).
func expRangeNormsUnroll4(g *indexGeneration, query []float64, norms []float64, lo, hi, limit int, sqrtNormA float64) []scoredRow {
	dim := g.dim
	vecs := g.vecs
	ids := g.ids
	top := make([]scoredRow, 0, limit)
	for i := lo; i < hi; i++ {
		base := i * dim
		var dot float64
		d := 0
		for ; d+4 <= dim; d += 4 {
			dot += query[d] * float64(vecs[base+d])
			dot += query[d+1] * float64(vecs[base+d+1])
			dot += query[d+2] * float64(vecs[base+d+2])
			dot += query[d+3] * float64(vecs[base+d+3])
		}
		for ; d < dim; d++ {
			dot += query[d] * float64(vecs[base+d])
		}
		nb := norms[i]
		score := 0.0
		if nb != 0 && sqrtNormA != 0 {
			score = dot / (sqrtNormA * math.Sqrt(nb))
		}
		if math.IsNaN(score) {
			continue
		}
		sr := scoredRow{id: ids[i], score: score}
		if len(top) < limit {
			top = insertTopK(top, sr)
		} else if sr.score > top[len(top)-1].score {
			top = insertTopK(top, sr)
		}
	}
	return top
}

// ── FASE 13 · validação de paridade dos kernels experimentais ──────────────

func TestExpKernelsParitySerial(t *testing.T) {
	store := idxStore(t, 8, 200) // n<256 → serial
	q := idxRandVec(rand.New(rand.NewSource(21)), 8)
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
	ref := gen.score(q, 10)
	norms := expPrecomputeNorms(gen)

	for _, k := range []struct {
		name string
		got  []scoredRow
	}{
		{"topK", expScoreSerialTopK(gen, q, 10, sqrtNormA)},
		{"unroll4", expRangeUnroll4(gen, q, 0, gen.n, 10, sqrtNormA)},
		{"norms", expRangeNorms(gen, q, norms, 0, gen.n, 10, sqrtNormA)},
		{"normsUnroll4", expRangeNormsUnroll4(gen, q, norms, 0, gen.n, 10, sqrtNormA)},
	} {
		if len(k.got) != len(ref) {
			t.Fatalf("%s: len=%d want %d", k.name, len(k.got), len(ref))
		}
		for i := range ref {
			if k.got[i].id != ref[i].id || k.got[i].score != ref[i].score {
				t.Fatalf("%s[%d]: (%s,%v) want (%s,%v)", k.name, i, k.got[i].id, k.got[i].score, ref[i].id, ref[i].score)
			}
		}
	}
}

func TestExpKernelsParityParallel(t *testing.T) {
	store := idxStore(t, 8, 1000) // n>=256 → parallel
	q := idxRandVec(rand.New(rand.NewSource(22)), 8)
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
	ref := gen.score(q, 10)
	norms := expPrecomputeNorms(gen)

	for _, k := range []struct {
		name string
		got  []scoredRow
	}{
		{"topK", expScoreParallelTopK(gen, q, 10, 0, sqrtNormA)},
		{"unroll4", expScoreParallelTopKUnroll4(gen, q, 10, 0, sqrtNormA)},
		{"norms", expScoreParallelNorms(gen, q, norms, 10, 0, sqrtNormA)},
		{"normsUnroll4", expScoreParallelNormsUnroll4(gen, q, norms, 10, 0, sqrtNormA)},
	} {
		if len(k.got) != len(ref) {
			t.Fatalf("%s: len=%d want %d", k.name, len(k.got), len(ref))
		}
		for i := range ref {
			if k.got[i].id != ref[i].id || k.got[i].score != ref[i].score {
				t.Fatalf("%s[%d]: (%s,%v) want (%s,%v)", k.name, i, k.got[i].id, k.got[i].score, ref[i].id, ref[i].score)
			}
		}
	}
}

func TestExpKernelsParityLargeDim(t *testing.T) {
	store := idxStore(t, 768, 300)
	q := idxRandVec(rand.New(rand.NewSource(23)), 768)
	store.indexEnabled.Store(true)
	if _, err := store.Search(q, 50); err != nil {
		t.Fatalf("warm: %v", err)
	}
	gen := store.index.gen.Load()
	var normA float64
	for _, v := range q {
		normA += v * v
	}
	sqrtNormA := math.Sqrt(normA)
	norms := expPrecomputeNorms(gen)

	// limit=50 cruzando vários limits; compara com o caminho SQL como oracle.
	store.indexEnabled.Store(false)
	sqlRes, err := store.Search(q, 50)
	if err != nil {
		t.Fatalf("sql: %v", err)
	}
	store.indexEnabled.Store(true)
	for _, k := range []struct {
		name string
		got  []scoredRow
	}{
		{"ref-index", gen.score(q, 50)},
		{"topK", expScoreParallelTopK(gen, q, 50, 0, sqrtNormA)},
		{"norms", expScoreParallelNorms(gen, q, norms, 50, 0, sqrtNormA)},
	} {
		if len(k.got) != len(sqlRes) {
			t.Fatalf("%s: len=%d want %d", k.name, len(k.got), len(sqlRes))
		}
		for i := range sqlRes {
			if k.got[i].id != sqlRes[i].ID || k.got[i].score != sqlRes[i].Score {
				t.Fatalf("%s[%d]: (%s,%v) want (%s,%v)", k.name, i, k.got[i].id, k.got[i].score, sqlRes[i].ID, sqlRes[i].Score)
			}
		}
	}
}

func TestExpInsertTopK(t *testing.T) {
	// janela vazia
	top := make([]scoredRow, 0, 3)
	top = insertTopK(top, scoredRow{id: "a", score: 0.5})
	if len(top) != 1 || top[0].id != "a" {
		t.Fatalf("insert into empty: %+v", top)
	}
	// preenche
	top = insertTopK(top, scoredRow{id: "b", score: 0.9})
	top = insertTopK(top, scoredRow{id: "c", score: 0.1})
	// cheia → só entra quem vence o pior
	top = insertTopK(top, scoredRow{id: "d", score: 0.8})
	if len(top) != 3 || top[0].id != "b" || top[1].id != "d" || top[2].id != "a" {
		t.Fatalf("after full insert: %+v", top)
	}
	top = insertTopK(top, scoredRow{id: "e", score: 0.05})
	if len(top) != 3 || top[2].id != "a" {
		t.Fatalf("loser must not enter: %+v", top)
	}
	// ordem descendente
	for i := 1; i < len(top); i++ {
		if top[i-1].score < top[i].score {
			t.Fatalf("not descending: %+v", top)
		}
	}
}

// ── FASE 10 · piso de leitura (memória) vs kernel completo ─────────────────
//
// expReadFloor: soma bruta de todos os floats do slab, sem query, sem nb, sem
// top-K, sem ids → mede a largura de banda de leitura pura do array contíguo.
// Se o kernel completo ficar perto deste piso, o gargalo é MEMÓRIA (e não
// faz sentido gastar mais ALU). Se ficar longe, há ALU/latência a atacar.
func expReadFloor(g *indexGeneration) float64 {
	vecs := g.vecs
	var acc float64
	for _, v := range vecs {
		acc += float64(v)
	}
	return acc
}

// expDotOnly: dot + nb (mesmo ALU do kernel real) mas sem top-K/ids/sort.
func expDotOnly(g *indexGeneration, query []float64, lo, hi int) (dotOut, nbOut float64) {
	dim := g.dim
	vecs := g.vecs
	var dot, nb float64
	for i := lo; i < hi; i++ {
		base := i * dim
		for d := 0; d < dim; d++ {
			v := float64(vecs[base+d])
			dot += query[d] * v
			nb += v * v
		}
	}
	return dot, nb
}

func BenchmarkExpReadFloor_N100000_Dim768(b *testing.B) {
	store := autopsyStore(b, autopsyDim, 100000)
	store.indexEnabled.Store(true)
	q := autopsyQuery(autopsyDim)
	if _, err := store.Search(q, 10); err != nil {
		b.Fatalf("warm: %v", err)
	}
	gen := store.index.gen.Load()
	var acc float64
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		acc += expReadFloor(gen)
	}
	b.ReportMetric(float64(100000)*float64(autopsyDim)*4*float64(b.N)/b.Elapsed().Seconds()/1e9, "GB/s")
	b.ReportMetric(float64(100000)*float64(b.N)/b.Elapsed().Seconds()/1e6, "Mvec/s")
	_ = acc
}

func BenchmarkExpDotOnly_N100000_Dim768(b *testing.B) {
	store := autopsyStore(b, autopsyDim, 100000)
	store.indexEnabled.Store(true)
	q := autopsyQuery(autopsyDim)
	if _, err := store.Search(q, 10); err != nil {
		b.Fatalf("warm: %v", err)
	}
	gen := store.index.gen.Load()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dot, nb := expDotOnly(gen, q, 0, gen.n)
		_ = dot
		_ = nb
	}
	b.ReportMetric(float64(100000)*float64(b.N)/b.Elapsed().Seconds()/1e6, "Mvec/s")
}

// expDotOnlyParallel: réplica paralela do kernel real (16 workers) mas sem
// top-K, sem ids, sem sort → isola o custo de ler + 2 FMA por dim do custo
// de manutenção do top-K/merge.
func expDotOnlyParallel(g *indexGeneration, query []float64, workers int) float64 {
	n := g.n
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers > n {
		workers = n
	}
	chunk := (n + workers - 1) / workers
	var mu sync.Mutex
	var acc float64
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
			d, nb := expDotOnly(g, query, lo, hi)
			mu.Lock()
			acc += d + nb
			mu.Unlock()
		}(lo, hi)
	}
	wg.Wait()
	return acc
}

func BenchmarkExpDotOnlyParallel_N100000_Dim768(b *testing.B) {
	store := autopsyStore(b, autopsyDim, 100000)
	store.indexEnabled.Store(true)
	q := autopsyQuery(autopsyDim)
	if _, err := store.Search(q, 10); err != nil {
		b.Fatalf("warm: %v", err)
	}
	gen := store.index.gen.Load()
	var acc float64
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		acc += expDotOnlyParallel(gen, q, 0)
	}
	b.ReportMetric(float64(100000)*float64(b.N)/b.Elapsed().Seconds()/1e6, "Mvec/s")
	_ = acc
}

// expReadFloorParallel: leitura pura (soma do slab) em 16 workers.
func expReadFloorParallel(g *indexGeneration, workers int) float64 {
	n := g.n
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	chunk := (n + workers - 1) / workers
	vecs := g.vecs
	dim := g.dim
	var mu sync.Mutex
	var acc float64
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
			var a float64
			for i := lo; i < hi; i++ {
				base := i * dim
				for d := 0; d < dim; d++ {
					a += float64(vecs[base+d])
				}
			}
			mu.Lock()
			acc += a
			mu.Unlock()
		}(lo, hi)
	}
	wg.Wait()
	return acc
}

func BenchmarkExpReadFloorParallel_N100000_Dim768(b *testing.B) {
	store := autopsyStore(b, autopsyDim, 100000)
	store.indexEnabled.Store(true)
	q := autopsyQuery(autopsyDim)
	if _, err := store.Search(q, 10); err != nil {
		b.Fatalf("warm: %v", err)
	}
	gen := store.index.gen.Load()
	var acc float64
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		acc += expReadFloorParallel(gen, 0)
	}
	b.ReportMetric(float64(100000)*float64(autopsyDim)*4*float64(b.N)/b.Elapsed().Seconds()/1e9, "GB/s")
	b.ReportMetric(float64(100000)*float64(b.N)/b.Elapsed().Seconds()/1e6, "Mvec/s")
	_ = acc
}

// expRangeTopKLazyID: como expRangeTopK mas lendo ids[i] SOMENTE quando a
// linha entra na janela (evita 2 streams de memória entrelaçados no loop).
func expRangeTopKLazyID(g *indexGeneration, query []float64, lo, hi, limit int, sqrtNormA float64) []scoredRow {
	dim := g.dim
	vecs := g.vecs
	ids := g.ids
	top := make([]scoredRow, 0, limit)
	for i := lo; i < hi; i++ {
		base := i * dim
		var dot, nb float64
		for d := 0; d < dim; d++ {
			v := float64(vecs[base+d])
			dot += query[d] * v
			nb += v * v
		}
		score := 0.0
		if nb != 0 && sqrtNormA != 0 {
			score = dot / (sqrtNormA * math.Sqrt(nb))
		}
		if math.IsNaN(score) {
			continue
		}
		if len(top) < limit {
			top = insertTopK(top, scoredRow{id: ids[i], score: score})
		} else if score > top[len(top)-1].score {
			top = insertTopK(top, scoredRow{id: ids[i], score: score})
		}
	}
	return top
}

func expScoreParallelTopKLazyID(g *indexGeneration, query []float64, limit, workers int, sqrtNormA float64) []scoredRow {
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
			local := expRangeTopKLazyID(g, query, lo, hi, limit, sqrtNormA)
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

func BenchmarkExpTopKLazyID_N100000_Dim768_L10(b *testing.B) {
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
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = expScoreParallelTopKLazyID(gen, q, 10, 0, sqrtNormA)
	}
	b.ReportMetric(float64(100000)*float64(b.N)/b.Elapsed().Seconds()/1e6, "Mvec/s")
}

// ── harness dos benchmarks de experimento ──────────────────────────────────

type expKernel int

const (
	expKernelBaseline expKernel = iota // scoreParallel atual (production)
	expKernelTopK
	expKernelNorms
	expKernelUnroll4
	expKernelNormsUnroll4
)

func benchExpKernel(b *testing.B, count, limit, workers int, kernel expKernel) {
	store := autopsyStore(b, autopsyDim, count)
	store.indexEnabled.Store(true)
	q := autopsyQuery(autopsyDim)
	if _, err := store.Search(q, limit); err != nil {
		b.Fatalf("warm search: %v", err)
	}
	gen := store.index.gen.Load()
	var normA float64
	for _, v := range q {
		normA += v * v
	}
	sqrtNormA := math.Sqrt(normA)

	var norms []float64
	if kernel == expKernelNorms || kernel == expKernelNormsUnroll4 {
		norms = expPrecomputeNorms(gen)
	}

	var check []scoredRow
	switch kernel {
	case expKernelBaseline:
		check = gen.score(q, limit)
	case expKernelTopK:
		check = expScoreParallelTopK(gen, q, limit, workers, sqrtNormA)
	case expKernelNorms:
		check = expScoreParallelNorms(gen, q, norms, limit, workers, sqrtNormA)
	case expKernelUnroll4:
		check = expScoreParallelTopKUnroll4(gen, q, limit, workers, sqrtNormA)
	case expKernelNormsUnroll4:
		check = expScoreParallelNormsUnroll4(gen, q, norms, limit, workers, sqrtNormA)
	}
	if len(check) != minInt(limit, gen.n) {
		b.Fatalf("kernel sanity: got %d want %d", len(check), minInt(limit, gen.n))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var r []scoredRow
		switch kernel {
		case expKernelBaseline:
			r = gen.score(q, limit)
		case expKernelTopK:
			r = expScoreParallelTopK(gen, q, limit, workers, sqrtNormA)
		case expKernelNorms:
			r = expScoreParallelNorms(gen, q, norms, limit, workers, sqrtNormA)
		case expKernelUnroll4:
			r = expScoreParallelTopKUnroll4(gen, q, limit, workers, sqrtNormA)
		case expKernelNormsUnroll4:
			r = expScoreParallelNormsUnroll4(gen, q, norms, limit, workers, sqrtNormA)
		}
		if len(r) != len(check) {
			b.Fatalf("kernel result instability")
		}
	}
	b.ReportMetric(float64(count)*float64(b.N)/b.Elapsed().Seconds()/1e6, "Mvec/s")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func benchExpWorkers(b *testing.B, count, limit, workers int) {
	benchExpKernel(b, count, limit, workers, expKernelTopK)
}

// ── benchmarks dos experimentos ────────────────────────────────────────────

func BenchmarkExpTopK_N100000_Dim768_L10(b *testing.B) {
	benchExpKernel(b, 100000, 10, 0, expKernelTopK)
}
func BenchmarkExpBaseline_N100000_Dim768_L10(b *testing.B) {
	benchExpKernel(b, 100000, 10, 0, expKernelBaseline)
}
func BenchmarkExpNorms_N100000_Dim768_L10(b *testing.B) {
	benchExpKernel(b, 100000, 10, 0, expKernelNorms)
}
func BenchmarkExpUnroll4_N100000_Dim768_L10(b *testing.B) {
	benchExpKernel(b, 100000, 10, 0, expKernelUnroll4)
}
func BenchmarkExpNormsUnroll4_N100000_Dim768_L10(b *testing.B) {
	benchExpKernel(b, 100000, 10, 0, expKernelNormsUnroll4)
}

func BenchmarkExpBaseline_N10000_Dim768_L10(b *testing.B) {
	benchExpKernel(b, 10000, 10, 0, expKernelBaseline)
}
func BenchmarkExpTopK_N10000_Dim768_L10(b *testing.B) {
	benchExpKernel(b, 10000, 10, 0, expKernelTopK)
}
func BenchmarkExpNorms_N10000_Dim768_L10(b *testing.B) {
	benchExpKernel(b, 10000, 10, 0, expKernelNorms)
}
func BenchmarkExpNormsUnroll4_N10000_Dim768_L10(b *testing.B) {
	benchExpKernel(b, 10000, 10, 0, expKernelNormsUnroll4)
}

func BenchmarkExpBaseline_N1000000_Dim768_L10(b *testing.B) {
	if testing.Short() {
		b.Skip("1M store in short mode")
	}
	benchExpKernel(b, 1000000, 10, 0, expKernelBaseline)
}
func BenchmarkExpTopK_N1000000_Dim768_L10(b *testing.B) {
	if testing.Short() {
		b.Skip("1M store in short mode")
	}
	benchExpKernel(b, 1000000, 10, 0, expKernelTopK)
}

// ── FASE 4 · ponto ótimo de paralelismo ────────────────────────────────────

func BenchmarkExpWorkers_W1(b *testing.B)  { benchExpWorkers(b, 100000, 10, 1) }
func BenchmarkExpWorkers_W2(b *testing.B)  { benchExpWorkers(b, 100000, 10, 2) }
func BenchmarkExpWorkers_W4(b *testing.B)  { benchExpWorkers(b, 100000, 10, 4) }
func BenchmarkExpWorkers_W8(b *testing.B)  { benchExpWorkers(b, 100000, 10, 8) }
func BenchmarkExpWorkers_W12(b *testing.B) { benchExpWorkers(b, 100000, 10, 12) }
func BenchmarkExpWorkers_W16(b *testing.B) { benchExpWorkers(b, 100000, 10, 16) }

func BenchmarkExpWorkers10K_W1(b *testing.B)  { benchExpWorkers(b, 10000, 10, 1) }
func BenchmarkExpWorkers10K_W4(b *testing.B)  { benchExpWorkers(b, 10000, 10, 4) }
func BenchmarkExpWorkers10K_W8(b *testing.B)  { benchExpWorkers(b, 10000, 10, 8) }
func BenchmarkExpWorkers10K_W16(b *testing.B) { benchExpWorkers(b, 10000, 10, 16) }
