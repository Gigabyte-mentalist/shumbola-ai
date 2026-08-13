// Package tasktree implements the Pentesting Task Tree (PTT), TZ 4.2:
// engagement holati (nishon → topilma → keyingi qadamlar) strukturali
// daraxtda saqlanadi.
package tasktree

import (
	"fmt"
	"sync"
	"time"
)

// Kind classifies what a Node represents.
type Kind string

const (
	KindTarget  Kind = "target"
	KindStep    Kind = "step"
	KindFinding Kind = "finding"
	KindError   Kind = "error"
)

// Node is a single entry in the task tree.
type Node struct {
	ID        string         `json:"id"`
	ParentID  string         `json:"parent_id,omitempty"`
	Kind      Kind           `json:"kind"`
	Summary   string         `json:"summary"`
	Details   map[string]any `json:"details,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

// maxNodes is the placeholder cap used by Summarize until real
// LLM-driven summarization lands (Bosqich 4, RAG/LoRA). It keeps state
// bounded for TZ 6.2's context budget without losing recent history.
const maxNodes = 200

// Tree is the append-mostly state for one engagement's ReAct run.
type Tree struct {
	mu           sync.Mutex
	EngagementID string `json:"engagement_id"`
	Nodes        []Node `json:"nodes"`
}

// New creates an empty task tree for an engagement.
func New(engagementID string) *Tree {
	return &Tree{EngagementID: engagementID}
}

// AddNode appends a new node and returns it. The node ID is derived from
// the current node count, so it stays consistent across process restarts
// without needing a separately persisted counter.
func (t *Tree) AddNode(parentID string, kind Kind, summary string, details map[string]any) Node {
	t.mu.Lock()
	defer t.mu.Unlock()

	seq := len(t.Nodes) + 1
	n := Node{
		ID:        fmt.Sprintf("%s-%d", t.EngagementID, seq),
		ParentID:  parentID,
		Kind:      kind,
		Summary:   summary,
		Details:   details,
		CreatedAt: time.Now().UTC(),
	}
	t.Nodes = append(t.Nodes, n)
	return n
}

// Snapshot returns a copy of the current nodes, safe to read concurrently
// with further writes.
func (t *Tree) Snapshot() []Node {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]Node, len(t.Nodes))
	copy(out, t.Nodes)
	return out
}

// HasTargetResult reports whether a finding or error node already exists
// for the given target summary — used by the StaticPlanner to avoid
// re-scanning a target it already has a result for.
func (t *Tree) HasTargetResult(target string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, n := range t.Nodes {
		if (n.Kind == KindFinding || n.Kind == KindError) && n.Details != nil {
			if got, ok := n.Details["target"]; ok && got == target {
				return true
			}
		}
	}
	return false
}

// HasToolResult reports whether a finding or error node already exists
// for the given (target, tool) pair — used by the StaticPlanner so a
// multi-tool sequence (e.g. nmap, then nuclei, then ffuf) doesn't repeat
// a tool it has already run against a target.
func (t *Tree) HasToolResult(target, tool string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, n := range t.Nodes {
		if (n.Kind == KindFinding || n.Kind == KindError) && n.Details != nil {
			gotTarget, ok := n.Details["target"]
			if !ok || gotTarget != target {
				continue
			}
			if gotTool, ok := n.Details["tool"]; ok && gotTool == tool {
				return true
			}
		}
	}
	return false
}

// Summarize bounds the tree to the most recent maxNodes entries. This is
// a placeholder for the LLM-driven summarization TZ 6.2 calls for once a
// real model is wired in; for Bosqich 1 it only needs to keep state from
// growing unbounded during a long-running engagement.
func (t *Tree) Summarize() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.Nodes) <= maxNodes {
		return
	}
	t.Nodes = t.Nodes[len(t.Nodes)-maxNodes:]
}
