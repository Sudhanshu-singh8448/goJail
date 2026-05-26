// Package sandbox manages the lifecycle of sandboxed code execution using nsjail.
package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nsjail-server/gojail/internal/config"
	"github.com/nsjail-server/gojail/internal/types"
)

// Sandbox manages a single code execution request inside an nsjail jail.
type Sandbox struct {
	ID             string
	WorkDir        string
	Language       *config.LanguageConfig
	NsjailPath     string
	ServerCfg      *config.ServerSettings
	SourceFilename string
	ArtifactName   string
	mu             sync.Mutex
	cleaned        bool
}

// NewSandbox creates a new sandbox with a unique working directory.
// Security fix #5: uses os.MkdirTemp for atomically unique directories.
// Security fix #7: caller must defer sandbox.Cleanup().
func NewSandbox(lang *config.LanguageConfig, nsjailPath string, cfg *config.ServerSettings) (*Sandbox, error) {
	// Ensure the jail root directory exists
	if err := os.MkdirAll(cfg.JailDir, 0755); err != nil {
		return nil, fmt.Errorf("creating jail root dir: %w", err)
	}

	// Create unique working directory atomically — no UID collision possible
	workDir, err := os.MkdirTemp(cfg.JailDir, "goboxd_*")
	if err != nil {
		return nil, fmt.Errorf("creating sandbox dir: %w", err)
	}

	return &Sandbox{
		ID:         filepath.Base(workDir),
		WorkDir:    workDir,
		Language:   lang,
		NsjailPath: nsjailPath,
		ServerCfg:  cfg,
	}, nil
}

// WriteSource writes the user's source code into the sandbox directory.
// Security fix #1: validates the filename is a safe single path component.
// Security fix #2: uses os.WriteFile, no shell commands.
func (s *Sandbox) WriteSource(source string, sourceFilename, artifactFilename string) error {
	// Resolve source filename
	resolved, err := s.Language.ResolveSourceFilename(sourceFilename)
	if err != nil {
		return err
	}
	s.SourceFilename = resolved

	// Resolve artifact filename
	artifact, err := s.Language.ResolveArtifactFilename(artifactFilename)
	if err != nil {
		return err
	}
	s.ArtifactName = artifact

	// Verify the resolved path stays within the work directory
	fullPath := filepath.Join(s.WorkDir, s.SourceFilename)
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return fmt.Errorf("resolving source path: %w", err)
	}
	absWorkDir, _ := filepath.Abs(s.WorkDir)
	if !strings.HasPrefix(absPath, absWorkDir+string(filepath.Separator)) {
		return fmt.Errorf("source path escapes sandbox directory")
	}

	// Write the source file
	if err := os.WriteFile(fullPath, []byte(source), 0644); err != nil {
		return fmt.Errorf("writing source file: %w", err)
	}

	return nil
}

// Build compiles the source code if the language requires it.
func (s *Sandbox) Build(requestLimits *types.LimitsOverride, flags []string) (*types.BuildResult, error) {
	if !s.Language.NeedsCompilation() {
		// No build step needed
		return &types.BuildResult{
			Status:     "ok",
			DurationMs: 0,
		}, nil
	}

	// Merge limits: language defaults, then request overrides
	limits := s.Language.Build.Limits
	if requestLimits != nil {
		limits = mergeLimits(limits, requestLimits)
	}

	// Build the command args
	args := s.buildCommandArgs(s.Language.Build, flags)

	// Build nsjail args
	nsjailArgs := buildNsjailArgs(s.NsjailPath, s.WorkDir, limits, true)
	nsjailArgs = append(nsjailArgs, "--")
	nsjailArgs = append(nsjailArgs, args...)

	start := time.Now()
	stdout, stderr, err := runNsjail(nsjailArgs, nil, s.ServerCfg.MaxStdoutCaptureBytes, s.ServerCfg.MaxStderrCaptureBytes)
	durationMs := time.Since(start).Milliseconds()

	result := &types.BuildResult{
		Stdout:     string(stdout),
		Stderr:     string(stderr),
		DurationMs: durationMs,
	}

	if err != nil {
		result.Status = ParseBuildStatus(err, string(stderr))
	} else {
		result.Status = "ok"
	}

	return result, nil
}

