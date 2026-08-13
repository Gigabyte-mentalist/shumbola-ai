// Package validation implements TZ 4.5's proof-of-exploit gate: every
// finding is checked against a proven/confidence criterion before it can
// be treated as a confirmed result — "isbotlanmagan bo'lsa — reportga
// chiqmaydi".
//
// Bosqich 2 ships a rule-based heuristic Validator, since no LLM is
// connected yet (see the shumbola-llm-server-deployment project notes).
// The Evaluate method is the seam a future LLM-backed validator
// (Bosqich 4) replaces without changing how the Engine or Task Tree use
// its result.
package validation

import "github.com/shumbola/agent-core/internal/tools"

// Confidence is how much weight a finding's Verdict should carry.
type Confidence string

const (
	ConfidenceLow    Confidence = "low"
	ConfidenceMedium Confidence = "medium"
	ConfidenceHigh   Confidence = "high"
)

// Verdict is the outcome of validating one Finding.
type Verdict struct {
	Proven     bool       `json:"proven"`
	Confidence Confidence `json:"confidence"`
	Reason     string     `json:"reason"`
}

// Validator evaluates tool findings for proof-of-exploit and assigns a
// confidence level, so low-confidence/unproven findings can be routed to
// operator review (TZ 4.5) instead of being treated as confirmed.
type Validator struct{}

// NewValidator builds the Bosqich 2 heuristic Validator.
func NewValidator() *Validator {
	return &Validator{}
}

// Evaluate applies the heuristic: a finding with no captured evidence
// (Detail) cannot be considered proven at all, regardless of severity —
// a tool claiming something without any observable evidence is exactly
// the false-positive shape TZ 4.5 exists to catch. A finding that does
// carry evidence is proven, with confidence scaled by severity — higher
// severity findings get more scrutiny value (and, later, more report
// prominence) once verified.
func (v *Validator) Evaluate(f tools.Finding) Verdict {
	if f.Detail == "" {
		return Verdict{
			Proven:     false,
			Confidence: ConfidenceLow,
			Reason:     "dalil (evidence) topilmadi — tool xulosasi tasdiqlanmagan",
		}
	}

	switch f.Severity {
	case tools.SeverityHigh, tools.SeverityCritical:
		return Verdict{Proven: true, Confidence: ConfidenceHigh, Reason: "dalil mavjud, yuqori jiddiylik"}
	case tools.SeverityLow, tools.SeverityMed:
		return Verdict{Proven: true, Confidence: ConfidenceMedium, Reason: "dalil mavjud, o'rta jiddiylik"}
	default: // info or unset
		return Verdict{Proven: true, Confidence: ConfidenceLow, Reason: "dalil mavjud, ma'lumot xarakteridagi topilma"}
	}
}
