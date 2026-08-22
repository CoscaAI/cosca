package cli

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/aitask"
	"github.com/CoscaAI/cosca/internal/compute"
	"github.com/CoscaAI/cosca/internal/sched"
)

// TestGPUPlanScheduler cruza task + probe real (sched puro, sem I/O).
func TestGPUPlanScheduler(t *testing.T) {
	s := sched.New(sched.WithGPUProvider(func() compute.GPUInfo {
		return compute.GPUInfo{Vendor: compute.GPUAmd, Model: "RX 6700 XT", VRAMGB: 12, HasROCm: true}
	}), sched.WithLocalModelCheck(func(aitask.Type) bool { return true }))

	// GPU task com VRAM suficiente → local-gpu.
	d := s.Decide(aitask.Segmentation, 0)
	if d.Target != sched.TargetLocalGPU {
		t.Fatalf("segmentation = %v, want local-gpu", d)
	}

	// GPU task com VRAM insuficiente → local-cpu (fallback).
	d2 := s.Decide(aitask.ImageGeneration, 20<<30)
	if d2.Target != sched.TargetLocalCPU {
		t.Fatalf("image_gen 20GiB on 12GiB = %v, want local-cpu", d2)
	}

	// Sem GPU → CPU.
	s2 := sched.New(sched.WithGPUProvider(func() compute.GPUInfo {
		return compute.GPUInfo{Vendor: compute.GPUNone}
	}))
	if d3 := s2.Decide(aitask.Segmentation, 0); d3.Target != sched.TargetLocalCPU {
		t.Fatalf("segmentation sem GPU = %v, want local-cpu", d3)
	}
}
