package rag

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRetrieveSendsCorrectRequestAndParsesResponse(t *testing.T) {
	var gotReq queryRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/query" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"chunks": []map[string]any{
				{"text": "SSH pentest info", "source": "hacktricks", "file_path": "a.md", "heading": "SSH", "distance": 0.32},
			},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	chunks, err := client.Retrieve(context.Background(), "ssh port 22", 3)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if gotReq.Query != "ssh port 22" || gotReq.K != 3 {
		t.Fatalf("unexpected request sent: %+v", gotReq)
	}
	if len(chunks) != 1 || chunks[0].Source != "hacktricks" || chunks[0].Text != "SSH pentest info" {
		t.Fatalf("unexpected chunks: %+v", chunks)
	}
}

func TestRetrieveSurfacesFastAPIErrorDetail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": "rag service not ready"})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	_, err := client.Retrieve(context.Background(), "query", 5)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "rag service not ready") {
		t.Fatalf("expected error to include FastAPI detail message, got: %v", err)
	}
}

func TestRetrieveEmptyChunks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"chunks": []map[string]any{}})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	chunks, err := client.Retrieve(context.Background(), "obscure query", 5)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(chunks) != 0 {
		t.Fatalf("expected 0 chunks, got %d", len(chunks))
	}
}
