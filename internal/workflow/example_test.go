package workflow

import (
	"context"
	"fmt"
	"strings"
)

// ExampleWorkflow demonstrates a 3-node typed-routing chain (fetch ->
// transform -> save) plus a JoinNode with two branches, executed end-to-end
// through the engine. The routing is value-based: fetch returns StringRoute
// "ok" and the edge decides to run transform; transform returns "done" and the
// edge decides to run the join; the join merges its two branch outputs and an
// always-follow edge runs save.
func ExampleWorkflow() {
	ctx := context.Background()

	fetch := NewFunctionNode("fetch", func(_ context.Context, _ map[string]any) (Route, any, error) {
		return StringRoute("ok"), "raw data", nil
	})
	transform := NewFunctionNode("transform", func(_ context.Context, in map[string]any) (Route, any, error) {
		return StringRoute("done"), strings.ToUpper(fmt.Sprint(in["fetch"])), nil
	})
	save := NewFunctionNode("save", func(_ context.Context, in map[string]any) (Route, any, error) {
		return nil, "saved:" + fmt.Sprint(in["transform"]), nil
	})
	workerA := NewFunctionNode("worker-a", func(_ context.Context, _ map[string]any) (Route, any, error) {
		return nil, "a", nil
	})
	workerB := NewFunctionNode("worker-b", func(_ context.Context, _ map[string]any) (Route, any, error) {
		return nil, "b", nil
	})
	join := NewJoinNode("join", []Node{workerA, workerB})

	wf, err := New("demo", []Edge{
		{From: fetch, To: transform, Route: StringRoute("ok")},
		{From: transform, To: join, Route: StringRoute("done")},
		{From: join, To: save}, // nil Route = always follow after the join merges
	})
	if err != nil {
		panic(err)
	}

	out, err := wf.Run(ctx, nil)
	if err != nil {
		panic(err)
	}

	fmt.Println("fetch:", out["fetch"])
	fmt.Println("transform:", out["transform"])
	fmt.Println("save:", out["save"])
	fmt.Println("worker-a:", out["worker-a"])
	fmt.Println("worker-b:", out["worker-b"])

	// Output:
	// fetch: raw data
	// transform: RAW DATA
	// save: saved:RAW DATA
	// worker-a: a
	// worker-b: b
}
