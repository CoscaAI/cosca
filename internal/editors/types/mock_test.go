// Package types defines the core interfaces and types for the Cosca editor
// adapter system, including the Editor interface, EditorCapabilities,
// EditorInfo, and testing mocks.
package types

// MockEditor implements Editor interface for testing.
type MockEditor struct {
	NameFunc         func() string
	VersionFunc      func() (string, error)
	DetectFunc       func() (bool, error)
	SetupFunc        func(config EditorConfig) error
	ValidateFunc     func() error
	TeardownFunc     func() error
	CapabilitiesFunc func() EditorCapabilities
	InfoFunc         func() (EditorInfo, error)
}

func (m *MockEditor) Name() string {
	if m.NameFunc != nil {
		return m.NameFunc()
	}
	return "mock-editor"
}

func (m *MockEditor) Version() (string, error) {
	if m.VersionFunc != nil {
		return m.VersionFunc()
	}
	return "1.0.0", nil
}

func (m *MockEditor) Detect() (bool, error) {
	if m.DetectFunc != nil {
		return m.DetectFunc()
	}
	return true, nil
}

func (m *MockEditor) Setup(config EditorConfig) error {
	if m.SetupFunc != nil {
		return m.SetupFunc(config)
	}
	return nil
}

func (m *MockEditor) Validate() error {
	if m.ValidateFunc != nil {
		return m.ValidateFunc()
	}
	return nil
}

func (m *MockEditor) Teardown() error {
	if m.TeardownFunc != nil {
		return m.TeardownFunc()
	}
	return nil
}

func (m *MockEditor) Capabilities() EditorCapabilities {
	if m.CapabilitiesFunc != nil {
		return m.CapabilitiesFunc()
	}
	return EditorCapabilities{}
}

func (m *MockEditor) Info() (EditorInfo, error) {
	if m.InfoFunc != nil {
		return m.InfoFunc()
	}
	return EditorInfo{Name: "mock-editor", Detected: true}, nil
}
