package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

func TestLoadServerConfig(t *testing.T) {
	yaml := `
server:
  port: 9000
  max_source_bytes: 1024
  max_tests: 10
  jail_dir: /tmp/test_jails
  nsjail_path: /usr/bin/nsjail
`
	path := writeTemp(t, yaml)
	cfg, err := LoadServerConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != 9000 {
		t.Errorf("expected port 9000, got %d", cfg.Server.Port)
	}
	if cfg.Server.MaxSourceBytes != 1024 {
		t.Errorf("expected max_source_bytes 1024, got %d", cfg.Server.MaxSourceBytes)
	}
	if cfg.Server.MaxTests != 10 {
		t.Errorf("expected max_tests 10, got %d", cfg.Server.MaxTests)
	}
	// Check defaults applied for missing fields
	if cfg.Server.MaxConcurrentJobs == 0 {
		t.Error("expected max_concurrent_jobs to be defaulted to NumCPU")
	}
}

func TestLoadServerConfigDefaults(t *testing.T) {
	yaml := `
server: {}
`
	path := writeTemp(t, yaml)
	cfg, err := LoadServerConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != 8000 {
		t.Errorf("expected default port 8000, got %d", cfg.Server.Port)
	}
	if cfg.Server.MaxSourceBytes != 262144 {
		t.Errorf("expected default max_source_bytes 262144, got %d", cfg.Server.MaxSourceBytes)
	}
}

func TestLoadLanguages(t *testing.T) {
	yaml := `
languages:
  - id: py3
    name: "Python 3"
    source_filename: solution.py
    smoke_cmd: ["/usr/bin/python3", "--version"]
    run:
      cmd: /usr/bin/python3
      args: ["{{source}}"]
      limits:
        wall_time_s: 9
        memory_kb: 102400
        max_processes: 100
  - id: cpp
    name: "C++"
    source_filename: solution.cpp
    artifact_filename: solution
    smoke_cmd: ["/usr/bin/g++", "--version"]
    build:
      cmd: /usr/bin/g++
      args: ["{{flags}}", "-o", "{{artifact}}", "{{source}}"]
      limits:
        wall_time_s: 3
        memory_kb: 1048576
        max_processes: 100
      flag_allowlist: ["-O2", "-Wall"]
    run:
      cmd: "./{{artifact}}"
      limits:
        wall_time_s: 3
        memory_kb: 524288
        max_processes: 64
`
	path := writeTemp(t, yaml)
	langs, err := LoadLanguages(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(langs) != 2 {
		t.Fatalf("expected 2 languages, got %d", len(langs))
	}
	if langs[0].ID != "py3" {
		t.Errorf("expected first language id 'py3', got %q", langs[0].ID)
	}
	if langs[1].NeedsCompilation() != true {
		t.Error("expected cpp to need compilation")
	}
	if langs[0].NeedsCompilation() != false {
		t.Error("expected py3 not to need compilation")
	}
}

func TestLoadLanguagesDuplicateID(t *testing.T) {
	yaml := `
languages:
  - id: py3
    name: "Python 3"
    source_filename: solution.py
    run:
      cmd: /usr/bin/python3
      args: ["{{source}}"]
      limits:
        wall_time_s: 9
        memory_kb: 102400
        max_processes: 100
  - id: py3
    name: "Python 3 Again"
    source_filename: solution.py
    run:
      cmd: /usr/bin/python3
      args: ["{{source}}"]
      limits:
        wall_time_s: 9
        memory_kb: 102400
        max_processes: 100
`
	path := writeTemp(t, yaml)
	_, err := LoadLanguages(path)
	if err == nil {
		t.Fatal("expected error for duplicate id")
	}
}

func TestLoadLanguagesNoRunCmd(t *testing.T) {
	yaml := `
languages:
  - id: broken
    name: "Broken"
    source_filename: test.txt
    run:
      cmd: ""
      limits:
        wall_time_s: 1
        memory_kb: 1024
        max_processes: 10
`
	path := writeTemp(t, yaml)
	_, err := LoadLanguages(path)
	if err == nil {
		t.Fatal("expected error for missing run cmd")
	}
}

func TestFindLanguage(t *testing.T) {
	langs := []LanguageConfig{
		{ID: "py3", Name: "Python 3"},
		{ID: "cpp", Name: "C++"},
	}
	found := FindLanguage(langs, "cpp")
	if found == nil {
		t.Fatal("expected to find cpp")
	}
	if found.Name != "C++" {
		t.Errorf("expected name 'C++', got %q", found.Name)
	}
	notFound := FindLanguage(langs, "rust")
	if notFound != nil {
		t.Error("expected nil for unknown language")
	}
}

func TestResolveSourceFilename(t *testing.T) {
	// Fixed filename
	lang := LanguageConfig{ID: "py3", SourceFilename: "solution.py"}
	name, err := lang.ResolveSourceFilename("")
	if err != nil {
		t.Fatal(err)
	}
	if name != "solution.py" {
		t.Errorf("expected 'solution.py', got %q", name)
	}

	// From request
	lang2 := LanguageConfig{ID: "java", SourceFilenameStrategy: "from_request"}
	name, err = lang2.ResolveSourceFilename("Main.java")
	if err != nil {
		t.Fatal(err)
	}
	if name != "Main.java" {
		t.Errorf("expected 'Main.java', got %q", name)
	}

	// From request but no name provided
	_, err = lang2.ResolveSourceFilename("")
	if err == nil {
		t.Error("expected error when from_request but no filename given")
	}
}

func TestLoadServerConfigMissingFile(t *testing.T) {
	_, err := LoadServerConfig(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
