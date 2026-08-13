// Package agentcore implements the ReAct loop (TZ 4.2): observe state,
// decide the next action, act through a Tool, observe the result, update
// the Task Tree, repeat.
package agentcore

import (
	"context"

	"github.com/shumbola/agent-core/internal/engagement"
	"github.com/shumbola/agent-core/internal/tasktree"
)

// Action is a single step the Planner wants the Engine to take: run Tool
// against Target. Reason is an optional human-readable justification
// (populated by LLMPlanner from the model's own explanation; StaticPlanner
// leaves it empty) shown to the operator at the HITL gate and recorded in
// the audit log, so an LLM-driven decision isn't a black box.
type Action struct {
	Tool   string
	Target string
	Reason string
}

// PlannerState is the information a Planner needs to decide the next
// Action. It intentionally exposes the engagement and task tree directly
// rather than a narrowed view — Bosqich 1's StaticPlanner and the future
// LLMPlanner both need the full picture (scope, history) to reason about
// "what's left to do".
type PlannerState struct {
	Engagement *engagement.Engagement
	Tree       *tasktree.Tree
}

// Planner decides the next ReAct action given the current state. Decide
// returns done=true when the Planner considers the engagement's current
// objective complete (TZ 4.2's "to'xtash sharti: vazifa yakunlanganda").
//
// Bosqich 1 ships StaticPlanner, a deterministic recon policy that needs
// no model. A future LLMPlanner (Bosqich 4) implements the same
// interface by prompting vLLM/Qwen with the state and parsing its tool
// choice — Engine, HITL, and the task tree do not change when that lands.
type Planner interface {
	Decide(ctx context.Context, state PlannerState) (action Action, done bool, err error)
}
