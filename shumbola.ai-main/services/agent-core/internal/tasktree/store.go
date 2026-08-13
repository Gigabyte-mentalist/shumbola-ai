package tasktree

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ErrNotFound is returned when an engagement has no persisted task tree yet.
var ErrNotFound = errors.New("tasktree: not found")

// FileStore persists one task tree per engagement as JSON, so state
// survives a restart (TZ 6.3).
type FileStore struct {
	mu  sync.Mutex
	dir string
}

// NewFileStore ensures dir exists and returns a FileStore backed by it.
func NewFileStore(dir string) (*FileStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("tasktree: create store dir: %w", err)
	}
	return &FileStore{dir: dir}, nil
}

func (s *FileStore) path(engagementID string) string {
	return filepath.Join(s.dir, engagementID+".json")
}

// Save persists the tree's current snapshot.
func (s *FileStore) Save(t *Tree) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(struct {
		EngagementID string `json:"engagement_id"`
		Nodes        []Node `json:"nodes"`
	}{t.EngagementID, t.Snapshot()}, "", "  ")
	if err != nil {
		return fmt.Errorf("tasktree: marshal: %w", err)
	}
	if err := os.WriteFile(s.path(t.EngagementID), data, 0o600); err != nil {
		return fmt.Errorf("tasktree: write: %w", err)
	}
	return nil
}

// LoadOrNew returns the persisted tree for engagementID, or a fresh empty
// tree if none exists yet.
func (s *FileStore) LoadOrNew(engagementID string) (*Tree, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path(engagementID))
	if errors.Is(err, os.ErrNotExist) {
		return New(engagementID), nil
	}
	if err != nil {
		return nil, fmt.Errorf("tasktree: read: %w", err)
	}
	var raw struct {
		EngagementID string `json:"engagement_id"`
		Nodes        []Node `json:"nodes"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("tasktree: unmarshal: %w", err)
	}
	return &Tree{EngagementID: raw.EngagementID, Nodes: raw.Nodes}, nil
}
