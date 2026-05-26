package api

import (
	"testing"

	"github.com/nsjail-server/gojail/internal/config"
	"github.com/nsjail-server/gojail/internal/types"
)

func defaultServerSettings() *config.ServerSettings {
	return &config.ServerSettings{
		Port:                  8000,
		MaxSourceBytes:        262144,
		MaxTests:              50,
		MaxConcurrentJobs:     4,
		MaxStdinBytes:         1048576,
		MaxExpectedBytes:      1048576,
		MaxStdoutCaptureBytes: 1048576,
		MaxStderrCaptureBytes: 262144,
		JailDir:               "/tmp/goboxd_jails",
		NsjailPath:            "/usr/bin/nsjail",
	}
}

func defaultLanguages() []config.LanguageConfig {
	return []config.LanguageConfig{
		{
			ID:             "py3",
			Name:           "Python 3",
			SourceFilename: "solution.py",
			Run: config.CommandConfig{
				Cmd:  "/usr/bin/python3",
				Args: []string{"{{source}}"},
				Limits: config.Limits{
					WallTimeS:    9,
					MemoryKB:     102400,
					MaxProcesses: 100,
				},
			},
		},
		{
			ID:               "cpp",
			Name:             "C++",
			SourceFilename:   "solution.cpp",
			ArtifactFilename: "solution",
			Build: &config.CommandConfig{
				Cmd:           "/usr/bin/g++",
				Args:          []string{"{{flags}}", "-o", "{{artifact}}", "{{source}}"},
				Limits:        config.Limits{WallTimeS: 3, MemoryKB: 1048576, MaxProcesses: 100},
				FlagAllowlist: []string{"-O0", "-O1", "-O2", "-O3", "-Wall", "-Wextra", "-std=c++*", "-lm"},
			},
			Run: config.CommandConfig{
				Cmd:    "./{{artifact}}",
				Limits: config.Limits{WallTimeS: 3, MemoryKB: 524288, MaxProcesses: 64},
			},
		},
		{
			ID:                       "java",
			Name:                     "Java",
			SourceFilenameStrategy:   "from_request",
			ArtifactFilenameStrategy: "from_request",
			Build: &config.CommandConfig{
				Cmd:           "/usr/bin/javac",
				Args:          []string{"{{flags}}", "{{source}}"},
				Limits:        config.Limits{WallTimeS: 6, MemoryKB: 102400, MaxProcesses: 100},
				FlagAllowlist: []string{},
			},
			Run: config.CommandConfig{
				Cmd:    "/usr/bin/java",
				Args:   []string{"{{artifact}}"},
				Limits: config.Limits{WallTimeS: 6, MemoryKB: 102400, MaxProcesses: 100},
			},
		},
	}
}

// --- Filename validation tests ---

func TestValidateFilename_Valid(t *testing.T) {
	valid := []string{"solution.py", "Main.java", "test_file.cpp", "a.out", "solution123.v"}
	for _, name := range valid {
		if err := ValidateFilename(name); err != nil {
			t.Errorf("expected %q to be valid, got error: %v", name, err)
		}
	}
}

func TestValidateFilename_PathTraversal(t *testing.T) {
	bad := []string{
		"../../etc/passwd",
		"../secret",
		"foo/bar",
		"foo\\bar",
		"/etc/passwd",
	}
	for _, name := range bad {
		if err := ValidateFilename(name); err == nil {
			t.Errorf("expected %q to be rejected", name)
		}
	}
}

func TestValidateFilename_LeadingDot(t *testing.T) {
	bad := []string{".hidden", "..double", ".bashrc"}
	for _, name := range bad {
		if err := ValidateFilename(name); err == nil {
			t.Errorf("expected %q to be rejected (leading dot)", name)
		}
	}
}

func TestValidateFilename_TooLong(t *testing.T) {
	long := ""
	for i := 0; i < 256; i++ {
		long += "a"
	}
	if err := ValidateFilename(long); err == nil {
		t.Error("expected filename longer than 255 chars to be rejected")
	}
}

func TestValidateFilename_Empty(t *testing.T) {
	if err := ValidateFilename(""); err == nil {
		t.Error("expected empty filename to be rejected")
	}
}

// --- Flag validation tests ---

func TestValidateFlags_Allowed(t *testing.T) {
	allowlist := []string{"-O0", "-O1", "-O2", "-O3", "-Wall", "-std=c++*", "-lm"}
	valid := [][]string{
		{"-O2"},
		{"-Wall", "-O3"},
		{"-std=c++17"},
		{"-std=c++20"},
		{"-lm"},
	}
	for _, flags := range valid {
		if err := ValidateFlags(flags, allowlist); err != nil {
			t.Errorf("expected flags %v to be allowed, got: %v", flags, err)
		}
	}
}

func TestValidateFlags_Rejected(t *testing.T) {
	allowlist := []string{"-O0", "-O1", "-O2", "-O3", "-Wall"}
	bad := [][]string{
		{"-fplugin=evil.so"},
		{"-x", "c"},
		{"-B/tmp/evil"},
		{"--specs=evil"},
		{"-Wl,-rpath,/evil"},
		{"@response_file"},
	}
	for _, flags := range bad {
		if err := ValidateFlags(flags, allowlist); err == nil {
			t.Errorf("expected flags %v to be rejected", flags)
		}
	}
}

