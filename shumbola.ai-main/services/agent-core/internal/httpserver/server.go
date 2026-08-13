// Package httpserver wires the agent-core HTTP API: auth/RBAC middleware,
// audit logging of every request, and the routes themselves. Bosqich 0
// only exposes a health check — engagement/scope/HITL endpoints land in
// Bosqich 1+.
package httpserver

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/shumbola/agent-core/internal/agentcore"
	"github.com/shumbola/agent-core/internal/audit"
	"github.com/shumbola/agent-core/internal/auth"
	"github.com/shumbola/agent-core/internal/engagement"
	"github.com/shumbola/agent-core/internal/exploit"
	"github.com/shumbola/agent-core/internal/guardrails"
	"github.com/shumbola/agent-core/internal/hitl"
	"github.com/shumbola/agent-core/internal/roe"
	"github.com/shumbola/agent-core/internal/tasktree"
)

// Deps are the dependencies the HTTP API needs. Engagements/Trees/
// Approvals/Engine are nil-able so Bosqich-0-only callers (e.g. tests
// that only care about /healthz and auth) can keep constructing a
// minimal server.
type Deps struct {
	KeyStore    auth.KeyStore
	AuditLogger *audit.Logger
	Engagements *engagement.FileStore
	Trees       *tasktree.FileStore
	Approvals   *hitl.Store
	Engine      *agentcore.Engine
	// RoEStore backs the FR-AUTHZ endpoints (shumbola_offensive_TZ.md,
	// Bosqich 6). Nil-able like the others above — a server built
	// without one simply doesn't expose the /roe, /mode, or
	// /scope-authz routes.
	RoEStore roe.Store
	// ExploitNucleiCheck/ExploitLimiter back the FR-EXPLOIT-2 endpoint
	// (Bosqich 8). Both nil-able — without ExploitNucleiCheck, the
	// /exploit/nuclei-check route is never registered at all.
	ExploitNucleiCheck *exploit.NucleiCheckAdapter
	ExploitLimiter     *guardrails.RateLimiter
	// ExploitBlastLimit is FR-SAFE-3's per-action target limit; <= 0
	// means guardrails.DefaultBlastRadius.
	ExploitBlastLimit int
	// ReportFontPath is passed through to report.RenderPDF for
	// Cyrillic-capable PDF rendering; empty is a valid fallback.
	ReportFontPath string
}

// New builds the root HTTP handler for agent-core. /healthz is reachable
// without a security key (needed for container/orchestrator health
// checks); every other route requires authentication. All requests,
// authenticated or not, are recorded in the audit log per TZ 6.1 — even
// rejected/unauthenticated attempts, since a failed auth attempt is
// itself an auditable event.
func New(deps Deps) http.Handler {
	root := http.NewServeMux()
	root.HandleFunc("GET /healthz", handleHealth)

	protected := http.NewServeMux()
	if deps.Engagements != nil {
		api := &engagementAPI{
			engagements:    deps.Engagements,
			trees:          deps.Trees,
			approvals:      deps.Approvals,
			engine:         deps.Engine,
			reportFontPath: deps.ReportFontPath,
		}
		api.register(protected)
	}
	if deps.Engagements != nil && deps.RoEStore != nil {
		roeAPI := &roeAPI{
			engagements: deps.Engagements,
			roeStore:    deps.RoEStore,
			auditLogger: deps.AuditLogger,
		}
		roeAPI.register(protected)
	}
	if deps.Engagements != nil && deps.RoEStore != nil && deps.ExploitNucleiCheck != nil {
		expAPI := &exploitAPI{
			engagements: deps.Engagements,
			roeStore:    deps.RoEStore,
			trees:       deps.Trees,
			auditLogger: deps.AuditLogger,
			nucleiCheck: deps.ExploitNucleiCheck,
			limiter:     deps.ExploitLimiter,
			blastLimit:  deps.ExploitBlastLimit,
		}
		expAPI.register(protected)
	}
	root.Handle("/", auth.Middleware(deps.KeyStore)(protected))

	// CORS wraps everything else: the browser's preflight OPTIONS
	// request carries no X-API-Key, so it must be answered before
	// reaching auth.Middleware (which would otherwise reject it with
	// 401 and break every cross-origin request from web-ui).
	return withCORS(withAuditLogging(deps.AuditLogger, root))
}

// withCORS lets the web UI (served from its own origin/port) call this
// API from the browser. A wildcard origin is acceptable here because
// authentication is a bearer-style X-API-Key header the browser must be
// explicitly told to send — unlike cookie-based auth, another origin's
// page can't make an authenticated request "ambiently", so this doesn't
// open a CSRF hole the way permissive CORS would with cookie auth.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, "+auth.APIKeyHeader)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// withAuditLogging records every request that reaches the server (TZ 6.1:
// "har bir harakat... vaqt tamg'asi bilan yozib boriladi"). It wraps
// auth.Middleware from the outside so unauthenticated/rejected attempts
// are still logged (as "anonymous") — but to also report the real
// actor for requests that *do* authenticate, it hands each request a
// shared auth.ActorRecorder via context, which Middleware fills in
// further down the chain (see auth.ActorRecorder for why a plain
// r.Context() read here wouldn't see it).
func withAuditLogging(auditLogger *audit.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		ctx, actorRec := auth.NewActorRecorderContext(r.Context())
		next.ServeHTTP(rec, r.WithContext(ctx))

		actor := actorRec.Actor()
		if actor == "" {
			actor = "anonymous"
		}

		if _, err := auditLogger.Append(actor, "http.request", r.URL.Path, map[string]any{
			"method":      r.Method,
			"status":      rec.status,
			"duration_ms": time.Since(start).Milliseconds(),
		}); err != nil {
			log.Printf("httpserver: failed to write audit entry: %v", err)
		}
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
