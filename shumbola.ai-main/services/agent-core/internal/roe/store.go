package roe

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ErrNotFound is returned when an engagement has no RoE document on file.
var ErrNotFound = errors.New("roe: not found")

// Store persists and retrieves RoE documents, one per engagement (an
// engagement has at most one active RoE — a new upload replaces it,
// mirroring how a real RoE amendment works: a superseding signed
// document, not a merge).
type Store interface {
	Save(d *Document) error
	Load(engagementID string) (*Document, error)
	// Update loads the document, applies fn, and saves the result
	// atomically with respect to other Update/Save calls for the same
	// engagement — used by CounterApprove so two concurrent
	// counter-approval requests can't both "win".
	Update(engagementID string, fn func(d *Document) error) (*Document, error)
}

// FileStore persists each engagement's RoE as its own JSON file under
// dir, mirroring engagement.FileStore's pattern so it survives a
// restart.
type FileStore struct {
	mu  sync.Mutex
	dir string
}

// NewFileStore ensures dir exists and returns a FileStore backed by it.
func NewFileStore(dir string) (*FileStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("roe: create store dir: %w", err)
	}
	return &FileStore{dir: dir}, nil
}

func (s *FileStore) path(engagementID string) string {
	return filepath.Join(s.dir, engagementID+".json")
}

// Save writes d to disk, overwriting any existing document for its
// EngagementID.
func (s *FileStore) Save(d *Document) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked(d)
}

func (s *FileStore) saveLocked(d *Document) error {
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return fmt.Errorf("roe: marshal: %w", err)
	}
	if err := os.WriteFile(s.path(d.EngagementID), data, 0o600); err != nil {
		return fmt.Errorf("roe: write: %w", err)
	}
	return nil
}

// Load reads the RoE document for engagementID from disk.
func (s *FileStore) Load(engagementID string) (*Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked(engagementID)
}

func (s *FileStore) loadLocked(engagementID string) (*Document, error) {
	data, err := os.ReadFile(s.path(engagementID))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("roe: read: %w", err)
	}
	var d Document
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("roe: unmarshal: %w", err)
	}
	return &d, nil
}

// Update loads the RoE document, applies fn to it, and persists the
// result, holding the store lock for the duration.
func (s *FileStore) Update(engagementID string, fn func(d *Document) error) (*Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, err := s.loadLocked(engagementID)
	if err != nil {
		return nil, err
	}
	if err := fn(d); err != nil {
		return nil, err
	}
	if err := s.saveLocked(d); err != nil {
		return nil, err
	}
	return d, nil
}
