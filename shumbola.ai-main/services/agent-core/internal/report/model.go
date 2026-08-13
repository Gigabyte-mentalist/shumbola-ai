// Package report implements the Report Engine (TZ 4.6): it turns a
// finished (or in-progress) engagement's Task Tree into a standards-
// mapped PDF report, in the operator's chosen language (TZ 6.5).
//
// Only proven findings are included — TZ 4.5: "isbotlanmagan bo'lsa —
// reportga chiqmaydi". Confidence and the standards mapping (OWASP/
// MITRE/CVSS) are attached per finding so the report can show its basis.
package report

import (
	"time"

	"github.com/shumbola/agent-core/internal/tools"
	"github.com/shumbola/agent-core/internal/validation"
)

// ReportFinding is one confirmed finding as it appears in the report.
type ReportFinding struct {
	Title       string
	Severity    tools.Severity
	CVSS        float64
	OWASP       string
	MITRE       string
	Target      string
	Tool        string
	Detail      string
	Remediation string
	Confidence  validation.Confidence
}

// Summary counts findings by severity, for the executive summary.
type Summary struct {
	Critical int
	High     int
	Medium   int
	Low      int
	Info     int
}

// Total returns the total number of findings the summary covers.
func (s Summary) Total() int {
	return s.Critical + s.High + s.Medium + s.Low + s.Info
}

// ReportData is everything a renderer (PDF, and later HTML/DOCX) needs
// to produce a report — independent of output format.
type ReportData struct {
	EngagementID string
	Scope        []string
	Lang         string
	GeneratedAt  time.Time
	Summary      Summary
	Findings     []ReportFinding
}
