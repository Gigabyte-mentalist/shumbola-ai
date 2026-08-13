package roe

import (
	"errors"
	"testing"
	"time"
)

func mustNew(t *testing.T, from, until time.Time) *Document {
	t.Helper()
	d, err := New("roe-1", "eng-1", "admin-1", from, until, []string{"no data deletion"}, []string{"security@client.example"}, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return d
}

func TestNewRejectsInvertedWindow(t *testing.T) {
	now := time.Now().UTC()
	_, err := New("roe-1", "eng-1", "admin-1", now, now.Add(-time.Hour), nil, nil, nil)
	if !errors.Is(err, ErrInvalidWindow) {
		t.Fatalf("expected ErrInvalidWindow, got %v", err)
	}
}

func TestNewRejectsMissingFields(t *testing.T) {
	now := time.Now().UTC()
	later := now.Add(time.Hour)
	if _, err := New("", "eng-1", "admin-1", now, later, nil, nil, nil); err == nil {
		t.Fatalf("expected error for empty id")
	}
	if _, err := New("roe-1", "", "admin-1", now, later, nil, nil, nil); err == nil {
		t.Fatalf("expected error for empty engagement id")
	}
	if _, err := New("roe-1", "eng-1", "", now, later, nil, nil, nil); err == nil {
		t.Fatalf("expected error for empty approved_by")
	}
}

func TestValidNow(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 1, 7, 0, 0, 0, 0, time.UTC)
	d := mustNew(t, from, until)

	cases := []struct {
		name string
		now  time.Time
		want bool
	}{
		{"before window", from.Add(-time.Second), false},
		{"at start", from, true},
		{"inside", from.Add(24 * time.Hour), true},
		{"at end", until, true},
		{"after window", until.Add(time.Second), false},
	}
	for _, tc := range cases {
		if got := d.ValidNow(tc.now); got != tc.want {
			t.Errorf("%s: ValidNow() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestTwoPersonSatisfiedRequiresDistinctCounterApprover(t *testing.T) {
	from := time.Now().UTC()
	d := mustNew(t, from, from.Add(time.Hour))

	if d.TwoPersonSatisfied() {
		t.Fatalf("expected unsatisfied before any counter-approval")
	}

	if err := d.CounterApprove(d.ApprovedBy); !errors.Is(err, ErrSameApprover) {
		t.Fatalf("expected ErrSameApprover when counter-approver matches the original approver, got %v", err)
	}
	if d.TwoPersonSatisfied() {
		t.Fatalf("a rejected same-approver attempt must not satisfy the two-person rule")
	}

	if err := d.CounterApprove("admin-2"); err != nil {
		t.Fatalf("CounterApprove: %v", err)
	}
	if !d.TwoPersonSatisfied() {
		t.Fatalf("expected satisfied after a distinct principal counter-approved")
	}

	if err := d.CounterApprove("admin-3"); !errors.Is(err, ErrAlreadyCounterApproved) {
		t.Fatalf("expected ErrAlreadyCounterApproved on a second attempt, got %v", err)
	}
}
