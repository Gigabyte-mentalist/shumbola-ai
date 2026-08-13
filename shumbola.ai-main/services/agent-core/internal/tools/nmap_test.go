package tools

import (
	"context"
	"errors"
	"testing"
)

type fakeRunner struct {
	lastSpec RunSpec
	result   RunResult
	err      error
}

func (f *fakeRunner) RunContainer(ctx context.Context, spec RunSpec) (RunResult, error) {
	f.lastSpec = spec
	return f.result, f.err
}

const sampleNmapOutput = `Starting Nmap 7.94 ( https://nmap.org ) at 2026-08-10 09:00 UTC
Nmap scan report for 10.0.0.5
Host is up (0.0010s latency).
Not shown: 997 closed tcp ports (reset)
PORT     STATE SERVICE  VERSION
22/tcp   open  ssh      OpenSSH 8.9p1 Ubuntu
80/tcp   open  http     nginx 1.24.0
443/tcp  open  ssl/http nginx 1.24.0

Nmap done: 1 IP address (1 host up) scanned in 4.21 seconds
`

func TestNmapToolValidate(t *testing.T) {
	tool := NewNmapTool(&fakeRunner{}, "")

	if err := tool.Validate(""); err == nil {
		t.Fatalf("expected error for empty target")
	}
	if err := tool.Validate("10.0.0.5; rm -rf /"); err == nil {
		t.Fatalf("expected error for target with shell metacharacters")
	}
	if err := tool.Validate("10.0.0.5"); err != nil {
		t.Fatalf("expected valid target to pass, got %v", err)
	}
}

func TestNmapToolExecuteParsesFindings(t *testing.T) {
	runner := &fakeRunner{result: RunResult{Stdout: sampleNmapOutput, ExitCode: 0}}
	tool := NewNmapTool(runner, "")

	result, err := tool.Execute(context.Background(), "10.0.0.5")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Findings) != 3 {
		t.Fatalf("expected 3 findings, got %d: %+v", len(result.Findings), result.Findings)
	}
	if result.Findings[0].Title != "22/tcp open (ssh)" {
		t.Fatalf("unexpected first finding title: %s", result.Findings[0].Title)
	}
	if runner.lastSpec.Cmd[len(runner.lastSpec.Cmd)-1] != "10.0.0.5" {
		t.Fatalf("expected target to be passed as the final Cmd arg, got %v", runner.lastSpec.Cmd)
	}
}

func TestNmapToolExecutePropagatesRunnerError(t *testing.T) {
	runner := &fakeRunner{err: errors.New("docker unavailable")}
	tool := NewNmapTool(runner, "")

	if _, err := tool.Execute(context.Background(), "10.0.0.5"); err == nil {
		t.Fatalf("expected error to propagate from runner")
	}
}

func TestNmapToolExecuteNonZeroExit(t *testing.T) {
	runner := &fakeRunner{result: RunResult{Stderr: "nmap: permission denied", ExitCode: 1}}
	tool := NewNmapTool(runner, "")

	if _, err := tool.Execute(context.Background(), "10.0.0.5"); err == nil {
		t.Fatalf("expected error for non-zero exit code")
	}
}

func TestNmapToolRequiresApproval(t *testing.T) {
	tool := NewNmapTool(&fakeRunner{}, "")
	if !tool.RequiresApproval() {
		t.Fatalf("expected nmap to require HITL approval (active scanning)")
	}
}
