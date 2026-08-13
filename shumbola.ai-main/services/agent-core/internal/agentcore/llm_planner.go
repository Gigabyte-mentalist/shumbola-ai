package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/shumbola/agent-core/internal/llm"
	"github.com/shumbola/agent-core/internal/rag"
	"github.com/shumbola/agent-core/internal/tasktree"
)

// ChatCompleter is the subset of *llm.Client's behavior LLMPlanner needs.
// Depending on this narrow interface (rather than *llm.Client directly)
// lets tests substitute a fake instead of a real vLLM server. Unlike
// tools.ContainerRunner (which duplicates sandbox's types to keep
// tools/sandbox decoupled in both directions), there's no cycle risk
// here — agentcore importing llm's types directly is fine.
type ChatCompleter interface {
	ChatCompletion(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (string, error)
}

// RAGRetriever is the subset of *rag.Client's behavior LLMPlanner needs
// (TZ 8-bo'lim: pentest bilim bazasi). Optional — a nil RAGRetriever
// means LLMPlanner reasons from the prompt alone, same as before RAG was
// wired in (graceful degradation, same pattern as ChatCompleter's server
// dependencies elsewhere in this codebase).
type RAGRetriever interface {
	Retrieve(ctx context.Context, query string, k int) ([]rag.Chunk, error)
}

// ragTopK is how many knowledge base chunks LLMPlanner asks for per
// decision — enough to give real grounding without bloating the prompt.
const ragTopK = 5

// ToolInfo describes one available tool for the LLM's system prompt.
// LLMPlanner never touches the actual tools.Tool implementations — it
// only proposes an Action{Tool, Target}; the Engine remains the sole
// place that validates scope and executes tools (TZ 4.3).
type ToolInfo struct {
	Name        string
	Description string
}

// llmActionResponse is the strict JSON shape the model is asked to
// produce (enforced server-side via ChatOptions.GuidedJSON when the
// backend supports it, and defensively re-parsed client-side either way).
type llmActionResponse struct {
	Done   bool   `json:"done"`
	Tool   string `json:"tool"`
	Target string `json:"target"`
	Reason string `json:"reason"`
}

// LLMPlanner is a Planner backed by a chat-completion model (TZ 4.2,
// Bosqich 4): it prompts the model with the engagement scope and Task
// Tree history and asks it to choose the next (tool, target) pair, or
// declare the engagement done. It replaces StaticPlanner's fixed
// tool-per-target sequence with model-driven judgment about what to try
// next and in what order — everything downstream (scope enforcement,
// HITL gating, Task Tree recording) is unchanged.
type LLMPlanner struct {
	client ChatCompleter
	tools  []ToolInfo
	rag    RAGRetriever
}

// NewLLMPlanner builds an LLMPlanner that chooses among the given tools.
// ragClient may be nil if no RAG service is configured.
func NewLLMPlanner(client ChatCompleter, tools []ToolInfo, ragClient RAGRetriever) *LLMPlanner {
	return &LLMPlanner{client: client, tools: tools, rag: ragClient}
}

// llmActionSchema forces guided decoding to emit all four keys, not just
// "done". A schema requiring only "done" was observed (manual testing
// against the real server) to make the guided decoder consistently emit
// `{"done": false, "tool": "nmap"}` with tool/target/reason omitted —
// technically schema-valid, since JSON Schema's "required" only says
// which keys must be present, not which are populated when other logic
// (like "tool must be set when done is false") depends on a field's
// value. Requiring every key removes that degenerate-but-valid escape
// hatch; when the model means done=true it must still emit tool/target/
// reason (LLMPlanner ignores them in that case).
var llmActionSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"done":   map[string]any{"type": "boolean"},
		"tool":   map[string]any{"type": "string"},
		"target": map[string]any{"type": "string"},
		"reason": map[string]any{"type": "string"},
	},
	"required": []string{"done", "tool", "target", "reason"},
}

