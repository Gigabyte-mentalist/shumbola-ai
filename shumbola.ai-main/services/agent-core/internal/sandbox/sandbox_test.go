package sandbox

import (
	"context"
	"testing"
	"time"
)

// TestPing only runs meaningfully when a Docker daemon is reachable
// (e.g. in CI/dev with Docker Desktop running). It skips otherwise so the
// suite stays green on machines without Docker.
func TestPing(t *testing.T) {
	m, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	defer m.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := m.Ping(ctx); err != nil {
		t.Skipf("docker daemon not reachable, skipping: %v", err)
	}
}
