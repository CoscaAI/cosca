package agentbridge

import (
	"testing"
	"time"
)

// TestSessionLifecycle_CreateThroughDestroy exercita o ciclo de vida completo:
// create → startTurn → endTurn → detach (com coords) → resume → stop → destroy.
func TestSessionLifecycle_CreateThroughDestroy(t *testing.T) {
	s := NewStore()
	sess := s.Create("ses_001", "opencode", "deepseek-v4-flash", "deepseek")
	if sess.Status != StatusCreated {
		t.Fatalf("expected created, got %s", sess.Status)
	}

	if err := s.StartTurn("ses_001"); err != nil {
		t.Fatalf("StartTurn: %v", err)
	}
	if err := s.EndTurn("ses_001"); err != nil {
		t.Fatalf("EndTurn: %v", err)
	}
	got, _ := s.Get("ses_001")
	if got.Status != StatusIdle {
		t.Fatalf("expected idle after turn, got %s", got.Status)
	}

	// Detach com coordenadas de resume.
	coords := &ResumeCoords{BridgePort: 43210, BridgeToken: "tok-123", LastSeenEvent: 42, SandboxID: "sbx-9"}
	if err := s.Detach("ses_001", coords); err != nil {
		t.Fatalf("Detach: %v", err)
	}
	got, _ = s.Get("ses_001")
	if got.Status != StatusSuspended {
		t.Fatalf("expected suspended, got %s", got.Status)
	}
	if got.Resume == nil || got.Resume.BridgePort != 43210 || got.Resume.LastSeenEvent != 42 {
		t.Fatalf("resume coords not persisted: %+v", got.Resume)
	}

	if err := s.Resume("ses_001"); err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if err := s.Stop("ses_001"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if err := s.Destroy("ses_001"); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
	if _, err := s.Get("ses_001"); err != ErrSessionNotFound {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
}

// TestSessionLifecycle_InvalidTransitions verifica que transições inválidas
// são rejeitadas (o trinômio deve ser disciplinado).
func TestSessionLifecycle_InvalidTransitions(t *testing.T) {
	s := NewStore()
	s.Create("ses_x", "opencode", "gpt-5.4", "openai")

	// Detach de um sessão "created" — permitido (é o fluxo normal).
	if err := s.Detach("ses_x", &ResumeCoords{BridgePort: 1}); err != nil {
		t.Fatalf("detach from created should be allowed: %v", err)
	}
	// Detach de uma sessão já suspensa — inválido.
	if err := s.Detach("ses_x", &ResumeCoords{BridgePort: 2}); err != ErrInvalidTransition {
		t.Fatalf("expected invalid transition, got %v", err)
	}
	// EndTurn de uma sessão não em turno — inválido.
	if err := s.EndTurn("ses_x"); err != ErrInvalidTransition {
		t.Fatalf("expected invalid transition, got %v", err)
	}
}

// TestSessionStore_NotFound verifica erros limpos para sessões ausentes.
func TestSessionStore_NotFound(t *testing.T) {
	s := NewStore()
	if _, err := s.Get("nao-existe"); err != ErrSessionNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
	if err := s.Destroy("nao-existe"); err != ErrSessionNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

// TestSessionStore_ListOrdering verifica que List retorna do mais novo
// para o mais antigo.
func TestSessionStore_ListOrdering(t *testing.T) {
	s := NewStore()
	s.Create("a", "opencode", "m1", "p1")
	time.Sleep(2 * time.Millisecond)
	s.Create("b", "claude-code", "m2", "p2")
	list := s.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(list))
	}
	if list[0].ID != "b" || list[1].ID != "a" {
		t.Fatalf("expected newest first, got %s, %s", list[0].ID, list[1].ID)
	}
}

// TestEventStore_AppendAndSince verifica o stream de eventos e o resume
// incremental (seq > after).
func TestEventStore_AppendAndSince(t *testing.T) {
	es := NewEventStore(100)
	e1 := es.Append("ses_a", EventBridgeHello, "handshake OK", 1)
	e2 := es.Append("ses_a", EventToolCall, "glob **/*.go", 2)
	e3 := es.Append("ses_a", EventToolResult, "214 arquivos", 3)

	if e1.Seq != 1 || e2.Seq != 2 || e3.Seq != 3 {
		t.Fatalf("seq not assigned in order: %d %d %d", e1.Seq, e2.Seq, e3.Seq)
	}
	all := es.ForSession("ses_a")
	if len(all) != 3 {
		t.Fatalf("expected 3 events, got %d", len(all))
	}
	inc := es.Since("ses_a", 2)
	if len(inc) != 1 || inc[0].Type != EventToolResult {
		t.Fatalf("Since(2) should return only event 3, got %+v", inc)
	}
}

// TestEventStore_BoundedCapacity verifica que o stream não cresce sem limite.
func TestEventStore_BoundedCapacity(t *testing.T) {
	es := NewEventStore(5)
	for i := int64(1); i <= 10; i++ {
		es.Append("ses_b", EventTextDelta, "x", i)
	}
	stream := es.ForSession("ses_b")
	if len(stream) != 5 {
		t.Fatalf("expected bounded to 5, got %d", len(stream))
	}
	if stream[0].Seq != 6 {
		t.Fatalf("oldest kept event should be seq 6, got %d", stream[0].Seq)
	}
}

// TestBridgeMonitor_LifecycleAndMask verifica o monitor e a máscara do token.
func TestBridgeMonitor_LifecycleAndMask(t *testing.T) {
	bm := NewBridgeMonitor()
	if st := bm.State(); st.State != BridgeOffline {
		t.Fatalf("expected offline, got %s", st.State)
	}

	bm.MarkStarting()
	bm.MarkOnline(4281, 43210, "s3cretToken123", "bwrap", "/tmp/event-log.ndjson")
	bm.SetLastSeenEvent(182)
	bm.SetResumeStrategy(ResumeRerun)

	st := bm.State()
	if st.State != BridgeOnline || st.WSPort != 43210 || st.LastSeenEvent != 182 {
		t.Fatalf("state wrong: %+v", st)
	}
	if st.ChannelToken == "s3cretToken123" {
		t.Fatal("token must be masked in output")
	}
	if len(st.ChannelToken) < 4 {
		t.Fatalf("masked token too short: %q", st.ChannelToken)
	}

	bm.MarkStopping()
	bm.MarkOffline()
	if st := bm.State(); st.State != BridgeOffline {
		t.Fatalf("expected offline after stop, got %s", st.State)
	}
}

// TestMaskToken verifica a máscara para tokens vazios/curtos.
func TestMaskToken(t *testing.T) {
	if maskToken("") != "" {
		t.Fatal("empty token should stay empty")
	}
	if maskToken("abc") != "••••" {
		t.Fatalf("short token should be fully masked, got %q", maskToken("abc"))
	}
	m := maskToken("0123456789abcdef")
	if m[:4] != "0123" || len(m) < 8 {
		t.Fatalf("masked token wrong: %q", m)
	}
}

// TestSessionStore_ConcurrentAccess verifica que o store é seguro sob
// concorrência (goroutines paralelas).
func TestSessionStore_ConcurrentAccess(t *testing.T) {
	s := NewStore()
	done := make(chan bool)
	for i := 0; i < 20; i++ {
		go func(n int) {
			id := "s"
			for j := 0; j < 20; j++ {
				_ = s.Create(id, "opencode", "m", "p")
				_ = s.StartTurn(id)
				_ = s.EndTurn(id)
				_ = s.Detach(id, &ResumeCoords{BridgePort: n})
				_, _ = s.Get(id)
			}
			done <- true
		}(i)
	}
	for i := 0; i < 20; i++ {
		<-done
	}
}
