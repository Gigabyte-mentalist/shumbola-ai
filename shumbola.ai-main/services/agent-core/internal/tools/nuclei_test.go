package tools

import (
	"context"
	"testing"
)

const sampleNucleiOutput = `{"template-id":"tech-detect","info":{"name":"Technology Detection","severity":"info"},"matched-at":"http://10.0.0.5:80"}
{"template-id":"CVE-2023-12345","info":{"name":"Example RCE","severity":"critical"},"matched-at":"http://10.0.0.5:80/vuln"}
not-json-status-line
{"template-id":"exposed-panel","info":{"name":"Exposed Admin Panel","severity":"medium"},"matched-at":"http://10.0.0.5:80/admin"}
`

func TestNucleiToolExecuteParsesFindings(t *testing.T) {
	runner := &fakeRunner{result: RunResult{Stdout: sampleNucleiOutput, ExitCode: 0}}
	tool := NewNucleiTool(runner, "")

	result, err := tool.Execute(context.Background(), "10.0.0.5")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Findings) != 3 {
		t.Fatalf("expected 3 findings, got %d: %+v", len(result.Findings), result.Findings)
	}
	if result.Findings[1].Severity != SeverityCritical {
		t.Fatalf("expected second finding to be critical, got %s", result.Findings[1].Severity)
	}
}

func TestNucleiToolExecuteIgnoresNonZeroExit(t *testing.T) {
	// nuclei can exit non-zero on some versions even with valid matches;
	// unlike nmap, we must not treat that as a hard failure.
	runner := &fakeRunner{result: RunResult{Stdout: sampleNucleiOutput, ExitCode: 1}}
	tool := NewNucleiTool(runner, "")

	result, err := tool.Execute(context.Background(), "10.0.0.5")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Findings) != 3 {
		t.Fatalf("expected findings despite non-zero exit, got %d", len(result.Findings))
	}
}

func TestNucleiToolValidate(t *testing.T) {
	tool := NewNucleiTool(&fakeRunner{}, "")
	if err := tool.Validate(""); err == nil {
		t.Fatalf("expected error for empty target")
	}
	if err := tool.Validate("10.0.0.5"); err != nil {
		t.Fatalf("expected valid target to pass, got %v", err)
	}
}

func TestNucleiToolRequiresApproval(t *testing.T) {
	tool := NewNucleiTool(&fakeRunner{}, "")
	if !tool.RequiresApproval() {
		t.Fatalf("expected nuclei to require HITL approval")
	}
}
