package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nsjail-server/gojail/internal/config"
	"github.com/nsjail-server/gojail/internal/sandbox"
	"github.com/nsjail-server/gojail/internal/types"
	"github.com/nsjail-server/gojail/internal/worker"
	"golang.org/x/sys/unix"
)

// Build-time variables injected via ldflags.
var (
	Version   = "dev"
	Commit    = "unknown"
	GoVersion = runtime.Version()
)

// Server holds the shared state for all HTTP handlers.
type Server struct {
	Config    *config.ServerSettings
	Languages []config.LanguageConfig
	Pool      *worker.Pool
}

// NewServer creates a new Server instance.
func NewServer(cfg *config.ServerSettings, langs []config.LanguageConfig, pool *worker.Pool) *Server {
	return &Server{
		Config:    cfg,
		Languages: langs,
		Pool:      pool,
	}
}

// HandleHealthz handles GET /healthz — liveness check.
func (s *Server) HandleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, HealthResponse{Status: "ok"})
}

// HandleReadyz handles GET /readyz — readiness check.
// Checks nsjail binary and each language's smoke command.
func (s *Server) HandleReadyz(w http.ResponseWriter, r *http.Request) {
	resp := ReadyzResponse{
		Status:    "ok",
		Languages: make(map[string]LanguageStatus),
	}

	// Check nsjail
	nsjailVer, err := sandbox.GetNsjailVersion(s.Config.NsjailPath)
	if err != nil {
		resp.Nsjail = NsjailStatus{OK: false, Error: err.Error()}
		resp.Status = "degraded"
	} else {
		resp.Nsjail = NsjailStatus{OK: true, Version: nsjailVer}
	}

	// Check each language
	for _, lang := range s.Languages {
		if len(lang.SmokeCmd) == 0 {
			resp.Languages[lang.ID] = LanguageStatus{OK: true, Version: "unknown"}
			continue
		}
		version, err := sandbox.RunSmokeCmd(lang.SmokeCmd)
		if err != nil {
			resp.Languages[lang.ID] = LanguageStatus{OK: false, Error: err.Error()}
			resp.Status = "degraded"
		} else {
			resp.Languages[lang.ID] = LanguageStatus{OK: true, Version: version}
		}
	}

	status := http.StatusOK
	if resp.Status == "degraded" {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, resp)
}

// HandleInfo handles GET /info — server info.
func (s *Server) HandleInfo(w http.ResponseWriter, r *http.Request) {
	nsjailVer, _ := sandbox.GetNsjailVersion(s.Config.NsjailPath)

	languages := make([]LanguageInfo, 0, len(s.Languages))
	for _, lang := range s.Languages {
		version := ""
		if len(lang.SmokeCmd) > 0 {
			version, _ = sandbox.RunSmokeCmd(lang.SmokeCmd)
		}
		languages = append(languages, LanguageInfo{
			ID:      lang.ID,
			Name:    lang.Name,
			Version: version,
			DefaultRunLimits: &LimitsDisplay{
				WallTimeS:    lang.Run.Limits.WallTimeS,
				MemoryKB:     lang.Run.Limits.MemoryKB,
				MaxProcesses: lang.Run.Limits.MaxProcesses,
			},
		})
	}

	poolStats := s.Pool.Stats()

	var lastErr *string
	if poolStats.HasInternalErr {
		t := poolStats.LastInternalErr.UTC().Format(time.RFC3339)
		lastErr = &t
	}

	// Get disk free space for jail directory
	diskFree := getDiskFree(s.Config.JailDir)

	resp := InfoResponse{
		BuildInfo: BuildInfo{
			Version:   Version,
			Commit:    Commit,
			GoVersion: GoVersion,
		},
		Nsjail: NsjailInfo{
			Path:    s.Config.NsjailPath,
			Version: nsjailVer,
		},
		Languages: languages,
		Limits: LimitsInfo{
			MaxSourceBytes:    s.Config.MaxSourceBytes,
			MaxTests:          s.Config.MaxTests,
			MaxConcurrentJobs: s.Pool.MaxConcurrent(),
		},
		Stats: StatsInfo{
			InFlightJobs:         poolStats.InFlight,
			JobsTotal:            poolStats.Total,
			JobsFailedInternal:   poolStats.FailedInternal,
			LastInternalErrorAt:  lastErr,
			DiskFreeBytesJailDir: diskFree,
		},
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleRun handles POST /run — the main code execution endpoint.
func (s *Server) HandleRun(w http.ResponseWriter, r *http.Request) {
	requestID := uuid.New().String()
	start := time.Now()

	logger := slog.With("request_id", requestID)

	// Security fix #4: limit request body size (2x max source to allow for JSON overhead)
	maxBody := int64(s.Config.MaxSourceBytes*2 + 1048576) // source + tests overhead
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)

	// Parse request
	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Warn("invalid request body", "error", err)
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: ErrorDetail{
				Code:    "invalid_json",
				Message: "invalid JSON body: " + err.Error(),
			},
		})
		return
	}

	logger = logger.With("language", req.Language)

	// Validate
	if errResp := ValidateRunRequest(&req, s.Config, s.Languages); errResp != nil {
		logger.Warn("validation failed", "code", errResp.Error.Code, "message", errResp.Error.Message)
		writeJSON(w, http.StatusBadRequest, errResp)
		return
	}

	lang := config.FindLanguage(s.Languages, req.Language)
	if lang == nil {
		// Should not happen after validation, but guard
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: ErrorDetail{Code: "unknown_language", Message: "language not found"},
		})
		return
	}

	// Run inside the worker pool — this blocks until a slot is available
	var resp RunResponse
	var runErr error

	err := s.Pool.SubmitAndWait(r.Context(), func() error {
		resp, runErr = s.executeRun(r.Context(), &req, lang, logger)
		return runErr
	})

	if err != nil {
		if err == context.Canceled || err == context.DeadlineExceeded {
			logger.Warn("request cancelled while waiting for pool slot")
			writeJSON(w, http.StatusServiceUnavailable, ErrorResponse{
				Error: ErrorDetail{Code: "queue_timeout", Message: "request cancelled while waiting"},
			})
			return
		}
		logger.Error("pool submission failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error: ErrorDetail{Code: "internal_error", Message: "execution failed"},
		})
		return
	}

	duration := time.Since(start)
	logger.Info("request completed",
		"status", resp.Status,
		"duration_ms", duration.Milliseconds(),
		"tests", len(resp.Tests),
	)

	writeJSON(w, http.StatusOK, resp)
}

