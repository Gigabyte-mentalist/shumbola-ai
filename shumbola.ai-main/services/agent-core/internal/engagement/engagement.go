// Package engagement models a pentest engagement and its scope allowlist.
// TZ 4.1: operator ruxsat berilgan nishonlar ro'yxatini kiritadi, va bu
// allowlist tizim darajasida majburlanadi (TZ 6.1) — the agent must never
// touch a target outside Scope, regardless of what the planner decides.
package engagement

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// Status is the lifecycle state of an engagement.
type Status string

const (
	StatusCreated Status = "created"
	StatusRunning Status = "running"
	StatusDone    Status = "done"
	StatusStopped Status = "stopped" // operator kill switch
	StatusError   Status = "error"
)

// Mode is how far an engagement is authorized to act. It defaults to
// ModeDetect, matching every existing engagement/behavior from before
// the offensive extension (shumbola_offensive_TZ.md) existed — nothing
// about detect-only engagements changes. ModeExploit additionally
// requires a valid, two-person-approved RoE document (see the roe and
// exploitgate packages) before any exploitation adapter (Bosqich 7/8)
// will act, regardless of this field — Mode alone is necessary but not
// sufficient (FR-AUTHZ-1, FR-SAFE-9: fail-closed).
type Mode string

const (
	ModeDetect  Mode = "detect"
	ModeExploit Mode = "exploit"
)

// AuthzLevel is the per-target authorization a scope entry carries
// (FR-AUTHZ-2). It is independent of Mode: Mode gates the engagement as
// a whole, AuthzLevel gates which specific targets within it may ever be
// exploited even once Mode is ModeExploit.
type AuthzLevel string

const (
	// AuthzReconOnly permits passive/information-gathering activity only.
	AuthzReconOnly AuthzLevel = "recon-only"
	// AuthzDetectOnly permits active scanning/detection (today's entire
	// tool set — nmap, nuclei, ffuf, testssl, smbenum) but no
	// exploitation. This is the default for any target with no explicit
	// entry, so existing engagements/scopes are unaffected.
	AuthzDetectOnly AuthzLevel = "detect-only"
	// AuthzExploitAllowed additionally permits validated exploitation
	// (Bosqich 7/8), subject to every other FR-AUTHZ/FR-SAFE gate.
	AuthzExploitAllowed AuthzLevel = "exploit-allowed"
)

// Scope is the allowlist of targets an engagement is permitted to touch.
// Entries may be exact IPs, CIDR blocks, or domain names.
type Scope struct {
	Targets []string `json:"targets"`
	// Authz maps a target (matching a Targets entry exactly) to its
	// authorization level. A target absent from this map is
	// AuthzDetectOnly — the safe default — not AuthzReconOnly, since
	// that's what every tool this system already runs needs.
	Authz map[string]AuthzLevel `json:"authz,omitempty"`
}

// AuthzFor reports target's authorization level, defaulting to
// AuthzDetectOnly when unset.
func (s Scope) AuthzFor(target string) AuthzLevel {
	if lvl, ok := s.Authz[target]; ok {
		return lvl
	}
	return AuthzDetectOnly
}

// Allows reports whether host is covered by the scope. IP hosts are
// matched against exact IPs and CIDR entries; non-IP hosts are matched by
// exact (case-insensitive) domain match. This is the single choke point
// tool execution must pass through before touching a target.
func (s Scope) Allows(host string) bool {
	ip := net.ParseIP(host)
	for _, entry := range s.Targets {
		if ip != nil {
			if _, cidr, err := net.ParseCIDR(entry); err == nil {
				if cidr.Contains(ip) {
					return true
				}
				continue
			}
			if entryIP := net.ParseIP(entry); entryIP != nil && entryIP.Equal(ip) {
				return true
			}
			continue
		}
		if strings.EqualFold(entry, host) {
			return true
		}
	}
	return false
}

// Engagement is a single pentest run against a fixed Scope.
type Engagement struct {
	ID         string    `json:"id"`
	Scope      Scope     `json:"scope"`
	Status     Status    `json:"status"`
	StatusNote string    `json:"status_note,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	CreatedBy  string    `json:"created_by"`
	// Mode defaults to ModeDetect; see the Mode doc comment. A JSON
	// omitempty is deliberately NOT used here — an engagement record
	// with no "mode" field at all (e.g. hand-edited, or from a store
	// bug) must decode to the zero value "" and be treated as anything
	// but ModeExploit, never silently default to permissive.
	Mode Mode `json:"mode"`
}

// New creates an Engagement with a fresh Scope. It performs no I/O; use a
// Store to persist it.
func New(id, createdBy string, targets []string) (*Engagement, error) {
	if id == "" {
		return nil, fmt.Errorf("engagement: id is required")
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("engagement: scope must include at least one target")
	}
	normalized := make([]string, 0, len(targets))
	for _, t := range targets {
		host, err := normalizeTarget(t)
		if err != nil {
			return nil, fmt.Errorf("engagement: target %q: %w", t, err)
		}
		normalized = append(normalized, host)
	}
	return &Engagement{
		ID:        id,
		Scope:     Scope{Targets: normalized},
		Status:    StatusCreated,
		CreatedAt: time.Now().UTC(),
		CreatedBy: createdBy,
		Mode:      ModeDetect,
	}, nil
}

// normalizeTarget turns an operator-supplied target into the bare
// host[:port] every tool wrapper (nmap, ffuf, ...) actually expects.
// Operators naturally paste whatever's in their browser's address bar —
// a full URL, scheme and path included — but a tool like nmap doesn't
// reject that outright: given "https://example.com/path" it treats the
// whole string as an unparsable target expression, scans zero hosts, and
// exits 0, which without this normalization would silently look like a
// clean "no findings" result instead of the misconfigured scope it
// actually is.
func normalizeTarget(raw string) (string, error) {
	target := strings.TrimSpace(raw)
	if target == "" {
		return "", fmt.Errorf("target must not be empty")
	}

	if strings.Contains(target, "://") {
		u, err := url.Parse(target)
		if err != nil || u.Host == "" {
			return "", fmt.Errorf("could not extract a host from URL")
		}
		return u.Host, nil
	}

	// A bare "host/path" (no scheme) — same reasoning, strip the path.
	if idx := strings.IndexAny(target, "/?#"); idx != -1 {
		target = target[:idx]
	}
	if target == "" {
		return "", fmt.Errorf("no host found")
	}
	return target, nil
}
