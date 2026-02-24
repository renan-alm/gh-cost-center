# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.0.0] - 2025-07-21

### Changed — Complete Go Rewrite
- **Full rewrite from Python to Go** — the tool is now a `gh` CLI extension (`gh-cost-center`)
- Authentication via `gh auth login` (no more `GITHUB_TOKEN` env var)
- Install with `gh extension install renan-alm/gh-cost-center`
- Plan/Apply workflow: `gh cost-center assign --mode plan|apply`
- Structured logging via `log/slog`
- File-based cost center cache with 24 h TTL (`.cache/cost_centers.json`)
- Cross-compiled release binaries via `gh-extension-precompile`

### Added
- `gh cost-center assign` — PRU, Teams, and Repository modes with `--teams` / `--repo` flags
- `gh cost-center list-users` — list Copilot licence holders
- `gh cost-center config` — show resolved configuration
- `gh cost-center report` — summary report (supports `--teams`)
- `gh cost-center cache` — `--stats`, `--clear`, `--cleanup`
- `gh cost-center version` — print version
- Budget creation for Copilot PRU and Actions (`--create-budgets`)
- Cost center auto-creation (`--create-cost-centers`)
- GHE Data Resident and GHES support via `api_base_url` config
- Graceful shutdown with OS signal handling
- CI workflow: `go build`, `go vet`, `go test -race`, `golangci-lint`
- Release workflow: `gh-extension-precompile`
- Dependabot configuration for Go modules

### Removed
- All Python source code (`src/`, `main.py`, `requirements.txt`)
- Docker artifacts (`Dockerfile`, `docker-compose.yml`)
- Shell automation scripts (`automation/`)
- Python CI/CD workflows (`cost-center-automation.yml`, `cost-center-sync-cached.yml`)
- Legacy documentation (`REMOVED_USERS_FEATURE.md`, `TEAMS_INTEGRATION.md`, `TEAMS_QUICKSTART.md`, `CACHING_IMPLEMENTATION.md`, `BUDGET_IMPROVEMENTS.md`)

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
