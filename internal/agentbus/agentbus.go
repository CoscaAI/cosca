// Package agentbus implements the ADR-017 [P6] agent↔agent message-bus: direct
// delivery between active agents with explicit delivery modes, delivery
// receipts, and an agent roster.
//
// Context. In the prime-agent mining (contract `agent_message.send` in
// `packages/coding-agent/src/core/prompts/rlm.ts` and `docs/long-running-agents.md`)
// active agents orchestrate each other directly, without a mandatory round-trip
// through the user. This package crystallizes that capability into a
// deterministic, zero-LLM runtime primitive on the Cosca boundary (never in the
// Kernel): the LLM produces the message content; the bus decides routing,
// priority, and delivery — deterministically and auditably.
//
// Invariants preserved (see docs/roadmap/evo-plano-evolutivo.md §1):
//   - I1 (deterministic governance): routing/priority/ordering are a pure
//     function of the message (mode + enqueue order) and the gate verdict. The
//     LLM never decides the access boundary nor the queue order.
//   - I2 (fail-closed): a message is only queued/delivered if the access gate
//     explicitly allows it. No verdict (ask, unknown, empty ruleset) ⇒ deny,
//     never open. The receipt is terminal in the non-`delivered` state.
//   - I7 (isolation/controlled execution): cross-agent delivery is gated at the
//     boundary, mirroring `internal/agents/subagent_permissions.go`.
//   - I5 (auditability): every receipt is uniquely identified, carries
//     provenance (From/To/Mode/CreatedAt) and a monotonic sequence, so delivery
//     can be audited. Note: receipt IDs are unique and monotonic within a
//     process, but are NOT reproducible across runs (see Receipt.ID).
//
// Scope. This bus is a runtime, in-process delivery primitive. It does NOT
// implement transport persistence or a distributed broker (that is an adapter
// concern at the edge, per ADR-017 §7). stdlib-only: no new external dependency.
package agentbus

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// AgentID is the logical identifier of an agent that can send and receive
// messages through the bus. It is the roster key and the routing target.
type AgentID string

// DeliveryMode determines how a message is placed relative to other pending
// messages in the target agent's queue.
//
// Modes carry a TOTAL precedence: steer(0) < auto(1) < follow_up(2). Within any
// single mode, ordering is FIFO by enqueue sequence. The resulting delivery
// order is therefore: all steers (FIFO), then all autos (FIFO), then all
// follow_ups (FIFO). A message never overtakes another of equal precedence.
//
//	steer      → highest precedence (priority / interruption): delivered first
//	auto       → middle precedence (route as decided)
//	follow_up  → lowest precedence (queue the response): delivered last
type DeliveryMode string

const (
	// ModeAuto routes the message as decided: it joins the target's queue at the
	// tail, after any steer messages already present. Between two non-steer
	// messages of equal precedence, arrival order wins.
	ModeAuto DeliveryMode = "auto"
	// ModeSteer gives the message priority over anything already queued for the
	// target: it jumps to the head of the queue, ahead of auto and follow_up
	// messages. A later steer still follows an earlier steer (FIFO among steer).
	ModeSteer DeliveryMode = "steer"
	// ModeFollowUp queues the message for a response: it joins the target's
	// queue at the tail, in arrival order.
	ModeFollowUp DeliveryMode = "follow_up"
)

// IsValid reports whether a delivery mode is one of the supported modes. The
// empty mode is normalized to ModeAuto by Send, so callers never need to set it.
func (m DeliveryMode) IsValid() bool {
	switch m {
	case ModeAuto, ModeSteer, ModeFollowUp:
		return true
	default:
		return false
	}
}

// ReceiptState is the delivery state of a message as observed by the sender.
//
//	queued    → the message is in the target's queue, not yet handed over
//	delivered → the message left the queue and was handed to the target
//	denied    → the message was rejected by the access gate (fail-closed, I2)
type ReceiptState string

const (
	// ReceiptQueued: the message is queued for the target, awaiting delivery.
	ReceiptQueued ReceiptState = "queued"
	// ReceiptDelivered: the message was delivered to the target agent.
	ReceiptDelivered ReceiptState = "delivered"
	// ReceiptDenied: the message was denied by the access gate. It never entered
	// the queue and will never reach the target. Terminal.
	ReceiptDenied ReceiptState = "denied"
)

