package engagement

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// ErrNotFound is returned when an engagement ID has no matching record.
var ErrNotFound = errors.New("engagement: not found")

// Store persists and retrieves engagements, and serializes concurrent
// mutation of a single engagement's status.
type Store interface {
	Save(e *Engagement) error
	Load(id string) (*Engagement, error)
	// Update loads the engagement, applies fn, and saves the result
	// atomically with respect to other Update/Save calls for the same id.
	Update(id string, fn func(e *Engagement) error) (*Engagement, error)
	// List returns every stored engagement, most recently created first.
	// Used by the web UI's dashboard (TZ 4.7).
	List() ([]*Engagement, error)
}

// FileStore persists each engagement as its own JSON file under dir, so
// engagements survive a restart (TZ 6.3: "Har bir engagement qayta
// tiklanadigan bo'lishi kerak").
type FileStore struct {
	mu  sync.Mutex
	dir string
}

// NewFileStore ensures dir exists and returns a FileStore backed by it.
func NewFileStore(dir string) (*FileStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("engagement: create store dir: %w", err)
	}
	return &FileStore{dir: dir}, nil
}

func (s *FileStore) path(id string) string {
	return filepath.Join(s.dir, id+".json")
}

// Save writes e to disk, overwriting any existing record.
func (s *FileStore) Save(e *Engagement) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked(e)
}

func (s *FileStore) saveLocked(e *Engagement) error {
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return fmt.Errorf("engagement: marshal: %w", err)
	}
	if err := os.WriteFile(s.path(e.ID), data, 0o600); err != nil {
		return fmt.Errorf("engagement: write: %w", err)
	}
	return nil
}

// Load reads the engagement with the given id from disk.
func (s *FileStore) Load(id string) (*Engagement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked(id)
}

func (s *FileStore) loadLocked(id string) (*Engagement, error) {
	data, err := os.ReadFile(s.path(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("engagement: read: %w", err)
	}
	var e Engagement
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, fmt.Errorf("engagement: unmarshal: %w", err)
	}
	return &e, nil
}

// List reads every engagement JSON file in the store directory, most
// recently created first. A file that fails to parse is skipped (logged
// via the returned error being nil for the rest — callers get a partial
// list rather than a hard failure) since one corrupt record shouldn't
// take down the whole dashboard.
func (s *FileStore) List() ([]*Engagement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("engagement: list store dir: %w", err)
	}

	var out []*Engagement
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		e, err := s.loadLocked(id)
		if err != nil {
			continue // skip unreadable/corrupt records rather than failing the whole list
		}
		out = append(out, e)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

// Update loads the engagement, applies fn to it, and persists the result,
// holding the store lock for the duration so concurrent updates to the
// same (or another) engagement don't race.
func (s *FileStore) Update(id string, fn func(e *Engagement) error) (*Engagement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, err := s.loadLocked(id)
	if err != nil {
		return nil, err
	}
	if err := fn(e); err != nil {
		return nil, err
	}
	if err := s.saveLocked(e); err != nil {
		return nil, err
	}
	return e, nil
}
