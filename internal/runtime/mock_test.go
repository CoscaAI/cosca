package runtime

import "context"

// MockSubsystem implements Subsystem interface for testing.
type MockSubsystem struct {
	NameFunc   func() string
	StartFunc  func(ctx context.Context) error
	StopFunc   func(ctx context.Context) error
	HealthFunc func() ComponentStatus
}

func (m *MockSubsystem) Name() string {
	if m.NameFunc != nil {
		return m.NameFunc()
	}
	return "mock"
}

func (m *MockSubsystem) Start(ctx context.Context) error {
	if m.StartFunc != nil {
		return m.StartFunc(ctx)
	}
	return nil
}

func (m *MockSubsystem) Stop(ctx context.Context) error {
	if m.StopFunc != nil {
		return m.StopFunc(ctx)
	}
	return nil
}

func (m *MockSubsystem) Health() ComponentStatus {
	if m.HealthFunc != nil {
		return m.HealthFunc()
	}
	return StatusHealthy
}
