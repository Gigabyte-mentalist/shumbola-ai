// Package roe models a signed Rules of Engagement document — the
// authorization record shumbola_offensive_TZ.md's FR-AUTHZ-1 requires on
// file before an engagement's exploitation mode means anything. This
// package only models and validates the document itself; the actual
// gate that combines it with an Engagement and a Principal to decide
// whether one specific action is authorized lives in internal/exploitgate
// (kept separate so each has one job and neither needs to import the
// other's HTTP-layer concerns).
package roe

import (
	"errors"
	"fmt"
	"time"
)

// ErrInvalidWindow is returned by New when validUntil doesn't fall after
// validFrom.
var ErrInvalidWindow = errors.New("roe: valid_until must be after valid_from")

// ErrAlreadyCounterApproved is returned by CounterApprove when the
// document already has a second signature.
var ErrAlreadyCounterApproved = errors.New("roe: already counter-approved")

// ErrSameApprover is returned by CounterApprove when by matches the
// document's original ApprovedBy — the two-person rule (FR-AUTHZ-5)
// requires an independent second signer, not the same principal signing
// twice.
var ErrSameApprover = errors.New("roe: counter-approval must be from a different principal than the original approval")

// Document is a signed Rules of Engagement record for one engagement.
// An engagement's exploitation mode is never authorized by Document
// alone — see exploitgate.Authorize, which treats a missing, expired, or
// not-yet-two-person-approved Document identically to "deny" (fail
// closed, FR-SAFE-9).
type Document struct {
	ID           string `json:"id"`
	EngagementID string `json:"engagement_id"`

	ValidFrom  time.Time `json:"valid_from"`
	ValidUntil time.Time `json:"valid_until"`

	ProhibitedActions []string `json:"prohibited_actions,omitempty"`
	Contacts          []string `json:"contacts,omitempty"`

	// AllowedDestructiveActions explicitly names the (tool/action)
	// identifiers a destructive step is permitted for (FR-SAFE-4).
	// Nil/empty — the default for every RoE — means nothing destructive
	// is permitted at all; there is no separate "off switch" for this,
	// only an explicit allowlist entry per action.
	AllowedDestructiveActions []string `json:"allowed_destructive_actions,omitempty"`

	// ApprovedBy/ApprovedAt is the document's first signature — the
	// principal who uploaded it. The HTTP layer, not this package,
	// enforces that only an Administrator can do this.
	ApprovedBy string    `json:"approved_by"`
	ApprovedAt time.Time `json:"approved_at"`

	// CounterApprovedBy/At is the second, independent signature the
	// two-person rule requires (FR-AUTHZ-5) before exploit mode is
	// usable. Empty until CounterApprove succeeds.
	CounterApprovedBy string     `json:"counter_approved_by,omitempty"`
	CounterApprovedAt *time.Time `json:"counter_approved_at,omitempty"`
}

// New builds a Document. It performs no I/O; use a Store to persist it.
// allowedDestructive may be nil — the default, permitting nothing
// destructive (FR-SAFE-4).
func New(id, engagementID, approvedBy string, validFrom, validUntil time.Time, prohibitedActions, contacts, allowedDestructive []string) (*Document, error) {
	if id == "" {
		return nil, fmt.Errorf("roe: id is required")
	}
	if engagementID == "" {
		return nil, fmt.Errorf("roe: engagement id is required")
	}
	if approvedBy == "" {
		return nil, fmt.Errorf("roe: approved_by is required")
	}
	if !validUntil.After(validFrom) {
		return nil, ErrInvalidWindow
	}
	return &Document{
		ID:                        id,
		EngagementID:              engagementID,
		ValidFrom:                 validFrom,
		ValidUntil:                validUntil,
		ProhibitedActions:         prohibitedActions,
		Contacts:                  contacts,
		AllowedDestructiveActions: allowedDestructive,
		ApprovedBy:                approvedBy,
		ApprovedAt:                time.Now().UTC(),
	}, nil
}

// ValidNow reports whether now falls within the document's authorized
// window (FR-AUTHZ-3). Callers pass time.Now().UTC() in production and a
// fixed instant in tests.
func (d *Document) ValidNow(now time.Time) bool {
	return !now.Before(d.ValidFrom) && !now.After(d.ValidUntil)
}

// TwoPersonSatisfied reports whether a second, distinct principal has
// countersigned (FR-AUTHZ-5). A document approved and "counter-approved"
// by the same principal does not satisfy this — the whole point is
// independent confirmation.
func (d *Document) TwoPersonSatisfied() bool {
	return d.CounterApprovedBy != "" && d.CounterApprovedBy != d.ApprovedBy
}

// AllowsDestructive reports whether action is explicitly permitted as a
// destructive step by this RoE (FR-SAFE-4). A nil Document — no RoE on
// file — never allows anything, matching every other fail-closed check
// in this system.
func (d *Document) AllowsDestructive(action string) bool {
	if d == nil {
		return false
	}
	for _, a := range d.AllowedDestructiveActions {
		if a == action {
			return true
		}
	}
	return false
}

// CounterApprove records by as the document's second signature. It
// fails if the document is already counter-approved, or if by is the
// same principal who provided the first signature (FR-AUTHZ-5).
func (d *Document) CounterApprove(by string) error {
	if by == "" {
		return fmt.Errorf("roe: counter-approver is required")
	}
	if d.CounterApprovedBy != "" {
		return ErrAlreadyCounterApproved
	}
	if by == d.ApprovedBy {
		return ErrSameApprover
	}
	now := time.Now().UTC()
	d.CounterApprovedBy = by
	d.CounterApprovedAt = &now
	return nil
}
