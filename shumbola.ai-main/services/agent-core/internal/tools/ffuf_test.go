package tools

import (
	"context"
	"testing"
)

const sampleFfufOutput = `{
  "results": [
    {"input": {"FUZZ": "admin"}, "status": 200, "length": 512, "words": 40, "url": "http://10.0.0.5/admin"},
    {"input": {"FUZZ": "backup"}, "status": 403, "length": 12, "words": 2, "url": "http://10.0.0.5/backup"},
    {"input": {"FUZZ": "doesnotexist"}, "status": 404, "length": 0, "words": 0, "url": "http://10.0.0.5/doesnotexist"}
  ]
}`

func TestFfufToolExecuteParsesFindings(t *testing.T) {
	runner := &fakeRunner{result: RunResult{Stdout: sampleFfufOutput, ExitCode: 0}}
	tool := NewFfufTool(runner, "", "")

	result, err := tool.Execute(context.Background(), "10.0.0.5")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Findings) != 2 {
		t.Fatalf("expected 2 findings (404 filtered out), got %d: %+v", len(result.Findings), result.Findings)
	}
}

// TestFfufToolExecuteParsesFindingsWithProgressNoise covers real ffuf
// output (secsi/ffuf v2.2.1): its live-progress lines, complete with
// ANSI cursor/color escape codes, land on stdout ahead of the JSON
// report — this used to make json.Unmarshal fail outright on the first
// escape byte ("invalid character '\x1b'"). The parser must find and
// use only the trailing JSON object.
func TestFfufToolExecuteParsesFindingsWithProgressNoise(t *testing.T) {
	noisy := "\x1b[2Kadmin                   [Status: 200, Size: 512, Words: 40, Lines: 10, Duration: 12ms]\x1b[0m\n" +
		"\x1b[2Kbackup                  [Status: 403, Size: 12, Words: 2, Lines: 1, Duration: 8ms]\x1b[0m\n" +
		sampleFfufOutput
	runner := &fakeRunner{result: RunResult{Stdout: noisy, ExitCode: 0}}
	tool := NewFfufTool(runner, "", "")

	result, err := tool.Execute(context.Background(), "10.0.0.5")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Findings) != 2 {
		t.Fatalf("expected 2 findings (404 filtered out), got %d: %+v", len(result.Findings), result.Findings)
	}
}

func TestFfufToolExecuteNonZeroExit(t *testing.T) {
	runner := &fakeRunner{result: RunResult{Stderr: "ffuf: connection refused", ExitCode: 1}}
	tool := NewFfufTool(runner, "", "")

	if _, err := tool.Execute(context.Background(), "10.0.0.5"); err == nil {
		t.Fatalf("expected error for non-zero exit code")
	}
}

func TestFfufToolValidate(t *testing.T) {
	tool := NewFfufTool(&fakeRunner{}, "", "")
	if err := tool.Validate(""); err == nil {
		t.Fatalf("expected error for empty target")
	}
}

func TestFfufToolUsesDefaultWordlistWhenEmpty(t *testing.T) {
	tool := NewFfufTool(&fakeRunner{}, "", "")
	if tool.wordlist != defaultFfufWordlist {
		t.Fatalf("expected default wordlist %q, got %q", defaultFfufWordlist, tool.wordlist)
	}
}
