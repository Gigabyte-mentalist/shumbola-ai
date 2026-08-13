package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func testStore() *InMemoryKeyStore {
	return NewInMemoryKeyStore(map[string]Principal{
		"operator-raw-key": {ID: "op-1", Role: RoleOperator},
		"admin-raw-key":    {ID: "admin-1", Role: RoleAdministrator},
	})
}

func TestAuthenticateValidKey(t *testing.T) {
	store := testStore()
	p, err := Authenticate(store, "operator-raw-key")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if p.ID != "op-1" || p.Role != RoleOperator {
		t.Fatalf("unexpected principal: %+v", p)
	}
}

func TestAuthenticateInvalidKey(t *testing.T) {
	store := testStore()
	if _, err := Authenticate(store, "wrong-key"); err == nil {
		t.Fatalf("expected error for invalid key")
	}
	if _, err := Authenticate(store, ""); err == nil {
		t.Fatalf("expected error for empty key")
	}
}

func TestMiddlewareRejectsMissingKey(t *testing.T) {
	store := testStore()
	handler := Middleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestMiddlewareAttachesPrincipal(t *testing.T) {
	store := testStore()
	var gotOK bool
	var gotPrincipal Principal
	handler := Middleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPrincipal, gotOK = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(APIKeyHeader, "operator-raw-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !gotOK || gotPrincipal.Role != RoleOperator {
		t.Fatalf("expected operator principal in context, got ok=%v principal=%+v", gotOK, gotPrincipal)
	}
}

func TestRequireRoleForbidsWrongRole(t *testing.T) {
	store := testStore()
	inner := RequireRole(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), RoleAdministrator)
	handler := Middleware(store)(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(APIKeyHeader, "operator-raw-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestRequireRoleAllowsMatchingRole(t *testing.T) {
	store := testStore()
	inner := RequireRole(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), RoleOperator, RoleAdministrator)
	handler := Middleware(store)(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(APIKeyHeader, "admin-raw-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
