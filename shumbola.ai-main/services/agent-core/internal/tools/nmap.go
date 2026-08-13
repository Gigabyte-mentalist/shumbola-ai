package tools

import (
	"context"
	"fmt"
)

// ContainerRunner is the subset of sandbox.Manager's behavior NmapTool
// depends on. Depending on this narrow interface (rather than
// *sandbox.Manager directly) lets tests substitute a fake runner instead
// of requiring a real Docker daemon.
type ContainerRunner interface {
	RunContainer(ctx context.Context, spec RunSpec) (RunResult, error)
}

// RunSpec mirrors sandbox.RunSpec — duplicated here so this package does
// not need to import sandbox, keeping the tools/sandbox dependency
// direction one-way (sandbox knows nothing about tools).
type RunSpec struct {
	Image       string
	Cmd         []string
	MemoryBytes int64
	NanoCPUs    int64
	Network     string
	ReadOnlyFS  bool
	// ExtraHosts injects "hostname:ip" entries into the container's
	// /etc/hosts (see ResolveExtraHost) — every resolver checks hosts
	// files before the network, so this lets container DNS resolution
	// succeed even when the container's own DNS path can't.
	ExtraHosts []string
}

// RunResult mirrors sandbox.RunResult.
type RunResult struct {
	Stdout   string
	Stderr   string
	ExitCode int64
}

// DefaultNmapImage is the container image NmapTool runs by default.
const DefaultNmapImage = "instrumentisto/nmap:latest"

// NmapTool wraps `nmap` for non-destructive service/version detection
// (TZ 4.3). It is intrusive/active by nature, so RequiresApproval is
// always true (TZ 4.4).
type NmapTool struct {
	runner ContainerRunner
	image  string
}

// NewNmapTool builds an NmapTool that executes via runner. If image is
// empty, DefaultNmapImage is used.
func NewNmapTool(runner ContainerRunner, image string) *NmapTool {
	if image == "" {
		image = DefaultNmapImage
	}
	return &NmapTool{runner: runner, image: image}
}

func (t *NmapTool) Name() string { return "nmap" }

func (t *NmapTool) RequiresApproval() bool { return true }

func (t *NmapTool) Validate(target string) error {
	return validateTarget("nmap", target)
}

func (t *NmapTool) Execute(ctx context.Context, target string) (Result, error) {
	if err := t.Validate(target); err != nil {
		return Result{}, err
	}

	// -sV: service/version detection. -Pn: skip host discovery (works
	// through firewalls that drop ping). -T4 ("Aggressive" timing):
	// nmap's default -T3 spends a long time retransmitting to ports a
	// filtering firewall/WAF is silently dropping — observed in practice
	// against a real, protected internet host taking long enough to hit
	// Engine's own toolExecutionTimeout even for a single default-range
	// scan. -T4 cuts per-probe wait/retry budgets significantly; still
	// well short of anything that risks flooding/DoS-ing the target.
	// Neither flag performs any exploitation — non-destructive per TZ
	// 2.1.
	res, err := t.runner.RunContainer(ctx, RunSpec{
		Image:      t.image,
		Cmd:        []string{"-sV", "-Pn", "-T4", target},
		Network:    "bridge", // nmap needs outbound network access to reach the target
		ReadOnlyFS: true,
		ExtraHosts: ResolveExtraHost(target),
	})
	if err != nil {
		return Result{}, fmt.Errorf("nmap: run container: %w", err)
	}
	if res.ExitCode != 0 {
		return Result{Raw: res.Stdout + res.Stderr}, fmt.Errorf("nmap: exited with code %d: %s", res.ExitCode, res.Stderr)
	}

	return Result{
		Raw:      res.Stdout,
		Findings: parseNmapOutput(res.Stdout),
	}, nil
}
