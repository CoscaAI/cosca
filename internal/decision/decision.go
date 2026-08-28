// Package decision é a memória de DECISÕES do Cognitive Control Plane — uma
// primitiva GENÉRICA event-sourced (append-only) que registrará o "porquê" das
// ações de orquestração: iniciar/parar uma operação, bypass de um gate,
// ajuste de risco, escalada ao Don e o veredito do Task Continuation Loop
// (continue/complete/abort).
//
// Este pacote é NEUTRO DE DOMÍNIO e stdlib-only: ele NÃO importa `internal/`
// de domínio — essas são fronteiras do ADAPTER. A persistência é um contrato
// (`DecisionStore`) cuja implementação concreta vive na borda do domínio e é
// injetada via `NewDecisionLog`. O cosca-trader foi o campo de prova; a
// primitiva pertence ao Root (ADR-015).
//
// Distinção vs `internal/audit`: `audit.DecisionStore` é a trilha de
// EXPLICABILIDADE (por que fiz isso + evidencias + laws + provider/model,
// SQLite, IDs D-0001). Este pacote é a memória event-sourced do CONTROL PLANE
// (latest-winner: active → superseded | redacted), histórico imutável de
// decisões operacionais. São conceitos complementares — coexistem, não se
// sobrepõem.
//
// Modelo event-sourced: cada operação (`Append`/`Supersede`/`Redact`) grava um
// EVENTO imutável no log. O estado efetivo de uma decisão é DERIVADO do log
// (latest-winner): o último evento de uma decisão determina seu `Status`
// (active → superseded | redacted). Assim o log é a fonte de verdade e nunca é
// mutado — auditoria completa e re-hidratável por replay.
package decision

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// Status de uma decisão (derivado do log — latest-winner).
const (
	StatusActive     = "active"     // decisão corrente/em vigor
	StatusSuperseded = "superseded" // substituída por uma decisão mais recente
	StatusRedacted   = "redacted"   // suprimida do rastro (auditoria)
)

// Op é a operação que gerou um evento do log (append-only).
const (
	OpAppend    = "append"    // criou a decisão (active)
	OpSupersede = "supersede" // marca a decisão como superseded
	OpRedact    = "redact"    // marca a decisão como redacted
)

// Kinds conhecidos de decisão do control plane (documentação — a primitiva é
// genérica e aceita qualquer Kind; estes são os usados pelo adapter).
const (
	KindOwmBypass     = "owm_bypass"     // gate de consciência pulou uma entrada
	KindStartStrategy = "start_strategy" // operação ativada
	KindStopStrategy  = "stop_strategy"  // operação desativada/substituída
	KindRiskAdjust    = "risk_adjust"    // ajuste de tamanho/risco por confiança
	KindEscalate      = "escalate"       // escalada ao Don (teto/ watchdog)
	KindContinue      = "continue"       // Task Continuation Loop decidiu CONTINUE
	KindComplete      = "complete"       // Task Continuation Loop decidiu COMPLETE
	KindAbort         = "abort"          // Task Continuation Loop decidiu ABORT
)

// Decision é uma decisão registrada do control plane. `Context` (chave/valor) e
// `Evidence` (lista de evidências) capturam o estado real no momento da decisão;
// `Rationale` é o porquê legível. `Status` é derivado do log (latest-winner):
// uma decisão appended nasce `active`; supersede/redact a alteram.
type Decision struct {
	ID        string            `json:"id"`
	TaskID    string            `json:"task_id,omitempty"` // tarefa associada (quando aplicável)
	Kind      string            `json:"kind"`
	Actor     string            `json:"actor"` // quem decidiu (ex.: "orchestrator:owm")
	Context   map[string]string `json:"context,omitempty"`
	Evidence  []string          `json:"evidence,omitempty"`
	Rationale string            `json:"rationale"`
	Status    string            `json:"status"` // active | superseded | redacted
	Timestamp time.Time         `json:"timestamp"`
}

// DecisionEvent é um evento IMUTÁVEL do log (append-only). Referencia a decisão
// que afeta (`DecisionID`) e a operação (`Op`); para `append`, carrega o
// conteúdo da decisão em `Decision`.
type DecisionEvent struct {
	ID         string    `json:"id"`          // ID único do evento
	DecisionID string    `json:"decision_id"` // a decisão que este evento afeta
	Op         string    `json:"op"`          // append | supersede | redact
	Decision   Decision  `json:"decision"`    // conteúdo (para append)
	Timestamp  time.Time `json:"timestamp"`
}

// Erros do contrato de decisões.
var (
	ErrUnknownDecision = errors.New("decisão desconhecida")
	ErrDuplicateID     = errors.New("decisão com ID duplicado")
)

