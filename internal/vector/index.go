// Package vector — In-memory full-scan index (otimização #1).
//
// O gargalo provado pela autópsia (L311) é ARQUITETURAL: o full-scan re-decodifica
// 100% do store via SQLite (scanAll) a cada query (~94% da latência; o dot em si
// é ~5-10% do total). Este arquivo adiciona um índice DERIVADO em memória:
//
//	scanAll (SQLite + clone de BLOB)  →  NÃO roda na query quente.
//	dot sobre array contíguo float32   →  roda a cada query (SoA, zero decode).
//
// Design:
//   - Layout Structure-of-Arrays: um único slab []float32 (n*dim) + []string de
//     ids. Nada de fatia de fatias — cache-friendly e SEM alocação por row.
//   - O índice é uma SNAPSHOT imutável por geração. Leitores pegam o ponteiro
//     atômico UMA vez e o usam sem lock (swap de geração é atômico).
//   - Invalidação: o SQLiteVec mantém um contador de versão (s.version), bumpado
//     em toda escrita COMMITTED. O snapshot só é válido se gen.version == s.version;
//     senão, recarrega sob reloadMu (um escritor de recarga por vez).
//   - Fail-safe: o índice NUNCA é fonte de verdade. Se o load falhar, a busca
//     cai no caminho SQL original (fallback). Se o load for interrompido por uma
//     escrita concorrente, descarta e tenta de novo (versão instável).
package vector

import (
	"encoding/binary"
	"fmt"
	"math"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog/log"
)

// maxIndexReloadAttempts bounds reload retries when the store keeps mutating
// while the snapshot is being built. Exhausting it is fail-safe: the caller
// falls back to the SQL scan for that query.
const maxIndexReloadAttempts = 3

// indexGeneration is an immutable snapshot of the store's vectors at a given
// version. All fields are read-only after construction; a search may hold the
// pointer without any lock.
type indexGeneration struct {
	// version is the store version this snapshot was built from. A snapshot is
	// valid only while it matches the store's current version.
	version uint64
	// ids[i] is the store id of row i.
	ids []string
	// vecs is the Structure-of-Arrays slab: row i's dim floats are
	// vecs[i*dim : (i+1)*dim].
	vecs []float32
	// vecs8 is the quantized slab (fast path, L339): row i's dim int8 values at
	// vecs8[i*dimPad : (i+1)*dimPad] (zero-padded to a multiple of 16 so the
	// AVX2 kernel can consume it). Quantization: round(v*127) clamped to
	// [-127,127]. The bias convention is applied at score time (qb=q+128).
	vecs8 []int8
	// sums8[i] is Σ vecs8[row i] — the per-row correction for the 128·Σr bias.
	sums8 []int32
	// nb[i] is ‖row i‖² computed from the EXACT float32 values (so the final
	// normalization is not degraded by quantization).
	nb []float32
	// vecs16 is the int16 slab (L347/FASE A): row i's dim int16 values at
	// vecs16[i*dimPad16 : (i+1)*dimPad16] (zero-padded to a multiple of 32 —
	// the dot16AVX2 kernel contract). Quantization: round(v·1024) clamped to
	// [-1023,1023] (grade 2^10 — safe for i32 accumulation at dim ≤ 2048).
	vecs16   []int16
	dim      int
	dimPad   int
	dimPad16 int
	n        int
}

// memoryBytes estimates the resident size of the generation (ids + vectors).
func (g *indexGeneration) memoryBytes() int64 {
	idsBytes := int64(0)
	for _, id := range g.ids {
		idsBytes += int64(len(id))
	}
	return idsBytes + int64(len(g.vecs))*4 + int64(len(g.vecs8)) + int64(len(g.sums8))*4 + int64(len(g.nb))*4 + int64(len(g.vecs16))*2
}

// quantize8 converts a float32 to the int8 lattice: round(v*127) clamped to
// [-127,127]. v=±1 maps to ±127 exactly; anything outside is clamped.
func quantize8(v float32) int8 {
	f := float64(v) * 127
	if f >= 127 {
		return 127
	}
	if f <= -127 {
		return -127
	}
	return int8(math.Round(f))
}

