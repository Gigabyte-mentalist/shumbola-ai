package agentcore

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/shumbola/agent-core/internal/audit"
	"github.com/shumbola/agent-core/internal/engagement"
	"github.com/shumbola/agent-core/internal/hitl"
	"github.com/shumbola/agent-core/internal/tasktree"
	"github.com/shumbola/agent-core/internal/tools"
	"github.com/shumbola/agent-core/internal/validation"
)

// toolExecutionTimeout bounds a single tool.Execute call. Found necessary
// in practice (2026-08-13): scanning a real, WAF/rate-limit-protected
// internet host can leave nmap's -sV service-detection phase waiting on
// probes that never get a response, for far longer than any reasonable
// wait — and Engine.Run's context has no deadline of its own (it's
// intentionally long-running, stopped only by the planner finishing or
// the kill switch), so without this a single slow target could hang an
// engagement indefinitely with no feedback to the operator at all.
//
// var, not const, so tests can shrink it temporarily rather than
// actually waiting out a multi-minute timeout.
var toolExecutionTimeout = 5 * time.Minute

// Engine runs the ReAct loop for one engagement at a time (TZ 4.2),
// enforcing scope (TZ 6.1) and the HITL gate (TZ 4.4) at every step, and
// supporting an operator-triggered kill switch (TZ 6.1).
type Engine struct {
	tools     map[string]tools.Tool
	planner   Planner
	approvals *hitl.Store
	audit     *audit.Logger
	engStore  engagement.Store
	treeStore *tasktree.FileStore
	validator *validation.Validator

	mu      sync.Mutex
	stopped map[string]context.CancelFunc
}

// NewEngine wires an Engine from its dependencies. toolset maps a tool's
// Name() to its implementation (e.g. {"nmap": nmapTool}). validator
// applies TZ 4.5's proof-of-exploit/confidence check to every finding
// before it lands in the task tree.
func NewEngine(toolset map[string]tools.Tool, planner Planner, approvals *hitl.Store, auditLogger *audit.Logger, engStore engagement.Store, treeStore *tasktree.FileStore, validator *validation.Validator) *Engine {
	return &Engine{
		tools:     toolset,
		planner:   planner,
		approvals: approvals,
		audit:     auditLogger,
		engStore:  engStore,
		treeStore: treeStore,
		validator: validator,
		stopped:   make(map[string]context.CancelFunc),
	}
}

// Run drives the ReAct loop for eng until the Planner signals it's done,
// a tool errors, scope is violated, or Stop is called for this
// engagement (kill switch). It persists the engagement and task tree
// after every step so an interrupted run can be resumed (TZ 6.3).
func (e *Engine) Run(parentCtx context.Context, eng *engagement.Engagement, tree *tasktree.Tree) error {
	ctx, cancel := context.WithCancel(parentCtx)
	e.mu.Lock()
	e.stopped[eng.ID] = cancel
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		delete(e.stopped, eng.ID)
		e.mu.Unlock()
		cancel()
	}()

	e.setStatus(eng, engagement.StatusRunning, "")

	// The system prompt asks the planner not to repeat a (tool, target)
	// pair it has already tried, but an LLM following that instruction is
	// a request, not a guarantee (observed in practice: models can loop
	// on the same no-findings step indefinitely). Enforce it here in
	// code instead — mirrors the scope check below, which for the same
	// reason is never taken on the planner's word alone. Seeded from the
	// tree's existing history so a resumed run (LoadOrNew) also honors
	// steps taken before a restart.
	executed := collectExecutedPairs(tree)
	repeatedSkips := 0

	for {
		if err := ctx.Err(); err != nil {
			e.setStatus(eng, engagement.StatusStopped, "operator to'xtatdi (kill switch)")
			return nil
		}

		action, done, err := e.planner.Decide(ctx, PlannerState{Engagement: eng, Tree: tree})
		if err != nil {
			if ctx.Err() != nil {
				// Stopped mid-planner-call (e.g. the LLM request was
				// in flight when the kill switch fired): not a real
				// planner failure.
				e.setStatus(eng, engagement.StatusStopped, "operator to'xtatdi (kill switch)")
				return nil
			}
			e.recordError(eng, tree, "", fmt.Sprintf("planner error: %v", err))
			return err
		}
		if done {
			e.setStatus(eng, engagement.StatusDone, "")
			e.persistTree(tree)
			return nil
		}

		key := action.Tool + "|" + action.Target
		if executed[key] {
			repeatedSkips++
			tree.AddNode("", tasktree.KindStep,
				fmt.Sprintf("%s ni %s ustida takror ishlatish taklif qilindi — o'tkazib yuborildi (allaqachon sinalgan)", action.Tool, action.Target),
				map[string]any{"target": action.Target, "tool": action.Tool})
			e.persistTree(tree)
			if repeatedSkips >= maxRepeatedSkips {
				e.setStatus(eng, engagement.StatusDone, "takrorlanuvchi takliflar sababli avtomatik yakunlandi")
				return nil
			}
			continue
		}

		if err := e.step(ctx, eng, tree, action); err != nil {
			if ctx.Err() != nil {
				// Stopped mid-approval-wait or mid-step: not a real error.
				e.setStatus(eng, engagement.StatusStopped, "operator to'xtatdi (kill switch)")
				return nil
			}
			e.recordError(eng, tree, action.Target, err.Error())
			return err
		}
		executed[key] = true
		repeatedSkips = 0

		tree.Summarize()
		e.persistTree(tree)
	}
}

