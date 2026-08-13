package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendAndVerify(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	logger, err := NewLogger(path)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	if _, err := logger.Append("operator-1", "scope.set", "engagement-1", map[string]any{"targets": []string{"10.0.0.0/24"}}); err != nil {
		t.Fatalf("Append 1: %v", err)
	}
	if _, err := logger.Append("system", "tool.invoke", "engagement-1", map[string]any{"tool": "nmap"}); err != nil {
		t.Fatalf("Append 2: %v", err)
	}
	e3, err := logger.Append("operator-1", "hitl.approve", "finding-7", nil)
	if err != nil {
		t.Fatalf("Append 3: %v", err)
	}
	if e3.PrevHash == GenesisHash {
		t.Fatalf("expected third entry to chain from second entry, not genesis")
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err := Verify(path); err != nil {
		t.Fatalf("Verify on untouched log: %v", err)
	}
}

func TestVerifyDetectsTampering(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	logger, err := NewLogger(path)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	if _, err := logger.Append("operator-1", "scope.set", "engagement-1", nil); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if _, err := logger.Append("operator-1", "hitl.approve", "finding-1", nil); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	tampered := strings.Replace(string(raw), "hitl.approve", "hitl.reject", 1)
	if tampered == string(raw) {
		t.Fatalf("tampering substitution did not change file content")
	}
	if err := os.WriteFile(path, []byte(tampered), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := Verify(path); err == nil {
		t.Fatalf("expected Verify to detect tampering, got nil error")
	}
}

func TestLoggerResumesChainAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	l1, err := NewLogger(path)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	first, err := l1.Append("operator-1", "scope.set", "engagement-1", nil)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := l1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	l2, err := NewLogger(path)
	if err != nil {
		t.Fatalf("NewLogger (reopen): %v", err)
	}
	second, err := l2.Append("operator-1", "engagement.start", "engagement-1", nil)
	if err != nil {
		t.Fatalf("Append after reopen: %v", err)
	}
	if err := l2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if second.PrevHash != first.Hash {
		t.Fatalf("expected chain to continue after reopen: got prev_hash %s, want %s", second.PrevHash, first.Hash)
	}
	if err := Verify(path); err != nil {
		t.Fatalf("Verify after reopen: %v", err)
	}
}
