package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/shumbola/agent-core/internal/audit"
	"github.com/shumbola/agent-core/internal/auth"
	"github.com/shumbola/agent-core/internal/engagement"
	"github.com/shumbola/agent-core/internal/roe"
)

func newRoETestServer(t *testing.T) http.Handler {
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
	roeStore, err := roe.NewFileStore(filepath.Join(dir, "roe"))
	if err != nil {
		t.Fatalf("roe.NewFileStore: %v", err)
	}

	keyStore := auth.NewInMemoryKeyStore(map[string]auth.Principal{
		"operator-key": {ID: "op-1", Role: auth.RoleOperator},
		"admin-1-key":  {ID: "admin-1", Role: auth.RoleAdministrator},
		"admin-2-key":  {ID: "admin-2", Role: auth.RoleAdministrator},
		"auditor-key":  {ID: "aud-1", Role: auth.RoleAuditor},
	})

	return New(Deps{
		KeyStore:    keyStore,
		AuditLogger: auditLogger,
		Engagements: engStore,
		RoEStore:    roeStore,
	})
}

func asKey(req *http.Request, key string) *http.Request {
	req.Header.Set(auth.APIKeyHeader, key)
	return req
}

// createTestEngagement creates an engagement (as the operator) and
// returns its ID, so RoE-focused tests don't repeat this boilerplate.
func createTestEngagement(t *testing.T, srv http.Handler, id string) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"id": id, "targets": []string{"10.0.0.5"}})
	req := asKey(httptest.NewRequest(http.MethodPost, "/engagements", bytes.NewReader(body)), "operator-key")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating engagement, got %d: %s", rec.Code, rec.Body.String())
	}
}

func uploadRoE(t *testing.T, srv http.Handler, engagementID, asAdminKey string, from, until time.Time) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"valid_from":         from,
		"valid_until":        until,
		"prohibited_actions": []string{"no data deletion"},
		"contacts":           []string{"security@client.example"},
	})
	req := asKey(httptest.NewRequest(http.MethodPost, "/engagements/"+engagementID+"/roe", bytes.NewReader(body)), asAdminKey)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func TestRoEUploadRequiresAdministrator(t *testing.T) {
	srv := newRoETestServer(t)
	createTestEngagement(t, srv, "eng-1")

	rec := uploadRoE(t, srv, "eng-1", "operator-key", time.Now(), time.Now().Add(time.Hour))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for an Operator uploading a RoE, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRoEUploadAndGet(t *testing.T) {
	srv := newRoETestServer(t)
	createTestEngagement(t, srv, "eng-1")

	from := time.Now().UTC()
	rec := uploadRoE(t, srv, "eng-1", "admin-1-key", from, from.Add(time.Hour))
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 uploading RoE, got %d: %s", rec.Code, rec.Body.String())
	}

	req := asKey(httptest.NewRequest(http.MethodGet, "/engagements/eng-1/roe", nil), "auditor-key")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for Auditor reading RoE, got %d: %s", rec.Code, rec.Body.String())
	}
	var doc roe.Document
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if doc.ApprovedBy != "admin-1" {
		t.Fatalf("expected ApprovedBy admin-1, got %q", doc.ApprovedBy)
	}
}

