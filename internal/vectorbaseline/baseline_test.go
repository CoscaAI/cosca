// Package vectorbaseline — instrumentação READ-ONLY do estado atual do índice
// vetorial de busca semântica (baseline de 2026-08-24).
//
// NÃO escreve, NÃO apaga, NÃO re-embedga o índice de produção. Abre o
// knowledge.db em mode=ro, lê os vetores BLOB float32, e replica em Go puro
// (matematicamente idêntico) os três caminhos de score do SQLiteVec:
//
//	float32 (ORACLE exato): cosine = dot/(‖q‖·‖r‖)
//	int16 (default de produção): dot16/(1024²·‖q‖·‖r‖)
//	int8  (fast path antigo):   dot8/(127²·‖q‖·‖r‖)
//
// Motivação: o kernel AVX2 faz a MESMA aritmética inteira (i16×i16→i32 e a
// correção de bias 128·Σr do int8) — a replicação em int64 é bit-idêntica ao
// resultado (dot16Pure é a referência portátil; o acumulador não estoura a
// dim 768). Assim obtemos recall/NDCG/distribuição SEM abrir o banco em modo
// de escrita e SEM depender de provider.
package vectorbaseline

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// ── Configuração ────────────────────────────────────────────────────────────

// kbPath resolve o caminho do knowledge.db (portas: env COSCA_KB_PATH, senão
// o default do repositório).
func kbPath(t testing.TB) string {
	t.Helper()
	if p := os.Getenv("COSCA_KB_PATH"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
		t.Logf("COSCA_KB_PATH=%s inexistente, usando default", p)
	}
	p := filepath.Join(os.Getenv("COSCA_ROOT"), ".cosca", "knowledge.db")
	if os.Getenv("COSCA_ROOT") == "" {
		// fallback: caminho relatível ao diretório de trabalho do teste.
		if cwd, err := os.Getwd(); err == nil {
			// ./internal/vectorbaseline -> suba 2 níveis até a raiz do repo.
			cands := []string{
				filepath.Join(cwd, "..", "..", ".cosca", "knowledge.db"),
				filepath.Join(cwd, "..", "..", "..", ".cosca", "knowledge.db"),
			}
			for _, c := range cands {
				if _, err := os.Stat(c); err == nil {
					return c
				}
			}
		}
	}
	return p
}

func openRO(t testing.TB) *sql.DB {
	t.Helper()
	p := kbPath(t)
	if _, err := os.Stat(p); err != nil {
		t.Skipf("sem knowledge.db em %s", p)
	}
	dsn := "file:" + strings.ReplaceAll(p, "\\", "/") + "?mode=ro"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open ro: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
	return db
}

// ── Modelo ──────────────────────────────────────────────────────────────────

type vrow struct {
	id, chunkID, documentID, content string
	vec                              []float64
	// pré-computos para os 3 caminhos de score
	f2   []float32 // float32 exato (ORACLE)
	q16  []int16   // int16 na grade 2^10
	q8   []int8    // int8 na grade 2^7
	nb   float64   // ‖r‖² exato em float32
	hash string    // sha256 do blob (dedup de vetor)
}

type scored struct {
	id    string
	score float64
}

func (r *vrow) scoreOracle(q []float64, qnorm float64) float64 {
	if r.nb == 0 || qnorm == 0 {
		return 0
	}
	var dot float64
	for d, v := range q {
		dot += v * float64(r.f2[d])
	}
	return dot / (qnorm * math.Sqrt(r.nb))
}

func (r *vrow) scoreInt16(q16 []int16, qnorm float64) float64 {
	if r.nb == 0 || qnorm == 0 {
		return 0
	}
	var dot int64
	for d := range q16 {
		dot += int64(q16[d]) * int64(r.q16[d])
	}
	return float64(dot) / (1048576.0 * qnorm * math.Sqrt(r.nb))
}

func (r *vrow) scoreInt8(qb []uint8, qnorm float64) float64 {
	if r.nb == 0 || qnorm == 0 {
		return 0
	}
	var dot, sum int64
	for d := range qb {
		rb := int64(r.q8[d])
		dot += int64(qb[d]) * rb
		sum += rb
	}
	return float64(dot-128*sum) / (16129.0 * qnorm * math.Sqrt(r.nb))
}

// ── Carregamento (read-only) ────────────────────────────────────────────────