func TestValidateFlags_EmptyAllowlist(t *testing.T) {
	// Empty allowlist means no flags are allowed
	if err := ValidateFlags([]string{"-O2"}, []string{}); err == nil {
		t.Error("expected flags to be rejected with empty allowlist")
	}
	// But no flags is fine
	if err := ValidateFlags([]string{}, []string{}); err != nil {
		t.Errorf("expected empty flags to be OK, got: %v", err)
	}
}

// --- Full request validation tests ---

func TestValidateRunRequest_Valid(t *testing.T) {
	req := &RunRequest{
		Language: "py3",
		Source:   "print('hello')",
		Tests:    []types.TestCase{{Stdin: "", ExpectedStdout: "hello\n"}},
	}
	errResp := ValidateRunRequest(req, defaultServerSettings(), defaultLanguages())
	if errResp != nil {
		t.Errorf("expected valid request, got error: %s", errResp.Error.Message)
	}
}

func TestValidateRunRequest_MissingLanguage(t *testing.T) {
	req := &RunRequest{
		Source: "print('hello')",
		Tests:  []types.TestCase{{Stdin: "", ExpectedStdout: "hello\n"}},
	}
	errResp := ValidateRunRequest(req, defaultServerSettings(), defaultLanguages())
	if errResp == nil {
		t.Fatal("expected error for missing language")
	}
	if errResp.Error.Code != "missing_language" {
		t.Errorf("expected code 'missing_language', got %q", errResp.Error.Code)
	}
}

func TestValidateRunRequest_UnknownLanguage(t *testing.T) {
	req := &RunRequest{
		Language: "rust",
		Source:   "fn main() {}",
		Tests:    []types.TestCase{{Stdin: "", ExpectedStdout: ""}},
	}
	errResp := ValidateRunRequest(req, defaultServerSettings(), defaultLanguages())
	if errResp == nil {
		t.Fatal("expected error for unknown language")
	}
	if errResp.Error.Code != "unknown_language" {
		t.Errorf("expected code 'unknown_language', got %q", errResp.Error.Code)
	}
}

func TestValidateRunRequest_NoTests(t *testing.T) {
	req := &RunRequest{
		Language: "py3",
		Source:   "print('hello')",
		Tests:    []types.TestCase{},
	}
	errResp := ValidateRunRequest(req, defaultServerSettings(), defaultLanguages())
	if errResp == nil {
		t.Fatal("expected error for no tests")
	}
	if errResp.Error.Code != "missing_tests" {
		t.Errorf("expected code 'missing_tests', got %q", errResp.Error.Code)
	}
}

func TestValidateRunRequest_SourceTooLarge(t *testing.T) {
	cfg := defaultServerSettings()
	cfg.MaxSourceBytes = 10 // tiny limit
	req := &RunRequest{
		Language: "py3",
		Source:   "this is more than 10 bytes of source code",
		Tests:    []types.TestCase{{Stdin: "", ExpectedStdout: ""}},
	}
	errResp := ValidateRunRequest(req, cfg, defaultLanguages())
	if errResp == nil {
		t.Fatal("expected error for oversized source")
	}
	if errResp.Error.Code != "source_too_large" {
		t.Errorf("expected code 'source_too_large', got %q", errResp.Error.Code)
	}
}

func TestValidateRunRequest_JavaMissingFilename(t *testing.T) {
	req := &RunRequest{
		Language: "java",
		Source:   "public class Main { public static void main(String[] args) {} }",
		Tests:    []types.TestCase{{Stdin: "", ExpectedStdout: ""}},
	}
	errResp := ValidateRunRequest(req, defaultServerSettings(), defaultLanguages())
	if errResp == nil {
		t.Fatal("expected error for missing filename with Java")
	}
	if errResp.Error.Code != "missing_filename" {
		t.Errorf("expected code 'missing_filename', got %q", errResp.Error.Code)
	}
}

func TestValidateRunRequest_DisallowedBuildFlag(t *testing.T) {
	req := &RunRequest{
		Language: "cpp",
		Source:   "#include <iostream>\nint main(){}",
		Build:    &PhaseOpts{Flags: []string{"-fplugin=evil.so"}},
		Tests:    []types.TestCase{{Stdin: "", ExpectedStdout: ""}},
	}
	errResp := ValidateRunRequest(req, defaultServerSettings(), defaultLanguages())
	if errResp == nil {
		t.Fatal("expected error for disallowed build flag")
	}
	if errResp.Error.Code != "disallowed_flag" {
		t.Errorf("expected code 'disallowed_flag', got %q", errResp.Error.Code)
	}
}

func TestValidateRunRequest_PathTraversalFilename(t *testing.T) {
	req := &RunRequest{
		Language:       "py3",
		Source:         "print('hello')",
		SourceFilename: "../../etc/passwd",
		Tests:          []types.TestCase{{Stdin: "", ExpectedStdout: ""}},
	}
	errResp := ValidateRunRequest(req, defaultServerSettings(), defaultLanguages())
	if errResp == nil {
		t.Fatal("expected error for path traversal in filename")
	}
	if errResp.Error.Code != "invalid_filename" {
		t.Errorf("expected code 'invalid_filename', got %q", errResp.Error.Code)
	}
}
