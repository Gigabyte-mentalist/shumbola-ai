// Package sandbox provides isolated container execution for pentesting
// tools (TZ 6.1: "tool'lar izolyatsiyalangan muhitda ishlaydi"). This is
// a Bosqich 0 skeleton — the Tool Layer (Bosqich 2) will call RunContainer
// for each wrapped tool (nmap, nuclei, ...).
package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// Manager wraps a Docker client to run tool invocations in isolated,
// resource-bounded containers.
type Manager struct {
	cli *client.Client
}

// NewManager connects to the Docker daemon using environment defaults
// (DOCKER_HOST, etc.).
func NewManager() (*Manager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("sandbox: create docker client: %w", err)
	}
	return &Manager{cli: cli}, nil
}

// Close releases the underlying Docker client connection.
func (m *Manager) Close() error {
	return m.cli.Close()
}

// Ping verifies the Docker daemon is reachable.
func (m *Manager) Ping(ctx context.Context) error {
	if _, err := m.cli.Ping(ctx); err != nil {
		return fmt.Errorf("sandbox: docker daemon unreachable: %w", err)
	}
	return nil
}

// RunSpec describes a single isolated, non-destructive tool invocation.
type RunSpec struct {
	Image       string
	Cmd         []string
	MemoryBytes int64  // 0 = daemon default
	NanoCPUs    int64  // 0 = daemon default
	Network     string // "" defaults to "none" (no network access)
	ReadOnlyFS  bool
	// ExtraHosts entries ("hostname:ip") are injected into the
	// container's /etc/hosts (Docker's ExtraHosts host-config field).
	ExtraHosts []string
}

// RunResult carries the captured output of a finished container run.
type RunResult struct {
	Stdout   string
	Stderr   string
	ExitCode int64
}

// RunContainer creates, starts, waits for, and removes a single container
// running spec.Cmd in spec.Image, isolated per TZ 6.1: no network access by
// default, and resource limits are enforced when set. It is intended to be
// called by tool wrappers (Bosqich 2), not directly by the agent core.
func (m *Manager) RunContainer(ctx context.Context, spec RunSpec) (RunResult, error) {
	networkMode := spec.Network
	if networkMode == "" {
		networkMode = "none"
	}

	resp, err := m.cli.ContainerCreate(ctx, &container.Config{
		Image: spec.Image,
		Cmd:   spec.Cmd,
	}, &container.HostConfig{
		NetworkMode:    container.NetworkMode(networkMode),
		ReadonlyRootfs: spec.ReadOnlyFS,
		ExtraHosts:     spec.ExtraHosts,
		Resources: container.Resources{
			Memory:   spec.MemoryBytes,
			NanoCPUs: spec.NanoCPUs,
		},
		AutoRemove: false, // removed explicitly below so we can still read logs on failure
	}, nil, nil, "")
	if err != nil {
		return RunResult{}, fmt.Errorf("sandbox: create container: %w", err)
	}
	containerID := resp.ID
	defer func() {
		_ = m.cli.ContainerRemove(context.Background(), containerID, container.RemoveOptions{Force: true})
	}()

	if err := m.cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return RunResult{}, fmt.Errorf("sandbox: start container: %w", err)
	}

	statusCh, errCh := m.cli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	var exitCode int64
	select {
	case err := <-errCh:
		if err != nil {
			return RunResult{}, fmt.Errorf("sandbox: wait for container: %w", err)
		}
	case status := <-statusCh:
		exitCode = status.StatusCode
	}

	logs, err := m.cli.ContainerLogs(ctx, containerID, container.LogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		return RunResult{}, fmt.Errorf("sandbox: read container logs: %w", err)
	}
	defer logs.Close()

	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, logs); err != nil && err != io.EOF {
		return RunResult{}, fmt.Errorf("sandbox: demux container logs: %w", err)
	}

	return RunResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}, nil
}
