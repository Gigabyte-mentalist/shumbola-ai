package guardrails

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shumbola/agent-core/internal/auth"
	"github.com/shumbola/agent-core/internal/engagement"
	"github.com/shumbola/agent-core/internal/roe"
)

// mockAdapter is a stand-in for a Bosqich 8 exploitation adapter,
// exercising the full FR-TEST checklist (shumbola_offensive_TZ.md 8-
// bo'lim) end to end against RunAuthorized without depending on any
// real exploitation tool: does dry-run work correctly, is HITL (here,
// the `confirmed` flag) unbypassable, does it stay inside scope, does
// the kill switch stop it, does rollback clean up.
type mockAdapter struct {
	artifactPath string
}

// plan is the dry-run: it only describes what would happen, and — the
// whole point of a dry run — creates no artifact and touches nothing.
func (m *mockAdapter) plan(_ context.Context, a Action) (Plan, error) {
	return Plan{Action: a, Summary: "would write a marker file simulating a proof-of-exploit artifact, then remove it"}, nil
}

// execute is the real action: it "does the exploit" (writes a file
// standing in for a proof artifact — e.g. a dropped test file a real
// sqlmap/Metasploit run might leave), registers its cleanup with rb, and
// respects ctx cancellation the same way agentcore.Engine's own kill
// switch handling does.
func (m *mockAdapter) execute(rb *Rollback) ExecuteFunc {
	return func(ctx context.Context, a Action) (Result, error) {
		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		default:
		}

		if err := os.WriteFile(m.artifactPath, []byte("proof-of-exploit marker"), 0o600); err != nil {
			return Result{}, err
		}
		rb.Register(func() error { return os.Remove(m.artifactPath) })

		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		default:
		}

		return Result{Verdict: VerdictProven, Detail: "marker written and verified", Evidence: "proof-of-exploit marker"}, nil
	}
}

func fullyApprovedSetup(t *testing.T, target string) (*engagement.Engagement, *roe.Document, auth.Principal) {
	t.Helper()
	now := time.Now().UTC()

	eng, err := engagement.New("eng-1", "op-1", []string{target})
	if err != nil {
		t.Fatalf("engagement.New: %v", err)
	}
	eng.Mode = engagement.ModeExploit
	eng.Scope.Authz = map[string]engagement.AuthzLevel{target: engagement.AuthzExploitAllowed}

	doc, err := roe.New("roe-1", eng.ID, "admin-1", now.Add(-time.Hour), now.Add(time.Hour), nil, nil, nil)
	if err != nil {
		t.Fatalf("roe.New: %v", err)
	}
	if err := doc.CounterApprove("admin-2"); err != nil {
		t.Fatalf("CounterApprove: %v", err)
	}

	return eng, doc, auth.Principal{ID: "op-1", Role: auth.RoleOperator}
}

// TestMockAdapterFullFlowPlanExecuteRollback covers: dry-run is
// side-effect-free, execution only happens after confirmation, and
// rollback cleans up the artifact execution left behind.
func TestMockAdapterFullFlowPlanExecuteRollback(t *testing.T) {
	target := "10.0.0.5"
	eng, doc, principal := fullyApprovedSetup(t, target)

	adapter := &mockAdapter{artifactPath: filepath.Join(t.TempDir(), "artifact.txt")}
	action := Action{Tool: "mock-exploit", Targets: []string{target}, Destructive: false, Description: "proof-of-exploit demo"}

	plan, err := adapter.plan(context.Background(), action)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if plan.Summary == "" {
		t.Fatalf("expected a non-empty dry-run summary for the operator to review")
	}
	if _, err := os.Stat(adapter.artifactPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry-run must not create any artifact")
	}

	var rb Rollback
	result, err := RunAuthorized(context.Background(), eng, doc, principal, action, 0, nil, true, adapter.execute(&rb))
	if err != nil {
		t.Fatalf("RunAuthorized: %v", err)
	}
	if result.Verdict != VerdictProven {
		t.Fatalf("expected VerdictProven, got %q", result.Verdict)
	}
	if _, err := os.Stat(adapter.artifactPath); err != nil {
		t.Fatalf("expected execute to have created the artifact: %v", err)
	}

	if errs := rb.Run(); len(errs) != 0 {
		t.Fatalf("rollback: expected no errors, got %v", errs)
	}
	if _, err := os.Stat(adapter.artifactPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected rollback to have removed the artifact")
	}
}

// TestMockAdapterRefusesWithoutConfirmation covers FR-EXPLOIT's "avval
// dry-run... keyin ikkinchi tasdiq bilan bajaradi": confirmed=false must
// never reach execute, regardless of how otherwise-authorized the
// action is.
func TestMockAdapterRefusesWithoutConfirmation(t *testing.T) {
	target := "10.0.0.5"
	eng, doc, principal := fullyApprovedSetup(t, target)
	adapter := &mockAdapter{artifactPath: filepath.Join(t.TempDir(), "artifact.txt")}
	action := Action{Tool: "mock-exploit", Targets: []string{target}}

	var rb Rollback
	if _, err := RunAuthorized(context.Background(), eng, doc, principal, action, 0, nil, false, adapter.execute(&rb)); err == nil {
		t.Fatalf("expected denial when confirmed=false")
	}
	if _, err := os.Stat(adapter.artifactPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("execute must never have run without confirmation")
	}
}

// TestMockAdapterStaysInsideScope covers FR-SAFE-2/FR-TEST: an action
// targeting a host outside the engagement's scope is denied before
// execute ever runs.
func TestMockAdapterStaysInsideScope(t *testing.T) {
	eng, doc, principal := fullyApprovedSetup(t, "10.0.0.5")
	adapter := &mockAdapter{artifactPath: filepath.Join(t.TempDir(), "artifact.txt")}
	outOfScope := Action{Tool: "mock-exploit", Targets: []string{"10.0.0.99"}}

	var rb Rollback
	if _, err := RunAuthorized(context.Background(), eng, doc, principal, outOfScope, 0, nil, true, adapter.execute(&rb)); err == nil {
		t.Fatalf("expected denial for a target outside engagement scope")
	}
	if _, err := os.Stat(adapter.artifactPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("execute must never have run for an out-of-scope target")
	}
}

// TestMockAdapterKillSwitchStopsExecution covers FR-SAFE-5: a context
// cancelled before execute runs (simulating the kill switch firing
// mid-exploitation, the same mechanism agentcore.Engine already uses)
// must stop the adapter before it does anything, and never report
// success.
func TestMockAdapterKillSwitchStopsExecution(t *testing.T) {
	target := "10.0.0.5"
	eng, doc, principal := fullyApprovedSetup(t, target)
	adapter := &mockAdapter{artifactPath: filepath.Join(t.TempDir(), "artifact.txt")}
	action := Action{Tool: "mock-exploit", Targets: []string{target}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // kill switch already fired before execute is reached

	var rb Rollback
	result, err := RunAuthorized(ctx, eng, doc, principal, action, 0, nil, true, adapter.execute(&rb))
	if err == nil {
		t.Fatalf("expected an error when the context is already cancelled")
	}
	if result.Verdict == VerdictProven {
		t.Fatalf("a cancelled action must never report VerdictProven")
	}
	if _, statErr := os.Stat(adapter.artifactPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("a cancelled action must never have created the artifact")
	}
}
