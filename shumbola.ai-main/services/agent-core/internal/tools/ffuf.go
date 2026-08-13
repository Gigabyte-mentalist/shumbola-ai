package tools

import (
	"context"
	"fmt"
)

// DefaultFfufImage is the container image FfufTool runs by default: a
// thin layer over secsi/ffuf (the public ffuf CLI image — the project's
// own ghcr.io/ffuf/ffuf image is not publicly pullable) that adds a
// small built-in wordlist so no external volume mount is required for a
// basic scan. Built from tool-images/ffuf/Dockerfile.
const DefaultFfufImage = "shumbola/ffuf:latest"

// defaultFfufWordlist is a small, built-in-image wordlist path, kept
// intentionally short — Bosqich 2 favors a quick, non-destructive
// discovery pass over an exhaustive one.
const defaultFfufWordlist = "/usr/share/wordlists/common.txt"

// FfufTool wraps `ffuf` for non-destructive content/directory discovery:
// it only issues GET requests for candidate paths, never submits data.
type FfufTool struct {
	runner   ContainerRunner
	image    string
	wordlist string
}

// NewFfufTool builds an FfufTool that executes via runner. If image or
// wordlist is empty, the package defaults are used.
func NewFfufTool(runner ContainerRunner, image, wordlist string) *FfufTool {
	if image == "" {
		image = DefaultFfufImage
	}
	if wordlist == "" {
		wordlist = defaultFfufWordlist
	}
	return &FfufTool{runner: runner, image: image, wordlist: wordlist}
}

func (t *FfufTool) Name() string { return "ffuf" }

func (t *FfufTool) RequiresApproval() bool { return true }

func (t *FfufTool) Validate(target string) error {
	return validateTarget("ffuf", target)
}

func (t *FfufTool) Execute(ctx context.Context, target string) (Result, error) {
	if err := t.Validate(target); err != nil {
		return Result{}, err
	}

	// -of json -o /dev/stdout: JSON results to stdout. "-o -" looks like
	// it should mean stdout too, but this ffuf build (secsi/ffuf v2.2.1)
	// silently ignores it and never writes JSON at all — confirmed by
	// running it directly; /dev/stdout is the form that actually works.
	// -mc all: report every status code we see rather than
	// pre-filtering, so the parser (not ffuf's own matcher) decides what
	// counts as a Finding. No -s (silent mode): combined with the
	// default multi-threaded run, -s crashes this ffuf build with a nil
	// pointer panic in its own progress-rate tracking (also confirmed by
	// running it directly) — so the live progress bar's ANSI escape
	// codes still land on stdout ahead of the JSON; parseFfufOutput
	// strips that noise instead.
	res, err := t.runner.RunContainer(ctx, RunSpec{
		Image:   t.image,
		Cmd:     []string{"-u", fmt.Sprintf("http://%s/FUZZ", target), "-w", t.wordlist, "-mc", "all", "-of", "json", "-o", "/dev/stdout"},
		Network: "bridge",
		// ffuf writes its default config dir under $HOME on startup even
		// though -o - sends results to stdout — read-only rootfs breaks it.
		ReadOnlyFS: false,
		ExtraHosts: ResolveExtraHost(target),
	})
	if err != nil {
		return Result{}, fmt.Errorf("ffuf: run container: %w", err)
	}
	if res.ExitCode != 0 {
		return Result{Raw: res.Stdout + res.Stderr}, fmt.Errorf("ffuf: exited with code %d: %s", res.ExitCode, res.Stderr)
	}

	findings, err := parseFfufOutput(res.Stdout)
	if err != nil {
		return Result{Raw: res.Stdout}, fmt.Errorf("ffuf: parse output: %w", err)
	}
	return Result{Raw: res.Stdout, Findings: findings}, nil
}
