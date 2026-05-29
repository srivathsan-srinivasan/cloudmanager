# LOG

## 2026-05-29

### Update

- Removed the runtime context-discovery fake fallback from the UI. If `:discover` finds no real provider contexts, CloudManager now returns an empty context list and warnings instead of inventing AWS/GCP/Azure/DigitalOcean accounts.
- Added `TestFetchContextsDoesNotInventFallbackContexts` to lock the no-fake-context behavior under an empty home and PATH.
- Validation: `GOCACHE=/tmp/go-build-cache go test ./internal/ui -run TestFetchContextsDoesNotInventFallbackContexts -count=1`; `GOCACHE=/tmp/go-build-cache go test ./internal/ui -count=1`; `GOCACHE=/tmp/go-build-cache go test ./...`; `git diff --check -- internal/ui/app.go internal/ui/app_test.go`.

## 2026-05-27

### Update

- Reworked the release branch contract toward `dev/*` work branches and protected `release/v1` as the only stable v1 release line.
- `scripts/release` now refuses to prepare releases outside `dev/*`, updates release pins, and tells the operator to open a PR into `release/v1`.
- The manual release workflow now refuses to tag unless run from `release/v1`.
- GoReleaser formula PRs now target `release/v1`.
- Added `docs/RELEASE.md` with the branch model, patch release flow, branch protection rules, and one-time `release/v1` creation step.
- Fixed first-run profile/discovery persistence by resetting Viper before each config load, preventing stale in-process config state from leaking across homes/config paths.
- Added first-run regressions for manual profile add and discovery import creating `.cloudmanager.json` and selecting the current context.
- Cleaned profile/discovery list descriptions so first-run GCP profiles no longer show empty boilerplate like `auth=-`, `tenant=-`, or `regions=global`; plain GCP native CLI contexts now render as `native CLI`.
- Polished profile onboarding/editing: the footer no longer advertises Azure-specific import as a primary action, the add/edit form is provider-aware, new profiles no longer default to AWS/us-east-1, GCP shows only project basics, AWS shows profile/regions, Azure shows tenant, and hidden auth mode/persistence are preserved when editing.
- Added `scripts/check-credentials` for lightweight provider credential checks, including GCP CLI token, ADC token, active project access, and optional CloudManager smoke tests.
- Validation: `bash -n scripts/release`; `bash -n scripts/install`; `ruby -e 'require "yaml"; YAML.load_file(".github/workflows/release.yml"); YAML.load_file(".goreleaser.yaml"); puts "yaml ok"'`; `git diff --check`; `go run github.com/goreleaser/goreleaser/v2@v2.9.0 check`; `GOCACHE=/tmp/go-build-cache go test ./internal/config ./internal/ui -count=1`; `GOCACHE=/tmp/go-build-cache go test ./...`.

### Update

- Adapted the v1.0.2 release path for protected release branches after GitHub rejected the workflow's direct push to `release/v1.0.0`.
- The manual release workflow now validates already-merged version pins and pushes only the annotated tag; GoReleaser runs only on the tag workflow.
- GoReleaser now opens a Homebrew formula PR from `formula/cloudmanager-{{ .Version }}` instead of pushing formula updates directly to the protected release branch.
- The workflow pins GoReleaser to `v2.9.0` so the existing formula publisher remains valid; newer v2 releases now fail `check` on deprecated `brews`.
- `scripts/release` is now a release-prep helper for PR branches: it updates `VERSION`, README install pins, and `scripts/install`, commits them, and optionally pushes the PR branch.
- Bumped the release-prep pins to `v1.0.2`; left `Formula/cloudmanager.rb` at `v1.0.0` because generated formula checksums must come from GoReleaser after artifacts exist.
- Validation: `bash -n scripts/release`; `bash -n scripts/install`; `ruby -e 'require "yaml"; YAML.load_file(".github/workflows/release.yml"); YAML.load_file(".goreleaser.yaml"); puts "yaml ok"'`; `git diff --check`; `go run github.com/goreleaser/goreleaser/v2@v2.9.0 check`; `GOCACHE=/tmp/go-build-cache go test ./...`.

### User Request Handled

- Prepare the next release path for `v1.0.2` after confirming the installed `v1.0.1` binary does not include VM status colorization.

### Key Code And Release Changes

1. Added a VM viewport regression test proving unselected `running` and `terminated` rows render with status colors when terminal color output is enabled.
2. Fixed release automation so `scripts/release` and the manual GitHub Actions workflow update README curl pins plus `scripts/install`, not only the Go install pin and formula.
3. Updated release docs to mention the installer default is part of release pinning.

### Validation Performed

- `bash -n scripts/release`
- `bash -n scripts/install`
- `ruby -e 'require "yaml"; YAML.load_file(".github/workflows/release.yml"); puts "workflow yaml ok"'`
- `git diff --check`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. Open and merge the PR from `release/v1.0.2-prep`.
2. After merge, run the manual **Release Go Module** workflow with `v1.0.2` from the release branch so the tag and real GoReleaser artifacts/checksums are created.

## 2026-05-23

### Update

- Fixed release workflow GoReleaser action mismatch: `.goreleaser.yaml` is version 2, so the workflow now uses `goreleaser/goreleaser-action@v6` with `version: '~> v2'` instead of v1 `latest`.

### Update

- Replaced the overbuilt GoReleaser config with a v1.0.0-ready config focused on Linux/macOS artifacts, checksums, `.deb`/`.rpm` packages, and generated Homebrew formula updates in the main repo.
- Removed Windows from the release matrix for now because `internal/sysusage` does not cross-compile on Windows.
- Fixed Homebrew generated formula test command to use `cloudmanager --version`.
- Verified the generated Homebrew formula now includes macOS universal plus Linux amd64/arm64 artifact URLs.
- Added `/dist/` to `.gitignore` for local GoReleaser snapshot output.
- Added optional React Web UI to the roadmap for setup, cached inventory browsing, component/plugin management, and install diagnostics.
- Validation: `go mod verify`; `GOCACHE=/tmp/go-build-cache go test ./...`; `go run github.com/goreleaser/goreleaser/v2@latest check`; `go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean --skip=before`.

### Update

- Added `scripts/install` as the curl-friendly installer for CloudManager.
- Installer prefers prebuilt GitHub Release artifacts, verifies `checksums.txt`, installs to Homebrew bin, `/usr/local/bin`, or `~/.local/bin`, and falls back to `go install` when artifacts are unavailable.
- Installer supports `--source`, `--binary`, `--version`, `--bin-dir`, and `--install-go`; Go is only installed automatically when the user explicitly passes `--install-go` and Homebrew is available.
- Reset release pinning to `v1.0.0` in `VERSION`, README examples, and the Homebrew formula.
- Updated the Homebrew formula to set `CGO_ENABLED=0` while still declaring `go` as a build dependency, so Brew installs Go if missing and does not require Clang for this build path.
- Validation: `bash -n scripts/install`; `scripts/install --help`; `bash -n scripts/release`; `ruby -c Formula/cloudmanager.rb`; `GOCACHE=/tmp/go-build-cache go test ./...`; `git diff --check`.

### User Request Handled

- Prepare CloudManager for the moved `vyoogam/cloudmanager` repo, improve public docs, and add a manual GitHub Actions release path.

### Key Code And Docs Changes

1. Updated Go module/import paths, release metadata, Homebrew formula, scripts, docs, and badges to `github.com/vyoogam/cloudmanager`.
2. Expanded README features into a public-facing feature list covering cockpit, inventory/search, provider coverage, access/actions, FinOps/health, and local-first safety.
3. Added README release-download and visitor badges.
4. Rebuilt `CONTRIBUTING.md` with setup, validation, contribution areas, design rules, and release-change expectations.
5. Added manual `workflow_dispatch` release automation: enter `vX.Y.Z`, optionally skip tests, bump release files, commit, tag, push, and run GoReleaser.
6. Added the static GitHub Pages docs page under `docs/index.html` plus `docs/.nojekyll`.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./...`
- `git diff --check`
- `ruby -c Formula/cloudmanager.rb`
- `bash -n scripts/release`
- Local docs preview was previously verified from `docs/`.

### Remaining Risks Or Follow-Up

1. GitHub Pages still needs repository settings pointed at the chosen source branch/path.
2. The manual release workflow will publish only after the pushed workflow is present on GitHub and Actions has write permissions enabled for the repo.

## 2026-05-19

### Update

- Implemented SQLite-backed VM/resource inventory caching behind the existing cache functions.
- Added `resource_inventory` and `resource_summaries` tables to the local SQLite DB at `~/.cloudmanager.db` / `CLOUDMANAGER_DB_PATH`.
- VM index, resource index, and summary cache saves now write SQLite first and keep the existing JSON files as compatibility fallback.
- Cache loads now prefer SQLite; if no SQLite rows exist, they fall back to existing JSON and import on the next save path.
- Resource inventory rows store provider/context/resource type/resource id/name/searchable text/tags/payload/seen-at so FTS and direct SQLite Find can be added next.
- Updated roadmap: access memory and inventory cache are now done; remaining local-index work is direct SQLite query paths, FTS, then JSON fallback removal.
- Validation: `GOCACHE=/tmp/go-build-cache go test ./internal/localdb ./internal/ui -run 'TestResourceRowsRoundTrip|TestSummaryRowsRoundTrip|TestVMIndexCacheRoundTrip|TestVMIndexCacheLoadsFromSQLiteWhenJSONMissing|TestResourceIndexCacheRoundTrip|TestResourceIndexCacheLoadsFromSQLiteWhenJSONMissing|TestResourceIndexCacheLoadWithSummariesOnly' -count=1`; `GOCACHE=/tmp/go-build-cache go test ./...`; `git diff --check -- internal/localdb/access.go internal/localdb/inventory.go internal/localdb/access_test.go internal/ui/vm_index_cache.go internal/ui/app_test.go`.

### User Request Handled

- Fix VM resource location display: GCP VMs were showing context `global` in Find instead of the actual zone, and Azure needed proper VM location handling.

### Key Code And UI Changes

1. Find records now carry a resource-level `Location` field and the Find table shows `Location` instead of context-only `Region`.
2. VM Find now prefers `VM.Zone` for location, with compatibility for old `Region` lookups.
3. Non-VM Find records also set resource locations from their own fields: disk/snapshot zone, database/cluster/network/subnet/firewall/storage region or availability zone.
4. Azure VM CLI and SDK fetchers now preserve Azure `location` into `core.VM.Zone` so the UI can show `eastus`, `centralindia`, etc.
5. AWS VM CLI and SDK fetchers now prefer EC2 Availability Zone over the broader configured region.
6. GCP VM zone normalization was covered with a regression test; provider mapping already preserved the zone, while Find was using the wrong display source.

### Validation Performed

- `git diff --check -- internal/ui/app.go internal/ui/app_test.go internal/providers/azure/client.go internal/providers/azure/client_test.go internal/providers/aws/client.go internal/providers/gcp/client_test.go`
- `GOCACHE=/tmp/go-build-cache go test ./internal/ui ./internal/providers/gcp ./internal/providers/azure ./internal/providers/aws -count=1`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. Existing cached/indexed rows that already contain `global` or missing Azure location need a fresh VM index refresh to pick up provider-side location values.

## 2026-05-18

### User Request Handled

- Fix confusing database counts/statuses, correctly derive GCP Cloud SQL running/stopped state, make the Databases tab clearer when empty, and improve CloudManager logging.

### Key Code And UI Changes

1. GCP Cloud SQL fetchers now derive display status from both `state` and `settings.activationPolicy`: `RUNNABLE+ALWAYS` becomes `RUNNING`, while `RUNNABLE+NEVER` becomes `STOPPED`.
2. Raw `RUNNABLE` is no longer counted as ready if it appears from old cache or incomplete data.
3. Dashboard database summary now exposes the hidden `other` bucket; compact card labels show `ready`, `down`, and `+N` other when needed.
4. Databases tab now renders explicit empty/error states instead of a blank table, including the distinction between selected-context rows and global dashboard/index counts.
5. Database fetches now log `fetch_start`, `fetch_completed`, and `fetch_failed` with provider/account/region/backend/count/status buckets.
6. Database index updates now log aggregate ready/down/other totals.
7. Logging now initializes before CLI subcommands as well as TUI/smoke flows, and `:logs` opens application logs from the command bar.
8. Public IP dashboard card now opens VM Find with `has:public-ip` instead of the unfiltered VM Find fallback.
9. VM Find results now include a visible `Public IP` column so the Public IP dashboard drill-down shows the address directly.
10. Table status text is now colorized after table layout across Find and resource viewports, while pre-styled selected rows are left untouched so the selection bar stays solid.
11. Roadmap now tracks shared sortable columns for Find and every resource viewport.
12. Centralized the status color system in `internal/ui`: `in-use`, `available`, `running`, reachable, and completed states are green; starting/checking states are blue; stopped/stopping/terminated/deallocated/unreachable states are amber; unknown states are subdued.
13. Manual Hosts now include a `Status` column and asynchronously test SSH first, then ping, reporting `ssh-ok`, `ping-ok`, or `unreachable`.
14. Status colors now use exact theme slots instead of broad green/amber buckets, so `running`, `available`, `ready`, `in-use`, `starting`, `stopping`, `stopped`, `terminated`, `deallocated`, `unknown`, reachable, and unreachable states can each be styled distinctly across all table text.
15. Theme configuration now applies beyond the dashboard: table headers, selected rows, status tokens, provider names, regions, IDs, IPs, instance types, booleans, and common column header tokens all use centralized adaptive theme colors that work on light and dark terminals.

### Validation Performed

- `git diff --check -- main.go internal/providers/gcp/databases.go internal/providers/gcp/resources_cli.go internal/providers/gcp/resources_cli_test.go internal/ui/app.go internal/ui/app_test.go internal/views/databases/view.go internal/views/databases/view_test.go README.md`
- `GOCACHE=/tmp/go-build-cache go test ./internal/providers/gcp ./internal/views/databases ./internal/ui -count=1`
- `GOCACHE=/tmp/go-build-cache go test ./...`
- `GOCACHE=/tmp/go-build-cache go test ./internal/ui -run 'TestSelectableDashboardPublicIPsOpensFilteredVMFind|TestSelectableDashboardOpensScopedFind|TestSelectableDashboardResourceCardsOpenFindScopes' -count=1`
- `GOCACHE=/tmp/go-build-cache go test ./internal/ui -count=1`
- `GOCACHE=/tmp/go-build-cache go test ./internal/ui -run 'TestSelectableDashboardPublicIPsOpensFilteredVMFind' -count=1`
- `GOCACHE=/tmp/go-build-cache go test ./internal/ui -run 'TestSelectableDashboardPublicIPsOpensFilteredVMFind|TestOperationalStateColorTones' -count=1`
- `GOCACHE=/tmp/go-build-cache go test ./internal/views/hosts -count=1`
- `GOCACHE=/tmp/go-build-cache go test ./internal/views/vms ./internal/views/disks ./internal/views/snapshots ./internal/views/databases ./internal/views/clusters ./internal/views/hosts ./internal/views/storage ./internal/views/networks ./internal/views/firewalls -count=1`
- `GOCACHE=/tmp/go-build-cache go test ./internal/ui -run 'TestOperationalStateColorTones|TestStatusColorUsesDistinctThemeSlots' -count=1`
- `GOCACHE=/tmp/go-build-cache go test ./internal/config -count=1`
- `GOCACHE=/tmp/go-build-cache go test ./internal/ui -run 'TestOperationalStateColorTones|TestStatusColorUsesDistinctThemeSlots|TestSemanticTableTokenColorsUseThemeSlots' -count=1`

### Remaining Risks Or Follow-Up

1. Dashboard totals are global indexed/cache totals; the Databases tab remains scoped to the selected context by design.
2. Existing cached raw `RUNNABLE` rows may stay in the `other` bucket until `:index-db`, `:index-all`, or a Databases tab refresh updates the cache.

### Update

- Added shared column sorting for previously unsorted viewports: global Find, Databases, Storage, Clusters, Networks, Subnets, Manual Hosts, and nested Firewall Rules. Existing VM, Disk, Snapshot, and Security Group sorting remains intact.
- Sort is opt-in via `S`; default provider/index order is preserved until the user chooses a column. Re-selecting the same column toggles ascending/descending.
- Removed broad semantic token coloring from table rows so names like `aws-prod-gateway`, `gcp-lab`, or `sg-web` are not randomly highlighted inside the Name column. Runtime state tokens still get status colors.
- Changed the active outer shell/sidebar border back to the subtle theme color instead of purple highlight.
- Validation: `git diff --check -- internal/ui/sort_helpers.go internal/ui/sort_helpers_test.go internal/ui/state_colors.go internal/ui/app.go internal/ui/app_test.go internal/views/databases/view.go internal/views/storage/view.go internal/views/clusters/view.go internal/views/networks/view.go internal/views/hosts/view.go internal/views/firewalls/rules_view.go`; `GOCACHE=/tmp/go-build-cache go test ./internal/ui ./internal/views/databases ./internal/views/storage ./internal/views/clusters ./internal/views/networks ./internal/views/hosts ./internal/views/firewalls -count=1`; `GOCACHE=/tmp/go-build-cache go test ./...`.

### Update

- Reworked sorting UX from a picker overlay to header-focused sorting: press `↑` on the first row to focus column headers, use `←/→` to choose a column, and press `Enter` to sort or reverse-sort that column.
- `S` now jumps directly into header focus instead of opening a separate sort menu.
- Header markers show the active header (`▸`) and current direction (`↑`/`↓`) without changing field lookup.
- Applied the header sorting flow to Find, VMs, Disks, Snapshots, Security Groups, Firewall Rules, Databases, Storage, Clusters, Networks/Subnets, and Manual Hosts.
- Validation: `GOCACHE=/tmp/go-build-cache go test ./internal/ui ./internal/views/databases ./internal/views/storage ./internal/views/clusters ./internal/views/networks ./internal/views/hosts ./internal/views/firewalls ./internal/views/vms ./internal/views/disks ./internal/views/snapshots -count=1`; `GOCACHE=/tmp/go-build-cache go test ./...`; `git diff --check`.

### Update

- Fixed the actual sort semantics: VM sorting no longer forces running instances to the top when sorting unrelated columns like Name or Cost.
- VM, Disk, Snapshot, and Security Group sorting now use the shared comparator: Status/State columns get status-aware rank; numeric/currency columns sort numerically; other columns sort alphabetically.
- Updated VM sort tests to prove Cost desc is not overridden by running-first behavior, and State sort still uses status rank.
- Validation: `GOCACHE=/tmp/go-build-cache go test ./internal/ui ./internal/views/vms ./internal/views/disks ./internal/views/snapshots ./internal/views/firewalls -count=1`; `GOCACHE=/tmp/go-build-cache go test ./...`; `git diff --check`.

### Update

- Fixed repeated header-sort `Enter`: sorting no longer exits header focus, so pressing `Enter` repeatedly on the same header toggles asc/desc instead of opening the first resource row.
- `Esc` or `Down` still exits header focus and returns to row navigation.
- Added a database regression test proving the second header `Enter` reverses the sort while header focus stays active.
- Validation: `GOCACHE=/tmp/go-build-cache go test ./internal/ui ./internal/views/databases ./internal/views/storage ./internal/views/clusters ./internal/views/networks ./internal/views/hosts ./internal/views/firewalls ./internal/views/vms ./internal/views/disks ./internal/views/snapshots -count=1`; `GOCACHE=/tmp/go-build-cache go test ./...`; `git diff --check`.

## 2026-05-17

### User Request Handled

- Add and improve an in-app help page for remembered shortcuts.

### Key Code And UI Changes

1. Added a scrollable shell-owned Help view opened with `?`, `F1`, or `:help`.
2. Help covers global navigation, resource-list controls, VM/firewall actions, profile/context commands, find/index commands, tab mapping, and current-view `ShortHelp()` when available.
3. Added a Help-local `/` filter; `Esc` clears the filter first, then closes Help.
4. Updated the footer and README keybindings to expose `?:Help` / `:help`.
5. Added UI tests for opening, rendering, filtering, clearing, and closing help.

### Validation Performed

- `git diff --check -- README.md internal/ui/app.go internal/ui/app_test.go`
- `GOCACHE=/tmp/go-build-cache go test ./internal/ui -count=1`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. Help content is maintained manually; future shortcut additions should update this page and README together.

## 2026-05-15

### User Request Handled

- Add a simple, elegant GitHub Pages introductory/docs page for CloudManager.

### Key Code And UI Changes

1. Added `docs/index.html` as a self-contained static GitHub Pages landing page.
2. The page introduces CloudManager, highlights core workflows, shows install commands, links to repo docs, and keeps the product boundary around small core plus optional components.
3. Added `docs/.nojekyll` so GitHub Pages serves the static docs directory plainly.

### Validation Performed

- `git diff --check -- docs/index.html docs/.nojekyll`
- Served `docs/` locally with `python3 -m http.server 4173`.
- Verified desktop and mobile renders in Playwright.
- Confirmed no current browser console errors or warnings after favicon fix.

### Remaining Risks Or Follow-Up

1. GitHub Pages still needs repo settings pointed at the `docs/` directory on the desired branch.

## 2026-05-14

### User Request Handled

- Check whether CloudManager lists load balancers, public IP endpoints, and domains, and add a practical export for known indexed public endpoints.

### Key Code And UI Changes

1. Confirmed native load balancer, DNS, and static public-IP inventory are still roadmap/component work, not current provider capabilities.
2. Added `:export-public-endpoints` aliases `:export-endpoints`, `:export-ips`, and `:export-domains`.
3. The export writes CSV rows for known indexed VM/host public endpoints and storage URIs, with provider/context/region/resource metadata and last-seen timestamps.
4. Default output path is `~/cloudmanager-public-endpoints.csv`; a path argument can override it.

### Validation Performed

- `gofmt` on touched Go files.
- `GOCACHE=/tmp/go-build-cache go test ./internal/config -count=1`

### Remaining Risks Or Follow-Up

1. `internal/ui` test runs hung in the tool harness with no visible `go` process in `ps`; focused UI export tests were added but not confirmed in this session.
2. Export scope is honest but narrow: it only exports fields CloudManager already indexes. Full cloudlist-like breadth needs LB, DNS, static IP, and/or external asset-inventory ingestion.

## 2026-05-10

### User Request Handled

- Add a firewall action to allow the user's current public IP.

### Key Code And UI Changes

1. Added `Add My IP` to firewall rule actions plus `i` as a table shortcut.
2. The action resolves the current public IP, converts IPv4 to `/32` and IPv6 to `/128`, and creates a new allow rule using the selected firewall rule as the protocol/port/resource template.
3. Firewall mutation guardrails still apply: SDK mode is required and read-only GCP effective policy rows remain blocked.
4. Added `internal/publicip` with small resolver fallbacks and CIDR normalization tests.
5. Added firewall regression tests for successful Add My IP and resolver failure handling.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/publicip ./internal/core ./internal/views/firewalls`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. Runtime IP resolution depends on outbound HTTPS to public IP resolver endpoints.

