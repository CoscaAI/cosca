// Package chunker — filtro de chunks triviais (L353).
//
// Filtro PREVENTIVO de comportamento-futuro: impede que chunks NOVOS sem
// conteúdo semântico entrem no corpus. Nada histórico é tocado — o filtro
// age apenas na saída do ChunkDocument.
//
// Regras (validades no dry-run L352, 0 falsos positivos estruturais):
//
//	T1 vazio            len(trim(content)) == 0
//	T2 separador puro   trim é só chars de separação (---, ***, ===, ...)
//	T3 zero alfanuméricos unicode (\p{L}\p{N}) — NUNCA por comprimento
//	T4 estrutura vazia  trim é só #, ` ou > (headings/fences/blockquotes)
//
// Nenhuma regra usa comprimento mínimo: "Go 1.26", "v1.2.3", "RAG", "API",
// "SQL", "404", "C++17" são preservados (têm alfanuméricos).
package chunker

import (
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
)

// Rule identifica qual regra T1-T4 classificou um chunk como trivial.
type Rule int

const (
	RuleNone Rule = iota
	RuleT1        // vazio
	RuleT2        // separador puro
	RuleT3        // zero alfanuméricos
	RuleT4        // estrutura vazia
)

func (r Rule) String() string {
	switch r {
	case RuleT1:
		return "T1"
	case RuleT2:
		return "T2"
	case RuleT3:
		return "T3"
	case RuleT4:
		return "T4"
	default:
		return "none"
	}
}

var (
	reT2    = regexp.MustCompile(`^[\-*_=~—–·•]+$`)
	reAlnum = regexp.MustCompile(`[\p{L}\p{N}]`)
	reT4    = regexp.MustCompile("^[#`>]+$")
)

// IsTrivial classifica um conteúdo como trivial (FILTER_CANDIDATE) pela
// primeira regra T1-T4 que dispara, na ordem de especificidade:
// T1 (vazio) → T2 (separador) → T4 (estrutura) → T3 (zero alfanuméricos).
func IsTrivial(content string) (bool, Rule) {
	t := strings.TrimSpace(content)
	switch {
	case t == "":
		return true, RuleT1
	case reT2.MatchString(t):
		return true, RuleT2
	case reT4.MatchString(t):
		return true, RuleT4
	case !reAlnum.MatchString(t):
		return true, RuleT3
	default:
		return false, RuleNone
	}
}

// TrivialStats é o snapshot dos contadores de observabilidade do filtro.
// Contém apenas CONTAGENS — nunca conteúdo dos chunks (sem dado sensível).
type TrivialStats struct {
	FilteredTotal int64            `json:"filtered_total"`
	FilteredT1    int64            `json:"filtered_t1"`
	FilteredT2    int64            `json:"filtered_t2"`
	FilteredT3    int64            `json:"filtered_t3"`
	FilteredT4    int64            `json:"filtered_t4"`
	BySource      map[string]int64 `json:"filtered_by_source"`   // section_type
	ByDocument    map[string]int64 `json:"filtered_by_document"` // document_id
}

// TrivialFilter acumula contadores atômicos + mapas protegidos por mutex.
type TrivialFilter struct {
	total  atomic.Int64
	byRule [4]atomic.Int64 // T1..T4

	mu       sync.Mutex
	bySource map[string]int64
	byDoc    map[string]int64
}

// NewTrivialFilter cria um filtro com contadores zerados.
func NewTrivialFilter() *TrivialFilter {
	return &TrivialFilter{
		bySource: make(map[string]int64),
		byDoc:    make(map[string]int64),
	}
}

// Record registra um chunk trivial filtrado (regra + origem). Apenas
// contagens — o conteúdo nunca é armazenado.
func (f *TrivialFilter) Record(rule Rule, source, documentID string) {
	if f == nil {
		return
	}
	f.total.Add(1)
	if rule >= RuleT1 && rule <= RuleT4 {
		f.byRule[rule-1].Add(1)
	}
	f.mu.Lock()
	f.bySource[source]++
	f.byDoc[documentID]++
	f.mu.Unlock()
}

// Snapshot devolve uma cópia consistente dos contadores.
func (f *TrivialFilter) Snapshot() TrivialStats {
	if f == nil {
		return TrivialStats{BySource: map[string]int64{}, ByDocument: map[string]int64{}}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	bySource := make(map[string]int64, len(f.bySource))
	for k, v := range f.bySource {
		bySource[k] = v
	}
	byDoc := make(map[string]int64, len(f.byDoc))
	for k, v := range f.byDoc {
		byDoc[k] = v
	}
	return TrivialStats{
		FilteredTotal: f.total.Load(),
		FilteredT1:    f.byRule[0].Load(),
		FilteredT2:    f.byRule[1].Load(),
		FilteredT3:    f.byRule[2].Load(),
		FilteredT4:    f.byRule[3].Load(),
		BySource:      bySource,
		ByDocument:    byDoc,
	}
}
