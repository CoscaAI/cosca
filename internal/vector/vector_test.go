package vector

import (
	"database/sql"
	"math"
	"testing"

	_ "modernc.org/sqlite"
)

// ── VectorRecord ────────────────────────────────────────────────────────────

func TestVectorRecord(t *testing.T) {
	vr := VectorRecord{
		ID:     "vec-1",
		Vector: []float64{0.1, 0.2, 0.3},
	}
	if vr.ID != "vec-1" {
		t.Errorf("ID = %q", vr.ID)
	}
	if len(vr.Vector) != 3 {
		t.Errorf("Vector len = %d", len(vr.Vector))
	}
}

func TestVectorRecordWithMetadata(t *testing.T) {
	vr := VectorRecord{
		ID:         "vec-2",
		Vector:     []float64{1.0, 0.0},
		DocumentID: "doc-1",
		ChunkID:    "chunk-1",
		Content:    "test content",
		Metadata:   map[string]string{"key": "val"},
	}
	if vr.DocumentID != "doc-1" {
		t.Errorf("DocumentID = %q", vr.DocumentID)
	}
	if vr.Metadata["key"] != "val" {
		t.Errorf("Metadata = %v", vr.Metadata)
	}
}

func TestVectorRecord_EmptyVector(t *testing.T) {
	vr := VectorRecord{
		ID:     "empty",
		Vector: []float64{},
	}
	if len(vr.Vector) != 0 {
		t.Errorf("Vector len = %d, want 0", len(vr.Vector))
	}
}

// ── SearchResult ────────────────────────────────────────────────────────────

func TestSearchResult(t *testing.T) {
	sr := SearchResult{
		ID:     "result-1",
		Score:  0.95,
		Vector: []float64{0.5, 0.5},
	}
	if sr.ID != "result-1" {
		t.Errorf("ID = %q", sr.ID)
	}
	if sr.Score != 0.95 {
		t.Errorf("Score = %f", sr.Score)
	}
}

// ── VectorStats ─────────────────────────────────────────────────────────────

func TestVectorStats(t *testing.T) {
	vs := VectorStats{
		TotalVectors: 100,
		Dimensions:   128,
		IndexType:    "flat",
		MemoryUsage:  1024,
		DeletedCount: 5,
	}
	if vs.TotalVectors != 100 {
		t.Errorf("TotalVectors = %d", vs.TotalVectors)
	}
	if vs.Dimensions != 128 {
		t.Errorf("Dimensions = %d", vs.Dimensions)
	}
}

// ── DefaultSearchParams ─────────────────────────────────────────────────────

func TestDefaultSearchParams(t *testing.T) {
	sp := DefaultSearchParams()
	if sp.Limit != 10 {
		t.Errorf("Limit = %d, want 10", sp.Limit)
	}
	if sp.Offset != 0 {
		t.Errorf("Offset = %d", sp.Offset)
	}
	if sp.IncludeVectors {
		t.Error("IncludeVectors should be false")
	}
}

// ── SearchParams ────────────────────────────────────────────────────────────

func TestSearchParamsDefaults(t *testing.T) {
	sp := SearchParams{}
	if sp.Limit != 0 {
		t.Errorf("Limit = %d", sp.Limit)
	}
}

func TestSearchParamsCustom(t *testing.T) {
	sp := SearchParams{
		Limit:          25,
		Offset:         5,
		IncludeVectors: true,
		MinScore:       0.5,
	}
	if sp.Limit != 25 {
		t.Errorf("Limit = %d", sp.Limit)
	}
	if !sp.IncludeVectors {
		t.Error("IncludeVectors should be true")
	}
}

func TestSearchParams_Filter(t *testing.T) {
	sp := SearchParams{
		Filter: map[string]string{"type": "agent"},
	}
	if sp.Filter["type"] != "agent" {
		t.Errorf("Filter = %v", sp.Filter)
	}
}

func TestSearchParams_FullConfig(t *testing.T) {
	sp := SearchParams{
		Limit:          50,
		Offset:         10,
		Filter:         map[string]string{"status": "active"},
		DocumentID:     "doc-123",
		EntityType:     "agent",
		IncludeVectors: true,
		MinScore:       0.7,
	}
	if sp.DocumentID != "doc-123" {
		t.Errorf("DocumentID = %q", sp.DocumentID)
	}
	if sp.EntityType != "agent" {
		t.Errorf("EntityType = %q", sp.EntityType)
	}
	if sp.MinScore != 0.7 {
		t.Errorf("MinScore = %f", sp.MinScore)
	}
}

