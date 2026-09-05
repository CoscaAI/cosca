package worldmodel

import (
	"context"
	"testing"
)

// mockAdapter is a test adapter.
type mockAdapter struct {
	name  string
	state AdapterState
}

func (m *mockAdapter) Name() string              { return m.name }
func (m *mockAdapter) State() AdapterState       { return m.state }
func (m *mockAdapter) Start(_ context.Context) error { m.state = AdapterRunning; return nil }
func (m *mockAdapter) Stop(_ context.Context) error  { m.state = AdapterStopped; return nil }
func (m *mockAdapter) Health(_ context.Context) error { return nil }

func TestRegistryRegister(t *testing.T) {
	r := NewRegistry()
	a := &mockAdapter{name: "test-adapter", state: AdapterCreated}

	if err := r.Register(a); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Duplicate should fail
	if err := r.Register(a); err == nil {
		t.Fatal("Register duplicate should fail")
	}
}

func TestRegistryGet(t *testing.T) {
	r := NewRegistry()
	a := &mockAdapter{name: "clip", state: AdapterCreated}
	r.Register(a)

	got, err := r.Get("clip")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name() != "clip" {
		t.Errorf("Get: got %v, want clip", got.Name())
	}

	// Missing should fail
	if _, err := r.Get("nonexistent"); err == nil {
		t.Fatal("Get nonexistent should fail")
	}
}

func TestRegistryList(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockAdapter{name: "a"})
	r.Register(&mockAdapter{name: "b"})
	r.Register(&mockAdapter{name: "c"})

	list := r.List()
	if len(list) != 3 {
		t.Errorf("List: got %d, want 3", len(list))
	}
}

func TestRegistryStartAll(t *testing.T) {
	r := NewRegistry()
	a1 := &mockAdapter{name: "a", state: AdapterCreated}
	a2 := &mockAdapter{name: "b", state: AdapterCreated}
	r.Register(a1)
	r.Register(a2)

	if err := r.StartAll(context.Background()); err != nil {
		t.Fatalf("StartAll: %v", err)
	}

	if a1.state != AdapterRunning {
		t.Errorf("adapter a: got %v, want running", a1.state)
	}
	if a2.state != AdapterRunning {
		t.Errorf("adapter b: got %v, want running", a2.state)
	}
}

func TestRegistryStopAll(t *testing.T) {
	r := NewRegistry()
	a := &mockAdapter{name: "a", state: AdapterRunning}
	r.Register(a)

	if err := r.StopAll(context.Background()); err != nil {
		t.Fatalf("StopAll: %v", err)
	}

	if a.state != AdapterStopped {
		t.Errorf("adapter a: got %v, want stopped", a.state)
	}
}

func TestRegistryHealthAll(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockAdapter{name: "a"})
	r.Register(&mockAdapter{name: "b"})

	results := r.HealthAll(context.Background())
	if len(results) != 2 {
		t.Errorf("HealthAll: got %d results, want 2", len(results))
	}
	for name, err := range results {
		if err != nil {
			t.Errorf("HealthAll %s: %v", name, err)
		}
	}
}
