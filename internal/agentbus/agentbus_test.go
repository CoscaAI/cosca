package agentbus

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/permission"
)

func allowAll() *PermissionGate {
	return NewPermissionGate(permission.Ruleset{
		{Permission: AgentMessagePermission, Pattern: "*", Action: permission.Allow},
	})
}

func denyAll() *PermissionGate {
	return NewPermissionGate(permission.Ruleset{
		{Permission: AgentMessagePermission, Pattern: "*", Action: permission.Deny},
	})
}

func testBus(gate Gate) (*Registry, *Bus) {
	reg := NewRegistry()
	reg.Register("a")
	reg.Register("b")
	return reg, NewBus(reg, gate)
}

// ─── (a) Send → queued → delivery → delivered ────────────────────────────────

func TestSend_QueuedThenDelivered(t *testing.T) {
	_, bus := testBus(allowAll())

	var received []Message
	bus.Handle("b", func(ctx context.Context, m Message) error {
		received = append(received, m)
		return nil
	})

	msg := Message{From: "a", To: "b", Mode: ModeAuto, Kind: string(KindTask), Payload: []byte("hello")}
	r, err := bus.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("Send: unexpected error %v", err)
	}
	if r.Poll() != ReceiptQueued {
		t.Fatalf("after Send, Poll=%s want %s", r.Poll(), ReceiptQueued)
	}
	if got := bus.PendingCount("b"); got != 1 {
		t.Fatalf("PendingCount(b)=%d want 1", got)
	}
	if r.From() != "a" || r.To() != "b" || r.Mode() != ModeAuto {
		t.Fatalf("receipt provenance mismatch: from=%s to=%s mode=%s", r.From(), r.To(), r.Mode())
	}
	if r.ID() == "" {
		t.Fatal("receipt id must not be empty")
	}
	if !r.DeliveredAt().IsZero() {
		t.Fatalf("DeliveredAt must be zero before delivery, got %v", r.DeliveredAt())
	}

	// Delivery transitions the receipt to delivered and hands the message over.
	delivered, ok, err := bus.Deliver(context.Background(), "b")
	if err != nil {
		t.Fatalf("Deliver: unexpected error %v", err)
	}
	if !ok {
		t.Fatal("Deliver: expected a message to be delivered")
	}
	if !reflect.DeepEqual(delivered, msg) {
		t.Fatalf("delivered message mismatch:\n got %+v\nwant %+v", delivered, msg)
	}
	if r.Poll() != ReceiptDelivered {
		t.Fatalf("after Deliver, Poll=%s want %s", r.Poll(), ReceiptDelivered)
	}
	if r.DeliveredAt().IsZero() {
		t.Fatal("DeliveredAt must be set once delivered")
	}
	if len(received) != 1 || !reflect.DeepEqual(received[0], msg) {
		t.Fatalf("consumer received %+v want the delivered message", received)
	}
	if got := bus.PendingCount("b"); got != 0 {
		t.Fatalf("PendingCount(b)=%d want 0", got)
	}
}

func TestSend_DeliverAll_Drains(t *testing.T) {
	reg, bus := testBus(allowAll())
	_ = reg
	for i := 0; i < 4; i++ {
		if _, err := bus.Send(context.Background(), Message{From: "a", To: "b", Kind: string(KindResult), Payload: []byte{byte(i)}}); err != nil {
			t.Fatalf("Send[%d]: %v", i, err)
		}
	}
	n, err := bus.DeliverAll(context.Background(), "b")
	if err != nil {
		t.Fatalf("DeliverAll: %v", err)
	}
	if n != 4 {
		t.Fatalf("DeliverAll count=%d want 4", n)
	}
	if got := bus.PendingCount("b"); got != 0 {
		t.Fatalf("PendingCount(b)=%d want 0 after drain", got)
	}
}

// ─── (b) Delivery modes: steer before auto/follow_up; follow_up queues ──────