// ── Store Interface (mock) ──────────────────────────────────────────────────

func TestStoreInterface(t *testing.T) {
	var store Store = &MockVectorStore{}
	_ = store
}

// ── CosineSimilarity ────────────────────────────────────────────────────────

func TestCosineSimilarity(t *testing.T) {
	a := []float64{1.0, 0.0, 0.0}
	b := []float64{1.0, 0.0, 0.0}
	sim, err := CosineSimilarity(a, b)
	if err != nil {
		t.Fatalf("CosineSimilarity error: %v", err)
	}
	if math.Abs(sim-1.0) > 1e-10 {
		t.Errorf("similarity = %f, want 1.0", sim)
	}
}

func TestCosineSimilarityOrthogonal(t *testing.T) {
	a := []float64{1.0, 0.0}
	b := []float64{0.0, 1.0}
	sim, _ := CosineSimilarity(a, b)
	if math.Abs(sim) > 1e-10 {
		t.Errorf("similarity = %f, want 0.0", sim)
	}
}

func TestCosineSimilarityMismatchedDims(t *testing.T) {
	a := []float64{1.0, 0.0}
	b := []float64{1.0}
	sim, _ := CosineSimilarity(a, b)
	if sim != 0 {
		t.Errorf("similarity = %f, want 0 for mismatched dims", sim)
	}
}

func TestCosineSimilarityZeroNorm(t *testing.T) {
	a := []float64{0.0, 0.0}
	b := []float64{1.0, 0.0}
	sim, _ := CosineSimilarity(a, b)
	if sim != 0 {
		t.Errorf("similarity = %f, want 0 for zero norm", sim)
	}
}

func TestCosineSimilarity_BothZero(t *testing.T) {
	a := []float64{0.0, 0.0, 0.0}
	b := []float64{0.0, 0.0, 0.0}
	sim, err := CosineSimilarity(a, b)
	if err != nil {
		t.Fatalf("CosineSimilarity error: %v", err)
	}
	if sim != 0 {
		t.Errorf("similarity = %f, want 0 for zero vectors", sim)
	}
}

func TestCosineSimilarity_NegativeValue(t *testing.T) {
	a := []float64{1.0, 0.0}
	b := []float64{-1.0, 0.0}
	sim, err := CosineSimilarity(a, b)
	if err != nil {
		t.Fatalf("CosineSimilarity error: %v", err)
	}
	if math.Abs(sim-(-1.0)) > 1e-10 {
		t.Errorf("similarity = %f, want -1.0", sim)
	}
}

func TestCosineSimilarity_LargeVectors(t *testing.T) {
	a := make([]float64, 100)
	b := make([]float64, 100)
	for i := range a {
		a[i] = 1.0
		b[i] = 1.0
	}
	sim, err := CosineSimilarity(a, b)
	if err != nil {
		t.Fatalf("CosineSimilarity error: %v", err)
	}
	if math.Abs(sim-1.0) > 1e-10 {
		t.Errorf("similarity = %f, want 1.0", sim)
	}
}

func TestCosineSimilarity_EmptyVectors(t *testing.T) {
	a := []float64{}
	b := []float64{}
	sim, _ := CosineSimilarity(a, b)
	if sim != 0 {
		t.Errorf("similarity = %f, want 0", sim)
	}
}

// ── Sqrt ────────────────────────────────────────────────────────────────────

func TestSqrt(t *testing.T) {
	tests := []struct {
		input float64
		want  float64
	}{
		{0, 0},
		{1, 1},
		{4, 2},
		{9, 3},
		{16, 4},
		{2, 1.4142135623730951},
		{100, 10},
		{0.25, 0.5},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := sqrt(tt.input)
			if math.Abs(got-tt.want) > 1e-10 {
				t.Errorf("sqrt(%f) = %f, want %f", tt.input, got, tt.want)
			}
		})
	}
}

// ── NormalizeVector ─────────────────────────────────────────────────────────

func TestNormalizeVector(t *testing.T) {
	v := []float64{3.0, 4.0}
	nv := NormalizeVector(v)
	var norm float64
	for _, val := range nv {
		norm += val * val
	}
	if math.Abs(norm-1.0) > 1e-10 {
		t.Errorf("norm = %f, want 1.0", norm)
	}
}

