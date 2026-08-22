package terminal

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// TreeNode is a single node in the interactive execution tree. Status is one
// of: running | done | failed | pending | idle.
type TreeNode struct {
	ID       string
	Label    string
	Detail   string
	Status   string
	Children []*TreeNode
}

// TreeView is a pure-lipgloss interactive tree with keyboard selection and
// expand/collapse. The root node is an invisible container; its children are
// the visible top-level rows.
type TreeView struct {
	root     *TreeNode
	selected int
	expanded map[string]bool
	OnSelect func(*TreeNode)
}

// NewTreeView returns an empty tree view with no visible rows.
func NewTreeView() *TreeView {
	return &TreeView{
		root: &TreeNode{
			ID:       "root",
			Label:    "EXECUÇÃO",
			Status:   "idle",
			Children: []*TreeNode{},
		},
		selected: 0,
		expanded: map[string]bool{"root": true},
	}
}

// SetRoot replaces the entire tree. Selection is clamped to the new size.
func (t *TreeView) SetRoot(node *TreeNode) {
	if node == nil {
		node = &TreeNode{
			ID:       "root",
			Label:    "EXECUÇÃO",
			Status:   "idle",
			Children: []*TreeNode{},
		}
	}
	if t.expanded == nil {
		t.expanded = map[string]bool{}
	}
	t.root = node
	t.expanded[node.ID] = true
	t.clampSelected()
}

// Upsert adds a node by ID or updates it in place. If the node already exists
// its label/detail/status are refreshed; existing children are preserved
// unless the caller passes a non-empty children slice.
func (t *TreeView) Upsert(node *TreeNode) {
	if node == nil {
		return
	}
	if existing := t.Find(node.ID); existing != nil {
		existing.Label = node.Label
		existing.Detail = node.Detail
		existing.Status = node.Status
		if len(node.Children) > 0 {
			existing.Children = node.Children
		}
		return
	}
	if t.root == nil {
		t.SetRoot(node)
		return
	}
	t.root.Children = append(t.root.Children, node)
}

// Find returns the node with the given ID (DFS), or nil.
func (t *TreeView) Find(id string) *TreeNode {
	var dfs func(*TreeNode) *TreeNode
	dfs = func(n *TreeNode) *TreeNode {
		if n == nil {
			return nil
		}
		if n.ID == id {
			return n
		}
		for _, c := range n.Children {
			if found := dfs(c); found != nil {
				return found
			}
		}
		return nil
	}
	return dfs(t.root)
}

type flatEntry struct {
	node  *TreeNode
	depth int
}

// flattenEntries returns the visible nodes (respecting expanded) in render
// order. The root container is not included.
func (t *TreeView) flattenEntries() []flatEntry {
	if t.root == nil {
		return nil
	}
	var out []flatEntry
	var walk func(*TreeNode, int)
	walk = func(n *TreeNode, depth int) {
		if n == nil {
			return
		}
		out = append(out, flatEntry{node: n, depth: depth})
		if t.expanded[n.ID] {
			for _, c := range n.Children {
				walk(c, depth+1)
			}
		}
	}
	for _, c := range t.root.Children {
		walk(c, 0)
	}
	return out
}

// Flatten returns the currently visible nodes (respecting expanded state).
func (t *TreeView) Flatten() []*TreeNode {
	entries := t.flattenEntries()
	nodes := make([]*TreeNode, 0, len(entries))
	for _, e := range entries {
		nodes = append(nodes, e.node)
	}
	return nodes
}

func (t *TreeView) clampSelected() {
	n := len(t.flattenEntries())
	if n == 0 {
		t.selected = 0
		return
	}
	if t.selected >= n {
		t.selected = n - 1
	}
	if t.selected < 0 {
		t.selected = 0
	}
}

// MoveUp moves the selection up one visible row.
func (t *TreeView) MoveUp() {
	t.clampSelected()
	if t.selected > 0 {
		t.selected--
	}
	if t.OnSelect != nil {
		t.OnSelect(t.Selected())
	}
}

