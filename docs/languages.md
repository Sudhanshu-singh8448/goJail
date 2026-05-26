# Language Registry

Languages are configured in `config/languages.yaml`. Adding a language is a YAML edit and a Dockerfile install step. No Go code change required.

## YAML Schema

```yaml
languages:
  - id: <string>                      # unique identifier used in API requests
    name: <string>                    # human-readable name
    source_filename: <string>         # default source filename (e.g. "solution.py")
    artifact_filename: <string>       # default compiled artifact name (optional)
    source_filename_strategy: from_request  # set if filename comes from the request
    artifact_filename_strategy: from_request
    smoke_cmd: [<string>, ...]        # command to verify the toolchain is installed
    build:                            # optional — only for compiled languages
      cmd: <string>                   # compiler binary path
      args: [<string>, ...]           # arguments, with {{source}}, {{artifact}}, {{flags}} placeholders
      limits:
        wall_time_s: <int>
        memory_kb: <int>
        max_processes: <int>
      flag_allowlist: [<string>, ...] # glob patterns for allowed build flags
    run:
      cmd: <string>                   # runtime binary or ./{{artifact}}
      args: [<string>, ...]
      limits:
        wall_time_s: <int>
        memory_kb: <int>
        max_processes: <int>
```

## Template Placeholders

| Placeholder | Replaced With |
|-------------|---------------|
| `{{source}}` | The resolved source filename |
| `{{artifact}}` | The resolved artifact filename |
| `{{flags}}` | Request-supplied build/run flags (filtered through allowlist) |

## Adding a New Language

### Step 1: Add to `config/languages.yaml`

```yaml
  - id: ruby
    name: "Ruby"
    source_filename: solution.rb
    smoke_cmd: ["/usr/bin/ruby", "--version"]
    run:
      cmd: /usr/bin/ruby
      args: ["{{source}}"]
      limits:
        wall_time_s: 9
        memory_kb: 102400
        max_processes: 100
```

### Step 2: Create install script at `scripts/lang_install/ruby.sh`

```bash
#!/bin/bash
set -e
apt-get install -y --no-install-recommends ruby
ruby --version
ruby -e 'puts "Ruby is working"'
```

### Step 3: Add to Dockerfile

```dockerfile
COPY scripts/ /tmp/scripts/
RUN bash /tmp/scripts/lang_install/ruby.sh
```

### Step 4: Rebuild and test

```bash
make build
make run
curl http://localhost:8000/readyz  # should show ruby: ok
```

No Go code change needed. The language is picked up automatically from the YAML at startup.

## Current Languages

| ID | Name | Compiled | Compiler/Runtime |
|----|------|----------|------------------|
| `py3` | Python 3 | no | `/usr/bin/python3` |
| `cpp` | C++ | yes | `/usr/bin/g++` |
| `c` | C | yes | `/usr/bin/gcc` |
| `java` | Java | yes | `/usr/bin/javac` + `/usr/bin/java` |
| `bash` | Bash | no | `/bin/bash` |
| `javascript` | JavaScript (Node.js) | no | `/usr/bin/node` |
| `verilog` | Verilog | yes | `/usr/bin/iverilog` + `/usr/bin/vvp` |
