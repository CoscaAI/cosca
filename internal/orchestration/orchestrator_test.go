package orchestration

import (
	"context"
	"testing"
)

func TestEngineRejectsNilRequest(t *testing.T) {
	engine := &Engine{}

	if _, err := engine.Execute(context.Background(), nil); err == nil {
		t.Fatal("Execute accepted a nil request")
	}
	if _, err := engine.ExecuteStream(context.Background(), nil); err == nil {
		t.Fatal("ExecuteStream accepted a nil request")
	}
}
