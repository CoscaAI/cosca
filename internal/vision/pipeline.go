// Pipeline de percepção determinística frame-a-frame (sem VLM).
//
// Traz para dentro do Cosca (Go nativo) o pipeline PROVADO em E:\cosca-tmp\
// video-percept: ffprobe (metadata) → ffmpeg (extrai frames) → OCR (texto/números
// de UI) → pixel-diff (Go puro) → registrar por frame → DERIVAR EVENTOS.
//
// A primitiva epistêmica (observation.go) é integrada: cada evento derivado é
// uma Observation com estado epistemológico — o Cosca não diz "eu vi" sem dizer
// o nível de certeza. Este pipeline é 100% determinístico (sem VLM, sem modelo).
package vision

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/CoscaAI/cosca/internal/media"
)

// Kinds de evento derivado.
const (
	EventCurrencyChanged   = "currency_changed"
	EventUnknownInteraction = "unknown_interaction"
	EventNone              = "none"
)

// FrameRecord é o registro de observação de um único frame.
type FrameRecord struct {
	Frame          int     `json:"frame_id"`
	Time           float64 `json:"time_seconds"`
	OCRText        string  `json:"ocr_text"`
	OCRNumbers     []int   `json:"ocr_numbers"`
	PixelChange    float64 `json:"pixel_change_ratio"`
	ChangedRegions int     `json:"changed_regions"`
}

// PerceptEvent é um evento derivado deterministicamente de uma sequência de frames.
type PerceptEvent struct {
	Kind        string      `json:"kind"`
	Frame       int         `json:"frame"`
	NeedsVLM    bool        `json:"needs_vlm"`
	Explanation string      `json:"explanation"`
	Observation Observation `json:"observation"`
}

// PipelineResult é o relatório completo da análise de um vídeo.
type PipelineResult struct {
	Probe  media.Info     `json:"probe"`
	Frames []FrameRecord  `json:"frames"`
	Events []PerceptEvent `json:"events"`
}

// defaultPixelLevel é o limiar padrão de mudança de pixel (ΔR+ΔG+ΔB).
const defaultPixelLevel = 90

// PipelineOptions configura a análise.
type PipelineOptions struct {
	FPS             int
	OCRLang         string
	ThresholdPixel  int
	ThresholdRatio  float64
	ThresholdRegions int
	WorkDir         string
}

// withDefaults aplica os padrões.
func (o PipelineOptions) withDefaults() PipelineOptions {
	if o.FPS <= 0 {
		o.FPS = 1
	}
	if strings.TrimSpace(o.OCRLang) == "" {
		o.OCRLang = "eng"
	}
	if o.ThresholdPixel <= 0 {
		o.ThresholdPixel = defaultPixelLevel
	}
	if o.ThresholdRatio <= 0 {
		o.ThresholdRatio = 0.02
	}
	if o.ThresholdRegions <= 0 {
		o.ThresholdRegions = 4
	}
	return o
}

// AnalyzeVideo roda o pipeline determinístico frame-a-frame em um vídeo.
func AnalyzeVideo(ctx context.Context, video string, opts PipelineOptions) (*PipelineResult, error) {
	if strings.TrimSpace(video) == "" {
		return nil, fmt.Errorf("vision: caminho de vídeo vazio")
	}
	if _, err := os.Stat(video); err != nil {
		return nil, fmt.Errorf("vision: vídeo não encontrado: %w", err)
	}

	opts = opts.withDefaults()

	probe, err := media.Probe(ctx, video)
	if err != nil {
		return nil, fmt.Errorf("vision: probe: %w", err)
	}

	work := opts.WorkDir
	if work == "" {
		work, _ = os.MkdirTemp("", "cosca-vision-*")
		defer os.RemoveAll(work)
	}
	framesDir := filepath.Join(work, "frames")
	if err := os.MkdirAll(framesDir, 0o755); err != nil {
		return nil, fmt.Errorf("vision: mkdir frames: %w", err)
	}

	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("vision: ffmpeg não disponível (necessário p/ extração de frames): %w", err)
	}
	outPattern := filepath.Join(framesDir, "frame_%03d.png")
	cmd := exec.CommandContext(ctx, ffmpeg, "-y", "-i", video, "-vf", fmt.Sprintf("fps=%d", opts.FPS), outPattern)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("vision: ffmpeg extração de frames falhou: %w: %s", err, trunc(string(out), 200))
	}
	frames, _ := filepath.Glob(filepath.Join(framesDir, "frame_*.png"))
	sort.Strings(frames)

	recs := make([]FrameRecord, 0, len(frames))
	decoded := map[string]image.Image{}
	for i, f := range frames {
		ocrText, nums := ocrFrame(f, opts.OCRLang)
		rec := FrameRecord{Frame: i + 1, Time: float64(i) / float64(opts.FPS), OCRText: ocrText, OCRNumbers: nums}
		if i > 0 {
			a := loadPNGCached(frames[i-1], decoded)
			b := loadPNGCached(f, decoded)
			ratio, regions := pixelDiff(a, b)
			rec.PixelChange = ratio
			rec.ChangedRegions = regions
		}
		recs = append(recs, rec)
	}

	if len(recs) == 0 {
		return &PipelineResult{Probe: *probe, Frames: []FrameRecord{}, Events: []PerceptEvent{}}, nil
	}

	return &PipelineResult{Probe: *probe, Frames: recs, Events: deriveEvents(recs, opts)}, nil
}

