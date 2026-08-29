package vision

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ── pixelDiff (Go puro, SEM binários) ─────────────────────────────────────

// solidImage cria uma imagem RGBA w×h preenchida com uma cor sólida.
func solidImage(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func TestPixelDiff_Identical(t *testing.T) {
	a := solidImage(32, 32, color.RGBA{255, 255, 255, 255})
	b := solidImage(32, 32, color.RGBA{255, 255, 255, 255})
	ratio, regions := pixelDiff(a, b)
	if ratio != 0 {
		t.Errorf("ratio = %v, want 0 for identical frames", ratio)
	}
	if regions != 0 {
		t.Errorf("regions = %d, want 0 for identical frames", regions)
	}
}

func TestPixelDiff_Different(t *testing.T) {
	a := solidImage(32, 32, color.RGBA{255, 255, 255, 255})
	b := solidImage(32, 32, color.RGBA{255, 255, 255, 255})
	// Um único pixel preto no canto (célula 4x4 do grid). |ΔR|+|ΔG|+|ΔB| é
	// grande (≈3×65535), muito acima do limiar 90.
	b.SetRGBA(0, 0, color.RGBA{0, 0, 0, 255})
	ratio, regions := pixelDiff(a, b)
	if ratio <= 0 {
		t.Errorf("ratio = %v, want > 0 for changed frame", ratio)
	}
	if regions != 1 {
		t.Errorf("regions = %d, want exactly 1 region changed", regions)
	}
}

func TestPixelDiff_MismatchedSize(t *testing.T) {
	a := solidImage(16, 16, color.RGBA{255, 255, 255, 255})
	b := solidImage(32, 32, color.RGBA{255, 255, 255, 255})
	ratio, regions := pixelDiff(a, b)
	if ratio != 0 || regions != 0 {
		t.Errorf("size mismatch should be (0,0), got (%v,%d)", ratio, regions)
	}
}

func TestPixelDiff_Nil(t *testing.T) {
	ratio, regions := pixelDiff(nil, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	if ratio != 0 || regions != 0 {
		t.Errorf("nil should be (0,0), got (%v,%d)", ratio, regions)
	}
}

// ── Derivação de eventos (SEM binários, SEM OCR) ──────────────────────────

func TestDeriveEvents_CurrencyChanged(t *testing.T) {
	recs := []FrameRecord{
		{Frame: 1, OCRNumbers: []int{10}, PixelChange: 0, ChangedRegions: 0},
		{Frame: 2, OCRNumbers: []int{12}, PixelChange: 0.05, ChangedRegions: 6},
	}
	events := deriveEvents(recs, PipelineOptions{})
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	ev := events[0]
	if ev.Kind != EventCurrencyChanged {
		t.Errorf("kind = %q, want %q", ev.Kind, EventCurrencyChanged)
	}
	if ev.NeedsVLM {
		t.Error("currency_changed must NOT need VLM")
	}
	if ev.Frame != 2 {
		t.Errorf("frame = %d, want 2", ev.Frame)
	}
	// Integração epistêmica: OBSERVED + corroborado por 2 fontes → HIGH.
	o := ev.Observation
	if o.Epistemic != EpistemicObserved {
		t.Errorf("epistemic = %q, want OBSERVED", o.Epistemic)
	}
	if !o.Corroborated {
		t.Error("currency_changed should be corroborated")
	}
	if o.Value != 12 {
		t.Errorf("value = %v, want 12", o.Value)
	}
	if o.Level() != EvidenceHigh {
		t.Errorf("level = %q, want HIGH (2 fontes independentes)", o.Level())
	}
}

func TestDeriveEvents_UnknownInteraction(t *testing.T) {
	// Mesmo número de UI, mas pixel mudou muito (>2%) e muitas regiões (>4).
	recs := []FrameRecord{
		{Frame: 1, OCRNumbers: []int{10}, PixelChange: 0, ChangedRegions: 0},
		{Frame: 2, OCRNumbers: []int{10}, PixelChange: 0.07, ChangedRegions: 9},
	}
	events := deriveEvents(recs, PipelineOptions{})
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	ev := events[0]
	if ev.Kind != EventUnknownInteraction {
		t.Errorf("kind = %q, want %q", ev.Kind, EventUnknownInteraction)
	}
	if !ev.NeedsVLM {
		t.Error("unknown_interaction must need VLM")
	}
	// Integração epistêmica: OBSERVED + corroborado por 1 fonte → MEDIUM.
	o := ev.Observation
	if o.Epistemic != EpistemicObserved {
		t.Errorf("epistemic = %q, want OBSERVED", o.Epistemic)
	}
	if !o.Corroborated {
		t.Error("unknown_interaction should be corroborated")
	}
	if o.Level() != EvidenceMedium {
		t.Errorf("level = %q, want MEDIUM (1 fonte pixel_diff)", o.Level())
	}
}

func TestDeriveEvents_None(t *testing.T) {
	// Mesmo número e pouca mudança → nada relevante.
	recs := []FrameRecord{
		{Frame: 1, OCRNumbers: []int{10}, PixelChange: 0, ChangedRegions: 0},
		{Frame: 2, OCRNumbers: []int{10}, PixelChange: 0.001, ChangedRegions: 1},
	}
	events := deriveEvents(recs, PipelineOptions{})
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	ev := events[0]
	if ev.Kind != EventNone {
		t.Errorf("kind = %q, want %q", ev.Kind, EventNone)
	}
	if ev.NeedsVLM {
		t.Error("none must NOT need VLM")
	}
	// Integração epistêmica: nada detectado → UNKNOWN, sem corroborar → NONE.
	o := ev.Observation
	if o.Epistemic != EpistemicUnknown {
		t.Errorf("epistemic = %q, want UNKNOWN", o.Epistemic)
	}
	if o.Corroborated {
		t.Error("none should not be corroborated")
	}
	if o.Level() != EvidenceNone {
		t.Errorf("level = %q, want NONE", o.Level())
	}
}

func TestDeriveEvents_FirstFrameSkipped(t *testing.T) {
	// Um único frame não gera evento algum (não há anterior p/ comparar).
	recs := []FrameRecord{{Frame: 1, OCRNumbers: []int{10}}}
	events := deriveEvents(recs, PipelineOptions{})
	if len(events) != 0 {
		t.Errorf("single frame should yield 0 events, got %d", len(events))
	}
}

func TestDeriveEvents_CustomThresholds(t *testing.T) {
	// Com thresholds mais exigentes, a mesma mudança vira "none".
	opts := PipelineOptions{ThresholdRatio: 0.5, ThresholdRegions: 20}
	recs := []FrameRecord{
		{Frame: 1, OCRNumbers: []int{10}, PixelChange: 0, ChangedRegions: 0},
		{Frame: 2, OCRNumbers: []int{10}, PixelChange: 0.07, ChangedRegions: 9},
	}
	events := deriveEvents(recs, opts)
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if events[0].Kind != EventNone {
		t.Errorf("kind = %q, want %q with stricter thresholds", events[0].Kind, EventNone)
	}
}

func TestExtractNumbers(t *testing.T) {
	tests := []struct {
		in   string
		want []int
	}{
		{"", []int{}},
		{"sem numeros aqui", []int{}},
		{"coin: 42", []int{42}},
		{"a1b2c34", []int{1, 2, 34}},
		{"10 + 10 = 20", []int{10, 10, 20}},
	}
	for _, tt := range tests {
		got := extractNumbers(tt.in)
		if !sameInts(got, tt.want) {
			t.Errorf("extractNumbers(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestSameInts(t *testing.T) {
	if sameInts([]int{1, 2}, []int{1, 2}) != true {
		t.Error("equal slices should be equal")
	}
	if sameInts([]int{1, 2, 3}, []int{1, 2}) != false {
		t.Error("different length should be false")
	}
	if sameInts(nil, []int{}) != true {
		t.Error("nil and empty should be equal")
	}
}

func TestPipelineOptions_Defaults(t *testing.T) {
	o := PipelineOptions{}.withDefaults()
	if o.FPS != 1 {
		t.Errorf("FPS = %d, want 1", o.FPS)
	}
	if o.OCRLang != "eng" {
		t.Errorf("OCRLang = %q, want eng", o.OCRLang)
	}
	if o.ThresholdPixel != defaultPixelLevel {
		t.Errorf("ThresholdPixel = %v, want %v", o.ThresholdPixel, defaultPixelLevel)
	}
	if o.ThresholdRatio != 0.02 {
		t.Errorf("ThresholdRatio = %v, want 0.02 (2%% do spec)", o.ThresholdRatio)
	}
	if o.ThresholdRegions != 4 {
		t.Errorf("ThresholdRegions = %d, want 4", o.ThresholdRegions)
	}
}

// ── AnalyzeVideo: entrada inválida / vídeo inexistente → erro limpo ───────

// TestAnalyzeVideo_EmptyPath não depende de nenhum binário: o caminho vazio é
// rejeitado antes de tocar em ffprobe.
func TestAnalyzeVideo_EmptyPath(t *testing.T) {
	_, err := AnalyzeVideo(context.Background(), "", PipelineOptions{})
	if err == nil {
		t.Fatal("expected error for empty video path")
	}
	if !strings.Contains(err.Error(), "caminho de vídeo vazio") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestAnalyzeVideo_NonexistentPath: vídeo que não existe → erro limpo com
// prefixo `vision:`. Passa mesmo sem ffprobe (retorna ffprobe-not-found) ou
// com ffprobe (retorna file-not-found) — ambos são erros limpos.
func TestAnalyzeVideo_NonexistentPath(t *testing.T) {
	_, err := AnalyzeVideo(context.Background(), "/nonexistent/video.mp4", PipelineOptions{})
	if err == nil {
		t.Fatal("expected error for nonexistent video")
	}
	if !strings.Contains(err.Error(), "vision:") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ── E2E (opcional): só roda se ffmpeg/ffprobe/tesseract existirem, senão SKIP.

// hasPipelineBins pula o teste E2E se algum binário do pipeline não existir.
func hasPipelineBins(t *testing.T) {
	t.Helper()
	for _, bin := range []string{"ffmpeg", "ffprobe", "tesseract"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s não disponível, pulando teste E2E", bin)
		}
	}
}

// makeTestFrameVideo gera um vídeo sintético curto (testsrc) com frames que
// mudam, para o pipeline E2E extrair e observar. Sem depender de OCR real.
func makeTestFrameVideo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "test.mp4")
	args := []string{
		"-y", "-f", "lavfi",
		"-i", "testsrc=duration=2:size=64x64:rate=10",
		"-c:v", "libx264", "-preset", "ultrafast",
		path,
	}
	cmd := exec.Command("ffmpeg", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("makeTestFrameVideo: %v: %s", err, out)
	}
	return path
}

// TestAnalyzeVideo_E2E valida o pipeline de ponta a ponta. Skip se os binários
// não existirem — NUNCA falha por ausência de dependência externa.
func TestAnalyzeVideo_E2E(t *testing.T) {
	hasPipelineBins(t)
	dir := t.TempDir()
	vid := makeTestFrameVideo(t, dir)

	result, err := AnalyzeVideo(context.Background(), vid, PipelineOptions{})
	if err != nil {
		t.Fatalf("AnalyzeVideo E2E failed: %v", err)
	}
	if result.Probe.Path != vid {
		t.Errorf("probe path mismatch: %q", result.Probe.Path)
	}
	if len(result.Frames) < 2 {
		t.Fatalf("expected >= 2 frames, got %d", len(result.Frames))
	}
	// Frames devem estar em ordem crescente de tempo.
	for i := 1; i < len(result.Frames); i++ {
		if result.Frames[i].Time < result.Frames[i-1].Time {
			t.Errorf("frame %d time %v < frame %d time %v", i, result.Frames[i].Time, i-1, result.Frames[i-1].Time)
		}
	}
	// Número de eventos deve ser frames-1 (primeiro não tem anterior).
	if len(result.Events) != len(result.Frames)-1 {
		t.Errorf("events = %d, want %d", len(result.Events), len(result.Frames)-1)
	}
	// Cada evento deve ter Observation preenchida com epistemic válido.
	for _, ev := range result.Events {
		if ev.Observation.Epistemic != EpistemicObserved && ev.Observation.Epistemic != EpistemicUnknown {
			t.Errorf("event frame %d has invalid epistemic %q", ev.Frame, ev.Observation.Epistemic)
		}
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────

func TestWritePNG(t *testing.T) {
	// Sanidade para o decodificador de PNG usado no pipeline.
	dir := t.TempDir()
	path := filepath.Join(dir, "frame.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, solidImage(4, 4, color.RGBA{1, 2, 3, 255})); err != nil {
		t.Fatal(err)
	}
	f.Close()

	img, err := loadPNG(path)
	if err != nil {
		t.Fatalf("loadPNG: %v", err)
	}
	if img.Bounds().Dx() != 4 || img.Bounds().Dy() != 4 {
		t.Errorf("decoded size = %dx%d, want 4x4", img.Bounds().Dx(), img.Bounds().Dy())
	}
}