// DecisionStore é a fronteira de persistência do DecisionLog. É stdlib-only e
// NÃO conhece o domínio. A implementação concreta é um ADAPTER (ex.: um
// backend apoiado em `internal/durable`, ou um arquivo JSONL append-only),
// injetada via `NewDecisionLog`; nil = em memória (fail-safe).
type DecisionStore interface {
	// AppendEvent grava um evento imutável (append-only, idempotente pelo ID).
	AppendEvent(ev DecisionEvent) error
	// Events devolve todos os eventos na ordem de append (replay/derivação).
	Events() ([]DecisionEvent, error)
	// Save persiste uma decisão diretamente (atalho: monta o evento de append).
	Save(d Decision) error
	// ByTask devolve as decisões efetivas de uma tarefa (derivadas).
	ByTask(taskID string) ([]Decision, error)
	// List devolve todas as decisões efetivas (derivadas do log).
	List() ([]Decision, error)
}

// DecisionLog é o log event-sourced de decisões (thread-safe). Mantém o estado
// efetivo (derivado) em memória e o persiste via `DecisionStore` (fora do lock).
type DecisionLog struct {
	mu    sync.RWMutex
	store DecisionStore

	// state deriva a visão latest-winner: decisionID → decisão efetiva.
	state map[string]*Decision
	// order preserva a ordem de criação das decisões (para List/Latest/ByTask).
	order []string
}

// NewDecisionLog cria o log. `store` nil = em memória (fail-safe: opera sem
// persistir, como o TaskOrchestrator em F1). Quando store != nil, re-hidrata o
// estado efetivo re-executando os eventos persistidos (replay).
func NewDecisionLog(store DecisionStore) *DecisionLog {
	l := &DecisionLog{
		store: store,
		state: map[string]*Decision{},
	}
	l.replay()
	return l
}

// replay re-hidrata o estado efetivo a partir dos eventos persistidos. Falha do
// store degrada para em memória (fail-safe, não crash); o estado corrente segue
// consistente a partir daqui.
func (l *DecisionLog) replay() {
	if l.store == nil {
		return
	}
	evs, err := l.store.Events()
	if err != nil {
		log.Printf("decision: carga do store falhou (%v) — seguindo em memória (fail-safe)", err)
		return
	}
	for _, ev := range evs {
		l.apply(ev)
	}
}

// Append registra uma nova decisão (nasce `active`). Gera o ID se ausente; um
// ID já registrado → ErrDuplicateID (append-only: a decisão é criada uma vez;
// para substituí-la, crie outra e `Supersede` a anterior). Persiste FORA do
// lock (lição F3: nunca I/O sob o mutex). Devolve o ID da decisão.
func (l *DecisionLog) Append(d Decision) (string, error) {
	l.mu.Lock()
	if d.ID != "" {
		if _, exists := l.state[d.ID]; exists {
			l.mu.Unlock()
			return "", errors.Join(ErrDuplicateID, fmt.Errorf("decisão %q já registrada", d.ID))
		}
	}
	ev := NewAppendEvent(d)
	l.apply(ev)
	id := ev.DecisionID
	l.mu.Unlock()
	l.persist(ev)
	return id, nil
}

// Supersede marca uma decisão como `superseded` (substituída por uma mais
// recente). A decisão continua no log (auditoria); apenas o estado derivado
// muda. Erro se a decisão não existe.
func (l *DecisionLog) Supersede(id string) error {
	ev := DecisionEvent{ID: newID(), DecisionID: id, Op: OpSupersede, Timestamp: time.Now().UTC()}
	l.mu.Lock()
	if _, ok := l.state[id]; !ok {
		l.mu.Unlock()
		return errors.Join(ErrUnknownDecision, fmt.Errorf("decisão %q não existe", id))
	}
	l.apply(ev)
	l.mu.Unlock()
	l.persist(ev)
	return nil
}

// Redact marca uma decisão como `redacted` (suprimida do rastro — ex.: foi
// enganosa). A decisão continua no log; apenas o estado derivado muda. Erro se
// a decisão não existe.
func (l *DecisionLog) Redact(id string) error {
	ev := DecisionEvent{ID: newID(), DecisionID: id, Op: OpRedact, Timestamp: time.Now().UTC()}
	l.mu.Lock()
	if _, ok := l.state[id]; !ok {
		l.mu.Unlock()
		return errors.Join(ErrUnknownDecision, fmt.Errorf("decisão %q não existe", id))
	}
	l.apply(ev)
	l.mu.Unlock()
	l.persist(ev)
	return nil
}

// ByTask devolve as decisões efetivas de uma tarefa (em ordem de criação).
func (l *DecisionLog) ByTask(taskID string) []Decision {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]Decision, 0)
	for _, id := range l.order {
		if d, ok := l.state[id]; ok && d.TaskID == taskID {
			out = append(out, cloneDecision(d))
		}
	}
	return out
}

