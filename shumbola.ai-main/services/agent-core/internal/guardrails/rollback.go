package guardrails

import "sync"

// CleanupFunc undoes one artifact or side effect an exploitation action
// left behind (a dropped test file, an open session, ...).
type CleanupFunc func() error

// Rollback accumulates cleanup funcs an adapter registers while an
// Action runs, then executes all of them afterward (FR-SAFE-6). Zero
// value is ready to use.
type Rollback struct {
	mu    sync.Mutex
	funcs []CleanupFunc
}

// Register adds fn to the set of cleanups Run will execute. Safe to call
// from concurrent goroutines (an adapter may leave artifacts from
// several concurrent sub-steps).
func (r *Rollback) Register(fn CleanupFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.funcs = append(r.funcs, fn)
}

// Run executes every registered cleanup, in reverse registration order
// (LIFO, mirroring defer — the most recently created artifact is the
// one most likely to depend on nothing created after it, so undoing
// newest-first is safest). It continues past individual failures rather
// than stopping at the first one, so one broken cleanup never blocks the
// rest, and returns every error encountered. A non-empty result is
// exactly the "qizil ogohlantirish" (red warning) FR-SAFE-6 calls for —
// the caller must surface it to the operator, not swallow it.
func (r *Rollback) Run() []error {
	r.mu.Lock()
	funcs := make([]CleanupFunc, len(r.funcs))
	copy(funcs, r.funcs)
	r.mu.Unlock()

	var errs []error
	for i := len(funcs) - 1; i >= 0; i-- {
		if err := funcs[i](); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}