// maxDecideAttempts bounds retries against malformed model output (TZ
// 4.2's "reja barbod bo'lsa... agent muqobil yo'lni sinaydi" — recovery
// logic). Guided-JSON decoding on reasoning models has been observed
// (manual testing against the real server) to occasionally degenerate
// into a run of blank/whitespace tokens instead of completing the
// schema, unrelated to prompt content — simply asking again succeeds.
const maxDecideAttempts = 3

func (p *LLMPlanner) Decide(ctx context.Context, state PlannerState) (Action, bool, error) {
	messages := []llm.Message{
		{Role: "system", Content: p.systemPrompt()},
		{Role: "user", Content: p.userPrompt(state, p.retrieveContext(ctx, state))},
	}

	var lastErr error
	for attempt := 1; attempt <= maxDecideAttempts; attempt++ {
		action, done, ok, err := p.decideOnce(ctx, messages)
		if ok {
			return action, done, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			break // kill switch / caller cancellation — don't keep retrying
		}
	}
	return Action{}, false, fmt.Errorf("llm planner: giving up after %d attempts: %w", maxDecideAttempts, lastErr)
}

// decideOnce makes a single model call and attempt to parse it. ok=false
// means the caller should retry (with err describing why this attempt
// failed); ok=true means action/done are the final result.
func (p *LLMPlanner) decideOnce(ctx context.Context, messages []llm.Message) (action Action, done bool, ok bool, err error) {
	content, err := p.client.ChatCompletion(ctx, messages, llm.ChatOptions{
		// Generous budget: reasoning models (this one included, TZ 7.2 —
		// Qwen3 "thinking mode") can spend several hundred tokens on
		// their reasoning_content trace before ever emitting the final
		// JSON content; a tight budget truncates mid-thought and leaves
		// content empty (observed in manual testing against the real
		// server with 1024).
		MaxTokens:   4096,
		Temperature: 0.2,
		GuidedJSON:  llmActionSchema,
	})
	if err != nil {
		return Action{}, false, false, fmt.Errorf("chat completion: %w", err)
	}

	parsed, err := parseLLMActionResponse(content)
	if err != nil {
		return Action{}, false, false, fmt.Errorf("parse response: %w (raw: %q)", err, content)
	}

	if parsed.Done {
		return Action{}, true, true, nil
	}
	if parsed.Tool == "" || parsed.Target == "" {
		return Action{}, false, false, fmt.Errorf("model returned done=false but left tool/target empty (raw: %q)", content)
	}
	return Action{Tool: parsed.Tool, Target: parsed.Target, Reason: parsed.Reason}, false, true, nil
}

// parseLLMActionResponse parses content as an llmActionResponse. If the
// content isn't valid JSON on its own (a model occasionally wraps JSON in
// prose or markdown fences despite instructions), it falls back to
// extracting the first top-level {...} block and parsing that.
func parseLLMActionResponse(content string) (llmActionResponse, error) {
	var resp llmActionResponse
	if err := json.Unmarshal([]byte(content), &resp); err == nil {
		return resp, nil
	}

	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start == -1 || end == -1 || end < start {
		return llmActionResponse{}, fmt.Errorf("no JSON object found in response")
	}
	if err := json.Unmarshal([]byte(content[start:end+1]), &resp); err != nil {
		return llmActionResponse{}, fmt.Errorf("extracted substring is not valid JSON: %w", err)
	}
	return resp, nil
}

func (p *LLMPlanner) systemPrompt() string {
	var b strings.Builder
	b.WriteString("Siz shumbola.ai pentest agentining orkestratori (miya)siz. ")
	b.WriteString("Sizning vazifangiz — keyingi eng mantiqiy tekshiruv qadamini tanlash. ")
	b.WriteString("Siz hech qachon tool'ni o'zingiz ishga tushirmaysiz va exploit yozmaysiz — faqat qaysi tool, qaysi nishonga qarshi ishlatilishini tanlaysiz; ")
	b.WriteString("haqiqiy ijro, scope tekshiruvi va inson tasdig'i (HITL) alohida tizim tomonidan amalga oshiriladi. ")
	b.WriteString("Mavjud tool'lar:\n")
	for _, t := range p.tools {
		fmt.Fprintf(&b, "- %s: %s\n", t.Name, t.Description)
	}
	b.WriteString("Faqat berilgan scope ichidagi nishonlarni tanlang. Har bir (nishon, tool) juftligini faqat bir marta tavsiya qiling — tarixda allaqachon sinalganlarini qaytarmang. ")
	b.WriteString("Barcha kerakli tekshiruvlar tugagan deb hisoblasangiz, done=true qaytaring. ")
	b.WriteString("Javobingiz FAQAT quyidagi JSON formatida bo'lsin, boshqa hech qanday matn qo'shmang: ")
	b.WriteString(`{"done": bool, "tool": "string", "target": "string", "reason": "qisqa asoslama"}`)
	return b.String()
}

