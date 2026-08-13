package validation

import (
	"testing"

	"github.com/shumbola/agent-core/internal/tools"
)

func TestEvaluateNoEvidenceIsNotProven(t *testing.T) {
	v := NewValidator()
	verdict := v.Evaluate(tools.Finding{Title: "something", Severity: tools.SeverityHigh, Detail: ""})
	if verdict.Proven {
		t.Fatalf("expected a finding with no Detail to be unproven regardless of severity")
	}
	if verdict.Confidence != ConfidenceLow {
		t.Fatalf("expected low confidence for unproven finding, got %s", verdict.Confidence)
	}
}

func TestEvaluateSeverityToConfidence(t *testing.T) {
	v := NewValidator()
	cases := []struct {
		severity tools.Severity
		want     Confidence
	}{
		{tools.SeverityCritical, ConfidenceHigh},
		{tools.SeverityHigh, ConfidenceHigh},
		{tools.SeverityMed, ConfidenceMedium},
		{tools.SeverityLow, ConfidenceMedium},
		{tools.SeverityInfo, ConfidenceLow},
		{"", ConfidenceLow},
	}
	for _, c := range cases {
		verdict := v.Evaluate(tools.Finding{Severity: c.severity, Detail: "evidence present"})
		if !verdict.Proven {
			t.Fatalf("severity %q: expected proven=true when Detail is present", c.severity)
		}
		if verdict.Confidence != c.want {
			t.Fatalf("severity %q: expected confidence %s, got %s", c.severity, c.want, verdict.Confidence)
		}
	}
}
