package tools

import "testing"

func TestResolveExtraHostSkipsIPTargets(t *testing.T) {
	if got := ResolveExtraHost("10.0.0.5"); got != nil {
		t.Fatalf("expected nil for an IP target, got %v", got)
	}
}

func TestResolveExtraHostSkipsIPWithPort(t *testing.T) {
	if got := ResolveExtraHost("10.0.0.5:8443"); got != nil {
		t.Fatalf("expected nil for an IP:port target, got %v", got)
	}
}

func TestResolveExtraHostResolvesRealHostname(t *testing.T) {
	// localhost is guaranteed resolvable without any real network
	// access, so this exercises the actual net.LookupHost path without
	// depending on external connectivity in CI.
	got := ResolveExtraHost("localhost")
	if len(got) != 1 {
		t.Fatalf("expected one ExtraHosts entry for localhost, got %v", got)
	}
	if got[0][:len("localhost:")] != "localhost:" {
		t.Fatalf("expected entry to start with %q, got %q", "localhost:", got[0])
	}
}

func TestResolveExtraHostUnresolvableReturnsNil(t *testing.T) {
	got := ResolveExtraHost("this-domain-should-not-exist.invalid")
	if got != nil {
		t.Fatalf("expected nil for an unresolvable host, got %v", got)
	}
}
