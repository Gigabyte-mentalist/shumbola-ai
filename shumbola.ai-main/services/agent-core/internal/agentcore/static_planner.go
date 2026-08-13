package agentcore

import "context"

// defaultToolSequence is the fixed order StaticPlanner runs tools in
// against every target: broad recon first (nmap), then vulnerability
// scanning (nuclei), content discovery (ffuf), and TLS inspection
// (testssl) last.
var defaultToolSequence = []string{"nmap", "smbenum", "nuclei", "ffuf", "testssl"}

// StaticPlanner is a deterministic, model-free Planner: it walks the
// engagement's scope in order and, for each target, proposes the next
// tool in ToolNames that doesn't have a result in the task tree yet. It
// exists so Bosqich 1/2 can prove out the Engine/HITL/Task Tree/audit/
// validation machinery without depending on a live LLM.
type StaticPlanner struct {
	// ToolNames is the fixed sequence of tools StaticPlanner runs against
	// every target, in order. Defaults to defaultToolSequence if empty.
	ToolNames []string
}

func (p *StaticPlanner) Decide(ctx context.Context, state PlannerState) (Action, bool, error) {
	toolNames := p.ToolNames
	if len(toolNames) == 0 {
		toolNames = defaultToolSequence
	}

	for _, target := range state.Engagement.Scope.Targets {
		for _, toolName := range toolNames {
			if state.Tree.HasToolResult(target, toolName) {
				continue
			}
			return Action{Tool: toolName, Target: target}, false, nil
		}
	}
	return Action{}, true, nil
}