// maxRepeatedSkips bounds how many times in a row the planner can
// propose an already-executed (tool, target) pair before Run gives up
// and marks the engagement done, rather than looping forever on a
// planner that never converges.
const maxRepeatedSkips = 3

// collectExecutedPairs seeds the dedup set from a tree's existing
// history (see the comment in Run), keyed the same way as the live loop.
func collectExecutedPairs(tree *tasktree.Tree) map[string]bool {
	executed := make(map[string]bool)
	for _, n := range tree.Snapshot() {
		target, tool := detailPair(n)
		if target == "" || tool == "" {
			continue
		}
		executed[tool+"|"+target] = true
	}
	return executed
}

// step executes a single ReAct action: scope check → HITL gate → tool
// execution → task tree/audit recording.
func (e *Engine) step(ctx context.Context, eng *engagement.Engagement, tree *tasktree.Tree, action Action) error {
	tool, ok := e.tools[action.Tool]
	if !ok {
		return fmt.Errorf("unknown tool %q", action.Tool)
	}

	if err := tool.Validate(action.Target); err != nil {
		return fmt.Errorf("validate target: %w", err)
	}

	// TZ 6.1: scope allowlist is enforced here, in code, before any tool
	// touches the network — never inferred from the planner's own claim.
	if !eng.Scope.Allows(action.Target) {
		e.logAudit(eng.ID, "scope.violation_blocked", action.Target, map[string]any{"tool": action.Tool})
		tree.AddNode("", tasktree.KindError, fmt.Sprintf("%s scope'dan tashqarida — bloklandi", action.Target), map[string]any{"target": action.Target})
		return fmt.Errorf("target %q is outside engagement scope", action.Target)
	}

	if tool.RequiresApproval() {
		desc := fmt.Sprintf("%s ni %s ustida ishga tushirish", tool.Name(), action.Target)
		if action.Reason != "" {
			// Populated by LLMPlanner with the model's own justification —
			// surfaced to the operator so an LLM-driven choice isn't a
			// black box at the point they're asked to approve it (TZ 4.4).
			desc = fmt.Sprintf("%s (sabab: %s)", desc, action.Reason)
		}
		e.logAudit(eng.ID, "hitl.requested", action.Target, map[string]any{"tool": action.Tool, "description": desc, "reason": action.Reason})

		approved, err := e.approvals.RequestApproval(ctx, eng.ID, desc)
		if err != nil {
			return err // ctx cancelled (kill switch) — caller treats this as a stop, not a failure
		}
		if !approved {
			e.logAudit(eng.ID, "hitl.rejected", action.Target, map[string]any{"tool": action.Tool})
			tree.AddNode("", tasktree.KindStep, fmt.Sprintf("%s operator tomonidan rad etildi", desc), map[string]any{"target": action.Target})
			return nil
		}
		e.logAudit(eng.ID, "hitl.approved", action.Target, map[string]any{"tool": action.Tool})
	}

	toolCtx, toolCancel := context.WithTimeout(ctx, toolExecutionTimeout)
	result, err := tool.Execute(toolCtx, action.Target)
	toolCancel()
	if err != nil {
		if toolCtx.Err() == context.DeadlineExceeded {
			err = fmt.Errorf("timed out after %s (target may be filtering/rate-limiting probes): %w", toolExecutionTimeout, err)
		}
		e.logAudit(eng.ID, "tool.error", action.Target, map[string]any{"tool": action.Tool, "error": err.Error()})
		return fmt.Errorf("execute %s against %s: %w", tool.Name(), action.Target, err)
	}

	e.logAudit(eng.ID, "tool.invoke", action.Target, map[string]any{
		"tool":          action.Tool,
		"finding_count": len(result.Findings),
	})

	if len(result.Findings) == 0 {
		tree.AddNode("", tasktree.KindFinding, fmt.Sprintf("%s: topilma yo'q", action.Target), map[string]any{
			"target": action.Target,
			"tool":   action.Tool,
		})
		return nil
	}
	for _, f := range result.Findings {
		details := map[string]any{
			"target":   action.Target,
			"tool":     action.Tool,
			"severity": f.Severity,
			"detail":   f.Detail,
		}
		// TZ 4.5: every finding is checked against a proof-of-exploit
		// criterion before it's treated as confirmed. Unproven/
		// low-confidence findings still land in the task tree (so the
		// operator can review them), but carry that verdict for the
		// Report Engine (Bosqich 3) to filter on later.
		if e.validator != nil {
			verdict := e.validator.Evaluate(f)
			details["proven"] = verdict.Proven
			details["confidence"] = verdict.Confidence
			details["validation_reason"] = verdict.Reason
		}
		tree.AddNode("", tasktree.KindFinding, f.Title, details)
	}
	return nil
}

