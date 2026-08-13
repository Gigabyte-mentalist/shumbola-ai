package tools

import (
	"context"
	"fmt"
)

// DefaultNucleiImage is the container image NucleiTool runs by default.
const DefaultNucleiImage = "projectdiscovery/nuclei:latest"

// NucleiTool wraps `nuclei` for template-based vulnerability/CVE
// detection against a target, using its default (non-destructive)
// template set. Like every active scanner, it requires HITL approval
// (TZ 4.4).
type NucleiTool struct {
	runner ContainerRunner
	image  string
}

// NewNucleiTool builds a NucleiTool that executes via runner. If image is
// empty, DefaultNucleiImage is used.
func NewNucleiTool(runner ContainerRunner, image string) *NucleiTool {
	if image == "" {
		image = DefaultNucleiImage
	}
	return &NucleiTool{runner: runner, image: image}
}

func (t *NucleiTool) Name() string { return "nuclei" }

func (t *NucleiTool) RequiresApproval() bool { return true }

func (t *NucleiTool) Validate(target string) error {
	return validateTarget("nuclei", target)
}

func (t *NucleiTool) Execute(ctx context.Context, target string) (Result, error) {
	if err := t.Validate(target); err != nil {
		return Result{}, err
	}

	// -jsonl: one JSON object per match on stdout, easy to parse
	// line-by-line. No -templates override — the default template set
	// ships with detection-only checks, no exploitation.
	res, err := t.runner.RunContainer(ctx, RunSpec{
		Image:   t.image,
		Cmd:     []string{"-target", target, "-jsonl", "-silent"},
		Network: "bridge",
		// nuclei writes its config/cache dir under $HOME on startup even
		// for a single scan — a read-only rootfs makes it fail before it
		// ever reaches the target.
		ReadOnlyFS: false,
		ExtraHosts: ResolveExtraHost(target),
	})
	if err != nil {
		return Result{}, fmt.Errorf("nuclei: run container: %w", err)
	}
	// Unlike nmap, nuclei exits non-zero when it simply finds nothing to
	// report on some versions — treat exit code as informational only
	// and rely on parsed output.
	return Result{
		Raw:      res.Stdout,
		Findings: parseNucleiOutput(res.Stdout),
	}, nil
}