// quantizeQuery8 builds the query's biased uint8 slab: qb[i] = q[i]+128, with
// the tail (dim..dimPad) padded with 128 (q=0), which contributes zero against
// the zero-padded rows. The result has len dimPad (multiple of 16).
func quantizeQuery8(query []float64, dimPad int) []uint8 {
	qb := make([]uint8, dimPad)
	for d, v := range query {
		qb[d] = uint8(int(quantize8(float32(v))) + 128)
	}
	for d := len(query); d < dimPad; d++ {
		qb[d] = 128
	}
	return qb
}

// score returns the top `limit` rows by cosine similarity against query,
// mirroring the exact arithmetic (and skip conditions) of the SQL path so the
// results are bit-identical. nil when the query dimension mismatches the
// stored dimension (parity with the SQL path, which skips every row).
func (g *indexGeneration) score(query []float64, limit int) []scoredRow {
	if g == nil || g.n == 0 || len(query) != g.dim {
		return nil
	}
	var normA float64
	for _, v := range query {
		normA += v * v
	}
	if normA == 0 {
		return nil
	}
	sqrtNormA := math.Sqrt(normA)

	// Mirror the SQL path: serial for small sets, parallel for large ones.
	if g.n < 256 {
		return g.scoreRange(query, 0, g.n, limit, sqrtNormA)
	}
	return g.scoreParallel(query, limit, 0, sqrtNormA)
}

// scoreParallel splits rows across `workers` goroutines (0 = NumCPU) and merges
// the per-worker top-K, exactly like scoreParallel on candidateRows.
func (g *indexGeneration) scoreParallel(query []float64, limit, workers int, sqrtNormA float64) []scoredRow {
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
			local := g.scoreRange(query, lo, hi, limit, sqrtNormA)
			mu.Lock()
			collected = append(collected, local...)
			mu.Unlock()
		}(lo, hi)
	}
	wg.Wait()

	if len(collected) == 0 {
		return nil
	}
	sort.Slice(collected, func(a, b int) bool {
		// Tie-break determinístico (L363): score desc, id asc.
		if collected[a].score != collected[b].score {
			return collected[a].score > collected[b].score
		}
		return collected[a].id < collected[b].id
	})
	if len(collected) > limit {
		collected = collected[:limit]
	}
	return collected
}

// scoreRange scores rows [lo,hi) serially and returns the range's top-K,
// sorted descending. The per-row math is bit-identical to dotFromBytes
// (dot / (‖query‖·‖row‖) with float64 accumulation in the same order).
//
// The top-K is kept in a bounded window of size `limit` (sorted insertion,
// not a full-chunk sort): the global top-K of the union of per-worker windows
// is provably the global top-K (every row that can reach the final top-K is
// kept by its worker's window). This removes the O(chunk·log chunk) sort and
// the per-worker chunk-sized allocation that dominated the previous
// implementation — measured ~1.2-1.3x faster and ~2000x less allocated per
// search (see perf_baseline_test.go + relatório da rodada #2).
func (g *indexGeneration) scoreRange(query []float64, lo, hi, limit int, sqrtNormA float64) []scoredRow {
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
		// Parity with dotFromBytes: zero-norm rows score 0.0 and are kept.
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
	return top
}

// insertScored inserts sr into a descending top-K window whose capacity is
// the limit. When the window is full and sr beats the current minimum, the
// worst element is dropped. The result stays sorted descending, so the window
// is directly usable as the range's top-K. Equal scores keep insertion order
// (new equal-score rows land below existing ones), matching the reference
// full-sort truncation for the score-distinct data used by the oracle tests.
// scoredLess define a ordem canônica do ranking: score DESC, e para scores
// EXATAMENTE iguais, id ASC. Isso torna o top-K DETERMINÍSTICO (L351/L363):
// o desempate não depende da ordem de chegada dos workers paralelos.
func scoredLess(a, b scoredRow) bool {
	if a.score != b.score {
		return a.score < b.score // b vence a se score maior
	}
	return a.id > b.id // scores iguais: id menor vence (id asc)
}

// insertScored inserts sr into a descending top-K window whose capacity is
// the limit. When the window is full and sr beats the current minimum, the
// worst element is dropped. The result stays sorted by (score desc, id asc),
// so the window is directly usable as the range's top-K — independent of the
// insertion order (deterministic tie-break, L363).
func insertScored(top []scoredRow, sr scoredRow) []scoredRow {
	i := len(top)
	for i > 0 && scoredLess(top[i-1], sr) {
		i--
	}
	if i >= cap(top) {
		return top // window full and sr does not beat the minimum
	}
	if len(top) < cap(top) {
		top = top[:len(top)+1]
	}
	copy(top[i+1:], top[i:len(top)-1])
	top[i] = sr
	return top
}

