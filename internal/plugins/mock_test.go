package plugins

import "context"

// MockPlugin implements Plugin interface for testing.
type MockPlugin struct {
	IDValue          string
	NameValue        string
	VersionValue     string
	DescriptionValue string
	AuthorValue      string
	InitFunc         func(ctx *PluginContext) error
	StartFunc        func() error
	StopFunc         func() error
	HealthFunc       func() (PluginHealth, error)
	RunFunc          func(ctx context.Context, params string) (string, error)
}

func (m *MockPlugin) ID() string          { return m.IDValue }
func (m *MockPlugin) Name() string        { return m.NameValue }
func (m *MockPlugin) Version() string     { return m.VersionValue }
func (m *MockPlugin) Description() string { return m.DescriptionValue }
func (m *MockPlugin) Author() string      { return m.AuthorValue }

func (m *MockPlugin) Init(ctx *PluginContext) error {
	if m.InitFunc != nil {
		return m.InitFunc(ctx)
	}
	return nil
}

func (m *MockPlugin) Start() error {
	if m.StartFunc != nil {
		return m.StartFunc()
	}
	return nil
}

func (m *MockPlugin) Stop() error {
	if m.StopFunc != nil {
		return m.StopFunc()
	}
	return nil
}

func (m *MockPlugin) Health() (PluginHealth, error) {
	if m.HealthFunc != nil {
		return m.HealthFunc()
	}
	return PluginHealth{PluginID: m.IDValue, Status: "healthy", State: PluginStateStarted}, nil
}

func (m *MockPlugin) Run(ctx context.Context, params string) (string, error) {
	if m.RunFunc != nil {
		return m.RunFunc(ctx, params)
	}
	return "", nil
}

// MockRuntimeAPI implements RuntimeAPI for testing.
type MockRuntimeAPI struct {
	GetConfigFunc      func(key string) (interface{}, error)
	SetConfigFunc      func(key string, value interface{}) error
	EmitEventFunc      func(eventType string, data interface{}) error
	RegisterHookFunc   func(hookPoint string, handler func(args interface{}) error) (string, error)
	UnregisterHookFunc func(hookID string) error
}

func (m *MockRuntimeAPI) GetConfig(key string) (interface{}, error) {
	if m.GetConfigFunc != nil {
		return m.GetConfigFunc(key)
	}
	return nil, nil
}

func (m *MockRuntimeAPI) SetConfig(key string, value interface{}) error {
	if m.SetConfigFunc != nil {
		return m.SetConfigFunc(key, value)
	}
	return nil
}

func (m *MockRuntimeAPI) EmitEvent(eventType string, data interface{}) error {
	if m.EmitEventFunc != nil {
		return m.EmitEventFunc(eventType, data)
	}
	return nil
}

func (m *MockRuntimeAPI) RegisterHook(hookPoint string, handler func(args interface{}) error) (string, error) {
	if m.RegisterHookFunc != nil {
		return m.RegisterHookFunc(hookPoint, handler)
	}
	return "hook-id", nil
}

func (m *MockRuntimeAPI) UnregisterHook(hookID string) error {
	if m.UnregisterHookFunc != nil {
		return m.UnregisterHookFunc(hookID)
	}
	return nil
}

// MockPluginLogger implements PluginLogger for testing.
type MockPluginLogger struct {
	DebugFunc func(msg string, keysAndValues ...interface{})
	InfoFunc  func(msg string, keysAndValues ...interface{})
	WarnFunc  func(msg string, keysAndValues ...interface{})
	ErrorFunc func(msg string, keysAndValues ...interface{})
}

func (m *MockPluginLogger) Debug(msg string, keysAndValues ...interface{}) {
	if m.DebugFunc != nil {
		m.DebugFunc(msg, keysAndValues...)
	}
}

func (m *MockPluginLogger) Info(msg string, keysAndValues ...interface{}) {
	if m.InfoFunc != nil {
		m.InfoFunc(msg, keysAndValues...)
	}
}

func (m *MockPluginLogger) Warn(msg string, keysAndValues ...interface{}) {
	if m.WarnFunc != nil {
		m.WarnFunc(msg, keysAndValues...)
	}
}

func (m *MockPluginLogger) Error(msg string, keysAndValues ...interface{}) {
	if m.ErrorFunc != nil {
		m.ErrorFunc(msg, keysAndValues...)
	}
}
