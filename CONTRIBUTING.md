# Contributing To CloudManager

CloudManager is a terminal control plane for cloud operators. Contributions are
welcome, but the bar is practical: keep the core fast, provider-aware, local by
default, and safe under pressure.

## Start Here

Before opening a large PR, check:

- [Issues](https://github.com/vyoogam/cloudmanager/issues)
- [Roadmap](docs/ROADMAP.md)
- [Provider Guide](docs/PROVIDER_GUIDE.md)

For small fixes, open the PR directly. For larger provider, component, storage,
or UI changes, open an issue first so the shape is clear.

## Development Setup

```bash
git clone https://github.com/vyoogam/cloudmanager.git
cd cloudmanager
go test ./...
go run .
```

CloudManager uses native provider CLIs where useful. You do not need every cloud
CLI installed to work on the project, but provider-specific manual testing
requires the relevant tool:

- AWS: `aws`
- GCP: `gcloud`
- Azure: `az`
- DigitalOcean: `doctl`
- Kubernetes: `kubectl`, optional `k9s`

## Branches And PRs

Use focused branches:

```bash
git checkout -b fix/gcp-vm-location
git checkout -b feature/storage-index
git checkout -b docs/pages-refresh
```

Keep PRs scoped. A provider fetcher fix should not also redesign the dashboard.
A docs update should not carry unrelated generated files.

## Validation

Run the narrow tests first, then the full suite:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/providers/gcp ./internal/ui -count=1
GOCACHE=/tmp/go-build-cache go test ./...
git diff --check
```

For UI behavior, add focused tests around the view or helper being changed.
For provider behavior, prefer parser/mapping tests over live cloud calls unless
the change specifically requires live verification.

## Contribution Areas

- New provider resource fetchers
- Provider-aware actions and guardrails
- Local SQLite inventory, FTS, and search
- CloudManager-local tags and annotations
- Manual host access and reachability checks
- Documentation and GitHub Pages
- Release packaging and installability
- Optional components for DNS, queues, WAF, IAM, load balancers, secrets, and
  incident-response views

## Design Rules

- Keep the core small; broad service depth belongs in components.
- Do not mutate cloud resources silently.
- Make provider-specific actions explicit.
- Keep local tags local unless the user chooses provider sync.
- Prefer indexed local search for speed.
- Preserve keyboard-first TUI workflows.
- Log failures clearly enough for operators to act.

## Release Changes

Release changes should update all install paths together:

- `VERSION`
- README pinned `go install` command
- `Formula/cloudmanager.rb`
- `.goreleaser.yaml` when ownership or release targets change
- `.github/workflows/release.yml` when release automation changes

Manual releases can be started from GitHub Actions with **Run workflow** on the
release workflow.

## Code Of Conduct

By participating, you agree to follow the [Code of Conduct](CODE_OF_CONDUCT.md).
