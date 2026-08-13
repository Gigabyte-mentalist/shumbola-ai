package guardrails

import "context"

// Plan is what an adapter's dry-run produces — the operator-facing
// description of exactly what an Action would do, before anything runs
// for real. FR-EXPLOIT requires every adapter to run dry-run first and
// show the operator this before a second, separate confirmation lets
// Execute happen.
type Plan struct {
	Action  Action
	Summary string // shown verbatim in the HITL approval prompt
}

// PlanFunc is an adapter's dry-run implementation. It must have no side
// effect on the target — that's the entire point of a dry run — and
// guardrails has no way to enforce that beyond this doc comment; each
// Bosqich 8 adapter is responsible for actually implementing "check"/
// "--dry-run" mode using its underlying tool's own support for it.
type PlanFunc func(ctx context.Context, a Action) (Plan, error)

// ExecuteFunc is an adapter's real execution, called only after Plan has
// produced a Plan and a second, independent confirmation has approved
// it (mirrors agentcore's own HITL gate, at the exploitation layer).
type ExecuteFunc func(ctx context.Context, a Action) (Result, error)

// Result is the deterministic, normalized outcome FR-EXPLOIT requires
// every adapter to report — never a tool's raw, adapter-specific output
// shape.
type Result struct {
	Verdict Verdict
	Detail  string
	// Evidence is the minimal proof collected (FR-REPORT: "faqat
	// zararsiz dalil" — e.g. a single non-destructive marker string, not
	// a full data dump). Left for the report layer (not yet built) to
	// consume and mask as needed.
	Evidence string
}

// Verdict is an exploitation attempt's normalized result.
type Verdict string

const (
	VerdictProven       Verdict = "proven"
	VerdictFailed       Verdict = "failed"
	VerdictInconclusive Verdict = "inconclusive"
)