// Latest devolve a decisão mais recente (por ordem de criação) de um `kind`,
// com seu estado efetivo corrente. `false` se nenhuma existe. É útil para
// consultar "qual foi a última decisão X" (ex.: último owm_bypass).
func (l *DecisionLog) Latest(kind string) (Decision, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	for i := len(l.order) - 1; i >= 0; i-- {
		if d, ok := l.state[l.order[i]]; ok && d.Kind == kind {
			return cloneDecision(d), true
		}
	}
	return Decision{}, false
}

// List devolve todas as decisões efetivas (em ordem de criação).
func (l *DecisionLog) List() []Decision {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]Decision, 0, len(l.order))
	for _, id := range l.order {
		if d, ok := l.state[id]; ok {
			out = append(out, cloneDecision(d))
		}
	}
	return out
}

// Count devolve o número de decisões efetivas rastreadas.
func (l *DecisionLog) Count() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.order)
}

// ── helpers ─────────────────────────────────────────────────────────────────

// apply aplica um evento ao estado efetivo (latest-winner). Usado tanto ao vivo
// quanto no replay. Assume l.mu já garantido com ESCRITA.
func (l *DecisionLog) apply(ev DecisionEvent) {
	switch ev.Op {
	case OpAppend:
		d := ev.Decision
		if d.ID == "" {
			d.ID = ev.DecisionID
		}
		if _, ok := l.state[d.ID]; !ok {
			l.order = append(l.order, d.ID)
		}
		d.Status = StatusActive
		l.state[d.ID] = &d
	case OpSupersede:
		if d, ok := l.state[ev.DecisionID]; ok {
			d.Status = StatusSuperseded
			l.state[ev.DecisionID] = d // d já é ponteiro — estado mutado no lugar
		}
	case OpRedact:
		if d, ok := l.state[ev.DecisionID]; ok {
			d.Status = StatusRedacted
			l.state[ev.DecisionID] = d
		}
	}
}

// persist grava o evento no store FORA do lock (lição F3: nunca I/O sob o
// mutex). Falha de persistência NÃO derruba o sistema — o estado segue em
// memória (fail-safe) e o erro é logado.
func (l *DecisionLog) persist(ev DecisionEvent) {
	if l.store == nil {
		return
	}
	if err := l.store.AppendEvent(ev); err != nil {
		log.Printf("decision: persistir evento falhou (%v) — mantendo em memória (fail-safe)", err)
	}
}

// cloneDecision devolve uma cópia independente (mapa e slice copiados) — leitura
// segura sem o lock.
func cloneDecision(d *Decision) Decision {
	c := *d
	if d.Context != nil {
		c.Context = make(map[string]string, len(d.Context))
		for k, v := range d.Context {
			c.Context[k] = v
		}
	}
	c.Evidence = append([]string(nil), d.Evidence...)
	return c
}

// NewAppendEvent monta o evento de append de uma decisão (gera IDs se
// ausentes, timezone UTC, status active). É o atalho usado pelos stores para
// o método `Save`.
func NewAppendEvent(d Decision) DecisionEvent {
	if d.ID == "" {
		d.ID = newID()
	}
	if d.Timestamp.IsZero() {
		d.Timestamp = time.Now().UTC()
	}
	d.Status = StatusActive
	return DecisionEvent{
		ID:         newID(),
		DecisionID: d.ID,
		Op:         OpAppend,
		Decision:   d,
		Timestamp:  d.Timestamp,
	}
}

// Derive é a função pura de derivação (latest-winner): a partir de uma lista de
// eventos em ordem de append, devolve as decisões efetivas. É usada pelos
// stores para materializar ByTask/List sem duplicar a lógica do DecisionLog.
func Derive(events []DecisionEvent) []Decision {
	eff := map[string]Decision{}
	var order []string
	for _, ev := range events {
		switch ev.Op {
		case OpAppend:
			d := ev.Decision
			if d.ID == "" {
				d.ID = ev.DecisionID
			}
			if _, ok := eff[d.ID]; !ok {
				order = append(order, d.ID)
			}
			d.Status = StatusActive
			eff[d.ID] = d
		case OpSupersede:
			if d, ok := eff[ev.DecisionID]; ok {
				d.Status = StatusSuperseded
				eff[ev.DecisionID] = d
			}
		case OpRedact:
			if d, ok := eff[ev.DecisionID]; ok {
				d.Status = StatusRedacted
				eff[ev.DecisionID] = d
			}
		}
	}
	out := make([]Decision, 0, len(order))
	for _, id := range order {
		out = append(out, eff[id])
	}
	return out
}

// newID gera um ID único (16 bytes aleatórios → hex). stdlib only (crypto/rand);
// se a fonte de aleatoriedade falhar (praticamente impossível), cai num ID
// monotônico por timestamp+counter para nunca colidir.
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return fmt.Sprintf("%x-%d", time.Now().UnixNano(), atomic.AddUint64(&idCounter, 1))
}

var idCounter uint64
