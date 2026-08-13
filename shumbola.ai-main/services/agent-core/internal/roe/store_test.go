package roe

import (
	"errors"
	"testing"
	"time"
)

func TestFileStoreSaveLoad(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	from := time.Now().UTC()
	d, err := New("roe-1", "eng-1", "admin-1", from, from.Add(time.Hour), nil, nil, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := store.Save(d); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load("eng-1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.ID != "roe-1" || loaded.ApprovedBy != "admin-1" {
		t.Fatalf("unexpected loaded document: %+v", loaded)
	}
}

func TestFileStoreLoadMissing(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	if _, err := store.Load("does-not-exist"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFileStoreUpdateAppliesCounterApprove(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	from := time.Now().UTC()
	d, err := New("roe-1", "eng-1", "admin-1", from, from.Add(time.Hour), nil, nil, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := store.Save(d); err != nil {
		t.Fatalf("Save: %v", err)
	}

	updated, err := store.Update("eng-1", func(doc *Document) error {
		return doc.CounterApprove("admin-2")
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !updated.TwoPersonSatisfied() {
		t.Fatalf("expected two-person rule satisfied after Update applied CounterApprove")
	}

	reloaded, err := store.Load("eng-1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reloaded.TwoPersonSatisfied() {
		t.Fatalf("expected the counter-approval to persist to disk")
	}
}
