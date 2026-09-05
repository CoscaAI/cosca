// Package vector — COSCA VECTOR SEARCH AUTOPSY — experimentos A/B (testes novos).
//
// Hipótese central: a varredura full-scan aloca/duplica cada BLOB de vetor
// (columnBlob + bytes.Clone), tornando o scan ~5x mais caro que a matemática.
// Estes testes medem o caminho atual vs variantes de TESTE (nada de produção).
package vector

import (
	"database/sql"
	"fmt"
	"sort"
	"testing"
	"time"
)

// TestAutopsyScanRawBytesAB compara o scan atual (rows.Scan em []byte, que faz
// database/sql clonar o BLOB) contra um scan em sql.RawBytes (sem clone).
// Resultado: isola quanto do tempo do scan é ALOCAÇÃO/CÓPIA vs decode do SQLite.
func TestAutopsyScanRawBytesAB(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	const count = 100000
	store, rawConn := autopsyOpen(t, autopsyDim, count, autopsyCachePath(autopsyDim, count))
	// garante que está populado
	if n, _ := store.Count(); n < count {
		populateAutopsy(store, autopsyDim, count)
	}
	q := "SELECT id, vector FROM vectors WHERE 1=1"

	// A: path atual — []byte (database/sql clona o BLOB: columnBlob + bytes.Clone)
	timeA := measureScan(t, rawConn, q, false)
	// B: sql.RawBytes — sem clone, sem alocação por linha
	timeB := measureScan(t, rawConn, q, true)

	t.Logf("scan A ([]byte, clone):     %8.1f ms", float64(timeA)/1e6)
	t.Logf("scan B (sql.RawBytes):      %8.1f ms", float64(timeB)/1e6)
	t.Logf("razão A/B:                  %.2fx — cópia/alocação do BLOB = %.1f%% do scan",
		float64(timeA)/float64(timeB), 100*(1-float64(timeB)/float64(timeA)))
}

func measureScan(t *testing.T, db *sql.DB, query string, raw bool) time.Duration {
	t.Helper()
	// pré-aquecimento
	if err := warmScan(db, query, raw); err != nil {
		t.Fatalf("warm: %v", err)
	}
	const iters = 5
	var total time.Duration
	for i := 0; i < iters; i++ {
		t0 := time.Now()
		if err := warmScan(db, query, raw); err != nil {
			t.Fatalf("run: %v", err)
		}
		total += time.Since(t0)
	}
	return total / iters
}

func warmScan(db *sql.DB, query string, raw bool) error {
	rows, err := db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()
	var n int
	for rows.Next() {
		if raw {
			var id string
			var blob sql.RawBytes
			if err := rows.Scan(&id, &blob); err != nil {
				return err
			}
			_ = blob
		} else {
			var id string
			var blob []byte
			if err := rows.Scan(&id, &blob); err != nil {
				return err
			}
			_ = blob
		}
		n++
	}
	return rows.Err()
}

// TestAutopsyComputeVsScanIsolation: quanto do Search total é compute vs scan.
func TestAutopsyComputeVsScanIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	store := autopsyStore(t, autopsyDim, 100000)
	q := autopsyQuery(autopsyDim)
	var normA float64
	for _, v := range q {
		normA += v * v
	}

	// medições repetidas (warm)
	var scanT, computeT time.Duration
	const iters = 5
	for i := 0; i < iters; i++ {
		where, args := store.filterClause(nil)
		t0 := time.Now()
		rows, err := store.scanAll(where, args)
		if err != nil {
			t.Fatalf("scanAll: %v", err)
		}
		scanT += time.Since(t0)

		t0 = time.Now()
		scratch := make([]float32, autopsyDim)
		var acc float64
		for _, r := range rows {
			if len(r.blob) == autopsyDim*4 {
				acc += dotFromBytes(q, r.blob, scratch, normA)
			}
		}
		computeT += time.Since(t0)
	}
	scan, compute := float64(scanT)/float64(iters)/1e6, float64(computeT)/float64(iters)/1e6
	t.Logf("scan (serial, sqlite+copy): %8.1f ms  = %.1f%% do total", scan, 100*scan/(scan+compute))
	t.Logf("compute (dot, 1 goroutine): %8.1f ms  = %.1f%% do total", compute, 100*compute/(scan+compute))
	t.Logf("scan/compute = %.1fx", scan/compute)
}

// TestAutopsyCrossover: brute-force full scan vs caminho bounded (candidatos).
// Encontra o ponto de cruzamento em função do nº de candidatos.
func TestAutopsyCrossover(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	store := autopsyStore(t, autopsyDim, 100000)
	q := autopsyQuery(autopsyDim)

	// full scan
	where, args := store.filterClause(nil)
	t0 := time.Now()
	rows, err := store.scanAll(where, args)
	if err != nil {
		t.Fatalf("scanAll: %v", err)
	}
	fullScan := time.Since(t0)

	// custo por linha do scan
	perRow := float64(fullScan) / float64(len(rows))

	// bounded path com M candidatos + pool 250
	for _, m := range []int{5, 20, 50, 100, 500, 1000, 5000} {
		cands := make([]string, m)
		for i := range cands {
			cands[i] = fmt.Sprintf("auto-%d", i)
		}
		t0 := time.Now()
		_, err := store.SearchWithCandidates(q, 8, cands, 250, nil)
		if err != nil {
			t.Fatalf("candidate search: %v", err)
		}
		d := time.Since(t0)
		scored := m + 250
		t.Logf("bounded M=%-5d (score %4d rows): %8.1f ms   (full scan = %8.1f ms)",
			m, scored, float64(d)/1e6, float64(fullScan)/1e6)
	}
	t.Logf("full scan per row = %.1f µs (%d rows)", perRow/1e3, len(rows))
}

// TestAutopsyPercentiles — p50/p95/p99 do Search completo (warm, 100K).
func TestAutopsyPercentiles(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	store := autopsyStore(t, autopsyDim, 100000)
	q := autopsyQuery(autopsyDim)
	const n = 60
	samples := make([]time.Duration, n)
	for i := 0; i < n; i++ {
		t0 := time.Now()
		if _, err := store.Search(q, 10); err != nil {
			t.Fatalf("search: %v", err)
		}
		samples[i] = time.Since(t0)
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	pct := func(p float64) time.Duration { return samples[int(float64(n-1)*p)] }
	t.Logf("Search completo (100K, dim768, warm): mean=%8.1f ms  p50=%8.1f ms  p95=%8.1f ms  p99=%8.1f ms  min=%8.1f ms",
		float64(sumDur(samples))/float64(n)/1e6, float64(pct(0.50))/1e6, float64(pct(0.95))/1e6, float64(pct(0.99))/1e6, float64(samples[0])/1e6)
}

func sumDur(d []time.Duration) time.Duration {
	var s time.Duration
	for _, v := range d {
		s += v
	}
	return s
}
