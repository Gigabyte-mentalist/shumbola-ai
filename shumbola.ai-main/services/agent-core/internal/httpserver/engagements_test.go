package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/shumbola/agent-core/internal/agentcore"
	"github.com/shumbola/agent-core/internal/audit"
	"github.com/shumbola/agent-core/internal/auth"
	"github.com/shumbola/agent-core/internal/engagement"
	"github.com/shumbola/agent-core/internal/hitl"
	"github.com/shumbola/agent-core/internal/tasktree"
	"github.com/shumbola/agent-core/internal/tools"
	"github.com/shumbola/agent-core/internal/validation"
)

// stubTool is a Tool double that always requires approval and returns a
// single canned finding with evidence, so the HTTP flow test doesn't
// need Docker.
type stubTool struct{ name string }

func (s stubTool) Name() string               { return s.name }
func (stubTool) RequiresApproval() bool       { return true }
func (stubTool) Validate(target string) error { return nil }
func (s stubTool) Execute(ctx context.Context, target string) (tools.Result, error) {
	return tools.Result{Findings: []tools.Finding{{
		Title:    s.name + " finding",
		Severity: tools.SeverityInfo,
		Detail:   "canned evidence for " + s.name,
	}}}, nil
}

func newFullTestServer(t *testing.T) http.Handler {
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
	approvals := hitl.NewStore()
	toolset := map[string]tools.Tool{
		"nmap":    stubTool{name: "nmap"},
		"smbenum": stubTool{name: "smbenum"},
		"nuclei":  stubTool{name: "nuclei"},
		"ffuf":    stubTool{name: "ffuf"},
		"testssl": stubTool{name: "testssl"},
	}
	engine := agentcore.NewEngine(toolset, &agentcore.StaticPlanner{}, approvals, auditLogger, engStore, treeStore, validation.NewValidator())

	keyStore := auth.NewInMemoryKeyStore(map[string]auth.Principal{
		"operator-key": {ID: "op-1", Role: auth.RoleOperator},
	})

	return New(Deps{
		KeyStore:    keyStore,
		AuditLogger: auditLogger,
		Engagements: engStore,
		Trees:       treeStore,
		Approvals:   approvals,
		Engine:      engine,
	})
}

func authed(req *http.Request) *http.Request {
	req.Header.Set(auth.APIKeyHeader, "operator-key")
	return req
}

func TestEngagementFullFlow(t *testing.T) {
	srv := newFullTestServer(t)

	// Create engagement with scope.
	body, _ := json.Marshal(map[string]any{"id": "eng-1", "targets": []string{"10.0.0.5"}})
	req := authed(httptest.NewRequest(http.MethodPost, "/engagements", bytes.NewReader(body)))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating engagement, got %d: %s", rec.Code, rec.Body.String())
	}

	// Start the engagement.
	req = authed(httptest.NewRequest(http.MethodPost, "/engagements/eng-1/start", nil))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 starting engagement, got %d: %s", rec.Code, rec.Body.String())
	}

	// StaticPlanner's default sequence is nmap, smbenum, nuclei, ffuf,
	// testssl — approve each one's HITL request in turn until done.
	for i := 0; i < 5; i++ {
		var approvalID string
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			req = authed(httptest.NewRequest(http.MethodGet, "/engagements/eng-1/approvals", nil))
			rec = httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			var approvals []map[string]any
			_ = json.Unmarshal(rec.Body.Bytes(), &approvals)
			if len(approvals) == 1 {
				approvalID = approvals[0]["id"].(string)
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if approvalID == "" {
			t.Fatalf("no pending approval appeared for step %d", i)
		}

		decision, _ := json.Marshal(map[string]bool{"approved": true})
		req = authed(httptest.NewRequest(http.MethodPost, "/engagements/eng-1/approvals/"+approvalID, bytes.NewReader(decision)))
		rec = httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 deciding approval %d, got %d: %s", i, rec.Code, rec.Body.String())
		}
	}

	// Poll until the engagement finishes, then check the task tree.
	deadline := time.Now().Add(2 * time.Second)
	var status string
	for time.Now().Before(deadline) {
		req = authed(httptest.NewRequest(http.MethodGet, "/engagements/eng-1", nil))
		rec = httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		var eng map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &eng)
		status, _ = eng["status"].(string)
		if status == "done" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if status != "done" {
		t.Fatalf("expected engagement status done after all 5 tools ran, got %q", status)
	}

	req = authed(httptest.NewRequest(http.MethodGet, "/engagements/eng-1/tasktree", nil))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var nodes []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &nodes)
	findingCount := 0
	for _, n := range nodes {
		if n["kind"] != "finding" {
			continue
		}
		findingCount++
		details, _ := n["details"].(map[string]any)
		if details == nil || details["confidence"] == nil || details["proven"] == nil {
			t.Fatalf("expected finding node to carry validator verdict, got details: %+v", details)
		}
	}
	if findingCount != 5 {
		t.Fatalf("expected 5 finding nodes (one per tool), got %d", findingCount)
	}
}

