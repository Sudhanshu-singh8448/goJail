// Package types defines shared data types used across the goboxd codebase.
// This package has no dependencies on other internal packages to avoid import cycles.
package types

// LimitsOverride allows partial override of language defaults.
// Pointers so zero-value means "not specified" rather than "set to zero".
type LimitsOverride struct {
	WallTimeS    *int `json:"wall_time_s,omitempty"`
	MemoryKB     *int `json:"memory_kb,omitempty"`
	MaxProcesses *int `json:"max_processes,omitempty"`
}

// TestCase is one test entry in the request.
type TestCase struct {
	Stdin          string `json:"stdin"`
	ExpectedStdout string `json:"expected_stdout"`
}

// BuildResult represents the result of the compilation step.
type BuildResult struct {
	Status     string `json:"status"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	DurationMs int64  `json:"duration_ms"`
}

// TestResult represents the result of one test execution.
type TestResult struct {
	Status       string `json:"status"`
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr"`
	DurationMs   int64  `json:"duration_ms"`
	MemoryPeakKB int64  `json:"memory_peak_kb"`
}
