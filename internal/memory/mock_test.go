package memory

import "context"

// MockStore implements Store interface for testing.
type MockStore struct {
	SaveFunc   func(ctx context.Context, record MemoryRecord) (*MemoryRecord, error)
	GetFunc    func(ctx context.Context, id string) (*MemoryRecord, error)
	DeleteFunc func(ctx context.Context, id string) error
	SearchFunc func(ctx context.Context, query string, opts SearchOptions) ([]MemoryRecord, error)
	IndexFunc  func(ctx context.Context, record MemoryRecord) error
	PruneFunc  func(ctx context.Context) (int, error)
	StatsFunc  func(ctx context.Context) (LayerStats, error)
	CloseFunc  func() error
}

func (m *MockStore) Save(ctx context.Context, record MemoryRecord) (*MemoryRecord, error) {
	if m.SaveFunc != nil {
		return m.SaveFunc(ctx, record)
	}
	record.ID = "mock-id"
	return &record, nil
}

func (m *MockStore) Get(ctx context.Context, id string) (*MemoryRecord, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, id)
	}
	return &MemoryRecord{ID: id}, nil
}

func (m *MockStore) Delete(ctx context.Context, id string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	return nil
}

func (m *MockStore) Search(ctx context.Context, query string, opts SearchOptions) ([]MemoryRecord, error) {
	if m.SearchFunc != nil {
		return m.SearchFunc(ctx, query, opts)
	}
	return []MemoryRecord{}, nil
}

func (m *MockStore) Index(ctx context.Context, record MemoryRecord) error {
	if m.IndexFunc != nil {
		return m.IndexFunc(ctx, record)
	}
	return nil
}

func (m *MockStore) Prune(ctx context.Context) (int, error) {
	if m.PruneFunc != nil {
		return m.PruneFunc(ctx)
	}
	return 0, nil
}

func (m *MockStore) Stats(ctx context.Context) (LayerStats, error) {
	if m.StatsFunc != nil {
		return m.StatsFunc(ctx)
	}
	return LayerStats{}, nil
}

func (m *MockStore) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}