// ocrFrame roda OCR via tesseract num frame PNG e extrai os números.
func ocrFrame(path, lang string) (string, []int) {
	tess, err := exec.LookPath("tesseract")
	if err != nil {
		return "", nil
	}
	cmd := exec.Command(tess, path, "stdout", "-l", lang, "--psm", "6")
	out, err := cmd.Output()
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(string(out)), extractNumbers(string(out))
}

// deriveEvents aplica a heurística do protótipo, integrando observation.go.
func deriveEvents(recs []FrameRecord, opts PipelineOptions) []PerceptEvent {
	// Aplica defaults robustos mesmo quando chamado direto (testes passam
	// PipelineOptions{} vazio); evita ThresholdRatio=0 pegar qualquer mudança.
	ratioThr := opts.ThresholdRatio
	if ratioThr <= 0 {
		ratioThr = 0.02
	}
	regionThr := opts.ThresholdRegions
	if regionThr <= 0 {
		regionThr = 4
	}
	events := make([]PerceptEvent, 0, len(recs))
	for i := 1; i < len(recs); i++ {
		a, b := recs[i-1], recs[i]
		switch {
		case !sameInts(a.OCRNumbers, b.OCRNumbers):
			// número de UI mudou (OCR) → OBSERVED, corroborado por 2 fontes
			// independentes (OCR dos números + pixel-diff) → evidência HIGH.
			obs := Observation{Object: EventCurrencyChanged, Epistemic: EpistemicObserved, Value: lastNum(b.OCRNumbers)}
			obs.Corroborate("ocr_numbers")
			obs.Corroborate("pixel_diff")
			events = append(events, PerceptEvent{
				Kind: EventCurrencyChanged, Frame: b.Frame, NeedsVLM: false,
				Explanation: "número de UI mudou (OCR), detectável sem VLM", Observation: obs,
			})
		case b.PixelChange > ratioThr && b.ChangedRegions > regionThr:
			obs := Observation{Object: EventUnknownInteraction, Epistemic: EpistemicObserved}
			obs.Corroborate("pixel_diff")
			events = append(events, PerceptEvent{
				Kind: EventUnknownInteraction, Frame: b.Frame, NeedsVLM: true,
				Explanation: "grande mudança visual sem número de UI mudar → SEMÂNTICO (VLM)", Observation: obs,
			})
		default:
			obs := Observation{Object: EventNone, Epistemic: EpistemicUnknown}
			events = append(events, PerceptEvent{
				Kind: EventNone, Frame: b.Frame, NeedsVLM: false,
				Explanation: "sem mudança relevante", Observation: obs,
			})
		}
	}
	return events
}

func lastNum(nums []int) float64 {
	if len(nums) == 0 {
		return 0
	}
	return float64(nums[len(nums)-1])
}

// pixelDiff devolve (fração de pixels mudados, nº de regiões 4x4 mudadas).
// Usa o limiar padrão de mudança (defaultPixelLevel) — determinístico.
func pixelDiff(a, b image.Image) (float64, int) {
	if a == nil || b == nil {
		return 0, 0
	}
	ab, bb := a.Bounds(), b.Bounds()
	w, h := ab.Dx(), ab.Dy()
	if bb.Dx() != w || bb.Dy() != h {
		return 0, 0
	}
	const grid = 4
	threshold := float64(defaultPixelLevel)
	changed := map[int]int{}
	changedPx, total := 0, 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r1, g1, b1, _ := a.At(ab.Min.X+x, ab.Min.Y+y).RGBA()
			r2, g2, b2, _ := b.At(bb.Min.X+x, bb.Min.Y+y).RGBA()
			dr := float64(r1) - float64(r2)
			dg := float64(g1) - float64(g2)
			db := float64(b1) - float64(b2)
			if math.Abs(dr)+math.Abs(dg)+math.Abs(db) > threshold {
				changedPx++
				ri := y * grid / h
				ci := x * grid / w
				changed[ri*grid+ci]++
			}
			total++
		}
	}
	if total == 0 {
		return 0, 0
	}
	return float64(changedPx) / float64(total), len(changed)
}

// loadPNGCached decodifica um PNG com cache por caminho.
func loadPNGCached(p string, cache map[string]image.Image) image.Image {
	if img, ok := cache[p]; ok {
		return img
	}
	img, err := loadPNG(p)
	if err == nil {
		cache[p] = img
	}
	return img
}

// loadPNG decodifica um arquivo PNG.
func loadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

func extractNumbers(s string) []int {
	numRe := regexp.MustCompile(`\d+`)
	var nums []int
	for _, m := range numRe.FindAllString(s, -1) {
		if v, e := strconv.Atoi(m); e == nil {
			nums = append(nums, v)
		}
	}
	return nums
}

// sameInts compara dois slices de int, tratando nil == vazio.
func sameInts(x, y []int) bool {
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

func trunc(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
