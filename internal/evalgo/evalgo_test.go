package evalgo

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestContainsCriterion_PassFail(t *testing.T) {
	ok, sig := ContainsCriterion("good").Check(nil, "this is a good result")
	if !ok {
		t.Fatalf("deveria passar, got ok=%v sig=%q", ok, sig)
	}
	ok, sig = ContainsCriterion("good").Check(nil, "bad result")
	if ok {
		t.Fatalf("deveria falhar, got ok=%v", ok)
	}
	if sig != "missing:good" {
		t.Fatalf("assinatura inesperada: %q", sig)
	}
}

func TestExcludeCriterion_SecurityI8(t *testing.T) {
	c := ExcludeCriterion("SECRET_123")
	ok, _ := c.Check(nil, "all good")
	if !ok {
		t.Fatal("sem segredo deveria passar")
	}
	ok, sig := c.Check(nil, "here is SECRET_123 leaked")
	if ok {
		t.Fatal("vazamento de segredo deveria falhar")
	}
	if !strings.HasPrefix(sig, "leaked:") {
		t.Fatalf("assinatura inesperada: %q", sig)
	}
}

func TestClusters_Deterministic(t *testing.T) {
	results := []Result{
		{CaseID: "a", Kind: "safety", Pass: false, Signature: "leaked:KEY"},
		{CaseID: "b", Kind: "safety", Pass: false, Signature: "leaked:KEY"},
		{CaseID: "c", Kind: "tool", Pass: false, Signature: "missing:ok"},
		{CaseID: "d", Kind: "tool", Pass: true, Signature: ""},
	}
	clusters := Clusters(results)
	if len(clusters) != 2 {
		t.Fatalf("esperava 2 clusters, got %d", len(clusters))
	}
	// Ordenado por Count desc (leaked:KEY tem 2, missing:ok tem 1).
	if clusters[0].Signature != "leaked:KEY" || clusters[0].Count != 2 {
		t.Fatalf("cluster primário inesperado: %+v", clusters[0])
	}
	if clusters[1].Signature != "missing:ok" || clusters[1].Count != 1 {
		t.Fatalf("cluster secundário inesperado: %+v", clusters[1])
	}
	if len(clusters[0].Cases) != 2 || clusters[0].Cases[0] != "a" || clusters[0].Cases[1] != "b" {
		t.Fatalf("cases do cluster fora de ordem: %v", clusters[0].Cases)
	}
}

func TestGate_Promote(t *testing.T) {
	g := NewGate(WithMinPassRate(0.8), WithMinCases(3))
	res := []Result{
		{CaseID: "a", Pass: true}, {CaseID: "b", Pass: true}, {CaseID: "c", Pass: true}, {CaseID: "d", Pass: true},
	}
	if !g.Decide(res).Promoted {
		t.Fatal("4/4 aprovados deveria promover")
	}
	// Taxa exatamente no limiar: 4/5 = 0.8 → promove (>=, não >).
	res2 := []Result{
		{CaseID: "a", Pass: true}, {CaseID: "b", Pass: true}, {CaseID: "c", Pass: true}, {CaseID: "d", Pass: true},
		{CaseID: "e", Pass: false, Signature: "minor"},
	}
	if !g.Decide(res2).Promoted {
		t.Fatal("4/5 = 0.8 no limiar deveria promover")
	}
}

func TestGate_BlocksOnLowRate(t *testing.T) {
	g := NewGate(WithMinPassRate(0.8), WithMinCases(3))
	res := []Result{
		{CaseID: "a", Pass: true}, {CaseID: "b", Pass: false, Signature: "x"}, {CaseID: "c", Pass: false, Signature: "y"},
	}
	if g.Decide(res).Promoted {
		t.Fatal("taxa baixa (1/3) nao deveria promover")
	}
}

func TestGate_BlocksOnTooFewCases(t *testing.T) {
	g := NewGate(WithMinPassRate(0.8), WithMinCases(3))
	res := []Result{{CaseID: "a", Pass: true}}
	if g.Decide(res).Promoted {
		t.Fatal("casos insuficientes nao deveria promover")
	}
}

func TestGate_BlocksOnBlockingSignature(t *testing.T) {
	// Fail-closed I2: vazamento de segredo NUNCA promove, mesmo com taxa alta.
	g := NewGate(WithMinPassRate(0.8), WithMinCases(3),
		WithBlockingSignatures("leaked:"))
	res := []Result{
		{CaseID: "a", Pass: true}, {CaseID: "b", Pass: true}, {CaseID: "c", Pass: true},
		{CaseID: "d", Pass: false, Signature: "leaked:KEY"},
	}
	if g.Decide(res).Promoted {
		t.Fatal("assinatura bloqueante nao deveria permitir promoção")
	}
}

func TestGate_Empty(t *testing.T) {
	if NewGate().Decide(nil).Promoted {
		t.Fatal("sem casos nao deveria promover")
	}
}

func TestFlywheel_Run(t *testing.T) {
	ctx := context.Background()
	cases := []Case{
		{ID: "c1", Kind: "safety", Input: "q", Criterion: ExcludeCriterion("SECRET")},
		{ID: "c2", Kind: "tool", Input: "q", Criterion: ContainsCriterion("ok")},
		{ID: "c3", Kind: "safety", Input: "q", Criterion: ExcludeCriterion("SECRET")},
	}
	producer := func(_ context.Context, _ any) (any, error) {
		return "ok and fine", nil
	}
	f := NewFlywheel(NewGate())
	rep, err := f.Run(ctx, cases, producer)
	if err != nil {
		t.Fatalf("Run erro: %v", err)
	}
	if rep.Total != 3 || rep.Passed != 3 || rep.PassRate != 1.0 {
		t.Fatalf("report inesperado: %+v", rep)
	}
}

func TestFlywheel_ProducerErrorFailsCase(t *testing.T) {
	ctx := context.Background()
	cases := []Case{
		{ID: "boom", Kind: "tool", Input: "q", Criterion: ContainsCriterion("ok")},
	}
	producer := func(_ context.Context, _ any) (any, error) {
		return nil, errors.New("boom")
	}
	rep, err := NewFlywheel(NewGate()).Run(ctx, cases, producer)
	if err != nil {
		t.Fatalf("Run erro: %v", err)
	}
	if rep.Passed != 0 {
		t.Fatal("falha do produtor deveria marcar o caso como falho")
	}
	if len(rep.Clusters) != 1 || rep.Clusters[0].Signature != "producer_error" {
		t.Fatalf("cluster de produtor inesperado: %+v", rep.Clusters)
	}
}

func TestFlywheel_NilCriterionError(t *testing.T) {
	cases := []Case{{ID: "x", Input: "q"}}
	_, err := NewFlywheel(NewGate()).Run(context.Background(), cases, func(_ context.Context, _ any) (any, error) { return nil, nil })
	if err == nil {
		t.Fatal("caso sem critério (espelho) deveria dar erro")
	}
}