// quantizeQuery16 builds the query's int16 slab on the grade 2^10 (dimPad16).
func quantizeQuery16(query []float64, dimPad16 int) []int16 {
	q16 := make([]int16, dimPad16)
	for d, v := range query {
		q16[d] = quantize16(float32(v))
	}
	return q16
}

// score16 is the int16 fast path (FASE A, L347): same contract as score, with
// the per-row dot on the int16 slab via dot16AVX2 (no bias — the i16×i16
// product fits i32 at grade 2^10). Recall ~float32 (0.990 real, L360).
func (g *indexGeneration) score16(query []float64, limit int) []scoredRow {
	if g == nil || g.n == 0 || len(query) != g.dim || g.vecs16 == nil {
		return nil
	}
	var normA float64
	for _, v := range query {
		normA += v * v
	}
	if normA == 0 {
		return nil
	}
	sqrtNormA := math.Sqrt(normA)
	q16 := quantizeQuery16(query, g.dimPad16)

	if g.n < 256 {
		return g.scoreRange16(q16, 0, g.n, limit, sqrtNormA)
	}
	return g.scoreParallel16(q16, limit, 0, sqrtNormA)
}

// scoreParallel16 mirrors scoreParallel8 for the int16 fast path.
func (g *indexGeneration) scoreParallel16(q16 []int16, limit, workers int, sqrtNormA float64) []scoredRow {
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
			local := g.scoreRange16(q16, lo, hi, limit, sqrtNormA)
			mu.Lock()
			collected = append(collected, local...)
			mu.Unlock()
		}(lo, hi)
	}
	wg.Wait()

	if len(collected) == 0 {
		return nil
	}
	sort.Slice(collected, func(a, b int) bool {
		// Tie-break determinístico (L363): score desc, id asc.
		if collected[a].score != collected[b].score {
			return collected[a].score > collected[b].score
		}
		return collected[a].id < collected[b].id
	})
	if len(collected) > limit {
		collected = collected[:limit]
	}
	return collected
}

// scoreRange16 scores rows [lo,hi) on the int16 slab:
//
//	dot16 = Σ q16·r16                    (dot16AVX2, grade 2^10)
//	score = dot16 / (1024² · ‖q‖ · ‖r‖)  (1048576 = 1024²)
//
// Same top-K window as scoreRange/scoreRange8.
func (g *indexGeneration) scoreRange16(q16 []int16, lo, hi, limit int, sqrtNormA float64) []scoredRow {
	dimPad16 := g.dimPad16
	vecs16 := g.vecs16
	ids := g.ids
	const scale = 1048576.0 // 1024 * 1024
	top := make([]scoredRow, 0, limit)
	for i := lo; i < hi; i++ {
		base := i * dimPad16
		dot16v := dot16(q16, vecs16[base:base+dimPad16])
		score := 0.0
		nb := float64(g.nb[i])
		if nb != 0 && sqrtNormA != 0 {
			score = float64(dot16v) / (scale * sqrtNormA * math.Sqrt(nb))
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
	return top
}

// score8 is the int8 fast path (L339): same contract as score, but the per-row
// dot runs on the quantized slab via the AVX2 kernel (dot8BiasAVX2) with the
// bias correction −128·Σr applied by dot8BiasDot. The final cosine score uses
// the EXACT float32 row norms (g.nb) and query norm, so only the dot term is
// quantized. Approximated by design — see TestInt8RecallAndScoreError for the
// measured recall vs the exact float32 path.
func (g *indexGeneration) score8(query []float64, limit int) []scoredRow {
	if g == nil || g.n == 0 || len(query) != g.dim || g.vecs8 == nil {
		return nil
	}
	var normA float64
	for _, v := range query {
		normA += v * v
	}
	if normA == 0 {
		return nil
	}
	sqrtNormA := math.Sqrt(normA)
	qb := quantizeQuery8(query, g.dimPad)

	if g.n < 256 {
		return g.scoreRange8(qb, 0, g.n, limit, sqrtNormA)
	}
	return g.scoreParallel8(qb, limit, 0, sqrtNormA)
}

// scoreParallel8 mirrors scoreParallel for the int8 fast path.
func (g *indexGeneration) scoreParallel8(qb []uint8, limit, workers int, sqrtNormA float64) []scoredRow {
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
			local := g.scoreRange8(qb, lo, hi, limit, sqrtNormA)
			mu.Lock()
			collected = append(collected, local...)
			mu.Unlock()
		}(lo, hi)
	}
	wg.Wait()

	if len(collected) == 0 {
		return nil
	}
	sort.Slice(collected, func(a, b int) bool {
		// Tie-break determinístico (L363): score desc, id asc.
		if collected[a].score != collected[b].score {
			return collected[a].score > collected[b].score
		}
		return collected[a].id < collected[b].id
	})
	if len(collected) > limit {
		collected = collected[:limit]
	}
	return collected
}

