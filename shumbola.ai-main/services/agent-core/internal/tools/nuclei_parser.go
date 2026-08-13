package tools

import (
	"encoding/json"
	"fmt"
	"strings"
)

// nucleiMatch mirrors the subset of nuclei's -jsonl output fields we
// care about. Real output has many more fields; we only decode what we
// turn into a Finding.
type nucleiMatch struct {
	TemplateID string `json:"template-id"`
	Info       struct {
		Name     string `json:"name"`
		Severity string `json:"severity"`
	} `json:"info"`
	MatchedAt string `json:"matched-at"`
}

// parseNucleiOutput reads nuclei's -jsonl stdout (one JSON object per
// line, one per match) into Findings. Lines that aren't valid JSON are
// skipped rather than failing the whole parse — nuclei can interleave
// occasional non-JSON status lines even with -silent on some versions.
func parseNucleiOutput(output string) []Finding {
	var findings []Finding
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var m nucleiMatch
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			continue
		}
		if m.TemplateID == "" {
			continue
		}

		findings = append(findings, Finding{
			Title:    fmt.Sprintf("%s (%s)", m.Info.Name, m.TemplateID),
			Severity: mapNucleiSeverity(m.Info.Severity),
			Detail:   fmt.Sprintf("Template %q matched at %q", m.TemplateID, m.MatchedAt),
		})
	}
	return findings
}

func mapNucleiSeverity(s string) Severity {
	switch strings.ToLower(s) {
	case "critical":
		return SeverityCritical
	case "high":
		return SeverityHigh
	case "medium":
		return SeverityMed
	case "low":
		return SeverityLow
	default:
		return SeverityInfo
	}
}
