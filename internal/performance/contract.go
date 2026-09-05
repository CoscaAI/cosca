// Package performance implementa o COSCA BENCHMARK CONTRACT (F9 do Hardware &
// Performance Brain — L319, L316, L317, L320).
//
// A LEI (L319): "Benchmark não altera FACT. Benchmark produz MEASURED."
// Se o FACT diz 31 GB RAM e o benchmark mede 37 GB/s (e amanhã 42), a RAM não
// virou 42 GB/s — o MEASURED mudou. Esta separação é o que evita corrupção
// epistemológica.
//
// O CONTRATO (L316) — o agente/executor não inventa métricas a cada rodada:
//
//	INPUT      → workload + dataset + hardware + restrições
//	EXPERIMENT → UMA variável por vez
//	OUTPUT     → latência, throughput, bandwidth, allocations, variance, confidence
//	DECISION   → KEEP / REJECT / INCONCLUSIVE
//
// A INVARIANTE (L320): uma etapa não fabrica evidência para a anterior.
// Um MEASURED não vira FACT por declaração — só por EXECUÇÃO OBSERVADA.
// Experimentos REJEITADOS são guardados (memória experimental, L316): um
// futuro "vamos pré-calcular normas!" encontra o registro REJECTED e não
// gasta tokens redescobrindo o que já foi refutado com dado.
package performance

import (
	"time"
)

// =============================================================================
// Classes epistemológicas (L317) — o que cada medição É
// =============================================================================

// EpistemicClass classifica a origem epistemológica de um valor.
type EpistemicClass string

const (
	// ClassFact: o sistema/hardware DECLAROU (ex: CPU = Ryzen, RAM = 31 GB).
	ClassFact EpistemicClass = "fact"
	// ClassMeasured: o benchmark OBSERVOU (ex: 12.4 Mvec/s, ~38 GB/s).
	ClassMeasured EpistemicClass = "measured"
	// ClassInferred: o sistema CONCLUIU (ex: provável memory-bound).
	ClassInferred EpistemicClass = "inferred"
	// ClassEvidence: prova registrada (benchmark id, commit, fingerprint).
	ClassEvidence EpistemicClass = "evidence"
	// ClassProfile: perfil consolidado (hardware + workload + evidência).
	ClassProfile EpistemicClass = "profile"
	// ClassDecision: decisão tomada (com status superseded quando contradita).
	ClassDecision EpistemicClass = "decision"
)

// Confidence classifica a confiança de uma medição (L316).
type Confidence string

const (
	ConfidenceHigh        Confidence = "high"
	ConfidenceMedium      Confidence = "medium"
	ConfidenceLow         Confidence = "low"
	ConfidenceInconclusive Confidence = "inconclusive"
)

// =============================================================================
// O CONTRATO (L316)
// =============================================================================

// WorkloadProfile descreve o que será medido (INPUT do contrato).
type WorkloadProfile struct {
	// Name identifica o workload (ex: "vector-search").
	Name string `json:"name" yaml:"name"`
	// Dimension (ex: 768), DatasetSize (ex: 100000), Limit (ex: 10).
	Dimension   int `json:"dimension,omitempty" yaml:"dimension,omitempty"`
	DatasetSize int `json:"dataset_size,omitempty" yaml:"dataset_size,omitempty"`
	Limit       int `json:"limit,omitempty" yaml:"limit,omitempty"`
	// Precision (ex: "float32"), Backend (ex: "cpu-go").
	Precision string `json:"precision,omitempty" yaml:"precision,omitempty"`
	Backend   string `json:"backend,omitempty" yaml:"backend,omitempty"`
	// HardwareFingerprint identifica o ambiente (L315) — correlaciona histórico.
	HardwareFingerprint string `json:"hardware_fingerprint,omitempty" yaml:"hardware_fingerprint,omitempty"`
	// Environment (ex: "jail", "bare-metal") — contexto de execução (F4).
	Environment string `json:"environment,omitempty" yaml:"environment,omitempty"`
}

