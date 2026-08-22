//
// Tests for `cosca trace causal <id> [--baseline <id>]` (internal/cli/trace.go).
//
// Covers:
//   - Causal chain output (numbered, arrows "caused")
//   - Divergence output with --baseline (SUSPICIOUS DIVERGENCE)
//   - --json output (graph nodes + edges + divergence)
//   - Empty trace and invalid baseline handling
//
// NOTE: tests that chdir() must NOT run in parallel — they mutate the
// process working directory.

package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/trace"
)

// seedTraceEvents registra uma sequência de eventos em um trace via CLI.
func seedTraceEvents(t *testing.T, root, id string, evs []struct {
	action string
	actor  string
	result string
}) {
	t.Helper()
	for _, ev := range evs {
		args := []string{"event", id, "--action", ev.action, "--actor", ev.actor}
		if ev.result != "" {
			args = append(args, "--result", ev.result)
		}
		if _, err := executeTrace(t, root, args...); err != nil {
			t.Fatalf("trace event: %v", err)
		}
	}
}

func TestTraceCausal_ChainOutput(t *testing.T) {
	root := fakeTraceTree(t)
	id := trace.NewID().String()
	seedTraceEvents(t, root, id, []struct {
		action string
		actor  string
		result string
	}{
		{"PLAN_CREATED", "kernel", "success"},
		{"AGENT_STARTED", "agent-x", "running"},
		{"TEST_FAILED", "agent-x", "failed"},
		{"RETRY", "agent-x", "running"},
		{"TEST_PASSED", "agent-x", "success"},
	})

	out, err := executeTrace(t, root, "causal", id)
	if err != nil {
		t.Fatalf("trace causal: %v", err)
	}
	if !strings.Contains(out, "CAUSAL GRAPH "+id) {
		t.Errorf("output sem header do grafo causal: %q", out)
	}
	if !strings.Contains(out, "Eventos") {
		t.Errorf("output sem contagem de eventos: %q", out)
	}
	// Cadeia numerada com setas de causa.
	for _, action := range []string{"PLAN_CREATED", "AGENT_STARTED", "TEST_FAILED", "RETRY", "TEST_PASSED"} {
		if !strings.Contains(out, action) {
			t.Errorf("cadeia causal sem %q: %q", action, out)
		}
	}
	if !strings.Contains(out, "1. PLAN_CREATED") {
		t.Errorf("cadeia sem numeração: %q", out)
	}
	if !strings.Contains(out, "caused") {
		t.Errorf("cadeia sem seta causa 'caused': %q", out)
	}
	// Sem baseline → sem divergência.
	if strings.Contains(out, "SUSPICIOUS DIVERGENCE") {
		t.Errorf("sem baseline não deveria marcar divergência: %q", out)
	}
}

func TestTraceCausal_DivergenceWithBaseline(t *testing.T) {
	root := fakeTraceTree(t)
	idA := trace.NewID().String()
	idB := trace.NewID().String()

	// Baseline: PLAN_CREATED → AGENT_STARTED → TEST_PASSED.
	seedTraceEvents(t, root, idA, []struct {
		action string
		actor  string
		result string
	}{
		{"PLAN_CREATED", "kernel", "success"},
		{"AGENT_STARTED", "agent-x", "running"},
		{"TEST_PASSED", "agent-x", "success"},
	})
	// Atual: PLAN_CREATED → AGENT_STARTED → TEST_FAILED → RETRY → TEST_PASSED.
	seedTraceEvents(t, root, idB, []struct {
		action string
		actor  string
		result string
	}{
		{"PLAN_CREATED", "kernel", "success"},
		{"AGENT_STARTED", "agent-x", "running"},
		{"TEST_FAILED", "agent-x", "failed"},
		{"RETRY", "agent-x", "running"},
		{"TEST_PASSED", "agent-x", "success"},
	})

	out, err := executeTrace(t, root, "causal", idB, "--baseline", idA)
	if err != nil {
		t.Fatalf("trace causal --baseline: %v", err)
	}
	if !strings.Contains(out, "Baseline") {
		t.Errorf("output sem baseline: %q", out)
	}
	if !strings.Contains(out, "SUSPICIOUS DIVERGENCE") {
		t.Errorf("output sem marcador SUSPICIOUS DIVERGENCE: %q", out)
	}
	// Divergência na posição 3 (TEST_FAILED ≠ TEST_PASSED) — 0-based 2.
	if !strings.Contains(out, "posição 3") {
		t.Errorf("output sem a posição da divergência: %q", out)
	}
	if !strings.Contains(out, "TEST_PASSED") || !strings.Contains(out, "TEST_FAILED") {
		t.Errorf("output sem as ações divergentes: %q", out)
	}
}

