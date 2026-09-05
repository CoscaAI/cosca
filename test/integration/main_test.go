
//go:build integration

// Package integration provides integration tests that exercise real
// combinations of multiple Cosca components working together.
//
// These tests are excluded from normal unit test runs. Execute them with:
//
//	go test -tags integration -count=1 -v ./test/integration/
package integration

import (
	"fmt"
	"os"
	"testing"
)

// TestMain initializes the integration test environment.
func TestMain(m *testing.M) {
	// Verify working directory
	if _, err := os.Getwd(); err != nil {
		fmt.Fprintf(os.Stderr, "integration: failed to get working directory: %v\n", err)
		os.Exit(1)
	}

	// Run all tests
	code := m.Run()
	os.Exit(code)
}