// Experiment descreve UMA variável a testar (L316: uma mudança por vez).
type Experiment struct {
	// Name da variável (ex: "workers", "block_size", "kernel").
	Name string `json:"name" yaml:"name"`
	// Value testado (ex: 8, 64, "unroll4").
	Value string `json:"value" yaml:"value"`
	// Description documenta a hipótese (L316: cada alteração precisa de hipótese).
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// Hypothesis registra o que se espera observar (refutação também é vitória).
	Hypothesis string `json:"hypothesis,omitempty" yaml:"hypothesis,omitempty"`
}

// Measurement é o OUTPUT do contrato — a observação de UMA execução.
type Measurement struct {
	// LatencyNs: tempo total por operação (ns).
	LatencyNs int64 `json:"latency_ns" yaml:"latency_ns"`
	// ThroughputPerSec: operações por segundo (ex: vectors/sec).
	ThroughputPerSec float64 `json:"throughput_per_sec" yaml:"throughput_per_sec"`
	// BandwidthBytesPerSec: bytes movimentados por segundo (Mvec/s × bytes/vec).
	BandwidthBytesPerSec float64 `json:"bandwidth_bytes_per_sec,omitempty" yaml:"bandwidth_bytes_per_sec,omitempty"`
	// AllocBytes e AllocOps: alocações da execução (B/op e allocs/op).
	AllocBytes int64 `json:"alloc_bytes,omitempty" yaml:"alloc_bytes,omitempty"`
	AllocOps   int64 `json:"alloc_ops,omitempty" yaml:"alloc_ops,omitempty"`
	// Samples: número de execuções usadas nesta medição.
	Samples int `json:"samples" yaml:"samples"`
}

// Stats agrega múltiplas medições com confiança estatística (L316 — nunca
// declarar "recorde" por uma única execução).
type Stats struct {
	MeanNs     int64   `json:"mean_ns" yaml:"mean_ns"`
	MinNs      int64   `json:"min_ns" yaml:"min_ns"`
	MaxNs      int64   `json:"max_ns" yaml:"max_ns"`
	StdDevNs   float64 `json:"stddev_ns" yaml:"stddev_ns"`
	// CoV = stddev/mean — coeficiente de variação (ruído relativo).
	CoV        float64 `json:"cov" yaml:"cov"`
	// P50Ns, P95Ns, P99Ns: percentis de latência.
	P50Ns      int64   `json:"p50_ns" yaml:"p50_ns"`
	P95Ns      int64   `json:"p95_ns" yaml:"p95_ns"`
	P99Ns      int64   `json:"p99_ns" yaml:"p99_ns"`
	// MeanThroughputPerSec e MeanBandwidthBytesPerSec: agregados.
	MeanThroughputPerSec    float64 `json:"mean_throughput_per_sec" yaml:"mean_throughput_per_sec"`
	MeanBandwidthBytesPerSec float64 `json:"mean_bandwidth_bytes_per_sec,omitempty" yaml:"mean_bandwidth_bytes_per_sec,omitempty"`
	// MeanAllocBytes e MeanAllocOps: médias de alocação.
	MeanAllocBytes int64 `json:"mean_alloc_bytes,omitempty" yaml:"mean_alloc_bytes,omitempty"`
	MeanAllocOps   int64 `json:"mean_alloc_ops,omitempty" yaml:"mean_alloc_ops,omitempty"`
	// Samples: número de execuções.
	Samples int `json:"samples" yaml:"samples"`
}

// Decision é o veredicto do contrato (L316).
type Decision string

const (
	DecisionKeep        Decision = "keep"
	DecisionReject      Decision = "reject"
	DecisionInconclusive Decision = "inconclusive"
)

