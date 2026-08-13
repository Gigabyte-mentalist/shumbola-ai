package standards

import (
	"testing"

	"github.com/shumbola/agent-core/internal/tools"
)

func TestOWASPCategoryPerTool(t *testing.T) {
	cases := []struct {
		tool  string
		title string
	}{
		{"nuclei", "CVE-2023-12345 RCE"},
		{"nuclei", "Exposed Admin Panel"},
		{"ffuf", "http://x/admin (status: 200)"},
		{"testssl", "TLS1 offered"},
		{"nmap", "22/tcp open (ssh)"},
		{"unknown-tool", "something"},
	}
	for _, c := range cases {
		got := OWASPCategory(c.tool, c.title)
		if got == "" {
			t.Fatalf("tool %q: expected non-empty OWASP category", c.tool)
		}
	}
}

func TestCVSSScoreOrdering(t *testing.T) {
	if CVSSScore(tools.SeverityCritical) <= CVSSScore(tools.SeverityHigh) {
		t.Fatalf("expected critical score > high score")
	}
	if CVSSScore(tools.SeverityHigh) <= CVSSScore(tools.SeverityMed) {
		t.Fatalf("expected high score > medium score")
	}
	if CVSSScore(tools.SeverityMed) <= CVSSScore(tools.SeverityLow) {
		t.Fatalf("expected medium score > low score")
	}
	if CVSSScore(tools.SeverityLow) <= CVSSScore(tools.SeverityInfo) {
		t.Fatalf("expected low score > info score")
	}
	if CVSSScore(tools.SeverityInfo) != 0.0 {
		t.Fatalf("expected info score to be 0.0, got %f", CVSSScore(tools.SeverityInfo))
	}
}

func TestRemediationCoversAllTools(t *testing.T) {
	for _, tool := range []string{"nmap", "nuclei", "ffuf", "testssl", "unknown"} {
		if Remediation(tool) == "" {
			t.Fatalf("expected non-empty remediation text for tool %q", tool)
		}
	}
}

func TestMITRETechniqueCoversAllTools(t *testing.T) {
	for _, tool := range []string{"nmap", "nuclei", "ffuf", "testssl", "unknown"} {
		if MITRETechnique(tool, "x") == "" {
			t.Fatalf("expected non-empty MITRE technique for tool %q", tool)
		}
	}
}
