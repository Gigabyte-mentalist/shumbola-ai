package tasktree

import "testing"

func TestAddNodeAndSnapshot(t *testing.T) {
	tree := New("eng-1")
	n1 := tree.AddNode("", KindTarget, "10.0.0.5", nil)
	n2 := tree.AddNode(n1.ID, KindFinding, "port 22 open", map[string]any{"target": "10.0.0.5"})

	snap := tree.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(snap))
	}
	if snap[1].ParentID != n1.ID {
		t.Fatalf("expected second node's parent to be %s, got %s", n1.ID, snap[1].ParentID)
	}
	if n2.ID == n1.ID {
		t.Fatalf("expected unique node IDs")
	}
}

func TestHasTargetResult(t *testing.T) {
	tree := New("eng-1")
	if tree.HasTargetResult("10.0.0.5") {
		t.Fatalf("expected no result yet")
	}
	tree.AddNode("", KindFinding, "port 22 open", map[string]any{"target": "10.0.0.5"})
	if !tree.HasTargetResult("10.0.0.5") {
		t.Fatalf("expected result to be recorded for 10.0.0.5")
	}
	if tree.HasTargetResult("10.0.0.6") {
		t.Fatalf("expected no result for a different target")
	}
}

func TestHasToolResult(t *testing.T) {
	tree := New("eng-1")
	if tree.HasToolResult("10.0.0.5", "nmap") {
		t.Fatalf("expected no result yet")
	}
	tree.AddNode("", KindFinding, "22/tcp open", map[string]any{"target": "10.0.0.5", "tool": "nmap"})
	if !tree.HasToolResult("10.0.0.5", "nmap") {
		t.Fatalf("expected result recorded for (10.0.0.5, nmap)")
	}
	if tree.HasToolResult("10.0.0.5", "nuclei") {
		t.Fatalf("expected no result for a different tool on the same target")
	}
	if tree.HasToolResult("10.0.0.6", "nmap") {
		t.Fatalf("expected no result for a different target")
	}
}

func TestSummarizeCapsNodeCount(t *testing.T) {
	tree := New("eng-1")
	for i := 0; i < maxNodes+50; i++ {
		tree.AddNode("", KindStep, "step", nil)
	}
	tree.Summarize()
	if len(tree.Snapshot()) != maxNodes {
		t.Fatalf("expected tree to be capped at %d nodes, got %d", maxNodes, len(tree.Snapshot()))
	}
}

func TestFileStoreSaveLoadOrNew(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	fresh, err := store.LoadOrNew("eng-1")
	if err != nil {
		t.Fatalf("LoadOrNew (missing): %v", err)
	}
	if len(fresh.Snapshot()) != 0 {
		t.Fatalf("expected fresh tree to be empty")
	}

	fresh.AddNode("", KindTarget, "10.0.0.5", nil)
	if err := store.Save(fresh); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reloaded, err := store.LoadOrNew("eng-1")
	if err != nil {
		t.Fatalf("LoadOrNew (existing): %v", err)
	}
	if len(reloaded.Snapshot()) != 1 {
		t.Fatalf("expected 1 persisted node, got %d", len(reloaded.Snapshot()))
	}
}
