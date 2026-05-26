# goboxd Implementation Tasks

## Phase 1: Cleanup & Foundation
- [x] Remove proto/ directory
- [x] Remove old config files (public_settings.conf, private_settings.conf)
- [x] Remove old internal packages (settings, codemanager, coderunner)
- [x] Update go.mod with new dependencies
- [x] Create config/languages.yaml
- [x] Create config/server.yaml

## Phase 2: Core Packages
- [x] Rewrite internal/config/config.go (YAML loading)
- [x] Create internal/config/config_test.go
- [x] Create internal/api/types.go (request/response structs)
- [x] Create internal/api/validation.go
- [x] Create internal/api/validation_test.go
- [x] Create internal/sandbox/sandbox.go
- [x] Create internal/sandbox/nsjail.go
- [x] Create internal/sandbox/status.go
- [x] Create internal/sandbox/status_test.go
- [x] Create internal/worker/pool.go
- [x] Create internal/stats/stats.go
- [x] Rewrite internal/logger/logger.go

## Phase 3: HTTP Layer
- [x] Create internal/api/handlers.go
- [x] Rewrite cmd/server/main.go

## Phase 4: Docker & Build
- [x] Rewrite Dockerfile
- [x] Update docker-compose.yml
- [x] Rewrite Makefile
- [x] Update entrypoint.sh
- [x] Update scripts/install.sh
- [x] Clean up lang_install scripts

## Phase 5: Documentation
- [x] Rewrite README.md
- [x] Rewrite docs/architecture.md
- [x] Create docs/api.md
- [x] Create docs/languages.md
- [x] Create docs/security.md
- [x] Create docs/benchmarks.md
- [x] Update docs/development.md

## Phase 6: Tests & Load Testing
- [x] Create tests/testdata/ fixtures
- [x] Create tests/integration_test.sh
- [x] Create tests/load_test.sh

## Phase 7: Verification
- [x] go vet clean
- [x] go build succeeds
- [x] Unit tests pass
