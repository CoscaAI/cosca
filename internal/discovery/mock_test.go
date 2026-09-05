package discovery

import "context"

// MockDiscoveryEngine implements a mock discovery engine for testing.
type MockDiscoveryEngine struct {
	DiscoverAllFunc     func(ctx context.Context) (*DiscoveryReport, error)
	GetCachedReportFunc func() (*DiscoveryReport, bool)
	InvalidateCacheFunc func()
}

func (m *MockDiscoveryEngine) DiscoverAll(ctx context.Context) (*DiscoveryReport, error) {
	if m.DiscoverAllFunc != nil {
		return m.DiscoverAllFunc(ctx)
	}
	return &DiscoveryReport{}, nil
}

func (m *MockDiscoveryEngine) GetCachedReport() (*DiscoveryReport, bool) {
	if m.GetCachedReportFunc != nil {
		return m.GetCachedReportFunc()
	}
	return nil, false
}

func (m *MockDiscoveryEngine) InvalidateCache() {
	if m.InvalidateCacheFunc != nil {
		m.InvalidateCacheFunc()
	}
}
