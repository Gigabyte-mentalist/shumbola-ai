package agentcore

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/shumbola/agent-core/internal/audit"
	"github.com/shumbola/agent-core/internal/engagement"
	"github.com/shumbola/agent-core/internal/hitl"
	"github.com/shumbola/agent-core/internal/tasktree"
	"github.com/shumbola/agent-core/internal/tools"
	"github.com/shumbola/agent-core/internal/validation"
)

// fakeTool is a controllable Tool double so engine tests don't depend on
// Docker or a real nmap binary.
type fakeTool struct {
	name             string
	requiresApproval bool
	result           tools.Result
	err              error
	invocations      []string
	// blockUntilCtxDone simulates a tool that never returns on its own
	// (e.g. nmap waiting on a WAF/rate-limiter that silently drops
	// probes) — Execute blocks until ctx is cancelled/times out.
	blockUntilCtxDone bool
}

func (f *fakeTool) Name() string           { return f.name }
func (f *fakeTool) RequiresApproval() bool { return f.requiresApproval }
func (f *fakeTool) Validate(target string) error {
	if target == "" {
		return errFakeEmptyTarget
	}
	return nil
}
func (f *fakeTool) Execute(ctx context.Context, target string) (tools.Result, error) {
	f.invocations = append(f.invocations, target)
	if f.blockUntilCtxDone {
		<-ctx.Done()
		return tools.Result{}, ctx.Err()
	}
	return f.result, f.err
}

var errFakeEmptyTarget = fakeErr("target required")

type fakeErr string

func (e fakeErr) Error() string { return string(e) }

func newTestEngine(t *testing.T, tool *fakeTool, planner Planner) (*Engine, *engagement.Engagement, *tasktree.Tree) {
	t.Helper()
	dir := t.TempDir()

	auditLogger, err := audit.NewLogger(filepath.Join(dir, "audit.log"))
	if err != nil {
		t.Fatalf("audit.NewLogger: %v", err)
	}
	t.Cleanup(func() { _ = auditLogger.Close() })

	engStore, err := engagement.NewFileStore(filepath.Join(dir, "engagements"))
	if err != nil {
		t.Fatalf("engagement.NewFileStore: %v", err)
	}
	treeStore, err := tasktree.NewFileStore(filepath.Join(dir, "tasktrees"))
	if err != nil {
		t.Fatalf("tasktree.NewFileStore: %v", err)
	}

	eng, err := engagement.New("eng-1", "op-1", []string{"10.0.0.5"})
	if err != nil {
		t.Fatalf("engagement.New: %v", err)
	}
	if err := engStore.Save(eng); err != nil {
		t.Fatalf("engStore.Save: %v", err)
	}
	tree := tasktree.New(eng.ID)

	approvals := hitl.NewStore()
	engine := NewEngine(map[string]tools.Tool{tool.name: tool}, planner, approvals, auditLogger, engStore, treeStore, validation.NewValidator())
	return engine, eng, tree
}

func TestEngineRunsRecordsFindings(t *testing.T) {
	tool := &fakeTool{
		name: "nmap",
		result: tools.Result{
			Findings: []tools.Finding{{Title: "22/tcp open (ssh)", Severity: tools.SeverityInfo, Detail: "..."}},
		},
	}
	planner := &StaticPlanner{ToolNames: []string{"nmap"}}
	engine, eng, tree := newTestEngine(t, tool, planner)

	if err := engine.Run(context.Background(), eng, tree); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if eng.Status != engagement.StatusDone {
		t.Fatalf("expected status done, got %s", eng.Status)
	}
	if len(tool.invocations) != 1 || tool.invocations[0] != "10.0.0.5" {
		t.Fatalf("expected tool invoked once with 10.0.0.5, got %v", tool.invocations)
	}
	findings := 0
	for _, n := range tree.Snapshot() {
		if n.Kind == tasktree.KindFinding {
			findings++
		}
	}
	if findings != 1 {
		t.Fatalf("expected 1 finding node, got %d", findings)
	}
}

func TestEngineBlocksOutOfScopeTarget(t *testing.T) {
	tool := &fakeTool{name: "nmap"}
	// Planner proposes a target that is NOT in the engagement's scope.
	planner := plannerFunc(func(ctx context.Context, state PlannerState) (Action, bool, error) {
		return Action{Tool: "nmap", Target: "10.0.0.99"}, false, nil
	})
	engine, eng, tree := newTestEngine(t, tool, planner)

	err := engine.Run(context.Background(), eng, tree)
	if err == nil {
		t.Fatalf("expected error for out-of-scope target")
	}
	if eng.Status != engagement.StatusError {
		t.Fatalf("expected status error, got %s", eng.Status)
	}
	if len(tool.invocations) != 0 {
		t.Fatalf("expected tool to never be invoked for an out-of-scope target, got %v", tool.invocations)
	}
}