func TestTraceCausal_NoDivergenceWithBaseline(t *testing.T) {
	root := fakeTraceTree(t)
	idA := trace.NewID().String()
	idB := trace.NewID().String()
	seq := []struct {
		action string
		actor  string
		result string
	}{
		{"PLAN_CREATED", "kernel", "success"},
		{"AGENT_STARTED", "agent-x", "running"},
		{"TEST_PASSED", "agent-x", "success"},
	}
	seedTraceEvents(t, root, idA, seq)
	seedTraceEvents(t, root, idB, seq)

	out, err := executeTrace(t, root, "causal", idB, "--baseline", idA)
	if err != nil {
		t.Fatalf("trace causal --baseline: %v", err)
	}
	if strings.Contains(out, "SUSPICIOUS DIVERGENCE") {
		t.Errorf("sequências idênticas não deveriam divergir: %q", out)
	}
	if !strings.Contains(out, "Sem divergência") {
		t.Errorf("output sem mensagem de sem-divergência: %q", out)
	}
}

func TestTraceCausal_JSON(t *testing.T) {
	root := fakeTraceTree(t)
	idA := trace.NewID().String()
	idB := trace.NewID().String()
	seedTraceEvents(t, root, idA, []struct {
		action string
		actor  string
		result string
	}{
		{"PLAN_CREATED", "kernel", "success"},
		{"TEST_PASSED", "agent-x", "success"},
	})
	seedTraceEvents(t, root, idB, []struct {
		action string
		actor  string
		result string
	}{
		{"PLAN_CREATED", "kernel", "success"},
		{"TEST_FAILED", "agent-x", "failed"},
	})

	out, err := executeTrace(t, root, "causal", idB, "--baseline", idA, "--json")
	if err != nil {
		t.Fatalf("trace causal --json: %v", err)
	}
	var res struct {
		TraceID    string             `json:"trace_id"`
		Nodes      []trace.CausalNode `json:"nodes"`
		Edges      []trace.CausalEdge `json:"edges"`
		Baseline   string             `json:"baseline"`
		Divergence []int              `json:"divergence"`
		Suspicious bool               `json:"suspicious"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("JSON inválido: %v (output: %q)", err, out)
	}
	if res.TraceID != idB {
		t.Errorf("trace_id = %q, esperava %q", res.TraceID, idB)
	}
	if len(res.Nodes) != 2 {
		t.Errorf("nodes = %d, esperava 2", len(res.Nodes))
	}
	if len(res.Edges) != 1 || res.Edges[0].Type != "caused" {
		t.Errorf("edges inesperadas: %+v", res.Edges)
	}
	if res.Baseline != idA {
		t.Errorf("baseline = %q, esperava %q", res.Baseline, idA)
	}
	if len(res.Divergence) != 1 || res.Divergence[0] != 1 {
		t.Errorf("divergence = %v, esperava [1]", res.Divergence)
	}
	if !res.Suspicious {
		t.Errorf("suspicious deveria ser true")
	}
}

func TestTraceCausal_EmptyTrace(t *testing.T) {
	root := fakeTraceTree(t)
	id := trace.NewID().String()

	out, err := executeTrace(t, root, "causal", id)
	if err != nil {
		t.Fatalf("trace causal (empty): %v", err)
	}
	if !strings.Contains(out, "sem eventos") {
		t.Errorf("output sem aviso de trace vazio: %q", out)
	}
}

func TestTraceCausal_InvalidBaseline(t *testing.T) {
	root := fakeTraceTree(t)
	id := trace.NewID().String()
	seedTraceEvents(t, root, id, []struct {
		action string
		actor  string
		result string
	}{
		{"PLAN_CREATED", "kernel", "success"},
	})

	// Baseline com Trace ID inválido.
	if _, err := executeTrace(t, root, "causal", id, "--baseline", "NAO-TRACE"); err == nil {
		t.Error("causal com baseline inválido deveria falhar")
	}
	// Baseline = o próprio trace.
	if _, err := executeTrace(t, root, "causal", id, "--baseline", id); err == nil {
		t.Error("causal com baseline = o próprio trace deveria falhar")
	}
}