// ExperimentResult é o resultado COMPLETO de um experimento — guardado mesmo
// quando rejeitado (memória experimental, L316).
type ExperimentResult struct {
	// Workload que foi medido.
	Workload WorkloadProfile `json:"workload" yaml:"workload"`
	// Experiment que foi testado.
	Experiment Experiment `json:"experiment" yaml:"experiment"`
	// BaselineStats: a referência (mesmo workload, configuração padrão).
	BaselineStats Stats `json:"baseline_stats" yaml:"baseline_stats"`
	// ResultStats: a medição do candidato.
	ResultStats Stats `json:"result_stats" yaml:"result_stats"`
	// Speedup: result/baseline (1.0 = igual, <1 = pior, >1 = melhor).
	Speedup float64 `json:"speedup" yaml:"speedup"`
	// DeltaPct: variação percentual vs baseline (−14.5 = 14.5% pior).
	DeltaPct float64 `json:"delta_pct" yaml:"delta_pct"`
	// Decision: KEEP / REJECT / INCONCLUSIVE.
	Decision Decision `json:"decision" yaml:"decision"`
	// Confidence da decisão.
	Confidence Confidence `json:"confidence" yaml:"confidence"`
	// Reason: por que a decisão foi tomada (ex: "overhead de acesso supera
	// benefício", "melhoria dentro do ruído").
	Reason string `json:"reason,omitempty" yaml:"reason,omitempty"`
	// EpistemicClass do resultado: sempre ClassMeasured (nunca vira FACT).
	EpistemicClass EpistemicClass `json:"epistemic_class" yaml:"epistemic_class"`
	// Provenance (L318): de onde veio, quando, com que confiança.
	Provenance Provenance `json:"provenance" yaml:"provenance"`
}

// Provenance registra a origem da medição (L318 — F5 GPU style).
type Provenance struct {
	// Source: quem gerou (ex: "cosca bench vector-search", "autopsy-2").
	Source string `json:"source" yaml:"source"`
	// Timestamp: quando foi medido.
	Timestamp time.Time `json:"timestamp" yaml:"timestamp"`
	// Evidence: referência à evidência (benchmark id, commit, arquivo).
	Evidence string `json:"evidence,omitempty" yaml:"evidence,omitempty"`
	// Confidence: confiança da medição.
	Confidence Confidence `json:"confidence" yaml:"confidence"`
}

// =============================================================================
// O Runner — executa com disciplina (L316: uma variável, múltiplas amostras)
// =============================================================================

// BenchFunc é a função que executa UMA operação do workload (ex: uma busca
// vetorial). Deve ser pura de efeitos colaterais relevantes à medição.
type BenchFunc func() (Measurement, error)

// Runner executa um benchmark com N amostras e produz Stats + decisão.
// O baseline é a referência; o candidato é o experimento (uma variável).
type Runner struct {
	// Samples: número de execuções por medição (default 5 — L316).
	Samples int
	// MinDifferencePct: diferença mínima (% ) para declarar KEEP/REJECT;
	// abaixo disso = INCONCLUSIVE (ruído).
	MinDifferencePct float64
}

// NewRunner cria um runner com defaults conservadores (L316: nunca declarar
// vitória por diferença pequena).
func NewRunner() *Runner {
	return &Runner{
		Samples:           5,
		MinDifferencePct:  2.0, // <2% = ruído (INCONCLUSIVE)
	}
}

