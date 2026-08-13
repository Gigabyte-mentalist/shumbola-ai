// Package hitl implements the human-in-the-loop approval gate required
// before any active/intrusive tool action (TZ 4.4): the Engine blocks on
// RequestApproval until an operator calls Decide, or the request's
// context is cancelled (e.g. by the kill switch).
package hitl

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrAlreadyDecided is returned by Decide when the approval has already
// been resolved.
var ErrAlreadyDecided = errors.New("hitl: approval already decided")

// ErrNotFound is returned when an approval ID doesn't exist.
var ErrNotFound = errors.New("hitl: approval not found")

// PendingApproval is a single gate awaiting an operator's decision.
type PendingApproval struct {
	ID           string    `json:"id"`
	EngagementID string    `json:"engagement_id"`
	Description  string    `json:"description"`
	CreatedAt    time.Time `json:"created_at"`

	done     chan struct{}
	approved bool
	decided  bool
}

// Store tracks pending approvals in memory, keyed by ID. Approvals are
// scoped to the lifetime of a running engagement, so no persistence is
// needed — a restart naturally requires the operator to re-approve.
type Store struct {
	mu      sync.Mutex
	byID    map[string]*PendingApproval
	nextSeq uint64
}

// NewStore builds an empty approval store.
func NewStore() *Store {
	return &Store{byID: make(map[string]*PendingApproval)}
}

// RequestApproval creates a pending approval and blocks until Decide is
// called for it or ctx is done (engagement stopped/kill switch, or the
// caller's own timeout). It returns the operator's decision, or an error
// if ctx was cancelled first.
func (s *Store) RequestApproval(ctx context.Context, engagementID, description string) (approved bool, err error) {
	s.mu.Lock()
	s.nextSeq++
	pa := &PendingApproval{
		ID:           fmt.Sprintf("%s-approval-%d", engagementID, s.nextSeq),
		EngagementID: engagementID,
		Description:  description,
		CreatedAt:    time.Now().UTC(),
		done:         make(chan struct{}),
	}
	s.byID[pa.ID] = pa
	s.mu.Unlock()

	select {
	case <-pa.done:
		// Decide already removed pa from byID under the lock; pa.approved
		// was set before done was closed, so it's safe to read here.
		return pa.approved, nil
	case <-ctx.Done():
		s.mu.Lock()
		delete(s.byID, pa.ID) // no-op if Decide already removed it
		s.mu.Unlock()
		return false, ctx.Err()
	}
}

// List returns the currently pending approvals for an engagement.
func (s *Store) List(engagementID string) []PendingApproval {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []PendingApproval
	for _, pa := range s.byID {
		if pa.EngagementID == engagementID {
			out = append(out, PendingApproval{
				ID:           pa.ID,
				EngagementID: pa.EngagementID,
				Description:  pa.Description,
				CreatedAt:    pa.CreatedAt,
			})
		}
	}
	return out
}

// Decide resolves a pending approval, unblocking the RequestApproval call
// that created it.
func (s *Store) Decide(id string, approved bool) error {
	s.mu.Lock()
	pa, ok := s.byID[id]
	if !ok {
		s.mu.Unlock()
		return ErrNotFound
	}
	if pa.decided {
		s.mu.Unlock()
		return ErrAlreadyDecided
	}
	pa.decided = true
	pa.approved = approved
	delete(s.byID, id)
	s.mu.Unlock()

	close(pa.done)
	return nil
}
