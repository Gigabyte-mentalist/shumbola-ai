package report

import (
	"bytes"
	"testing"
	"time"

	"github.com/shumbola/agent-core/internal/tools"
	"github.com/shumbola/agent-core/internal/validation"
)

func sampleReportData() ReportData {
	return ReportData{
		EngagementID: "eng-1",
		Scope:        []string{"10.0.0.5"},
		Lang:         LangUzLatn,
		GeneratedAt:  time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC),
		Summary:      Summary{High: 1},
		Findings: []ReportFinding{
			{
				Title:       "22/tcp open (ssh)",
				Severity:    tools.SeverityHigh,
				CVSS:        7.5,
				OWASP:       "A05:2021 - Security Misconfiguration",
				MITRE:       "T1046 - Network Service Discovery",
				Target:      "10.0.0.5",
				Tool:        "nmap",
				Detail:      "Port 22/tcp is open",
				Remediation: "Firewall bilan cheklang.",
				Confidence:  validation.ConfidenceHigh,
			},
		},
	}
}

func TestRenderPDFProducesValidHeader(t *testing.T) {
	out, err := RenderPDF(sampleReportData(), "")
	if err != nil {
		t.Fatalf("RenderPDF: %v", err)
	}
	if len(out) == 0 {
		t.Fatalf("expected non-empty PDF output")
	}
	if !bytes.HasPrefix(out, []byte("%PDF")) {
		t.Fatalf("expected output to start with %%PDF header, got: %q", out[:min(20, len(out))])
	}
}

func TestRenderPDFEmptyFindings(t *testing.T) {
	data := sampleReportData()
	data.Findings = nil
	data.Summary = Summary{}

	out, err := RenderPDF(data, "")
	if err != nil {
		t.Fatalf("RenderPDF with no findings: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF")) {
		t.Fatalf("expected valid PDF header even with no findings")
	}
}

func TestRenderPDFAllLanguages(t *testing.T) {
	for _, lang := range []string{LangUzLatn, LangUzCyrl, LangRu} {
		data := sampleReportData()
		data.Lang = lang
		out, err := RenderPDF(data, "")
		if err != nil {
			t.Fatalf("RenderPDF lang=%s: %v", lang, err)
		}
		if !bytes.HasPrefix(out, []byte("%PDF")) {
			t.Fatalf("lang=%s: expected valid PDF header", lang)
		}
	}
}

func TestRenderPDFFallsBackWhenFontMissing(t *testing.T) {
	// A nonexistent font path must not error — RenderPDF falls back to
	// the core font and still produces a valid PDF.
	out, err := RenderPDF(sampleReportData(), "/nonexistent/font.ttf")
	if err != nil {
		t.Fatalf("RenderPDF with missing font path: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF")) {
		t.Fatalf("expected valid PDF header when font path is missing")
	}
}