// IsTerminal reports whether the receipt state is final (no further transition
// is possible). Queued is the only non-terminal state.
func (s ReceiptState) IsTerminal() bool {
	return s == ReceiptDelivered || s == ReceiptDenied
}

// Kind is the semantic content type carried by a Message. It is declared by the
// sender and is opaque to the bus — the bus never interprets content (I1/I2).
type Kind string

const (
	// KindTask is a work item / directive.
	KindTask Kind = "task"
	// KindResult is the outcome of a completed task.
	KindResult Kind = "result"
	// KindProposal is a proposed design/decision for review.
	KindProposal Kind = "proposal"
	// KindSignal is a control/coordination signal (watchdog, steer hint, etc.).
	KindSignal Kind = "signal"
)

// Message is a single agent-to-agent message. From/To/Mode/Kind are routing and
// provenance fields; Payload is the agent-produced content. The bus treats the
// payload as an opaque byte slab: the sending agent chooses both its meaning and
// its serialization (zero-LLM framing, I1/I2).
//
// Time authority (distinct clocks, never conflated):
//   - CreatedAt is the PRODUCER-declared timestamp (sender provenance, I5). The
//     bus preserves it verbatim and never overwrites it with the runtime clock.
//   - Receipt.QueuedAt is the RUNTIME clock when the bus accepted/enqueued the
//     message.
//   - Receipt.DeliveredAt is the RUNTIME clock at the hand-off.
//
// A zero CreatedAt (producer did not set one) is preserved as zero; the bus
// does not substitute its own clock in its place.
type Message struct {
	From      AgentID
	To        AgentID
	Mode      DeliveryMode
	Kind      string
	Payload   []byte
	CreatedAt time.Time
}

// Sentinel errors returned by the bus. Callers may use errors.Is.
var (
	// ErrDenied is wrapped by the gate when a message is refused at the access
	// boundary. The corresponding receipt is terminal in the denied state.
	ErrDenied = errors.New("agentbus: message denied by access gate")
	// ErrInvalidMsg is returned when a required envelope field is missing
	// (empty From or To).
	ErrInvalidMsg = errors.New("agentbus: invalid message")
	// ErrUnknownMode is returned when the delivery mode is not one of
	// auto/steer/follow_up (after empty-mode normalization).
	ErrUnknownMode = errors.New("agentbus: unknown delivery mode")
	// ErrUnknownTarget is returned when a message targets an agent that is not
	// in the roster. The bus never queues to an unknown receiver.
	ErrUnknownTarget = errors.New("agentbus: target agent not in roster")
)

// Registry is the roster of active agents — the single source of truth for
// "who can receive a message". A message can only be queued to a registered,
// active agent. Register/Unregister/List are thread-safe.
type Registry struct {
	mu     sync.RWMutex
	agents map[AgentID]time.Time // id -> registeredAt
}

// NewRegistry returns an empty roster.
func NewRegistry() *Registry {
	return &Registry{agents: make(map[AgentID]time.Time)}
}

// Register adds an agent to the roster and reports whether it was newly added.
// Registering an already-registered agent is a no-op that returns false (the
// existing registration is preserved, not re-timestamped).
func (r *Registry) Register(id AgentID) bool {
	if id == "" {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.agents[id]; ok {
		return false
	}
	r.agents[id] = time.Now().UTC()
	return true
}

// Unregister removes an agent from the roster and reports whether it was
// present. Removing an unknown agent is a no-op that returns false.
func (r *Registry) Unregister(id AgentID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.agents[id]; !ok {
		return false
	}
	delete(r.agents, id)
	return true
}

// IsRegistered reports whether the agent is present in the roster.
func (r *Registry) IsRegistered(id AgentID) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.agents[id]
	return ok
}

// List returns the ids of all registered agents in deterministic (sorted) order.
// The sorted order makes roster discovery reproducible across callers.
func (r *Registry) List() []AgentID {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]AgentID, 0, len(r.agents))
	for id := range r.agents {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Count returns the number of registered agents.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.agents)
}