func TestNormalizeVectorZero(t *testing.T) {
	v := []float64{0.0, 0.0}
	nv := NormalizeVector(v)
	if len(nv) != 2 {
		t.Errorf("len = %d, want 2", len(nv))
	}
	// Should return the same zero vector
	if nv[0] != 0.0 || nv[1] != 0.0 {
		t.Errorf("nv = %v, want zero vector", nv)
	}
}

func TestNormalizeVectorIdentity(t *testing.T) {
	v := []float64{1.0, 0.0}
	nv := NormalizeVector(v)
	if math.Abs(nv[0]-1.0) > 1e-10 {
		t.Errorf("nv[0] = %f", nv[0])
	}
}

func TestNormalizeVector_Empty(t *testing.T) {
	v := []float64{}
	nv := NormalizeVector(v)
	if len(nv) != 0 {
		t.Errorf("len = %d, want 0", len(nv))
	}
}

func TestNormalizeVector_AlreadyNormalized(t *testing.T) {
	v := []float64{0.6, 0.8}
	nv := NormalizeVector(v)
	var norm float64
	for _, val := range nv {
		norm += val * val
	}
	if math.Abs(norm-1.0) > 1e-10 {
		t.Errorf("norm = %f, want 1.0", norm)
	}
}

// ── MockVectorStore ─────────────────────────────────────────────────────────

func TestMockVectorStore_Default(t *testing.T) {

	mock := &MockVectorStore{}

	err := mock.Store(128, nil)
	if err != nil {
		t.Errorf("Store error: %v", err)
	}

	results, err := mock.Search(nil, 10)
	if err != nil {
		t.Errorf("Search error: %v", err)
	}
	if results != nil {
		t.Error("expected nil results")
	}

	dim := mock.Dimension()
	if dim != 128 {
		t.Errorf("Dimension = %d, want 128", dim)
	}

	count, err := mock.Count()
	if err != nil {
		t.Errorf("Count error: %v", err)
	}
	if count != 0 {
		t.Errorf("Count = %d, want 0", count)
	}
}

func TestMockVectorStore_CustomFuncs(t *testing.T) {

	mock := &MockVectorStore{
		SearchFunc: func(query []float64, limit int) ([]SearchResult, error) {
			return []SearchResult{{ID: "r1", Score: 0.9}}, nil
		},
		CountFunc: func() (int, error) {
			return 42, nil
		},
		StatsFunc: func() (VectorStats, error) {
			return VectorStats{TotalVectors: 100, Dimensions: 256}, nil
		},
	}

	results, err := mock.Search([]float64{1, 2}, 5)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].ID != "r1" {
		t.Errorf("ID = %q", results[0].ID)
	}

	count, err := mock.Count()
	if err != nil {
		t.Fatalf("Count error: %v", err)
	}
	if count != 42 {
		t.Errorf("Count = %d, want 42", count)
	}

	stats, err := mock.Stats()
	if err != nil {
		t.Fatalf("Stats error: %v", err)
	}
	if stats.TotalVectors != 100 {
		t.Errorf("TotalVectors = %d, want 100", stats.TotalVectors)
	}
}

// ── Binary encoding ─────────────────────────────────────────────────────────

func TestFloat32SliceToBytes(t *testing.T) {

	tests := []struct {
		name   string
		vector []float64
	}{
		{"normal", []float64{1.0, 2.0, 3.0}},
		{"empty", []float64{}},
		{"single", []float64{42.0}},
		{"negative", []float64{-1.0, -0.5, 0.0}},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			data := float32SliceToBytes(tc.vector)
			// Each value is stored as a float32 (4 bytes)
			if len(data) != len(tc.vector)*4 {
				t.Errorf("byte len = %d, want %d", len(data), len(tc.vector)*4)
			}
		})
	}
}

func TestBytesToFloat32Slice(t *testing.T) {

	tests := []struct {
		name string
		data []float64
	}{
		{"normal", []float64{1.0, 2.0, 3.0}},
		{"single", []float64{42.5}},
		{"negative", []float64{-1.0, -0.5}},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			encoded := float32SliceToBytes(tc.data)
			decoded := bytesToFloat32Slice(encoded)
			if len(decoded) != len(tc.data) {
				t.Fatalf("decoded len = %d, want %d", len(decoded), len(tc.data))
			}
			for i := range tc.data {
				if math.Abs(decoded[i]-tc.data[i]) > 1e-6 {
					t.Errorf("decoded[%d] = %f, want %f", i, decoded[i], tc.data[i])
				}
			}
		})
	}
}

