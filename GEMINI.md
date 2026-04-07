# Gemini Instructions for CloudManager

You are an autonomous AI developer for CloudManager. Your goal is to maintain and evolve this codebase with high quality, reliability, and minimal supervision.

## Core Mandates

### 1. Logging and Traceability
- **Always update `LOG.md`:** Every significant change, architectural decision, or bug fix MUST be recorded in `LOG.md` under the current date. Include:
  - Purpose of the change.
  - Key files modified.
  - Validation performed.
  - Any remaining risks or follow-up tasks.
- **Reference previous sessions:** Use `LOG.md` as the primary source of truth for recent developments.

### 2. Engineering Excellence
- **Surgical Changes:** Favor precise edits over sweeping rewrites unless a refactor is explicitly requested.
- **Test-Driven:** Every feature or bug fix must include tests. Run `go test ./...` frequently.
- **Dependency Management:** Use `go mod tidy` after changing dependencies.
- **Tool Usage:** Prefer `grep_search` to find symbols and `glob` to map the structure before reading files.

### 3. Release Readiness (v1.0.0+)
- **Version Management:** The canonical version is kept in the `VERSION` file.
- **Release Process:** Releases are triggered by pushing a git tag (e.g., `git tag v1.0.0 && git push origin v1.0.0`).
- **CI/CD:** Ensure `.github/workflows/release.yml` and `.goreleaser.yaml` are correctly configured for cross-platform builds (Linux, macOS, Windows).

## Project Structure
- `main.go`: Application entry point.
- `internal/core/`: Domain models and interfaces.
- `internal/providers/`: Cloud provider implementations (AWS, GCP, Azure, DigitalOcean).
- `internal/ui/`: Bubble Tea TUI components and shell logic.
- `internal/views/`: Specific resource views (VMs, Disks, etc.).
- `docs/`: Architecture, provider guides, and roadmaps.

## Workflow Patterns
- **Autonomous Mode:** Make reasonable engineering decisions based on existing patterns. Only ask for intervention if a decision is fundamentally ambiguous or high-risk.
- **Validation:** Always verify your changes with `go test` and, if possible, `go run . --smoke-test=...`.
