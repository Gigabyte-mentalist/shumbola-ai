package tools

import (
	"context"
	"errors"
	"testing"
)

// sampleSmbEnumOutput is real nmap output (captured against a Samba test
// container, 2026-08-12) — two scripts produced results, the other two
// (smb-os-discovery, smb-enum-shares) found nothing on that particular
// server, which is normal: how much an SMB server reveals depends on its
// own guest-access/signing configuration.
const sampleSmbEnumOutput = `Starting Nmap 7.98 ( https://nmap.org ) at 2026-08-12 03:22 +0000
Nmap scan report for 172.17.0.4
Host is up (0.000044s latency).

PORT    STATE SERVICE
139/tcp open  netbios-ssn
445/tcp open  microsoft-ds
MAC Address: 2E:19:E9:99:59:36 (Unknown)

Host script results:
| smb2-security-mode:
|   3.1.1:
|_    Message signing enabled but not required
| smb-protocols:
|   dialects:
|     2.1
|     3.0
|     3.0.2
|_    3.1.1

Nmap done: 1 IP address (1 host up) scanned in 3.72 seconds
`

func TestSmbEnumToolExecuteParsesFindings(t *testing.T) {
	runner := &fakeRunner{result: RunResult{Stdout: sampleSmbEnumOutput, ExitCode: 0}}
	tool := NewSmbEnumTool(runner, "")

	result, err := tool.Execute(context.Background(), "10.0.0.5")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Findings) != 2 {
		t.Fatalf("expected 2 findings, got %d: %+v", len(result.Findings), result.Findings)
	}
	if result.Findings[0].Title != "smb2-security-mode" {
		t.Fatalf("unexpected first finding title: %s", result.Findings[0].Title)
	}
	if result.Findings[1].Title != "smb-protocols" {
		t.Fatalf("unexpected second finding title: %s", result.Findings[1].Title)
	}
	if runner.lastSpec.Cmd[len(runner.lastSpec.Cmd)-1] != "10.0.0.5" {
		t.Fatalf("expected target to be passed as the final Cmd arg, got %v", runner.lastSpec.Cmd)
	}
}

func TestSmbEnumToolExecuteNoScriptResults(t *testing.T) {
	// No "Host script results:" section at all — e.g. the ports were
	// filtered/closed, so the scripts never even ran. Not an error: same
	// "nothing to report" shape as nuclei/ffuf finding nothing.
	runner := &fakeRunner{result: RunResult{Stdout: "Nmap scan report for 10.0.0.5\nHost is up.\n", ExitCode: 0}}
	tool := NewSmbEnumTool(runner, "")

	result, err := tool.Execute(context.Background(), "10.0.0.5")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(result.Findings), result.Findings)
	}
}

func TestSmbEnumToolValidate(t *testing.T) {
	tool := NewSmbEnumTool(&fakeRunner{}, "")
	if err := tool.Validate(""); err == nil {
		t.Fatalf("expected error for empty target")
	}
	if err := tool.Validate("10.0.0.5"); err != nil {
		t.Fatalf("expected valid target to pass, got %v", err)
	}
}

func TestSmbEnumToolRequiresApproval(t *testing.T) {
	tool := NewSmbEnumTool(&fakeRunner{}, "")
	if !tool.RequiresApproval() {
		t.Fatalf("expected smbenum to require HITL approval (active scanning)")
	}
}

func TestSmbEnumToolExecutePropagatesRunnerError(t *testing.T) {
	runner := &fakeRunner{err: errors.New("docker unavailable")}
	tool := NewSmbEnumTool(runner, "")

	if _, err := tool.Execute(context.Background(), "10.0.0.5"); err == nil {
		t.Fatalf("expected error to propagate from runner")
	}
}