// Run mede o baseline e o candidato e decide KEEP/REJECT/INCONCLUSIVE.
func (r *Runner) Run(workload WorkloadProfile, exp Experiment, baselineFn, candidateFn BenchFunc, source string) (*ExperimentResult, error) {
	baseline, err := r.measure(baselineFn)
	if err != nil {
		return nil, err
	}
	candidate, err := r.measure(candidateFn)
	if err != nil {
		return nil, err
	}

	res := &ExperimentResult{
		Workload:       workload,
		Experiment:     exp,
		BaselineStats:  baseline,
		ResultStats:    candidate,
		EpistemicClass: ClassMeasured,
		Provenance: Provenance{
			Source:     source,
			Timestamp:  time.Now().UTC(),
			Confidence: confidenceFor(baseline, candidate),
		},
	}

	// Speedup e delta (usando a mediana de latência — robusta a outliers).
	baseMed := float64(baseline.P50Ns)
	candMed := float64(candidate.P50Ns)
	if baseMed > 0 {
		res.Speedup = baseMed / candMed
		res.DeltaPct = (candMed - baseMed) / baseMed * 100.0
	}

	// Decisão (L316: só KEEP/REJECT com diferença consistente; senão INCONCLUSIVE).
	// PARA LATÊNCIA, MENOR É MELHOR: delta negativo = candidato mais rápido = KEEP;
	// delta positivo = candidato mais lento = REJECT. Speedup = base/cand (inverso).
	abs := res.DeltaPct
	if abs < 0 {
		abs = -abs
	}
	switch {
	case abs < r.MinDifferencePct:
		res.Decision = DecisionInconclusive
		res.Reason = "melhoria dentro do ruído (diferença < " + formatPct(r.MinDifferencePct) + ")"
	case res.DeltaPct < 0:
		res.Decision = DecisionKeep
		res.Reason = "candidato mais rápido que o baseline (latência menor)"
	case res.DeltaPct > 0:
		res.Decision = DecisionReject
		res.Reason = "candidato mais lento que o baseline (overhead supera benefício)"
	}
	res.Confidence = res.Provenance.Confidence

	return res, nil
}

// measure executa a função N vezes e agrega em Stats (com percentis).
func (r *Runner) measure(fn BenchFunc) (Stats, error) {
	n := r.Samples
	if n <= 0 {
		n = 5
	}
	lats := make([]int64, 0, n)
	var sumThroughput, sumBandwidth float64
	var sumAllocBytes, sumAllocOps int64

	for i := 0; i < n; i++ {
		m, err := fn()
		if err != nil {
			return Stats{}, err
		}
		lats = append(lats, m.LatencyNs)
		sumThroughput += m.ThroughputPerSec
		sumBandwidth += m.BandwidthBytesPerSec
		sumAllocBytes += m.AllocBytes
		sumAllocOps += m.AllocOps
	}

	stats := Stats{
		Samples:                 n,
		MeanNs:                  mean(lats),
		MinNs:                   min(lats),
		MaxNs:                   max(lats),
		StdDevNs:                stddev(lats),
		MeanThroughputPerSec:    sumThroughput / float64(n),
		MeanBandwidthBytesPerSec: sumBandwidth / float64(n),
		MeanAllocBytes:          sumAllocBytes / int64(n),
		MeanAllocOps:            sumAllocOps / int64(n),
		P50Ns:                   percentile(lats, 0.50),
		P95Ns:                   percentile(lats, 0.95),
		P99Ns:                   percentile(lats, 0.99),
	}
	if stats.MeanNs > 0 {
		stats.CoV = stats.StdDevNs / float64(stats.MeanNs)
	}
	return stats, nil
}

// confidenceFor classifica a confiança pela variância relativa (L316):
// CoV baixo + amostras suficientes = high; senão medium/low.
func confidenceFor(base, cand Stats) Confidence {
	// Usa o pior CoV entre baseline e candidato.
	cov := base.CoV
	if cand.CoV > cov {
		cov = cand.CoV
	}
	switch {
	case cov < 0.05:
		return ConfidenceHigh
	case cov < 0.15:
		return ConfidenceMedium
	default:
		return ConfidenceLow
	}
}

func formatPct(p float64) string {
	// "2%" sem dependência extra de formatação.
	if p == float64(int64(p)) {
		return itoa(int64(p)) + "%"
	}
	return itoa(int64(p*10)/10) + "%"
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