func TestDeliveryModes_Precedence(t *testing.T) {
	_, bus := testBus(allowAll())

	var delivered []DeliveryMode
	bus.Handle("b", func(ctx context.Context, m Message) error {
		delivered = append(delivered, m.Mode)
		return nil
	})

	mustSend := func(m DeliveryMode) {
		if _, err := bus.Send(context.Background(), Message{From: "a", To: "b", Mode: m, Kind: string(KindSignal)}); err != nil {
			t.Fatalf("Send(%s): %v", m, err)
		}
	}
	mustSend(ModeAuto)
	mustSend(ModeSteer)
	mustSend(ModeFollowUp)

	// Queue snapshot in deterministic delivery order.
	qq := bus.Pending("b")
	if len(qq) != 3 {
		t.Fatalf("Pending(b)=%d want 3", len(qq))
	}
	wantOrder := []DeliveryMode{ModeSteer, ModeAuto, ModeFollowUp}
	for i, qm := range qq {
		if qm.Mode != wantOrder[i] {
			t.Fatalf("queue[%d].Mode=%s want %s (order %v)", i, qm.Mode, wantOrder[i], wantOrder)
		}
	}

	// Delivery order must be steer -> auto -> follow_up.
	bus.DeliverAll(context.Background(), "b")
	if !reflect.DeepEqual(delivered, wantOrder) {
		t.Fatalf("delivery order=%v want %v", delivered, wantOrder)
	}
}

func TestDeliveryModes_SteerInterruptsQueuedAuto(t *testing.T) {
	reg, bus := testBus(allowAll())
	_ = reg

	var delivered []DeliveryMode
	bus.Handle("b", func(ctx context.Context, m Message) error {
		delivered = append(delivered, m.Mode)
		return nil
	})

	// An auto is already queued, then a steer arrives: the steer must be handed
	// over first (interruption/head-of-queue), leaving auto behind.
	if _, err := bus.Send(context.Background(), Message{From: "a", To: "b", Mode: ModeAuto, Kind: string(KindTask)}); err != nil {
		t.Fatalf("Send(auto): %v", err)
	}
	if _, err := bus.Send(context.Background(), Message{From: "a", To: "b", Mode: ModeSteer, Kind: string(KindSignal)}); err != nil {
		t.Fatalf("Send(steer): %v", err)
	}

	bus.DeliverAll(context.Background(), "b")
	want := []DeliveryMode{ModeSteer, ModeAuto}
	if !reflect.DeepEqual(delivered, want) {
		t.Fatalf("delivery order=%v want %v", delivered, want)
	}
}

func TestDeliveryModes_FollowUpQueuesAtTail(t *testing.T) {
	reg, bus := testBus(allowAll())
	_ = reg

	if _, err := bus.Send(context.Background(), Message{From: "a", To: "b", Mode: ModeFollowUp, Kind: string(KindResult)}); err != nil {
		t.Fatalf("Send(follow_up): %v", err)
	}
	if _, err := bus.Send(context.Background(), Message{From: "a", To: "b", Mode: ModeFollowUp, Kind: string(KindResult)}); err != nil {
		t.Fatalf("Send(follow_up2): %v", err)
	}
	// follow_up messages queue sequentially at the tail (arrival order).
	qq := bus.Pending("b")
	if len(qq) != 2 {
		t.Fatalf("Pending(b)=%d want 2", len(qq))
	}
	if qq[0].Mode != ModeFollowUp || qq[1].Mode != ModeFollowUp {
		t.Fatalf("expected two follow_up queued, got %v, %v", qq[0].Mode, qq[1].Mode)
	}
}

// ─── (c) Roster register/unregister/list ─────────────────────────────────────

