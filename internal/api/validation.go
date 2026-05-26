package api

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/nsjail-server/gojail/internal/config"
)

// ValidateRunRequest validates a RunRequest against the server config and registered languages.
// Returns an ErrorResponse if validation fails, nil otherwise.
func ValidateRunRequest(req *RunRequest, cfg *config.ServerSettings, langs []config.LanguageConfig) *ErrorResponse {
	// Language is required
	if req.Language == "" {
		return &ErrorResponse{Error: ErrorDetail{
			Code:    "missing_language",
			Message: "language is required",
		}}
	}

	// Language must be registered
	lang := config.FindLanguage(langs, req.Language)
	if lang == nil {
		return &ErrorResponse{Error: ErrorDetail{
			Code:    "unknown_language",
			Message: fmt.Sprintf("language %q is not configured", req.Language),
		}}
	}

	// Source is required
	if req.Source == "" {
		return &ErrorResponse{Error: ErrorDetail{
			Code:    "missing_source",
			Message: "source is required",
		}}
	}

	// Source must be valid UTF-8
	if !utf8.ValidString(req.Source) {
		return &ErrorResponse{Error: ErrorDetail{
			Code:    "invalid_source",
			Message: "source must be valid UTF-8",
		}}
	}

	// Source size check
	if len(req.Source) > cfg.MaxSourceBytes {
		return &ErrorResponse{Error: ErrorDetail{
			Code:    "source_too_large",
			Message: fmt.Sprintf("source exceeds maximum size of %d bytes", cfg.MaxSourceBytes),
		}}
	}

	// Tests are required, at least one
	if len(req.Tests) == 0 {
		return &ErrorResponse{Error: ErrorDetail{
			Code:    "missing_tests",
			Message: "at least one test is required",
		}}
	}

	// Test count check
	if len(req.Tests) > cfg.MaxTests {
		return &ErrorResponse{Error: ErrorDetail{
			Code:    "too_many_tests",
			Message: fmt.Sprintf("number of tests (%d) exceeds maximum of %d", len(req.Tests), cfg.MaxTests),
		}}
	}

	// Per-test stdin and expected_stdout size checks
	for i, tc := range req.Tests {
		if len(tc.Stdin) > cfg.MaxStdinBytes {
			return &ErrorResponse{Error: ErrorDetail{
				Code:    "stdin_too_large",
				Message: fmt.Sprintf("test %d stdin exceeds maximum size of %d bytes", i, cfg.MaxStdinBytes),
			}}
		}
		if len(tc.ExpectedStdout) > cfg.MaxExpectedBytes {
			return &ErrorResponse{Error: ErrorDetail{
				Code:    "expected_stdout_too_large",
				Message: fmt.Sprintf("test %d expected_stdout exceeds maximum size of %d bytes", i, cfg.MaxExpectedBytes),
			}}
		}
	}

	// Validate source_filename if provided
	if req.SourceFilename != "" {
		if err := ValidateFilename(req.SourceFilename); err != nil {
			return &ErrorResponse{Error: ErrorDetail{
				Code:    "invalid_filename",
				Message: fmt.Sprintf("source_filename: %s", err),
			}}
		}
	}

	// Validate artifact_filename if provided
	if req.ArtifactFilename != "" {
		if err := ValidateFilename(req.ArtifactFilename); err != nil {
			return &ErrorResponse{Error: ErrorDetail{
				Code:    "invalid_filename",
				Message: fmt.Sprintf("artifact_filename: %s", err),
			}}
		}
	}

	// Validate source_filename / artifact_filename for languages that require them
	if lang.SourceFilenameStrategy == "from_request" && req.SourceFilename == "" {
		return &ErrorResponse{Error: ErrorDetail{
			Code:    "missing_filename",
			Message: fmt.Sprintf("language %q requires source_filename in the request", req.Language),
		}}
	}
	if lang.ArtifactFilenameStrategy == "from_request" && req.ArtifactFilename == "" {
		return &ErrorResponse{Error: ErrorDetail{
			Code:    "missing_filename",
			Message: fmt.Sprintf("language %q requires artifact_filename in the request", req.Language),
		}}
	}

	// Validate build flags
	if req.Build != nil && len(req.Build.Flags) > 0 {
		if lang.Build == nil {
			return &ErrorResponse{Error: ErrorDetail{
				Code:    "invalid_flags",
				Message: fmt.Sprintf("language %q does not support build flags (no build step)", req.Language),
			}}
		}
		if err := ValidateFlags(req.Build.Flags, lang.Build.FlagAllowlist); err != nil {
			return &ErrorResponse{Error: ErrorDetail{
				Code:    "disallowed_flag",
				Message: fmt.Sprintf("build flags: %s", err),
			}}
		}
	}

	// Validate run flags
	if req.Run != nil && len(req.Run.Flags) > 0 {
		if err := ValidateFlags(req.Run.Flags, lang.Run.FlagAllowlist); err != nil {
			return &ErrorResponse{Error: ErrorDetail{
				Code:    "disallowed_flag",
				Message: fmt.Sprintf("run flags: %s", err),
			}}
		}
	}

	return nil
}

// ValidateFilename ensures a filename is a single, safe path component.
// Security fix #1: prevents path traversal via filename.
func ValidateFilename(name string) error {
	if name == "" {
		return fmt.Errorf("filename must not be empty")
	}

	// Length cap
	if len(name) > 255 {
		return fmt.Errorf("filename must be at most 255 characters")
	}

	// No path separators
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("filename must be a single path component (no separators)")
	}

	// No leading dot (prevents .., .hidden, etc.)
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("filename must not start with a dot")
	}

	// No null bytes
	if strings.ContainsRune(name, 0) {
		return fmt.Errorf("filename must not contain null bytes")
	}

	// Must equal its own base (catches edge cases)
	if filepath.Base(name) != name {
		return fmt.Errorf("filename must be a single path component")
	}

	return nil
}

// ValidateFlags checks that every flag in the list is allowed by the allowlist.
// Allowlist entries can use glob patterns (e.g., "-std=c++*" matches "-std=c++17").
// Security fix #3: prevents compiler-flag injection.
func ValidateFlags(flags []string, allowlist []string) error {
	if len(allowlist) == 0 && len(flags) > 0 {
		return fmt.Errorf("flag %q is not allowed (no flags permitted for this language)", flags[0])
	}

	for _, flag := range flags {
		if !isFlagAllowed(flag, allowlist) {
			return fmt.Errorf("flag %q is not in the allow-list", flag)
		}
	}
	return nil
}

// isFlagAllowed checks a single flag against the allowlist using glob matching.
func isFlagAllowed(flag string, allowlist []string) bool {
	for _, pattern := range allowlist {
		if matched, _ := filepath.Match(pattern, flag); matched {
			return true
		}
		// Exact match fallback
		if flag == pattern {
			return true
		}
	}
	return false
}