// executeRun performs the actual code execution inside a sandbox.
func (s *Server) executeRun(ctx context.Context, req *RunRequest, lang *config.LanguageConfig, logger *slog.Logger) (RunResponse, error) {
	var resp RunResponse

	// Create sandbox
	sb, err := sandbox.NewSandbox(lang, s.Config.NsjailPath, s.Config)
	if err != nil {
		logger.Error("failed to create sandbox", "error", err)
		resp.Status = sandbox.StatusInternalError
		return resp, fmt.Errorf("creating sandbox: %w", err)
	}
	// Security fix #7: cleanup on every exit path
	defer sb.Cleanup()

	// Write source
	if err := sb.WriteSource(req.Source, req.SourceFilename, req.ArtifactFilename); err != nil {
		logger.Error("failed to write source", "error", err)
		resp.Status = sandbox.StatusInternalError
		return resp, fmt.Errorf("writing source: %w", err)
	}

	// Build step
	var buildLimits *types.LimitsOverride
	var buildFlags []string
	if req.Build != nil {
		buildLimits = req.Build.Limits
		buildFlags = req.Build.Flags
	}

	buildResult, err := sb.Build(buildLimits, buildFlags)
	if err != nil {
		logger.Error("build execution failed", "error", err)
		resp.Status = sandbox.StatusInternalError
		resp.Build = &types.BuildResult{Status: sandbox.StatusBuildInternalError}
		return resp, nil // Return nil so pool doesn't count as internal error
	}

	if lang.NeedsCompilation() {
		resp.Build = buildResult
	}

	// If build failed, mark all tests as not_executed
	if buildResult.Status != sandbox.StatusBuildOK {
		resp.Status = sandbox.StatusOverallBuildFailed
		resp.Tests = make([]types.TestResult, len(req.Tests))
		for i := range resp.Tests {
			resp.Tests[i] = types.TestResult{Status: sandbox.StatusNotExecuted}
		}
		return resp, nil
	}

	// Run tests concurrently
	var runLimits *types.LimitsOverride
	var runFlags []string
	if req.Run != nil {
		runLimits = req.Run.Limits
		runFlags = req.Run.Flags
	}

	results := make([]types.TestResult, len(req.Tests))
	var wg sync.WaitGroup

	for i, test := range req.Tests {
		wg.Add(1)
		go func(idx int, tc types.TestCase) {
			defer wg.Done()

			result, err := sb.RunTest(tc, runLimits, runFlags)
			if err != nil {
				logger.Error("test execution failed", "test_index", idx, "error", err)
				results[idx] = types.TestResult{Status: sandbox.StatusInternalError}
				return
			}
			results[idx] = *result
		}(i, test)
	}
	wg.Wait()

	resp.Tests = results

	// Compute overall status
	testStatuses := make([]string, len(results))
	for i, r := range results {
		testStatuses[i] = r.Status
	}
	resp.Status = sandbox.ComputeOverallStatus(buildResult.Status, testStatuses)

	return resp, nil
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// getDiskFree returns available disk space in bytes for the given path.
func getDiskFree(path string) int64 {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0
	}
	return int64(stat.Bavail) * int64(stat.Bsize)
}
