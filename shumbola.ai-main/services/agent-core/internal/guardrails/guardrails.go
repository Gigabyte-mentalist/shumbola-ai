// Package guardrails implements shumbola_offensive_TZ.md's FR-SAFE
// safety controls (Bosqich 7): blast-radius limiting, the destructive-
// action block, a dry-run/Plan scaffold, a per-target rate limiter, and
// a rollback registry. It covers a different concern than exploitgate
// (FR-AUTHZ — who is allowed to act at all): guardrails governs how
// safely an already-authorized action may run. A Bosqich 8 adapter must
// pass both this package's Evaluate and exploitgate.Authorize before
// acting — neither substitutes for the other.
package guardrails

import (
	"fmt"
	"sync"
	"time"

	"github.com/shumbola/agent-core/internal/roe"
)

// DefaultBlastRadius is FR-SAFE-3's default limit on how many targets a
// single exploitation action may touch.
const DefaultBlastRadius = 1

// Action describes one exploitation step a Bosqich 8 adapter intends to
// perform — guardrails' unit of evaluation, independent of which
// underlying tool (sqlmap, Metasploit RPC, ...) implements it.
type Action struct {
	// Tool identifies the adapter/module (e.g. "sqlmap", "msf-check").
	// Also doubles as the identifier CheckDestructive matches against
	// roe.Document.AllowedDestructiveActions.
	Tool string
	// Targets are every host this single action would touch. Almost
	// always exactly one — see CheckBlastRadius.
	Targets []string
	// Destructive is true if the action could modify/delete data, cause
	// denial of service, or stop a running service (FR-SAFE-4). An
	// adapter must self-declare this honestly; guardrails has no way to
	// infer it from Tool/Targets alone.
	Destructive bool
	// Description is the human-readable summary shown to the operator
	// during dry-run/HITL — what will actually happen, not just which
	// tool runs.
	Description string
}

// CheckBlastRadius rejects an action touching more targets than limit
// allows (FR-SAFE-3). limit <= 0 is treated as DefaultBlastRadius, never
// as "no limit" — there is no way to disable this check via input.
func CheckBlastRadius(a Action, limit int) error {
	if limit <= 0 {
		limit = DefaultBlastRadius
	}
	if len(a.Targets) == 0 {
		return fmt.Errorf("guardrails: action has no targets")
	}
	if len(a.Targets) > limit {
		return fmt.Errorf("guardrails: action touches %d targets, exceeding the blast-radius limit of %d", len(a.Targets), limit)
	}
	return nil
}

// CheckDestructive rejects a destructive action unless it is explicitly
// named in doc's AllowedDestructiveActions (FR-SAFE-4). A non-destructive
// action always passes; a nil doc never permits a destructive one.
func CheckDestructive(a Action, doc *roe.Document) error {
	if !a.Destructive {
		return nil
	}
	if !doc.AllowsDestructive(a.Tool) {
		return fmt.Errorf("guardrails: destructive action %q is not explicitly permitted by the RoE", a.Tool)
	}
	return nil
}

// RateLimiter enforces FR-SAFE-7's per-target throttle: no two actions
// against the same target closer together than minGap. Zero value is
// not usable — construct with NewRateLimiter.
type RateLimiter struct {
	mu       sync.Mutex
	minGap   time.Duration
	lastCall map[string]time.Time
}

// NewRateLimiter builds a RateLimiter enforcing at least minGap between
// actions against the same target.
func NewRateLimiter(minGap time.Duration) *RateLimiter {
	return &RateLimiter{minGap: minGap, lastCall: make(map[string]time.Time)}
}

// Allow reports whether target may be acted on at now — and if so,
// records now as the target's last call time, so back-to-back calls in
// a single evaluation (e.g. from CheckAll's loop over Action.Targets)
// correctly rate-limit each other, not just calls from different
// actions.
func (r *RateLimiter) Allow(target string, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	last, ok := r.lastCall[target]
	if ok && now.Sub(last) < r.minGap {
		return false
	}
	r.lastCall[target] = now
	return true
}

// Evaluate runs every FR-SAFE safety check this package is responsible
// for — blast-radius, the destructive-action block, and (if limiter is
// non-nil) the rate limit — and returns the first failure. limiter may
// be nil to skip rate limiting (e.g. in a dry-run that must not record
// a real call).
func Evaluate(a Action, doc *roe.Document, blastLimit int, limiter *RateLimiter, now time.Time) error {
	if err := CheckBlastRadius(a, blastLimit); err != nil {
		return err
	}
	if err := CheckDestructive(a, doc); err != nil {
		return err
	}
	if limiter != nil {
		for _, t := range a.Targets {
			if !limiter.Allow(t, now) {
				return fmt.Errorf("guardrails: rate limit exceeded for target %q", t)
			}
		}
	}
	return nil
}
