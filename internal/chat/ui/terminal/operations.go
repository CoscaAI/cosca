package terminal

import (
	"fmt"
	"strings"

	"github.com/CoscaAI/cosca/internal/trace"
)

// OperationsPanel is the right-side interactive execution tree. It is fed by
// real pipeline events (mapped to trace.Event) and renders the live tree with
// an animated progress header.
type OperationsPanel struct {
	tree    *TreeView
	events  int
	active  int
	frame   int
	entries []trace.Event
}

// NewOperationsPanel returns an empty operations panel.
func NewOperationsPanel() *OperationsPanel {
	return &OperationsPanel{
		tree:    NewTreeView(),
		entries: []trace.Event{},
	}
}

// SetFrame updates the animation frame used for the header progress bar.
func (p *OperationsPanel) SetFrame(f int) {
	p.frame = f
}

// AddEvent ingests one pipeline-derived event into the operations ledger and
// rebuilds the tree. Expansion state is preserved because node IDs are
// deterministic from the event sequence.
func (p *OperationsPanel) AddEvent(ev trace.Event) {
	p.events++
	p.entries = append(p.entries, ev)
	p.active = countRunning(p.entries)
	p.tree.SetRoot(BuildTreeFromEvents(p.entries))
	// Auto-expand run roots so new work is visible immediately.
	if p.tree.root != nil {
		for _, c := range p.tree.root.Children {
			p.tree.ExpandID(c.ID)
		}
	}
}

func countRunning(events []trace.Event) int {
	n := 0
	for _, ev := range events {
		if eventStatus(ev) == "running" {
			n++
		}
	}
	return n
}

// eventStatus derives a tree status from a trace event's action/result.
func eventStatus(ev trace.Event) string {
	a := strings.ToUpper(ev.Action)
	if ev.Result == "failed" ||
		strings.HasSuffix(a, "_FAILED") ||
		strings.HasSuffix(a, "_ERROR") {
		return "failed"
	}
	if ev.Result == "success" ||
		strings.HasSuffix(a, "_DONE") ||
		strings.HasSuffix(a, "_COMPLETED") ||
		strings.HasSuffix(a, "_END") {
		return "done"
	}
	if ev.Result == "running" ||
		strings.HasSuffix(a, "_STARTED") ||
		strings.HasSuffix(a, "_RETRYING") ||
		a == "PLAN_CREATED" ||
		a == "CONTENT" {
		return "running"
	}
	return "pending"
}

// BuildTreeFromEvents builds an execution tree from a trace event ledger,
// nesting events by ParentEvent. Events sharing a TraceID are grouped under a
// single run root; the root's status reflects the most recent terminal event.
func BuildTreeFromEvents(events []trace.Event) *TreeNode {
	root := &TreeNode{
		ID:       "root",
		Label:    "EXECUÇÃO",
		Status:   "idle",
		Children: []*TreeNode{},
	}
	if len(events) == 0 {
		return root
	}

	nodes := map[string]*TreeNode{"root": root}
	traceSeq := map[string]int{}

	for i, ev := range events {
		tid := ev.TraceID
		if tid == "" {
			tid = fmt.Sprintf("evt-%d", i)
		}

		traceRoot, ok := nodes[tid]
		if !ok {
			traceRoot = &TreeNode{
				ID:       tid,
				Label:    "EXECUÇÃO " + shortID(tid),
				Status:   "running",
				Children: []*TreeNode{},
			}
			nodes[tid] = traceRoot
			root.Children = append(root.Children, traceRoot)
		}

		st := eventStatus(ev)

		// Terminal markers update the run root instead of adding a leaf.
		if ev.Action == ActionExecutionDone || ev.Action == ActionExecutionFailed {
			traceRoot.Status = st
			continue
		}

		traceSeq[tid]++
		id := fmt.Sprintf("%s-%d", tid, traceSeq[tid])
		node := &TreeNode{
			ID:     id,
			Label:  ev.Action,
			Detail: ev.Details,
			Status: st,
		}
		nodes[id] = node

		parent := nodes[ev.ParentEvent]
		if parent == nil {
			parent = traceRoot
		}
		parent.Children = append(parent.Children, node)

		if st == "failed" || st == "done" {
			traceRoot.Status = st
		} else if traceRoot.Status == "" {
			traceRoot.Status = "running"
		}
	}

	return root
}

func shortID(id string) string {
	parts := strings.Split(id, "-")
	if len(parts) == 0 {
		return id
	}
	return parts[len(parts)-1]
}

// Render draws the operations panel: "EXECUÇÃO" header with event counts, an
// animated progress bar, the interactive tree, and a detail footer.
func (p *OperationsPanel) Render(width, height int) string {
	styleWidth := width - taskPanelStyle.GetHorizontalBorderSize()
	if styleWidth < 1 {
		styleWidth = 1
	}
	contentWidth := styleWidth - taskPanelStyle.GetHorizontalPadding()
	if contentWidth < 1 {
		contentWidth = 1
	}
	contentHeight := height - taskPanelStyle.GetVerticalFrameSize()
	if contentHeight < 1 {
		contentHeight = 1
	}

	panel := taskPanelStyle.Width(styleWidth).Height(contentHeight)

	var b strings.Builder
	b.WriteString(titleSubStyle.Render(" EXECUÇÃO "))
	b.WriteString(taskAgentStyle.Render(fmt.Sprintf("%d", p.events)))
	if p.active > 0 {
		b.WriteString(taskRunningStyle.Render(fmt.Sprintf(" · %d ativos", p.active)))
	}
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", contentWidth))
	b.WriteString("\n")

	pct := 0.0
	if p.events > 0 {
		pct = float64(p.events-p.active) / float64(p.events)
	}
	b.WriteString(progressFillStyle.Render(AnimatedProgress(pct, contentWidth, p.frame)))
	b.WriteString("\n")

	treeHeight := contentHeight - 3
	if treeHeight < 1 {
		treeHeight = 1
	}
	b.WriteString(p.tree.Render(contentWidth, treeHeight))

	return panel.Render(b.String())
}

// Tree exposes the underlying TreeView for keyboard navigation.
func (p *OperationsPanel) Tree() *TreeView {
	return p.tree
}