// Gate decides whether a message may cross the agent→agent boundary. It is the
// I7 access front that mirrors subagent permission derivation: the sender cannot
// reach a receiver unless the gate permits it.
//
// Implementations MUST be deterministic and zero-LLM (I1). A gate with an empty
// or unknown policy MUST fail closed (I2) — return a non-nil error wrapping
// ErrDenied — never open.
type Gate interface {
	// Allow returns nil if the message may be queued/delivered, or an error
	// (wrapping ErrDenied) if it is denied.
	Allow(ctx context.Context, msg Message) error
}

// Handler is invoked when a message is delivered to its target agent. It returns
// an error only when the consumer considers the delivery a failure; the message
// is still marked delivered before the handler runs.
type Handler func(ctx context.Context, msg Message) error

// queueEntry couples a queued message with its receipt and monotonic sequence
// number. The sequence number is the deterministic tie-breaker for ordering
// among messages of equal precedence.
type queueEntry struct {
	seq     uint64
	receipt *Receipt
	msg     Message
}

// Bus is the in-process agent↔agent message bus. It owns per-target queues, the
// access gate, and per-target consumer handlers. All operations are thread-safe.
//
// Ordering contract (deterministic, I1): within a target's queue, messages are
// ordered by (precedence, seq) where precedence is steer(0) < auto(1) <
// follow_up(2) and seq increases monotonically with enqueue time.
type Bus struct {
	registry *Registry
	gate     Gate

	mu       sync.Mutex
	seq      uint64
	queues   map[AgentID][]*queueEntry
	handlers map[AgentID]Handler
}

// NewBus creates a bus bound to the given roster and boundary gate. A nil gate
// fails closed (I2): nothing is delivered.
func NewBus(reg *Registry, gate Gate) *Bus {
	return &Bus{
		registry: reg,
		gate:     gate,
		queues:   make(map[AgentID][]*queueEntry),
		handlers: make(map[AgentID]Handler),
	}
}

// Handle registers the consumer handler for an agent. Messages delivered to the
// agent are passed to it. Passing a nil handler unregisters the consumer; in
// that case Deliver still marks the message delivered (the bus guarantees the
// hand-over, not the consumer's acceptance).
func (b *Bus) Handle(id AgentID, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if h == nil {
		delete(b.handlers, id)
		return
	}
	b.handlers[id] = h
}

// Send validates, gates, and enqueues a message for its target, returning a
// deterministic receipt. On gate denial the receipt is terminal in the denied
// state and Send returns a non-nil error wrapping ErrDenied; the message never
// enters the queue and never reaches the target (I2). On success the receipt is
// in the queued state.
func (b *Bus) Send(ctx context.Context, msg Message) (*Receipt, error) {
	if msg.From == "" || msg.To == "" {
		return nil, ErrInvalidMsg
	}
	mode := msg.Mode
	if mode == "" {
		mode = ModeAuto
	}
	if !mode.IsValid() {
		return nil, ErrUnknownMode
	}
	if b.registry == nil || !b.registry.IsRegistered(msg.To) {
		return nil, ErrUnknownTarget
	}

	// Access gate on the boundary. Fail-closed: a nil gate denies everything.
	if b.gate != nil {
		if err := b.gate.Allow(ctx, msg); err != nil {
			r := newReceipt(msg, ReceiptDenied)
			return r, fmt.Errorf("%w: %s -> %s", ErrDenied, msg.From, msg.To)
		}
	} else {
		r := newReceipt(msg, ReceiptDenied)
		return r, fmt.Errorf("%w: %s -> %s (nil gate)", ErrDenied, msg.From, msg.To)
	}

	// Normalize mode on the message so the receiver observes the effective mode.
	msg.Mode = mode

	r := newReceipt(msg, ReceiptQueued)
	b.mu.Lock()
	b.enqueue(msg, r)
	b.mu.Unlock()
	return r, nil
}

// enqueue inserts a message into its target's queue, keeping the queue ordered
// by (precedence, seq). A message with lower precedence goes behind any
// higher-precedence message already queued, and behind other messages of equal
// precedence (FIFO). Called with b.mu held.
func (b *Bus) enqueue(msg Message, r *Receipt) {
	b.seq++
	entry := &queueEntry{seq: b.seq, receipt: r, msg: msg}
	b.queues[msg.To] = insertByPrecedence(b.queues[msg.To], entry)
}

// precedence maps a delivery mode to its ordering rank. Lower is delivered
// earlier. The total order (I1, deterministic) is: steer < auto < follow_up.
func precedence(m DeliveryMode) int {
	switch m {
	case ModeSteer:
		return 0
	case ModeAuto:
		return 1
	default: // ModeFollowUp
		return 2
	}
}

