# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [2.1.0] - 2026-03-10
## [2.1.1] - 2026-03-23

### Fixed

- support UUID values in manual team mappings (c0e0690)

## [2.1.0] - 2026-03-11

### Added

- Fail fast approach, logs refactoring and add tests (34ad3e0)

### Changed

- Update project version to 2.0.0 (08038f6)

### Fixed

- repair release workflow parsing and clean up CHANGELOG for v2.1.0 (55c8b82)
- resolve cost center names to UUIDs when auto_create is disabled (7af8f73)


### Fixed — Error Propagation & Exit Codes
- **Partial assignment failures now exit 1** — `logAssignmentResults` returns an error when any user assignments fail, so CI/CD pipelines correctly detect incomplete runs
- **`EnsureBudgetsForCostCenter` returns errors** — per-product budget failures are accumulated and propagated to callers instead of being silently logged; `BudgetsAPIUnavailableError` still triggers graceful degradation (returns nil)
- **Scanner errors in confirmation prompts** — `confirmApply()` and repo/custom-prop prompts now check `scanner.Err()` and return I/O failures instead of silently aborting
- **Timestamp save errors propagated** — failure to save the run timestamp now returns an error (exit 1) instead of only logging
- **Version command uses `RunE`** — `cmd/version.go` returns errors on I/O failures (missing VERSION file still falls back to `"dev"`)
- **Removed duplicate SIGPIPE handler** — consolidated from `internal/logging/logger.go` into `main.go` only

### Fixed — Logging Consistency
- **Eliminated triple-logging on assignment failures** — consolidated to one ERROR per cost center (with user list) and one summary line; removed redundant WARN
- **`.env` parse failure promoted to ERROR** — was incorrectly logged as WARN
- **Standardized `mode=plan` log messages** — all plan-mode messages now use `"mode=plan: would <action>"` format
- **Consolidated duplicate deduplication logs** — merged WARN + INFO into a single INFO with all metrics
- **Cache log levels lowered to DEBUG** — `Cache cleared` and `Cleanup complete` now use DEBUG, consistent with other cache operations
- **Added fallback context in teams manager** — WARN log when falling back to cost center name as ID

### Fixed — Cost Center Name-to-UUID Resolution
- **`auto_create: false` now resolves names to UUIDs** — when `auto_create` is disabled (teams and PRU modes), cost center names are resolved to UUIDs via the billing API instead of being passed directly in API URLs. Previously, names with non-ASCII characters were sent as UUIDs, causing 404 errors.
- **Sync aborts on unresolved names** — if any cost center name cannot be resolved, the sync fails fast with an actionable error listing all unresolved names and suggesting to enable `auto_create` or verify names in billing settings
- **UUID validation guards on API calls** — `GetCostCenter()`, `AddUsersToCostCenter()`, and `RemoveUsersFromCostCenter()` now validate that IDs look like UUIDs before making HTTP requests; non-UUID values (including those with non-ASCII characters) are rejected with a descriptive error
- **Actionable 404 error messages** — cost-center-not-found errors now include guidance ("may have been deleted or renamed — verify in billing settings")
- **Config-time warning for non-UUID mapping values** — `resolveTeamsMode()` emits a WARN when `auto_create: false` with `strategy: manual` and mapping values don't look like UUIDs

### Added
- `ResolveCostCenters()` in GitHub client — resolves a list of cost center names to UUIDs without creating them
- `IsCostCenterNotFound()` helper — classifies `APIError` as a 404
- `IsValidCostCenterUUID()` / `ValidateCostCenterID()` — UUID format validation with non-ASCII character detection
- Comprehensive test coverage for error propagation: budget manager, teams, repository, customprop, cmd/assign, cmd/version, and logging packages

## [2.0.0] - 2026-03-05

### Changed — Complete Go Rewrite
- **Full rewrite from Python to Go** — the tool is now a `gh` CLI extension (`gh-cost-center`)
- Multi-source authentication: `--token` flag → `GITHUB_TOKEN` → `GH_TOKEN` → `gh auth token`
- Automatic `.env` file loading (does not override existing environment variables)
- Install with `gh extension install renan-alm/gh-cost-center`
- Plan/Apply workflow: `gh cost-center assign --mode plan|apply`
- Structured logging via `log/slog`
- File-based cost center cache with 24 h TTL (`.cache/cost_centers.json`)
- Cross-compiled release binaries via `gh-extension-precompile`
- Go version updated to 1.25

### Changed — Mode-Centric Configuration Architecture

