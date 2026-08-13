package tools

import "net"

// ResolveExtraHost returns a Docker ExtraHosts entry ("host:ip") that
// makes the container's own /etc/hosts resolve target — every resolver
// (nmap's own, glibc, musl, ...) checks /etc/hosts before ever making a
// network DNS query, so this lets tool containers reach a hostname
// target reliably using THIS machine's own (already-working) resolver,
// regardless of whether the sandboxed container's own DNS path works.
//
// Found necessary in practice, not preemptively (2026-08-13): on one
// tested network, in-container DNS resolution hung for tens of seconds
// to several minutes — even with the Docker daemon's own "dns" setting
// pointed at a nameserver confirmed reachable and correct — while the
// host machine's resolver (used here, via net.LookupHost) resolved the
// same name in milliseconds. Root cause not fully isolated (something in
// how the container's system resolver reaches the configured nameserver
// over the sandbox network vs. a tool's own resolver implementation
// doing the same lookup), so this sidesteps the whole path rather than
// depending on it working.
//
// Returns nil if target is already an IP (nothing to resolve) or
// resolution fails — in both cases the caller's RunSpec.ExtraHosts ends
// up empty, so the tool falls back to whatever DNS behavior it already
// had.
func ResolveExtraHost(target string) []string {
	host := target
	if h, _, err := net.SplitHostPort(target); err == nil {
		host = h
	}
	if net.ParseIP(host) != nil {
		return nil
	}

	ips, err := net.LookupHost(host)
	if err != nil || len(ips) == 0 {
		return nil
	}
	return []string{host + ":" + ips[0]}
}