func TestBytesToFloat32Slice_Empty(t *testing.T) {

	result := bytesToFloat32Slice([]byte{})
	if result != nil {
		t.Errorf("got %v, want nil", result)
	}

	result = bytesToFloat32Slice(nil)
	if result != nil {
		t.Errorf("got %v, want nil", result)
	}
}

func TestBytesToFloat32Slice_InvalidLength(t *testing.T) {

	// A byte slice whose length is not a multiple of 4
	data := []byte{1, 2, 3}
	result := bytesToFloat32Slice(data)
	// Integer division: 3/4 = 0, so returns nil
	if len(result) != 0 {
		t.Errorf("got %d elements, want 0", len(result))
	}
}

func TestRoundTrip_Encoding(t *testing.T) {

	original := []float64{0.1, -0.2, 3.14, 2.718, 1.618}
	encoded := float32SliceToBytes(original)
	decoded := bytesToFloat32Slice(encoded)

	if len(decoded) != len(original) {
		t.Fatalf("len mismatch: %d vs %d", len(decoded), len(original))
	}
	for i := range original {
		if math.Abs(decoded[i]-original[i]) > 1e-6 {
			t.Errorf("value[%d] = %f, want %f", i, decoded[i], original[i])
		}
	}
}

// ── escapeJSONPath ──────────────────────────────────────────────────────────

