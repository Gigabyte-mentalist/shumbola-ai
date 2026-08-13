package tools

import (
	"context"
	"fmt"
)

// DefaultSmbEnumImage reuses the same nmap image as NmapTool (TZ 4.3 lists
// SMB enumeration as network recon, and nmap's own bundled NSE scripts
// cover it — no separate tool/image needed).
const DefaultSmbEnumImage = "instrumentisto/nmap:latest"

// smbEnumScripts are the NSE scripts SmbEnumTool runs. All are read-only
// enumeration over an SMB session (anonymous where the server allows it)
// — no credential guessing, no writes. smb-enum-users is deliberately
// excluded even though it exists: nmap tags it "auth intrusive" because
// it probes for valid usernames, which can look like the opening move of
// a brute-force/lockout attack on some SMB configurations — out of scope
// for the non-destructive MVP (TZ 2.1).
const smbEnumScripts = "smb-os-discovery,smb-protocols,smb2-security-mode,smb-enum-shares"

// SmbEnumTool wraps nmap's SMB NSE scripts for read-only Windows/Samba
// service enumeration (OS/version, supported protocol dialects, signing
// requirements, share names) — the recon gap left by NmapTool's plain
// -sV, which only reports "microsoft-ds"/"netbios-ssn" as opaque service
// names without asking the SMB service itself for detail.
type SmbEnumTool struct {
	runner ContainerRunner
	image  string
}

// NewSmbEnumTool builds a SmbEnumTool that executes via runner. If image
// is empty, DefaultSmbEnumImage is used.
func NewSmbEnumTool(runner ContainerRunner, image string) *SmbEnumTool {
	if image == "" {
		image = DefaultSmbEnumImage
	}
	return &SmbEnumTool{runner: runner, image: image}
}

func (t *SmbEnumTool) Name() string { return "smbenum" }

func (t *SmbEnumTool) RequiresApproval() bool { return true }

func (t *SmbEnumTool) Validate(target string) error {
	return validateTarget("smbenum", target)
}

func (t *SmbEnumTool) Execute(ctx context.Context, target string) (Result, error) {
	if err := t.Validate(target); err != nil {
		return Result{}, err
	}

	// -Pn: skip host discovery (same reasoning as NmapTool). -p 139,445:
	// SMB only runs on these two ports, no point probing the rest.
	res, err := t.runner.RunContainer(ctx, RunSpec{
		Image:      t.image,
		Cmd:        []string{"-Pn", "-p", "139,445", "--script", smbEnumScripts, target},
		Network:    "bridge",
		ReadOnlyFS: true,
		ExtraHosts: ResolveExtraHost(target),
	})
	if err != nil {
		return Result{}, fmt.Errorf("smbenum: run container: %w", err)
	}
	if res.ExitCode != 0 {
		return Result{Raw: res.Stdout + res.Stderr}, fmt.Errorf("smbenum: exited with code %d: %s", res.ExitCode, res.Stderr)
	}

	return Result{
		Raw:      res.Stdout,
		Findings: parseSmbEnumOutput(res.Stdout),
	}, nil
}