func (p *LLMPlanner) userPrompt(state PlannerState, ragContext string) string {
	var b strings.Builder
	b.WriteString("Scope (ruxsat berilgan nishonlar):\n")
	for _, t := range state.Engagement.Scope.Targets {
		fmt.Fprintf(&b, "- %s\n", t)
	}

	b.WriteString("\nHozirgacha bajarilgan qadamlar va topilmalar:\n")
	nodes := state.Tree.Snapshot()
	if len(nodes) == 0 {
		b.WriteString("(hali hech narsa qilinmagan)\n")
	}
	for _, n := range nodes {
		fmt.Fprintf(&b, "- [%s] %s", n.Kind, n.Summary)
		if target, tool := detailPair(n); target != "" || tool != "" {
			fmt.Fprintf(&b, " (target=%s, tool=%s)", target, tool)
		}
		b.WriteString("\n")
	}

	if ragContext != "" {
		b.WriteString("\nBilim bazasidan tegishli ma'lumot (qaror qabul qilishda hisobga oling):\n")
		b.WriteString(ragContext)
	}

	b.WriteString("\nKeyingi qadamni tanlang.")
	return b.String()
}

// retrieveContext queries the RAG knowledge base (if configured) for
// passages relevant to the current state and formats them for the
// prompt. A retrieval failure or missing RAGRetriever degrades to an
// empty string — RAG is an enhancement, not a hard dependency, so it
// never blocks a planning decision (mirrors the rest of this codebase's
// "missing optional dependency" handling, e.g. sandbox/LLM_BASE_URL).
func (p *LLMPlanner) retrieveContext(ctx context.Context, state PlannerState) string {
	if p.rag == nil {
		return ""
	}

	chunks, err := p.rag.Retrieve(ctx, ragQueryText(state), ragTopK)
	if err != nil {
		log.Printf("llm planner: rag retrieval failed, continuing without it: %v", err)
		return ""
	}
	if len(chunks) == 0 {
		return ""
	}

	var b strings.Builder
	for _, c := range chunks {
		fmt.Fprintf(&b, "- [%s: %s] %s\n", c.Source, c.Heading, truncate(c.Text, 600))
	}
	return b.String()
}

// ragQueryText builds the knowledge-base search query from the current
// state: the most recent finding is the most specific, actionable signal
// available (e.g. an nmap result naming an open service) and tends to
// surface the most relevant technique/checklist chunks. With no findings
// yet, it falls back to a generic recon query so the very first decision
// still gets some grounding.
func ragQueryText(state PlannerState) string {
	nodes := state.Tree.Snapshot()
	for i := len(nodes) - 1; i >= 0; i-- {
		if nodes[i].Kind == tasktree.KindFinding {
			return nodes[i].Summary
		}
	}
	return "web application and network penetration testing methodology initial reconnaissance"
}

// truncate keeps prompt size bounded — knowledge base chunks can run to
// ~800 tokens each (see the RAG ingestion pipeline's chunking target);
// a short excerpt is enough context without ballooning the prompt across
// ragTopK chunks.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func detailPair(n tasktree.Node) (target, tool string) {
	if n.Details == nil {
		return "", ""
	}
	if v, ok := n.Details["target"].(string); ok {
		target = v
	}
	if v, ok := n.Details["tool"].(string); ok {
		tool = v
	}
	return target, tool
}