## 2026-05-09

### User Request Handled

- Automate the Go/Homebrew release flow.

### Key Code And Packaging Changes

1. Added `scripts/release` to cut stable releases from a clean worktree.
2. The release script runs tests, updates `VERSION`, README pinned `go install`, and `Formula/cloudmanager.rb`, commits the release bump, creates an annotated tag, and pushes branch/tag unless `--no-push` is used.
3. GoReleaser now injects `main.Version` and `main.BuildTime` with ldflags, so release artifacts do not report `vdev`.
4. Updated release workflow comments to reflect that the checked-in formula is bumped before tagging.
5. Documented the release command in README.

### Validation Performed

- `bash -n scripts/release`
- `scripts/release --help`
- `git diff --check`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. This keeps the Homebrew formula in the main repository. A separate generated tap such as `srivathsan-srinivasan/homebrew-cloudmanager` can be added later if we want GoReleaser to publish formula/cask updates directly.

## 2026-05-05

### Update

- Fixed Storage navigation after action-menu changes by focusing the rebuilt storage table and preserving cursor position across refreshes.
- Applied the same table focus/cursor preservation to Databases, which used the same thin table pattern.
- Added regression tests proving arrow-down moves selection in Storage and Databases.
- Validation: `GOCACHE=/tmp/go-build-cache go test ./internal/views/storage ./internal/views/databases` and `GOCACHE=/tmp/go-build-cache go test ./...`.

### Update

- Fixed Storage discovery gaps: GCP SDK mode now uses the Google Cloud Storage API with active `gcloud` token fallback, and GCP CLI mode falls back to the SDK path if `gcloud storage buckets list` fails.
- Azure Storage now includes blob container rows in addition to storage account rows by calling `az storage container list --auth-mode login` per storage account.
- Remaining Azure caveat: blob containers require Azure data-plane permission on the account/container; if the user only has management-plane read, account rows can appear while containers are skipped.
- Validation: `GOCACHE=/tmp/go-build-cache go test ./internal/providers/gcp ./internal/providers/azure ./internal/providers` and `GOCACHE=/tmp/go-build-cache go test ./...`.

### Update

- Added Database action menu with Describe, Copy ID, Copy Console URL, and CloudManager tag actions.
- Added Storage action menu with Describe, Copy URI, Copy ID, Copy Console URL, and CloudManager tag actions.
- Added shared clipboard helper for copy actions from non-VM resource views.
- Database view now tracks filtered visible rows before actions, so action selection follows the row the user actually sees.
- Storage remains bucket/account level; object browsing inside buckets is still a follow-up.
- Validation: `GOCACHE=/tmp/go-build-cache go test ./internal/core ./internal/views/databases ./internal/views/storage` and `GOCACHE=/tmp/go-build-cache go test ./...`.

### User Request Handled

- Check for accidentally committed sensitive values and prepare CloudManager for Go/Homebrew installation.

### Key Code And Packaging Changes

1. Changed the Go module path from local `cloudmanager` to `github.com/srivathsan-srinivasan/cloudmanager` so `go install` works from GitHub.
2. Added `Formula/cloudmanager.rb` for Homebrew tap installation.
3. Updated README install instructions for modern Go install and Homebrew tap usage.
4. Updated GoReleaser GitHub owner/repo and release workflow Go setup.
5. Replaced realistic-looking README/test Azure tenant/subscription examples with clearly fake values.

### Validation Performed

- Strict secret-pattern scan for private keys, AWS/GitHub/GCP token shapes, old tenant/domain examples, and realistic Azure IDs.
- `ruby -c Formula/cloudmanager.rb`
- `brew style Formula/cloudmanager.rb`
- `GOCACHE=/tmp/go-build-cache GOBIN=/private/tmp/cloudmanager-install-test go install .`
- `/private/tmp/cloudmanager-install-test/cloudmanager --version`
- `GOCACHE=/tmp/go-build-cache go test ./...`
- `git diff --check`

### Remaining Risks Or Follow-Up

1. Homebrew `brew audit` could not fully validate the formula by path because this Homebrew version disables path audit; audit by name will work after the formula is tapped/published.
2. `v1.0.1` was cut and pushed; `brew install cloudmanager` from the tap completed, but `brew test` hit local Homebrew Ruby/Bundler issues unrelated to the formula.

## 2026-05-04

### Update 4

- Reframed README around CloudManager as a fast, auditable terminal control plane for cloud operations.
- Added the product thesis: speed, firefighting, instantaneous access, traceability, provider-aware actions, and small-core extensibility.
- Added README sections for Core vs Components and community contribution areas.
- Added roadmap component-system direction: `cloudmanager component list/install/enable/disable/update`, component manifest shape, security rules, and practical build order.
- Validation: documentation-only change; no tests run.

### Update 3

- Added a shared `internal/views/tagging` helper for CloudManager-local tag parsing and saving.
- Added `t: Tag` overlays to Disks, Snapshots, and Storage.
- Tag saves now write generic `resource_tags` entries with kind-specific targets and immediately refresh the local rows plus resource indexes.
- Added regression tests for disk, snapshot, storage tagging, and shared tag input parsing.
- Validation: `GOCACHE=/tmp/go-build-cache go test ./internal/config ./internal/views/tagging ./internal/views/disks ./internal/views/snapshots ./internal/views/storage` and `GOCACHE=/tmp/go-build-cache go test ./...`.

### Update 2

- CloudManager-local tags are now modeled as reusable resource tags, scoped by provider/account/region plus `resource_kind`.
- Existing VM tagging now uses the generic tag path; old VM tag config remains compatible.
- Resource indexes now overlay CloudManager tags for clusters, databases, disks, snapshots, networks, subnets, security groups, storage, and VMs, so Find Resources can match tags beyond VMs.
- Validation: `GOCACHE=/tmp/go-build-cache go test ./internal/config ./internal/ui` and `GOCACHE=/tmp/go-build-cache go test ./...`.

### Update

- Footer process memory now reports CloudManager RSS from the OS as `Mem:234MB` style, not Go heap allocation.
- Footer CPU now uses recent process CPU deltas and renders as `CPU:0.3%` style.
- Validation: `GOCACHE=/tmp/go-build-cache go test ./internal/sysusage ./internal/ui`, `GOCACHE=/tmp/go-build-cache go test ./internal/views/vms`, and `GOCACHE=/tmp/go-build-cache go test ./...`.

### User Request Handled

- Start the proposed SQLite build order, add process usage visibility, and capture roadmap direction for inventory tags, IAM access visibility, plugins/agents, and incident-response services.

### Key Code And UI Changes

1. Added `internal/localdb` with a SQLite-backed `access_profiles` table.
2. VM Access now looks up learned SSH profiles and shows learned SSH before generic access methods.
3. Private-key SSH launch records the working provider/context/resource/user/IP/key tuple into SQLite.
4. Added copyable authorized_keys bootstrap command from the private-key panel via `b`; it is explicit and does not mutate a remote host automatically.
5. Footer now shows CloudManager process usage as `CPU:x.x% Mem:yMB`.
6. Roadmap now covers SQLite resource inventory, CloudManager/provider tags, refresh semantics, IAM `Show my access`, plugin/MCP/A2A interfaces, and incident-response service expansion.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/localdb ./internal/sysusage ./internal/views/vms ./internal/ui`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. SQLite currently stores access memory only; resource index tables are next in the proposed build order.
2. The bootstrap command is copyable/manual; marking `default_key_ready` should happen after a confirmed bootstrap execution flow.
3. Footer memory is now process RSS; it is actual resident process memory, not total machine memory percentage.

## 2026-05-04 (Access Picker)

### User Request Handled

- Fix VM Access picker pollution where unrelated SSH config aliases such as GitHub entries appeared for AWS instances, and make private-key SSH use a dedicated key dropdown instead of dumping every key in the action list.
- Fix private-key SSH panel layout so it no longer renders beside or over the VM table.

### Key Code And UI Changes

1. SSH config fallback matching now requires exact VM name, instance ID, public IP, private IP, or HostName matches; loose substring matching was removed.
2. The top-level Access picker now shows one `Private key SSH` option when keys exist, not every key file.
3. Selecting `Private key SSH` opens a compact form with `user> ubuntu`, selected key, selected IP mode, and command preview.
4. Press `k` in the private-key form to open a Bubble Tea key-file dropdown; key files are no longer rendered as access actions.
5. Press `u` to edit the SSH username, `p` to toggle public/private IP, and `Enter` to execute/copy commands like `ssh -i <key> <user>@<ip>`.
6. Key discovery reads `~/.ssh` and `~/sshkeys` by default, plus `CLOUDMANAGER_SSH_KEY_DIRS` when set, and ignores public keys/non-identity files.
7. AWS native access labels now render as `AWS SSM Session Manager` or `AWS EC2 Instance Connect` instead of generic `AWS native access`.
8. Private-key SSH now renders as a centered fixed-width panel with truncated fields and command preview, preventing VM table bleed on wide terminals.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/access ./internal/views/vms`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. The private-key picker defaults to `ubuntu`; a later key manager can persist per-instance username/key mappings.

## 2026-05-04 (Earlier)

### User Request Handled

- Make AWS VM describe show real EC2 detail instead of only the cached table row.

### Key Code And UI Changes

1. AWS `Describe` now formats detailed instance data for both CLI and SDK backends.
2. Describe output includes identity, AMI, lifecycle, launch time / last start, uptime, SSH key pair, IAM profile, VPC/subnet IDs, VPC/subnet CIDRs, IPs/DNS, security groups, network interfaces, root device, volumes, virtualization, source/dest check, and tags.
3. The VM `d` shortcut now calls the provider describe path, matching the action menu, instead of rendering `core.DescribeVM` from cached list data.
4. AWS CLI describe enriches the instance with `describe-vpcs` and `describe-subnets` CIDR lookups when available.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/providers/aws ./internal/views/vms`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. AWS describe now does extra VPC/subnet API calls; failures are shown as warnings while preserving the main instance details.

## 2026-05-03

### User Request Handled

- Smooth first-time context onboarding, avoid GCP project pollution during discovery, add fake Terraform state for ingestion testing, fix small-terminal dashboard card overlap, and make Storage table columns use available terminal width.

### Key Code And UI Changes

1. `:discover` and Profiles / Contexts `r` now open a selectable discovered-context import list instead of auto-importing everything.
2. Discovered contexts default to unchecked; users explicitly select with `Space`, can toggle all with `a`, filter with `/`, and import with `Enter`.
3. Startup discovery no longer persists discovered contexts automatically; it only displays discovered contexts when no managed contexts exist.
4. Added `examples/terraform/fake-cloudmanager.tfstate` with fake AWS, GCP, and Azure resources matching the current Terraform ingestion matcher.
5. Tightened dashboard card width and row calculation so cards wrap cleanly on narrow terminals.
6. Storage tables now use resource-table expanded visible column widths and storage-specific minimum widths, so wider terminals show full values such as `Cloud Storage bucket`.
7. Updated README with selective discovery and fake tfstate usage.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/ui ./internal/iac`
- `GOCACHE=/tmp/go-build-cache go test ./internal/views/storage ./internal/ui`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. The discovery picker still lists all visible projects/subscriptions; it avoids persistence pollution, but provider/project filters could make very large lists faster to triage.

## 2026-05-02 (Update 12)

### User Request Handled

- Add cloud storage listing/indexing as the next resource surface.

### Key Code And UI Changes

1. Added a read-only Storage tab registered as resource view `8`; Hosts moved to `9`.
2. Added normalized `core.StorageBucket` data plus configurable `storage_columns`.
3. Added provider storage fetchers for AWS S3 buckets, GCP Cloud Storage buckets, and Azure storage accounts.
4. Added `CapabilityStorage`, CLI/SDK provider bindings, storage dashboard card, `:find-storage`, `:index-storage`, and storage support in `:index-all`.
5. Persisted storage rows and storage summaries in the local resource index cache.
6. Updated docs and provider authoring guidance for storage support.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/providers ./internal/ui ./internal/views/storage ./internal/config`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. Storage is list/search only; no bucket/container mutation actions were added.
2. AWS storage listing uses global `list-buckets`, so per-bucket region/encryption/versioning remain `unknown` until we add optional deeper enrichment.

## 2026-05-02 (Update 11)

### User Request Handled

- Restore normal table-style selection highlighting in Find Resources views.

### Key Code And UI Changes

1. Find Resources now opens with the result table focused, not the text input.
2. Arrow keys move the selected row immediately, so the selected-row style renders like normal resource views.
3. `/` now focuses the Find filter input.
4. `Esc` first leaves filter-edit mode, then closes Find on the next press.
5. Added regression tests for table-first Find focus and `/` filter focus.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/ui -run 'TestFindOpensWithTableFocused|TestFindSlashFocusesFilterInput|TestFind|TestSelectableDashboard|TestOpenSelectedGlobalVM' -count=1 -v`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. Find uses the same selected-row style as other tables; future theme changes should keep table styles centralized.

## 2026-05-02 (Update 10)

### User Request Handled

- Tighten the non-VM local resource index cache after it still behaved unlike the VM cache.

### Key Code And UI Changes

1. Existing config files now default `resource_index_persistence_enabled` to true when the key is missing.
2. Resource cache loading now counts summary-only cache payloads too, so dashboard counts can survive restart even if a resource type has zero rows.
3. Find Resources now refreshes when cluster, database, or resource-summary index updates arrive.
4. Find metadata now reports the correct indexed totals for disks, snapshots, networks, subnets, firewalls, and all indexed resources.
5. Added regression coverage for existing-config defaults and summary-only resource cache loading.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/config ./internal/ui -run 'TestLoadDefaultsResourceIndexPersistenceForExistingConfig|TestResourceIndexCache' -count=1 -v`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. Resource destination views still perform live provider refresh when opened; cached rows currently power Find/dashboard, not offline table rendering.

## 2026-05-02 (Update 9)

### User Request Handled

- Make Find Resources results drop into the selected resource view with the matching row filtered.

### Key Code And UI Changes

1. Added `SetSearchQuery` support to Disks, Snapshots, Firewalls, and Networks views.
2. Selecting Find results now opens the target tab and applies the selected resource ID/name filter.
3. Subnet Find results now open the Networks view directly in the Subnets pane using a `subnet:<id>` handoff.
4. Added regression tests for search handoff in disks, snapshots, networks, subnets, firewalls, and existing Find flows.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/views/disks ./internal/views/snapshots ./internal/views/networks ./internal/views/firewalls ./internal/ui -run 'TestSetSearchQuery|TestFind|TestSelectableDashboard' -count=1 -v`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. The selected destination view still fetches fresh provider data on open; the cache/index is used for Find, then the view refreshes its live table.

## 2026-05-02 (Update 8)

### User Request Handled

- Persist `index-all` resource indexes locally like the VM index, with a 24 hour default TTL.

### Key Code And UI Changes

1. Added a separate resource index cache at `~/.cloudmanager-resource-index.json`.
2. Persisted indexed clusters, databases, disks, snapshots, networks, subnets, firewalls, and resource summary counts.
3. Added config keys:
   - `resource_index_persistence_enabled`
   - `resource_index_cache_ttl_hours`
4. Added Settings controls for resource-index persistence and TTL.
5. Resource indexes load on app startup and immediately feed dashboard counts and scoped Find Resources.
6. Updated README config example.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/ui -run 'TestResourceIndexCacheRoundTrip|TestVMIndexCacheRoundTrip|TestSelectableDashboard' -count=1 -v`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. Resource cache is still JSON. SQLite + FTS remains the roadmap upgrade for very large estates.
2. VM index cache remains in its existing separate file for compatibility.

## 2026-05-02 (Update 7)

### User Request Handled

- Add SQLite and external-tool integration direction to the roadmap without turning CloudManager into a bloated all-in-one platform.

### Key Code And UI Changes

1. Updated `docs/ROADMAP.md` with a vNext lane for lean integrations.
2. Captured SQLite + FTS as a later local-index upgrade when JSON cache/search latency becomes visible.
3. Captured optional adapter direction for Steampipe, Cloudlist, CloudQuery, and Prowler.
4. Kept the product boundary explicit: external engines feed Find Resources and dashboard context; CloudManager remains the operator cockpit.

### Validation Performed

- Documentation-only change; no test run needed.

### Remaining Risks Or Follow-Up

1. The actual adapter protocol and storage schema still need design before implementation.

## 2026-05-02 (Update 6)

### User Request Handled

- Wire Home dashboard resource cards to scoped Find views instead of only showing counts.

### Key Code And UI Changes

1. Added row-level Find indexes for indexed disks, snapshots, networks, subnets, and security groups.
2. Added scoped commands:
   - `:find-disks`
   - `:find-snapshots`
   - `:find-networks`
   - `:find-subnets`
   - `:find-firewalls`
3. Dashboard cards for disks, snapshots, networks, subnets, and security groups now open the matching Find scope.
4. `index-all` now stores the fetched resource rows for these Find scopes, not just summary counts.
5. Network indexing now keeps network rows even if subnet listing fails, while logging the subnet warning.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/ui -run 'TestSelectableDashboard' -count=1 -v`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. Selecting a Find result opens the target tab, but disk/snapshot/network/firewall destination views still need table-local search handoff for perfect landing/filtering parity with DBs and clusters.

