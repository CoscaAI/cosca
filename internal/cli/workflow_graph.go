package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/workflow"
)

// NewWorkflowGraphRunCommand runs a small typed-routing workflow (Google
// ADK-Go style) through the internal/workflow engine and prints the routing
// path. It demonstrates value-based routing: nodes return Route values and
// edges decide the next node from that value — the inverse of the classic
// DependsOn planner.
func NewWorkflowGraphRunCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "graph-run",
		Short: "Run a demo typed-routing workflow",
		Long: `Run a small demo workflow on the new typed-routing engine.

Nodes return Route values ("ok", "done", ...) and edges route to the next node
based on that value, Google ADK-Go style. This contrasts with the classic
pipeline planner, which wires tasks by DependsOn IDs.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			trace, out, err := runDemoGraph(cmd.Context())
			if err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout, "Routing path:")
			for _, step := range trace {
				fmt.Fprintln(os.Stdout, "  "+step)
			}
			fmt.Fprintln(os.Stdout, "Final state:")
			keys := make([]string, 0, len(out))
			for k := range out {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(os.Stdout, "  %s: %v\n", k, out[k])
			}
			return nil
		},
	}
}

// runDemoGraph builds and runs a 3-node chain (fetch -> transform -> save)
// with a concurrent two-branch JoinNode in the middle, then returns the ordered
// routing path and the merged output state.
func runDemoGraph(ctx context.Context) ([]string, map[string]any, error) {
	var mu sync.Mutex
	var path []string
	visit := func(name string) {
		mu.Lock()
		path = append(path, name)
		mu.Unlock()
	}

	fetch := workflow.NewFunctionNode("fetch", func(_ context.Context, _ map[string]any) (workflow.Route, any, error) {
		visit("fetch")
		return workflow.StringRoute("ok"), "raw payload", nil
	})
	transform := workflow.NewFunctionNode("transform", func(_ context.Context, in map[string]any) (workflow.Route, any, error) {
		visit("transform")
		return workflow.StringRoute("done"), strings.ToUpper(fmt.Sprint(in["fetch"])), nil
	})
	save := workflow.NewFunctionNode("save", func(_ context.Context, in map[string]any) (workflow.Route, any, error) {
		visit("save")
		return nil, "saved:" + fmt.Sprint(in["transform"]), nil
	})
	workerA := workflow.NewFunctionNode("worker-a", func(_ context.Context, _ map[string]any) (workflow.Route, any, error) {
		visit("worker-a")
		return nil, "result-a", nil
	})
	workerB := workflow.NewFunctionNode("worker-b", func(_ context.Context, _ map[string]any) (workflow.Route, any, error) {
		visit("worker-b")
		return nil, "result-b", nil
	})
	join := workflow.NewJoinNode("join", []workflow.Node{workerA, workerB})
	join.Concurrent = true

	wf, err := workflow.New("demo", []workflow.Edge{
		{From: fetch, To: transform, Route: workflow.StringRoute("ok")},
		{From: transform, To: join, Route: workflow.StringRoute("done")},
		{From: join, To: save}, // nil Route = always follow after the join merges
	}, workflow.WithMaxConcurrency(2))
	if err != nil {
		return nil, nil, err
	}

	out, err := wf.Run(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	return path, out, nil
}
