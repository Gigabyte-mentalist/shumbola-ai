package auth

import (
	"context"
	"net/http"
)

type contextKey int

const (
	principalContextKey contextKey = iota
	actorRecorderContextKey
)

// APIKeyHeader is the HTTP header operators present their security key in.
const APIKeyHeader = "X-API-Key"

// ActorRecorder captures the authenticated actor ID for a request so code
// wrapping Middleware from the *outside* (e.g. audit logging, which needs
// to run outside auth so it can also log rejected/unauthenticated
// attempts) can still learn who Middleware authenticated. This is needed
// because Middleware attaches the Principal via r.WithContext, which
// returns a new *http.Request rather than mutating the one the outer
// wrapper is holding — an outer wrapper reading r.Context() after
// ServeHTTP returns would otherwise never see it.
type ActorRecorder struct {
	actor string
}

// NewActorRecorderContext returns a context carrying a fresh
// ActorRecorder, and the recorder itself so the caller can read it after
// the request has been handled.
func NewActorRecorderContext(ctx context.Context) (context.Context, *ActorRecorder) {
	rec := &ActorRecorder{}
	return context.WithValue(ctx, actorRecorderContextKey, rec), rec
}

// Actor returns the recorded actor ID, or "" if none was recorded (e.g.
// authentication never succeeded for this request).
func (r *ActorRecorder) Actor() string {
	if r == nil {
		return ""
	}
	return r.actor
}

// Middleware authenticates every request against store via the
// X-API-Key header and attaches the resolved Principal to the request
// context. Requests without a valid key are rejected with 401.
func Middleware(store KeyStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, err := Authenticate(store, r.Header.Get(APIKeyHeader))
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if rec, ok := r.Context().Value(actorRecorderContextKey).(*ActorRecorder); ok {
				rec.actor = principal.ID
			}
			ctx := context.WithValue(r.Context(), principalContextKey, principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole wraps a handler so it only proceeds if the authenticated
// principal (set by Middleware) has one of the allowed roles.
func RequireRole(next http.Handler, allowed ...Role) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := FromContext(r.Context())
		if !ok || !HasRole(principal, allowed...) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// FromContext retrieves the Principal attached by Middleware, if any.
func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalContextKey).(Principal)
	return p, ok
}