> **BREAKING:** Configuration format and CLI flags changed. See the [Quick Configuration Guide](QUICK_CONFIG_GUIDE.md) for migration.

- **`cost_center.mode`** is now the single source of truth — supports `"users"`, `"teams"`, `"repos"`, and `"custom-prop"`
- Modes are mutually exclusive; each mode's settings live under its own YAML key (`cost_center.users`, `cost_center.teams`, `cost_center.repos`, `cost_center.custom_prop`)
- `organizations` moved from `teams` to top-level `github.organizations`
- `teams.mode` renamed to `teams.strategy` (`"auto"` / `"manual"`)
- `remove_users_no_longer_in_teams` renamed to `remove_unmatched_users`
- `type: "custom-property"` field removed from custom-prop cost center entries (implicit from mode)
- Confirmation prompts changed from "type apply" to `yes/no`
- Config models fully rewritten (`internal/config/models.go`, `internal/config/config.go`)
- Config tests fully rewritten to cover all 4 modes

### Added
- **Custom-property cost center mode** (`cost_center.mode: "custom-prop"`) — assign repos to cost centers using AND-filter logic on GitHub custom properties
- New `internal/customprop` package — extracted from `internal/repository` into its own package with full test coverage
- `gh cost-center assign` — PRU, Teams, Repos, and Custom-Prop modes (mode selected via YAML config)
- `--token` global flag — pass a GitHub token directly without environment variables
- `.env` file auto-loading — supports `GITHUB_TOKEN`, `GH_TOKEN`, and other env vars
- `gh cost-center list-users` — list Copilot licence holders
- `gh cost-center config` — show resolved configuration
- `gh cost-center report` — summary report
- `gh cost-center cache` — `--stats`, `--clear`, `--cleanup`
- `gh cost-center version` — print version
- Budget creation for Copilot PRU and Actions (`--create-budgets`)
- Cost center auto-creation (`--create-cost-centers`)
- GHE Data Resident and GHES support via `api_base_url` config
- Graceful shutdown with OS signal handling
- Pre-commit hook for linting and formatting checks
- CI workflow: `go build`, `go vet`, `go test -race`, `golangci-lint`
- CI badges in README
- Release workflow: `gh-extension-precompile` (tags `v*`, auto-generates CHANGELOG)
- Dependabot configuration for Go modules
- Quick Configuration Guide (`QUICK_CONFIG_GUIDE.md`) with examples for all 4 modes
- Example config for custom-property mode (`config/config-custom-prop.yaml`)

### Removed
- `--teams` and `--repo` CLI flags on `assign` command (replaced by `cost_center.mode` in YAML)
- `--teams` flag on `report` command (mode is read from config)
- `internal/repository/custom_property_manager.go` (moved to `internal/customprop/`)
- All Python source code (`src/`, `main.py`, `requirements.txt`)
- Docker artifacts (`Dockerfile`, `docker-compose.yml`)
- Shell automation scripts (`automation/`)
- Python CI/CD workflows (`cost-center-automation.yml`, `cost-center-sync-cached.yml`)
- Legacy documentation (`REMOVED_USERS_FEATURE.md`, `TEAMS_INTEGRATION.md`, `TEAMS_QUICKSTART.md`, `CACHING_IMPLEMENTATION.md`, `BUDGET_IMPROVEMENTS.md`)

### Dependencies
- `actions/checkout` bumped from 4 to 6
- `actions/setup-go` bumped from 5 to 6

## [1.0.0] - 2024-09-25

### Added
- Initial release of the cost center automation tool
- GitHub Actions workflow for automated cost center management
- Support for incremental and full processing modes
- Automatic enterprise detection and cost center assignment
- Comprehensive documentation and setup instructions

### Features
- **Automated Cost Center Management**: Creates and assigns cost centers automatically
- **Incremental Processing**: Only processes changes since last run for efficiency
- **Enterprise Detection**: Automatically detects GitHub Enterprise context
- **Flexible Configuration**: Supports both GitHub Actions and local execution modes
- **Comprehensive Logging**: Detailed logging and artifact collection

### Workflows
- `cost-center-automation.yml`: Main automation workflow

### Configuration
- Support for `COST_CENTER_AUTOMATION_TOKEN` secret
- Configurable cron schedules (every 6 hours by default)
- Manual workflow dispatch with mode selection

### Documentation
- Complete setup instructions for GitHub Actions and local execution
- Troubleshooting guide with common issues and solutions
- API reference and configuration options