func TestRegistry_RegisterUnregisterList(t *testing.T) {
	reg := NewRegistry()

	if !reg.Register("alpha") {
		t.Fatal("Register(alpha): want true (new)")
	}
	if !reg.Register("beta") {
		t.Fatal("Register(beta): want true (new)")
	}
	if reg.Register("alpha") {
		t.Fatal("Register(alpha) duplicate: want false")
	}
	if reg.Register("") {
		t.Fatal("Register(empty id): want false")
	}

	if !reg.IsRegistered("alpha") {
		t.Fatal("IsRegistered(alpha): want true")
	}
	if reg.IsRegistered("gamma") {
		t.Fatal("IsRegistered(gamma): want false")
	}
	if got := reg.Count(); got != 2 {
		t.Fatalf("Count()=%d want 2", got)
	}

	// List is deterministic (sorted).
	if got := reg.List(); !reflect.DeepEqual(got, []AgentID{"alpha", "beta"}) {
		t.Fatalf("List()=%v want [alpha beta]", got)
	}

	if !reg.Unregister("alpha") {
		t.Fatal("Unregister(alpha): want true")
	}
	if reg.Unregister("alpha") {
		t.Fatal("Unregister(alpha) again: want false")
	}
	if reg.IsRegistered("alpha") {
		t.Fatal("IsRegistered(alpha) after unregister: want false")
	}
	if got := reg.Count(); got != 1 {
		t.Fatalf("Count()=%d want 1", got)
	}
	if got := reg.List(); !reflect.DeepEqual(got, []AgentID{"beta"}) {
		t.Fatalf("List()=%v want [beta]", got)
	}
}

func TestRegistry_RosterReflectsInSend(t *testing.T) {
	reg, bus := testBus(allowAll())
	_ = reg

	// Unregistered target cannot receive.
	if _, err := bus.Send(context.Background(), Message{From: "a", To: "ghost"}); !errors.Is(err, ErrUnknownTarget) {
		t.Fatalf("Send to ghost: want ErrUnknownTarget, got %v", err)
	}
	// Registering the target makes it reachable.
	bus.Registry().Register("ghost")
	if _, err := bus.Send(context.Background(), Message{From: "a", To: "ghost"}); err != nil {
		t.Fatalf("Send after register: unexpected error %v", err)
	}
}

// ─── (d) Gate-blocked message: denied receipt, never delivered ──────────────

func TestGate_BlockedMessageNeverDelivered(t *testing.T) {
	reg, bus := testBus(denyAll())
	_ = reg

	var received []Message
	bus.Handle("b", func(ctx context.Context, m Message) error {
		received = append(received, m)
		return nil
	})

	r, err := bus.Send(context.Background(), Message{From: "a", To: "b", Mode: ModeAuto, Kind: string(KindTask), Payload: []byte("probe")})
	if err == nil {
		t.Fatal("Send: want error for gate-denied message")
	}
	if !errors.Is(err, ErrDenied) {
		t.Fatalf("Send: want ErrDenied, got %v", err)
	}
	if r.Poll() != ReceiptDenied {
		t.Fatalf("blocked receipt Poll=%s want %s (never delivered)", r.Poll(), ReceiptDenied)
	}
	// A denied receipt is terminal: Notify returns immediately.
	if got := r.Notify(context.Background()); got != ReceiptDenied {
		t.Fatalf("Notify on denied receipt=%s want %s", got, ReceiptDenied)
	}
	// Nothing entered the queue and nothing was delivered.
	if got := bus.PendingCount("b"); got != 0 {
		t.Fatalf("PendingCount(b)=%d want 0 for blocked message", got)
	}
	if _, ok, err := bus.Deliver(context.Background(), "b"); err != nil || ok {
		t.Fatalf("Deliver after block: ok=%v err=%v, want false,nil", ok, err)
	}
	if len(received) != 0 {
		t.Fatalf("consumer received %d messages, want 0 (blocked)", len(received))
	}
}

