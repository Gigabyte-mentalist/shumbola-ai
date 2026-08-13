package hitl

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestRequestApprovalBlocksUntilDecided(t *testing.T) {
	store := NewStore()
	var wg sync.WaitGroup
	var approved bool
	var reqErr error

	wg.Add(1)
	go func() {
		defer wg.Done()
		approved, reqErr = store.RequestApproval(context.Background(), "eng-1", "run nmap against 10.0.0.5")
	}()

	// Give the goroutine a moment to register the pending approval.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if pending := store.List("eng-1"); len(pending) == 1 {
			if err := store.Decide(pending[0].ID, true); err != nil {
				t.Fatalf("Decide: %v", err)
			}
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	wg.Wait()
	if reqErr != nil {
		t.Fatalf("RequestApproval: %v", reqErr)
	}
	if !approved {
		t.Fatalf("expected approved=true")
	}
	if pending := store.List("eng-1"); len(pending) != 0 {
		t.Fatalf("expected no pending approvals after decision, got %d", len(pending))
	}
}

func TestRequestApprovalCancelledByContext(t *testing.T) {
	store := NewStore()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	var reqErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, reqErr = store.RequestApproval(ctx, "eng-1", "run nmap")
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(store.List("eng-1")) == 1 {
			cancel()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	wg.Wait()
	if reqErr == nil {
		t.Fatalf("expected error when context is cancelled before decision")
	}
}

func TestDecideUnknownID(t *testing.T) {
	store := NewStore()
	if err := store.Decide("does-not-exist", true); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDecideTwiceFails(t *testing.T) {
	store := NewStore()
	go func() { _, _ = store.RequestApproval(context.Background(), "eng-1", "desc") }()

	deadline := time.Now().Add(2 * time.Second)
	var id string
	for time.Now().Before(deadline) {
		if pending := store.List("eng-1"); len(pending) == 1 {
			id = pending[0].ID
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if id == "" {
		t.Fatalf("approval never appeared")
	}
	if err := store.Decide(id, true); err != nil {
		t.Fatalf("first Decide: %v", err)
	}
	if err := store.Decide(id, true); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for a second decide on a resolved+removed approval, got %v", err)
	}
}
