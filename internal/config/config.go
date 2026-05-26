// Package config loads and validates the YAML-based server and language configuration.
package config

import (
	"fmt"
	"os"
	"runtime"

	"gopkg.in/yaml.v3"
)

// ServerConfig holds server-level settings loaded from server.yaml.
type ServerConfig struct {
	Server ServerSettings `yaml:"server"`
}

// ServerSettings contains the actual server configuration fields.
type ServerSettings struct {
	Port                  int    `yaml:"port"`
	MaxSourceBytes        int    `yaml:"max_source_bytes"`
	MaxTests              int    `yaml:"max_tests"`
	MaxConcurrentJobs     int    `yaml:"max_concurrent_jobs"`
	MaxStdinBytes         int    `yaml:"max_stdin_bytes"`
	MaxExpectedBytes      int    `yaml:"max_expected_bytes"`
	MaxStdoutCaptureBytes int    `yaml:"max_stdout_capture_bytes"`
	MaxStderrCaptureBytes int    `yaml:"max_stderr_capture_bytes"`
	JailDir               string `yaml:"jail_dir"`
	NsjailPath            string `yaml:"nsjail_path"`
	OrphanSweepMinutes    int    `yaml:"orphan_sweep_minutes"`
}

// Limits represents resource limits for a build or run phase.
type Limits struct {
	WallTimeS    int `yaml:"wall_time_s"    json:"wall_time_s"`
	MemoryKB     int `yaml:"memory_kb"      json:"memory_kb"`
	MaxProcesses int `yaml:"max_processes"  json:"max_processes"`
}

// CommandConfig represents the configuration for a build or run command.
type CommandConfig struct {
	Cmd           string   `yaml:"cmd"`
	Args          []string `yaml:"args"`
	Limits        Limits   `yaml:"limits"`
	FlagAllowlist []string `yaml:"flag_allowlist"`
}

// LanguageConfig represents one language entry in languages.yaml.
type LanguageConfig struct {
	ID                       string         `yaml:"id"`
	Name                     string         `yaml:"name"`
	SourceFilename           string         `yaml:"source_filename"`
	ArtifactFilename         string         `yaml:"artifact_filename"`
	SourceFilenameStrategy   string         `yaml:"source_filename_strategy"`
	ArtifactFilenameStrategy string         `yaml:"artifact_filename_strategy"`
	SmokeCmd                 []string       `yaml:"smoke_cmd"`
	Build                    *CommandConfig `yaml:"build"`
	Run                      CommandConfig  `yaml:"run"`
}

// LanguagesConfig is the top-level wrapper for the languages YAML file.
type LanguagesConfig struct {
	Languages []LanguageConfig `yaml:"languages"`
}

// NeedsCompilation returns true if the language has a build step.
func (l *LanguageConfig) NeedsCompilation() bool {
	return l.Build != nil
}

// ResolveSourceFilename determines the source filename for a request.
// If the strategy is "from_request", the caller must supply the filename.
func (l *LanguageConfig) ResolveSourceFilename(requestFilename string) (string, error) {
	if l.SourceFilenameStrategy == "from_request" {
		if requestFilename == "" {
			return "", fmt.Errorf("language %q requires source_filename in the request", l.ID)
		}
		return requestFilename, nil
	}
	if l.SourceFilename != "" {
		return l.SourceFilename, nil
	}
	return "", fmt.Errorf("language %q has no source filename configured", l.ID)
}

// ResolveArtifactFilename determines the artifact (binary) filename for a request.
func (l *LanguageConfig) ResolveArtifactFilename(requestFilename string) (string, error) {
	if l.ArtifactFilenameStrategy == "from_request" {
		if requestFilename == "" {
			return "", fmt.Errorf("language %q requires artifact_filename in the request", l.ID)
		}
		return requestFilename, nil
	}
	if l.ArtifactFilename != "" {
		return l.ArtifactFilename, nil
	}
	// No artifact needed (interpreted language)
	return "", nil
}

// LoadServerConfig reads and parses the server.yaml configuration file.
func LoadServerConfig(path string) (*ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading server config: %w", err)
	}

	cfg := &ServerConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing server config: %w", err)
	}

	// Apply defaults
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8000
	}
	if cfg.Server.MaxSourceBytes == 0 {
		cfg.Server.MaxSourceBytes = 262144 // 256 KiB
	}
	if cfg.Server.MaxTests == 0 {
		cfg.Server.MaxTests = 50
	}
	if cfg.Server.MaxConcurrentJobs == 0 {
		cfg.Server.MaxConcurrentJobs = runtime.NumCPU()
	}
	if cfg.Server.MaxStdinBytes == 0 {
		cfg.Server.MaxStdinBytes = 1048576
	}
	if cfg.Server.MaxExpectedBytes == 0 {
		cfg.Server.MaxExpectedBytes = 1048576
	}
	if cfg.Server.MaxStdoutCaptureBytes == 0 {
		cfg.Server.MaxStdoutCaptureBytes = 1048576
	}
	if cfg.Server.MaxStderrCaptureBytes == 0 {
		cfg.Server.MaxStderrCaptureBytes = 262144
	}
	if cfg.Server.JailDir == "" {
		cfg.Server.JailDir = "/tmp/goboxd_jails"
	}
	if cfg.Server.NsjailPath == "" {
		cfg.Server.NsjailPath = "/usr/bin/nsjail"
	}
	if cfg.Server.OrphanSweepMinutes == 0 {
		cfg.Server.OrphanSweepMinutes = 5
	}

	return cfg, nil
}

// LoadLanguages reads and parses the languages.yaml configuration file.
func LoadLanguages(path string) ([]LanguageConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading languages config: %w", err)
	}

	cfg := &LanguagesConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing languages config: %w", err)
	}

	if len(cfg.Languages) == 0 {
		return nil, fmt.Errorf("no languages configured")
	}

	// Validate each language has at minimum an id and a run command
	seen := make(map[string]bool)
	for i, lang := range cfg.Languages {
		if lang.ID == "" {
			return nil, fmt.Errorf("language at index %d has no id", i)
		}
		if seen[lang.ID] {
			return nil, fmt.Errorf("duplicate language id: %q", lang.ID)
		}
		seen[lang.ID] = true

		if lang.Run.Cmd == "" {
			return nil, fmt.Errorf("language %q has no run command", lang.ID)
		}
	}

	return cfg.Languages, nil
}

// FindLanguage looks up a language by its ID in the loaded config.
func FindLanguage(langs []LanguageConfig, id string) *LanguageConfig {
	for i := range langs {
		if langs[i].ID == id {
			return &langs[i]
		}
	}
	return nil
}
