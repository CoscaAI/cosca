package search

import "context"

// MockSearchEngine implements a mock search engine for testing.
type MockSearchEngine struct {
	SearchFunc func(ctx context.Context, params SearchParams) (*SearchResults, error)
}

func (m *MockSearchEngine) Search(ctx context.Context, params SearchParams) (*SearchResults, error) {
	if m.SearchFunc != nil {
		return m.SearchFunc(ctx, params)
	}
	return &SearchResults{}, nil
}