func TestRoECounterApproveBySameAdministratorFails(t *testing.T) {
	srv := newRoETestServer(t)
	createTestEngagement(t, srv, "eng-1")
	from := time.Now().UTC()
	uploadRoE(t, srv, "eng-1", "admin-1-key", from, from.Add(time.Hour))

	req := asKey(httptest.NewRequest(http.MethodPost, "/engagements/eng-1/roe/counter-approve", nil), "admin-1-key")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 when the same Administrator tries to counter-approve their own RoE, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRoECounterApproveByDifferentAdministratorSucceeds(t *testing.T) {
	srv := newRoETestServer(t)
	createTestEngagement(t, srv, "eng-1")
	from := time.Now().UTC()
	uploadRoE(t, srv, "eng-1", "admin-1-key", from, from.Add(time.Hour))

	req := asKey(httptest.NewRequest(http.MethodPost, "/engagements/eng-1/roe/counter-approve", nil), "admin-2-key")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var doc roe.Document
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !doc.TwoPersonSatisfied() {
		t.Fatalf("expected two-person rule satisfied after a distinct Administrator counter-approved")
	}
}

func setMode(t *testing.T, srv http.Handler, engagementID, asKeyName string, mode engagement.Mode) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"mode": mode})
	req := asKey(httptest.NewRequest(http.MethodPost, "/engagements/"+engagementID+"/mode", bytes.NewReader(body)), asKeyName)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func TestSetModeExploitDeniedWithoutRoE(t *testing.T) {
	srv := newRoETestServer(t)
	createTestEngagement(t, srv, "eng-1")

	rec := setMode(t, srv, "eng-1", "operator-key", engagement.ModeExploit)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 requesting exploit mode with no RoE on file, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSetModeExploitDeniedWithoutTwoPersonApproval(t *testing.T) {
	srv := newRoETestServer(t)
	createTestEngagement(t, srv, "eng-1")
	from := time.Now().UTC()
	uploadRoE(t, srv, "eng-1", "admin-1-key", from, from.Add(time.Hour))
	// No counter-approval.

	rec := setMode(t, srv, "eng-1", "operator-key", engagement.ModeExploit)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 requesting exploit mode without two-person approval, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSetModeExploitSucceedsAfterFullApproval(t *testing.T) {
	srv := newRoETestServer(t)
	createTestEngagement(t, srv, "eng-1")
	from := time.Now().UTC()
	uploadRoE(t, srv, "eng-1", "admin-1-key", from, from.Add(time.Hour))

	req := asKey(httptest.NewRequest(http.MethodPost, "/engagements/eng-1/roe/counter-approve", nil), "admin-2-key")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("counter-approve: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = setMode(t, srv, "eng-1", "operator-key", engagement.ModeExploit)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 requesting exploit mode after full RoE approval, got %d: %s", rec.Code, rec.Body.String())
	}
	var eng engagement.Engagement
	if err := json.Unmarshal(rec.Body.Bytes(), &eng); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if eng.Mode != engagement.ModeExploit {
		t.Fatalf("expected engagement.Mode ModeExploit, got %q", eng.Mode)
	}
}

func TestSetModeForbiddenForAuditor(t *testing.T) {
	srv := newRoETestServer(t)
	createTestEngagement(t, srv, "eng-1")

	rec := setMode(t, srv, "eng-1", "auditor-key", engagement.ModeExploit)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for an Auditor requesting mode change, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSetScopeAuthzUpdatesTarget(t *testing.T) {
	srv := newRoETestServer(t)
	createTestEngagement(t, srv, "eng-1")

	body, _ := json.Marshal(map[string]any{"target": "10.0.0.5", "level": engagement.AuthzExploitAllowed})
	req := asKey(httptest.NewRequest(http.MethodPatch, "/engagements/eng-1/scope-authz", bytes.NewReader(body)), "operator-key")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var eng engagement.Engagement
	if err := json.Unmarshal(rec.Body.Bytes(), &eng); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if eng.Scope.AuthzFor("10.0.0.5") != engagement.AuthzExploitAllowed {
		t.Fatalf("expected 10.0.0.5 to be AuthzExploitAllowed, got %q", eng.Scope.AuthzFor("10.0.0.5"))
	}
}

func TestSetScopeAuthzRejectsTargetOutsideScope(t *testing.T) {
	srv := newRoETestServer(t)
	createTestEngagement(t, srv, "eng-1")

	body, _ := json.Marshal(map[string]any{"target": "10.0.0.99", "level": engagement.AuthzExploitAllowed})
	req := asKey(httptest.NewRequest(http.MethodPatch, "/engagements/eng-1/scope-authz", bytes.NewReader(body)), "operator-key")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a target not in scope, got %d: %s", rec.Code, rec.Body.String())
	}
}
