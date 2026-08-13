// Package rag is a thin HTTP client for the shumbola.ai RAG microservice
// (services/rag-service), which wraps the pentest knowledge base (TZ
// 8-bo'lim: OWASP WSTG, HackTricks, PayloadsAllTheThings, OWASP Top 10/
// API-Security, Cheat Sheet Series — 7760 embedded chunks). Embedding and
// vector search stay in Python (chromadb + fastembed have no Go-native
// equivalent, and the query embedding must use the exact same model the
// knowledge base was indexed with) — this package only calls out to it,
// mirroring internal/llm's client-to-a-model-server pattern.
package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Chunk is one retrieved knowledge base passage.
type Chunk struct {
	Text     string  `json:"text"`
	Source   string  `json:"source"`
	FilePath string  `json:"file_path"`
	Heading  string  `json:"heading"`
	Distance float64 `json:"distance"`
}

// Client queries the RAG microservice's /query endpoint.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient builds a Client against baseURL (e.g. "http://localhost:8090").
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			// Retrieval is just an embedding + ANN lookup — should be
			// fast; bounded generously in case the service is cold
			// (first request after startup loads the ONNX model).
			Timeout: 30 * time.Second,
		},
	}
}

type queryRequest struct {
	Query string `json:"query"`
	K     int    `json:"k"`
}

type queryResponse struct {
	Chunks []Chunk `json:"chunks"`
	Detail string  `json:"detail"` // FastAPI's error body shape
}

// Retrieve returns the top k chunks most relevant to query.
func (c *Client) Retrieve(ctx context.Context, query string, k int) ([]Chunk, error) {
	reqBody, err := json.Marshal(queryRequest{Query: query, K: k})
	if err != nil {
		return nil, fmt.Errorf("rag: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/query", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("rag: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("rag: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("rag: read response: %w", err)
	}

	var parsed queryResponse
	if resp.StatusCode != http.StatusOK {
		_ = json.Unmarshal(body, &parsed) // best-effort — surface FastAPI's detail message if present
		if parsed.Detail != "" {
			return nil, fmt.Errorf("rag: server returned status %d: %s", resp.StatusCode, parsed.Detail)
		}
		return nil, fmt.Errorf("rag: server returned status %d: %s", resp.StatusCode, string(body))
	}

	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("rag: unmarshal response: %w", err)
	}
	return parsed.Chunks, nil
}