// insertByPrecedence inserts entry into queue keeping it sorted by
// (precedence, seq). A new entry is placed BEFORE the first existing entry of
// strictly lower precedence, and AFTER every entry of equal or higher
// precedence — preserving FIFO within a precedence class.
func insertByPrecedence(queue []*queueEntry, entry *queueEntry) []*queueEntry {
	p := precedence(entry.msg.Mode)
	idx := len(queue)
	for i, e := range queue {
		if precedence(e.msg.Mode) > p {
			idx = i
			break
		}
	}
	queue = append(queue, nil)
	copy(queue[idx+1:], queue[idx:])
	queue[idx] = entry
	return queue
}

// Deliver hands the next queued message for an agent to its consumer handler (if
// any), marking its receipt delivered. It returns the delivered message, whether
// one was delivered, and any handler error. If the queue is empty it returns
// (Message{}, false, nil).
//
// Semantics (the contract): `delivered` means the HAND-OFF completed — the
// message left the queue and reached its target's consumer — NOT that the
// handler finished processing it. The message is marked delivered BEFORE the
// handler runs, so a handler error (returned separately) never changes the
// delivery state; the receipt stays `delivered`, and the handler error is a
// separate concern for the caller.
//
// Lifecycle policy (I7): the access authorization is granted at Send time, but
// the deliver step re-checks the roster. If the target was removed from the
// roster between Send and Deliver, the message is NOT handed over and remains
// pending in the queue (receipt stays `queued`). It is never silently dropped
// (I5 auditability): the Sender can observe it via Pending/PendingCount and the
// receipt remains non-terminal. Delivery is the only path that transitions a
// receipt from Queued to Delivered; a denied receipt never reaches Deliver.
func (b *Bus) Deliver(ctx context.Context, to AgentID) (Message, bool, error) {
	b.mu.Lock()
	queue := b.queues[to]
	if len(queue) == 0 {
		b.mu.Unlock()
		return Message{}, false, nil
	}
	// Re-check the roster: an agent removed from the roster must not receive
	// further messages.
	if b.registry == nil || !b.registry.IsRegistered(to) {
		b.mu.Unlock()
		return Message{}, false, nil
	}
	entry := queue[0]
	if len(queue) == 1 {
		delete(b.queues, to)
	} else {
		b.queues[to] = queue[1:]
	}
	handler := b.handlers[to]
	b.mu.Unlock()

	// Mark delivered before invoking the consumer so the receipt is deterministic
	// even if the handler blocks or fails.
	entry.receipt.markDelivered(time.Now().UTC())

	if handler == nil {
		return entry.msg, true, nil
	}
	if err := handler(ctx, entry.msg); err != nil {
		return entry.msg, true, err
	}
	return entry.msg, true, nil
}

// DeliverAll drains the queue for an agent, delivering every pending message in
// order, and returns how many were delivered. It is a convenience for tests and
// single-consumer agents.
func (b *Bus) DeliverAll(ctx context.Context, to AgentID) (int, error) {
	n := 0
	for {
		_, ok, err := b.Deliver(ctx, to)
		if err != nil {
			return n, err
		}
		if !ok {
			return n, nil
		}
		n++
	}
}

// Pending returns the messages currently queued for an agent, in deterministic
// delivery order. It is a read-only inspection view.
func (b *Bus) Pending(to AgentID) []Message {
	b.mu.Lock()
	defer b.mu.Unlock()
	queue := b.queues[to]
	out := make([]Message, 0, len(queue))
	for _, e := range queue {
		out = append(out, e.msg)
	}
	return out
}

// PendingCount returns how many messages are queued for an agent.
func (b *Bus) PendingCount(to AgentID) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.queues[to])
}

// Registry returns the roster bound to this bus.
func (b *Bus) Registry() *Registry { return b.registry }

// Receipt is the deterministic delivery handle returned by Send. It is safe for
// concurrent use. State transitions are:
//
//	queued   → delivered   (via Deliver)
//	denied   ∞              (terminal, set at Send; never enters the queue)
//
// Poll reads the current state without blocking; Notify blocks until the state
// is terminal (delivered or denied).
type Receipt struct {
	mu          sync.RWMutex
	id          string
	from        AgentID
	to          AgentID
	mode        DeliveryMode
	kind        string
	state       ReceiptState
	queuedAt    time.Time
	deliveredAt time.Time
	done        chan struct{}
	once        sync.Once
}

