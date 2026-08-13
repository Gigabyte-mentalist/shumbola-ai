package httpserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shumbola/agent-core/internal/audit"
	"github.com/shumbola/agent-core/internal/auth"
)

func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	srv, _ := newTestServerWithAuditPath(t)
	return srv
}

func newTestServerWithAuditPath(t *testing.T) (http.Handler, string) {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "audit.log")
	logger, err := audit.NewLogger(logPath)
	if err != nil {
		t.Fatalf("audit.NewLogger: %v", err)
	}
	t.Cleanup(func() { _ = logger.Close() })

	store := auth.NewInMemoryKeyStore(map[string]auth.Principal{
		"operator-key": {ID: "op-1", Role: auth.RoleOperator},
	})
	return New(Deps{KeyStore: store, AuditLogger: logger}), logPath
}

func TestHealthzIsPublic(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /healthz without auth, got %d", rec.Code)
	}
}

func TestProtectedRouteRequiresAuth(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/engagements", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for protected route without auth, got %d", rec.Code)
	}
}

// TestAuditLogRecordsRealActor guards against a bug where the audit
// entry for an authenticated request was written with actor "anonymous"
// because the audit wrapper read the pre-auth *http.Request's context
// instead of the one auth.Middleware attached the Principal to.
func TestAuditLogRecordsRealActor(t *testing.T) {
	srv, logPath := newTestServerWithAuditPath(t)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	req.Header.Set(auth.APIKeyHeader, "operator-key")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile audit log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	last := lines[len(lines)-1]
	if !strings.Contains(last, `"actor":"op-1"`) {
		t.Fatalf("expected audit entry to record actor op-1, got: %s", last)
	}
}

func TestCORSPreflightBypassesAuth(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodOptions, "/engagements", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for CORS preflight (no auth required), got %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("expected Access-Control-Allow-Origin header, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSHeaderPresentOnRealResponse(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("expected Access-Control-Allow-Origin on a real response too, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestAuditLogRecordsAnonymousForRejectedAuth(t *testing.T) {
	srv, logPath := newTestServerWithAuditPath(t)

	req := httptest.NewRequest(http.MethodGet, "/engagements", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile audit log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	last := lines[len(lines)-1]
	if !strings.Contains(last, `"actor":"anonymous"`) {
		t.Fatalf("expected rejected auth attempt to still be audit-logged as anonymous, got: %s", last)
	}
}