func TestGate_ScopedDenyOnlyTarget(t *testing.T) {
	reg := NewRegistry()
	reg.Register("a")
	reg.Register("b")
	reg.Register("c")
	// b is denied, everything else is allowed.
	gate := NewPermissionGate(permission.Ruleset{
		{Permission: AgentMessagePermission, Pattern: "b", Action: permission.Deny},
		{Permission: AgentMessagePermission, Pattern: "*", Action: permission.Allow},
	})
	bus := NewBus(reg, gate)

	if _, err := bus.Send(context.Background(), Message{From: "a", To: "b"}); !errors.Is(err, ErrDenied) {
		t.Fatalf("Send to b (denied): want ErrDenied, got %v", err)
	}
	if _, err := bus.Send(context.Background(), Message{From: "a", To: "c"}); err != nil {
		t.Fatalf("Send to c (allowed): unexpected error %v", err)
	}
}

func TestGate_FailClosed(t *testing.T) {
	reg := NewRegistry()
	reg.Register("a")
	reg.Register("b")

	// Empty ruleset: the permission engine defaults to "ask" ⇒ deny (I2).
	busEmpty := NewBus(reg, NewPermissionGate(nil))
	if _, err := busEmpty.Send(context.Background(), Message{From: "a", To: "b"}); !errors.Is(err, ErrDenied) {
		t.Fatalf("empty ruleset: want ErrDenied (fail-closed), got %v", err)
	}

	// A nil gate on the bus denies everything (default-deny, I2).
	busNilGate := NewBus(reg, nil)
	if _, err := busNilGate.Send(context.Background(), Message{From: "a", To: "b"}); !errors.Is(err, ErrDenied) {
		t.Fatalf("nil gate: want ErrDenied (fail-closed), got %v", err)
	}
}

// ─── (e) Determinism of receipt / queue ──────────────────────────────────────

func TestReceipt_NotifyBlocksUntilDelivered(t *testing.T) {
	reg, bus := testBus(allowAll())
	_ = reg

	r, err := bus.Send(context.Background(), Message{From: "a", To: "b", Kind: string(KindTask)})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	done := make(chan ReceiptState, 1)
	go func() {
		done <- r.Notify(context.Background())
	}()

	// Notify must still be blocked (queued) shortly after send.
	select {
	case <-done:
		t.Fatal("Notify returned while message was still queued")
	case <-time.After(50 * time.Millisecond):
	}

	// Deliver → Notify unblocks with Delivered, deterministically.
	if _, ok, err := bus.Deliver(context.Background(), "b"); err != nil || !ok {
		t.Fatalf("Deliver: ok=%v err=%v", ok, err)
	}
	select {
	case state := <-done:
		if state != ReceiptDelivered {
			t.Fatalf("Notify returned %s want %s", state, ReceiptDelivered)
		}
	case <-time.After(time.Second):
		t.Fatal("Notify did not unblock after delivery")
	}
}

func TestDeterminism_SameInputSameOrder(t *testing.T) {
	inModes := []DeliveryMode{ModeFollowUp, ModeAuto, ModeSteer, ModeAuto, ModeSteer, ModeFollowUp}

	run := func() []DeliveryMode {
		reg, bus := testBus(allowAll())
		_ = reg
		var delivered []DeliveryMode
		bus.Handle("b", func(ctx context.Context, m Message) error {
			delivered = append(delivered, m.Mode)
			return nil
		})
		for i, m := range inModes {
			if _, err := bus.Send(context.Background(), Message{From: "a", To: "b", Mode: m, Kind: string(KindSignal)}); err != nil {
				t.Fatalf("Send[%d](%s): %v", i, m, err)
			}
		}
		if _, err := bus.DeliverAll(context.Background(), "b"); err != nil {
			t.Fatalf("DeliverAll: %v", err)
		}
		return delivered
	}

	first := run()
	second := run()
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("non-deterministic delivery order:\n first  %v\n second %v", first, second)
	}
	// With total precedence steer < auto < follow_up (FIFO within each): all
	// steers, then all autos, then all follow_ups.
	want := []DeliveryMode{ModeSteer, ModeSteer, ModeAuto, ModeAuto, ModeFollowUp, ModeFollowUp}
	if !reflect.DeepEqual(first, want) {
		t.Fatalf("delivery order=%v want %v", first, want)
	}
}

