package engagement

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestScopeAllowsCIDR(t *testing.T) {
	s := Scope{Targets: []string{"10.0.0.0/24", "example.com"}}

	if !s.Allows("10.0.0.5") {
		t.Fatalf("expected 10.0.0.5 to be in scope (CIDR match)")
	}
	if s.Allows("10.0.1.5") {
		t.Fatalf("expected 10.0.1.5 to be out of scope")
	}
	if !s.Allows("example.com") {
		t.Fatalf("expected example.com to be in scope (exact domain match)")
	}
	if s.Allows("evil.com") {
		t.Fatalf("expected evil.com to be out of scope")
	}
}

func TestScopeAllowsExactIP(t *testing.T) {
	s := Scope{Targets: []string{"192.168.1.10"}}
	if !s.Allows("192.168.1.10") {
		t.Fatalf("expected exact IP match to be in scope")
	}
	if s.Allows("192.168.1.11") {
		t.Fatalf("expected different IP to be out of scope")
	}
}

func TestFileStoreSaveLoadUpdate(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	e, err := New("eng-1", "op-1", []string{"10.0.0.0/24"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := store.Save(e); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load("eng-1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Status != StatusCreated {
		t.Fatalf("expected status %s, got %s", StatusCreated, loaded.Status)
	}

	updated, err := store.Update("eng-1", func(e *Engagement) error {
		e.Status = StatusRunning
		return nil
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Status != StatusRunning {
		t.Fatalf("expected status %s after update, got %s", StatusRunning, updated.Status)
	}

	reloaded, err := store.Load("eng-1")
	if err != nil {
		t.Fatalf("Load after update: %v", err)
	}
	if reloaded.Status != StatusRunning {
		t.Fatalf("expected persisted status %s, got %s", StatusRunning, reloaded.Status)
	}
}

func TestFileStoreLoadMissing(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	if _, err := store.Load("does-not-exist"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestNewRejectsEmptyScope(t *testing.T) {
	if _, err := New("eng-1", "op-1", nil); err == nil {
		t.Fatalf("expected error for empty scope")
	}
}

// TestNewNormalizesURLTargets covers a real-world operator mistake: pasting
// a full URL (scheme, and often a path) instead of a bare host. nmap
// doesn't reject "https://example.com/path" outright — it treats the
// whole string as an unparsable target, scans zero hosts, and exits 0,
// which without normalization looks exactly like a clean "no findings"
// result instead of the misconfigured scope it actually is (found via a
// real engagement stuck at empty task tree, 2026-08-13).
func TestNewNormalizesURLTargets(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://university.pdp.uz/uz", "university.pdp.uz"},
		{"http://example.com", "example.com"},
		{"https://example.com:8443/a/b?x=1", "example.com:8443"},
		{"example.com/some/path", "example.com"},
		{"  10.0.0.5  ", "10.0.0.5"},
	}
	for _, tc := range cases {
		eng, err := New("eng-1", "op-1", []string{tc.in})
		if err != nil {
			t.Fatalf("New(%q): %v", tc.in, err)
		}
		if got := eng.Scope.Targets[0]; got != tc.want {
			t.Fatalf("New(%q): got target %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNewRejectsUnparsableURLTarget(t *testing.T) {
	if _, err := New("eng-1", "op-1", []string{"https://"}); err == nil {
		t.Fatalf("expected error for a URL with no host")
	}
}

func TestFileStoreList(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	e1, err := New("eng-1", "op-1", []string{"10.0.0.5"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	e1.CreatedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := store.Save(e1); err != nil {
		t.Fatalf("Save e1: %v", err)
	}

	e2, err := New("eng-2", "op-1", []string{"10.0.0.6"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	e2.CreatedAt = time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	if err := store.Save(e2); err != nil {
		t.Fatalf("Save e2: %v", err)
	}

	list, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 engagements, got %d", len(list))
	}
	if list[0].ID != "eng-2" || list[1].ID != "eng-1" {
		t.Fatalf("expected most-recently-created first, got order: %s, %s", list[0].ID, list[1].ID)
	}
}

func TestFileStoreListEmpty(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	list, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected empty list, got %d", len(list))
	}
}

func TestNewDefaultsToDetectMode(t *testing.T) {
	eng, err := New("eng-1", "op-1", []string{"10.0.0.5"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if eng.Mode != ModeDetect {
		t.Fatalf("expected new engagements to default to ModeDetect, got %q", eng.Mode)
	}
}

func TestScopeAuthzForDefaultsToDetectOnly(t *testing.T) {
	s := Scope{Targets: []string{"10.0.0.5", "10.0.0.6"}, Authz: map[string]AuthzLevel{
		"10.0.0.6": AuthzExploitAllowed,
	}}
	if got := s.AuthzFor("10.0.0.5"); got != AuthzDetectOnly {
		t.Fatalf("expected default AuthzDetectOnly for a target with no explicit entry, got %q", got)
	}
	if got := s.AuthzFor("10.0.0.6"); got != AuthzExploitAllowed {
		t.Fatalf("expected AuthzExploitAllowed for the explicitly authorized target, got %q", got)
	}
}

// TestLegacyRecordWithoutModeFieldStaysNonExploit covers fail-closed
// behavior for engagements persisted before the offensive extension
// existed: their JSON has no "mode" key at all. Unmarshaling must leave
// Mode as the zero value "", which is never ModeExploit — a pre-existing
// record must never silently become exploit-authorized just because a
// newer binary reads it.
func TestLegacyRecordWithoutModeFieldStaysNonExploit(t *testing.T) {
	legacyJSON := `{
		"id": "eng-old",
		"scope": {"targets": ["10.0.0.5"]},
		"status": "created",
		"created_at": "2026-01-01T00:00:00Z",
		"created_by": "op-1"
	}`
	var eng Engagement
	if err := json.Unmarshal([]byte(legacyJSON), &eng); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if eng.Mode == ModeExploit {
		t.Fatalf("legacy record with no mode field must not decode as ModeExploit")
	}
}

func TestFileStorePathUsesDir(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	if got := store.path("eng-1"); got != filepath.Join(dir, "eng-1.json") {
		t.Fatalf("unexpected path: %s", got)
	}
}
