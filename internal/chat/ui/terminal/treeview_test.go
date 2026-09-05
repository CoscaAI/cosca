package terminal

import (
	"strings"
	"testing"
)

func sampleTree() *TreeView {
	tv := NewTreeView()
	tv.SetRoot(&TreeNode{
		ID:     "root",
		Label:  "EXECUÇÃO",
		Status: "running",
		Children: []*TreeNode{
			{
				ID:     "a",
				Label:  "PLAN_CREATED",
				Status: "running",
				Detail: "plan for the task",
				Children: []*TreeNode{
					{ID: "a1", Label: "TOOL_STARTED", Status: "running"},
					{ID: "a2", Label: "TOOL_RESULT", Status: "done"},
				},
			},
			{ID: "b", Label: "BUILD_STARTED", Status: "pending"},
		},
	})
	return tv
}

func TestTreeFlattenRespectsExpanded(t *testing.T) {
	tv := sampleTree()

	// Root children visible by default; "a" is not expanded yet.
	flat := tv.Flatten()
	if len(flat) != 2 {
		t.Fatalf("flatten with collapsed children = %d nodes, want 2", len(flat))
	}

	tv.expanded["a"] = true
	flat = tv.Flatten()
	if len(flat) != 4 {
		t.Fatalf("flatten with expanded 'a' = %d nodes, want 4", len(flat))
	}
	if flat[0].ID != "a" || flat[1].ID != "a1" || flat[2].ID != "a2" || flat[3].ID != "b" {
		t.Fatalf("unexpected flatten order: %v", flat)
	}
}

func TestTreeUpsert(t *testing.T) {
	tv := NewTreeView()

	tv.Upsert(&TreeNode{ID: "x", Label: "PLAN_CREATED", Status: "running"})
	if tv.Find("x") == nil {
		t.Fatalf("Upsert did not add node x")
	}

	tv.Upsert(&TreeNode{ID: "x", Label: "PLAN_CREATED", Status: "done"})
	found := tv.Find("x")
	if found.Status != "done" {
		t.Fatalf("Upsert did not update status: %q", found.Status)
	}
}

func TestTreeSelection(t *testing.T) {
	tv := sampleTree()

	if got := tv.Selected(); got == nil || got.ID != "a" {
		t.Fatalf("initial selection = %v, want a", got)
	}
	tv.MoveDown()
	if got := tv.Selected(); got == nil || got.ID != "b" {
		t.Fatalf("selection after MoveDown = %v, want b", got)
	}
	tv.MoveDown() // already at bottom — should clamp
	if got := tv.Selected(); got == nil || got.ID != "b" {
		t.Fatalf("selection after clamped MoveDown = %v, want b", got)
	}
	tv.MoveUp()
	if got := tv.Selected(); got == nil || got.ID != "a" {
		t.Fatalf("selection after MoveUp = %v, want a", got)
	}
}

func TestTreeToggleExpand(t *testing.T) {
	tv := sampleTree()

	// "a" is currently collapsed.
	if tv.expanded["a"] {
		t.Fatalf("expected 'a' collapsed initially")
	}
	tv.ToggleExpand()
	if !tv.expanded["a"] {
		t.Fatalf("expected 'a' expanded after ToggleExpand")
	}
	tv.ToggleExpand()
	if tv.expanded["a"] {
		t.Fatalf("expected 'a' collapsed after second ToggleExpand")
	}
}

func TestTreeRenderGlyphs(t *testing.T) {
	tv := NewTreeView()
	tv.SetRoot(&TreeNode{
		ID:     "root",
		Label:  "EXECUÇÃO",
		Status: "running",
		Children: []*TreeNode{
			{
				ID:     "a",
				Label:  "PLAN_CREATED",
				Status: "running",
				Children: []*TreeNode{
					{ID: "a1", Label: "TOOL_STARTED", Status: "running"},
					{ID: "a2", Label: "TOOL_RESULT", Status: "done"},
				},
			},
			{
				ID:       "c",
				Label:    "BUILD_STARTED",
				Status:   "pending",
				Children: []*TreeNode{{ID: "c1", Label: "BUILD_END", Status: "done"}},
			},
		},
	})
	// Expand "a" only — "c" stays collapsed so both ▾ and ▸ appear.
	tv.expanded["a"] = true
	out := tv.Render(60, 10)

	for _, glyph := range []string{"▸", "▾", "⠿", "✓", "○"} {
		if !strings.Contains(out, glyph) {
			t.Fatalf("render missing glyph %q in:\n%s", glyph, out)
		}
	}
}

func TestTreeRenderSelectionHighlight(t *testing.T) {
	tv := sampleTree()
	out := tv.Render(60, 10)
	if !strings.Contains(out, "PLAN_CREATED") {
		t.Fatalf("render missing selected label:\n%s", out)
	}
}

func TestTreeRenderTruncation(t *testing.T) {
	tv := NewTreeView()
	tv.SetRoot(&TreeNode{
		ID:     "root",
		Label:  "EXECUÇÃO",
		Status: "idle",
		Children: []*TreeNode{
			{ID: "x", Label: "a-very-very-long-label-that-cannot-possibly-fit", Status: "done"},
		},
	})
	out := tv.Render(12, 5)
	if !strings.Contains(out, "...") {
		t.Fatalf("render did not truncate long label:\n%s", out)
	}
}

func TestTreeRenderDetailFooter(t *testing.T) {
	tv := sampleTree()
	out := tv.Render(40, 6)
	if !strings.Contains(out, "plan for the task") {
		t.Fatalf("render missing detail footer:\n%s", out)
	}
}