func TestDeterminism_ReceiptSequence(t *testing.T) {
	reg, bus := testBus(allowAll())
	_ = reg

	var ids []string
	for i := 0; i < 5; i++ {
		r, err := bus.Send(context.Background(), Message{From: "a", To: "b", Kind: string(KindTask)})
		if err != nil {
			t.Fatalf("Send[%d]: %v", i, err)
		}
		ids = append(ids, r.ID())
	}
	// Every receipt id is unique.
	seen := make(map[string]bool)
	for _, id := range ids {
		if seen[id] {
			t.Fatalf("duplicate receipt id %q", id)
		}
		seen[id] = true
	}
}

// ─── Edge / validation ───────────────────────────────────────────────────────

func TestSend_Validation(t *testing.T) {
	reg, bus := testBus(allowAll())
	_ = reg
	ctx := context.Background()

	if _, err := bus.Send(ctx, Message{To: "b"}); !errors.Is(err, ErrInvalidMsg) {
		t.Fatalf("empty From: want ErrInvalidMsg, got %v", err)
	}
	if _, err := bus.Send(ctx, Message{From: "a"}); !errors.Is(err, ErrInvalidMsg) {
		t.Fatalf("empty To: want ErrInvalidMsg, got %v", err)
	}
	if _, err := bus.Send(ctx, Message{From: "a", To: "b", Mode: "bogus"}); !errors.Is(err, ErrUnknownMode) {
		t.Fatalf("bogus mode: want ErrUnknownMode, got %v", err)
	}
}

func TestEmptyMode_NormalizedToAuto(t *testing.T) {
	reg, bus := testBus(allowAll())
	_ = reg

	r, err := bus.Send(context.Background(), Message{From: "a", To: "b"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if r.Mode() != ModeAuto {
		t.Fatalf("normalized mode=%s want %s", r.Mode(), ModeAuto)
	}
}

// ─── Thread safety (run with -race) ─────────────────────────────────────────

func TestBus_ConcurrentSendDeliver(t *testing.T) {
	reg, bus := testBus(allowAll())
	_ = reg

	const n = 200
	var mu sync.Mutex
	delivered := 0
	bus.Handle("b", func(ctx context.Context, m Message) error {
		mu.Lock()
		delivered++
		mu.Unlock()
		return nil
	})

	var wg sync.WaitGroup
	// Producers send concurrently; a single consumer drains.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			if _, err := bus.Send(context.Background(), Message{From: "a", To: "b", Kind: string(KindTask)}); err != nil {
				t.Errorf("concurrent Send: %v", err)
				return
			}
		}
	}()
	wg.Wait()

	for bus.PendingCount("b") > 0 {
		if _, ok, err := bus.Deliver(context.Background(), "b"); err != nil || !ok {
			t.Fatalf("concurrent Deliver: ok=%v err=%v", ok, err)
		}
	}
	if delivered != n {
		t.Fatalf("delivered=%d want %d", delivered, n)
	}
}

// ─── (1/7) Ordering matrix: exact total order contract ───────────────────────

// TestDeliveryModes_Matrix proves the exact total order contract for the input
// steer1, auto1, follow1, steer2, auto2, follow2. With total precedence
// steer < auto < follow_up (FIFO within each class), the delivery order must be
// exactly steer1, steer2, auto1, auto2, follow1, follow2. Indices are encoded
// in the payload so FIFO within a class is provable, not just the class order.
func TestDeliveryModes_Matrix(t *testing.T) {
	_, bus := testBus(allowAll())

	var delivered []int
	bus.Handle("b", func(ctx context.Context, m Message) error {
		delivered = append(delivered, int(m.Payload[0]))
		return nil
	})

	// Sender order, index encoded in payload: steer1(0), auto1(1), follow1(2),
	// steer2(3), auto2(4), follow2(5).
	seq := []struct {
		mode DeliveryMode
		idx  int
	}{
		{ModeSteer, 0}, {ModeAuto, 1}, {ModeFollowUp, 2},
		{ModeSteer, 3}, {ModeAuto, 4}, {ModeFollowUp, 5},
	}
	for _, s := range seq {
		if _, err := bus.Send(context.Background(), Message{From: "a", To: "b", Mode: s.mode, Kind: string(KindSignal), Payload: []byte{byte(s.idx)}}); err != nil {
			t.Fatalf("Send(idx=%d, mode=%s): %v", s.idx, s.mode, err)
		}
	}

	wantOrder := []int{0, 3, 1, 4, 2, 5}
	bus.DeliverAll(context.Background(), "b")
	if !reflect.DeepEqual(delivered, wantOrder) {
		t.Fatalf("delivery index order=%v want %v", delivered, wantOrder)
	}
}