func loadRows(t testing.TB, db *sql.DB) (rows []*vrow, badDim int) {
	t.Helper()
	q := `SELECT id, chunk_id, document_id, content, vector FROM vectors`
	rws, err := db.Query(q)
	if err != nil {
		t.Fatalf("query vectors: %v", err)
	}
	defer rws.Close()
	for rws.Next() {
		var id, cid, did, content string
		var blob []byte
		if err := rws.Scan(&id, &cid, &did, &content, &blob); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if len(blob) == 0 || len(blob)%4 != 0 || len(blob)/4 != 768 {
			badDim++
			continue
		}
		r := &vrow{id: id, chunkID: cid, documentID: did, content: content}
		r.f2 = make([]float32, 768)
		r.vec = make([]float64, 768)
		r.q16 = make([]int16, 768)
		r.q8 = make([]int8, 768)
		var nbsq float64
		for d := 0; d < 768; d++ {
			f := math.Float32frombits(binary.LittleEndian.Uint32(blob[d*4:]))
			r.f2[d] = f
			r.vec[d] = float64(f)
			r.q16[d] = quantize16(f)
			r.q8[d] = quantize8(f)
			nbsq += float64(f) * float64(f)
		}
		r.nb = nbsq
		h := sha256.Sum256(blob)
		r.hash = hex.EncodeToString(h[:])
		rows = append(rows, r)
	}
	if err := rws.Err(); err != nil {
		t.Fatalf("rows err: %v", err)
	}
	return rows, badDim
}

func quantize16(v float32) int16 {
	f := float64(v) * 1024
	if f >= 1024 {
		return 1023
	}
	if f <= -1024 {
		return -1023
	}
	return int16(math.Round(f))
}

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

// ── Score central (determinístico: score desc, id asc) ──────────────────────

func topK(rows []*vrow, score func(*vrow) float64, k int) []scored {
	out := make([]scored, 0, k)
	for _, r := range rows {
		s := score(r)
		if math.IsNaN(s) {
			continue
		}
		out = append(out, scored{id: r.id, score: s})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		return out[i].id < out[j].id
	})
	if len(out) > k {
		out = out[:k]
	}
	return out
}

// ── Métricas ────────────────────────────────────────────────────────────────

func recallAt(oracle, approx []scored, n int) float64 {
	if n > len(oracle) {
		n = len(oracle)
	}
	if n == 0 {
		return 1
	}
	ids := make(map[string]bool, n)
	for _, o := range oracle[:n] {
		ids[o.id] = true
	}
	hit := 0
	for _, g := range approx {
		if ids[g.id] {
			hit++
		}
	}
	return float64(hit) / float64(n)
}

func jaccard(oracle, approx []scored) float64 {
	os := make(map[string]bool, len(oracle))
	for _, o := range oracle {
		os[o.id] = true
	}
	in := 0
	union := len(os)
	for _, g := range approx {
		if os[g.id] {
			in++
		} else {
			union++
		}
	}
	if union == 0 {
		return 1
	}
	return float64(in) / float64(union)
}

func ndcgAt(oracle, approx []scored, n int) float64 {
	if n > len(approx) {
		n = len(approx)
	}
	if n == 0 {
		return 1
	}
	os := make(map[string]bool, len(oracle))
	for _, o := range oracle {
		os[o.id] = true
	}
	var dcg, idcg float64
	for i := 0; i < n; i++ {
		rel := 0.0
		if os[approx[i].id] {
			rel = 1
		}
		dcg += rel / math.Log2(float64(i+2))
		idcg += 1 / math.Log2(float64(i+2))
	}
	if idcg == 0 {
		return 0
	}
	return dcg / idcg
}

// ── Testes ──────────────────────────────────────────────────────────────────

func TestBaselineReadonly(t *testing.T) {
	db := openRO(t)
	defer db.Close()

	rows, badDim := loadRows(t, db)
	if len(rows) == 0 {
		t.Fatalf("nenhum vetor válido (dim=768) carregado")
	}
	t.Logf("── ESTADO DO ÍNDICE ──")
	t.Logf("vetores válidos (dim=768, len(blob)/4==768): %d   rows-dim-mismatch(puladas no load)=%d", len(rows), badDim)

	// ── Deduplicação ──────────────────────────────────────────────────────
	dupStats(t, rows)

	// ── Amostra de queries (self-match, conteúdo distinto) ───────────────
	qrows := sampleQueries(rows, t, 30)
	t.Logf("── QUERIES (self-match de conteúdo distinto): %d ──", len(qrows))

	runRecall(t, rows, qrows) // logs recall/NDCG/jaccard por int16 e int8

	// ── Distribuição de scores (ranking achatado?) ────────────────────────
	scoreDistribution(t, rows, qrows)

	// ── Real-text self-match via nomic (provider local) ───────────────────
	realTextSelfMatch(t, rows, qrows)
}

