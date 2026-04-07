# LOG

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
