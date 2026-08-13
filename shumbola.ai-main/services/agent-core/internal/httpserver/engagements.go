package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/shumbola/agent-core/internal/agentcore"
	"github.com/shumbola/agent-core/internal/auth"
	"github.com/shumbola/agent-core/internal/engagement"
	"github.com/shumbola/agent-core/internal/hitl"
	"github.com/shumbola/agent-core/internal/report"
	"github.com/shumbola/agent-core/internal/tasktree"
)

// engagementAPI holds the dependencies engagement-related handlers need.
type engagementAPI struct {
	engagements *engagement.FileStore
	trees       *tasktree.FileStore
	approvals   *hitl.Store
	engine      *agentcore.Engine
	// reportFontPath points at a Cyrillic-capable TTF (see report.RenderPDF);
	// empty means the report falls back to a Latin-only core font.
	reportFontPath string
}

func (a *engagementAPI) register(mux *http.ServeMux) {
	mux.Handle("POST /engagements", auth.RequireRole(http.HandlerFunc(a.createEngagement), auth.RoleOperator, auth.RoleAdministrator))
	mux.HandleFunc("GET /engagements", a.listEngagements)
	mux.HandleFunc("GET /engagements/{id}", a.getEngagement)
	mux.Handle("POST /engagements/{id}/start", auth.RequireRole(http.HandlerFunc(a.startEngagement), auth.RoleOperator, auth.RoleAdministrator))
	mux.HandleFunc("GET /engagements/{id}/tasktree", a.getTaskTree)
	mux.HandleFunc("GET /engagements/{id}/approvals", a.listApprovals)
	mux.Handle("POST /engagements/{id}/approvals/{approvalID}", auth.RequireRole(http.HandlerFunc(a.decideApproval), auth.RoleOperator, auth.RoleAdministrator))
	mux.Handle("POST /engagements/{id}/stop", auth.RequireRole(http.HandlerFunc(a.stopEngagement), auth.RoleOperator, auth.RoleAdministrator))
	mux.Handle("GET /engagements/{id}/report", auth.RequireRole(http.HandlerFunc(a.getReport), auth.RoleOperator, auth.RoleAdministrator, auth.RoleAuditor))
}

type createEngagementRequest struct {
	ID      string   `json:"id"`
	Targets []string `json:"targets"`
}

// createEngagement handles TZ 4.1's "scope belgilash": only an
// authenticated Operator/Administrator can set the initial scope
// allowlist, and doing so is itself the required audit trail (TZ 4.4's
// "Scope birinchi marta belgilanganda" gate).
func (a *engagementAPI) createEngagement(w http.ResponseWriter, r *http.Request) {
	var req createEngagementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	principal, _ := auth.FromContext(r.Context())
	eng, err := engagement.New(req.ID, principal.ID, req.Targets)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := a.engagements.Save(eng); err != nil {
		http.Error(w, "failed to save engagement", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, eng)
}

// listEngagements powers the web UI's dashboard (TZ 4.7). Like the other
// read endpoints (getEngagement, getTaskTree, listApprovals), it only
// requires authentication, not a specific role — read access is open to
// any authenticated principal.
func (a *engagementAPI) listEngagements(w http.ResponseWriter, r *http.Request) {
	list, err := a.engagements.List()
	if err != nil {
		http.Error(w, "failed to list engagements", http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []*engagement.Engagement{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *engagementAPI) getEngagement(w http.ResponseWriter, r *http.Request) {
	eng, err := a.engagements.Load(r.PathValue("id"))
	if errors.Is(err, engagement.ErrNotFound) {
		http.Error(w, "engagement not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to load engagement", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, eng)
}

// startEngagement kicks off the ReAct loop in the background. The HTTP
// response returns immediately; progress is observed via the task tree
// and approvals endpoints.
func (a *engagementAPI) startEngagement(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	eng, err := a.engagements.Load(id)
	if errors.Is(err, engagement.ErrNotFound) {
		http.Error(w, "engagement not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to load engagement", http.StatusInternalServerError)
		return
	}
	if eng.Status == engagement.StatusRunning {
		// Starting an already-running engagement a second time (a
		// double-click, a client retry, ...) would spawn a second
		// Engine.Run goroutine racing the first over the same
		// engagement/tree objects — neither the engine nor the task
		// tree/hitl stores are designed to have two runs for one
		// engagement in flight at once. Reject outright rather than
		// let that happen.
		http.Error(w, "engagement is already running", http.StatusConflict)
		return
	}

	tree, err := a.trees.LoadOrNew(id)
	if err != nil {
		http.Error(w, "failed to load task tree", http.StatusInternalServerError)
		return
	}

	go func() {
		// Deliberately detached from the request context: the run must
		// outlive this HTTP response and is only stopped via the kill
		// switch (POST /stop), never by the client disconnecting.
		_ = a.engine.Run(context.Background(), eng, tree)
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "started"})
}

func (a *engagementAPI) getTaskTree(w http.ResponseWriter, r *http.Request) {
	tree, err := a.trees.LoadOrNew(r.PathValue("id"))
	if err != nil {
		http.Error(w, "failed to load task tree", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, tree.Snapshot())
}

func (a *engagementAPI) listApprovals(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.approvals.List(r.PathValue("id")))
}

type decisionRequest struct {
	Approved bool `json:"approved"`
}

func (a *engagementAPI) decideApproval(w http.ResponseWriter, r *http.Request) {
	var req decisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := a.approvals.Decide(r.PathValue("approvalID"), req.Approved); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"approved": req.Approved})
}

// getReport builds and returns a PDF report for the engagement's current
// Task Tree state (TZ 4.6). Only proof-of-exploit-validated findings are
// included (TZ 4.5) — report.Build enforces that filter. lang selects
// the report language (TZ 6.5); it defaults to uz-Latn if omitted or
// unrecognized.
func (a *engagementAPI) getReport(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	eng, err := a.engagements.Load(id)
	if errors.Is(err, engagement.ErrNotFound) {
		http.Error(w, "engagement not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to load engagement", http.StatusInternalServerError)
		return
	}

	tree, err := a.trees.LoadOrNew(id)
	if err != nil {
		http.Error(w, "failed to load task tree", http.StatusInternalServerError)
		return
	}

	lang := r.URL.Query().Get("lang")
	data := report.Build(eng, tree, lang)

	pdfBytes, err := report.RenderPDF(data, a.reportFontPath)
	if err != nil {
		http.Error(w, "failed to render report", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-report.pdf"`, id))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdfBytes)
}

// stopEngagement is the kill switch: TZ 6.1 requires the operator be able
// to halt all activity for an engagement with a single action.
func (a *engagementAPI) stopEngagement(w http.ResponseWriter, r *http.Request) {
	stopped := a.engine.Stop(r.PathValue("id"))
	writeJSON(w, http.StatusOK, map[string]bool{"stopped": stopped})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