// TestEngineStopsRepeatingPlannerLoop covers a planner (e.g. an LLM)
// that keeps proposing the same already-executed (tool, target) pair
// instead of concluding done — observed in practice against the real
// model despite the system prompt asking it not to repeat steps. The
// Engine must enforce this in code rather than trust the planner, the
// same way it enforces scope.
func TestEngineStopsRepeatingPlannerLoop(t *testing.T) {
	tool := &fakeTool{name: "nuclei"}
	planner := plannerFunc(func(ctx context.Context, state PlannerState) (Action, bool, error) {
		return Action{Tool: "nuclei", Target: "10.0.0.5"}, false, nil
	})
	engine, eng, tree := newTestEngine(t, tool, planner)

	if err := engine.Run(context.Background(), eng, tree); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if eng.Status != engagement.StatusDone {
		t.Fatalf("expected status done, got %s (note: %s)", eng.Status, eng.StatusNote)
	}
	if len(tool.invocations) != 1 {
		t.Fatalf("expected the tool to run exactly once despite repeated proposals, got %d: %v", len(tool.invocations), tool.invocations)
	}
}

func TestEngineWaitsForHITLApproval(t *testing.T) {
	tool := &fakeTool{name: "nmap", requiresApproval: true}
	planner := &StaticPlanner{ToolNames: []string{"nmap"}}
	engine, eng, tree := newTestEngine(t, tool, planner)

	done := make(chan error, 1)
	go func() { done <- engine.Run(context.Background(), eng, tree) }()

	approved := waitForApproval(t, engine, eng.ID)
	if err := engine.approvals.Decide(approved, true); err != nil {
		t.Fatalf("Decide: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Run did not complete after approval")
	}

	if len(tool.invocations) != 1 {
		t.Fatalf("expected tool to run once after approval, got %d", len(tool.invocations))
	}
}

// TestEngineToolExecutionTimeout covers a real symptom found in manual
// testing (2026-08-13): scanning a real, WAF/rate-limit-protected
// internet host left nmap waiting on probes that never got a response,
// hanging the whole engagement forever with zero feedback — Engine.Run's
// context has no deadline of its own, so nothing would ever unstick it.
// A tool.Execute call is now bounded by toolExecutionTimeout.
func TestEngineToolExecutionTimeout(t *testing.T) {
	old := toolExecutionTimeout
	toolExecutionTimeout = 50 * time.Millisecond
	defer func() { toolExecutionTimeout = old }()

	tool := &fakeTool{name: "nmap", blockUntilCtxDone: true}
	planner := &StaticPlanner{ToolNames: []string{"nmap"}}
	engine, eng, tree := newTestEngine(t, tool, planner)

	err := engine.Run(context.Background(), eng, tree)
	if err == nil {
		t.Fatalf("expected an error from a timed-out tool execution")
	}
	if eng.Status != engagement.StatusError {
		t.Fatalf("expected status error, got %s (note: %s)", eng.Status, eng.StatusNote)
	}
}

func TestEngineKillSwitchStopsRun(t *testing.T) {
	tool := &fakeTool{name: "nmap", requiresApproval: true}
	planner := &StaticPlanner{}
	engine, eng, tree := newTestEngine(t, tool, planner)

	done := make(chan error, 1)
	go func() { done <- engine.Run(context.Background(), eng, tree) }()

	waitForApproval(t, engine, eng.ID)
	if !engine.Stop(eng.ID) {
		t.Fatalf("expected Stop to find a running engagement")
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Run did not stop after kill switch")
	}

	if eng.Status != engagement.StatusStopped {
		t.Fatalf("expected status stopped, got %s", eng.Status)
	}
	if len(tool.invocations) != 0 {
		t.Fatalf("expected tool never invoked once killed before approval, got %v", tool.invocations)
	}
}

// TestEngineKillSwitchDuringPlannerCallStopsCleanly covers the case
// where Stop fires while the planner (e.g. an in-flight LLM request) is
// mid-call rather than while waiting on HITL approval — the resulting
// context-cancellation error must be treated as a stop, not a planner
// failure (regression: this used to record status "error" instead of
// "stopped").
func TestEngineKillSwitchDuringPlannerCallStopsCleanly(t *testing.T) {
	inPlanner := make(chan struct{})
	planner := plannerFunc(func(ctx context.Context, state PlannerState) (Action, bool, error) {
		close(inPlanner)
		<-ctx.Done()
		return Action{}, false, ctx.Err()
	})
	tool := &fakeTool{name: "nmap"}
	engine, eng, tree := newTestEngine(t, tool, planner)

	done := make(chan error, 1)
	go func() { done <- engine.Run(context.Background(), eng, tree) }()

	<-inPlanner
	if !engine.Stop(eng.ID) {
		t.Fatalf("expected Stop to find a running engagement")
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Run did not stop after kill switch")
	}

	if eng.Status != engagement.StatusStopped {
		t.Fatalf("expected status stopped, got %s (note: %s)", eng.Status, eng.StatusNote)
	}
}

// plannerFunc adapts a function to the Planner interface.
type plannerFunc func(ctx context.Context, state PlannerState) (Action, bool, error)

func (f plannerFunc) Decide(ctx context.Context, state PlannerState) (Action, bool, error) {
	return f(ctx, state)
}

// waitForApproval polls until a pending approval appears for engagementID
// and returns its ID.
func waitForApproval(t *testing.T, engine *Engine, engagementID string) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if pending := engine.approvals.List(engagementID); len(pending) == 1 {
			return pending[0].ID
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no pending approval appeared for %s", engagementID)
	return ""
}