## 2026-05-02 (Update 5)

### User Request Handled

- Fix Home dashboard arrow navigation still not moving between cards.

### Key Code And UI Changes

1. Fixed the real Home key-routing condition.
   - Home can render while an initial tab view is still present in `viewStack`.
   - Dashboard key handling now checks the actual Home state (`activeCtx.Provider == ""`) instead of requiring an empty `viewStack`.

2. Preserved view input behavior.
   - View-level input handling is skipped only while Home is active.
   - Context filter Enter still activates matching contexts.

3. Added regression coverage.
   - New UI test verifies right arrow moves the dashboard cursor even when Home is visible with an initial view stack.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/ui -run 'TestHomeDashboardArrowKeys|TestSelectableDashboard' -count=1 -v`
- `GOCACHE=/tmp/go-build-cache go test ./...`

## 2026-05-02 (Update 4)

### User Request Handled

- Fix Home dashboard navigation, show the new dashboard boxes, and add an index-all option.

### Key Code And UI Changes

1. Fixed Home dashboard keyboard focus.
   - New app sessions now focus the Home dashboard instead of the sidebar.
   - Arrow keys and `h/j/k/l` move across dashboard cards immediately.
   - Context filter Enter still activates matching context rows.

2. Added dashboard summary boxes.
   - Disks indexed.
   - Snapshots indexed.
   - Networks seen.
   - Subnets seen.
   - Security groups seen.
   - Existing legacy default dashboard configs are upgraded to include the new default boxes.

3. Added `index-all`.
   - New command aliases: `:index-all`, `:refresh-all`, `:reindex-all`.
   - New Settings action: `Index all resources now`.
   - Index-all refreshes supported VM, DB, cluster, disk, snapshot, network, and firewall summaries across contexts.

4. Kept counts honest.
   - Disk/snapshot counts come from explicit index-all summary fetches.
   - Network/subnet/firewall counts prefer provider summaries when indexed and fall back to VM-derived metadata otherwise.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/config ./internal/ui`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. Disks/snapshots are dashboard summaries only; Find scopes for disks/snapshots still need app-level row indexes.
2. Index-all can be API-heavy across many contexts, so it is command/settings-triggered rather than automatic startup behavior.

## 2026-05-02 (Update 3)

### User Request Handled

- Avoid polluting local kubeconfig when jumping from a cluster to k9s.

### Key Code And UI Changes

1. Added target-aware kube context resolution.
   - `core.EnsureKubeContextForTarget` now checks existing kubeconfig contexts before running provider credential fetch commands.
   - Matching prefers exact expected context, exact cluster name, normalized equivalents, then high-confidence fuzzy matches using cluster name plus account/location/id metadata.

2. Updated Kubernetes provider launchers.
   - AWS, GCP, Azure, and DigitalOcean k9s launch paths now pass cluster metadata into the resolver.
   - Provider `get-credentials` / `update-kubeconfig` commands only run when no existing context matches.

3. Added regression coverage.
   - Existing exact context behavior remains covered.
   - New test verifies a renamed local context can be reused without running the fetch command.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/core ./internal/providers/aws ./internal/providers/gcp ./internal/providers/azure ./internal/providers/digitalocean ./internal/views/clusters`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. Fuzzy matching is conservative. Ambiguous or weak matches intentionally fall back to provider credential fetch.
2. A future UX pass can show which kube context was selected before launching k9s.

## 2026-05-02 (Update 2)

### User Request Handled

- Make the Home dashboard selectable and more useful as a command surface.

### Key Code And UI Changes

1. Added selectable dashboard widgets.
   - Arrow keys / `h j k l` move between Home dashboard cards.
   - `Enter` opens the selected card's operational destination.
   - VM cards open Find VMs, DB cards open Find Databases, Kubernetes opens Find Kubernetes, Terraform opens Find All with `iac:terraform`, and manual hosts opens Find Hosts.
   - Network/subnet cards open Networks; security group card opens Firewalls.

2. Added more dashboard widgets.
   - Networks seen.
   - Subnets seen.
   - Security groups seen.
   - Terraform managed.
   - Manual hosts.

3. Kept summary counts honest.
   - Network/subnet/security-group counts are derived from indexed VM metadata, not claimed as full provider inventory.
   - Disk counts were not added because there is no app-level disk index yet.

4. Updated README dashboard widget example.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/config ./internal/ui`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. True disk/network/firewall global summaries need app-level indexes for those resource types.
2. A future pass can add Find scopes for networks, subnets, and firewalls once those indexes exist.

## 2026-05-02

### User Request Handled

- Add Terraform state ingestion for read-only cross-reference of IaC-managed cloud resources.

### Key Code And UI Changes

1. Added local Terraform state indexing.
   - New `internal/iac` package parses configured `.tfstate` files.
   - Parsed state is cached by path, file size, and modification time to avoid repeated startup work.
   - Matching supports VMs, databases, and Kubernetes clusters using provider, ID, name, and VM IP signals.

2. Added CloudManager config support.
   - New `terraform_state_paths` config key accepts local state file paths.
   - Paths are normalized without lowercasing so case-sensitive filesystems remain safe.

3. Wired IaC labels into indexed resources.
   - Matching resources get display labels like `iac:terraform` and `tf:<terraform-address>`.
   - VM, database, and cluster indexes now carry these labels into Find Resources.
   - The feature is read-only and does not run Terraform or mutate cloud resources.

4. Updated README.
   - Added `terraform_state_paths` example and a short safety note.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/iac ./internal/config ./internal/ui ./internal/views/vms ./internal/views/databases ./internal/views/clusters`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. Only local state files are supported. Remote backend pull/import UX is future work.
2. Matching is intentionally best-effort metadata enrichment, not authoritative ownership proof.
3. No TUI file picker yet for adding Terraform state paths.

## 2026-05-01 (Update 4)

### User Request Handled

- Clarify product naming: the feature is `Find Resources`; `Find VMs` is only one scope.

### Key Code And UI Changes

1. Updated UI copy.
   - Default find input placeholder is now `find resources`.
   - Find picker footer uses `Find Resources`.
   - Settings status says `Find resources`.

2. Preserved scoped labels.
   - Scope panels still render `Find VMs`, `Find Databases`, `Find Kubernetes`, etc.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/ui`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. `global_search_enabled` remains the config key for backwards compatibility; user-facing wording now says Find Resources.

## 2026-05-01 (Update 3)

### User Request Handled

- Replace one noisy global search mental model with scoped Find Resources entry points.

### Key Code And UI Changes

1. Added scoped Find Resources.
   - `g` now opens a Find scope picker instead of jumping straight into VM search.
   - Added scopes for VMs, databases, Kubernetes clusters, manual hosts, and all indexed resources.

2. Added command aliases.
   - `:find-vms`
   - `:find-dbs`
   - `:find-k8s`
   - `:find-hosts`
   - `:find-all`

3. Reworked the existing VM global search table into the first scoped resource finder.
   - The panel title now reflects the active scope, e.g. `Find VMs` or `Find Databases`.
   - Results use a generic row shape: type, name, ID, provider, context, region, status, match.
   - Existing VM search behavior is preserved as the VM scope, not as the whole feature name.

4. Wired DB and Kubernetes indexes into Find.
   - Database find searches the app-level database index and opens the Databases tab.
   - Kubernetes find searches the app-level cluster index and opens the Clusters tab.
   - DB and Cluster views now accept `SetSearchQuery` so selected find results filter the destination view.

5. Updated README keybindings.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/ui ./internal/views/databases ./internal/views/clusters`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. Network, subnet, and firewall find scopes are not wired yet because they need app-level indexes like VMs/DBs/clusters.
2. `find-all` intentionally searches indexed resources only; stale or missing indexes still require opening the relevant tab or running refresh/index commands.

## 2026-05-01 (Update 2)

### User Request Handled

- Add the next practical step: TUI Azure subscription picker for profile/context import.

### Key Code And UI Changes

1. Added an Azure subscription picker.
   - Available from `Profiles / Contexts` with `z`.
   - Also available from Settings as `Import Azure subscriptions`.
   - Command palette aliases: `:azure-profiles`, `:azure-contexts`, `:azure-subscriptions`, `:import-azure`.

2. Added multi-select import behavior.
   - Runs Azure context discovery through `az account list`.
   - New subscriptions are selected by default.
   - Existing subscriptions are marked `saved` and left unselected.
   - `Space` toggles one subscription, `a` toggles all, `Enter` imports selected subscriptions.

3. Added Azure-safe upsert behavior.
   - Upserts by Azure subscription ID.
   - Preserves an existing CloudManager-friendly `context_name`.
   - Refreshes subscription display name and tenant from Azure.
   - Does not call `az account set`.

4. Updated README with the picker flow.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/ui`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. The picker does not yet provide inline renaming during import. Users can edit friendly names after import in `Profiles / Contexts`.
2. A future pass should add equivalent pickers for AWS/GCP account/project discovery where useful.

## 2026-05-01

### User Request Handled

- Fix `go run .` startup panic: `index out of range [6] with length 6` from Bubble Tea table resize.

### Key Code And UI Changes

1. Fixed global search table resizing.
   - `resizeGlobalSearch()` now clears existing table rows before changing visible columns.
   - This avoids Charm table rendering old 8-column rows against a narrower responsive column set during startup/window resize.

2. Added regression coverage.
   - New UI test verifies resizing global search with existing rows does not panic.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/ui`
- `GOCACHE=/tmp/go-build-cache go test ./...`
- `GOCACHE=/tmp/go-build-cache go run .` started the TUI without the table panic; quit with `q`.

### Remaining Risks Or Follow-Up

1. Sandbox blocked writing `/Users/ranger/.cloudmanager.log` during the interactive check, but the TUI still started and the reported panic did not recur.

## 2026-04-29 (Update 18)

### User Request Handled

- Make profile/context management primarily TUI-driven instead of command-driven.

### Key Code And UI Changes

1. Upgraded the managed context screen into `Profiles / Contexts`.
   - Shows the current profile/context with a `*` marker.
   - `a` adds, `e`/`Enter` edits, `u` sets current, `l` logs in, `d` removes, `r` discovers, and `b` backs up config.

2. Added selected-profile login from the TUI.
   - Azure runs `az login --use-device-code` and includes `--tenant` when configured.
   - AWS runs `aws sso login` and includes `--profile` when configured.
   - GCP runs `gcloud auth login --no-browser`.
   - DigitalOcean runs `doctl auth init`.

3. Added safer delete behavior.
   - Delete now requires `y` confirmation.
   - Removing the current context clears `current_context` and the active context.

4. Added command-palette aliases.
   - `:profiles`, `:profile`, `:contexts`, `:creds`, and `:credentials` open the TUI profile manager.
   - `:add-profile` and `:add-context` open the add form.

5. Updated README to document the TUI-first flow.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/ui ./internal/config .`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. The selected-profile login flow still delegates to provider CLIs. A CloudManager-only credential broker/JIT vault remains future work.
2. A richer Azure subscription picker after `az account list` would improve add-profile ergonomics further.

## 2026-04-29 (Update 17)

### User Request Handled

- Implement an AWS-profile-style Azure profile/context system for the CLI/TUI.

### Key Code And UI Changes

1. Added first-class CloudManager context names.
   - Managed contexts now persist `context_name` and `tenant`.
   - `current_context` tracks the selected profile/context in `~/.cloudmanager.json`.
   - Context display, filtering, auth refs, and cache identity now understand human-friendly names.

2. Added profile/context CLI commands.
   - `cloudmanager profile list`
   - `cloudmanager profile add <name> --provider azure --tenant <tenant> --subscription-id <id> --subscription-name <name>`
   - `cloudmanager profile use <name>`
   - `cloudmanager profile current`
   - `cloudmanager login <name>`
   - `cloudmanager context ...` is an alias for `profile ...`.

3. Mapped provider discovery into the context model.
   - AWS discovered profiles keep the AWS profile name as the CloudManager context.
   - Azure discovered subscriptions get sanitized names from subscription display names, tenant IDs are retained, and subscription IDs remain the execution target.

4. Wired current context into TUI startup.
   - If `current_context` is set, the context list selects it and makes it active on launch.
   - Context filtering now matches context name and Azure tenant.

5. Expanded managed context editing.
   - The TUI context form now captures context name and tenant alongside provider, account, auth mode, persistence, credential ref, and regions.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test . ./internal/config ./internal/providers/aws ./internal/providers/azure ./internal/ui`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. Azure resource command execution should continue to pass subscription IDs explicitly. This update establishes the context resolver and CLI UX; provider command paths should be audited next for any hidden `az account set` assumptions.
2. The profile store intentionally reuses `~/.cloudmanager.json` instead of adding a parallel `~/.config/<tool>/profiles.yaml`.

## 2026-04-29 (Update 16)

### User Request Handled

- Start the practical secure-auth build order: auth modes, persistence policy, add-provider entry point, and runtime-only session scaffolding.

### Key Code And UI Changes

1. Added explicit auth metadata to managed contexts.
   - `auth_mode`: `native-cli`, `jit-session`, `awsume`, `vault`, `manual`
   - `credential_persistence`: `memory`, `keychain`, `native-cli`, `vault`, `none`
   - Defaults preserve compatibility: native CLI contexts default to `native-cli` persistence.
   - JIT/awsume contexts default to `memory`.

2. Propagated auth metadata into `core.CloudContext`.
   - `AuthMode`
   - `CredentialScope`
   - Context filtering now includes these fields.

3. Added runtime-only auth session scaffolding in `internal/auth`.
   - In-memory session store.
   - Expiry-aware retrieval.
   - Child-process environment injection helper.
   - No persistence and no parent-shell export.

4. Added an Add Provider entry point.
   - `:add-provider`, `:provider`, and `:providers` open a managed context form.
   - The form now captures provider, account, display name, auth mode, persistence policy, credential ref, and regions.
   - Settings includes `Add provider`.

5. Updated README with `:add-provider` and auth metadata examples.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/auth ./internal/config ./internal/ui`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. Providers do not yet consume `internal/auth.Session`; native CLI mode remains the working execution path.
2. Next implementation slice should wire AWS awsume/JIT session resolution into provider command/env execution, then repeat for Azure and GCP.

## 2026-04-29 (Update 15)

### User Request Handled

- Make GCP CLI login terminal-safe for CloudManager's wrapped login flow.

### Key Code And UI Changes

1. Updated provider login items in `internal/ui/app.go`.
   - GCP user login now runs `gcloud auth login --no-browser`.
   - ADC login remains a separate explicit option.

2. Added regression coverage in `internal/ui/app_test.go`.
   - Verifies GCP user login keeps `--no-browser`.

3. Updated README GCP prerequisite text.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/ui`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. Native CLI login flows still persist credentials in each provider CLI's normal location. A CloudManager-only JIT credential broker is a separate architecture from wrapped CLI auth.

## 2026-04-29 (Update 14)

### User Request Handled

- Add Bubble Tea wrapped interactive login flows for provider CLIs, with Azure login support for multiple subscriptions.

### Key Code And UI Changes

1. Added a provider login picker in `internal/ui/app.go`.
   - New command aliases: `:login`, `:auth`, `:provider-login`, `:cli-login`.
   - Settings includes `Provider CLI login`.
   - Managed contexts view includes `l:Login`.

2. Added wrapped native CLI login commands.
   - Azure: `az login --use-device-code`
   - GCP: `gcloud auth login`
   - GCP ADC: `gcloud auth application-default login`
   - AWS SSO setup: `aws configure sso`
   - AWS access key setup: `aws configure`
   - AWS SSO login: `aws sso login`
   - DigitalOcean: `doctl auth init`

3. Login completion now refreshes context discovery.
   - After a successful login command exits, CloudManager runs provider discovery.
   - For Azure, `az account list` discovery will import all visible subscriptions.
   - Missing CLIs show as unavailable instead of failing only after selection.

4. Updated README with `:login` and DigitalOcean CLI prerequisites.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/ui`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. AWS SSO login without an active default profile may still require `aws configure sso` first; both paths are exposed.
2. Provider-specific subscription/account selection after login is still handled by discovery and managed-context editing, not a dedicated post-login wizard.

## 2026-04-29 (Update 13)

### User Request Handled

- Fix access-command copying so it copies only the selected command, and make startup faster by lazying cloud CLI discovery.

### Key Code And UI Changes

1. Tightened VM access copy behavior.
   - `c` in the access picker now copies only the selected method's `CopyText`.
   - It also opens a borderless fallback detail pane containing only that command, so visual copy no longer includes picker borders.
   - Added an injectable clipboard writer and regression coverage for exact command copy.

2. Made context discovery explicit for fast startup.
   - Added config key `discover_contexts_on_start`, default `false`.
   - Startup now loads managed contexts and local index cache without scanning provider CLIs unless the option is enabled.
   - Added `:discover` / `:discover-contexts` / `:scan-contexts` to run cloud CLI context discovery on demand.
   - Settings now includes `Discover contexts on startup`.

3. Kept the access resolver lighter.
   - `internal/access` no longer imports the provider registry; provider-native commands are supplied by the VM view.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/ui ./internal/views/vms ./internal/config ./internal/access`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. Go compile/link time is still affected by the broad provider SDK imports in the main binary. Runtime startup is now lazier; compile-time slimming would need a larger provider-package split or build tags.

## 2026-04-29 (Update 12)

### User Request Handled

- Implement the first access resolver flow for easier VM access instead of directly attempting a single provider SSH command.

### Key Code And UI Changes

1. Added a reusable access model and resolver.
   - New `internal/core/access.go` defines `AccessMethod`.
   - New `internal/access/` resolves provider-native SSH, matching `~/.ssh/config` entries, direct SSH by public/private IP, and a remediation guide fallback.
   - `CLOUDMANAGER_SSH_CONFIG` can point the resolver at a custom SSH config file.

2. Updated VM SSH UX.
   - Pressing `s` or choosing `SSH` now opens an access-method picker.
   - `Enter` runs the selected method.
   - `c` copies the selected command.
   - If a connection fails, the view returns to the picker so another method can be tried.

3. Added tests for SSH config matching and the access picker.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/access ./internal/views/vms`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. Manual hosts, SSH key scanning, per-instance key mapping, and authorized_keys/provider-metadata remediation are still the next layer.
2. The resolver currently parses simple OpenSSH config entries; complex `Include`, `Match`, and wildcard expansion are intentionally not implemented yet.

## 2026-04-28 (Update 11)

### User Request Handled

- Improve the home dashboard visual design and add selectable dashboard themes with heavier, btop-style separators.

### Key Code And UI Changes

1. Reworked the home dashboard renderer in `internal/ui/app.go`.
   - Replaced the noisy bordered card grid with compact metric blocks.
   - Switched the dashboard divider to a heavy rule.
   - Added a cleaner inventory summary with accent bars and a borderless command row.

2. Added dashboard theme selection.
   - New config key: `dashboard_theme`.
   - Supported themes: `btop`, `neon`, `classic`, `mono`.
   - Settings now includes `Dashboard theme` and cycles through these choices.

