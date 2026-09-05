package acquisition

import (
	"fmt"
	"sync"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Acquisition Budget
// ═══════════════════════════════════════════════════════════════════════════════
//
// Regra do Don: a aquisição de evidência externa roda dentro de um orçamento
// (fontes, arquivos, rede, tempo e tokens de IA). Se não conseguir evidência
// suficiente dentro do orçamento, o sistema para de buscar e devolve:
//
//	"Não consegui validar com confiança dentro do orçamento. Preciso da sua
//	 decisão."
//
// Isso impede o sistema de entrar numa espiral: não sei → busca → não sabe →
// busca mais → contexto explode → custo explode.
//
// O AcquisitionTracker é puramente opt-in: quando Client.Tracker é nil o fetch
// roda exatamente como antes (zero mudança de comportamento).

// AcquisitionBudget define os limites de consumo de uma aquisição antes que o
// sistema esteja autorizado a continuar buscando evidência externa.
type AcquisitionBudget struct {
	// MaxSources é o teto de fontes consultadas. Default 8.
	MaxSources int `json:"max_sources"`

	// MaxFiles é o teto de arquivos adquiridos. Default 100.
	MaxFiles int `json:"max_files"`

	// MaxNetworkMB é o teto de tráfego de rede em MiB. Default 20.
	MaxNetworkMB int64 `json:"max_network_mb"`

	// MaxTime é o teto de tempo acumulado da aquisição. Default 30s.
	MaxTime time.Duration `json:"max_time"`

	// MaxAITokens é o teto de tokens de IA consumidos. Default 8000.
	MaxAITokens int `json:"max_ai_tokens"`
}

// DefaultAcquisitionBudget devolve o orçamento de aquisição padrão aprovado
// pelo Don: Fontes: 8 | Arquivos: 100 | Rede: 20MB | Tempo: 30s | IA: 8k.
func DefaultAcquisitionBudget() AcquisitionBudget {
	return AcquisitionBudget{
		MaxSources:   8,
		MaxFiles:     100,
		MaxNetworkMB: 20,
		MaxTime:      30 * time.Second,
		MaxAITokens:  8000,
	}
}

// AcquisitionSpent é a fotografia do consumo acumulado de uma aquisição.
type AcquisitionSpent struct {
	Sources  int           `json:"sources"`
	Files    int           `json:"files"`
	Bytes    int64         `json:"bytes"`
	Duration time.Duration `json:"duration"`
	AITokens int           `json:"ai_tokens"`
}

// AcquisitionTracker acumula o consumo da aquisição (fontes, arquivos, bytes,
// tempo e tokens de IA) e decide quando alguma dimensão estourou o orçamento.
type AcquisitionTracker struct {
	mu     sync.Mutex
	budget AcquisitionBudget
	spent  AcquisitionSpent
}

// NewAcquisitionTracker cria um AcquisitionTracker vinculado ao orçamento dado.
// Se um orçamento zero-valued (sem nenhuma dimensão preenchida) for passado,
// os defaults do Don são assumidos.
func NewAcquisitionTracker(budget AcquisitionBudget) *AcquisitionTracker {
	if budget.MaxSources == 0 && budget.MaxFiles == 0 && budget.MaxNetworkMB == 0 &&
		budget.MaxTime == 0 && budget.MaxAITokens == 0 {
		budget = DefaultAcquisitionBudget()
	}
	return &AcquisitionTracker{budget: budget}
}

// RecordSource contabiliza uma fonte consultada.
func (t *AcquisitionTracker) RecordSource() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.spent.Sources++
}

// RecordFile contabiliza um arquivo adquirido.
func (t *AcquisitionTracker) RecordFile() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.spent.Files++
}

// RecordBytes acumula bytes de tráfego de rede. Valores negativos são
// ignorados.
func (t *AcquisitionTracker) RecordBytes(n int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if n < 0 {
		n = 0
	}
	t.spent.Bytes += n
}

// RecordDuration acumula tempo de aquisição. Valores negativos são ignorados.
func (t *AcquisitionTracker) RecordDuration(d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if d < 0 {
		d = 0
	}
	t.spent.Duration += d
}

// RecordAITokens acumula tokens de IA consumidos. Valores negativos são
// ignorados.
func (t *AcquisitionTracker) RecordAITokens(n int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if n < 0 {
		n = 0
	}
	t.spent.AITokens += n
}

// Spent devolve uma fotografia do consumo acumulado.
func (t *AcquisitionTracker) Spent() AcquisitionSpent {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.spent
}

// Exceeded informa se alguma dimensão do orçamento foi estourada (fontes >
// Max, arquivos > Max, rede > Max, tempo > Max ou tokens de IA > Max).
func (t *AcquisitionTracker) Exceeded() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.exceededLocked()
}

// WhichExceeded devolve os nomes (pt-BR) das dimensões que estouraram o
// orçamento: "fontes", "arquivos", "rede", "tempo", "tokens IA".
func (t *AcquisitionTracker) WhichExceeded() []string {
	t.mu.Lock()
	defer t.mu.Unlock()

	var dims []string
	if t.spent.Sources > t.budget.MaxSources {
		dims = append(dims, "fontes")
	}
	if t.spent.Files > t.budget.MaxFiles {
		dims = append(dims, "arquivos")
	}
	if t.spent.Bytes > t.budget.MaxNetworkMB<<20 {
		dims = append(dims, "rede")
	}
	if t.spent.Duration > t.budget.MaxTime {
		dims = append(dims, "tempo")
	}
	if t.spent.AITokens > t.budget.MaxAITokens {
		dims = append(dims, "tokens IA")
	}
	return dims
}

// Summary renderiza o consumo da aquisição em pt-BR no formato:
// "Fontes: N/8 · Arquivos: N/100 · Rede: N/20MB · Tempo: Ns/30s · IA: N/8k".
// Quando alguma dimensão estoura, acrescenta o aviso de confiança do Don.
func (t *AcquisitionTracker) Summary() string {
	s := t.Spent()

	summary := fmt.Sprintf("Fontes: %d/%d · Arquivos: %d/%d · Rede: %d/%dMB · Tempo: %ds/%ds · IA: %d/%dk",
		s.Sources, t.budget.MaxSources,
		s.Files, t.budget.MaxFiles,
		s.Bytes>>20, t.budget.MaxNetworkMB,
		int(s.Duration.Seconds()), int(t.budget.MaxTime.Seconds()),
		s.AITokens/1000, t.budget.MaxAITokens/1000)

	if t.Exceeded() {
		summary += "\nNão consegui validar com confiança dentro do orçamento. Preciso da sua decisão."
	}
	return summary
}

func (t *AcquisitionTracker) exceededLocked() bool {
	return t.spent.Sources > t.budget.MaxSources ||
		t.spent.Files > t.budget.MaxFiles ||
		t.spent.Bytes > t.budget.MaxNetworkMB<<20 ||
		t.spent.Duration > t.budget.MaxTime ||
		t.spent.AITokens > t.budget.MaxAITokens
}
