package tools

import (
	"context"
	"testing"
)

const sampleTestsslOutput = `[
  {"id": "cert_expiration", "finding": "expires in 45 days", "severity": "OK"},
  {"id": "SSLv3", "finding": "not offered (OK)", "severity": "OK"},
  {"id": "TLS1", "finding": "offered (deprecated)", "severity": "MEDIUM"},
  {"id": "heartbleed", "finding": "not vulnerable", "severity": "OK"},
  {"id": "ROBOT", "finding": "VULNERABLE", "severity": "CRITICAL", "cve": "CVE-2017-13099"}
]`

func TestTestsslToolExecuteParsesFindingsSkipsOK(t *testing.T) {
	runner := &fakeRunner{result: RunResult{Stdout: sampleTestsslOutput, ExitCode: 0}}
	tool := NewTestsslTool(runner, "")

	result, err := tool.Execute(context.Background(), "10.0.0.5")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Findings) != 2 {
		t.Fatalf("expected 2 findings (OK entries skipped), got %d: %+v", len(result.Findings), result.Findings)
	}
	if result.Findings[1].Severity != SeverityCritical {
		t.Fatalf("expected ROBOT finding to be critical, got %s", result.Findings[1].Severity)
	}
	if result.Findings[1].Title != "ROBOT (CVE-2017-13099)" {
		t.Fatalf("unexpected title: %s", result.Findings[1].Title)
	}
}

func TestTestsslToolValidate(t *testing.T) {
	tool := NewTestsslTool(&fakeRunner{}, "")
	if err := tool.Validate(""); err == nil {
		t.Fatalf("expected error for empty target")
	}
}

func TestTestsslToolRequiresApproval(t *testing.T) {
	tool := NewTestsslTool(&fakeRunner{}, "")
	if !tool.RequiresApproval() {
		t.Fatalf("expected testssl to require HITL approval")
	}
}
