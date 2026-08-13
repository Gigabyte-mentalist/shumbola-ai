// Package config loads agent-core runtime configuration from the
// environment. Kept deliberately simple (no config library) — Bosqich 0
// only needs enough knobs to start the server and audit log.
package config

import "os"

// Config holds the environment-derived settings for the agent-core service.
type Config struct {
	ListenAddr      string
	AuditLogPath    string
	DockerHost      string
	SeedOperatorKey string
	SeedAdminKey    string
	// SeedAdminKey2 is a second Administrator seed key. FR-AUTHZ-5's
	// two-person rule fundamentally requires at least two distinct
	// Administrators to ever counter-approve a RoE — a deployment with
	// only SeedAdminKey set can upload a RoE but can never satisfy the
	// two-person rule at all. Optional so existing single-admin setups
	// (which never enable exploit mode) keep working unchanged.
	SeedAdminKey2 string
	// ReportFontPath, if set, points at a Cyrillic-capable TTF (e.g. the
	// ttf-dejavu package's DejaVuSans.ttf) used to render uz-Cyrl/ru PDF
	// reports correctly. If empty, main probes a few common install
	// paths before falling back to report.RenderPDF's Latin-only default.
	ReportFontPath string
	// LLMBaseURL, if set, points at an OpenAI-compatible chat completions
	// endpoint (e.g. "http://<gpu-server>:18000/v1") and switches the
	// Engine from StaticPlanner to LLMPlanner. Left unset, agent-core
	// degrades to StaticPlanner — deployment-specific, never defaulted
	// in code.
	LLMBaseURL string
	// LLMModel is the model name to request (must match what the server
	// is actually serving, e.g. "Qwen/Qwen3-32B-FP8").
	LLMModel string
	// RAGServiceURL, if set, points at the RAG microservice
	// (services/rag-service, e.g. "http://localhost:8090") and lets
	// LLMPlanner ground its decisions in the pentest knowledge base (TZ
	// 8-bo'lim). Left unset, LLMPlanner reasons from the prompt alone.
	RAGServiceURL string
}

// Load reads configuration from environment variables, applying sensible
// development defaults where unset.
func Load() Config {
	return Config{
		ListenAddr:      getenv("LISTEN_ADDR", ":8080"),
		AuditLogPath:    getenv("AUDIT_LOG_PATH", "data/audit.log"),
		DockerHost:      os.Getenv("DOCKER_HOST"), // empty = docker client uses its own default
		SeedOperatorKey: os.Getenv("SEED_OPERATOR_KEY"),
		SeedAdminKey:    os.Getenv("SEED_ADMIN_KEY"),
		SeedAdminKey2:   os.Getenv("SEED_ADMIN_KEY_2"),
		ReportFontPath:  os.Getenv("REPORT_FONT_PATH"),
		LLMBaseURL:      os.Getenv("LLM_BASE_URL"),
		LLMModel:        os.Getenv("LLM_MODEL"),
		RAGServiceURL:   os.Getenv("RAG_SERVICE_URL"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
