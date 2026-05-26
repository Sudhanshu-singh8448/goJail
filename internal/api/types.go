// Package api defines the HTTP request and response types for the goboxd API.
package api

import (
	"github.com/nsjail-server/gojail/internal/types"
)

// RunRequest is the JSON body for POST /run.
type RunRequest struct {
	Language         string         `json:"language"`
	Source           string         `json:"source"`
	SourceFilename   string         `json:"source_filename,omitempty"`
	ArtifactFilename string         `json:"artifact_filename,omitempty"`
	Build            *PhaseOpts     `json:"build,omitempty"`
	Run              *PhaseOpts     `json:"run,omitempty"`
	Tests            []types.TestCase `json:"tests"`
}

// PhaseOpts holds per-request overrides for a build or run phase.
type PhaseOpts struct {
	Limits *types.LimitsOverride `json:"limits,omitempty"`
	Flags  []string              `json:"flags,omitempty"`
}

// RunResponse is the JSON response for POST /run.
type RunResponse struct {
	Status string              `json:"status"`
	Build  *types.BuildResult  `json:"build,omitempty"`
	Tests  []types.TestResult  `json:"tests"`
}

// ErrorResponse is returned for 4xx/5xx errors.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains the error code and message.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// HealthResponse is the response for GET /healthz.
type HealthResponse struct {
	Status string `json:"status"`
}

// ReadyzResponse is the response for GET /readyz.
type ReadyzResponse struct {
	Status    string                      `json:"status"`
	Nsjail    NsjailStatus                `json:"nsjail"`
	Languages map[string]LanguageStatus   `json:"languages"`
}

// NsjailStatus reports nsjail health.
type NsjailStatus struct {
	OK      bool   `json:"ok"`
	Version string `json:"version,omitempty"`
	Error   string `json:"error,omitempty"`
}

// LanguageStatus reports a single language's health.
type LanguageStatus struct {
	OK      bool   `json:"ok"`
	Version string `json:"version,omitempty"`
	Error   string `json:"error,omitempty"`
}

// InfoResponse is the response for GET /info.
type InfoResponse struct {
	BuildInfo  BuildInfo      `json:"build_info"`
	Nsjail     NsjailInfo     `json:"nsjail"`
	Languages  []LanguageInfo `json:"languages"`
	Limits     LimitsInfo     `json:"limits"`
	Stats      StatsInfo      `json:"stats"`
}

// BuildInfo holds the server's build metadata.
type BuildInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	GoVersion string `json:"go_version"`
}

// NsjailInfo holds nsjail metadata.
type NsjailInfo struct {
	Path    string `json:"path"`
	Version string `json:"version"`
}

// LanguageInfo holds info about a registered language for /info.
type LanguageInfo struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Version         string          `json:"version"`
	DefaultRunLimits *LimitsDisplay `json:"default_run_limits"`
}

// LimitsDisplay is the JSON shape for limits in /info responses.
type LimitsDisplay struct {
	WallTimeS    int `json:"wall_time_s"`
	MemoryKB     int `json:"memory_kb"`
	MaxProcesses int `json:"max_processes"`
}

// LimitsInfo holds the server's global limits for /info.
type LimitsInfo struct {
	MaxSourceBytes    int `json:"max_source_bytes"`
	MaxTests          int `json:"max_tests"`
	MaxConcurrentJobs int `json:"max_concurrent_jobs"`
}

// StatsInfo holds runtime statistics for /info.
type StatsInfo struct {
	InFlightJobs         int64   `json:"in_flight_jobs"`
	JobsTotal            int64   `json:"jobs_total"`
	JobsFailedInternal   int64   `json:"jobs_failed_internal"`
	LastInternalErrorAt  *string `json:"last_internal_error_at,omitempty"`
	DiskFreeBytesJailDir int64   `json:"disk_free_bytes_jail_dir"`
}
