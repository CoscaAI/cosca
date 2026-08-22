package knowledge

import (
	"context"

	"github.com/CoscaAI/cosca/internal/search"
)

// MockEngine implements a mock knowledge engine for testing.
type MockEngine struct {
	SearchFunc   func(ctx context.Context, params search.SearchParams) (*search.SearchResults, error)
	QueryFunc    func(ctx context.Context, query string) (*search.SearchResults, error)
	GetStatsFunc func() (*Stats, error)
	CloseFunc    func() error
}

func (m *MockEngine) Search(ctx context.Context, params search.SearchParams) (*search.SearchResults, error) {
	if m.SearchFunc != nil {
		return m.SearchFunc(ctx, params)
	}
	return &search.SearchResults{}, nil
}

func (m *MockEngine) Query(ctx context.Context, query string) (*search.SearchResults, error) {
	if m.QueryFunc != nil {
		return m.QueryFunc(ctx, query)
	}
	return &search.SearchResults{}, nil
}

func (m *MockEngine) GetStats() (*Stats, error) {
	if m.GetStatsFunc != nil {
		return m.GetStatsFunc()
	}
	return &Stats{}, nil
}

func (m *MockEngine) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}
