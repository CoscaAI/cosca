package trace

import (
	"net/http"
	"regexp"
	"testing"
)

func TestNewSpanID(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{16}$`)
	seen := map[SpanID]bool{}
	for i := 0; i < 100; i++ {
		id := NewSpanID()
		if !re.MatchString(string(id)) {
			t.Fatalf("span id = %q, want 16-char hex", id)
		}
		if seen[id] {
			t.Fatalf("duplicate span id: %q", id)
		}
		seen[id] = true
	}
}

func TestTraceparentString(t *testing.T) {
	tp := Traceparent{Version: "00", TraceID: "0af7651916cd43dd8448eb211c80319c", ParentID: "b7ad6b7169203331", TraceFlags: "01"}
	want := "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
	if tp.String() != want {
		t.Fatalf("String = %q, want %q", tp.String(), want)
	}
}

func TestNewTraceparentFromCoscaID(t *testing.T) {
	tp := NewTraceparent("TRACE-20260814-ABCDEF01")
	if tp.Version != "00" || tp.TraceFlags != TraceFlagsSampled {
		t.Fatalf("tp: %+v", tp)
	}
	if len(tp.TraceID) != 32 || !isHex(tp.TraceID) {
		t.Fatalf("trace id = %q", tp.TraceID)
	}
	if len(tp.ParentID) != 16 {
		t.Fatalf("parent id = %q", tp.ParentID)
	}
	// Determinístico: mesmo Cosca ID → mesmo TraceID.
	tp2 := NewTraceparent("TRACE-20260814-ABCDEF01")
	if tp.TraceID != tp2.TraceID {
		t.Fatal("coscaToHex must be deterministic")
	}
	// Diferente Cosca ID → diferente TraceID.
	tp3 := NewTraceparent("TRACE-20260814-ABCDEF02")
	if tp.TraceID == tp3.TraceID {
		t.Fatal("different cosca IDs must produce different trace IDs")
	}
}

func TestParseTraceparent(t *testing.T) {
	valid := "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
	tp, ok := ParseTraceparent(valid)
	if !ok {
		t.Fatal("valid traceparent must parse")
	}
	if tp.TraceID != "0af7651916cd43dd8448eb211c80319c" || tp.ParentID != "b7ad6b7169203331" {
		t.Fatalf("parsed: %+v", tp)
	}
	// Round-trip: parse(String) == original.
	rt, ok := ParseTraceparent(tp.String())
	if !ok || rt.String() != tp.String() {
		t.Fatalf("round-trip failed: %+v vs %+v", rt, tp)
	}

	invalid := []string{
		"",               // vazio
		"01-0000...",     // versão errada
		"00-abc-0000-01", // trace id curto
		"00-00000000000000000000000000000000-xyz-01",              // parent não-hex
		"00-00000000000000000000000000000000-0000000000000000-02", // flags inválidas
		"00-0000-0000-0000-00-extra",                              // 5 partes
	}
	for _, v := range invalid {
		if _, ok := ParseTraceparent(v); ok {
			t.Errorf("invalid traceparent %q parsed as valid", v)
		}
	}
}

func TestNewChildSpanAndPropagateStep(t *testing.T) {
	parent := NewTraceparent("TRACE-20260814-ABCDEF01")
	child := parent.NewChildSpan()
	if child.TraceID != parent.TraceID {
		t.Fatal("child must share the trace ID")
	}
	if child.ParentID == parent.ParentID {
		t.Fatal("child must have a NEW parent span ID")
	}
	if child.Version != parent.Version || child.TraceFlags != parent.TraceFlags {
		t.Fatal("child must inherit version and flags")
	}

	propChild, spanID := PropagateStep(parent)
	if propChild.TraceID != parent.TraceID {
		t.Fatal("propagated child must share trace ID")
	}
	if spanID != propChild.ParentID {
		t.Fatal("PropagateStep must return the child's span ID")
	}
}

func TestInjectAndExtract(t *testing.T) {
	tp := NewTraceparent("TRACE-20260814-ABCDEF01")

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	tp.Inject(req)

	// traceparent + X-Cosca-Trace headers.
	if got := req.Header.Get(TraceparentHeader); got != tp.String() {
		t.Fatalf("traceparent header = %q, want %q", got, tp.String())
	}
	if got := req.Header.Get("X-Cosca-Trace"); got != string(tp.TraceID) {
		t.Fatalf("X-Cosca-Trace = %q", got)
	}

	extracted, ok := ExtractTraceparent(req)
	if !ok || extracted.String() != tp.String() {
		t.Fatalf("extract = %+v, %v", extracted, ok)
	}

	// Request sem header → não extrai.
	empty, _ := http.NewRequest("GET", "http://example.com", nil)
	if _, ok := ExtractTraceparent(empty); ok {
		t.Fatal("request without traceparent must not extract")
	}
}

func TestCoscaToHexStableAndValid(t *testing.T) {
	id := TraceID("TRACE-20260814-ABCDEF01")
	h1 := coscaToHex(id)
	h2 := coscaToHex(id)
	if h1 != h2 || len(h1) != 32 || !isHex(h1) {
		t.Fatalf("coscaToHex: %q vs %q", h1, h2)
	}
}

func TestIsHex(t *testing.T) {
	if !isHex("0123456789abcdefABCDEF") {
		t.Fatal("valid hex rejected")
	}
	if isHex("0123g") || isHex("") {
		t.Fatal("invalid hex accepted")
	}
}