// MoveDown moves the selection down one visible row.
func (t *TreeView) MoveDown() {
	t.clampSelected()
	if t.selected < len(t.flattenEntries())-1 {
		t.selected++
	}
	if t.OnSelect != nil {
		t.OnSelect(t.Selected())
	}
}

// ToggleExpand expands or collapses the selected node.
func (t *TreeView) ToggleExpand() {
	node := t.Selected()
	if node == nil {
		return
	}
	t.expanded[node.ID] = !t.expanded[node.ID]
	t.clampSelected()
}

// ExpandID forces a node ID to be expanded.
func (t *TreeView) ExpandID(id string) {
	t.expanded[id] = true
}

// Selected returns the currently selected visible node, or nil.
func (t *TreeView) Selected() *TreeNode {
	entries := t.flattenEntries()
	if len(entries) == 0 {
		return nil
	}
	t.clampSelected()
	return entries[t.selected].node
}

// Render draws the tree into the given box.
func (t *TreeView) Render(width, height int) string {
	if width < 4 {
		width = 4
	}
	if height < 1 {
		height = 1
	}

	entries := t.flattenEntries()
	if len(entries) > 0 {
		t.clampSelected()
	}

	// Reserve a footer line when the selected node has a detail.
	detail := ""
	if sel := t.Selected(); sel != nil && strings.TrimSpace(sel.Detail) != "" {
		detail = truncateWidth(sel.Detail, width-3)
	}
	treeRows := height
	if detail != "" {
		treeRows = height - 1
		if treeRows < 1 {
			treeRows = 1
		}
	}

	rows := make([]string, 0, treeRows)
	for i, e := range entries {
		if len(rows) >= treeRows {
			break
		}
		rows = append(rows, t.renderRow(e, i, width))
	}
	if len(entries) > len(rows) {
		if len(rows) > 0 {
			rows[len(rows)-1] = infoBubble.Render(fmt.Sprintf("  ... %d more", len(entries)-len(rows)+1))
		}
	}

	var b strings.Builder
	for _, r := range rows {
		b.WriteString(r)
		b.WriteString("\n")
	}
	if detail != "" {
		b.WriteString(lipgloss.NewStyle().
			Foreground(colorTextMuted).
			Italic(true).
			Render("· " + detail))
	}

	return strings.TrimRight(b.String(), "\n")
}

func (t *TreeView) renderRow(e flatEntry, index int, width int) string {
	node := e.node
	hasChildren := len(node.Children) > 0

	toggle := " "
	if hasChildren {
		if t.expanded[node.ID] {
			toggle = "▾"
		} else {
			toggle = "▸"
		}
	}

	glyph := StatusDot(node.Status, 0)

	label := node.Label
	if label == "" {
		label = node.ID
	}

	prefix := strings.Repeat("  ", e.depth) + toggle + " " + glyph + " "
	avail := width - lipgloss.Width(prefix)
	if avail < 4 {
		avail = 4
	}
	label = truncateWidth(label, avail)

	row := prefix + label

	statusStyle := statusRowStyle(node.Status)
	if index == t.selected {
		row = lipgloss.NewStyle().
			Background(th.Selection).
			Foreground(colorFg).
			Render(row)
	} else {
		row = statusStyle.Render(row)
	}
	return row
}

func statusRowStyle(status string) lipgloss.Style {
	switch status {
	case "running":
		return taskRunningStyle
	case "done":
		return taskDoneStyle
	case "failed":
		return taskFailedStyle
	case "pending":
		return taskPendingStyle
	default:
		return lipgloss.NewStyle().Foreground(colorTextMuted)
	}
}

// truncateWidth truncates s to at most maxW display columns, appending "...".
func truncateWidth(s string, maxW int) string {
	if maxW < 4 {
		maxW = 4
	}
	if lipgloss.Width(s) <= maxW {
		return s
	}
	var b strings.Builder
	w := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > maxW-3 {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	return b.String() + "..."
}
