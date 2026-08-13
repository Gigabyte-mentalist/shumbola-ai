package tools

import (
	"context"
	"fmt"
)

// DefaultTestsslImage is the container image TestsslTool runs by default.
const DefaultTestsslImage = "drwetter/testssl.sh:latest"

// TestsslTool wraps `testssl.sh` for read-only TLS/certificate/protocol
// inspection: it only performs TLS handshakes, never sends application
// data or attempts to alter server state.
type TestsslTool struct {
	runner ContainerRunner
	image  string
}

// NewTestsslTool builds a TestsslTool that executes via runner. If image
// is empty, DefaultTestsslImage is used.
func NewTestsslTool(runner ContainerRunner, image string) *TestsslTool {
	if image == "" {
		image = DefaultTestsslImage
	}
	return &TestsslTool{runner: runner, image: image}
}

func (t *TestsslTool) Name() string { return "testssl" }

func (t *TestsslTool) RequiresApproval() bool { return true }

func (t *TestsslTool) Validate(target string) error {
	return validateTarget("testssl", target)
}

func (t *TestsslTool) Execute(ctx context.Context, target string) (Result, error) {
	if err := t.Validate(target); err != nil {
		return Result{}, err
	}

	// --jsonfile-pretty -: structured JSON report to stdout instead of a
	// file, so we can parse it directly from the container's output.
	res, err := t.runner.RunContainer(ctx, RunSpec{
		Image:   t.image,
		Cmd:     []string{"--jsonfile-pretty", "-", target},
		Network: "bridge",
		// testssl.sh needs to mktemp scratch files during a scan — a
		// read-only rootfs breaks it before it can report anything.
		ReadOnlyFS: false,
		ExtraHosts: ResolveExtraHost(target),
	})
	if err != nil {
		return Result{}, fmt.Errorf("testssl: run container: %w", err)
	}

	findings, err := parseTestsslOutput(res.Stdout)
	if err != nil {
		return Result{Raw: res.Stdout}, fmt.Errorf("testssl: parse output: %w", err)
	}
	return Result{Raw: res.Stdout, Findings: findings}, nil
}
