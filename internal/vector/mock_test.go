package vector

// MockVectorStore implements Store interface for testing.
type MockVectorStore struct {
	StoreFunc            func(dimension int, vectors []VectorRecord) error
	SearchFunc           func(query []float64, limit int) ([]SearchResult, error)
	SearchWithFilterFunc func(query []float64, limit int, filter map[string]string) ([]SearchResult, error)
	DeleteFunc           func(ids []string) error
	DeleteByDocumentFunc func(documentID string) error
	DeleteByEntityFunc   func(entityID string) error
	RebuildFunc          func() error
	StatsFunc            func() (VectorStats, error)
	DimensionFunc        func() int
	CountFunc            func() (int, error)
	CloseFunc            func() error
}

func (m *MockVectorStore) Store(dimension int, vectors []VectorRecord) error {
	if m.StoreFunc != nil {
		return m.StoreFunc(dimension, vectors)
	}
	return nil
}

func (m *MockVectorStore) Search(query []float64, limit int) ([]SearchResult, error) {
	if m.SearchFunc != nil {
		return m.SearchFunc(query, limit)
	}
	return nil, nil
}

func (m *MockVectorStore) SearchWithFilter(query []float64, limit int, filter map[string]string) ([]SearchResult, error) {
	if m.SearchWithFilterFunc != nil {
		return m.SearchWithFilterFunc(query, limit, filter)
	}
	return nil, nil
}

func (m *MockVectorStore) Delete(ids []string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ids)
	}
	return nil
}

func (m *MockVectorStore) DeleteByDocument(documentID string) error {
	if m.DeleteByDocumentFunc != nil {
		return m.DeleteByDocumentFunc(documentID)
	}
	return nil
}

func (m *MockVectorStore) DeleteByEntity(entityID string) error {
	if m.DeleteByEntityFunc != nil {
		return m.DeleteByEntityFunc(entityID)
	}
	return nil
}

func (m *MockVectorStore) Rebuild() error {
	if m.RebuildFunc != nil {
		return m.RebuildFunc()
	}
	return nil
}

func (m *MockVectorStore) Stats() (VectorStats, error) {
	if m.StatsFunc != nil {
		return m.StatsFunc()
	}
	return VectorStats{}, nil
}

func (m *MockVectorStore) Dimension() int {
	if m.DimensionFunc != nil {
		return m.DimensionFunc()
	}
	return 128
}

func (m *MockVectorStore) Count() (int, error) {
	if m.CountFunc != nil {
		return m.CountFunc()
	}
	return 0, nil
}

func (m *MockVectorStore) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}