// errEngagementNotRunning is a sentinel used internally to abort an
// engagement.Store.Update call without persisting anything, when the
// engagement's on-disk status turns out not to be "running" after all.
var errEngagementNotRunning = fmt.Errorf("engagement is not running")

// Stop triggers the kill switch for a running engagement: any step in
// progress (including a pending HITL wait) is cancelled immediately.
//
// If no in-memory goroutine currently owns the engagement — e.g. the
// process restarted while it was mid-run, orphaning it — there is
// nothing left to cancel, but the persisted status would otherwise be
// stuck at "running" forever with no way for the operator to correct
// it. In that case Stop falls back to forcing the persisted status to
// "stopped" directly. Returns false only when the engagement was
// already in a terminal (non-running) state.
func (e *Engine) Stop(engagementID string) bool {
	e.mu.Lock()
	cancel, ok := e.stopped[engagementID]
	e.mu.Unlock()
	if ok {
		cancel()
		return true
	}

	if e.engStore == nil {
		return false
	}
	_, err := e.engStore.Update(engagementID, func(persisted *engagement.Engagement) error {
		if persisted.Status != engagement.StatusRunning {
			return errEngagementNotRunning
		}
		persisted.Status = engagement.StatusStopped
		persisted.StatusNote = "operator to'xtatdi (kill switch)"
		return nil
	})
	return err == nil
}

func (e *Engine) setStatus(eng *engagement.Engagement, status engagement.Status, note string) {
	eng.Status = status
	eng.StatusNote = note
	if e.engStore != nil {
		_, _ = e.engStore.Update(eng.ID, func(persisted *engagement.Engagement) error {
			persisted.Status = status
			persisted.StatusNote = note
			return nil
		})
	}
}

func (e *Engine) recordError(eng *engagement.Engagement, tree *tasktree.Tree, target, reason string) {
	tree.AddNode("", tasktree.KindError, reason, map[string]any{"target": target})
	e.persistTree(tree)
	e.setStatus(eng, engagement.StatusError, reason)
	e.logAudit(eng.ID, "engine.error", target, map[string]any{"reason": reason})
}

func (e *Engine) persistTree(tree *tasktree.Tree) {
	if e.treeStore == nil {
		return
	}
	_ = e.treeStore.Save(tree)
}

func (e *Engine) logAudit(engagementID, action, resource string, details map[string]any) {
	if e.audit == nil {
		return
	}
	_, _ = e.audit.Append("system", action, resource, mergeDetails(engagementID, details))
}

func mergeDetails(engagementID string, details map[string]any) map[string]any {
	out := map[string]any{"engagement_id": engagementID}
	for k, v := range details {
		out[k] = v
	}
	return out
}
