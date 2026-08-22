package plugins

import (
	"testing"
)

func BenchmarkPluginManifestValidation(b *testing.B) {
	m := &PluginManifest{
		ID:          "test-plugin",
		Name:        "Test Plugin",
		Version:     "1.0.0",
		Description: "A test plugin for benchmarking",
		Author:      "Cosca Team",
		Runtime:     RuntimeGo,
		Permissions: []string{"filesystem", "network"},
		Hooks:       []string{"knowledge.search", "memory.store"},
		Dependencies: []PluginDependency{
			{PluginID: "cosca-core", Version: ">=1.0.0"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := m.Validate(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBasePluginLifecycle(b *testing.B) {
	baseDir := b.TempDir()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		p := &BasePlugin{
			IDValue:      "bench-plugin",
			NameValue:    "Bench Plugin",
			VersionValue: "1.0.0",
			DescValue:    "Benchmark test plugin",
			AuthorValue:  "Cosca Team",
		}
		ctx := NewPluginContext(nil, nil, baseDir, nil)
		_ = p.Init(ctx)
		_ = p.Start()
		_ = p.Stop()
	}
}

func BenchmarkBasePluginHealth(b *testing.B) {
	p := &BasePlugin{
		IDValue:      "health-bench",
		NameValue:    "Health Bench",
		VersionValue: "1.0.0",
	}
	baseDir := b.TempDir()
	ctx := NewPluginContext(nil, nil, baseDir, nil)
	_ = p.Init(ctx)
	_ = p.Start()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.Health()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPluginContextCreation(b *testing.B) {
	baseDir := b.TempDir()
	for i := 0; i < b.N; i++ {
		_ = NewPluginContext(
			map[string]interface{}{"key": "value"},
			nil,
			baseDir,
			nil,
		)
	}
}

func BenchmarkPluginManifestValidationInvalid(b *testing.B) {
	m := &PluginManifest{
		ID:      "",
		Name:    "",
		Version: "",
		Runtime: "",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.Validate()
	}
}