3. Updated README config example with `dashboard_theme`.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/config ./internal/ui`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. The global `theme` color object still controls the app shell colors; `dashboard_theme` currently controls only the home dashboard.

## 2026-04-28 (Update 10)

### User Request Handled

- Make copyable VM commands and multi-line remediation output easier to copy without terminal border characters.

### Key Code And UI Changes

1. Made the VM detail/remediation pane copy-first in `internal/views/vms/view.go`.
   - Removed the rounded border from the VM detail viewport.
   - Detail, cost, FinOps, action-failure, and SSH remediation output is now stored as copyable text.
   - `c` copies the current VM detail/remediation output to the system clipboard.
   - Clipboard support checks `pbcopy`, `wl-copy`, `xclip`, then `xsel`.

2. Added app-level status updates for child views.
   - `ui.StatusUpdateMsg` lets VM detail panes surface hints like `c copy, Esc close` in the main footer.

3. Updated README keybindings.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/views/vms`
- `GOCACHE=/tmp/go-build-cache go test ./internal/ui`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. Other resource detail panes still use their existing bordered viewport styles. VM command/remediation output was fixed first because it contains the IAP, SSM, SSH, and cost commands users copy most often.

## 2026-04-28 (Update 9)

### User Request Handled

- Improve the home dashboard layout when the context sidebar is hidden, and add a database indexing option.

### Key Code And UI Changes

1. Improved home dashboard layout in `internal/ui/app.go`.
   - Home content is now centered in a constrained canvas when extra width is available.
   - Dashboard cards adapt to available width and can render up to four cards per row.
   - Summary details are grouped in a bordered section.
   - Dashboard actions render as compact command chips instead of a sparse vertical help list.

2. Added database indexing controls.
   - Settings now includes `Index databases`.
   - `prefetch_resources` can include `databases`.
   - `:index-db`, `:index-dbs`, and `:index-databases` refresh the database index.
   - `:summary` refreshes databases only when database indexing is enabled; clusters still refresh for Kubernetes summary.

3. Updated README.
   - Added `:index-db`.
   - Config example now includes `databases` in `prefetch_resources`.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/ui`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. Database indexing is in-memory only today. Persisting database indexes like the VM index is a logical next step.
2. A visual browser-style screenshot is not available for the Bubble Tea TUI in this environment; validation covered rendering width behavior through tests.

## 2026-04-28 (Update 8)

### User Request Handled

- Add user-configurable dashboard widgets.

### Key Code And UI Changes

1. Added dashboard widget configuration in `internal/config/config.go`.
   - New config key: `dashboard_widgets`.
   - Default widgets:
     - `contexts`
     - `indexed_vms`
     - `running_vms`
     - `stopped_vms`
     - `public_ips`
     - `backend`
     - `databases`
     - `kubernetes`
     - `db_contexts`
   - Supported optional widgets also include `vm_providers`, `vm_index_age`, and `k8s_hidden`.
   - Unknown or duplicate widget IDs are ignored; an empty/invalid list falls back to defaults.

2. Reworked the home dashboard renderer in `internal/ui/app.go`.
   - Dashboard cards now render in the exact order from `dashboard_widgets`.
   - Cards auto-wrap three per row.
   - Widgets read only from local indexes/caches; dashboard rendering does not call cloud APIs.

3. Updated README config example with `dashboard_widgets`.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/config ./internal/ui`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. This is config-file driven. A later UI pass can add a dashboard editor screen for toggling/reordering widgets without editing JSON.

## 2026-04-28 (Update 7)

### User Request Handled

- Make the dashboard less confusing and add high-level database/Kubernetes summary cards.

### Key Code And UI Changes

1. Updated the home dashboard in `internal/ui/app.go`.
   - Reworded the VM ratio line from `Running mix` to `VMs running`.
   - Reworded provider totals to `VMs by provider`.
   - Added database summary card: total DBs plus running/stopped counts.
   - Added Kubernetes summary card: `clusters:pools:nodes`.

2. Added local app-level indexes for clusters and databases.
   - `ClusterIndexUpdateMsg` and `DatabaseIndexUpdateMsg` update dashboard summaries from real fetched data.
   - Cluster and database views now publish index updates after successful tab fetches.
   - Added `:summary`, `:refresh-dashboard`, and `:dashboard-refresh` to fetch database and cluster summaries across known contexts.

3. Improved Kubernetes node metadata parsing in `internal/core/vm.go`.
   - Extracts cluster names and nodepool names from common EKS, GKE, AKS, and Karpenter labels/tags.

4. Updated README with the new `:summary` command.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/core ./internal/ui ./internal/views/clusters ./internal/views/databases`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. Database and cluster summaries are in-memory for now. They populate after opening those tabs or running `:summary`; they are not persisted to disk yet like the VM index.
2. Kubernetes pool inference depends on provider tags/labels. More markers can be added from real account data.

## 2026-04-28 (Update 6)

### User Request Handled

- Fix global search result activation so selecting a VM reliably drops into the VM resource view.

### Key Code And UI Changes

1. Hardened global search activation in `internal/ui/app.go`.
   - Selecting a global VM search result now forces the VM root tab/view instead of relying on the existing view stack.
   - This prevents stale drill-down views from receiving the VM initialization/search query.
   - The selected VM context is still applied and the VM table is filtered by instance ID.

2. Added regression coverage in `internal/ui/app_test.go`.
   - Verifies global search initializes the VM tab and filter.
   - Verifies global search truncates an existing VM drill-down stack back to the VM root view.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/ui`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. If a VM result no longer exists in the provider after cache load, CloudManager will still open the VM tab filtered to that instance ID; the live VM fetch may then show zero rows until the index is refreshed.

## 2026-04-28 (Update 5)

### User Request Handled

- Stop startup VM indexing when a persisted VM index cache is already available.

### Key Code And UI Changes

1. Fixed startup VM prefetch ordering in `internal/ui/app.go`.
   - Context loading now waits for the VM index cache check before starting automatic VM prefetch.
   - If a valid persisted VM index is loaded, startup prefetch is skipped and the app uses the cached records.
   - If the cache is empty, expired, missing, disabled, or fails to load, `prefetch_on_start` still indexes VMs as configured.
   - Manual `:index`, `:reindex`, and Settings `Refresh VM index now` still force a refresh.

2. Added regression coverage in `internal/ui/app_test.go`.
   - Verifies startup prefetch waits for the cache check.
   - Verifies warm cache skips prefetch.
   - Verifies empty cache still triggers configured startup prefetch.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/ui`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. With `prefetch_on_start=true` and no usable cache, startup indexing still runs by design. To never index on startup, disable `Index VMs on startup` in Settings or set `prefetch_on_start=false`.

## 2026-04-28 (Update 4)

### User Request Handled

- Hide Kubernetes worker-node VMs by default so VM lists, search, and dashboard are not polluted by ephemeral cluster nodes.

### Key Code And UI Changes

1. Added Kubernetes worker-node classification in `internal/core/vm.go`.
   - Detects strong EKS, GKE, AKS, Karpenter, and Kubernetes cluster/nodepool metadata in VM names, labels, resource groups, networks, subnets, and security groups.
   - Avoids hiding generic machines just because their name contains `node`.

2. Added default hide behavior and toggles.
   - New config key: `hide_kubernetes_nodes`, default `true`.
   - `K` toggles Kubernetes worker nodes in the VM table, home dashboard, and global VM search.
   - `:k8s-nodes` and `:kubernetes-nodes` also toggle visibility.
   - Settings exposes `Hide Kubernetes nodes`.

3. Applied the filter consistently.
   - VM table hides Kubernetes worker nodes by default and shows a hidden-node banner.
   - Home dashboard and global search compute counts from the filtered VM index unless the toggle is enabled.
   - The persisted VM index still keeps all VMs; filtering is display/search behavior only.

4. Updated README keybindings and config example.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/core ./internal/config ./internal/ui ./internal/views/vms`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. Detection is heuristic because cloud providers expose node metadata differently. It currently keys off strong provider/Kubernetes markers; more markers can be added if real accounts expose different tags.

## 2026-04-28 (Update 3)

### User Request Handled

- Add a home dashboard for indexed VM inventory and make it easy to return home.

### Key Code And UI Changes

1. Expanded the home panel in `internal/ui/app.go`.
   - Shows indexed VM totals, running count, stopped count, public-IP count, backend mode, indexed-context count, provider breakdown, running mix, and last-index age.
   - Dashboard stats are computed from the persisted/global VM index, so it works without refetching when cache data exists.

2. Added home navigation.
   - `H` returns to the home dashboard.
   - `:home`, `:dashboard`, and `:dash` open the same dashboard.
   - Footer and README keybindings now expose the home/dashboard path.

3. Added regression coverage in `internal/ui/app_test.go`.
   - Verifies VM dashboard aggregation.
   - Verifies `:dashboard` clears the active resource view and returns to home.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/ui`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. Dashboard currently summarizes the local VM index only. A Steampipe-style query plugin can be added later as an optional backend for richer cross-resource SQL queries.

## 2026-04-28 (Update 2)

### User Request Handled

- Persist the VM global-search index locally so CloudManager does not need to re-index every startup.

### Key Code And UI Changes

1. Added VM index persistence.
   - New config keys:
     - `vm_index_persistence_enabled` defaults to `true`
     - `vm_index_cache_ttl_hours` defaults to `24`
   - The VM index is saved as `~/.cloudmanager-vm-index.json` with `0600` permissions.
   - The cache format is versioned JSON with context, VM, and `seen_at` fields.
   - Expired records are dropped on load according to the TTL.

2. Wired cache load/save into the app shell.
   - `App.Init()` now loads contexts and the persisted VM index in parallel.
   - VM index updates from normal VM loads or startup/manual indexing save the cache automatically when persistence is enabled.

3. Added refresh controls.
   - Settings now includes:
     - Persist VM index
     - VM index cache TTL
     - Refresh VM index now
   - Command palette supports `:refresh-index`, `:reindex`, and `:index`.

4. Updated README config example with the new VM index persistence keys.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/config ./internal/ui`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. The cache is not encrypted. It is protected with file mode `0600`; real encryption can be added later if needed. Base64 was intentionally not used because it is obfuscation, not encryption.

## 2026-04-28

### User Request Handled

- Fix cloud-context sidebar filtering so pressing Enter on a filtered context opens resources.

### Key Code And UI Changes

1. Fixed filtered sidebar selection in `internal/ui/app.go`.
   - When the sidebar list is filtered, `Enter` now applies the filter and resolves the best matching leaf cloud context.
   - If the filtered selection lands on a provider/account node, CloudManager now chooses the first leaf context under that node instead of only expanding the tree.
   - After activation, focus moves to the main resource pane and initializes the active resource view.

2. Improved context filter matching in `internal/ui/tree.go`.
   - Tree filter values now include provider, account ID, account name, display name, region, credential profile, and descendant context text.
   - This lets context-name searches match collapsed account/provider nodes and still open resources.

3. Added regression coverage in `internal/ui/app_test.go`.
   - Covers filtering to a collapsed account/context and pressing Enter to activate the matching cloud context.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/ui`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. If multiple contexts match the same query, the first match in tree order is opened. More explicit disambiguation could be added later if needed.

## 2026-04-27 (Update 4)

### User Request Handled

- Expand Settings, add managed credential/context backup/edit/remove flows, and improve the main page.

### Key Code And UI Changes

1. Expanded the Settings screen in `internal/ui/app.go`.
   - Settings now exposes backend mode, global search, VM startup indexing, prefetch concurrency, resource cache TTL, billing, billing cache TTL, metrics, metrics period, metrics cache TTL, managed contexts, and config backup.
   - Settings remains available through `,`, `:settings`, `:set`, `:prefs`, and `:preferences`.

2. Added managed context credential management.
   - `:creds`, `:credentials`, and `:contexts` open the managed-context screen.
   - Operators can add, edit, remove, or manually back up CloudManager managed contexts.
   - Add/edit form supports provider, account ID, account name, credential profile/auth reference, and regions.
   - Remove and save paths call `config.BackupConfig()` before mutating `~/.cloudmanager.json`.
   - This manages CloudManager config contexts only; it does not delete external AWS/GCP/Azure credential files.

3. Added config backup support in `internal/config/config.go`.
   - Backups are written beside the config as `.cloudmanager.backup-YYYYMMDD-HHMMSS.json`.

4. Improved the initial home panel.
   - Added metric cards for context count, indexed VM count, and backend mode.
   - Added settings and managed-context hints.

5. Updated README keybindings for Settings, global search, and managed contexts.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/config ./internal/ui`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. Settings uses toggles/cycles and a managed-context form; arbitrary string settings such as theme colors or Gemini model are still best edited directly in the config file for now.

## 2026-04-27 (Update 3)

### User Request Handled

- Add an in-app option to enable VM indexing and make settings accessible.

### Key Code And UI Changes

1. Added a dedicated Settings screen in `internal/ui/app.go`.
   - `,` opens Settings.
   - `:settings`, `:set`, `:prefs`, and `:preferences` also open Settings.
   - Settings currently exposes:
     - Global VM search
     - Index VMs on startup

2. Settings toggles persist to `~/.cloudmanager.json`.
   - Enabling VM indexing sets `prefetch_on_start` and ensures `vms` is in `prefetch_resources`.
   - Enabling VM indexing from the running app starts the VM prefetch flow immediately when contexts are available.

3. Updated the empty-state help and footer to show Settings access.
4. Updated README keybindings with `,` and `g`.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/config ./internal/ui ./internal/views/vms`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. Settings currently covers global search and VM startup indexing only; broader app preferences can be added under the same screen.

## 2026-04-27 (Update 2)

### User Request Handled

- Add a path toward global VM search with configurable startup prefetch.

### Key Code And UI Changes

1. Added config switches in `internal/config/config.go`:
   - `global_search_enabled` defaults to `true`.
   - `prefetch_on_start` defaults to `false`.
   - `prefetch_resources` defaults to `["vms"]` and also accepts `"all"`.
   - `prefetch_concurrency` defaults to `4` and is capped at `16`.

2. Added app-owned VM indexing in `internal/ui/app.go`.
   - VM search records include the VM plus its provider/account/region context.
   - Startup prefetch, when enabled, fetches VM lists per context in bounded background batches.
   - Prefetch results update the index only through Bubble Tea messages on the app update path.
   - Normal VM view loads also publish VM index updates, so global search works for visited contexts even without startup prefetch.

3. Added global VM search UX.
   - `g` opens global VM search.
   - `:find <query>` / `:search <query>` opens global search with an initial query.
   - Search matches VM name, instance ID, private IP, public IP, labels, network, subnet, security groups, resource group, zone, provider, account, and region.
   - `Enter` on a match switches to the VM's context and opens the VM tab filtered to that instance ID.

4. Replaced the empty initial main pane with a concise help panel when no context is selected.
   - Shows global search, command palette, tab, backend, and log shortcuts.
   - Shows VM indexing status when prefetch is running.

5. Updated README config example with the new global search and prefetch keys.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. Prefetch currently indexes VMs only; the config is shaped to allow more resource types later.
2. No live cloud prefetch run was executed in this environment, so provider API throttling behavior is validated structurally but not against live accounts.

## 2026-04-27

### User Request Handled

- Cleanup unnecessary files in the repository.

### Key Code And UI Changes

1. Updated `.gitignore` to exclude:
   - All `*.log` files.
   - OS metadata (`.DS_Store`).
   - Local tooling directory (`.codex-mcp/`).

2. Removed the following unnecessary files and directories:
   - Temporary scripts: `fix_file.py`, `fix_file2.py`, `fix_file3.py`, `fix_lucide.sh`, `patch_lucide.sh`, `append_actions.sh`, `patch_actions.sh`, `patch_analytics.sh`, `fix_ui.go`, `patch_rules.go`.
   - Log files: `current_ui.log`, `err_run.log`, `err.log`.
   - OS metadata: `.DS_Store` and `internal/.DS_Store`.
   - Local MCP tooling: `.codex-mcp/`.

### Validation Performed

- Verified scripts were not imported in the Go codebase or mentioned in the documentation.
- Updated `.gitignore` to prevent re-addition of logs and metadata.

## 2026-04-24

### User Request Handled

- Identify and fix the critical flaws in the project called out during review.

### Key Code And UI Changes

1. Removed shell-string execution from cluster and cost flows.
   - Reworked `internal/core/cluster.go` so kube-context preparation now runs explicit argument-safe commands instead of `bash -c`.
   - Updated AWS, GCP, Azure, and DigitalOcean k9s launchers to pass structured command args.
   - Reworked VM cost-command execution in `internal/views/vms/view.go` to build argv slices and execute them directly, while preserving a rendered command preview for the UI.

2. Removed unsafe cache mutation from async view commands.
   - VM, disk, and snapshot fetch commands no longer read/write view cache maps from background closures.
   - VM metrics and cost enrichment now apply cached data on the UI side and only persist cache updates back in the Bubble Tea update path.
   - VM auto-billing fan-out on every fetch was removed; the VM table no longer triggers per-instance billing lookups just from loading the screen.

3. Fixed double-processing of key events in the networks view.
   - `internal/views/networks/view.go` no longer updates the network/subnet tables inside both the key handlers and the outer update router.
   - This restores single-step cursor movement and stable selection behavior.

4. Added focused regression coverage.
   - Added cluster helper tests for existing-context and fetch-failure handling.
   - Added network view tests for single-step cursor movement in both network and subnet tables.
   - Updated VM tests for the new explicit cost-command builder and non-automatic billing behavior.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/core ./internal/views/vms ./internal/views/networks ./internal/views/disks ./internal/views/snapshots ./internal/views/clusters`
- `GOCACHE=/tmp/go-build-cache go test ./internal/providers/aws ./internal/providers/gcp ./internal/providers/azure ./internal/providers/digitalocean`

### Remaining Risks Or Follow-Up

1. The cluster preparation path now executes provider kubeconfig commands before launching `k9s`; that is safer than shell composition, but it still depends on the relevant CLIs being installed and authenticated in the operator shell.
2. The VM table now avoids automatic billing fan-out entirely. If table-level cost enrichment is wanted later, it should be reintroduced as an explicit user action with bounded concurrency and provider-aware rate limiting.

## 2026-04-20

### User Request Handled

- Fix firewall rule update/edit behavior across the supported cloud providers.

### Key Code And UI Changes

1. Fixed Azure firewall mutation wiring in `internal/providers/registry.go`.
   - Azure firewall bindings now include `ExecuteFirewallActionSDK` in both CLI-backed and SDK-backed provider registrations.
   - This restores rule edit/delete execution from the shared `FirewallProvider` path instead of returning the registry-level “firewall modification not supported” error.

2. Fixed Azure firewall rule normalization in `internal/providers/azure/firewalls.go`.
   - Azure firewall-rule rows now preserve the rule name, NSG resource ID, provider label, and parent resource metadata needed later by edit/delete flows.
   - This fixes the path where fetched Azure rules reached the editor without enough identity data to update the selected NSG rule.

3. Fixed GCP firewall-rule round-tripping in `internal/providers/gcp/firewalls.go` and `internal/providers/gcp/firewalls_edit.go`.
   - Normalized GCP firewall rows now retain the underlying firewall object name.
   - Disabled classic GCP rules now carry a visible disabled marker in the description so the direct toggle flow can infer the current state.
   - GCP add/edit mutation helpers now:
     - split comma-separated ports into valid Compute API port lists
     - round-trip tag and service-account selectors instead of flattening them into ranges
     - preserve sibling `allowed[]` / `denied[]` entries when editing a single selected row
     - remove only the selected row entry on delete, deleting the whole firewall object only when the last entry is removed

