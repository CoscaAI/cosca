// Cold-Start Contract (mineração LLRT — "cold path é uma decisão de arquitetura,
// não um acidente"). Mede o tempo de BOOT → primeiro resultado como um SLO de
// primeira classe, registrado como MEASURED (nunca FACT) — I4/I5.
//
// A LEI da casa (L319): "benchmark não altera FACT; produz MEASURED." Um
// cold-start não muda o que o Cosca É; apenas observa o que ele CUSTA para
// começar. CoV/percentis (L316): nunca declarar vitória/regressão por uma única
// execução — agregar N amostras e usar a mediana.
//
// Determinístico (I1): o veredicto contra o orçamento é função pura de um valor
// medido — ZERO LLM, ZERO julgamento de modelo.
package performance

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// =============================================================================
// Tipos do contrato de cold-start
// =============================================================================

// ColdStartSample é uma medição individual (boot → primeiro resultado).
// EpistemicClass é sempre ClassMeasured (L319).
type ColdStartSample struct {
	// Name identifica o que foi medido (ex: "cosca.version", "cosca.serve.ready").
	Name string `json:"name"`
	// ElapsedMs: tempo decorrido do boot até o primeiro resultado (ms).
	ElapsedMs float64 `json:"elapsed_ms"`
	// Epistemic: sempre ClassMeasured.
	Epistemic EpistemicClass `json:"epistemic_class"`
	// At: instante da medição.
	At time.Time `json:"at"`
	// Provenance: quem mediu (Source) e referência de evidência.
	Provenance Provenance `json:"provenance"`
}

// ColdStartSummary agrega N amostras com confiança estatística (L316).
type ColdStartSummary struct {
	Name      string         `json:"name"`
	Samples   int            `json:"samples"`
	MinMs     float64        `json:"min_ms"`
	MeanMs    float64        `json:"mean_ms"`
	MedianMs  float64        `json:"median_ms"`
	MaxMs     float64        `json:"max_ms"`
	P95Ms     float64        `json:"p95_ms"`
	CoV       float64        `json:"cov"`
	Epistemic EpistemicClass `json:"epistemic_class"`
}

// ColdStartDecision é o veredicto deterministico (I1) contra o orçamento.
type ColdStartDecision string

const (
	// ColdStartPass: mediana dentro do orçamento.
	ColdStartPass ColdStartDecision = "pass"
	// ColdStartWarn: mediana acima do orçamento, ainda abaixo de 2× (aproximando).
	ColdStartWarn ColdStartDecision = "warn"
	// ColdStartFail: mediana acima de 2× o orçamento (regressão real).
	ColdStartFail ColdStartDecision = "fail"
)

// NewColdStartSample constrói uma amostra já marcada como MEASURED.
func NewColdStartSample(name string, elapsed time.Duration, source string) ColdStartSample {
	return ColdStartSample{
		Name:      name,
		ElapsedMs: float64(elapsed.Nanoseconds()) / 1e6, // ns → ms
		Epistemic: ClassMeasured,
		At:        time.Now().UTC(),
		Provenance: Provenance{
			Source:     source,
			Timestamp:  time.Now().UTC(),
			Confidence: ConfidenceMedium,
		},
	}
}

// SummarizeColdStart agrega N amostras (mín/mediana/máx/p95/CoV). Retorna um
// summary vazio (Samples=0) se não houver amostras.
func SummarizeColdStart(samples []ColdStartSample) ColdStartSummary {
	sum := ColdStartSummary{Epistemic: ClassMeasured}
	if len(samples) == 0 {
		return sum
	}
	vals := make([]float64, 0, len(samples))
	for _, s := range samples {
		vals = append(vals, s.ElapsedMs)
	}
	sum = ColdStartSummary{
		Name:      samples[0].Name,
		Samples:   len(vals),
		MinMs:     fmin(vals),
		MeanMs:    fmean(vals),
		MedianMs:  fpercentile(vals, 0.50),
		MaxMs:     fmax(vals),
		P95Ms:     fpercentile(vals, 0.95),
		Epistemic: ClassMeasured,
	}
	if sum.MeanMs > 0 {
		sum.CoV = fstddev(vals) / sum.MeanMs
	}
	return sum
}

// EvaluateColdStart decide pass/warn/fail contra o orçamento (budgetMs) usando
// a mediana — robusta a outliers. Função pura (I1: determinístico, sem LLM).
//
//	median <= budget        -> pass
//	budget < median <= 2*budget -> warn
//	median > 2*budget       -> fail
func EvaluateColdStart(summary ColdStartSummary, budgetMs float64) ColdStartDecision {
	if summary.Samples == 0 {
		return ColdStartFail // sem evidência = fail-closed (I2)
	}
	switch {
	case summary.MedianMs <= budgetMs:
		return ColdStartPass
	case summary.MedianMs <= 2*budgetMs:
		return ColdStartWarn
	default:
		return ColdStartFail
	}
}

// =============================================================================
// ColdStartStore — memória experimental (L316): guarda medições, inclusive as
// que revelam regressão. Append-only JSONL, mesma filosofia do chain.dat.
// =============================================================================

// ColdStartStore persiste ColdStartSamples em um arquivo JSONL append-only.
type ColdStartStore struct {
	mu   sync.Mutex
	path string
}

// NewColdStartStore cria um store em `path`. Vazio/inexistente é válido.
func NewColdStartStore(path string) *ColdStartStore {
	return &ColdStartStore{path: path}
}

// Record append uma amostra ao histórico.
func (s *ColdStartStore) Record(sample ColdStartSample) error {
	if s.path == "" {
		return nil // modo volátil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("coldstart record: mkdir: %w", err)
	}
	data, err := json.Marshal(sample)
	if err != nil {
		return fmt.Errorf("coldstart record: marshal: %w", err)
	}
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("coldstart record: open: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("coldstart record: write: %w", err)
	}
	return nil
}

// LoadAll carrega o histórico (para relatórios e regressão).
func (s *ColdStartStore) LoadAll() ([]ColdStartSample, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("coldstart load: %w", err)
	}
	var samples []ColdStartSample
	for _, line := range splitLines(string(data)) {
		if line == "" {
			continue
		}
		var sample ColdStartSample
		if err := json.Unmarshal([]byte(line), &sample); err != nil {
			continue // linha corrompida: ignora (append-only resiliente)
		}
		samples = append(samples, sample)
	}
	return samples, nil
}

// =============================================================================
// Estatística float (régua para latência em ms)
// =============================================================================

func fmean(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	var sum float64
	for _, x := range v {
		sum += x
	}
	return sum / float64(len(v))
}

func fmin(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	m := v[0]
	for _, x := range v[1:] {
		if x < m {
			m = x
		}
	}
	return m
}

func fmax(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	m := v[0]
	for _, x := range v[1:] {
		if x > m {
			m = x
		}
	}
	return m
}

func fstddev(v []float64) float64 {
	if len(v) < 2 {
		return 0
	}
	m := fmean(v)
	var sumSq float64
	for _, x := range v {
		d := x - m
		sumSq += d * d
	}
	return math.Sqrt(sumSq / float64(len(v)))
}

func fpercentile(v []float64, p float64) float64 {
	if len(v) == 0 {
		return 0
	}
	if len(v) == 1 {
		return v[0]
	}
	sorted := make([]float64, len(v))
	copy(sorted, v)
	sort.Float64s(sorted)
	pos := p * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return sorted[lo]
	}
	frac := pos - float64(lo)
	return sorted[lo] + frac*(sorted[hi]-sorted[lo])
}
