package sandbox

import (
	"os/exec"
	"strings"
)

// Status constants matching the API contract.
const (
	StatusAccepted                  = "accepted"
	StatusWrongOutput               = "wrong_output"
	StatusOutputWhitespaceMismatch  = "output_whitespace_mismatch"
	StatusTimeExceeded              = "time_exceeded"
	StatusMemoryExceeded            = "memory_exceeded"
	StatusRuntimeError              = "runtime_error"
	StatusNotExecuted               = "not_executed"
	StatusInternalError             = "internal_error"

	StatusBuildOK                   = "ok"
	StatusBuildFailed               = "failed"
	StatusBuildInternalError        = "internal_error"

	StatusOverallAccepted           = "accepted"
	StatusOverallBuildFailed        = "build_failed"
)

// ParseBuildStatus determines the build status from the nsjail exit error and stderr.
func ParseBuildStatus(err error, stderr string) string {
	if err == nil {
		return StatusBuildOK
	}

	// Check for nsjail-level errors vs user code errors
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode := exitErr.ExitCode()
		// nsjail uses exit code 109 for internal errors
		if exitCode == 109 {
			return StatusBuildInternalError
		}
	}

	// Any other non-zero exit from compilation = build failed
	return StatusBuildFailed
}

// ParseTestStatus determines the test status from the nsjail exit error and stderr.
func ParseTestStatus(err error, stderr string) string {
	if err == nil {
		return StatusAccepted
	}

	// Check stderr for nsjail-specific messages
	if strings.Contains(stderr, "run time >= time limit") ||
		strings.Contains(stderr, "time limit") {
		return StatusTimeExceeded
	}

	if strings.Contains(stderr, "mem_max") ||
		strings.Contains(stderr, "memory limit") ||
		strings.Contains(stderr, "rlimit_as") {
		return StatusMemoryExceeded
	}

	if strings.Contains(stderr, "terminated with signal") {
		// Could be SIGSEGV (memory) or other signal (runtime error)
		if strings.Contains(stderr, "SIGKILL") || strings.Contains(stderr, "signal: 9") {
			return StatusMemoryExceeded
		}
		return StatusRuntimeError
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode := exitErr.ExitCode()
		// nsjail internal error
		if exitCode == 109 {
			return StatusInternalError
		}
		// Non-zero exit from user program
		return StatusRuntimeError
	}

	return StatusRuntimeError
}

// CompareOutput compares actual program output against expected output.
func CompareOutput(actual, expected string) string {
	if actual == expected {
		return StatusAccepted
	}

	// Check if they match after trimming whitespace
	if strings.TrimSpace(actual) == strings.TrimSpace(expected) {
		return StatusOutputWhitespaceMismatch
	}

	return StatusWrongOutput
}

// ComputeOverallStatus determines the top-level status from build + test results.
func ComputeOverallStatus(buildStatus string, tests []string) string {
	if buildStatus != StatusBuildOK {
		return StatusOverallBuildFailed
	}

	for _, status := range tests {
		if status != StatusAccepted {
			return status
		}
	}

	return StatusOverallAccepted
}