// ─── (4) Lifecycle policy: unregister after Send ─────────────────────────────

func TestLifecycle_UnregisterAfterSendBlocksDelivery(t *testing.T) {
	reg, bus := testBus(allowAll())
	_ = reg

	var received []Message
	bus.Handle("b", func(ctx context.Context, m Message) error {
		received = append(received, m)
		return nil
	})

	r, err := bus.Send(context.Background(), Message{From: "a", To: "b", Kind: string(KindTask)})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if r.Poll() != ReceiptQueued {
		t.Fatalf("Poll=%s want queued", r.Poll())
	}

	// Target leaves the roster after the message was authorized at the boundary.
	if !bus.Registry().Unregister("b") {
		t.Fatal("Unregister(b): want true")
	}

	// The message is NOT handed over; it stays pending, receipt stays queued
	// (delivery is blocked by the roster re-check, never silently dropped).
	if _, ok, err := bus.Deliver(context.Background(), "b"); err != nil || ok {
		t.Fatalf("Deliver after unregister: ok=%v err=%v, want false,nil", ok, err)
	}
	if r.Poll() != ReceiptQueued {
		t.Fatalf("Poll=%s want queued (must not become delivered)", r.Poll())
	}
	if got := bus.PendingCount("b"); got != 1 {
		t.Fatalf("PendingCount(b)=%d want 1 (message must remain, not dropped)", got)
	}
	if len(received) != 0 {
		t.Fatalf("consumer received %d messages, want 0 (target left roster)", len(received))
	}
}

func TestLifecycle_ReregisterResumesDelivery(t *testing.T) {
	reg, bus := testBus(allowAll())
	_ = reg

	var received []Message
	bus.Handle("b", func(ctx context.Context, m Message) error {
		received = append(received, m)
		return nil
	})

	r, _ := bus.Send(context.Background(), Message{From: "a", To: "b", Kind: string(KindTask)})
	bus.Registry().Unregister("b")
	if _, ok, _ := bus.Deliver(context.Background(), "b"); ok {
		t.Fatal("Deliver while unregistered: want not delivered")
	}

	bus.Registry().Register("b") // target comes back online
	if _, ok, err := bus.Deliver(context.Background(), "b"); err != nil {
		t.Fatalf("Deliver after re-register: %v", err)
	} else if !ok {
		t.Fatal("Deliver after re-register: want delivered")
	}
	if r.Poll() != ReceiptDelivered {
		t.Fatalf("Poll=%s want delivered after re-register", r.Poll())
	}
	if len(received) != 1 {
		t.Fatalf("consumer received %d, want 1", len(received))
	}
}

// ─── (5) Deliver semantics: hand-off, not handler processing ────────────────

func TestDeliver_HandlerErrorKeepsDelivered(t *testing.T) {
	_, bus := testBus(allowAll())

	bus.Handle("b", func(ctx context.Context, m Message) error {
		return errors.New("consumer failed to process")
	})

	r, err := bus.Send(context.Background(), Message{From: "a", To: "b", Kind: string(KindTask)})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	_, ok, err := bus.Deliver(context.Background(), "b")
	if !ok {
		t.Fatal("Deliver: want ok=true (hand-off completed)")
	}
	if err == nil {
		t.Fatal("Deliver: want handler error returned separately")
	}
	// delivered = hand-off done, regardless of the handler's own failure.
	if r.Poll() != ReceiptDelivered {
		t.Fatalf("Poll=%s want delivered even though handler errored", r.Poll())
	}
}