// ── Deduplicação (conteúdo + vetor, cross-documento) ────────────────────────

func dupStats(t testing.TB, rows []*vrow) {
	byVec := map[string][]*vrow{}
	byContent := map[string][]*vrow{}
	emptyContent := 0
	for _, r := range rows {
		byVec[r.hash] = append(byVec[r.hash], r)
		if strings.TrimSpace(r.content) == "" {
			emptyContent++
		}
		key := strings.TrimSpace(r.content)
		byContent[key] = append(byContent[key], r)
	}

	// vetor duplicado
	var vecNotDup int
	maxVecCopies := 0
	var maxVecKey string
	for k, v := range byVec {
		if len(v) == 1 {
			vecNotDup++
		}
		if len(v) > maxVecCopies {
			maxVecCopies = len(v)
			maxVecKey = k
		}
	}
	dupVec := len(rows) - vecNotDup
	t.Logf("── DUPLICATAS POR VETOR (sha256 do blob float32) ──")
	t.Logf("distintos=%d  duplicados=%d  pctDuplicado=%.1f%%  MAX CÓPIAS=%d (hash=%s)",
		len(byVec), dupVec, 100*float64(dupVec)/float64(len(rows)), maxVecCopies, maxVecKey[:12])

	// conteúdo duplicado
	contentGroups := 0
	var maxContentCopies, maxContentDocs int
	var maxContentKey string
	crossDocContent := 0
	gt10, gt1 := 0, 0
	var copies []int
	for k, v := range byContent {
		if k == "" {
			continue // não conta grupo de content vazio
		}
		contentGroups++
		n := len(v)
		copies = append(copies, n)
		if n > maxContentCopies {
			maxContentCopies = n
			maxContentKey = k
		}
		if n > 10 {
			gt10 += n
		}
		if n > 1 {
			gt1 += n
		}
		docs := map[string]bool{}
		for _, r := range v {
			docs[r.documentID] = true
		}
		if len(docs) > 1 {
			crossDocContent++
		}
		if len(docs) > maxContentDocs {
			maxContentDocs = len(docs)
		}
	}
	dupContentRows := len(rows) - contentGroups // linhas que pertencem a um grupo com >1 cópia não são "unique"; aproximação
	t.Logf("── DUPLICATAS POR CONTEÚDO (content, cross-documento) ──")
	t.Logf("content vazio=%d  conteúdos distintos(com texto)=%d  grupos cross-documento(>1 doc)=%d",
		emptyContent, contentGroups, crossDocContent)
	t.Logf("MAX cópias de UM conteúdo=%d  MAX documentos de UM conteúdo=%d  exemplo(len=%d): %q",
		maxContentCopies, maxContentDocs, len(maxContentKey), truncate(maxContentKey, 120))
	t.Logf("linhas em grupos >1 cópia=%d (%.1f%%)   em grupos >10 cópias=%d (%.1f%%)",
		gt1, 100*float64(gt1)/float64(len(rows)), gt10, 100*float64(gt10)/float64(len(rows)))
	sort.Ints(copies)
	if len(copies) > 0 {
		med := copies[len(copies)/2]
		p90 := copies[int(float64(len(copies))*0.9)]
		t.Logf("mediana de cópias por conteúdo=%d  p90=%d  grupos=%d", med, p90, len(copies))
	}
	// justamente: % de linhas que não são a 1ª ocorrência do seu conteúdo
	seen := map[string]bool{}
	dupContentRows = 0
	for _, r := range rows {
		k := strings.TrimSpace(r.content)
		if k == "" {
			continue
		}
		if seen[k] {
			dupContentRows++
		} else {
			seen[k] = true
		}
	}
	t.Logf("linhas duplicadas por conteúdo (2ª+ ocorrência)=%d  pctDuplicado=%.1f%%",
		dupContentRows, 100*float64(dupContentRows)/float64(len(rows)))
}

