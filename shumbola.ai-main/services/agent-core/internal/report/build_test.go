package report

import (
	"encoding/json"
	"testing"

	"github.com/shumbola/agent-core/internal/engagement"
	"github.com/shumbola/agent-core/internal/tasktree"
	"github.com/shumbola/agent-core/internal/tools"
	"github.com/shumbola/agent-core/internal/validation"
)

func testEngagement(t *testing.T) *engagement.Engagement {
	t.Helper()
	eng, err := engagement.New("eng-1", "op-1", []string{"10.0.0.5"})
	if err != nil {
		t.Fatalf("engagement.New: %v", err)
	}
	return eng
}

func TestBuildExcludesUnprovenFindings(t *testing.T) {
	tree := tasktree.New("eng-1")
	tree.AddNode("", tasktree.KindFinding, "proven finding", map[string]any{
		"target": "10.0.0.5", "tool": "nmap", "severity": tools.SeverityHigh,
		"detail": "evidence", "proven": true, "confidence": validation.ConfidenceHigh,
	})
	tree.AddNode("", tasktree.KindFinding, "unproven finding", map[string]any{
		"target": "10.0.0.5", "tool": "nmap", "severity": tools.SeverityHigh,
		"detail": "", "proven": false, "confidence": validation.ConfidenceLow,
	})

	data := Build(testEngagement(t), tree, LangUzLatn)
	if len(data.Findings) != 1 {
		t.Fatalf("expected 1 proven finding, got %d: %+v", len(data.Findings), data.Findings)
	}
	if data.Findings[0].Title != "proven finding" {
		t.Fatalf("unexpected finding included: %s", data.Findings[0].Title)
	}
}

func TestBuildSortsBySeverityDescending(t *testing.T) {
	tree := tasktree.New("eng-1")
	add := func(title string, sev tools.Severity) {
		tree.AddNode("", tasktree.KindFinding, title, map[string]any{
			"target": "10.0.0.5", "tool": "nuclei", "severity": sev,
			"detail": "evidence", "proven": true, "confidence": validation.ConfidenceMedium,
		})
	}
	add("low finding", tools.SeverityLow)
	add("critical finding", tools.SeverityCritical)
	add("medium finding", tools.SeverityMed)

	data := Build(testEngagement(t), tree, LangUzLatn)
	if len(data.Findings) != 3 {
		t.Fatalf("expected 3 findings, got %d", len(data.Findings))
	}
	if data.Findings[0].Severity != tools.SeverityCritical || data.Findings[2].Severity != tools.SeverityLow {
		t.Fatalf("expected findings sorted critical->low, got %+v", data.Findings)
	}
}

func TestBuildSummaryCounts(t *testing.T) {
	tree := tasktree.New("eng-1")
	add := func(sev tools.Severity) {
		tree.AddNode("", tasktree.KindFinding, "f", map[string]any{
			"target": "10.0.0.5", "tool": "nmap", "severity": sev,
			"detail": "evidence", "proven": true, "confidence": validation.ConfidenceMedium,
		})
	}
	add(tools.SeverityCritical)
	add(tools.SeverityHigh)
	add(tools.SeverityHigh)
	add(tools.SeverityLow)

	data := Build(testEngagement(t), tree, LangUzLatn)
	if data.Summary.Critical != 1 || data.Summary.High != 2 || data.Summary.Low != 1 {
		t.Fatalf("unexpected summary: %+v", data.Summary)
	}
	if data.Summary.Total() != 4 {
		t.Fatalf("expected total 4, got %d", data.Summary.Total())
	}
}

// TestBuildHandlesJSONRoundTrippedTree guards against the type-assertion
// bug where Details values stored as tools.Severity/validation.Confidence
// become plain strings after a JSON marshal/unmarshal round trip (as
// happens whenever a tree is reloaded from tasktree.FileStore).
func TestBuildHandlesJSONRoundTrippedTree(t *testing.T) {
	tree := tasktree.New("eng-1")
	tree.AddNode("", tasktree.KindFinding, "roundtrip finding", map[string]any{
		"target": "10.0.0.5", "tool": "nuclei", "severity": tools.SeverityCritical,
		"detail": "evidence", "proven": true, "confidence": validation.ConfidenceHigh,
	})

	raw, err := json.Marshal(struct {
		EngagementID string          `json:"engagement_id"`
		Nodes        []tasktree.Node `json:"nodes"`
	}{tree.EngagementID, tree.Snapshot()})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded struct {
		EngagementID string          `json:"engagement_id"`
		Nodes        []tasktree.Node `json:"nodes"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	roundTripped := tasktree.New(decoded.EngagementID)
	for _, n := range decoded.Nodes {
		roundTripped.AddNode(n.ParentID, n.Kind, n.Summary, n.Details)
	}

	data := Build(testEngagement(t), roundTripped, LangUzLatn)
	if len(data.Findings) != 1 {
		t.Fatalf("expected 1 finding to survive JSON round trip, got %d", len(data.Findings))
	}
	if data.Findings[0].Severity != tools.SeverityCritical {
		t.Fatalf("expected severity to survive JSON round trip as %q, got %q", tools.SeverityCritical, data.Findings[0].Severity)
	}
	if data.Summary.Critical != 1 {
		t.Fatalf("expected summary to count the round-tripped critical finding, got %+v", data.Summary)
	}
}
