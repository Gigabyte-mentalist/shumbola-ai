// Command server runs the shumbola.ai agent-core HTTP API.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/shumbola/agent-core/internal/agentcore"
	"github.com/shumbola/agent-core/internal/audit"
	"github.com/shumbola/agent-core/internal/auth"
	"github.com/shumbola/agent-core/internal/config"
	"github.com/shumbola/agent-core/internal/engagement"
	"github.com/shumbola/agent-core/internal/exploit"
	"github.com/shumbola/agent-core/internal/guardrails"
	"github.com/shumbola/agent-core/internal/hitl"
	"github.com/shumbola/agent-core/internal/httpserver"
	"github.com/shumbola/agent-core/internal/llm"
	"github.com/shumbola/agent-core/internal/rag"
	"github.com/shumbola/agent-core/internal/roe"
	"github.com/shumbola/agent-core/internal/sandbox"
	"github.com/shumbola/agent-core/internal/tasktree"
	"github.com/shumbola/agent-core/internal/tools"
	"github.com/shumbola/agent-core/internal/validation"
)

func main() {
	cfg := config.Load()

	dataDir := "data"
	if dir := filepath.Dir(cfg.AuditLogPath); dir != "." {
		dataDir = dir
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("create data directory: %v", err)
	}

	auditLogger, err := audit.NewLogger(cfg.AuditLogPath)
	if err != nil {
		log.Fatalf("open audit log: %v", err)
	}
	defer auditLogger.Close()

	seeds := map[string]auth.Principal{}
	if cfg.SeedOperatorKey != "" {
		seeds[cfg.SeedOperatorKey] = auth.Principal{ID: "seed-operator", Role: auth.RoleOperator}
	}
	if cfg.SeedAdminKey != "" {
		seeds[cfg.SeedAdminKey] = auth.Principal{ID: "seed-admin", Role: auth.RoleAdministrator}
	}
	if cfg.SeedAdminKey2 != "" {
		seeds[cfg.SeedAdminKey2] = auth.Principal{ID: "seed-admin-2", Role: auth.RoleAdministrator}
	}
	if len(seeds) == 0 {
		log.Println("warning: no SEED_OPERATOR_KEY/SEED_ADMIN_KEY set — no one can authenticate yet")
	}
	keyStore := auth.NewInMemoryKeyStore(seeds)

	engStore, err := engagement.NewFileStore(filepath.Join(dataDir, "engagements"))
	if err != nil {
		log.Fatalf("open engagement store: %v", err)
	}
	treeStore, err := tasktree.NewFileStore(filepath.Join(dataDir, "tasktrees"))
	if err != nil {
		log.Fatalf("open task tree store: %v", err)
	}
	roeStore, err := roe.NewFileStore(filepath.Join(dataDir, "roe"))
	if err != nil {
		log.Fatalf("open RoE store: %v", err)
	}
	approvals := hitl.NewStore()

	toolset := map[string]tools.Tool{}
	var nucleiCheckAdapter *exploit.NucleiCheckAdapter
	sandboxMgr, err := sandbox.NewManager()
	if err != nil {
		// A missing/unreachable Docker daemon should not prevent the
		// rest of the API (engagement/scope management, audit, RBAC)
		// from working — only tool execution degrades, and it fails
		// loudly per-request rather than crashing the server (TZ 6.3).
		log.Printf("warning: sandbox unavailable, tools will not run: %v", err)
	} else {
		runner := &sandboxRunner{mgr: sandboxMgr}
		toolset["nmap"] = tools.NewNmapTool(runner, "")
		toolset["smbenum"] = tools.NewSmbEnumTool(runner, "")
		toolset["nuclei"] = tools.NewNucleiTool(runner, "")
		toolset["ffuf"] = tools.NewFfufTool(runner, "", "")
		toolset["testssl"] = tools.NewTestsslTool(runner, "")
		nucleiCheckAdapter = exploit.NewNucleiCheckAdapter(runner, "")
	}
	// FR-SAFE-7: at least 30s between exploitation actions against the
	// same target, shared across all requests (a package-level limiter,
	// not per-request) so it actually throttles repeated calls.
	exploitLimiter := guardrails.NewRateLimiter(30 * time.Second)

	planner := choosePlanner(cfg, toolset)

	validator := validation.NewValidator()
	engine := agentcore.NewEngine(toolset, planner, approvals, auditLogger, engStore, treeStore, validator)

	reportFontPath := cfg.ReportFontPath
	if reportFontPath == "" {
		reportFontPath = detectReportFont()
	}
	if reportFontPath == "" {
		log.Println("warning: no Cyrillic font found (set REPORT_FONT_PATH) — uz-Cyrl/ru PDF reports will fall back to a Latin-only font")
	}

	handler := httpserver.New(httpserver.Deps{
		KeyStore:           keyStore,
		AuditLogger:        auditLogger,
		Engagements:        engStore,
		Trees:              treeStore,
		Approvals:          approvals,
		Engine:             engine,
		RoEStore:           roeStore,
		ExploitNucleiCheck: nucleiCheckAdapter,
		ExploitLimiter:     exploitLimiter,
		ReportFontPath:     reportFontPath,
	})

	log.Printf("agent-core listening on %s", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, handler); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// candidateReportFonts are common install paths for the Cyrillic-capable
// DejaVu Sans font across the Linux distros this service is likely to
// run in (the shipped Dockerfile installs Alpine's ttf-dejavu package).
var candidateReportFonts = []string{
	"/usr/share/fonts/dejavu/DejaVuSans.ttf",          // Alpine ttf-dejavu
	"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", // Debian/Ubuntu fonts-dejavu-core
}

// detectReportFont returns the first candidate font path that exists on
// disk, or "" if none do.
func detectReportFont() string {
	for _, path := range candidateReportFonts {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// toolDescriptions is shown to LLMPlanner's system prompt so the model
// knows what each tool does. Kept alongside the toolset construction in
// main() since both are driven by the same fixed tool names.
var toolDescriptions = map[string]string{
	"nmap":    "Tarmoq portlarini va ishlayotgan xizmatlarni skanerlaydi (recon).",
	"smbenum": "SMB xizmatini (135/139/445-port) o'qish uchun tekshiradi: OS, protokol versiyalari, umumiy papkalar (shares). Faqat nmap SMB/microsoft-ds portini topgan nishonlarda foydali.",
	"nuclei":  "Ma'lum CVE/zaifliklar shabloniga asoslanib zaifliklarni qidiradi.",
	"ffuf":    "Yashirin fayl/katalog/yo'llarni (masalan admin panel) topadi.",
	"testssl": "TLS/SSL konfiguratsiyasi va sertifikatidagi zaifliklarni tekshiradi.",
}

// choosePlanner returns an LLMPlanner if cfg.LLMBaseURL is configured,
// otherwise falls back to StaticPlanner — mirrors the sandbox-unavailable
// degradation above: a missing/unset LLM endpoint should not prevent the
// rest of the service from working.
func choosePlanner(cfg config.Config, toolset map[string]tools.Tool) agentcore.Planner {
	if cfg.LLMBaseURL == "" {
		log.Println("warning: LLM_BASE_URL not set — using StaticPlanner (no model-driven reasoning)")
		return &agentcore.StaticPlanner{}
	}

	var toolInfos []agentcore.ToolInfo
	for name := range toolset {
		desc := toolDescriptions[name]
		if desc == "" {
			desc = name
		}
		toolInfos = append(toolInfos, agentcore.ToolInfo{Name: name, Description: desc})
	}

	client := llm.NewClient(cfg.LLMBaseURL, cfg.LLMModel)
	log.Printf("using LLMPlanner: model=%s base_url=%s", cfg.LLMModel, cfg.LLMBaseURL)

	var ragClient agentcore.RAGRetriever
	if cfg.RAGServiceURL != "" {
		ragClient = rag.NewClient(cfg.RAGServiceURL)
		log.Printf("RAG knowledge base enabled: %s", cfg.RAGServiceURL)
	} else {
		log.Println("warning: RAG_SERVICE_URL not set — LLMPlanner will reason without the knowledge base")
	}

	return agentcore.NewLLMPlanner(client, toolInfos, ragClient)
}

// sandboxRunner adapts *sandbox.Manager to tools.ContainerRunner. The
// tools package can't import sandbox directly (that would make tool
// wrappers depend on the concrete container backend), so this thin
// struct-to-struct translation lives at the wiring layer instead.
type sandboxRunner struct {
	mgr *sandbox.Manager
}

func (r *sandboxRunner) RunContainer(ctx context.Context, spec tools.RunSpec) (tools.RunResult, error) {
	res, err := r.mgr.RunContainer(ctx, sandbox.RunSpec{
		Image:       spec.Image,
		Cmd:         spec.Cmd,
		MemoryBytes: spec.MemoryBytes,
		NanoCPUs:    spec.NanoCPUs,
		Network:     spec.Network,
		ReadOnlyFS:  spec.ReadOnlyFS,
		ExtraHosts:  spec.ExtraHosts,
	})
	if err != nil {
		return tools.RunResult{}, err
	}
	return tools.RunResult{Stdout: res.Stdout, Stderr: res.Stderr, ExitCode: res.ExitCode}, nil
}
