// Package llm is a minimal OpenAI-compatible chat completion client, used
// to talk to the vLLM server (TZ 7.2: "vLLM OpenAI-mos API beradi").
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Message is one OpenAI-style chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatOptions controls a single ChatCompletion call.
type ChatOptions struct {
	MaxTokens   int
	Temperature float64
	// GuidedJSON, if set, is passed as vLLM's `guided_json` extension
	// field so the server constrains its output to this JSON schema.
	// Callers still get back a plain string — Go structs live in the
	// caller so this package stays free of any one caller's schema.
	GuidedJSON map[string]any
}

// Client is a small OpenAI-compatible chat completions client.
type Client struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewClient builds a Client against baseURL (e.g.
// "http://192.168.10.201:18000/v1") for the given model name. baseURL
// must not have a trailing slash requirement — it is joined with
// "/chat/completions" directly.
func NewClient(baseURL, model string) *Client {
	return &Client{
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			// Reasoning models can take a while to finish thinking
			// before the final answer. At a several-thousand-token
			// budget (LLMPlanner asks for up to 4096), worst-case
			// generation time can genuinely exceed two minutes — a
			// 120s timeout was observed to fail closed on real
			// requests during manual testing against the live server,
			// even though the request was still actively generating.
			Timeout: 5 * time.Minute,
		},
	}
}

type chatRequest struct {
	Model       string         `json:"model"`
	Messages    []Message      `json:"messages"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
	Temperature float64        `json:"temperature,omitempty"`
	GuidedJSON  map[string]any `json:"guided_json,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
			// ReasoningContent holds a reasoning-parser-enabled server's
			// "thinking" trace, normally distinct from Content. With
			// vLLM's deepseek_r1 parser + guided_json combined (as used
			// by LLMPlanner), the parser has been observed to place the
			// entire schema-constrained answer here and leave Content
			// null — guided decoding skips the <think> tag flow the
			// parser expects, so it never splits the two. See
			// ChatCompletion's fallback below.
			ReasoningContent string `json:"reasoning_content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// ChatCompletion sends messages to the server and returns the first
// choice's final answer content (not the reasoning trace, for
// reasoning-parser-enabled servers — see TZ note on Qwen3 thinking mode).
func (c *Client) ChatCompletion(ctx context.Context, messages []Message, opts ChatOptions) (string, error) {
	reqBody, err := json.Marshal(chatRequest{
		Model:       c.model,
		Messages:    messages,
		MaxTokens:   opts.MaxTokens,
		Temperature: opts.Temperature,
		GuidedJSON:  opts.GuidedJSON,
	})
	if err != nil {
		return "", fmt.Errorf("llm: marshal request: %w", err)
	}

	url := c.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("llm: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("llm: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("llm: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm: server returned status %d: %s", resp.StatusCode, string(body))
	}

	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("llm: unmarshal response: %w", err)
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("llm: server error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("llm: response has no choices")
	}
	msg := parsed.Choices[0].Message
	if msg.Content != "" {
		return msg.Content, nil
	}
	if opts.GuidedJSON != nil && msg.ReasoningContent != "" {
		// See the ReasoningContent field comment: a guided_json request
		// against a reasoning-parser server can land the whole answer
		// here instead of Content. Using it is safe specifically because
		// GuidedJSON already constrained the model to emit nothing but
		// the requested schema — there's no free-form "thinking" prose
		// mixed in to worry about misinterpreting as the answer. Without
		// GuidedJSON, a non-empty ReasoningContent alongside an empty
		// Content more likely means genuine mid-thought truncation, so
		// this fallback does NOT apply — that falls through to the error
		// below instead of silently returning a truncated fragment.
		return msg.ReasoningContent, nil
	}
	return "", fmt.Errorf("llm: response content is empty (model may have run out of tokens mid-reasoning — consider raising MaxTokens)")
}
