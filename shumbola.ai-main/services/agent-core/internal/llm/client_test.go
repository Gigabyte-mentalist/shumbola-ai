package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChatCompletionSendsCorrectRequest(t *testing.T) {
	var gotReq chatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": `{"done":false}`}}},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "Qwen/Qwen3-32B-FP8")
	content, err := client.ChatCompletion(context.Background(), []Message{
		{Role: "system", Content: "you are a planner"},
		{Role: "user", Content: "what next?"},
	}, ChatOptions{MaxTokens: 512, Temperature: 0.2})
	if err != nil {
		t.Fatalf("ChatCompletion: %v", err)
	}
	if content != `{"done":false}` {
		t.Fatalf("unexpected content: %s", content)
	}

	if gotReq.Model != "Qwen/Qwen3-32B-FP8" {
		t.Fatalf("unexpected model in request: %s", gotReq.Model)
	}
	if len(gotReq.Messages) != 2 || gotReq.Messages[1].Content != "what next?" {
		t.Fatalf("unexpected messages in request: %+v", gotReq.Messages)
	}
	if gotReq.MaxTokens != 512 {
		t.Fatalf("expected max_tokens 512, got %d", gotReq.MaxTokens)
	}
}

func TestChatCompletionPassesGuidedJSON(t *testing.T) {
	var gotReq chatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotReq)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": "ok"}}},
		})
	}))
	defer srv.Close()

	schema := map[string]any{"type": "object", "properties": map[string]any{"done": map[string]any{"type": "boolean"}}}
	client := NewClient(srv.URL, "test-model")
	if _, err := client.ChatCompletion(context.Background(), []Message{{Role: "user", Content: "hi"}}, ChatOptions{GuidedJSON: schema}); err != nil {
		t.Fatalf("ChatCompletion: %v", err)
	}
	if gotReq.GuidedJSON == nil {
		t.Fatalf("expected guided_json to be sent in request")
	}
}

func TestChatCompletionNonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-model")
	if _, err := client.ChatCompletion(context.Background(), []Message{{Role: "user", Content: "hi"}}, ChatOptions{}); err == nil {
		t.Fatalf("expected error for non-200 status")
	}
}

func TestChatCompletionEmptyContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": ""}}},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-model")
	if _, err := client.ChatCompletion(context.Background(), []Message{{Role: "user", Content: "hi"}}, ChatOptions{}); err == nil {
		t.Fatalf("expected error for empty content (e.g. truncated reasoning)")
	}
}

// TestChatCompletionFallsBackToReasoningContentWithGuidedJSON reproduces
// a real observed vLLM behavior: with deepseek_r1's reasoning parser +
// guided_json combined, the schema-constrained answer can land entirely
// in reasoning_content while content stays null. When GuidedJSON was
// requested, ChatCompletion must recover the answer from there rather
// than erroring out.
func TestChatCompletionFallsBackToReasoningContentWithGuidedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{
				"content":           nil,
				"reasoning_content": `{"done":false,"tool":"nmap","target":"127.0.0.1"}`,
			}}},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-model")
	content, err := client.ChatCompletion(context.Background(), []Message{{Role: "user", Content: "hi"}}, ChatOptions{
		GuidedJSON: map[string]any{"type": "object"},
	})
	if err != nil {
		t.Fatalf("ChatCompletion: %v", err)
	}
	if content != `{"done":false,"tool":"nmap","target":"127.0.0.1"}` {
		t.Fatalf("expected fallback to reasoning_content, got: %s", content)
	}
}

// TestChatCompletionDoesNotFallBackWithoutGuidedJSON guards against
// silently returning a truncated mid-thought fragment as if it were a
// real answer when GuidedJSON wasn't used — see the ReasoningContent
// field comment for why the fallback is scoped to the guided case only.
func TestChatCompletionDoesNotFallBackWithoutGuidedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{
				"content":           nil,
				"reasoning_content": "Okay, let me think about th",
			}}},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-model")
	if _, err := client.ChatCompletion(context.Background(), []Message{{Role: "user", Content: "hi"}}, ChatOptions{}); err == nil {
		t.Fatalf("expected error, not a silent fallback to a mid-thought reasoning fragment")
	}
}

func TestChatCompletionNoChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{}})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-model")
	if _, err := client.ChatCompletion(context.Background(), []Message{{Role: "user", Content: "hi"}}, ChatOptions{}); err == nil {
		t.Fatalf("expected error when response has no choices")
	}
}