// ─── (3) Time authority: producer provenance preserved ───────────────────────

func TestMessage_CreatedAtPreserved(t *testing.T) {
	_, bus := testBus(allowAll())

	producerTS := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	var got Message
	bus.Handle("b", func(ctx context.Context, m Message) error {
		got = m
		return nil
	})

	r, err := bus.Send(context.Background(), Message{From: "a", To: "b", Kind: string(KindTask), CreatedAt: producerTS})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	bus.Deliver(context.Background(), "b")

	// CreatedAt is producer provenance — preserved verbatim, never overwritten.
	if !got.CreatedAt.Equal(producerTS) {
		t.Fatalf("CreatedAt=%v want %v (producer provenance must be preserved)", got.CreatedAt, producerTS)
	}
	// QueuedAt is the runtime accept clock; both clocks are distinct.
	if r.QueuedAt().IsZero() {
		t.Fatal("QueuedAt (runtime accept) must be set")
	}
	if time.Since(r.QueuedAt()) > time.Minute {
		t.Fatalf("QueuedAt=%v not plausibly the runtime accept timestamp", r.QueuedAt())
	}
}

// ─── (2) Receipt id: uniqueness + monotonicity (NOT cross-run determinism) ───

func TestReceiptID_UniqueAndMonotonic(t *testing.T) {
	regA := NewRegistry()
	regA.Register("a")
	regA.Register("b")
	busA := NewBus(regA, allowAll())
	regB := NewRegistry()
	regB.Register("a")
	regB.Register("b")
	busB := NewBus(regB, allowAll())

	parseSeq := func(id string) uint64 {
		// id format: "r-<n>-<from>-<to>-<state>"; n is the global creation counter.
		parts := strings.Split(id, "-")
		n, _ := strconv.ParseUint(parts[1], 10, 64)
		return n
	}

	seen := make(map[string]bool)
	var last uint64
	for i := 0; i < 5; i++ {
		rA, _ := busA.Send(context.Background(), Message{From: "a", To: "b"})
		rB, _ := busB.Send(context.Background(), Message{From: "a", To: "b"})
		for _, id := range []string{rA.ID(), rB.ID()} {
			if seen[id] {
				t.Fatalf("duplicate receipt id %q across two independent buses", id)
			}
			seen[id] = true
		}
		// Monotonic: a new bus's id is still strictly greater than the last one
		// from the previous bus (the counter is process-global).
		sA, sB := parseSeq(rA.ID()), parseSeq(rB.ID())
		if sA <= last {
			t.Fatalf("id seq %d not > last %d (monotonicity broken)", sA, last)
		}
		last = sA
		if sB <= last {
			t.Fatalf("id seq %d not > last %d across buses (monotonicity broken)", sB, last)
		}
		last = sB
	}
}

// ─── (6) Concurrent chaos: Send/Deliver/Handle/Register/Unregister (-race) ───

func TestBus_ConcurrentChaos(t *testing.T) {
	reg := NewRegistry()
	reg.Register("src")
	bus := NewBus(reg, allowAll())

	const workers = 16
	const rounds = 40
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			to := AgentID("t" + strconv.Itoa(i))
			for r := 0; r < rounds; r++ {
				reg.Register(to)
				if _, err := bus.Send(context.Background(), Message{From: "src", To: to, Kind: string(KindSignal)}); err != nil {
					t.Errorf("Send: %v", err)
					continue
				}
				if r%3 == 0 {
					reg.Unregister(to)
				}
				bus.Deliver(context.Background(), to)
				reg.Register(to)
				bus.Handle(to, func(ctx context.Context, m Message) error { return nil })
			}
		}(w)
	}
	wg.Wait()

	// Internal consistency after concurrent mutation: count is never negative.
	if got := reg.Count(); got < 0 {
		t.Fatalf("Count()=%d want >=0", got)
	}
}
