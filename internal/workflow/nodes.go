package workflow

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Func is the execution contract shared by FunctionNode, ToolNode and
// AgentNode. It receives the merged input state so far and returns a Route
// value (compared against outgoing edges), an output value, and an error.
// Returning a map[string]any output merges those keys into the workflow state;
// any other output is stored under the node's Name().
type Func func(ctx context.Context, input map[string]any) (Route, any, error)

// nodeBase supplies the Name() method for concrete node types. The name is
// unexported so it cannot collide with the Node interface method; use the
// New* constructors to create nodes.
type nodeBase struct {
	name string
}

// Name implements Node.
func (b *nodeBase) Name() string { return b.name }

// FunctionNode runs a Go function.
type FunctionNode struct {
	nodeBase
	Fn Func
}

// NewFunctionNode builds a FunctionNode with the given name and function.
func NewFunctionNode(name string, fn Func) *FunctionNode {
	return &FunctionNode{nodeBase: nodeBase{name: name}, Fn: fn}
}

// Run implements Node.
func (n *FunctionNode) Run(ctx context.Context, input map[string]any) (Route, map[string]any, error) {
	return runCore(ctx, n.Name(), n.Fn, input)
}

// ToolNode delegates to a tool call. It is intentionally func-based so the
// workflow engine stays decoupled from internal/chat (no import cycle). Use a
// constructor or an adapter to bridge the real chat.Tool.Execute surface.
type ToolNode struct {
	nodeBase
	Tool Func
}

// NewToolNode builds a ToolNode with the given name and tool function.
func NewToolNode(name string, tool Func) *ToolNode {
	return &ToolNode{nodeBase: nodeBase{name: name}, Tool: tool}
}

// Run implements Node.
func (n *ToolNode) Run(ctx context.Context, input map[string]any) (Route, map[string]any, error) {
	return runCore(ctx, n.Name(), n.Tool, input)
}

// AgentNode delegates to an agent invocation. Like ToolNode it uses a simple
// func contract instead of importing the agent engine.
type AgentNode struct {
	nodeBase
	Agent Func
}

// NewAgentNode builds an AgentNode with the given name and agent function.
func NewAgentNode(name string, agent Func) *AgentNode {
	return &AgentNode{nodeBase: nodeBase{name: name}, Agent: agent}
}

// Run implements Node.
func (n *AgentNode) Run(ctx context.Context, input map[string]any) (Route, map[string]any, error) {
	return runCore(ctx, n.Name(), n.Agent, input)
}

func runCore(ctx context.Context, name string, fn Func, input map[string]any) (Route, map[string]any, error) {
	route, out, err := fn(ctx, input)
	if err != nil {
		return nil, nil, err
	}
	if m, ok := out.(map[string]any); ok {
		return route, m, nil
	}
	if out != nil {
		return route, map[string]any{name: out}, nil
	}
	return route, nil, nil
}

// JoinNode runs a set of branch nodes and merges their outputs. Each branch
// executes once; outputs are keyed by branch node name (or by the keys of
// map-valued branch outputs). When Concurrent is true the branches run in
// parallel bounded by the workflow's WithMaxConcurrency value; otherwise they
// run sequentially. JoinNode returns a nil Route, so connect it with a
// nil-route edge (always follow) or wrap it in a FunctionNode to emit a typed
// route.
type JoinNode struct {
	nodeBase
	Branches   []Node
	Concurrent bool
}

// NewJoinNode builds a JoinNode with the given name and branches.
func NewJoinNode(name string, branches []Node) *JoinNode {
	return &JoinNode{nodeBase: nodeBase{name: name}, Branches: branches}
}

// Name implements Node.
func (n *JoinNode) Name() string { return n.name }

// Run implements Node.
func (n *JoinNode) Run(ctx context.Context, input map[string]any) (Route, map[string]any, error) {
	merged := make(map[string]any)
	if len(n.Branches) == 0 {
		return nil, merged, nil
	}

	if !n.Concurrent || len(n.Branches) == 1 {
		for _, b := range n.Branches {
			if err := runBranch(ctx, b, cloneMap(input), merged); err != nil {
				return nil, merged, err
			}
		}
		return nil, merged, nil
	}

	st := stateFrom(ctx)
	limit := len(n.Branches)
	if st != nil && st.maxConcurrency > 0 {
		limit = st.maxConcurrency
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error
	sem := make(chan struct{}, limit)
	for _, b := range n.Branches {
		b := b
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			return nil, merged, ctx.Err()
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			branchOut := make(map[string]any)
			err := runBranch(ctx, b, cloneMap(input), branchOut)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			mergeOutput(merged, branchOut)
		}()
	}
	wg.Wait()
	return nil, merged, errors.Join(errs...)
}

// runBranch executes a single branch node, checking the run's visited set so a
// branch that repeats a node already executed elsewhere is caught as a cycle.
func runBranch(ctx context.Context, b Node, input, merged map[string]any) error {
	st := stateFrom(ctx)
	if st != nil {
		if err := st.markExecuted(b.Name()); err != nil {
			return err
		}
	}
	_, out, err := b.Run(ctx, input)
	if err != nil {
		return fmt.Errorf("%w: node %q: %w", ErrBranchFailed, b.Name(), err)
	}
	mergeOutput(merged, out)
	return nil
}

// validateNode ensures concrete node implementations are runnable.
func validateNode(n Node) error {
	switch v := n.(type) {
	case *FunctionNode:
		if v.Fn == nil {
			return fmt.Errorf("node %q: FunctionNode has nil Fn", n.Name())
		}
	case *ToolNode:
		if v.Tool == nil {
			return fmt.Errorf("node %q: ToolNode has nil Tool", n.Name())
		}
	case *AgentNode:
		if v.Agent == nil {
			return fmt.Errorf("node %q: AgentNode has nil Agent", n.Name())
		}
	case *JoinNode:
		if len(v.Branches) == 0 {
			return fmt.Errorf("node %q: JoinNode has no branches", n.Name())
		}
		for _, b := range v.Branches {
			if b == nil {
				return fmt.Errorf("node %q: JoinNode has a nil branch", n.Name())
			}
			if err := validateNode(b); err != nil {
				return err
			}
		}
	}
	return nil
}
