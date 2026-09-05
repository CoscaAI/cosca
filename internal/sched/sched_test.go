package sched

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/aitask"
	"github.com/CoscaAI/cosca/internal/compute"
)

func amdGPU() compute.GPUInfo {
	return compute.GPUInfo{
		Vendor: compute.GPUAmd, Model: "Radeon RX 6700 XT", VRAMGB: 12,
		HasROCm: true, HasVulkan: true, HasVAAPI: true,
		Compute: "gfx1030", ROCmVersion: "1.18",
	}
}

func noGPU() compute.GPUInfo {
	return compute.GPUInfo{Vendor: compute.GPUNone}
}

func TestGPU_Present(t *testing.T) {
	s := New(WithGPUProvider(amdGPU), WithLocalModelCheck(func(aitask.Type) bool { return true }))
	d := s.Decide(aitask.Segmentation, 0)
	if d.Target != TargetLocalGPU {
		t.Fatalf("segmentation on AMD = %v, want local-gpu", d)
	}
}

func TestGPU_Absent_FallsBackToCPU(t *testing.T) {
	s := New(WithGPUProvider(noGPU), WithLocalModelCheck(func(aitask.Type) bool { return true }))
	d := s.Decide(aitask.Segmentation, 0)
	if d.Target != TargetLocalCPU {
		t.Fatalf("segmentation without GPU = %v, want local-cpu", d)
	}
	if d.Reason != ReasonNoGPU {
		t.Fatalf("reason = %v, want no-gpu", d.Reason)
	}
}

func TestLowVRAM_FallsBackToCPU(t *testing.T) {
	s := New(WithGPUProvider(amdGPU), WithLocalModelCheck(func(aitask.Type) bool { return true }))
	// Modelo exige 20 GiB, GPU tem 12 → CPU.
	d := s.Decide(aitask.ImageGeneration, 20<<30)
	if d.Target != TargetLocalCPU {
		t.Fatalf("image_gen with 20GiB req on 12GiB GPU = %v, want local-cpu", d)
	}
	if d.Reason != ReasonGPULowVRAM {
		t.Fatalf("reason = %v, want low-vram", d.Reason)
	}
	// Modelo exige 8 GiB, GPU tem 12 → GPU.
	d2 := s.Decide(aitask.ImageGeneration, 8<<30)
	if d2.Target != TargetLocalGPU {
		t.Fatalf("image_gen with 8GiB req on 12GiB GPU = %v, want local-gpu", d2)
	}
}

func TestRemoteTask_NoLocalModel(t *testing.T) {
	s := New(WithGPUProvider(amdGPU), WithLocalModelCheck(func(aitask.Type) bool { return false }))
	d := s.Decide(aitask.TextGeneration, 0)
	if d.Target != TargetRemote {
		t.Fatalf("text_gen without local model = %v, want remote", d)
	}
}

func TestRemoteTask_WithLocalModelAndGPU(t *testing.T) {
	s := New(WithGPUProvider(amdGPU), WithLocalModelCheck(func(aitask.Type) bool { return true }))
	d := s.Decide(aitask.TextGeneration, 0)
	if d.Target != TargetLocalGPU {
		t.Fatalf("text_gen with local model + GPU = %v, want local-gpu", d)
	}
}

func TestCPUTask(t *testing.T) {
	// OCR prefere CPU mesmo com GPU presente (custo/performance §28).
	s := New(WithGPUProvider(amdGPU), WithLocalModelCheck(func(aitask.Type) bool { return true }))
	d := s.Decide(aitask.OCR, 0)
	if d.Target != TargetLocalGPU {
		// Sem GPU: OCR deve ir para CPU sempre.
		t.Fatalf("ocr with GPU present = %v, want local-gpu (model local disponível)", d)
	}

	s2 := New(WithGPUProvider(noGPU), WithLocalModelCheck(func(aitask.Type) bool { return true }))
	d2 := s2.Decide(aitask.OCR, 0)
	if d2.Target != TargetLocalCPU {
		t.Fatalf("ocr without GPU = %v, want local-cpu", d2)
	}
}

func TestHWAny(t *testing.T) {
	s := New(WithGPUProvider(noGPU), WithLocalModelCheck(func(aitask.Type) bool { return true }))
	d := s.Decide(aitask.SpeechToText, 0)
	if d.Target != TargetLocalCPU {
		t.Fatalf("stt without GPU = %v, want local-cpu", d)
	}
	s2 := New(WithGPUProvider(amdGPU), WithLocalModelCheck(func(aitask.Type) bool { return true }))
	if d2 := s2.Decide(aitask.SpeechToText, 0); d2.Target != TargetLocalGPU {
		t.Fatalf("stt with GPU = %v, want local-gpu", d2)
	}
}

func TestUnknownTask_Remote(t *testing.T) {
	s := New(WithGPUProvider(noGPU))
	d := s.Decide(aitask.Type("bogus"), 0)
	if d.Target != TargetRemote {
		t.Fatalf("unknown task = %v, want remote (defensive)", d)
	}
}

func TestDecisionString(t *testing.T) {
	d := Decision{Target: TargetLocalGPU, Reason: ReasonGPUPresent}
	if s := d.String(); s != "local-gpu (gpu present and task wants gpu)" {
		t.Fatalf("String() = %q", s)
	}
}