// ── Amostragem de queries (conteúdo distinto) ───────────────────────────────

func sampleQueries(rows []*vrow, t testing.TB, want int) []*vrow {
	seen := map[string]bool{}
	out := make([]*vrow, 0, want)
	// embaralha um pouco para não amostrar só os primeiros rowid.
	rng := rand.New(rand.NewSource(20260824))
	idx := rng.Perm(len(rows))
	for _, i := range idx {
		r := rows[i]
		k := strings.TrimSpace(r.content)
		if k == "" {
			continue
		}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, r)
		if len(out) == want {
			break
		}
	}
	return out
}

// ── Recall int16/int8 vs float32 oracle ─────────────────────────────────────

func runRecall(t testing.TB, rows, qrows []*vrow) (r16, r8, ndcg16, ndcg8, jac16, jac8 float64) {
	const K = 50
	n := 0
	byID := make(map[string]*vrow, len(rows))
	for _, r := range rows {
		byID[r.id] = r
	}
	var r15, r110, r150, r85, r810, r850, d1650, d850 float64
	var rr16_10, rr8_10 float64 // ranking recall@10 estrito (top-10 vs top-10)
	var divContent, divDocs float64
	var top1Changed16, top1Changed8, top1Empate16, top1Empate8 int
	var maxErr16, maxErr8, sumErr16, sumErr8 float64
	var oracleNearTie float64

	for _, q := range qrows {
		q16 := make([]int16, 768)
		qb := make([]uint8, 768)
		var qnorm float64
		for d, v := range q.vec {
			q16[d] = quantize16(float32(v))
			qb[d] = uint8(int(quantize8(float32(v))) + 128)
			qnorm += v * v
		}
		qnorm = math.Sqrt(qnorm)

		oracle := topK(rows, func(r *vrow) float64 { return r.scoreOracle(q.vec, qnorm) }, K)
		approx16 := topK(rows, func(r *vrow) float64 { return r.scoreInt16(q16, qnorm) }, K)
		approx8 := topK(rows, func(r *vrow) float64 { return r.scoreInt8(qb, qnorm) }, K)

		r15 += recallAt(oracle, approx16, 1)
		r110 += recallAt(oracle, approx16, 10)
		r150 += recallAt(oracle, approx16, 50)
		r85 += recallAt(oracle, approx8, 5)
		r810 += recallAt(oracle, approx8, 10)
		r850 += recallAt(oracle, approx8, 50)
		ndcg16 += ndcgAt(oracle, approx16, 10)
		ndcg8 += ndcgAt(oracle, approx8, 10)
		jac16 += jaccard(oracle, approx16)
		jac8 += jaccard(oracle, approx8)
		// ranking recall@10 estrito (aproximação top-10 vs oracle top-10)
		rr16_10 += recallAt(oracle, approx16[:10], 10)
		rr8_10 += recallAt(oracle, approx8[:10], 10)
		// diversidade do top-10 oracle (conteúdos e documentos distintos)
		cset, dset := map[string]bool{}, map[string]bool{}
		for _, o := range oracle[:10] {
			rr := byID[o.id]
			cset[rr.content] = true
			dset[rr.documentID] = true
		}
		divContent += float64(len(cset))
		divDocs += float64(len(dset))

		// distintos@50 (exclui a loteria das duplicatas ~1.0)
		var d16, d8, cnt float64
		for _, o := range oracle {
			if o.score < 0.99 {
				cnt++
				for _, g := range approx16 {
					if g.id == o.id {
						d16++
						break
					}
				}
				for _, g := range approx8 {
					if g.id == o.id {
						d8++
						break
					}
				}
			}
		}
		if cnt > 0 {
			d1650 += d16 / cnt
			d850 += d8 / cnt
		}

		// quase-empates no oracle top-50
		for _, o := range oracle {
			if o.score >= 0.99 {
				oracleNearTie++
			}
		}

		// top-1 mudou? + gap (empate = gap < 0.02)
		if approx16[0].id != oracle[0].id {
			top1Changed16++
			if math.Abs(oracle[0].score-approx16[0].score) < 0.02 {
				top1Empate16++
			}
		}
		if approx8[0].id != oracle[0].id {
			top1Changed8++
			if math.Abs(oracle[0].score-approx8[0].score) < 0.02 {
				top1Empate8++
			}
		}

		// erro de score (mesmos ids)
		omap := map[string]float64{}
		for _, o := range oracle {
			omap[o.id] = o.score
		}
		for _, g := range approx16 {
			if s, ok := omap[g.id]; ok {
				d := math.Abs(g.score - s)
				if d > maxErr16 {
					maxErr16 = d
				}
				sumErr16 += d
			}
		}
		for _, g := range approx8 {
			if s, ok := omap[g.id]; ok {
				d := math.Abs(g.score - s)
				if d > maxErr8 {
					maxErr8 = d
				}
				sumErr8 += d
			}
		}
		n++
	}

	f := func(x float64) float64 { return x / float64(n) }
	recall1 := f(r15)
	t.Logf("── RECALL vs float32 ORACLE (self-match real, %d queries, K=%d) ──", n, K)
	t.Logf("int16 (default de produção):   set-recall@1=%.4f @5=%.4f @10=%.4f @50=%.4f  ranking-recall@10=%.4f  NDCG@10=%.4f  jaccard@50=%.4f  distintos@50=%.4f",
		recall1, f(r15), f(r110), f(r150), f(rr16_10), f(ndcg16), f(jac16), f(d1650))
	t.Logf("int8  (fast path antigo):      set-recall@1=%.4f @5=%.4f @10=%.4f @50=%.4f  ranking-recall@10=%.4f  NDCG@10=%.4f  jaccard@50=%.4f  distintos@50=%.4f",
		f(r85), f(r85), f(r810), f(r850), f(rr8_10), f(ndcg8), f(jac8), f(d850))
	t.Logf("diversidade top-10 (oracle): %.2f conteúdos distintos / %.2f documentos distintos (de 10)",
		f(divContent), f(divDocs))
	t.Logf("top-1 mudou: int16=%d (empates %d)   int8=%d (empates %d)",
		top1Changed16, top1Empate16, top1Changed8, top1Empate8)
	t.Logf("MAX abs score err: int16=%.6f (mean %.6f)   int8=%.6f (mean %.6f)",
		maxErr16, sumErr16/float64(n*K), maxErr8, sumErr8/float64(n*K))
	t.Logf("quase-empates (score>=0.99) no oracle top-50: %.1f%% do total",
		100*oracleNearTie/float64(n*K))
	return recall1, 0, f(ndcg16), f(ndcg8), f(jac16), f(jac8)
}

