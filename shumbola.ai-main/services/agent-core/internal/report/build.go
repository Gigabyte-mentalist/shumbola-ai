package report

import (
	"sort"
	"time"

	"github.com/shumbola/agent-core/internal/engagement"
	"github.com/shumbola/agent-core/internal/standards"
	"github.com/shumbola/agent-core/internal/tasktree"
	"github.com/shumbola/agent-core/internal/tools"
	"github.com/shumbola/agent-core/internal/validation"
)

var severityRank = map[tools.Severity]int{
	tools.SeverityCritical: 4,
	tools.SeverityHigh:     3,
	tools.SeverityMed:      2,
	tools.SeverityLow:      1,
	tools.SeverityInfo:     0,
}

// Build turns eng's engagement info and tree's current Task Tree state
// into ReportData. Only finding nodes whose validation verdict was
// Proven are included — TZ 4.5's "isbotlanmagan bo'lsa — reportga
// chiqmaydi" is enforced here, not left to the renderer.
func Build(eng *engagement.Engagement, tree *tasktree.Tree, lang string) ReportData {
	if lang == "" {
		lang = LangUzLatn
	}

	data := ReportData{
		EngagementID: eng.ID,
		Scope:        append([]string{}, eng.Scope.Targets...),
		Lang:         lang,
		GeneratedAt:  time.Now().UTC(),
	}

	for _, n := range tree.Snapshot() {
		if n.Kind != tasktree.KindFinding || n.Details == nil {
			continue
		}
		proven, _ := n.Details["proven"].(bool)
		if !proven {
			continue
		}

		toolName := detailString(n.Details, "tool")
		target := detailString(n.Details, "target")
		detail := detailString(n.Details, "detail")
		confidence := validation.Confidence(detailString(n.Details, "confidence"))
		sev := tools.Severity(detailString(n.Details, "severity"))

		f := ReportFinding{
			Title:       n.Summary,
			Severity:    sev,
			CVSS:        standards.CVSSScore(sev),
			OWASP:       standards.OWASPCategory(toolName, n.Summary),
			MITRE:       standards.MITRETechnique(toolName, n.Summary),
			Target:      target,
			Tool:        toolName,
			Detail:      detail,
			Remediation: standards.Remediation(toolName),
			Confidence:  confidence,
		}
		data.Findings = append(data.Findings, f)

		switch sev {
		case tools.SeverityCritical:
			data.Summary.Critical++
		case tools.SeverityHigh:
			data.Summary.High++
		case tools.SeverityMed:
			data.Summary.Medium++
		case tools.SeverityLow:
			data.Summary.Low++
		default:
			data.Summary.Info++
		}
	}

	sort.SliceStable(data.Findings, func(i, j int) bool {
		return severityRank[data.Findings[i].Severity] > severityRank[data.Findings[j].Severity]
	})

	return data
}

// detailString reads a string-ish value out of a task tree node's
// Details map. Details is populated directly (as tools.Severity,
// validation.Confidence, etc.) when a node is added in-process, but
// becomes plain map[string]any with plain string values once it's been
// round-tripped through tasktree.FileStore's JSON persistence — this
// handles both cases so Build works the same whether it's reading a
// freshly-run tree or one reloaded from disk.
func detailString(details map[string]any, key string) string {
	switch v := details[key].(type) {
	case string:
		return v
	case tools.Severity:
		return string(v)
	case validation.Confidence:
		return string(v)
	default:
		return ""
	}
}
