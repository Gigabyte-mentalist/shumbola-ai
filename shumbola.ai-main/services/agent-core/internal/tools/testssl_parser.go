package tools

import (
	"encoding/json"
	"fmt"
	"strings"
)

// testsslEntry mirrors one element of testssl.sh's --jsonfile-pretty
// array output.
type testsslEntry struct {
	ID       string `json:"id"`
	Finding  string `json:"finding"`
	Severity string `json:"severity"`
	CVE      string `json:"cve"`
}

// parseTestsslOutput turns testssl.sh's JSON array into Findings,
// skipping "OK"/"INFO"-with-no-issue style entries that just confirm a
// secure configuration rather than reporting a problem.
func parseTestsslOutput(output string) ([]Finding, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return nil, nil
	}
	var entries []testsslEntry
	if err := json.Unmarshal([]byte(output), &entries); err != nil {
		return nil, fmt.Errorf("unmarshal testssl json: %w", err)
	}

	var findings []Finding
	for _, e := range entries {
		sev := mapTestsslSeverity(e.Severity)
		if sev == "" {
			continue // "OK" — confirms a secure setting, not a finding
		}
		title := e.ID
		if e.CVE != "" {
			title = fmt.Sprintf("%s (%s)", e.ID, e.CVE)
		}
		findings = append(findings, Finding{
			Title:    title,
			Severity: sev,
			Detail:   e.Finding,
		})
	}
	return findings, nil
}

func mapTestsslSeverity(s string) Severity {
	switch strings.ToUpper(s) {
	case "OK":
		return ""
	case "CRITICAL":
		return SeverityCritical
	case "HIGH":
		return SeverityHigh
	case "MEDIUM", "WARN":
		return SeverityMed
	case "LOW":
		return SeverityLow
	default: // INFO, DEBUG, or anything unrecognized
		return SeverityInfo
	}
}
