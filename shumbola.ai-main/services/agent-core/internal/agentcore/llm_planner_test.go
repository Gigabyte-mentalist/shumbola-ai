package agentcore

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/shumbola/agent-core/internal/engagement"
	"github.com/shumbola/agent-core/internal/llm"
	"github.com/shumbola/agent-core/internal/rag"
	"github.com/shumbola/agent-core/internal/tasktree"
)

type fakeChatCompleter struct {
	response string
	err      error
	lastOpts llm.ChatOptions
	// lastMessages captures the messages of the most recent call, so
	// tests can inspect what was actually sent to the model (e.g. to
	// verify RAG context landed in the prompt).
	lastMessages []llm.Message

	// responses, if set, is consumed one entry per call (for testing
	// retry behavior); response/err above are used when it's empty.
	responses []string
	calls     int
}

func (f *fakeChatCompleter) ChatCompletion(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (string, error) {
	f.lastOpts = opts
	f.lastMessages = messages
	f.calls++
	if len(f.responses) > 0 {
		idx := f.calls - 1
		if idx >= len(f.responses) {
			idx = len(f.responses) - 1
		}
		return f.responses[idx], nil
	}
	if f.err != nil {
		return "", f.err
	}
	return f.response, nil
}

func testPlannerState(t *testing.T) PlannerState {
	t.Helper()
	eng, err := engagement.New("eng-1", "op-1", []string{"10.0.0.5"})
	if err != nil {
		t.Fatalf("engagement.New: %v", err)
	}
	return PlannerState{Engagement: eng, Tree: tasktree.New("eng-1")}
}

func TestLLMPlannerParsesValidJSON(t *testing.T) {
	fake := &fakeChatCompleter{response: `{"done":false,"tool":"nmap","target":"10.0.0.5","reason":"birinchi recon"}`}
	planner := NewLLMPlanner(fake, []ToolInfo{{Name: "nmap", Description: "port scanner"}}, nil)

	action, done, err := planner.Decide(context.Background(), testPlannerState(t))
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if done {
		t.Fatalf("expected done=false")
	}
	if action.Tool != "nmap" || action.Target != "10.0.0.5" || action.Reason != "birinchi recon" {
		t.Fatalf("unexpected action: %+v", action)
	}
	if fake.lastOpts.GuidedJSON == nil {
		t.Fatalf("expected GuidedJSON schema to be passed to ChatCompletion")
	}
}

func TestLLMPlannerExtractsJSONFromProse(t *testing.T) {
	fake := &fakeChatCompleter{response: "Albatta, mana natija:\n```json\n{\"done\":false,\"tool\":\"nuclei\",\"target\":\"10.0.0.5\",\"reason\":\"cve qidirish\"}\n```\nUmid qilamanki foydali."}
	planner := NewLLMPlanner(fake, nil, nil)

	action, done, err := planner.Decide(context.Background(), testPlannerState(t))
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if done {
		t.Fatalf("expected done=false")
	}
	if action.Tool != "nuclei" || action.Target != "10.0.0.5" {
		t.Fatalf("unexpected action extracted from prose-wrapped JSON: %+v", action)
	}
}

func TestLLMPlannerDone(t *testing.T) {
	fake := &fakeChatCompleter{response: `{"done":true}`}
	planner := NewLLMPlanner(fake, nil, nil)

	_, done, err := planner.Decide(context.Background(), testPlannerState(t))
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if !done {
		t.Fatalf("expected done=true")
	}
}

func TestLLMPlannerPropagatesClientError(t *testing.T) {
	fake := &fakeChatCompleter{err: errors.New("connection refused")}
	planner := NewLLMPlanner(fake, nil, nil)

	if _, _, err := planner.Decide(context.Background(), testPlannerState(t)); err == nil {
		t.Fatalf("expected error to propagate from ChatCompleter")
	}
}

func TestLLMPlannerRejectsMissingToolOrTarget(t *testing.T) {
	fake := &fakeChatCompleter{response: `{"done":false,"tool":"","target":""}`}
	planner := NewLLMPlanner(fake, nil, nil)

	if _, _, err := planner.Decide(context.Background(), testPlannerState(t)); err == nil {
		t.Fatalf("expected error when done=false but tool/target are empty")
	}
}

// TestLLMPlannerRetriesOnMalformedOutput reproduces a real observed
// failure: guided-JSON decoding on the reasoning model occasionally
// degenerates into a truncated/blank-token response instead of
// completing the schema. A retry with the same prompt succeeds — this
// is the TZ 4.2 "reja barbod bo'lsa qayta urinadi" recovery logic.
func TestLLMPlannerRetriesOnMalformedOutput(t *testing.T) {
	fake := &fakeChatCompleter{responses: []string{
		`{"done": false, "tool": "nmap"    ` + strings.Repeat("\n", 10) + `}`, // degenerate: target/reason missing
		`{"done":false,"tool":"nmap","target":"10.0.0.5","reason":"recon"}`,
	}}
	planner := NewLLMPlanner(fake, nil, nil)

	action, done, err := planner.Decide(context.Background(), testPlannerState(t))
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if done {
		t.Fatalf("expected done=false")
	}
	if action.Tool != "nmap" || action.Target != "10.0.0.5" {
		t.Fatalf("unexpected action after retry: %+v", action)
	}
	if fake.calls != 2 {
		t.Fatalf("expected exactly 2 calls (1 failed + 1 retry), got %d", fake.calls)
	}
}

func TestLLMPlannerGivesUpAfterMaxAttempts(t *testing.T) {
	fake := &fakeChatCompleter{response: "this is never valid JSON"}
	planner := NewLLMPlanner(fake, nil, nil)

	if _, _, err := planner.Decide(context.Background(), testPlannerState(t)); err == nil {
		t.Fatalf("expected error after exhausting retries")
	}
	if fake.calls != maxDecideAttempts {
		t.Fatalf("expected exactly %d attempts, got %d", maxDecideAttempts, fake.calls)
	}
}

func TestLLMPlannerRejectsUnparsableResponse(t *testing.T) {
	fake := &fakeChatCompleter{response: "bu umuman JSON emas va jufт qavs ham yo'q"}
	planner := NewLLMPlanner(fake, nil, nil)

	if _, _, err := planner.Decide(context.Background(), testPlannerState(t)); err == nil {
		t.Fatalf("expected error for unparsable response")
	}
}

func TestLLMPlannerSystemPromptListsTools(t *testing.T) {
	planner := NewLLMPlanner(&fakeChatCompleter{}, []ToolInfo{
		{Name: "nmap", Description: "port scanner"},
		{Name: "nuclei", Description: "vuln scanner"},
	}, nil)
	prompt := planner.systemPrompt()
	if !containsAll(prompt, "nmap", "port scanner", "nuclei", "vuln scanner") {
		t.Fatalf("expected system prompt to mention all tools, got: %s", prompt)
	}
}

func TestLLMPlannerUserPromptIncludesScopeAndHistory(t *testing.T) {
	planner := NewLLMPlanner(&fakeChatCompleter{}, nil, nil)
	state := testPlannerState(t)
	state.Tree.AddNode("", tasktree.KindFinding, "22/tcp open", map[string]any{"target": "10.0.0.5", "tool": "nmap"})

	prompt := planner.userPrompt(state, "")
	if !containsAll(prompt, "10.0.0.5", "22/tcp open", "tool=nmap") {
		t.Fatalf("expected user prompt to include scope and history, got: %s", prompt)
	}
}

func TestLLMPlannerUserPromptIncludesRAGContext(t *testing.T) {
	planner := NewLLMPlanner(&fakeChatCompleter{}, nil, nil)
	state := testPlannerState(t)

	prompt := planner.userPrompt(state, "- [hacktricks: SSH] some pentest technique\n")
	if !containsAll(prompt, "Bilim bazasidan tegishli ma'lumot", "hacktricks: SSH", "some pentest technique") {
		t.Fatalf("expected user prompt to include RAG context section, got: %s", prompt)
	}
}

type fakeRAGRetriever struct {
	chunks []rag.Chunk
	err    error
}

func (f *fakeRAGRetriever) Retrieve(ctx context.Context, query string, k int) ([]rag.Chunk, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.chunks, nil
}

func TestLLMPlannerRAGGroundsDecision(t *testing.T) {
	fake := &fakeChatCompleter{response: `{"done":false,"tool":"nmap","target":"10.0.0.5","reason":"recon"}`}
	ragFake := &fakeRAGRetriever{chunks: []rag.Chunk{
		{Text: "SSH pentest checklist details", Source: "hacktricks", Heading: "SSH"},
	}}
	planner := NewLLMPlanner(fake, nil, ragFake)

	if _, _, err := planner.Decide(context.Background(), testPlannerState(t)); err != nil {
		t.Fatalf("Decide: %v", err)
	}

	sentPrompt := fake.lastMessages[len(fake.lastMessages)-1].Content
	if !containsAll(sentPrompt, "SSH pentest checklist details", "hacktricks") {
		t.Fatalf("expected RAG chunk to be included in the prompt sent to the model, got: %s", sentPrompt)
	}
}

func TestLLMPlannerDecideSucceedsWhenRAGFails(t *testing.T) {
	fake := &fakeChatCompleter{response: `{"done":false,"tool":"nmap","target":"10.0.0.5","reason":"recon"}`}
	ragFake := &fakeRAGRetriever{err: errors.New("rag service unreachable")}
	planner := NewLLMPlanner(fake, nil, ragFake)

	action, _, err := planner.Decide(context.Background(), testPlannerState(t))
	if err != nil {
		t.Fatalf("expected Decide to succeed despite RAG failure, got: %v", err)
	}
	if action.Tool != "nmap" {
		t.Fatalf("unexpected action: %+v", action)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