// TestStartEngagementRejectsDoubleStart covers a real symptom found in
// manual testing (2026-08-13): an engagement stuck "running" forever with
// an empty task tree and no pending approval, for reasons a direct LLM/
// RAG call couldn't reproduce. Starting an already-running engagement a
// second time (double-click, client retry) used to spawn a second
// Engine.Run goroutine racing the first over the same engagement/tree/
// approval-store state — nothing in that machinery is designed to have
// two runs in flight for one engagement at once. /start must reject it.
func TestStartEngagementRejectsDoubleStart(t *testing.T) {
	srv := newFullTestServer(t)

	body, _ := json.Marshal(map[string]any{"id": "eng-double", "targets": []string{"10.0.0.5"}})
	req := authed(httptest.NewRequest(http.MethodPost, "/engagements", bytes.NewReader(body)))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating engagement, got %d: %s", rec.Code, rec.Body.String())
	}

	start := func() int {
		req := authed(httptest.NewRequest(http.MethodPost, "/engagements/eng-double/start", nil))
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := start(); code != http.StatusAccepted {
		t.Fatalf("expected 202 on first start, got %d", code)
	}

	// The first run blocks on nmap's HITL approval (never decided in
	// this test) — poll until the engine has actually reached "running"
	// (the goroutine needs a moment to be scheduled) before racing it.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		req := authed(httptest.NewRequest(http.MethodGet, "/engagements/eng-double", nil))
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		var eng map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &eng)
		if eng["status"] == "running" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if code := start(); code != http.StatusConflict {
		t.Fatalf("expected 409 rejecting a second start while running, got %d", code)
	}
}

func TestListEngagementsReturnsEmptyArrayNotNull(t *testing.T) {
	srv := newFullTestServer(t)
	req := authed(httptest.NewRequest(http.MethodGet, "/engagements", nil))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "[]\n" {
		t.Fatalf("expected empty JSON array, got %q", rec.Body.String())
	}
}

func TestListEngagementsReturnsCreatedOnes(t *testing.T) {
	srv := newFullTestServer(t)

	for _, id := range []string{"eng-a", "eng-b"} {
		body, _ := json.Marshal(map[string]any{"id": id, "targets": []string{"10.0.0.5"}})
		req := authed(httptest.NewRequest(http.MethodPost, "/engagements", bytes.NewReader(body)))
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 creating %s, got %d", id, rec.Code)
		}
	}

	req := authed(httptest.NewRequest(http.MethodGet, "/engagements", nil))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var list []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 engagements, got %d", len(list))
	}
}

func TestListEngagementsRequiresAuth(t *testing.T) {
	srv := newFullTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/engagements", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}
}

func TestEngagementCreateRejectsEmptyScope(t *testing.T) {
	srv := newFullTestServer(t)
	body, _ := json.Marshal(map[string]any{"id": "eng-2", "targets": []string{}})
	req := authed(httptest.NewRequest(http.MethodPost, "/engagements", bytes.NewReader(body)))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty scope, got %d", rec.Code)
	}
}

func TestStopEngagementReturnsFalseWhenNotRunning(t *testing.T) {
	srv := newFullTestServer(t)
	req := authed(httptest.NewRequest(http.MethodPost, "/engagements/never-started/stop", nil))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]bool
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["stopped"] {
		t.Fatalf("expected stopped=false for an engagement that was never started")
	}
}