// scoreRange8 scores rows [lo,hi) on the quantized slab. Per row:
//
//	dot8 = Σ qb·rb − 128·Σr          (dot8BiasDot, kernel AVX2)
//	score = dot8 / (127² · ‖q‖ · ‖r‖)  (‖r‖ from the exact float32 norm)
//
// scale=16129=127² because q8=round(q·127) and r8=round(r·127), so
// Σq8·r8 ≈ 127²·Σq·r; dividing by 16129 brings the score back to the same
// unit as the float32 path (cosine similarity). Zero-norm rows score 0.0 and
// are kept, mirroring scoreRange.
func (g *indexGeneration) scoreRange8(qb []uint8, lo, hi, limit int, sqrtNormA float64) []scoredRow {
	dimPad := g.dimPad
	vecs8 := g.vecs8
	ids := g.ids
	const scale = 16129.0 // 127 * 127
	top := make([]scoredRow, 0, limit)
	for i := lo; i < hi; i++ {
		base := i * dimPad
		dot8 := dot8BiasDot(qb, vecs8[base:base+dimPad], g.sums8[i])
		score := 0.0
		nb := float64(g.nb[i])
		if nb != 0 && sqrtNormA != 0 {
			score = float64(dot8) / (scale * sqrtNormA * math.Sqrt(nb))
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
	return top
}

// InMemoryIndex manages the derived in-memory snapshot of a SQLiteVec store.
// It owns the generation pointer (atomic swap), the reload serialization, and
// the version-stability check. It is NEVER the source of truth: every reload
// reads committed rows from the store, and any failure degrades to the SQL scan.
type InMemoryIndex struct {
	store *SQLiteVec

	reloadMu sync.Mutex // serializes concurrent reloads (one builder at a time)
	gen      atomic.Pointer[indexGeneration]
	// int8Enabled gates the quantized AVX2 fast path (L339). When off, scoring
	// uses the exact float32 slab; the SQL scan remains the ultimate fail-safe.
	// Defaults to on (NewSQLiteVec, unless SQLiteVecConfig.DisableInt8).
	int8Enabled atomic.Bool
	// int16Enabled gates the int16 fast path (FASE A, L347) — recall ~float32
	// (0.990 real, L360) at ~2.18x float32 speed. Off by default: the decision
	// to promote it to default is the Don's (with the real-index numbers).
	int16Enabled atomic.Bool
}

// newInMemoryIndex wires an index to its store.
func newInMemoryIndex(s *SQLiteVec) *InMemoryIndex {
	return &InMemoryIndex{store: s}
}

// SetInt8Enabled toggles the quantized fast path at runtime.
func (ix *InMemoryIndex) SetInt8Enabled(on bool) {
	ix.int8Enabled.Store(on)
}

// SetInt16Enabled toggles the int16 fast path at runtime (FASE A, L347).
func (ix *InMemoryIndex) SetInt16Enabled(on bool) {
	ix.int16Enabled.Store(on)
}

// snapshot returns the current generation, reloading from the store when the
// snapshot is missing or stale (gen.version != store version). Callers must
// hold the store lock (RLock) — the version is read under it, so a committed
// write can never interleave with the reload (writes take the Lock).
//
// Returns nil when loading fails or the store keeps mutating mid-load: the
// caller then falls back to the SQL scan (fail-safe).
func (ix *InMemoryIndex) snapshot() *indexGeneration {
	gen := ix.gen.Load()
	if gen != nil && gen.version == ix.store.version {
		return gen
	}

	ix.reloadMu.Lock()
	defer ix.reloadMu.Unlock()

	for attempt := 0; attempt < maxIndexReloadAttempts; attempt++ {
		gen = ix.gen.Load()
		if gen != nil && gen.version == ix.store.version {
			return gen // another goroutine reloaded while we waited
		}
		startVersion := ix.store.version
		newGen, err := ix.loadGeneration()
		if err != nil {
			log.Warn().Err(err).Msg("in-memory vector index load failed; falling back to SQL scan")
			return nil
		}
		if startVersion == ix.store.version && newGen.version == ix.store.version {
			ix.gen.Store(newGen)
			return newGen
		}
		// The store version moved while we were building — the snapshot would
		// be stale. Discard and retry (bounded).
	}
	log.Warn().Msg("in-memory vector index reload thrashing; falling back to SQL scan for this query")
	return nil
}

// loadGeneration reads every vector row from the store and decodes it into the
// contiguous SoA slab. Rows whose BLOB length mismatches the store dimension
// are skipped — the same rows the SQL path skips at score time (parity).
func (ix *InMemoryIndex) loadGeneration() (*indexGeneration, error) {
	s := ix.store
	where, args := s.filterClause(nil)
	rows, err := s.scanAll(where, args)
	if err != nil {
		return nil, fmt.Errorf("index load scan: %w", err)
	}

	dim := s.dimension
	dimPad := (dim + 15) &^ 15 // múltiplo de 16 para o kernel AVX2
	n := len(rows)
	ids := make([]string, n)
	vecs := make([]float32, n*dim)
	kept := 0
	for _, r := range rows {
		if len(r.blob) != dim*4 {
			continue
		}
		ids[kept] = r.id
		base := kept * dim
		for d := 0; d < dim; d++ {
			vecs[base+d] = math.Float32frombits(binary.LittleEndian.Uint32(r.blob[d*4:]))
		}
		kept++
	}

	// Quantização int8 (fast path L339): round(v·127) clampado, pad 16 zerado,
	// Σr por row (correção de bias) e norma² EXATA em float32 por row.
	// Quantização int16 (FASE A, L347): round(v·1024) clampado, pad 32 zerado.
	dimPad16 := (dim + 31) &^ 31
	vecs8 := make([]int8, kept*dimPad)
	vecs16 := make([]int16, kept*dimPad16)
	sums8 := make([]int32, kept)
	nb := make([]float32, kept)
	for i := 0; i < kept; i++ {
		base := i * dim
		base8 := i * dimPad
		base16 := i * dimPad16
		var sum int32
		var nbsq float64
		for d := 0; d < dim; d++ {
			v := vecs[base+d]
			q := quantize8(v)
			vecs8[base8+d] = q
			vecs16[base16+d] = quantize16(v)
			sum += int32(q)
			nbsq += float64(v) * float64(v)
		}
		sums8[i] = sum
		nb[i] = float32(nbsq)
	}

	gen := &indexGeneration{
		version:  s.version,
		ids:      ids[:kept],
		vecs:     vecs[:kept*dim],
		vecs8:    vecs8,
		vecs16:   vecs16,
		sums8:    sums8,
		nb:       nb,
		dim:      dim,
		dimPad:   dimPad,
		dimPad16: dimPad16,
		n:        kept,
	}
	return gen, nil
}

// search scores the current snapshot against query and returns the top-K rows.
// Returns nil when there is no snapshot (empty store, load failure, or query
// dimension mismatch) — the caller decides whether to fall back to SQL.
// Routing: int16 fast path (when enabled) → int8 fast path (when enabled) →
// exact float32. SQL scan remains the ultimate fail-safe.
func (ix *InMemoryIndex) search(query []float64, limit int) []scoredRow {
	gen := ix.snapshot()
	if gen == nil {
		return nil
	}
	if ix.int16Enabled.Load() {
		if scores := gen.score16(query, limit); scores != nil {
			return scores
		}
	}
	if ix.int8Enabled.Load() {
		if scores := gen.score8(query, limit); scores != nil {
			return scores
		}
	}
	return gen.score(query, limit)
}

// invalidate drops the current snapshot, forcing the next search to reload.
// Used after writes that cannot prove commit ordering (e.g. external StoreTx
// callers that never notify StoreTxCommitted).
func (ix *InMemoryIndex) invalidate() {
	ix.gen.Store(nil)
}

// count returns the number of vectors in the current snapshot (0 until built).
func (ix *InMemoryIndex) count() int {
	gen := ix.gen.Load()
	if gen == nil {
		return 0
	}
	return gen.n
}

// memoryBytes returns the resident size of the current snapshot.
func (ix *InMemoryIndex) memoryBytes() int64 {
	gen := ix.gen.Load()
	if gen == nil {
		return 0
	}
	return gen.memoryBytes()
}
