// Endpoints implementing shumbola_offensive_TZ.md's FR-AUTHZ framework
// (Bosqich 6): uploading/counter-approving a Rules of Engagement
// document, and requesting an engagement's exploit mode. No
// exploitation adapter exists yet (Bosqich 7/8) — these endpoints only
// manage the authorization state that exploitgate.Authorize will later
// require before any such adapter is allowed to act.
package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/shumbola/agent-core/internal/audit"
	"github.com/shumbola/agent-core/internal/auth"
	"github.com/shumbola/agent-core/internal/engagement"
	"github.com/shumbola/agent-core/internal/roe"
)

// roeAPI holds the dependencies the FR-AUTHZ endpoints need.
type roeAPI struct {
	engagements *engagement.FileStore
	roeStore    roe.Store
	auditLogger *audit.Logger
}

func (a *roeAPI) register(mux *http.ServeMux) {
	// Uploading and counter-approving a RoE is deliberately
	// Administrator-only (FR-AUTHZ-5's two-person rule needs two
	// distinct Administrators; an Operator alone can never satisfy it).
	mux.Handle("POST /engagements/{id}/roe", auth.RequireRole(http.HandlerFunc(a.uploadRoE), auth.RoleAdministrator))
	mux.Handle("POST /engagements/{id}/roe/counter-approve", auth.RequireRole(http.HandlerFunc(a.counterApproveRoE), auth.RoleAdministrator))
	// Reading a RoE's state (including for Auditors, who may view but
	// never approve/exploit) needs no role restriction beyond
	// authentication, matching every other read endpoint in this API.
	mux.HandleFunc("GET /engagements/{id}/roe", a.getRoE)

	// Requesting exploit mode and marking a target exploit-allowed are
	// Operator/Administrator actions — never Auditor (FR-AUTHZ-5).
	mux.Handle("POST /engagements/{id}/mode", auth.RequireRole(http.HandlerFunc(a.setMode), auth.RoleOperator, auth.RoleAdministrator))
	mux.Handle("PATCH /engagements/{id}/scope-authz", auth.RequireRole(http.HandlerFunc(a.setScopeAuthz), auth.RoleOperator, auth.RoleAdministrator))
}

type uploadRoERequest struct {
	ValidFrom                 time.Time `json:"valid_from"`
	ValidUntil                time.Time `json:"valid_until"`
	ProhibitedActions         []string  `json:"prohibited_actions"`
	Contacts                  []string  `json:"contacts"`
	AllowedDestructiveActions []string  `json:"allowed_destructive_actions"`
}