func TestEscapeJSONPath(t *testing.T) {

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no escape needed", "simple", "simple"},
		{"with single quote", "it's", "it''s"},
		{"empty", "", ""},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := escapeJSONPath(tc.input)
			if got != tc.want {
				t.Errorf("escapeJSONPath(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ── SQLiteVec Tests ─────────────────────────────────────────────────────────

// newTestSQLiteVec creates a SQLiteVec backed by an in-memory database.
func newTestSQLiteVec(t *testing.T, dimension int) *SQLiteVec {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store, err := NewSQLiteVec(SQLiteVecConfig{
		DB:         db,
		Dimension:  dimension,
		TableName:  "vectors",
		Normalized: false,
	})
	if err != nil {
		t.Fatalf("NewSQLiteVec: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	return store
}

func TestNewSQLiteVec_NilDB(t *testing.T) {

	store, err := NewSQLiteVec(SQLiteVecConfig{
		DB:        nil,
		Dimension: 128,
	})
	if err == nil {
		t.Fatal("expected error for nil DB")
	}
	if store != nil {
		t.Error("store should be nil on error")
	}
}

func TestNewSQLiteVec_ZeroDimension(t *testing.T) {

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	store, err := NewSQLiteVec(SQLiteVecConfig{
		DB:        db,
		Dimension: 0,
	})
	if err != nil {
		t.Fatalf("NewSQLiteVec: %v", err)
	}
	if store.Dimension() != 128 {
		t.Errorf("Dimension = %d, want 128 (default)", store.Dimension())
	}
}

func TestNewSQLiteVec_DefaultTableName(t *testing.T) {

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	store, err := NewSQLiteVec(SQLiteVecConfig{
		DB:        db,
		Dimension: 64,
	})
	if err != nil {
		t.Fatalf("NewSQLiteVec: %v", err)
	}
	if store.tableName != "vectors" {
		t.Errorf("tableName = %q, want vectors", store.tableName)
	}
}

func TestSQLiteVec_Store(t *testing.T) {

	store := newTestSQLiteVec(t, 4)

	vectors := []VectorRecord{
		{
			ID:         "v1",
			Vector:     []float64{1.0, 0.0, 0.0, 0.0},
			DocumentID: "doc1",
			Content:    "first vector",
			Metadata:   map[string]string{"type": "test"},
		},
		{
			ID:         "v2",
			Vector:     []float64{0.0, 1.0, 0.0, 0.0},
			DocumentID: "doc2",
			Content:    "second vector",
		},
	}

	err := store.Store(4, vectors)
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	count, err := store.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 2 {
		t.Errorf("Count = %d, want 2", count)
	}
}

func TestSQLiteVec_StoreEmpty(t *testing.T) {

	store := newTestSQLiteVec(t, 4)

	err := store.Store(4, nil)
	if err != nil {
		t.Fatalf("Store empty: %v", err)
	}

	err = store.Store(4, []VectorRecord{})
	if err != nil {
		t.Fatalf("Store empty slice: %v", err)
	}
}

func TestSQLiteVec_StoreWithoutID(t *testing.T) {

	store := newTestSQLiteVec(t, 2)

	vectors := []VectorRecord{
		{
			Vector:  []float64{1.0, 2.0},
			Content: "auto id",
		},
	}

	err := store.Store(2, vectors)
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	count, err := store.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 1 {
		t.Errorf("Count = %d, want 1", count)
	}
}

func TestSQLiteVec_StoreNormalized(t *testing.T) {

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	store, err := NewSQLiteVec(SQLiteVecConfig{
		DB:          db,
		Dimension:   2,
		Normalized:  true,
		DisableInt8: true, DisableInt16: true, // contrato exato (valida a normalização do Store)
	})
	if err != nil {
		t.Fatalf("NewSQLiteVec: %v", err)
	}
	defer func() { _ = store.Close() }()

	vectors := []VectorRecord{
		{
			ID:     "v1",
			Vector: []float64{3.0, 4.0}, // Should be normalized to [0.6, 0.8]
		},
	}

	err = store.Store(2, vectors)
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	results, err := store.Search([]float64{0.6, 0.8}, 1)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if math.Abs(results[0].Score-1.0) > 1e-6 {
		t.Errorf("Score = %f, want ~1.0", results[0].Score)
	}
}

func TestSQLiteVec_Search(t *testing.T) {

	store := newTestSQLiteVec(t, 4)

	vectors := []VectorRecord{
		{ID: "v1", Vector: []float64{1.0, 0.0, 0.0, 0.0}},
		{ID: "v2", Vector: []float64{0.0, 1.0, 0.0, 0.0}},
		{ID: "v3", Vector: []float64{1.0, 0.0, 0.0, 0.5}},
	}

	err := store.Store(4, vectors)
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	// Search with query similar to v1
	results, err := store.Search([]float64{1.0, 0.0, 0.0, 0.0}, 2)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].ID != "v1" {
		t.Errorf("top result = %q, want v1", results[0].ID)
	}
}

func TestSQLiteVec_SearchDefaultLimit(t *testing.T) {

	store := newTestSQLiteVec(t, 2)

	// Store multiple vectors
	for i := 0; i < 20; i++ {
		err := store.Store(2, []VectorRecord{
			{ID: "vec", Vector: []float64{float64(i), float64(i)}},
		})
		if err != nil {
			t.Fatalf("Store: %v", err)
		}
	}

	results, err := store.Search([]float64{1.0, 1.0}, 0) // 0 should default to 10
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) > 10 {
		t.Errorf("got %d results, want <= 10", len(results))
	}
}

func TestSQLiteVec_SearchWithFilter(t *testing.T) {

	store := newTestSQLiteVec(t, 2)

	vectors := []VectorRecord{
		{ID: "v1", Vector: []float64{1.0, 0.0}, Metadata: map[string]string{"type": "agent"}},
		{ID: "v2", Vector: []float64{0.0, 1.0}, Metadata: map[string]string{"type": "skill"}},
	}

	err := store.Store(2, vectors)
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	// Filter for agents only
	results, err := store.SearchWithFilter([]float64{1.0, 0.0}, 10, map[string]string{"type": "agent"})
	if err != nil {
		t.Fatalf("SearchWithFilter: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].ID != "v1" {
		t.Errorf("ID = %q, want v1", results[0].ID)
	}
}

func TestSQLiteVec_SearchDimensionMismatch(t *testing.T) {

	store := newTestSQLiteVec(t, 4)

	err := store.Store(4, []VectorRecord{
		{ID: "v1", Vector: []float64{1.0, 0.0, 0.0, 0.0}},
	})
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	// Query with wrong dimension - no results should match
	results, err := store.Search([]float64{1.0, 0.0}, 10) // 2D query, 4D stored
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0 (dimension mismatch)", len(results))
	}
}

func TestSQLiteVec_Delete(t *testing.T) {

	store := newTestSQLiteVec(t, 2)

	err := store.Store(2, []VectorRecord{
		{ID: "v1", Vector: []float64{1.0, 0.0}},
		{ID: "v2", Vector: []float64{0.0, 1.0}},
		{ID: "v3", Vector: []float64{1.0, 1.0}},
	})
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	err = store.Delete([]string{"v1", "v2"})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	count, err := store.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 1 {
		t.Errorf("Count = %d, want 1", count)
	}
}

func TestSQLiteVec_DeleteEmpty(t *testing.T) {

	store := newTestSQLiteVec(t, 2)

	err := store.Delete(nil)
	if err != nil {
		t.Fatalf("Delete nil: %v", err)
	}

	err = store.Delete([]string{})
	if err != nil {
		t.Fatalf("Delete empty: %v", err)
	}
}

func TestSQLiteVec_DeleteByDocument(t *testing.T) {

	store := newTestSQLiteVec(t, 2)

	err := store.Store(2, []VectorRecord{
		{ID: "v1", Vector: []float64{1.0, 0.0}, DocumentID: "doc-keep"},
		{ID: "v2", Vector: []float64{0.0, 1.0}, DocumentID: "doc-delete"},
		{ID: "v3", Vector: []float64{1.0, 1.0}, DocumentID: "doc-delete"},
	})
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	err = store.DeleteByDocument("doc-delete")
	if err != nil {
		t.Fatalf("DeleteByDocument: %v", err)
	}

	count, err := store.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 1 {
		t.Errorf("Count = %d, want 1", count)
	}
}

func TestSQLiteVec_DeleteByEntity(t *testing.T) {

	store := newTestSQLiteVec(t, 2)

	err := store.Store(2, []VectorRecord{
		{ID: "v1", Vector: []float64{1.0, 0.0}, EntityID: "e1"},
		{ID: "v2", Vector: []float64{0.0, 1.0}, EntityID: "e1"},
		{ID: "v3", Vector: []float64{1.0, 1.0}, EntityID: "e2"},
	})
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	err = store.DeleteByEntity("e1")
	if err != nil {
		t.Fatalf("DeleteByEntity: %v", err)
	}

	count, err := store.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 1 {
		t.Errorf("Count = %d, want 1", count)
	}
}

func TestSQLiteVec_Stats(t *testing.T) {

	store := newTestSQLiteVec(t, 4)

	err := store.Store(4, []VectorRecord{
		{ID: "v1", Vector: []float64{1.0, 0.0, 0.0, 0.0}},
		{ID: "v2", Vector: []float64{0.0, 1.0, 0.0, 0.0}},
	})
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	stats, err := store.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.TotalVectors != 2 {
		t.Errorf("TotalVectors = %d, want 2", stats.TotalVectors)
	}
	if stats.Dimensions != 4 {
		t.Errorf("Dimensions = %d, want 4", stats.Dimensions)
	}
	if stats.IndexType != "flat_bruteforce" {
		t.Errorf("IndexType = %q, want flat_bruteforce", stats.IndexType)
	}
}

func TestSQLiteVec_Dimension(t *testing.T) {

	store := newTestSQLiteVec(t, 128)
	if store.Dimension() != 128 {
		t.Errorf("Dimension = %d, want 128", store.Dimension())
	}
}

func TestSQLiteVec_Rebuild(t *testing.T) {

	store := newTestSQLiteVec(t, 2)

	err := store.Store(2, []VectorRecord{
		{ID: "v1", Vector: []float64{1.0, 0.0}},
	})
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	err = store.Rebuild()
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	// After rebuild, table is empty
	count, err := store.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 0 {
		t.Errorf("Count = %d, want 0 after rebuild", count)
	}
}

func TestSQLiteVec_SearchResultFields(t *testing.T) {

	store := newTestSQLiteVec(t, 2)

	err := store.Store(2, []VectorRecord{
		{
			ID:         "v1",
			Vector:     []float64{1.0, 0.0},
			DocumentID: "doc-1",
			ChunkID:    "chunk-1",
			EntityID:   "entity-1",
			Content:    "test content here",
			Metadata:   map[string]string{"key": "value"},
		},
	})
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	results, err := store.Search([]float64{1.0, 0.0}, 1)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}

	r := results[0]
	if r.DocumentID != "doc-1" {
		t.Errorf("DocumentID = %q", r.DocumentID)
	}
	if r.ChunkID != "chunk-1" {
		t.Errorf("ChunkID = %q", r.ChunkID)
	}
	if r.EntityID != "entity-1" {
		t.Errorf("EntityID = %q", r.EntityID)
	}
	if r.Content != "test content here" {
		t.Errorf("Content = %q", r.Content)
	}
	if r.Metadata["key"] != "value" {
		t.Errorf("Metadata = %v", r.Metadata)
	}
}

func TestSQLiteVec_StoreWithNilMetadata(t *testing.T) {

	store := newTestSQLiteVec(t, 2)

	err := store.Store(2, []VectorRecord{
		{ID: "v1", Vector: []float64{1.0, 0.0}, Metadata: nil},
	})
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	results, err := store.Search([]float64{1.0, 0.0}, 1)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
}

func TestSQLiteVec_UpdateDimension(t *testing.T) {

	store := newTestSQLiteVec(t, 2)

	// A store has one fixed dimension and must reject mixed dimensions.
	err := store.Store(8, []VectorRecord{
		{ID: "v1", Vector: []float64{1, 2, 3, 4, 5, 6, 7, 8}},
	})
	if err == nil {
		t.Fatal("expected dimension mismatch error")
	}
	if store.Dimension() != 2 {
		t.Errorf("Dimension = %d, want 2", store.Dimension())
	}
}

func TestSQLiteVec_SearchWithCandidates(t *testing.T) {
	store := newTestSQLiteVec(t, 2)

	err := store.Store(2, []VectorRecord{
		{ID: "c1", Vector: []float64{1.0, 0.0}, Metadata: map[string]string{"type": "agent"}},
		{ID: "c2", Vector: []float64{0.0, 1.0}, Metadata: map[string]string{"type": "skill"}},
		{ID: "c3", Vector: []float64{1.0, 1.0}, Metadata: map[string]string{"type": "agent"}},
		{ID: "c4", Vector: []float64{-1.0, 1.0}},
		{ID: "c5", Vector: []float64{0.5, 0.5}},
	})
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	// Candidates only: results restricted to c2/c4, sorted by similarity desc.
	results, err := store.SearchWithCandidates([]float64{1.0, 0.0}, 10, []string{"c2", "c4"}, 0, nil)
	if err != nil {
		t.Fatalf("SearchWithCandidates: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2 (candidates only)", len(results))
	}
	for _, r := range results {
		if r.ID != "c2" && r.ID != "c4" {
			t.Errorf("result %q outside candidate set", r.ID)
		}
	}
	if results[0].ID != "c2" || results[1].ID != "c4" {
		t.Errorf("wrong candidate ordering: %q, %q", results[0].ID, results[1].ID)
	}

	// Limit truncation applies to the candidate set.
	results, err = store.SearchWithCandidates([]float64{1.0, 0.0}, 1, []string{"c2", "c4"}, 0, nil)
	if err != nil {
		t.Fatalf("SearchWithCandidates: %v", err)
	}
	if len(results) != 1 || results[0].ID != "c2" {
		t.Errorf("limit truncation failed: %+v", results)
	}

	// Filter applies to candidates.
	results, err = store.SearchWithCandidates([]float64{1.0, 0.0}, 10, []string{"c1", "c2"}, 0, map[string]string{"type": "skill"})
	if err != nil {
		t.Fatalf("SearchWithCandidates filter: %v", err)
	}
	if len(results) != 1 || results[0].ID != "c2" {
		t.Errorf("filter not applied: %+v", results)
	}

	// Recency pool adds rows outside the candidate set.
	results, err = store.SearchWithCandidates([]float64{1.0, 0.0}, 10, []string{"c2"}, 5, nil)
	if err != nil {
		t.Fatalf("SearchWithCandidates pool: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("expected candidate + pool rows, got %d", len(results))
	}
	found := false
	for _, r := range results {
		if r.ID == "c1" {
			found = true
		}
	}
	if !found {
		t.Errorf("recent pool row missing from results: %+v", results)
	}

	// Empty candidate set + no pool degrades to the full scan (all rows).
	results, err = store.SearchWithCandidates([]float64{1.0, 0.0}, 10, nil, 0, nil)
	if err != nil {
		t.Fatalf("SearchWithCandidates fallback: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("fallback full scan got %d results, want 5", len(results))
	}

	// Dimension mismatch: no results, same as SearchWithFilter.
	results, err = store.SearchWithCandidates([]float64{1.0, 0.0, 0.0}, 10, []string{"c2"}, 5, nil)
	if err != nil {
		t.Fatalf("SearchWithCandidates mismatch: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("dimension mismatch got %d results, want 0", len(results))
	}
}
