package guardrails

import (
	"testing"
	"time"

	"github.com/shumbola/agent-core/internal/roe"
)

func TestCheckBlastRadiusDefaultsToOne(t *testing.T) {
	a := Action{Tool: "sqlmap", Targets: []string{"10.0.0.5", "10.0.0.6"}}
	if err := CheckBlastRadius(a, 0); err == nil {
		t.Fatalf("expected denial: 2 targets exceeds the default blast-radius limit of 1")
	}
	single := Action{Tool: "sqlmap", Targets: []string{"10.0.0.5"}}
	if err := CheckBlastRadius(single, 0); err != nil {
		t.Fatalf("expected a single-target action to pass the default limit, got: %v", err)
	}
}

func TestCheckBlastRadiusRejectsEmptyTargets(t *testing.T) {
	if err := CheckBlastRadius(Action{Tool: "sqlmap"}, 1); err == nil {
		t.Fatalf("expected denial for an action with no targets")
	}
}

func TestCheckBlastRadiusRespectsExplicitLimit(t *testing.T) {
	a := Action{Tool: "sqlmap", Targets: []string{"10.0.0.5", "10.0.0.6", "10.0.0.7"}}
	if err := CheckBlastRadius(a, 3); err != nil {
		t.Fatalf("expected 3 targets to pass an explicit limit of 3, got: %v", err)
	}
	if err := CheckBlastRadius(a, 2); err == nil {
		t.Fatalf("expected 3 targets to fail an explicit limit of 2")
	}
}

func TestCheckDestructiveAllowsNonDestructiveRegardlessOfRoE(t *testing.T) {
	a := Action{Tool: "nuclei-exploit", Destructive: false}
	if err := CheckDestructive(a, nil); err != nil {
		t.Fatalf("expected a non-destructive action to pass even with a nil RoE, got: %v", err)
	}
}

func TestCheckDestructiveDeniesWithoutExplicitAllowance(t *testing.T) {
	a := Action{Tool: "msf-exploit", Destructive: true}
	if err := CheckDestructive(a, nil); err == nil {
		t.Fatalf("expected denial for a destructive action with a nil RoE")
	}

	from := time.Now().UTC()
	doc, err := roe.New("roe-1", "eng-1", "admin-1", from, from.Add(time.Hour), nil, nil, []string{"some-other-tool"})
	if err != nil {
		t.Fatalf("roe.New: %v", err)
	}
	if err := CheckDestructive(a, doc); err == nil {
		t.Fatalf("expected denial: RoE allows a different tool, not %q", a.Tool)
	}
}

func TestCheckDestructiveAllowsExplicitlyPermittedAction(t *testing.T) {
	from := time.Now().UTC()
	doc, err := roe.New("roe-1", "eng-1", "admin-1", from, from.Add(time.Hour), nil, nil, []string{"msf-exploit"})
	if err != nil {
		t.Fatalf("roe.New: %v", err)
	}
	a := Action{Tool: "msf-exploit", Destructive: true}
	if err := CheckDestructive(a, doc); err != nil {
		t.Fatalf("expected the explicitly-allowed destructive action to pass, got: %v", err)
	}
}

func TestRateLimiterBlocksWithinMinGap(t *testing.T) {
	rl := NewRateLimiter(time.Minute)
	now := time.Now()

	if !rl.Allow("10.0.0.5", now) {
		t.Fatalf("expected the first call for a target to be allowed")
	}
	if rl.Allow("10.0.0.5", now.Add(10*time.Second)) {
		t.Fatalf("expected a call within minGap to be denied")
	}
	if !rl.Allow("10.0.0.5", now.Add(2*time.Minute)) {
		t.Fatalf("expected a call after minGap has elapsed to be allowed")
	}
}

func TestRateLimiterTracksTargetsIndependently(t *testing.T) {
	rl := NewRateLimiter(time.Minute)
	now := time.Now()
	if !rl.Allow("10.0.0.5", now) {
		t.Fatalf("expected first call for 10.0.0.5 to be allowed")
	}
	if !rl.Allow("10.0.0.6", now) {
		t.Fatalf("expected first call for a different target to be allowed regardless of 10.0.0.5's state")
	}
}

func TestEvaluateComposesAllChecks(t *testing.T) {
	now := time.Now().UTC()
	doc, err := roe.New("roe-1", "eng-1", "admin-1", now.Add(-time.Hour), now.Add(time.Hour), nil, nil, nil)
	if err != nil {
		t.Fatalf("roe.New: %v", err)
	}
	rl := NewRateLimiter(time.Minute)

	ok := Action{Tool: "nuclei-exploit", Targets: []string{"10.0.0.5"}, Destructive: false}
	if err := Evaluate(ok, doc, 0, rl, now); err != nil {
		t.Fatalf("expected a valid non-destructive single-target action to pass, got: %v", err)
	}

	// Same target again, immediately — rate limit should now reject it.
	if err := Evaluate(ok, doc, 0, rl, now.Add(time.Second)); err == nil {
		t.Fatalf("expected the rate limiter to reject a second call to the same target within minGap")
	}

	destructive := Action{Tool: "msf-exploit", Targets: []string{"10.0.0.6"}, Destructive: true}
	if err := Evaluate(destructive, doc, 0, rl, now); err == nil {
		t.Fatalf("expected denial for a destructive action the RoE doesn't explicitly allow")
	}

	tooManyTargets := Action{Tool: "nuclei-exploit", Targets: []string{"10.0.0.7", "10.0.0.8"}, Destructive: false}
	if err := Evaluate(tooManyTargets, doc, 0, rl, now); err == nil {
		t.Fatalf("expected denial for an action exceeding the default blast radius")
	}
}

func TestRollbackRunsAllCleanupsInReverseOrder(t *testing.T) {
	var order []int
	var rb Rollback
	rb.Register(func() error { order = append(order, 1); return nil })
	rb.Register(func() error { order = append(order, 2); return nil })
	rb.Register(func() error { order = append(order, 3); return nil })

	if errs := rb.Run(); len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
	want := []int{3, 2, 1}
	if len(order) != len(want) {
		t.Fatalf("expected %d cleanups to run, got %d", len(want), len(order))
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("expected LIFO cleanup order %v, got %v", want, order)
		}
	}
}

func TestRollbackContinuesPastIndividualFailures(t *testing.T) {
	failing := errFake("cleanup failed")
	var ran []int
	var rb Rollback
	rb.Register(func() error { ran = append(ran, 1); return nil })
	rb.Register(func() error { ran = append(ran, 2); return failing })
	rb.Register(func() error { ran = append(ran, 3); return nil })

	errs := rb.Run()
	if len(errs) != 1 {
		t.Fatalf("expected exactly 1 error surfaced, got %d: %v", len(errs), errs)
	}
	if len(ran) != 3 {
		t.Fatalf("expected all 3 cleanups to run despite the middle one failing, got %v", ran)
	}
}

type errFake string

func (e errFake) Error() string { return string(e) }