4. Blocked read-only GCP policy-derived firewall rows from mutation in `internal/views/firewalls/rules_view.go`.
   - Effective firewall-policy rows remain visible in the table, but edit/delete now stop early with an in-app read-only explanation instead of attempting an invalid `Firewalls.Patch` call.

5. Added regression coverage.
   - Added Azure mapping assertions for mutation-critical fields.
   - Added GCP tests for disabled-rule markers and sibling-preserving edit patches.
   - Added a provider-registry test ensuring Azure firewall bindings expose mutation support.
   - Added a firewall-rules view test ensuring GCP policy-derived rows are blocked as read-only.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/views/firewalls`
- `GOCACHE=/tmp/go-build-cache go test ./internal/providers/aws ./internal/providers/gcp ./internal/providers/azure`

### Remaining Risks Or Follow-Up

1. These fixes were validated through unit tests only in this session; they were not exercised against live AWS, GCP, or Azure APIs from this environment.
2. GCP firewall-policy rows are intentionally read-only in the current UI because they are sourced from effective policy evaluation, not directly mutable classic firewall resources.

## 2026-04-20 (Update 2)

### User Request Handled

- Fix AWS firewall rule updates that still were not applying.
- Clean up the firewall JSON edit screen.

### Key Code And UI Changes

1. Reworked AWS firewall-rule loading in `internal/providers/aws/firewalls.go`.
   - AWS firewall rows now load from `DescribeSecurityGroupRules` instead of expanding `DescribeSecurityGroups` permissions into synthetic row IDs.
   - Each normalized AWS row now carries the real `SecurityGroupRuleId` (`sgr-...`), which is the identifier AWS needs for precise rule mutation.

2. Tightened AWS firewall mutation behavior in `internal/providers/aws/firewalls_edit.go`.
   - Delete now revokes by `SecurityGroupRuleId` when available.
   - Edit now uses `ModifySecurityGroupRules` for in-place updates when the selected rule can be modified directly.
   - If the edit changes the AWS rule kind/direction beyond what in-place modification supports, the code falls back to revoke-by-ID plus authorize-new-rule.
   - Added normalization helpers so placeholder values like `-` are not pushed back into AWS as literal field values.

3. Cleaned up the firewall JSON editor in `internal/views/firewalls/rules_view.go`.
   - The edit pane now uses a full-width panel instead of a cramped side-by-side overlay.
   - The JSON payload now contains editable fields only:
     - direction
     - protocol
     - portRange
     - source
     - destination
     - action
     - priority
     - description
   - Read-only resource metadata is shown separately above the editor instead of being mixed into the JSON buffer.
   - The editor now shows line numbers and no longer renders placeholder `-` values as editable content.

4. Added focused AWS and editor regressions.
   - Added AWS tests for real rule-ID mapping, rule-request generation, and in-place modify eligibility.
   - Added a firewall edit-pane layout test so the JSON editor must stay within the active window.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/providers/aws ./internal/views/firewalls`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. AWS edit behavior was validated through unit tests only in this environment; no live AWS rule modification was executed here.
2. AWS in-place modify still follows AWS API limits: changing a rule between CIDR, prefix-list, and referenced-security-group types falls back to revoke-and-authorize instead of using `ModifySecurityGroupRules`.

## 2026-04-20 (Update 3)

### User Request Handled

- Replace the awkward JSON firewall-rule editor with an easier terminal form for AWS security-group rule edits.

### Key Code And UI Changes

1. Replaced the AWS edit experience in `internal/views/firewalls/rules_view.go`.
   - Pressing `e` on an AWS security-group rule now opens a compact terminal form instead of a raw JSON editor.
   - The AWS form exposes the fields operators actually care about:
     - direction
     - protocol
     - ports
     - peer (`CIDR`, `sg-...`, or `pl-...`)
     - description
   - AWS `Action` is no longer editable in the form because security-group rules are allow-only.

2. Kept non-AWS firewall edits functional with a generic form.
   - GCP/Azure-style rules now use a structured field form instead of JSON as well.
   - Generic fields include direction, action, protocol, ports, source, destination, description, and priority.

3. Improved terminal ergonomics for firewall-rule editing.
   - The edit view now uses a full-width form panel instead of a cramped JSON block.
   - Added direct field validation for bad direction/action/priority input.
   - Updated rule-edit help text and form focus handling (`Tab`, `Shift+Tab`, `Enter`, `F2`, `Ctrl+S`).

4. Updated firewall-rule view tests.
   - Added/updated coverage for:
     - opening the AWS terminal edit form
     - submitting an edited AWS SG rule through the form
     - rejecting invalid form input
     - keeping the edit pane within the active terminal window

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/views/firewalls`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. The new terminal form was validated through unit tests only in this environment; it was not exercised against a live AWS account in this session.
2. The AWS edit form intentionally models the security-group abstraction, not AWS Network Firewall resources.

## 2026-04-20 (Update 4)

### User Request Handled

- Fix AWS security-group rule edits that still were not saving when operators added another port in the terminal form.

### Key Code And UI Changes

1. Finished the AWS multi-port edit/replace path in `internal/providers/aws/firewalls_edit.go`.
   - AWS edit logic now consistently treats comma-separated ports as multiple SG rules when an operator edits a single selected rule into multiple ports.
   - Revoke and authorize flows now both use the same multi-permission builder, so replace-style edits can revoke the original rule and create the new split port rules correctly.
   - In-place AWS modify still stays limited to a single port/range per rule, matching EC2 `ModifySecurityGroupRules`.

2. Tightened AWS rule-request and permission validation.
   - Invalid AWS port segments now fail early with a clear error instead of silently producing a partial permission.
   - Description fields are only attached to AWS peers when present, avoiding noisy empty values in the generated request payload.

3. Kept failed saves visible in the firewall edit form.
   - Firewall provider errors now remain in the edit pane as `formError` instead of feeling like the save was ignored.
   - The AWS edit helper text now explicitly states that comma-separated ports create separate AWS SG rules.

4. Added regression coverage for the exact AWS SG edit case.
   - Added AWS tests for:
     - rejecting multi-port in-place modify requests
     - splitting comma-separated ports into multiple EC2 permissions
     - forcing replace behavior when an edit expands to multiple ports
   - Added a firewall-view test ensuring provider save errors keep the edit form open and visible.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/providers/aws`
- `GOCACHE=/tmp/go-build-cache go test ./internal/views/firewalls`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. The fix was validated through unit tests only in this environment; no live AWS security-group edit was executed here.
2. Editing one AWS SG row into multiple ports intentionally replaces the selected rule with multiple EC2 security-group rules, because AWS SG rules do not support a single row containing multiple discrete ports.

## 2026-04-21

### User Request Handled

- Fix GCP SDK auth failures that were stopping resource screens with `could not find default credentials` when ADC was not configured.

### Key Code And UI Changes

1. Added shared GCP SDK auth fallback in `internal/providers/gcp/auth.go`.
   - GCP SDK clients now try normal Google Application Default Credentials first.
   - If ADC is missing, CloudManager now falls back to the active `gcloud` login by calling `gcloud auth print-access-token` and building the SDK client from that token.
   - If both ADC and `gcloud` token fallback fail, the returned error now explains both attempts and points operators at the concrete fixes:
     - `gcloud auth application-default login`
     - `GOOGLE_APPLICATION_CREDENTIALS`
     - `gcloud auth login`

2. Applied the shared auth helper across GCP SDK-backed provider paths.
   - Compute-backed resource flows now use the fallback helper:
     - VMs
     - disks
     - snapshots
     - networks
     - firewalls
     - firewall edits
   - The same fallback path now also covers GKE, Cloud SQL, Monitoring, Recommender, and BigQuery SDK clients.

3. Added focused GCP auth tests.
   - Added tests for:
     - trimming the `gcloud auth print-access-token` output correctly
     - preserving CLI stderr in fallback errors so auth failures are diagnosable in the terminal

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/providers/gcp`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. This was validated through unit tests only in this environment; no live GCP SDK call was executed here.
2. The CLI-token fallback uses a static access token per client creation. That is fine for the app’s short-lived request pattern, but long-running background operations would need a refreshable token source instead.

## 2026-03-27

### Purpose
This file is the running handoff for recent CloudManager work so future runs can recover context quickly without depending on prior chat history.

### What Was Done

1. Rebuilt AWS context modeling so the app uses `account -> region -> VMs` instead of conflating AWS account identity, auth profile, and region in profile names.
   - Added explicit credential profile handling in the cloud context model.
   - Normalized AWS display labels to account-centric names like `9431 (main)`.
   - Updated AWS parsing so region aliases map back to a base auth profile when possible.

2. Added an app-owned multi-cloud context registry in config.
   - `~/.cloudmanager.json` now supports managed cloud contexts for AWS, GCP, and Azure.
   - The app prefers managed contexts and only falls back to CLI discovery when needed.
   - GCP selection writes into this same registry.
   - Goal: stable `account/project/subscription -> region -> resources` behavior across clouds.

3. Fixed stale async fetch races in the UI.
   - VM, disk, and snapshot fetches now carry request identity and ignore stale responses after account or region switches.
   - This addressed cases where rows from an older cloud context overwrote the current selection.

4. Fixed terminal clipping and pane overflow behavior.
   - Added a top-level render clamp so child views cannot draw past the active terminal window.
   - Tightened table height calculations so bottom borders stay visible even with many rows.

5. Added horizontal column panning to all resource tables.
   - Implemented for VMs, Disks, and Snapshots.
   - Selection no longer spills outside the pane when wide columns are present.
   - Navigation uses `Left` / `Right` or `h` / `l`.

6. Added regression coverage for large and narrow table layouts.
   - Added tests for large VM datasets and narrow panes.
   - Added matching narrow-pane coverage for disk and snapshot tables.

7. Added provider SSH command coverage.
   - Added tests for AWS, GCP, and Azure SSH command construction.

8. Added a provider smoke-test command.
   - New CLI flag in `main.go`: `--smoke-test=all|aws|gcp|azure`
   - New package: `internal/smoke`
   - The smoke test:
     - loads managed contexts, with provider-by-provider discovery fallback
     - validates VM listing through the configured backend
     - validates non-destructive `Describe` on a sample VM when one exists
     - validates SSH command construction
     - prints preview commands for `Start`, `Stop`, `Restart`, and `SSH`
   - Output order was made deterministic.
   - Added smoke-package tests for preview command generation and provider target parsing.

9. Improved per-VM cost visibility in the VM table.
   - VM rows now always carry visible `Cost` and `Cost Trend` placeholders instead of blank cells.
   - Cost enrichment now:
     - uses cached per-resource results
     - respects billing cache TTL
     - limits concurrent resource-cost lookups
     - reapplies table ordering after enrichment so the row order stays stable
   - Added VM-view tests for cost enrichment and cache reuse.

10. Changed VM ordering so running instances are always pinned above non-running instances.
   - This priority is preserved on initial fetch, after cost enrichment, and after manual sort changes.
   - Manual sorting still applies within each state group.
   - Added VM-view tests for running-first ordering, including when sorting by cost.

11. Fixed Azure VM identity data used by cost lookup and table display.
   - Azure VM fetchers now store the real resource ID in `VM.ID` instead of the VM name.
   - This fixes the `Instance ID` column and makes Azure per-resource cost queries use the correct identifier.

12. Changed VM cost behavior from automatic background fetch to on-demand lookup guidance.
   - VM table fetch no longer auto-enriches rows with provider billing data.
   - Added a VM action named `Cost`.
   - The `Cost` action opens the detail pane with:
     - a billing console link
     - provider-specific guidance
     - for AWS, a prebuilt `aws ce get-cost-and-usage` command for the selected VM
   - This keeps the fast operational view simple and avoids mixing approximate UI comprehension with deeper billing logic.
   - Added tests to ensure VM fetch does not auto-trigger cost lookup and that the AWS cost guide is generated.

13. Introduced a provider registry and capability-driven shell wiring.
   - Added a registry in `internal/providers/registry.go`.
   - Provider metadata now defines:
     - display name
     - ordering
     - aliases
     - capabilities
     - optional global leaf label
   - AWS, GCP, and Azure are now bound through registry-backed CLI/SDK adapters instead of hardcoded switch logic in the app shell.

14. Made the UI capability-driven without changing the main operator flow.
   - The sidebar provider ordering now follows the provider registry instead of a hardcoded cloud list.
   - The main tab bar now renders only the tabs supported by the active provider.
   - This preserves current AWS/GCP/Azure behavior while allowing providers with partial support to plug in cleanly.

15. Added DigitalOcean as a proof provider.
   - New provider package: `internal/providers/digitalocean`
   - DigitalOcean currently plugs in as a VM-focused provider using `doctl`.
   - Discovery uses the current `doctl` account.
   - VM listing, basic VM actions, and SSH command generation are wired.
   - Unsupported capabilities such as disks and snapshots stay hidden through the capability model.

16. Added provider authoring documentation.
   - New guide: `docs/PROVIDER_GUIDE.md`
   - Documents the registry model, capability contract, provider packaging, and the minimal proof standard for community-contributed providers.

### Key Files Touched

- `main.go`
- `internal/config/config.go`
- `internal/core/context.go`
- `internal/providers/aws/parser.go`
- `internal/providers/aws/parser_test.go`
- `internal/providers/aws/client.go`
- `internal/providers/aws/client_test.go`
- `internal/providers/gcp/client_test.go`
- `internal/providers/azure/client_test.go`
- `internal/ui/app.go`
- `internal/ui/app_test.go`
- `internal/ui/tree.go`
- `internal/views/vms/view.go`
- `internal/views/vms/view_test.go`
- `internal/views/disks/view.go`
- `internal/views/disks/view_test.go`
- `internal/views/snapshots/view.go`
- `internal/views/snapshots/view_test.go`
- `internal/smoke/smoke.go`
- `internal/smoke/smoke_test.go`

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./...`
  - Passed after the smoke-test package and tests were added.
  - Passed again after the VM cost, running-first ordering, and Azure VM ID fixes.
  - Passed again after switching VM cost handling to the on-demand `Cost` action flow.
  - Passed again after the provider registry refactor, capability-driven tab rendering, and DigitalOcean integration.

- `GOCACHE=/tmp/go-build-cache go run . --smoke-test=all`
  - The command path runs.
  - Observed environment/runtime limitations in this session:
    - Azure CLI missing: `az` not installed in this environment.
    - AWS fetches failed due endpoint connectivity restrictions from this environment.

### Important Runtime Notes

- Live `Start`, `Stop`, `Restart`, and `SSH` actions were not executed against cloud resources in this session.
- The smoke-test command is designed to be non-destructive:
  - it performs real fetch and describe checks
  - it builds SSH commands
  - it prints preview commands for state-changing actions instead of executing them

### Outstanding Follow-Up

1. Run `--smoke-test=all` in a fully provisioned environment with:
   - network access to cloud APIs
   - `aws`, `gcloud`, and `az` installed
   - valid credentials for the managed contexts

2. Decide whether the smoke test should gain stricter provider-specific dry-run behavior where supported beyond the current preview-only output.

3. If future UI regressions appear, inspect pane width and column-window math first; most recent table overflow issues were caused by unconstrained render width and selection drawing past the pane.

## 2026-03-30

### User Request Handled

- Investigate whether disk selection is implemented, since disk rows could not be selected on any cloud platform.
- Remove `Cost` / `Cost Trend` from the VM table and keep cost lookup on-demand from the Actions menu with provider-specific CLI commands.

### Key Code And UI Changes

1. Fixed disk row navigation and selection in `internal/views/disks/view.go`.
   - Disk support was already implemented for AWS, GCP, and Azure at the provider layer.
   - The actual issue was view event routing: table arrow keys were being consumed before the Bubble Tea table handled them.
   - The disk view now routes keys like the VM view so row navigation works again.
   - Added visible-row tracking so filtered disk rows map back to the correct selected disk when opening Actions.

2. Removed VM cost columns from the operational table flow.
   - Removed `Cost` and `Cost Trend` from the canonical/default VM columns in `internal/core/vm.go` and `internal/config/config.go`.
   - Added VM column sanitization so older saved configs that still contain those deprecated columns no longer render them.
   - `buildVMColumns` now sanitizes columns defensively at render time as well.