// newReceipt creates a receipt in the given (already terminal or queued) state.
// For a denied receipt the done channel is closed immediately so Notify returns
// without blocking.
func newReceipt(msg Message, state ReceiptState) *Receipt {
	r := &Receipt{
		id:       newReceiptID(msg.From, msg.To, state),
		from:     msg.From,
		to:       msg.To,
		mode:     msg.Mode,
		kind:     msg.Kind,
		state:    state,
		done:     make(chan struct{}),
		queuedAt: time.Now().UTC(),
	}
	if state == ReceiptDenied {
		r.once.Do(func() { close(r.done) })
	}
	return r
}

// receiptSeq is a process-global monotonically increasing counter. It exists to
// guarantee receipt-id UNIQUENESS across every Bus instance in the process and
// to make the ids MONOTONIC in creation order. It does NOT guarantee
// reproducibility: because it is global and shared across all Buses, the exact
// numeric value of an id depends on how many receipts any Bus created before it
// in the same process (and over the lifetime of a run it never resets). Two
// identically-shaped sequences of operations in two separate runs will therefore
// produce DIFFERENT receipt ids. Queue ordering is NOT derived from this
// counter — that is the (precedence, seq) contract in Bus.
var receiptSeq uint64

// ID returns the receipt's identifier. The identifier is guaranteed to be unique
// across all Buses in the process and monotonically increasing in creation
// order (uniqueness + monotonicity). It is NOT reproducible across runs: the
// value depends on process-global creation history, not on the operation
// sequence alone.
func (r *Receipt) ID() string { return r.id }

// From returns the sending agent.
func (r *Receipt) From() AgentID { return r.from }

// To returns the receiving agent.
func (r *Receipt) To() AgentID { return r.to }

// Mode returns the effective delivery mode (already normalized to auto for the
// empty-mode case).
func (r *Receipt) Mode() DeliveryMode { return r.mode }

// Kind returns the message kind.
func (r *Receipt) Kind() string { return r.kind }

// Poll returns the current delivery state immediately, without blocking.
func (r *Receipt) Poll() ReceiptState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state
}

// Notify blocks until the receipt reaches a terminal state (delivered or
// denied) or ctx is cancelled. It returns the observed state; on cancellation
// it returns the current state (which may still be queued).
func (r *Receipt) Notify(ctx context.Context) ReceiptState {
	r.mu.RLock()
	if r.state.IsTerminal() {
		r.mu.RUnlock()
		return r.state
	}
	done := r.done
	r.mu.RUnlock()

	select {
	case <-done:
		r.mu.RLock()
		defer r.mu.RUnlock()
		return r.state
	case <-ctx.Done():
		r.mu.RLock()
		defer r.mu.RUnlock()
		return r.state
	}
}

// markDelivered transitions the receipt to delivered and closes its done channel.
func (r *Receipt) markDelivered(at time.Time) {
	r.mu.Lock()
	if r.state != ReceiptQueued {
		r.mu.Unlock()
		return
	}
	r.state = ReceiptDelivered
	r.deliveredAt = at
	r.mu.Unlock()
	r.once.Do(func() { close(r.done) })
}

// DeliveredAt returns the delivery timestamp (zero if not yet delivered).
func (r *Receipt) DeliveredAt() time.Time {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.deliveredAt
}

// newReceiptID builds a receipt id from the message's provenance, the state,
// and a process-global counter. The atomic counter guarantees uniqueness and
// monotonicity within a process; reproducibility of the id across runs is not
// guaranteed (and not part of the contract). Ordering in the queue never comes
// from this id — see (precedence, seq) in Bus.
func newReceiptID(from, to AgentID, state ReceiptState) string {
	n := atomic.AddUint64(&receiptSeq, 1)
	return fmt.Sprintf("r-%d-%s-%s-%s", n, from, to, state)
}

// QueuedAt returns the timestamp at which the message entered the queue (the
// moment the receipt was created). Zero for the zero-value Receipt.
func (r *Receipt) QueuedAt() time.Time {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.queuedAt
}
