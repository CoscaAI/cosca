// Package procgen — Procedural Kernel System (ordem do Professor, L303).
//
// Fase do roadmap Creative/Scientific/Media: kernels procedurais ANTES de
// Scene Graph e Visual Compiler. Cada família (Noise/Math/Pattern) é composta
// de funções puras determinísticas + nós do Node Graph (nodegraph.Executor).
// Desde o nascimento, todo nó registra metadados de performance (OpMetric) —
// o embrião do Performance Memory.
package procgen

// RNG é um PRNG determinístico (splitmix64).
//
// Determinismo P1: mesmo seed = mesma sequência, em qualquer máquina
// (splitmix64 é puramente inteiro, sem float — portável byte a byte).
// Nenhum estado além de um uint64; nenhuma dependência de plataforma.
type RNG struct {
	state uint64
}

// NewRNG cria um RNG a partir de um seed. O bit pattern do int64 é usado
// diretamente como estado inicial — determinístico e portável.
func NewRNG(seed int64) *RNG {
	return &RNG{state: uint64(seed)}
}

// splitmix64 é o passo de mistura puramente inteiro (portátil byte a byte).
func splitmix64(x uint64) uint64 {
	x += 0x9E3779B97F4A7C15
	z := x
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// Uint64 devolve o próximo valor de 64 bits e avança o estado.
func (r *RNG) Uint64() uint64 {
	r.state = splitmix64(r.state)
	return r.state
}

// Float64 devolve um float em [0,1) usando 53 bits (mesma técnica do
// math/rand — isento de viés significativo, só inteiro).
func (r *RNG) Float64() float64 {
	return float64(r.Uint64()>>11) * (1.0 / (1 << 53))
}

// Intn devolve um int em [0,n). Para n <= 0 devolve 0 (fail-closed).
func (r *RNG) Intn(n int) int {
	if n <= 0 {
		return 0
	}
	return int(r.Uint64() % uint64(n))
}

// Range devolve um float em [min,max] (uniforme).
func (r *RNG) Range(min, max float64) float64 {
	if max < min {
		min, max = max, min
	}
	return min + r.Float64()*(max-min)
}