3. Tightened the VM `Cost` action to be command-first.
   - The `Cost` action text now shows provider-specific CLI commands instead of table-driven cost fields.
   - AWS: on-demand `aws ce get-cost-and-usage` commands with `json` and `table` output variants.
   - GCP: `bq query` templates against billing export tables with `prettyjson` and `csv` output variants.
   - Azure: `az rest` Cost Management query commands with `json` and `table` output variants.
   - DigitalOcean remains a placeholder because a provider-native per-resource cost CLI flow is not yet wired.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/views/disks ./internal/views/vms ./internal/config`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. GCP cost commands depend on Cloud Billing export data in BigQuery; if `gcp_billing_dataset` is unset, the action shows a runnable template with placeholders/default assumptions instead of a fully resolved table path.
2. Azure cost commands are currently generated through `az rest` against Cost Management; they were validated by compile/tests here, not by a live Azure CLI run in this environment.
3. Snapshot view still uses its older key-routing pattern and may deserve the same visible-row/input-routing cleanup if similar selection issues are reported there.

## 2026-04-01

### User Request Handled

- Confirm what should happen next and proceed with the most direct follow-up from the handoff.
- Clean up git worktrees if needed before making changes.
- Run the next practical steps: execute smoke validation in this environment, then move implementation forward.

### Key Code And UI Changes

1. Refreshed snapshot view input routing in `internal/views/snapshots/view.go`.
   - Aligned `SnapshotsView.Update` with the disk view pattern so table key handling does not short-circuit Bubble Tea table updates.
   - This restores arrow-key row navigation while preserving snapshot pane-specific handlers.

2. Added visible-row tracking for snapshots.
   - `SnapshotsView` now keeps a filtered `visibleSnaps` slice and uses it for row rendering, cursor bounds, and action selection.
   - Search, sort, refresh, and column rebuilds now stay aligned with the rows the operator can actually see.

3. Added snapshot regression coverage in `internal/views/snapshots/view_test.go`.
   - Added a row-navigation test for `Down`.
   - Added a filtered-selection test so actions resolve the selected snapshot from visible rows instead of raw backing order.

4. Inspected git worktree state before editing.
   - Confirmed there is only one active worktree for this repository (`/Users/ranger/CLOUDMANAGER` on `main`).
   - No destructive cleanup was performed because the dirty tree appears to be active project work, not stale auxiliary worktrees.

5. Added an initial firewall/security-group slice for AWS.
   - Added real `SecurityGroup` and `FirewallRule` core models in `internal/core/firewall.go` and `internal/core/firewall_rule.go`, replacing the old stubs.
   - Added an AWS SDK-backed firewall fetcher in `internal/providers/aws/firewalls.go`.
   - Wired `FirewallProvider` through the CLI and SDK provider adapters plus the provider registry.
   - Enabled `CapabilityFirewalls` for AWS only; GCP and Azure remain hidden until they have real implementations.

6. Added new firewall views and tab wiring.
   - New package: `internal/views/firewalls`
   - Added a `Firewalls` tab in `main.go` as tab `4`.
   - The new view supports:
     - searchable security-group listing
     - `View Rules` drill-down
     - local `Describe` panes for groups and rules
   - The child rules view is read-only for now; no delete/mutation flow was added in this slice.

7. Added focused regression coverage for the firewall slice.
   - Added provider capability assertions in `internal/providers/registry_test.go`.
   - Added view tests for narrow rendering and `View Rules` drill-down in `internal/views/firewalls/view_test.go`.

8. Extended firewall support to GCP.
   - Added `internal/providers/gcp/firewalls.go`.
   - GCP firewall rows are grouped by VPC network so the existing Firewalls tab maps cleanly onto GCP’s firewall model.
   - `View Rules` now drills into GCP firewall rules filtered to the selected network.
   - Enabled `CapabilityFirewalls` for GCP in the provider registry for both CLI-mode and SDK-mode app operation.

9. Added focused GCP firewall unit coverage.
   - Added tests for GCP firewall rule normalization, open-port audit detection, and network-name extraction in `internal/providers/gcp/firewalls_test.go`.

10. Extended firewall support to Azure NSGs.
   - Added `internal/providers/azure/firewalls.go`.
   - Azure rows now map Network Security Groups into the existing Firewalls tab.
   - `View Rules` drills into NSG rules via the Azure network SDK and includes both custom and default rules.
   - Enabled `CapabilityFirewalls` for Azure in the provider registry for both CLI-mode and SDK-mode app operation.

11. Added focused Azure firewall unit coverage and dependency wiring.
   - Added tests for Azure NSG rule normalization, open-port audit detection, and resource-group extraction in `internal/providers/azure/firewalls_test.go`.
   - Added Azure network SDK dependency to `go.mod` / `go.sum`.

12. Added richer firewall UX in the TUI.
   - `internal/views/firewalls/view.go` now supports audit highlighting for risky firewall rows, sort configuration (`S`), and persisted firewall column configuration (`C`) backed by `firewall_columns` in config.
   - `internal/views/firewalls/rules_view.go` now styles risky rules in alert color and deny rules in a subdued style.
   - Firewalls views can now be opened in a filtered/scoped mode for drill-down navigation.

13. Added VM → Firewalls cross-navigation.
   - Added `SecurityGroups` to `core.VM`.
   - Added `View Firewalls` to VM actions.
   - AWS VM fetchers now retain security group IDs.
   - GCP VM fetchers now carry the network name into `SecurityGroups` for compatibility with the network-grouped firewall model.
   - Azure VM fetchers now resolve attached NICs in the SDK path to populate virtual network, subnet, private/public IPs, and related NSG names so VM drill-down can target Azure firewalls as well.

14. Closed two remaining firewall data gaps.
   - GCP firewall groups now compute `AttachedResources` from aggregated instance network-interface usage instead of a placeholder `0`.
   - Added Azure helper coverage for NIC/public-IP/NSG enrichment and GCP coverage for network attachment counting.

15. Fixed firewall table rendering regressions and improved GCP rule drill-down fidelity.
   - Firewalls and Firewall Rules rows now truncate raw cell values before applying per-row styling, which prevents leaked ANSI fragments in clipped cells and keeps long risky/selected rows from blowing out the fixed window.
   - GCP network rule drill-down now prefers `networks.getEffectiveFirewalls`, so the rules pane includes effective firewall policy rules in addition to classic VPC firewall rules.
   - GCP classic firewall mapping now preserves source tags, target tags, and target service accounts in normalized Source/Destination fields instead of dropping those selectors from some directions.
   - Added focused tests for risky firewall row clipping, selected firewall-rule clipping, GCP tag mapping, and GCP effective firewall policy mapping.

16. Promoted VM-style table scrolling and pane clamping into shared UI behavior.
   - Added shared helpers in `internal/ui/table_helpers.go` for table viewport sizing, visible-column windowing, width-safe truncation, scroll-hint headers, and final pane clamping.
   - Refactored VM, Disk, Snapshot, Firewalls, and Firewall Rules views to use those shared helpers instead of separate view-local copies.
   - Firewalls and Firewall Rules now support `Left` / `Right` horizontal paging like the VM view, so wide tables no longer depend on dropping columns to fit.
   - Firewall selection, actions overlays, and rules describe panes now render through the shared clamp path, which keeps the TUI box intact in fixed/narrow windows.
   - Added focused firewall tests for horizontal panning plus overlay/describe-pane fit within the active window.

17. Added a reusable resource-table factory for future services.
   - `internal/ui/table_helpers.go` now exposes shared default table styles plus `NewResourceTable(...)`, which builds a width-aware, horizontally pageable Bubble Tea table from a column list.
   - VM, Disk, Snapshot, Firewalls, and Firewall Rules table constructors now use that shared factory instead of each view repeating Bubble Tea style/setup boilerplate.
   - This is the new baseline for future table-backed services such as RDS, Cloud SQL, clusters, or databases: provide columns + rows + view-specific actions, and inherit the shared scroll/clamp behavior automatically.

18. Removed inline ANSI row coloring from firewall tables.
   - `internal/views/firewalls/view.go` no longer colors risky security-group cells directly inside Bubble Tea table rows; risky groups now use a plain-text `⚠` marker in the `Name` column.
   - `internal/views/firewalls/rules_view.go` no longer colors risky or deny rule cells inline; risky rules now use a plain-text `⚠` marker in `Direction`, and deny rules use a plain-text `⊘` marker in `Action`.
   - Added regressions to ensure firewall table cells stay within width limits and do not leak raw ANSI escape sequences when clipped.

19. Changed firewall risk emphasis to selection-state styling and fixed GCP rule merging.
   - Firewalls and Firewall Rules now switch the selected-row bar to the alert color when the currently selected item is risky, instead of relying on colored row text.
   - GCP firewall rule drill-down now always merges classic VPC firewall rules with effective firewall-policy rules, so network-tag selectors from classic rules and policy-derived rules show up together in one table.
   - Added focused GCP coverage to verify classic tag-based rules and policy rules survive the merge together.

20. Fixed GCP CLI-backend resource loading for non-VM tabs.
   - Added GCP CLI-backed fetchers for disks, snapshots, firewall groups, and firewall-rule drill-down in `internal/providers/gcp/resources_cli.go`.
   - The GCP `cli` backend in `internal/providers/registry.go` now uses `gcloud` for those resource lists instead of silently depending on ADC-backed SDK auth for non-VM tabs.
   - The GCP `sdk` backend now falls back to the CLI fetch path for disks, snapshots, and firewalls when SDK auth fails, which keeps resource visibility working in mixed-auth local setups.
   - Added parser and summarization coverage for GCP CLI disks, snapshots, and firewall-group aggregation.

21. Added a backend-mode indicator and in-app application logs.
   - The app footer now shows a small `Mode: CLI|SDK` indicator plus an `L:Logs` hint so operators can see the current fetch mode at a glance.
   - Added a file-backed logger in `internal/logging/logging.go` that writes to `~/.cloudmanager.log` (or a temp fallback path if the home directory is unavailable).
   - Added a shell-level logs view in `internal/ui/app.go`: press `L` to open recent application logs, `r` to reload, and `Esc` to close.
   - Added structured log entries for app lifecycle events, context/tab changes, resource fetch start/completion/failure across VM/Disk/Snapshot/Firewall views, and GCP SDK→CLI fallback events.
   - Added app tests for the footer mode badge and log-view layout.

22. Fixed shell geometry so pane borders anchor to the terminal window.
   - `internal/ui/app.go` now computes explicit outer pane sizes and inner content viewports instead of sizing child views against the full terminal and then adding shell borders on top.
   - Main views are now initialized/resized with the true content area inside the shell border and below the tab bar, which keeps the outer frame fixed to the terminal like a proper dashboard shell.
   - Sidebar, main pane, config pane, and logs pane now all render through the same outer-size-aware border sizing path.
   - Updated app tests to assert against content-area resize math rather than the older ad hoc main-width logic.

23. Tightened the shell chrome toward a more k9s-like layout.
   - Replaced the separate full-border sidebar and main-pane boxes with a single outer shell frame plus lighter internal dividers.
   - The sidebar now renders as a docked left panel with a right-side divider instead of a standalone bordered box.
   - The main content area remains tab-docked inside the outer shell, while config and logs views also reuse the same shell frame instead of opening in separate nested pane borders.
   - Added `ShellStyle` in `internal/ui/styles.go` and kept the outer-frame sizing/layout logic centralized in `internal/ui/app.go`.

24. Made shared resource tables expand into newly available screen width.
   - `internal/ui/table_helpers.go` now expands the currently visible column set to consume the full available table viewport width instead of preserving each column's original fixed width.
   - This applies automatically to all resource-table views that use the shared helper path, including VMs, Disks, Snapshots, Firewalls, and Firewall Rules.
   - The main user-visible effect is that hiding the left pane or resizing the terminal wider now causes visible columns to stretch and fill the newly available space instead of leaving unused blank area.
   - Added focused helper tests to verify full-width expansion in normal layouts and correct clamping when only one column fits.

25. Changed shared table expansion from equal growth to weighted growth.
   - `internal/ui/table_helpers.go` now weights extra width toward long-text columns such as `Name`, `Description`, `ID`, `Image`, `Network`, `Subnet`, `VPC`, `Project`, `Context`, and `Tags`.
   - Compact/status-style columns such as `Status`, `State`, `Zone`, `Region`, `Count`, `Rules`, `Ports`, `Protocol`, `Action`, `Direction`, `Age`, `CPU`, `RAM`, `Size`, and `Cost` now stay tighter when the table gains room.
   - This keeps wide panes from wasting space on short status/count fields and makes the stretched layout read more naturally after hiding the sidebar or maximizing the terminal.
   - Added targeted helper coverage to assert that descriptive columns absorb more of the new width than compact columns.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go run . --smoke-test=all`
  - Result in this environment:
    - AWS contexts failed on live endpoint connectivity to EC2.
    - GCP contexts failed via `gcloud` command execution.
    - Azure CLI (`az`) is not installed.
    - DigitalOcean CLI (`doctl`) is not installed.
    - Summary: `checked=28 passed=0 failed=28`

- `GOCACHE=/tmp/go-build-cache go test ./internal/views/snapshots`
- `GOCACHE=/tmp/go-build-cache go test ./internal/views/snapshots ./internal/views/disks ./internal/views/vms ./internal/config`
- `GOCACHE=/tmp/go-build-cache go test ./internal/views/firewalls ./internal/providers`
- `GOCACHE=/tmp/go-build-cache go test ./internal/providers/gcp ./internal/providers`
- `GOCACHE=/tmp/go-build-cache go test ./internal/providers/azure ./internal/providers`
- `GOCACHE=/tmp/go-build-cache go test ./internal/views/firewalls ./internal/ui ./internal/providers ./internal/providers/gcp`
- `GOCACHE=/tmp/go-build-cache go test ./internal/views/firewalls ./internal/ui ./internal/providers ./internal/providers/azure`
- `GOCACHE=/tmp/go-build-cache go test ./internal/views/firewalls ./internal/views/vms ./internal/config ./internal/core ./internal/providers/aws ./internal/providers/gcp`
- `GOCACHE=/tmp/go-build-cache go test ./internal/views/... ./internal/providers ./internal/config ./internal/ui ./internal/core`
- `GOCACHE=/tmp/go-build-cache go test ./internal/providers/azure ./internal/providers/gcp ./internal/providers`
- `GOCACHE=/tmp/go-build-cache go test ./internal/views/firewalls ./internal/views/vms ./internal/config ./internal/core`
- `GOCACHE=/tmp/go-build-cache go test ./internal/views/firewalls ./internal/providers/gcp`
- `GOCACHE=/tmp/go-build-cache go test ./internal/views/vms ./internal/views/disks ./internal/views/snapshots ./internal/views/firewalls ./internal/ui`
- `GOCACHE=/tmp/go-build-cache go test ./internal/ui ./internal/views/vms ./internal/views/disks ./internal/views/snapshots ./internal/views/firewalls`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

1. Full end-to-end smoke validation is still pending in an environment with cloud API access plus working `aws`, `gcloud`, and `az` installations.
2. Azure VM → Firewalls enrichment is implemented in the SDK path. The CLI VM path still does not resolve NSG/network attachments, so Azure drill-down quality depends on backend choice.
3. GCP drill-down now includes effective firewall policies from `networks.getEffectiveFirewalls`, but regional network firewall policy coverage is still incomplete because that requires additional regional effective-firewall queries.
4. GCP’s firewall model remains network-grouped rather than SG-like, so drill-down is useful but still reflects GCP networking semantics rather than per-VM security-group semantics.
5. Firewall mutations are intentionally not implemented yet. The current slice is read-only: list, inspect, and drill into rules.
6. The repository is still in a large uncommitted refactor state, so future cleanup should be branch-aware and avoid assuming untracked paths are disposable.

## 2026-04-14

### Purpose
Completed the SDK backend implementations for Clusters and Databases across AWS, GCP, and Azure to remove remaining stubs and advance the roadmap. The Monitoring/Metrics and Billing/FinOps modules were verified to be already fully implemented in the SDK layer.

### What Was Done
1. Implemented `FetchClustersSDK` for AWS (EKS), GCP (GKE), and Azure (AKS).
2. Implemented `FetchDatabasesSDK` for AWS (RDS), GCP (Cloud SQL), and Azure (PostgreSQL Flexible Servers).
3. Added the necessary cloud provider SDK dependencies to `go.mod` (`eks`, `rds`, `container/v1`, `sqladmin/v1beta4`, `armcontainerservice`, `armpostgresqlflexibleservers`).
4. Fixed deprecated client initializations for older Azure SDK packages (`NewManagedClustersClient` and `NewServersClient` vs factory).
5. Cleaned up unused imports in the AWS implementations.
6. Verified that metrics and billing data models and SDK fetchers were completely implemented and not just stubs.

### Key Files Touched
- `go.mod`, `go.sum`
- `internal/providers/aws/clusters.go`
- `internal/providers/aws/databases.go`
- `internal/providers/gcp/clusters.go`
- `internal/providers/gcp/databases.go`
- `internal/providers/azure/clusters.go`
- `internal/providers/azure/databases.go`

### Validation Performed
- `go mod tidy` executed.
- `go build ./...` passes successfully.
- `go test ./...` passes successfully. Checked `err.log` and `err_run.log`, both were clear.

### Remaining Risks Or Follow-Up
- The Clusters and Databases TUI views (`internal/views/clusters`, `internal/views/databases`) still need tests.
- Azure currently only fetches PostgreSQL Flexible Servers; additional DB engines might be needed based on user demand.

## 2026-04-14 (Update)

### What Was Done
- Discovered that while `Databases` fetchers were implemented for all providers, they were accidentally omitted from the `backendBindings` in `internal/providers/registry.go`. 
- Added the `Databases: databaseFuncs{...}` block to the CLI and SDK bindings for AWS, GCP, and Azure in the registry.
- This fixes the issue where the Databases tab would show up as empty/unsupported despite the API fetchers being fully written.

## 2026-04-14 (Update 2)

### What Was Done
- Discovered that when using the CLI backend for AWS (`aws`), commands like `eks list-clusters` and `rds describe-db-instances` would hang because the AWS CLI defaults to using a paginator (`less`) for output, requiring the user to press `q` to exit.
- Added `--no-cli-pager` globally to all `aws` CLI command executions (VMs, Clusters, Databases, SSH/SSM) in `internal/providers/aws/client.go` and `internal/providers/aws/clusters.go` to ensure silent, non-interactive JSON parsing doesn't hang.
- Updated AWS client tests to expect the new `--no-cli-pager` flag.

## 2026-04-14 (Update 3)

### What Was Done
- Added an in-app global hotkey (`B`) to allow operators to instantly toggle between `CLI` and `SDK` backends.
- Pressing `B` saves the new backend preference to config, logs the change, updates the Mode badge in the footer, and automatically dispatches a refresh command (`r`) to the currently active view so data is immediately re-fetched via the new backend.
- Updated the main footer to advertise the new `B:Mode` hotkey alongside the logs hotkey.

## 2026-04-14 (Update 4)

### What Was Done
- Discovered the `Networks` tab was empty because `FetchNetworks` and `FetchSubnets` were returning `not implemented` stubs in both the `SDKProvider` and `CLIProvider` adapters in `internal/providers/sdk.go` and `internal/providers/cli.go`.
- Fixed the interface adapters so they properly delegate requests to the registered backend bindings. Networks and Subnets now successfully render in the TUI across AWS, GCP, and Azure.

## 2026-04-14 (Update 5)

### What Was Done
- Fixed a UX "infinite loop" bug in the `Networks` tab where users trying to select `View VMs` for a subnet were trapped in a navigation loop.
- The root cause was that `paneSubnetActions` was missing from the keyboard event routing switch in `internal/views/networks/view.go`, causing "Enter" keystrokes to fall through to `handleTableKeys`. This incorrectly switched the active pane back to the parent `paneActions` instead of executing the subnet action, forcing the user into a cycle of sub-menus without ever launching the `VMsView`.

## 2026-04-14 (Update 6)

### What Was Done
- Discovered that while backend implementations existed for firewall rule modifications (`ExecuteFirewallActionSDK`), the TUI's `RulesView` was completely read-only and skipped the actions list.
- Implemented the `Action` menu pattern for firewall rules, similar to the VMs and Networks tabs.
- Pressing `Enter` on a firewall rule now opens a menu with: `Describe`, `Enable` (GCP), `Disable` (GCP), and `Delete` (Destructive).
- Actions are fully wired to the backend execution layer. Confirmed modifications trigger a background CLI/SDK execution and a subsequent table refresh upon success or gracefully show an error notification in the UI if unsupported by the specific cloud provider.

## 2026-04-16

### Purpose
Completed a request to completely rewrite `ExecuteFirewallActionSDK` to use native Go SDKs instead of shelling out to `os/exec` for AWS, GCP, and Azure. 

### What Was Done
1. **AWS**: Updated `internal/providers/aws/firewalls_edit.go` to use `ec2.Client`. Implemented `Delete` and `Edit` actions. Added a "Revoke then Authorize" logic specifically for AWS to handle "Edit" correctly. Wrote `toAWSPermission` mapping helper to convert the normalized `core.FirewallRule` into `ec2types.IpPermission`.
2. **GCP**: Renamed `firewalls_k9s.go` to `internal/providers/gcp/firewalls_edit.go` and rewrote it using `compute.NewService(ctx)`. Implemented `Delete`, `Enable`, `Disable` (using Patch API on the `Disabled` boolean) and `Edit` (using Patch API mapping normalized properties like `SourceRanges`, `Allowed` and `Denied` slices back to a GCP `compute.Firewall` object). Stripped `-allow-`/`-deny-` index suffixes when modifying rules since GCP rules bundle multiple definitions into one API object.
3. **Azure**: Rewrote `internal/providers/azure/firewalls_edit.go` to use `armnetwork.NewSecurityRulesClient`. Implemented `Delete` (`BeginDelete`) and `Edit` (`BeginCreateOrUpdate`). Mapped all `core.FirewallRule` fields to the `armnetwork.SecurityRule` properties format.
4. Added `applog.Infof("AUDIT: user modified firewall rule: ...")` tracking to all destructive and edit operations across the three providers.
5. Successfully compiled with `go build ./...`

### Key Files Touched
- `internal/providers/aws/firewalls_edit.go`
- `internal/providers/gcp/firewalls_k9s.go` (Deleted)
- `internal/providers/gcp/firewalls_edit.go` (Added)
- `internal/providers/azure/firewalls_edit.go`
- `LOG.md`

## 2026-04-14 (Update 7)

### What Was Done
- Completely refactored `ExecuteFirewallActionSDK` across AWS, GCP, and Azure to utilize pure Go SDKs instead of relying on `os/exec` wrappers (`az`, `aws`, `gcloud`).
- **AWS:** Implemented deletions and edits using `ec2.Client` (`RevokeSecurityGroupIngress`/`Egress` and `AuthorizeSecurityGroupIngress`/`Egress`).
- **GCP:** Implemented deletions, enable/disable toggles, and property updates using `compute.Service` (`Firewalls.Delete` and `Firewalls.Patch`).
- **Azure:** Implemented deletions and property updates using `armnetwork.SecurityRulesClient` (`BeginDelete` and `BeginCreateOrUpdate`).
- **TUI Update:** Added a new `paneEditRule` UI form within the Firewalls module. Users can now select "Edit" to open an interactive form to modify Protocol, Port Range, and IP/CIDR block allocations directly in the application.
- Added comprehensive audit logs (`applog.Infof("AUDIT: ...")`) tracking any destructive or state-modifying actions applied to security groups across all cloud providers.

