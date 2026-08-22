package vector

import (
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// newBenchVectorStore creates an in-memory SQLite-backed vector store.
func newBenchVectorStore(b *testing.B) (*SQLiteVec, *sql.DB) {
	b.Helper()

	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "vec_bench.db")
	_ = os.MkdirAll(tmpDir, 0755)

	rawConn, err := sql.Open("sqlite", dbPath+"?mode=rwc&_journal_mode=WAL&_synchronous=NORMAL")
	if err != nil {
		b.Fatalf("failed to open sqlite: %v", err)
	}
	rawConn.SetMaxOpenConns(1)

	cfg := SQLiteVecConfig{
		DB:        rawConn,
		Dimension: 128,
		TableName: "vectors",
	}

	store, err := NewSQLiteVec(cfg)
	if err != nil {
		b.Fatalf("failed to create vector store: %v", err)
	}

	b.Cleanup(func() {
		_ = rawConn.Close()
	})

	return store, rawConn
}

// generateRandomVector creates a random vector of given dimensionality.
func generateRandomVector(dim int) []float64 {
	vec := make([]float64, dim)
	for i := 0; i < dim; i++ {
		vec[i] = rand.Float64()*2 - 1 // range [-1, 1]
	}
	return vec
}

// populateVectorStore inserts random vectors for benchmarking.
func populateVectorStore(b *testing.B, store *SQLiteVec, count int) {
	b.Helper()
	batchSize := 100
	for batch := 0; batch < count; batch += batchSize {
		n := batchSize
		if batch+n > count {
			n = count - batch
		}
		records := make([]VectorRecord, n)
		for i := 0; i < n; i++ {
			idx := batch + i
			records[i] = VectorRecord{
				ID:         fmt.Sprintf("vec-%d", idx),
				Vector:     generateRandomVector(128),
				DocumentID: fmt.Sprintf("doc-%d", idx%10),
				ChunkID:    fmt.Sprintf("chunk-%d", idx%50),
				Content:    fmt.Sprintf("Content for vector %d with performance optimization keywords.", idx),
				Metadata:   map[string]string{"index": fmt.Sprintf("%d", idx)},
			}
		}
		if err := store.Store(128, records); err != nil {
			b.Fatalf("failed to store batch %d: %v", batch, err)
		}
	}
}

// ── Vector Search Benchmarks (Brute-Force Cosine Similarity) ─────────────────

func BenchmarkVectorSearch_100Vectors(b *testing.B) {
	store, _ := newBenchVectorStore(b)
	populateVectorStore(b, store, 100)
	query := generateRandomVector(128)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := store.Search(query, 10)
		if err != nil {
			b.Fatalf("search failed: %v", err)
		}
		_ = results
	}
}

func BenchmarkVectorSearch_500Vectors(b *testing.B) {
	store, _ := newBenchVectorStore(b)
	populateVectorStore(b, store, 500)
	query := generateRandomVector(128)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := store.Search(query, 10)
		if err != nil {
			b.Fatalf("search failed: %v", err)
		}
		_ = results
	}
}

func BenchmarkVectorSearch_1000Vectors(b *testing.B) {
	store, _ := newBenchVectorStore(b)
	populateVectorStore(b, store, 1000)
	query := generateRandomVector(128)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := store.Search(query, 10)
		if err != nil {
			b.Fatalf("search failed: %v", err)
		}
		_ = results
	}
}

func BenchmarkVectorSearch_5000Vectors(b *testing.B) {
	store, _ := newBenchVectorStore(b)
	populateVectorStore(b, store, 5000)
	query := generateRandomVector(128)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := store.Search(query, 10)
		if err != nil {
			b.Fatalf("search failed: %v", err)
		}
		_ = results
	}
}

func BenchmarkVectorSearch_10000Vectors(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping large vector benchmark in short mode")
	}
	store, _ := newBenchVectorStore(b)
	populateVectorStore(b, store, 10000)
	query := generateRandomVector(128)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := store.Search(query, 10)
		if err != nil {
			b.Fatalf("search failed: %v", err)
		}
		_ = results
	}
}

// ── Vector Index Operations ─────────────────────────────────────────────────

func BenchmarkVectorStore_Insert(b *testing.B) {
	store, _ := newBenchVectorStore(b)
	vec := generateRandomVector(128)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		records := []VectorRecord{
			{
				ID:      fmt.Sprintf("bench-%d", i),
				Vector:  vec,
				Content: "benchmark content",
			},
		}
		if err := store.Store(128, records); err != nil {
			b.Fatalf("store failed: %v", err)
		}
	}
}

func BenchmarkVectorStore_BatchInsert(b *testing.B) {
	store, _ := newBenchVectorStore(b)
	vecs := make([]float64, 128)
	for i := range vecs {
		vecs[i] = rand.Float64()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		records := make([]VectorRecord, 50)
		for j := 0; j < 50; j++ {
			records[j] = VectorRecord{
				ID:      fmt.Sprintf("batch-%d-%d", i, j),
				Vector:  generateRandomVector(128),
				Content: fmt.Sprintf("batch content %d", j),
			}
		}
		if err := store.Store(128, records); err != nil {
			b.Fatalf("batch store failed: %v", err)
		}
	}
}

// ── Binary Encoding Benchmarks ──────────────────────────────────────────────

func BenchmarkFloat32SliceToBytes_128d(b *testing.B) {
	vec := generateRandomVector(128)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = float32SliceToBytes(vec)
	}
}

func BenchmarkBytesToFloat32Slice_128d(b *testing.B) {
	vec := generateRandomVector(128)
	raw := float32SliceToBytes(vec)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bytesToFloat32Slice(raw)
	}
}