// uploadRoE creates (or replaces) the RoE document for an engagement —
// FR-AUTHZ-1's required first signature. It never enables exploit mode
// by itself; see setMode.
func (a *roeAPI) uploadRoE(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := a.engagements.Load(id); errors.Is(err, engagement.ErrNotFound) {
		http.Error(w, "engagement not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "failed to load engagement", http.StatusInternalServerError)
		return
	}

	var req uploadRoERequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	principal, _ := auth.FromContext(r.Context())
	docID := fmt.Sprintf("%s-roe-%d", id, time.Now().UTC().UnixNano())
	doc, err := roe.New(docID, id, principal.ID, req.ValidFrom, req.ValidUntil, req.ProhibitedActions, req.Contacts, req.AllowedDestructiveActions)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := a.roeStore.Save(doc); err != nil {
		http.Error(w, "failed to save RoE", http.StatusInternalServerError)
		return
	}

	a.audit(principal.ID, "roe.uploaded", id, map[string]any{"roe_id": doc.ID, "valid_from": doc.ValidFrom, "valid_until": doc.ValidUntil})
	writeJSON(w, http.StatusCreated, doc)
}

func (a *roeAPI) getRoE(w http.ResponseWriter, r *http.Request) {
	doc, err := a.roeStore.Load(r.PathValue("id"))
	if errors.Is(err, roe.ErrNotFound) {
		http.Error(w, "no RoE on file for this engagement", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to load RoE", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

// counterApproveRoE records the second, independent signature
// FR-AUTHZ-5's two-person rule requires. It fails if no RoE is on file,
// if it's already counter-approved, or if the calling Administrator is
// the same principal who uploaded it.
func (a *roeAPI) counterApproveRoE(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	principal, _ := auth.FromContext(r.Context())

	doc, err := a.roeStore.Update(id, func(d *roe.Document) error {
		return d.CounterApprove(principal.ID)
	})
	if errors.Is(err, roe.ErrNotFound) {
		http.Error(w, "no RoE on file for this engagement", http.StatusNotFound)
		return
	}
	if errors.Is(err, roe.ErrSameApprover) || errors.Is(err, roe.ErrAlreadyCounterApproved) {
		a.audit(principal.ID, "roe.counter_approve_denied", id, map[string]any{"reason": err.Error()})
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, "failed to update RoE", http.StatusInternalServerError)
		return
	}

	a.audit(principal.ID, "roe.counter_approved", id, map[string]any{"roe_id": doc.ID})
	writeJSON(w, http.StatusOK, doc)
}

type setModeRequest struct {
	Mode engagement.Mode `json:"mode"`
}

// setMode requests a change to an engagement's Mode. Moving to
// ModeExploit is gated by FR-AUTHZ-1/3/5: a valid, two-person-approved
// RoE must already be on file, or the request is denied — this endpoint
// enables nothing by itself beyond recording the operator's intent
// alongside a passing authorization check.
func (a *roeAPI) setMode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	principal, _ := auth.FromContext(r.Context())

	var req setModeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Mode != engagement.ModeDetect && req.Mode != engagement.ModeExploit {
		http.Error(w, fmt.Sprintf("mode must be %q or %q", engagement.ModeDetect, engagement.ModeExploit), http.StatusBadRequest)
		return
	}

	if req.Mode == engagement.ModeExploit {
		doc, err := a.roeStore.Load(id)
		if errors.Is(err, roe.ErrNotFound) {
			a.audit(principal.ID, "engagement.mode_change_denied", id, map[string]any{"requested_mode": req.Mode, "reason": "no RoE on file"})
			http.Error(w, "exploit mode requires a RoE on file (FR-AUTHZ-1)", http.StatusForbidden)
			return
		}
		if err != nil {
			http.Error(w, "failed to load RoE", http.StatusInternalServerError)
			return
		}
		now := time.Now().UTC()
		if !doc.ValidNow(now) {
			a.audit(principal.ID, "engagement.mode_change_denied", id, map[string]any{"requested_mode": req.Mode, "reason": "RoE outside valid window", "roe_id": doc.ID})
			http.Error(w, "RoE is outside its authorized time window (FR-AUTHZ-3)", http.StatusForbidden)
			return
		}
		if !doc.TwoPersonSatisfied() {
			a.audit(principal.ID, "engagement.mode_change_denied", id, map[string]any{"requested_mode": req.Mode, "reason": "two-person rule not satisfied", "roe_id": doc.ID})
			http.Error(w, "RoE lacks the required independent second approval (FR-AUTHZ-5)", http.StatusForbidden)
			return
		}
	}

	eng, err := a.engagements.Update(id, func(e *engagement.Engagement) error {
		e.Mode = req.Mode
		return nil
	})
	if errors.Is(err, engagement.ErrNotFound) {
		http.Error(w, "engagement not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to update engagement", http.StatusInternalServerError)
		return
	}

	a.audit(principal.ID, "engagement.mode_changed", id, map[string]any{"mode": req.Mode})
	writeJSON(w, http.StatusOK, eng)
}

type setScopeAuthzRequest struct {
	Target string                `json:"target"`
	Level  engagement.AuthzLevel `json:"level"`
}

var validAuthzLevels = map[engagement.AuthzLevel]bool{
	engagement.AuthzReconOnly:      true,
	engagement.AuthzDetectOnly:     true,
	engagement.AuthzExploitAllowed: true,
}

// setScopeAuthz sets one target's authorization level within an
// engagement's scope (FR-AUTHZ-2). The target must already be part of
// the engagement's scope — this changes what's authorized for an
// existing target, it does not add new targets to scope.
func (a *roeAPI) setScopeAuthz(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	principal, _ := auth.FromContext(r.Context())

	var req setScopeAuthzRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if !validAuthzLevels[req.Level] {
		http.Error(w, fmt.Sprintf("level must be one of %q, %q, %q", engagement.AuthzReconOnly, engagement.AuthzDetectOnly, engagement.AuthzExploitAllowed), http.StatusBadRequest)
		return
	}

	eng, err := a.engagements.Update(id, func(e *engagement.Engagement) error {
		if !e.Scope.Allows(req.Target) {
			return fmt.Errorf("target %q is not part of this engagement's scope", req.Target)
		}
		if e.Scope.Authz == nil {
			e.Scope.Authz = map[string]engagement.AuthzLevel{}
		}
		e.Scope.Authz[req.Target] = req.Level
		return nil
	})
	if errors.Is(err, engagement.ErrNotFound) {
		http.Error(w, "engagement not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a.audit(principal.ID, "engagement.scope_authz_updated", id, map[string]any{"target": req.Target, "level": req.Level})
	writeJSON(w, http.StatusOK, eng)
}

func (a *roeAPI) audit(actor, action, resource string, details map[string]any) {
	if a.auditLogger == nil {
		return
	}
	_, _ = a.auditLogger.Append(actor, action, resource, details)
}
