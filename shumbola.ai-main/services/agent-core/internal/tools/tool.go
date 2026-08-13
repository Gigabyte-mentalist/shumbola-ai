// Package tools wraps external pentesting tools behind a validated,
// parsed interface (TZ 4.3): "kirish parametrlari tekshiriladi, chiqishi
// strukturaga o'giriladi (parsing), xavfli bayroqlar bloklanadi."
package tools

import "context"

// Severity is a coarse finding severity, refined by the Validation layer
// (Bosqich 2) once CVSS scoring is wired in.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityLow      Severity = "low"
	SeverityMed      Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// Finding is one structured observation extracted from a tool's raw output.
type Finding struct {
	Title    string   `json:"title"`
	Severity Severity `json:"severity"`
	Detail   string   `json:"detail"`
}

// Result is a tool invocation's outcome: the raw output (kept for the
// audit trail) plus whatever structured Findings could be parsed from it.
type Result struct {
	Raw      string    `json:"raw"`
	Findings []Finding `json:"findings"`
}

// Tool is the interface every wrapped pentesting tool implements. Per TZ
// 4.3, the model never runs a tool directly — it only selects one via the
// Planner, and the Engine calls through this interface, which is the
// choke point for validation and (via RequiresApproval) HITL gating.
type Tool interface {
	// Name identifies the tool for logging, task tree entries, and the
	// Planner's action selection (e.g. "nmap").
	Name() string
	// RequiresApproval reports whether TZ 4.4's HITL gate must be
	// satisfied before Execute runs. Any active/intrusive tool (which
	// includes network scanning) must return true.
	RequiresApproval() bool
	// Validate checks the target is well-formed for this tool, before
	// any scope check or execution. It does not check engagement scope —
	// that is enforced separately by the caller (engagement.Scope.Allows).
	Validate(target string) error
	// Execute runs the tool against target and returns its parsed result.
	Execute(ctx context.Context, target string) (Result, error)
}
