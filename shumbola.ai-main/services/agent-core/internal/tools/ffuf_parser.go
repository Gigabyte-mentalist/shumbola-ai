package tools

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ffufOutput mirrors the subset of ffuf's `-of json` output we care
// about — a single JSON object with a "results" array, one entry per
// probed path (we ran with -mc all, so this includes 404s too).
type ffufOutput struct {
	Results []struct {
		Input  map[string]string `json:"input"`
		Status int               `json:"status"`
		Length int               `json:"length"`
		Words  int               `json:"words"`
		URL    string            `json:"url"`
	} `json:"results"`
}

// parseFfufOutput turns ffuf's JSON report into Findings, one per
// discovered path that didn't 404 — a bare 404 carries no signal and
// would just be noise in the task tree.
func parseFfufOutput(output string) ([]Finding, error) {
	if output == "" {
		return nil, nil
	}

	var parsed ffufOutput
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		// ffuf's own live-progress output (ANSI cursor/color codes, one
		// line per probed path) lands on stdout ahead of the JSON report
		// (see the comment in FfufTool.Execute) — none of those lines
		// contain '{' or '}', so the JSON object is reliably the
		// substring from the first '{' to the last '}'.
		start := strings.Index(output, "{")
		end := strings.LastIndex(output, "}")
		if start == -1 || end == -1 || end < start {
			return nil, fmt.Errorf("unmarshal ffuf json: %w (no JSON object found in output)", err)
		}
		if jsonErr := json.Unmarshal([]byte(output[start:end+1]), &parsed); jsonErr != nil {
			return nil, fmt.Errorf("unmarshal ffuf json: %w", jsonErr)
		}
	}

	var findings []Finding
	for _, r := range parsed.Results {
		if r.Status == 404 {
			continue
		}
		findings = append(findings, Finding{
			Title:    fmt.Sprintf("%s (status: %d)", r.URL, r.Status),
			Severity: SeverityInfo,
			Detail:   fmt.Sprintf("URL %q responded with status %d, length %d, words %d", r.URL, r.Status, r.Length, r.Words),
		})
	}
	return findings, nil
}