// ── Distribuição de scores (ranking achatado) ───────────────────────────────

func scoreDistribution(t testing.TB, rows, qrows []*vrow) {
	type stat struct {
		mu, sd, med, min, max, gap float64
		fra98, fra99                float64 // fração do top-10 dentro de 0.98/0.99 do top-1
	}
	var top10 []float64
	var gaps []float64
	var fra99Total int
	samples := 0
	aggregate := func(col []float64) (mu, sd, med, mn, mx float64, frac98, frac99 float64) {
		if len(col) == 0 {
			return
		}
		s := append([]float64(nil), col...)
		sort.Float64s(s)
		mn, mx = s[0], s[len(s)-1]
		var sum float64
		for _, v := range s {
			sum += v
		}
		mu = sum / float64(len(s))
		var vv float64
		for _, v := range s {
			vv += (v - mu) * (v - mu)
		}
		sd = math.Sqrt(vv / float64(len(s)))
		med = s[len(s)/2]
		frac99, frac98 = 0, 0
		return
	}

	for _, q := range qrows {
		var qnorm float64
		for _, v := range q.vec {
			qnorm += v * v
		}
		qnorm = math.Sqrt(qnorm)
		approx := topK(rows, func(r *vrow) float64 { return r.scoreOracle(q.vec, qnorm) }, 10)
		if len(approx) < 10 {
			continue
		}
		top := make([]float64, 10)
		for i, g := range approx {
			top[i] = g.score
		}
		gap := top[0] - top[9]
		gaps = append(gaps, gap)
		top10 = append(top10, top...)
		samples++
		// inversões? conta quantos dos ranks 2..10 estão a >=0.99 do top-1
		for _, s := range top[1:] {
			if s >= 0.99 {
				fra99Total++
			}
		}
	}
	mu, sd, med, mn, mx, _, _ := aggregate(top10)
	gmu, gsd, gmed, gmn, gmx, _, _ := aggregate(gaps)
	t.Logf("── DISTRIBUIÇÃO DOS top-10 SCORES (oracle float32, %d queries) ──", samples)
	t.Logf("score top-10: min=%.4f  max=%.4f  mean=%.4f  med=%.4f  sd=%.4f", mn, mx, mu, med, sd)
	t.Logf("GAP top1->top10: min=%.4f  max=%.4f  mean=%.4f  med=%.4f  sd=%.4f", gmn, gmx, gmu, gmed, gsd)
	t.Logf("ranks 2..10 do top-10 que estao >=0.99 do top-1: %d de %d (%.1f%%)",
		fra99Total, samples*9, 100*float64(fra99Total)/float64(samples*9))
}