// RunTest runs the compiled/interpreted program against a single test case.
func (s *Sandbox) RunTest(test types.TestCase, requestLimits *types.LimitsOverride, flags []string) (*types.TestResult, error) {
	// Merge limits
	limits := s.Language.Run.Limits
	if requestLimits != nil {
		limits = mergeLimits(limits, requestLimits)
	}

	// Build the run command args
	args := s.buildCommandArgs(&s.Language.Run, flags)

	// Build nsjail args
	nsjailArgs := buildNsjailArgs(s.NsjailPath, s.WorkDir, limits, false)
	nsjailArgs = append(nsjailArgs, "--")
	nsjailArgs = append(nsjailArgs, args...)

	// Create stdin from test input
	var stdinReader *strings.Reader
	if test.Stdin != "" {
		stdinReader = strings.NewReader(test.Stdin)
	}

	start := time.Now()
	stdout, stderr, err := runNsjail(nsjailArgs, stdinReader, s.ServerCfg.MaxStdoutCaptureBytes, s.ServerCfg.MaxStderrCaptureBytes)
	durationMs := time.Since(start).Milliseconds()

	result := &types.TestResult{
		Stdout:     string(stdout),
		Stderr:     string(stderr),
		DurationMs: durationMs,
	}

	if err != nil {
		result.Status = ParseTestStatus(err, string(stderr))
	} else {
		// Compare output
		result.Status = CompareOutput(string(stdout), test.ExpectedStdout)
	}

	// Try to read memory peak from nsjail log
	logPath := filepath.Join(s.WorkDir, "nsjail.log")
	if memKB, logErr := parseNsjailLog(logPath); logErr == nil {
		result.MemoryPeakKB = memKB
	}

	return result, nil
}

// Cleanup removes the sandbox's working directory.
// Security fix #7: safe to call multiple times, uses sync.Mutex.
// Security fix #2: uses os.RemoveAll, no shell commands.
func (s *Sandbox) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cleaned {
		return
	}
	s.cleaned = true

	if s.WorkDir != "" && s.WorkDir != "/" {
		os.RemoveAll(s.WorkDir)
	}
}

// buildCommandArgs constructs the command + args for a build or run step.
func (s *Sandbox) buildCommandArgs(cmdCfg *config.CommandConfig, requestFlags []string) []string {
	cmd := s.templateReplace(cmdCfg.Cmd)
	args := []string{cmd}

	for _, arg := range cmdCfg.Args {
		if arg == "{{flags}}" {
			// Insert request flags at this position
			args = append(args, requestFlags...)
			continue
		}
		args = append(args, s.templateReplace(arg))
	}

	return args
}

// templateReplace replaces {{source}}, {{artifact}}, etc. in a string.
func (s *Sandbox) templateReplace(tmpl string) string {
	r := strings.NewReplacer(
		"{{source}}", s.SourceFilename,
		"{{artifact}}", s.ArtifactName,
	)
	return r.Replace(tmpl)
}

// mergeLimits merges request-level limit overrides on top of language defaults.
func mergeLimits(defaults config.Limits, overrides *types.LimitsOverride) config.Limits {
	result := defaults
	if overrides.WallTimeS != nil {
		result.WallTimeS = *overrides.WallTimeS
	}
	if overrides.MemoryKB != nil {
		result.MemoryKB = *overrides.MemoryKB
	}
	if overrides.MaxProcesses != nil {
		result.MaxProcesses = *overrides.MaxProcesses
	}
	return result
}

// SweepOrphanJails removes stale jail directories older than maxAge.
// Called at startup and periodically. Security fix #7.
func SweepOrphanJails(jailDir string, maxAge time.Duration) int {
	entries, err := os.ReadDir(jailDir)
	if err != nil {
		return 0
	}

	swept := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if !strings.HasPrefix(entry.Name(), "goboxd_") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if time.Since(info.ModTime()) > maxAge {
			path := filepath.Join(jailDir, entry.Name())
			if err := os.RemoveAll(path); err == nil {
				swept++
			}
		}
	}
	return swept
}
