package aitask

import "testing"

func TestAllTypesValid(t *testing.T) {
	if len(AllTypes) != 18 {
		t.Fatalf("len(AllTypes) = %d, want 18 (manifesto §4)", len(AllTypes))
	}
	for _, typ := range AllTypes {
		if !typ.Valid() {
			t.Fatalf("type %q should be valid", typ)
		}
	}
	if Type("bogus").Valid() {
		t.Fatal("bogus type should be invalid")
	}
}

func TestCatalogComplete(t *testing.T) {
	// Todo tipo canônico tem entrada no catálogo.
	for _, typ := range AllTypes {
		if _, ok := Catalog[typ]; !ok {
			t.Fatalf("missing catalog entry for %q", typ)
		}
	}
	// Toda entrada do catálogo é um tipo canônico.
	for typ := range Catalog {
		if !typ.Valid() {
			t.Fatalf("catalog contains invalid type %q", typ)
		}
	}
}

func TestTaskMetadata(t *testing.T) {
	whisper, ok := Get(SpeechToText)
	if !ok {
		t.Fatal("Get(speech_to_text) should succeed")
	}
	if whisper.Input != ModAudio {
		t.Fatalf("STT input = %q, want audio", whisper.Input)
	}
	if whisper.Output != ModText {
		t.Fatalf("STT output = %q, want text", whisper.Output)
	}
	if len(whisper.Executors) == 0 {
		t.Fatal("STT should have executor hints")
	}

	// A GPU local (gfx1030) roda whisper — o hint deve refletir.
	foundWhisper := false
	for _, e := range whisper.Executors {
		if e.Name == "whisper" {
			foundWhisper = true
		}
	}
	if !foundWhisper {
		t.Fatal("whisper executor hint missing")
	}
}

func TestGetInvalid(t *testing.T) {
	if _, ok := Get(Type("nope")); ok {
		t.Fatal("Get(invalid) should be false")
	}
}

func TestMustGetPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("MustGet(invalid) should panic")
		}
	}()
	_ = MustGet(Type("nope"))
}

func TestHardwareClasses(t *testing.T) {
	// GPU-pesadas devem pedir HWGPU (alvo ROCm gfx1030).
	gpuTasks := []Type{ImageGeneration, VideoGeneration, Segmentation, Upscale, Detection}
	for _, typ := range gpuTasks {
		task := MustGet(typ)
		if task.Hardware != HWGPU {
			t.Errorf("%s hardware = %q, want gpu", typ, task.Hardware)
		}
	}
	// Leves devem rodar em CPU.
	cpuTasks := []Type{OCR, Embedding, Simulation, TextToSpeech}
	for _, typ := range cpuTasks {
		task := MustGet(typ)
		if task.Hardware != HWCpu {
			t.Errorf("%s hardware = %q, want cpu", typ, task.Hardware)
		}
	}
}

func TestSortedTypes(t *testing.T) {
	types := SortedTypes()
	if len(types) != 18 {
		t.Fatalf("len(SortedTypes) = %d, want 18", len(types))
	}
	for i := 1; i < len(types); i++ {
		if types[i-1] >= types[i] {
			t.Fatalf("not sorted at %d: %q >= %q", i, types[i-1], types[i])
		}
	}
}