## 2026-04-14 (Update 8)

### What Was Done
- **UX Request:** Addressed the UX question regarding a Steampipe dashboard / K9s-style hotkey interface vs. the current full-screen Action menu.
- Added `Security Groups` as a default column in the `VMs` selection view.
- Introduced `K9s`-style table hotkeys as a new UX paradigm to bypass full-screen Action menus, significantly speeding up workflows.
- Implemented `e` (Edit), `d` (Describe), `ctrl+d` (Delete), and `x` (Toggle Enable/Disable) natively on the `Firewall Rules` table rows, removing the need to press `Enter` to find these actions. Updated the `ShortHelp` text to advertise these new hotkeys directly to the operator.

## 2026-04-14 (Update 9)

### What Was Done
- UX update: Mapped standard K9s-style hotkeys (`d`, `ctrl+d`, `s`) directly to the VM table rows to bypass the full-screen Action menu for common operations.
- Appended `Security Groups` to the list of `DefaultVMColumns` per user request, and mapped it to the `preferredWidths` logic so that security groups render optimally on the VM tables.

## 2026-04-14 (Update 10)

### What Was Done
- UX update: Addressed the issue where full-screen `Action` menus (opened via `Enter`) would unnecessarily paginate after 4 items despite having plenty of available screen height.
- Implemented `ui.ActionListHeight()`, an intelligent helper that dynamically computes the exact minimum height required for a `bubbles/list` instance based on the total number of items, descriptions, and overhead padding, while restricting it to the maximum available terminal height.
- Hooked this dynamic sizing function into the `Resize` loops of all modules that utilize action menus (`VMs`, `Firewalls`, `Firewall Rules`, `Disks`, `Snapshots`, `Networks`, and `Clusters`), completely eliminating unnecessary pagination and utilizing the available terminal area effectively.

## 2026-04-14 (Update 11)

### What Was Done
- UX update: Column configuration menus were intercepting keys but failing to dispatch them down to the underlying `bubbles/list` model if filtering was enabled. Appended `SetFilteringEnabled(false)` to all `columnConfigList` and `sortList` instantiations globally so spacebar toggling and up/down navigation work as expected.
- UX update: Added context to the Actions menu so that users explicitly see what resource they are affecting. `list.Title` is now dynamically set to `Actions: <Resource Name>` upon pressing `Enter`. 
- Modified the Overlay rendering pipeline in `VMs`, `Firewalls`, and `Disks` tabs. Previously, opening the actions menu or a confirmation dialog replaced the entire screen background. They now use `lipgloss.Place` correctly to ensure the application's breadcrumb header and the table itself remain visible behind the overlay, maintaining deep UX context.

## 2026-04-14 (Update 12)

### What Was Done
- **UX Fix:** Addressed a critical table wrapping bug in the `bubbles/table` integration. When configuring multiple columns (e.g., adding `Security Groups` and `Cost Trend`), the table would wrap vertically instead of enforcing horizontal pagination.
- **Root Cause:** The `VisibleColumnsForWidth` function calculating how many columns to fit in the terminal viewport was missing the `overhead` cost of the table styling. `bubbles/table` explicitly adds `Padding(0, 1)` to all default cells, meaning every rendered column consumes an extra 2 characters. The UI was over-allocating width, causing `lipgloss` to soft-wrap the header rows to the next line.
- **Fix:** Updated the layout math in `internal/ui/table_helpers.go` to deduct `overheadPerCol = 2` during width assignment and expansion. Tables now rigidly respect the maximum terminal width and enforce proper horizontal scrolling via the `h` / `l` keys.
- **Test:** Rewrote table bounds assertions in `table_helpers_test.go` to factor in padding overheads natively.

## 2026-04-14 (Update 13)

### What Was Done
- **UX Redesign:** Completely overhauled how the Action Menu and Confirmation overlays are rendered in the VMs tab. Previously, pressing `Enter` to open an action menu would completely replace the main table rendering with a blank background and a floating list, losing all context of what row was selected.
- Implemented a "Responsive Sidebar Split" UX. When `Enter` (Menu) or `ctrl+d` (Terminate) is pressed, the application dynamically triggers a table resize event (`v.refreshTable()`). It forces the `bubbles/table` model to rigidly shrink horizontally (down to 40 columns min) and cleanly truncates columns using horizontal pagination logic.
- Using `lipgloss.JoinHorizontal`, the `v.actions` or `confirmView` is immediately drawn inline to the right of the shrunken table. The selected row remains highlighted, giving perfect visual context to what instance the operator is interacting with.

## 2026-04-14 (Update 14)

### What Was Done
- **UX Innovation:** Steampipe was evaluated and discarded due to its massive architectural burden (PostgreSQL requirement), but the core UX value of its keyboard-driven workflow was adopted. 
- Implemented a native `K9s`-style global Command Bar in the TUI to dramatically increase power-user navigation speed.
- Users can now press `:` at any point in the application to drop a text input bar from the top of the terminal screen.
- Supported syntax allows for instant module switching (`:vms`, `:disks`, `:fw`, `:clusters`, `:dbs`, `:nets`) without needing to remember numerical tab bindings.
- Supported syntax allows for instant cross-cloud context jumping via `:ctx <query>`, where users can type fragments of their account ID, profile name, or region to instantly teleport their active session (e.g. `:ctx prod`, `:ctx us-east-1`).

## 2026-04-14 (Update 15)

### What Was Done
- **UX Request:** The `Cost` action for VMs previously just dumped a block of CLI commands (e.g., `aws ce get-cost-and-usage ...`) into a read-only view, forcing the user to copy-paste it into their own terminal.
- Replaced the `costGuide` static rendering logic with `executeCostCommandCmd`, which initiates a background subprocess to directly execute the cloud provider's native billing CLI commands against the selected instance.
- The `c` hotkey and the `Cost` action menu item now automatically fetch the cost report and render the STDOUT response dynamically inside the application's Describe pane.

## 2026-04-14 (Update 16)

### What Was Done
- **UX Audit:** Audited the remaining codebase for static "guide" or copy/paste instructions. 
- Found that the `s` (SSH) hotkey added in a previous commit was mistakenly wired to run `SSH` as a background `ExecuteActionCmd` instead of utilizing `tea.ExecProcess`, which would have prevented the terminal handoff required for an interactive SSH session. Re-wired the `s` hotkey so it successfully yields the terminal TTY to the SSH client.
- Found the static `sshRemediationGuide`, which instructs the user to run CLI commands to create a firewall rule if an SSH connection fails (e.g., due to missing IAP rules on GCP). Left this intentionally as a static guide, as dynamically executing security group / IAM creation in the background after a failure is an unsafe anti-pattern.

## 2026-04-14 (Update 17)

### What Was Done
- **Sprint 4 (FinOps Intelligence):** Enhanced the `Gemini FinOps` analysis engine integration in `internal/views/vms/view.go`. 
- Updated the AI prompt to actively instruct Gemini to act as a DevOps architect issuing CLI commands. The prompt now requires Gemini to provide explicit, copy-pasteable CLI execution strategies (e.g., `aws ec2 modify-instance-attribute`, `gcloud compute instances set-machine-type`) to apply its rightsizing recommendations.
- Updated the AI prompt to support Markdown, enabling bolding, headers, and code blocks for clearer readability within the UI's `Describe` pane.
- Upgraded the underlying `FetchVMCostSDK` for GCP (`internal/providers/gcp/billing.go`) to attempt a targeted BigQuery cost query for the specific compute instance instead of returning a hardcoded zero.

## 2026-04-14 (Update 18)

### What Was Done
- **Bug Fix:** Fixed an issue where GCP VMs were not rendering for users running in `SDK` mode without Google Application Default Credentials (ADC) configured.
- The GCP Go SDK strictly requires ADC (`gcloud auth application-default login`). Unlike other resources (Disks, Snapshots, Clusters) which properly caught the SDK auth error and fell back to executing the CLI (`gcloud compute ...`), the `FetchVMsSDK` function was missing its `SDKWithCLIAuthFallback` wrapper. 
- Implemented the wrapper in `internal/providers/gcp/client.go` and updated `internal/providers/registry.go` to ensure GCP VM queries gracefully fall back to the CLI when ADC is missing.

## 2026-04-14 (Update 19)

### What Was Done
- **Feature Request:** Added functionality to create completely new firewall rules from scratch across AWS, GCP, and Azure directly from the TUI.
- Users can now press `a` while viewing a Security Group's rules to open the `➕ ADD FIREWALL RULE` form.
- The 5-field form captures `Direction`, `Action`, `Protocol`, `Ports`, and `IP/CIDR`.
- **SDK Execution:** 
  - AWS: Submits a new `AuthorizeSecurityGroupIngress/Egress` payload.
  - GCP: Auto-generates a unique rule name (`fw-rule-[timestamp]`), binds it to the global network URL matching the parent group, and calls `compute.Service.Firewalls.Insert`.
  - Azure: Auto-generates a unique rule name (`rule-[timestamp]`), defaults priority to 1000 to ensure evaluation, and calls `BeginCreateOrUpdate`.
- Added corresponding `AUDIT` logging entries for all new rule creations.

## 2026-04-14 (Update 20)

### What Was Done
- **UX Consistency:** The `Command Bar` (`:`) and `Firewall Edit/Add Forms` previously used the legacy overlay engine, which erased the table context behind a blank background. 
- Overhauled the Command Bar to render strictly inside the Header/Breadcrumb row layout component (`app.go`), retaining full table visibility while active.
- Overhauled the `paneEditRule` and `paneAddRule` in `internal/views/firewalls/rules_view.go` to use the `lipgloss.JoinHorizontal` responsive split engine. The forms now render natively on the right sidebar while gracefully shrinking and truncating the Firewall Rules table on the left, keeping context visually intact.

## 2026-04-14 (Update 21)

### What Was Done
- **UX Fix:** Fixed silent error failures when triggering the FinOps recommendation engine without a valid `GEMINI_API_KEY`.
- Previously, a missing API key would only log an error to the bottom status bar, leaving the user staring at an unchanged table. The application now gracefully intercepts the error, transitions to the `Describe` pane, and provides a beautifully formatted Markdown overlay with exact instructions on how to acquire and configure the Gemini API key.
- **UX Addition:** Discovered that the `FinOps` action lacked a top-level hotkey. Added `f` to the VMs table key bindings so users can instantly trigger Gemini AI FinOps recommendations without opening the action menu. Updated the `ShortHelp` bar to advertise this new shortcut.

## 2026-04-18

### User Request Handled

- Fixed the firewall add/edit form flow so pressing `Enter` submits as expected instead of only working on the final field.

### Key Code And UI Changes

1. Updated `internal/views/firewalls/rules_view.go`.
   - `Enter` now advances to the next field when the form cursor is not on the last input.
   - `Enter` still submits the add/edit form from the final field.
   - Added small focus helpers so add/edit forms share the same navigation behavior.

2. Added regression coverage in `internal/views/firewalls/rules_view_test.go`.
   - Added tests for Enter advancing focus on add/edit forms.
   - Added tests for Enter submitting add/edit forms from the final field.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/views/firewalls`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

- Tab and Shift+Tab remain the primary explicit field-navigation keys, but Enter now works as a forward form action.

## 2026-04-18 (Update)

### User Request Handled

- Made firewall rule actions context-aware and blocked firewall mutations in CLI mode with a clear SDK-mode hint.

### Key Code And UI Changes

1. Updated firewall rule action generation in `internal/core/firewall_rule.go`.
   - `Enable` and `Disable` are now only included for GCP firewall rules.
   - AWS and Azure rule menus no longer show GCP-only toggle actions.

2. Updated `internal/views/firewalls/rules_view.go`.
   - Firewalls now build action menus from the active provider context.
   - CLI mode now blocks firewall mutations and opens a warning pane instead of opening the edit/add flow.
   - Added info/warn/error logging around firewall action attempts and blocked CLI mutations.

3. Updated `internal/providers/cli.go`.
   - CLI-backed firewall mutation calls now return a direct SDK-mode error instead of attempting a mutation path.

4. Added regression coverage in `internal/views/firewalls/rules_view_test.go`.
   - Verified AWS no longer receives GCP-only toggle actions.
   - Verified CLI-mode firewall mutation attempts are blocked with an SDK-mode warning.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/views/firewalls ./internal/providers`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

- Firewall edits still use the existing multi-field rule form in SDK mode. If you want true inline single-field editing, that is a separate UX change.

## 2026-04-18 (Update 2)

### User Request Handled

- Replaced the firewall rule edit form with a k9s-style inline quick-edit flow.

### Key Code And UI Changes

1. Updated `internal/views/firewalls/rules_view.go`.
   - Pressing `e` now opens a single-field inline editor instead of a multi-input form.
   - `Tab` / `Shift+Tab` cycle editable fields.
   - `Enter` saves the currently selected field immediately.
   - The edit pane now renders as `QUICK EDIT FIREWALL RULE` and shows the current field plus its current value.
   - Added field-aware logging so action attempts include the specific field being edited.

2. Updated `internal/providers/gcp/firewalls_edit.go`.
   - GCP firewall edits now patch `Description` as well as the existing rule fields.

3. Updated `internal/views/firewalls/rules_view_test.go`.
   - Added coverage for opening the inline editor from `e`.
   - Added coverage for cycling quick-edit fields and submitting a single-field edit.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/views/firewalls ./internal/providers/gcp`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

- The quick-edit flow currently exposes the most practical rule fields inline. If you want direction or other provider-specific metadata editable in the same pane, that can be added next.

## 2026-04-18 (Update 3)

### User Request Handled

- Removed the edit-pane form and changed firewall rule editing to a true inline quick-edit bar.

### Key Code And UI Changes

1. Updated `internal/views/firewalls/rules_view.go`.
   - The edit pane now renders as a simple inline bar below the table instead of a boxed form.
   - `e` opens inline edit mode on one field at a time.
   - `Tab` / `Shift+Tab` cycle fields, and `Enter` saves the current field directly.
   - The table stays full-width while editing instead of shrinking into a form split.

2. Kept add-rule behavior separate.
   - The `Add` flow still uses the existing multi-field form.
   - Only the edit path was converted to inline editing.

3. Updated tests in `internal/views/firewalls/rules_view_test.go`.
   - Verified the inline edit bar opens from `e`.
   - Verified field cycling and inline submit behavior.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/views/firewalls`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

- Inline edit currently targets a small set of practical firewall fields. If you want more provider-specific fields exposed inline, that can be extended per provider.

## 2026-04-19

### User Request Handled

- Tightened firewall editing so the edit flow is a true in-place inline prompt instead of a form-style pane.

### Key Code And UI Changes

1. Updated `internal/views/firewalls/rules_view.go`.
   - The edit state now renders as a minimal inline prompt line beneath the table.
   - No boxed edit form or split-pane editor is used for rule edits.
   - `Tab` / `Shift+Tab` still cycle fields, and `Enter` saves the current field immediately.

2. Kept add-rule behavior unchanged.
   - The add-rule workflow still uses the existing multi-field form.

3. Expanded and adjusted coverage in `internal/views/firewalls/rules_view_test.go`.
   - Verified the inline edit prompt is shown.
   - Verified add-rule submit behavior.
   - Verified edit-rule field cycling and submit behavior.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/views/firewalls`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

- The inline editor currently edits one field at a time and is optimized for the most common firewall properties. Provider-specific field expansion can be added later if needed.

## 2026-04-19 (Update)

### User Request Handled

- Investigated why firewall submit appeared to do nothing and added execution-level tests for add/edit submits.

### Key Code And UI Changes

1. Updated `internal/views/firewalls/rules_view.go`.
   - Edit hotkey setup now returns the inline editor initialization command instead of dropping it.
   - Added explicit submit/open logging for firewall add and edit actions.
   - Routed firewall mutation execution through a testable provider lookup hook so submit tests can exercise the real command path.

2. Updated `internal/providers/mock.go`.
   - Added `ExecuteFirewallActionFn` to the mock provider so tests can verify actual add/edit execution without cloud side effects.

3. Expanded `internal/views/firewalls/rules_view_test.go`.
   - Added execution-level add submit coverage that invokes the returned command and confirms the mock provider receives the mutation.
   - Added execution-level edit submit coverage with the same end-to-end command invocation.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/views/firewalls`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

- If the live app still shows no change after submit, the remaining likely causes are backend auth/permissions or provider-side API errors, which should now surface in the log file and mutation completion path.

## 2026-04-20

### User Request Handled

- Moved firewall editing fully into the highlighted table row so the active field is edited in place instead of in any separate prompt area.

### Key Code And UI Changes

1. Updated `internal/views/firewalls/rules_view.go`.
   - The edit state now renders the selected row with the active field value replaced by the live inline input.
   - The edit pane no longer renders a separate bottom editor bar.
   - Typing updates the selected row immediately so the operator edits directly in the table.
   - Submit logging was kept in place for add and edit flows.

2. Expanded `internal/views/firewalls/rules_view_test.go`.
   - Updated inline-edit coverage to assert the active value appears in the rendered table row.
   - Kept add and edit execution tests that invoke the returned submit command.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/views/firewalls`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

- This is still row-inline, not a true per-cell cursor model. If you want actual arrow-key cell navigation, that would require a deeper table interaction refactor.

## 2026-04-20 (Update)

### User Request Handled

- Fixed the inline firewall editor so typed text is committed into the rule model immediately, not only on submit.

### Key Code And UI Changes

1. Updated `internal/views/firewalls/rules_view.go`.
   - Inline edit now commits the current cell value into `pendingRule` on every keystroke.
   - Switching fields preserves unsaved edits instead of dropping them.
   - Enter still submits the currently highlighted field, but the underlying rule state is already current before submit.

2. Expanded `internal/views/firewalls/rules_view_test.go`.
   - Added coverage to verify typing updates the inline cell value immediately.
   - Added coverage to verify tabbing away from a field keeps the typed value.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/views/firewalls`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

- The inline editor is still row-based rather than a true table-cell cursor implementation with left/right cell focus.

## 2026-04-20 (Update 2)

### User Request Handled

- Added true inline cell navigation for firewall rule edits so the active field can be moved with left/right arrows, not just Tab.

### Key Code And UI Changes

1. Updated `internal/views/firewalls/rules_view.go`.
   - Edit mode now treats `←/→` as field navigation between editable columns.
   - `Tab` still advances and `Shift+Tab` still moves backward, but the row editor now matches the requested left/right cell behavior.
   - Updated the short help text to advertise inline edit field navigation.

2. Expanded `internal/views/firewalls/rules_view_test.go`.
   - Added coverage for `←/→` moving between fields.
   - Kept the inline typing test to verify edits persist while moving between fields.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/views/firewalls`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

- This is still one-row-at-a-time inline editing. A full spreadsheet-style cursor inside table cells would be a larger interaction refactor.

## 2026-04-20 (Update 3)

### User Request Handled

- Fixed the firewall inline edit rendering bug where embedding the full text input widget inside the table was corrupting the row layout.

### Key Code And UI Changes

1. Updated `internal/views/firewalls/rules_view.go`.
   - Replaced `textinput.View()` rendering inside the table cell with a plain inline cell renderer.
   - The active cell now shows the current text plus a small cursor marker, which keeps table layout stable while preserving inline edit feedback.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/views/firewalls`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

- The editor is still inline-row based rather than a true cell widget with per-character cursor rendering inside the table cell.

## 2026-04-20 (Update 5)

### User Request Handled

- Replaced firewall rule edit mode with a raw JSON editor that marshals the normalized firewall rule payload the SDK mutation path already consumes.

