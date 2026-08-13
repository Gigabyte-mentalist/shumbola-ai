package guardrails

import (
	"context"
	"fmt"
	"time"

	"github.com/shumbola/agent-core/internal/auth"
	"github.com/shumbola/agent-core/internal/engagement"
	"github.com/shumbola/agent-core/internal/exploitgate"
	"github.com/shumbola/agent-core/internal/roe"
)

// RunAuthorized is the single entry point a Bosqich 8 exploitation
// adapter is expected to call to actually run an Action — deliberately
// the only path to invoking exec, so there is no way for an adapter to
// reach real execution without passing every gate (FR-SAFE-1: "bu
// gate'ni o'chiradigan konfiguratsiya kaliti mavjud emas").
//
// It requires, in order: confirmed (the operator's second, independent
// approval after reviewing a Plan — see plan.go — refused outright if
// false), exploitgate.Authorize for every target in the action (FR-AUTHZ:
// RoE validity, two-person rule, per-target authorization level,
// engagement mode), and Evaluate (FR-SAFE: blast-radius, the
// destructive-action block, rate limiting). Only once all of that
// passes does exec ever run.
func RunAuthorized(
	ctx context.Context,
	eng *engagement.Engagement,
	doc *roe.Document,
	principal auth.Principal,
	a Action,
	blastLimit int,
	limiter *RateLimiter,
	confirmed bool,
	exec ExecuteFunc,
) (Result, error) {
	if !confirmed {
		return Result{}, fmt.Errorf("guardrails: action was not confirmed after dry-run")
	}

	now := time.Now().UTC()
	for _, target := range a.Targets {
		if err := exploitgate.Authorize(eng, doc, target, principal, now); err != nil {
			return Result{}, err
		}
	}
	if err := Evaluate(a, doc, blastLimit, limiter, now); err != nil {
		return Result{}, err
	}

	return exec(ctx, a)
}