// ── Real-text self-match via provider local (nomic-embed-text) ──────────────

func realTextSelfMatch(t testing.TB, rows, qrows []*vrow) {
	client := &http.Client{Timeout: 90 * time.Second}
	model := os.Getenv("COSCA_EMBED_MODEL")
	if model == "" {
		model = "nomic-embed-text"
	}
	var top1ScoreSum, hitsTop1, n float64
	var srcLow, srcDupCollision, srcMissing float64
	var srcScoreSum float64
	var elapsed time.Duration
	var errCount int
	var wg sync.WaitGroup
	sem := make(chan struct{}, 2) // limita chamadas locais concorrentes

	type tc struct {
		r    *vrow
		emb  []float64
		err  error
	}
	tasks := make(chan tc, len(qrows))
	for _, r := range qrows {
		if len(r.content) < 20 || len(r.content) > 2000 {
			continue
		}
		wg.Add(1)
		go func(r *vrow) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			emb, err := embedText(client, model, r.content)
			tasks <- tc{r: r, emb: emb, err: err}
		}(r)
	}
	wg.Wait()
	close(tasks)
	for tc := range tasks {
		if tc.err != nil {
			errCount++
			continue
		}
		if len(tc.emb) != 768 {
			errCount++
			continue
		}
		start := time.Now()
		oracle := topK(rows, func(x *vrow) float64 {
			var qnorm float64
			for _, v := range tc.emb {
				qnorm += v * v
			}
			qnorm = math.Sqrt(qnorm)
			return x.scoreOracle(tc.emb, qnorm)
		}, 5)
		elapsed += time.Since(start)
		n++
		top1 := oracle[0]
		top1ScoreSum += top1.score
		if top1.id == tc.r.id {
			hitsTop1++
		}
		// score do PRÓPRIO chunk no oracle top-5 (ajuda a distinguir causa da
		// falha de self-match: colisão de duplicata vs vetor de outro provider).
		var srcScore float64
		found := false
		for _, o := range oracle {
			if o.id == tc.r.id {
				srcScore = o.score
				found = true
				break
			}
		}
		srcScoreSum += srcScore
		if top1.id != tc.r.id {
			if found && srcScore >= 0.99 {
				srcDupCollision++ // colisão: outro vizinho quase-idêntico ganhou o desempate
			} else if !found || srcScore < 0.9 {
				srcLow++ // o próprio chunk tem score baixo → vetor de outro provider
			} else {
				srcMissing++
			}
		}
	}
	if n == 0 {
		t.Logf("real-text self-match: nenhuma query possível (provider falhou em %d).", errCount)
		return
	}
	t.Logf("── REAL-TEXT SELF-MATCH (nomic-embed-text via 11434, %d queries) ──", int(n))
	t.Logf("self-match top-1 (próprio chunk = top-1): %.1f%%  top1-score médio=%.4f  erros de embed=%d  tempo médio/busca=%.2fms",
		100*hitsTop1/n, top1ScoreSum/n, errCount, float64(elapsed)/float64(time.Duration(n))/float64(time.Millisecond))
	t.Logf("score do próprio chunk (médio)=%.4f", srcScoreSum/n)
	t.Logf("falhas top-1: colisão de duplicata (score próprio >=0.99)=%.1f%%  score próprio <0.9 (outro provider)=%.1f%%  outras=%.1f%%",
		100*srcDupCollision/n, 100*srcLow/n, 100*srcMissing/n)
}

func embedText(client *http.Client, model, text string) ([]float64, error) {
	body, err := json.Marshal(map[string]string{"model": model, "prompt": text})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", "http://localhost:11434/api/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, os.ErrInvalid
	}
	var out struct {
		Embedding []float64 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Embedding, nil
}

// helpers
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
