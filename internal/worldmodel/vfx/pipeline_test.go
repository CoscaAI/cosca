package vfx

import (
	"context"
	"testing"

	"github.com/CoscaAI/cosca/internal/worldmodel"
)

// ──────────────────────────────────────────────────────────────
// Pipeline tests
// ──────────────────────────────────────────────────────────────

func TestPipelineConfigDefaults(t *testing.T) {
	config := DefaultPipelineConfig()

	if config.MaxFrames != 300 {
		t.Errorf("MaxFrames: got %v, want 300", config.MaxFrames)
	}
	if config.MaxFPS != 30.0 {
		t.Errorf("MaxFPS: got %v, want 30.0", config.MaxFPS)
	}
}

func TestPipelineNew(t *testing.T) {
	config := DefaultPipelineConfig()
	p := NewPipeline(config)

	if p == nil {
		t.Fatal("NewPipeline returned nil")
	}
	if p.taichi == nil {
		t.Error("taichi adapter not initialized")
	}
}

func TestPipelineNewMinimal(t *testing.T) {
	config := PipelineConfig{}
	p := NewPipeline(config)

	if p.taichi != nil {
		t.Error("taichi should be nil when not configured")
	}
}

func TestPipelineNoAdapter(t *testing.T) {
	config := PipelineConfig{}
	p := NewPipeline(config)
	ctx := context.Background()

	vfxConfig := worldmodel.VFXConfig{
		Type:       worldmodel.VFXParticles,
		Resolution: 32,
		Duration:   1.0,
		FrameRate:  30.0,
	}

	_, err := p.Simulate(ctx, vfxConfig)
	if err == nil {
		t.Error("Simulate without adapter should fail")
	}

	_, err = p.Step(ctx)
	if err == nil {
		t.Error("Step without adapter should fail")
	}

	err = p.Reset(ctx)
	if err == nil {
		t.Error("Reset without adapter should fail")
	}
}

// ──────────────────────────────────────────────────────────────
// Preset tests
// ──────────────────────────────────────────────────────────────

func TestDustParticles(t *testing.T) {
	config := DustParticles(worldmodel.Vec3{X: 1, Y: 2, Z: 3})

	if config.Type != worldmodel.VFXDust {
		t.Errorf("type: got %v, want dust", config.Type)
	}
	if config.Resolution != 64 {
		t.Errorf("resolution: got %v, want 64", config.Resolution)
	}
	if config.Origin.X != 1 || config.Origin.Y != 2 || config.Origin.Z != 3 {
		t.Errorf("origin: got %v, want {1,2,3}", config.Origin)
	}
	if config.Params["count"] != 1000 {
		t.Errorf("count: got %v, want 1000", config.Params["count"])
	}
}

func TestWindEffect(t *testing.T) {
	config := WindEffect(worldmodel.Vec3{X: 1, Y: 0, Z: 0})

	if config.Type != worldmodel.VFXWind {
		t.Errorf("type: got %v, want wind", config.Type)
	}
	if config.Params["speed"] != 1.0 {
		t.Errorf("speed: got %v, want 1.0", config.Params["speed"])
	}
}

func TestFireEffect(t *testing.T) {
	config := FireEffect(worldmodel.Vec3{X: 5, Y: 0, Z: 3})

	if config.Type != worldmodel.VFXFire {
		t.Errorf("type: got %v, want fire", config.Type)
	}
	if config.Origin.X != 5 {
		t.Errorf("origin X: got %v, want 5", config.Origin.X)
	}
	if config.Params["intensity"] != 1.0 {
		t.Errorf("intensity: got %v, want 1.0", config.Params["intensity"])
	}
}

func TestSmokeEffect(t *testing.T) {
	config := SmokeEffect(worldmodel.Vec3{X: 0, Y: 0, Z: 0})

	if config.Type != worldmodel.VFXSmoke {
		t.Errorf("type: got %v, want smoke", config.Type)
	}
	if config.Params["density"] != 0.5 {
		t.Errorf("density: got %v, want 0.5", config.Params["density"])
	}
}

func TestFluidEffect(t *testing.T) {
	config := FluidEffect(worldmodel.Vec3{X: 0, Y: 5, Z: 0})

	if config.Type != worldmodel.VFXFluids {
		t.Errorf("type: got %v, want fluids", config.Type)
	}
	if config.Params["viscosity"] != 0.1 {
		t.Errorf("viscosity: got %v, want 0.1", config.Params["viscosity"])
	}
	if config.Params["gravity"] != -9.8 {
		t.Errorf("gravity: got %v, want -9.8", config.Params["gravity"])
	}
}

// ──────────────────────────────────────────────────────────────
// VFXResult tests
// ──────────────────────────────────────────────────────────────

func TestVFXResultStructure(t *testing.T) {
	result := &VFXResult{
		Type: worldmodel.VFXParticles,
		Frames: []worldmodel.Frame{
			{Step: 0, Width: 64, Height: 64, Data: []byte{0x01}},
			{Step: 1, Width: 64, Height: 64, Data: []byte{0x02}},
			{Step: 2, Width: 64, Height: 64, Data: []byte{0x03}},
		},
		Count: 3,
	}

	if result.Type != worldmodel.VFXParticles {
		t.Errorf("type: got %v, want particles", result.Type)
	}
	if result.Count != 3 {
		t.Errorf("count: got %d, want 3", result.Count)
	}
	if len(result.Frames) != 3 {
		t.Errorf("frames: got %d, want 3", len(result.Frames))
	}
	for i, f := range result.Frames {
		if f.Step != int64(i) {
			t.Errorf("frame %d step: got %d, want %d", i, f.Step, i)
		}
	}
}

// ──────────────────────────────────────────────────────────────
// Adapter config tests
// ──────────────────────────────────────────────────────────────

func TestTaichiConfig(t *testing.T) {
	config := TaichiConfig{
		Device: "cuda",
		Script: "custom/taichi.py",
	}
	adapter := NewTaichiAdapter(config)

	if adapter.config.Device != "cuda" {
		t.Errorf("device: got %v, want cuda", adapter.config.Device)
	}
	if adapter.config.Script != "custom/taichi.py" {
		t.Errorf("script: got %v, want custom/taichi.py", adapter.config.Script)
	}
}

func TestTaichiConfigDefaultScript(t *testing.T) {
	config := TaichiConfig{Device: "cpu"}
	adapter := NewTaichiAdapter(config)

	if adapter.config.Script == "" {
		t.Error("script should have default value")
	}
}

// ──────────────────────────────────────────────────────────────
// Integration test
// ──────────────────────────────────────────────────────────────

func TestPipelineIntegration(t *testing.T) {
	config := PipelineConfig{
		MaxFrames: 10,
		MaxFPS:    30.0,
		// No taichi adapter
	}

	p := NewPipeline(config)
	ctx := context.Background()

	vfxConfig := worldmodel.VFXConfig{
		Type:       worldmodel.VFXParticles,
		Resolution: 32,
		Duration:   1.0,
		FrameRate:  30.0,
	}

	_, err := p.Simulate(ctx, vfxConfig)
	if err == nil {
		t.Error("Simulate without adapter should fail")
	}
}