### Key Code And UI Changes

1. Updated `internal/views/firewalls/rules_view.go`.
   - Edit mode now opens a multiline JSON textarea instead of field-by-field inline editing.
   - The editor is seeded from the selected rule as pretty-printed JSON.
   - `Ctrl+S` parses the JSON and submits it through the existing provider mutation command.
   - Invalid JSON now stays in the editor and surfaces an error message instead of silently failing.

2. Updated `internal/views/firewalls/rules_view_test.go`.
   - Added coverage for opening the JSON editor.
   - Added coverage for JSON submit executing the provider.
   - Added coverage for invalid JSON blocking submit.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/views/firewalls`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

- JSON submit currently uses `Ctrl+S`; if you want a different save key or a dedicated JSON schema/validation pass, that can be added next.

## 2026-04-20 (Update 4)

### User Request Handled

- Fixed the inline firewall cursor behavior so left/right movement can stay inside the active field and only changes fields at the edges.

### Key Code And UI Changes

1. Updated `internal/views/firewalls/rules_view.go`.
   - Removed the accidental overlap where `right` was being treated as a field-switch key before cursor movement ran.
   - The edit mode now uses `left`/`right` for character-level cursor motion within the current field.
   - The same keys only switch fields when the cursor is already at the start or end of the current value.
   - Added per-field cursor position tracking so the inline editor restores cursor location when returning to a field.

2. Updated `internal/views/firewalls/rules_view_test.go`.
   - Added coverage that right-arrow moves the cursor inside the current field without switching fields.
   - Kept coverage for switching to adjacent fields when the cursor is at a boundary.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/views/firewalls -run TestEditRuleArrowKeysMoveCursorWithinField -v`
- `GOCACHE=/tmp/go-build-cache go test ./internal/views/firewalls`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

- The editor is still an inline row editor rather than a full spreadsheet-style per-cell widget with independent column focus state outside the rule edit flow.

## 2026-04-20 (Update 6)

### User Request Handled

- Fixed the firewall JSON edit save flow so it uses a terminal-safe submit key and is easier to complete in a live TUI session.

### Key Code And UI Changes

1. Updated `internal/views/firewalls/rules_view.go`.
   - JSON edit submit now accepts `F2` as the primary save key, with `Ctrl+S` kept as a secondary path.
   - Updated the JSON editor footer to advertise `F2/Ctrl+S` instead of only `Ctrl+S`.
   - Added stronger submit-path logging around the existing edit action flow.

2. Updated `internal/views/firewalls/rules_view_test.go`.
   - Switched the JSON submit tests to use `F2` so the coverage matches the terminal-safe save path.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/views/firewalls`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

- If you want a one-key submit that is even more discoverable, we can add an explicit "Save" action in the edit pane alongside the `F2` shortcut.

## 2026-04-22

### User Request Handled

- Fixed Codex MCP startup failures for `fetch` and `desktop-commander`.

### Key Code And Runtime Changes

1. Repaired Codex MCP server configuration in `/Users/ranger/.codex/config.toml`.
   - Replaced the broken `fetch` npm entry (`@modelcontextprotocol/server-fetch`, which now 404s) with a local Python runtime:
     - command: `/Users/ranger/CLOUDMANAGER/.codex-mcp/fetch-venv/bin/python`
     - args: `["-m", "mcp_server_fetch"]`
   - Replaced the broken cached `desktop-commander` `npx` entry with a clean repo-local install:
     - command: `node`
     - args: `["/Users/ranger/CLOUDMANAGER/.codex-mcp/desktop-commander/node_modules/@wonderwhy-er/desktop-commander/dist/index.js"]`

2. Added local MCP runtimes under `/Users/ranger/CLOUDMANAGER/.codex-mcp/`.
   - `desktop-commander/` contains a clean npm install of `@wonderwhy-er/desktop-commander@0.2.38`.
   - `fetch-venv/` contains a Python virtualenv with `mcp-server-fetch`.

3. Backed up the prior Codex config before editing.
   - Backup file: `/Users/ranger/.codex/config.toml.bak-mcp-fix-20260422`

### Validation Performed

- Executed raw MCP stdio `initialize` handshakes against both repaired runtimes.
- Verified `desktop-commander` initialize response:
  - `name: desktop-commander`
  - `version: 0.2.38`
- Verified `fetch` initialize response:
  - `name: mcp-fetch`
  - `version: 1.27.0`

### Remaining Risks Or Follow-Up

1. Codex needs a restart or MCP server reload to pick up the new `~/.codex/config.toml` entries if the current app process still has the old startup state cached.
2. The repaired MCP runtimes now live inside the repo at `.codex-mcp/`; if that directory is deleted, the Codex MCP config will need to be updated again or the runtimes reinstalled.

## 2026-04-29 (Manual Hosts)

### User Request Handled

- Added a practical path for unmanaged/third-party VMs where the user has IP/DNS, username, and SSH key or SSH config, but does not want provider API credentials.

### Key Code And UI Changes

1. Added manual host config and indexing.
   - `manual_hosts` now persists in `~/.cloudmanager.json`.
   - Manual hosts create a synthetic `Manual / manual-hosts / global` context.
   - Manual hosts are projected into the local VM search index so global search can find them by name, IP, username, tags, or ID.

2. Added the Hosts resource view.
   - Registered tab `8` as `Hosts`.
   - Added `:add-host`, `:host-add`, and `:manual-host`.
   - Settings now includes `Add manual host`.
   - The Hosts view supports `Enter`/`s` to launch SSH and `c` to copy only the SSH command.

3. Added manual SSH access resolution.
   - `ssh_config_host` produces `ssh <alias>`.
   - `key_path` produces `ssh -i <key> user@host`.
   - Non-SSH manual connections are stored as metadata but are not runnable yet.

4. Fixed global-search drop behavior for manual hosts.
   - Selecting a manual host result opens the Hosts tab instead of the VM tab.
   - The Hosts view receives a search filter so the selected host is narrowed immediately.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/config ./internal/access ./internal/hosts ./internal/views/hosts ./internal/ui`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

- RDP is metadata-only today; no runnable RDP launcher has been added.
- `key_ref` and `password_ref` are placeholders for a later keychain/vault-backed secret resolver.
- Edit/remove flows for manual hosts are not implemented yet.
- Manual hosts currently reuse the VM search index shape; a future global search model should support typed resources directly.

## 2026-04-29 (CloudManager Tags)

### User Request Handled

- Added CloudManager-only tags so the same logical tag, for example `VFWEB`, can be applied across providers, accounts, projects, regions, and manual hosts without mutating provider metadata.

### Key Code And UI Changes

1. Added `resource_tags` config support.
   - Tags can target a VM by provider/account/region plus resource ID, name, private IP, or public IP.
   - Tags are stored only in CloudManager config.
   - Provider tags, GCP labels, and instance metadata are not modified.

2. Added VM table tagging UX.
   - Select a VM and press `t`.
   - Enter comma-separated tags such as `VFWEB,prod`.
   - CloudManager backs up config before saving.

3. Applied tag overlays during indexing and rendering.
   - Tags render into VM labels as `cm:<tag>`, for example `cm:VFWEB`.
   - Global search can match these local tags.
   - Cached VM index records get the overlay reapplied at load time.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/config ./internal/ui ./internal/views/vms`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

- Tag remove/edit UI is not implemented yet.
- Tags currently target VMs only; disks, databases, clusters, and firewalls can use the same model later.

## 2026-04-29 (Provider Authoring)

### User Request Handled

- Clarified where community providers should be added, using an OVHcloud-style provider as the example, while keeping CloudManager lightweight and portable.

### Key Code And Docs Changes

1. Updated `docs/PROVIDER_GUIDE.md`.
   - Added provider tiers: manual hosts, CLI provider, SDK provider, external provider process.
   - Documented where a new provider belongs: `internal/providers/<provider>/`.
   - Added an OVHcloud-style registration example.
   - Expanded the current capability list to include hosts, clusters, databases, and metrics.

2. Added lightweight-binary guidance.
   - Default binary should avoid pulling every long-tail provider SDK.
   - New providers should start CLI-only or manual when possible.
   - Large SDKs should move behind optional builds or a future external provider process.

3. Added global access/configure UX target.
   - Current surfaces: `g`, `,`, `:`, `:login`, `:add-provider`, `:add-host`.
   - Target is one command palette that exposes configured resources, missing setup, manual hosts, login, tags, indexing, settings, and logs.

### Validation Performed

- Documentation-only change; no test run required.

### Remaining Risks Or Follow-Up

- External provider process protocol is not implemented yet.
- The default binary still imports built-in provider packages directly; slimming long-tail SDKs needs build-tag or sidecar work.

## 2026-04-29 (SDK Update Policy)

### User Request Handled

- Added explicit guidance for updating current cloud SDK dependencies.

### Key Docs Changes

1. Updated `docs/PROVIDER_GUIDE.md`.
   - Added `SDK Update Policy`.
   - Requires scoped, reversible upgrades.
   - Documents provider-specific cautions for AWS SDK v2, Azure ARM packages, and GCP mixed SDK/API clients.
   - Defines required test matrix for SDK upgrades.
   - Calls out auth fallback, normalized model mapping, unsupported capability behavior, and binary-size review.

### Validation Performed

- Documentation-only change; no test run required.

### Remaining Risks Or Follow-Up

- No automated dependency-update bot or SDK-upgrade CI matrix exists yet.

## 2026-04-29 (Manual Host UX Fix)

### User Request Handled

- Fixed manual host add/save ergonomics and simplified the Manual context tree label.

### Key Code And UI Changes

1. Manual host form save keys.
   - Save now accepts `Enter`, `Ctrl+J`, `Ctrl+M`, and `F2`.
   - Footer now advertises `Enter/F2: Save`.

2. Manual context tree label.
   - Manual provider now renders as `Manual > Hosts`.
   - Removed the confusing `Manual > Manual Hosts > Manual Hosts` nesting.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/ui ./internal/providers ./internal/config`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

- If a terminal still sends an unusual key sequence for Return, `F2` is now the explicit fallback save key.

## 2026-04-29 (Manual Host Removal)

### User Request Handled

- Added an in-app way to remove manual hosts.

### Key Code And UI Changes

1. Hosts view removal.
   - Press `d` on a selected manual host to remove it.
   - CloudManager backs up config before saving.
   - Hosts table refreshes immediately.

2. Shell/index refresh.
   - Manual host removal emits a shell refresh message.
   - Context tree reloads from config.
   - Manual host records are pruned from the local global-search index before reindexing remaining manual hosts.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/views/hosts ./internal/ui`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

- There is no confirmation prompt yet; removal is immediate after `d`, but config is backed up first.

## 2026-04-29 (Manual Host Remove Confirmation)

### User Request Handled

- Added confirmation before removing a manual host.

### Key Code And UI Changes

1. Hosts view removal flow.
   - `d` now opens a confirmation prompt.
   - `Enter` or `y` confirms removal.
   - `Esc` or `n` cancels.

2. Added tests for confirm and cancel behavior.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/views/hosts ./internal/ui`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

- Confirmation is inline in the Hosts view, not a shared modal component yet.

## 2026-05-11 (Access Method Cleanup)

### User Request Handled

- Cleaned up noisy VM access methods after the access picker showed learned SSH, weak SSH config aliases, direct default SSH, private-key picker, and remediation together.

### Key Code And UI Changes

1. Access method pruning.
   - Added `access.PruneFallbacks`.
   - Hides direct `ssh <ip>` fallback by default.
   - Hides SSH config and private-key fallback when a learned SSH method exists.
   - Hides remediation when any runnable or selectable access method exists.
   - Fallbacks can still be shown with `CLOUDMANAGER_SHOW_ACCESS_FALLBACKS=1`.

2. SSH config matching.
   - Skips SSH config aliases that are literal IP addresses.
   - Keeps friendly aliases that point to a VM IP through `HostName`.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/access ./internal/views/vms ./internal/localdb`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

- Fallback visibility is currently controlled by env var, not yet by a TUI setting.

## 2026-05-12 (GCP VM Describe Fix)

### User Request Handled

- Fixed GCP VM describe failures when the selected VM record had an empty or full-URL zone.

### Key Code And UI Changes

1. GCP zone resolution.
   - Normalizes GCP zone values before VM actions and SSH commands.
   - If a VM has no zone, CloudManager now looks up the VM in the project and uses the discovered zone.

2. GCP SDK describe output.
   - SDK describe now returns the full instance JSON instead of a short hand-written summary.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/providers/gcp ./internal/providers ./internal/views/vms`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

- If a VM is selected from stale cache and no longer exists in GCP, describe now fails with an explicit "instance not found in project" zone-discovery error.

## 2026-05-12 (Live GCP Describe Sweep)

### User Request Handled

- Tested GCP VM describe across configured CloudManager GCP contexts and checked real describe output.

### Key Code And UI Changes

1. Added skipped-by-default live diagnostic test.
   - `CLOUDMANAGER_LIVE_GCP_DESCRIBE=1 go test -v ./internal/providers/gcp -run TestLiveDescribeConfiguredGCPContexts -count=1`
   - Lists one VM per configured GCP context, then validates both CLI and SDK describe output.

2. Improved GCP CLI VM-list errors.
   - `FetchVMsCLI` now uses `CombinedOutput`.
   - CloudManager now surfaces the actual gcloud error text, including `SERVICE_DISABLED`.
   - Added `--quiet` to GCP compute actions to avoid interactive prompts in the TUI path.

### Validation Performed

- Live describe sweep:
  - Passed CLI and SDK describe for every configured GCP context with Compute enabled and at least one VM.
  - Skipped contexts with no VMs: `firecompass-meun`, `firecompass-uat`.
  - Failed before describe because Compute Engine API is disabled: `firecompass-qa-meun`, `fireshadow`, `gen-lang-client-0618559363`, `sys-74378647694670153862756860`.
- `GOCACHE=/tmp/go-build-cache go test ./internal/providers/gcp ./internal/providers ./internal/views/vms`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

- Configured GCP projects with disabled Compute API will still fail VM listing, but the UI now has the real provider error instead of only `exit status 1`.

## 2026-05-12 (All-Context Describe Sweep)

### User Request Handled

- Checked VM Describe across all configured CloudManager cloud contexts and both CLI/SDK backends where supported.
- Rechecked local Describe panes for VMs, disks, snapshots, firewalls, networks, databases, and storage.

### Key Code And UI Changes

1. Added skipped-by-default all-provider live diagnostic test.
   - `CLOUDMANAGER_LIVE_DESCRIBE=1 go test -v ./internal/providers -run TestLiveVMDescribeAllConfiguredContexts -count=1`
   - Walks configured contexts, lists VMs, describes one VM, and verifies non-empty output containing the VM name or ID.

2. Fixed AWS SDK Describe panics.
   - AWS SDK instance describe is now nil-safe for missing instance state, placement, IAM profile, EBS block mapping, and network association fields.

3. Improved Azure CLI VM-list errors.
   - Azure VM listing now captures stderr with `CombinedOutput`, so revoked-token errors are visible instead of only `exit status 1`.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/providers/aws ./internal/providers/azure ./internal/providers ./internal/views/vms ./internal/views/disks ./internal/views/snapshots ./internal/views/firewalls ./internal/views/networks ./internal/views/databases ./internal/views/storage`
- `GOCACHE=/tmp/go-build-cache go test ./...`
- Live all-context Describe sweep:
  - AWS CLI and SDK Describe passed across all configured AWS contexts/regions.
  - GCP CLI and SDK Describe passed for contexts with Compute enabled and VMs.
  - GCP contexts with no VMs skipped: `firecompass-meun`, `firecompass-uat`.
  - GCP contexts blocked before Describe because Compute API is disabled: `firecompass-qa-meun`, `fireshadow`, `gen-lang-client-0618559363`, `sys-74378647694670153862756860`.
  - Azure CLI and SDK blocked before Describe because the active Azure CLI grant is revoked/expired (`AADSTS50173`).

### Remaining Risks Or Follow-Up

- Azure Describe cannot be verified until the user refreshes Azure auth with `az logout` and `az login`.
- Disabled GCP Compute projects will continue to fail VM listing until Compute Engine API is enabled or those projects are removed from CloudManager contexts.

## 2026-05-12 (Copy Details Shortcut)

### User Request Handled

- Added a `c` / `C` clipboard shortcut for detail panes so users can copy clean text instead of terminal-rendered borders.

### Key Code And UI Changes

1. Added copyable detail state to disks, snapshots, firewalls, firewall rules, and networks.
   - Describe/detail content is stored as plain text and copied through the shared clipboard helper.
   - Disk and snapshot describe panes now show `c copy` in the status hint.

2. Made existing copy shortcuts accept uppercase `C`.
   - Updated VM access/describe copy paths, database/storage detail copy, and manual host command copy.

3. Added focused key-handler coverage.
   - Tests assert that `C` produces a copy command for disk, snapshot, security-group, firewall-rule, and network detail panes.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/views/disks ./internal/views/snapshots ./internal/views/firewalls ./internal/views/networks ./internal/views/databases ./internal/views/storage ./internal/views/vms ./internal/views/hosts`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

- Firewalls and networks copy silently today because those views do not have a status line; a small toast/status abstraction would make copy feedback consistent across every pane.

## 2026-05-12 (Provider Console Drill-Down)

### User Request Handled

- Added a browser drill-down path from CloudManager resources to provider console pages.

### Key Code And UI Changes

1. Added provider console URL builders.
   - Supports VMs, disks, snapshots, networks, subnets, security groups, firewall rules, databases, and storage where CloudManager has enough provider identity.
   - Covers AWS, GCP, and Azure resource URL patterns.

2. Added browser-open command support.
   - New `internal/browseropen` helper uses `open`, `xdg-open`, or Windows URL handler.
   - New `ui.OpenURLCmd` wraps browser launch as a Bubble Tea command.

3. Added TUI drill-down affordances.
   - Detail panes now include a visible `Console:` URL when available.
   - Press `o` / `O` from detail panes to open the provider console.
   - Resource action menus now include `Open Console` where applicable.

### Validation Performed

- `GOCACHE=/tmp/go-build-cache go test ./internal/core ./internal/views/vms ./internal/views/disks ./internal/views/snapshots ./internal/views/firewalls ./internal/views/networks ./internal/views/databases ./internal/views/storage`
- `GOCACHE=/tmp/go-build-cache go test ./...`

### Remaining Risks Or Follow-Up

- Terminal-native clickable buttons are not portable; the reliable UX is visible URL plus `o` to open.
- Some provider URL formats may need refinement as we see real-world console routes, especially GCP subnet/firewall deep links and AWS console fragments.

## 2026-05-12 (Prerequisite Installer)

### User Request Handled

- Added an installation script to check, install, and update CloudManager prerequisites and provider CLIs.

### Key Code And UI Changes

1. Added `scripts/install-prereqs`.
   - Supports `--check`, `--install`, and `--update`.
   - Supports selection flags: `--core`, `--cloud`, `--k8s`, `--all`, `--aws`, `--gcp`, `--azure`, `--doctl`, `--kubectl`.
   - Checks Git, Go, OpenSSH, curl, AWS CLI, AWS SSM Session Manager plugin, Google Cloud CLI, Azure CLI, DigitalOcean CLI, and kubectl.

2. Installer behavior.
   - Homebrew support is first-class on macOS.
   - Linux `apt` support is best-effort for safe/common packages.
   - The script does not run cloud login flows; authentication remains in CloudManager `:login` or native CLIs.

3. README updated.
   - Added prerequisite installer usage examples.

### Validation Performed

- `bash -n scripts/install-prereqs`
- `scripts/install-prereqs --check --all`

### Remaining Risks Or Follow-Up

- Linux cloud CLI installs that require vendor package repositories are reported/skipped instead of mutating system package sources automatically.
