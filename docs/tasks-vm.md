# Tasks: VM & Compute Resources (Refined)

> **Scope:** Disks, Snapshots, VM Detail Drill-Down, Tab Bar. Images/AMIs cut from v1.
> **Owner:** Agent VM
> **Backend:** SDK only for new resources. CLI maintained only for existing VM operations.
> **Dependencies:** ARCH-1 (Provider interface refactor) must be done first.

---

## ARCH-1: Refactor Provider Interface (BLOCKING — DO FIRST)
**Priority:** P0 — Blocks everything else.
**Files:** `internal/providers/provider.go`, `cli.go`, `sdk.go`

### Description
The current `Provider` interface will bloat to 20+ methods. Refactor into small, composable interfaces using Go's type assertion pattern. This is the single most important architectural change.

### Acceptance Criteria
- [ ] **Keep** the existing `Provider` interface as-is (VMs only) — no breaking changes.
- [ ] Add new per-domain interfaces in `provider.go`:
  ```go
  type DiskProvider interface {
      FetchDisks(ctx context.Context, cloudCtx core.CloudContext) ([]core.Disk, error)
      ExecuteDiskAction(ctx context.Context, action string, disk core.Disk, cloudCtx core.CloudContext) (string, error)
  }
  type SnapshotProvider interface {
      FetchSnapshots(ctx context.Context, cloudCtx core.CloudContext) ([]core.Snapshot, error)
      ExecuteSnapshotAction(ctx context.Context, action string, snap core.Snapshot, cloudCtx core.CloudContext) (string, error)
  }
  type FirewallProvider interface {
      FetchSecurityGroups(ctx context.Context, cloudCtx core.CloudContext) ([]core.SecurityGroup, error)
      FetchFirewallRules(ctx context.Context, groupID string, cloudCtx core.CloudContext) ([]core.FirewallRule, error)
  }
  type NetworkProvider interface {
      FetchNetworks(ctx context.Context, cloudCtx core.CloudContext) ([]core.Network, error)
      FetchSubnets(ctx context.Context, cloudCtx core.CloudContext) ([]core.Subnet, error)
  }
  type MetricsProvider interface {
      FetchVMMetrics(ctx context.Context, vm core.VM, cloudCtx core.CloudContext, period time.Duration) (*core.VMMetrics, error)
  }
  type BillingProvider interface {
      FetchAccountCost(ctx context.Context, cloudCtx core.CloudContext) (*core.AccountCost, error)
      FetchResourceCost(ctx context.Context, resourceID string, cloudCtx core.CloudContext) (*core.ResourceCost, error)
      FetchRecommendations(ctx context.Context, cloudCtx core.CloudContext) ([]core.Recommendation, error)
  }
  ```
- [ ] `SDKProvider` implements ALL interfaces (incrementally, as SDK fetch functions are added).
- [ ] `CLIProvider` implements ONLY `Provider` (VMs). It does NOT implement the new interfaces.
- [ ] Views use type assertions:
  ```go
  provider := providers.GetProvider(cfg)
  if dp, ok := provider.(providers.DiskProvider); ok {
      disks, err := dp.FetchDisks(ctx, cloudCtx)
  } else {
      // Show "Switch to SDK backend for disk management"
  }
  ```
- [ ] Update `ARCHITECTURE.md` to document the new pattern.

---

## ARCH-2: Test Infrastructure
**Priority:** P0
**Files:** `internal/core/*_test.go`, `internal/providers/mock.go`

### Acceptance Criteria
- [ ] Create `internal/providers/mock.go` with `MockProvider` implementing all interfaces using function fields:
  ```go
  type MockProvider struct {
      FetchVMsFn    func(ctx, cloudCtx) ([]core.VM, error)
      FetchDisksFn  func(ctx, cloudCtx) ([]core.Disk, error)
      // ... one field per interface method
  }
  ```
- [ ] Create `internal/core/vm_test.go` — test `VM.GetField()` for all columns.
- [ ] Create `internal/core/disk_test.go` — test `Disk.GetField()`.
- [ ] Create `internal/core/snapshot_test.go` — test `Snapshot.GetField()`.
- [ ] Create `internal/ui/tree_test.go` — restore the tree tests that were deleted during refactor.

---

## VM-1: Disk Resource Model
**Priority:** P0
**File:** `internal/core/disk.go`

### Acceptance Criteria
- [ ] Define `Disk` struct:
  ```
  Name, ID, State, SizeGB (int), Type (SSD/HDD/etc), Zone, AttachedToVM,
  AttachedToVMID, IOPS (string), Throughput (string), Encrypted (bool),
  CreatedAt (string), ResourceGroup (string), Labels (string)
  ```
- [ ] Implement `Resource` interface: `GetID()`, `GetName()`, `GetKind() → "Disk"`, `GetField(col string)`.
- [ ] Define `DefaultDiskColumns`: `["Name", "ID", "State", "Size (GB)", "Type", "Attached To", "Zone", "Encrypted"]`.
- [ ] Define `DiskActions()`: `[Detach, Delete, Resize, Create Snapshot, Describe]`. `Delete` is dangerous.

---

## VM-2: Snapshot Resource Model
**Priority:** P0
**File:** `internal/core/snapshot.go`

### Acceptance Criteria
- [ ] Define `Snapshot` struct:
  ```
  Name, ID, State, SizeGB (int), SourceDiskName, SourceDiskID,
  CreatedAt, Zone, ResourceGroup, Description, Labels
  ```
- [ ] Implement `Resource` interface. `GetKind() → "Snapshot"`.
- [ ] Define `DefaultSnapshotColumns`: `["Name", "ID", "State", "Size (GB)", "Source Disk", "Created At"]`.
- [ ] Define `SnapshotActions()`: `[Create Disk, Delete, Describe]`. `Delete` is dangerous.

---

## VM-3: AWS Disk Fetching (SDK Only)
**Priority:** P0
**File:** `internal/providers/aws/disks.go`

### Acceptance Criteria
- [ ] Create `internal/providers/aws/disks.go`.
- [ ] `FetchDisksSDK(ctx, profile, region) ([]core.Disk, error)`:
  - Use `ec2.DescribeVolumes` with paginator.
  - Map: `VolumeId → ID`, `State`, `Size → SizeGB`, `VolumeType → Type`, `AvailabilityZone → Zone`, `Encrypted`, `Iops`, `Throughput`.
  - Extract `Attachments[0].InstanceId → AttachedToVMID`. Resolve VM name from tags if available.
  - Parse `Tags` into `Labels`, `CreateTime → CreatedAt`.
- [ ] Handle unattached volumes (`AttachedToVM = "-"`).

---

## VM-4: GCP Disk Fetching (SDK Only)
**Priority:** P0
**File:** `internal/providers/gcp/disks.go`

### Acceptance Criteria
- [ ] `FetchDisksSDK(ctx, project) ([]core.Disk, error)`:
  - Use `compute.DisksService.AggregatedList(project)` with pagination.
  - Map: `name`, `id`, `status → State`, `sizeGb → SizeGB`, `type → Type` (extract last URL segment), `zone` (extract last segment), `users[0] → AttachedToVM` (extract instance name from URL).
  - Parse `labels → Labels`, `creationTimestamp → CreatedAt`.
- [ ] Handle zonal vs regional disks.

---

## VM-5: Azure Disk Fetching (SDK Only)
**Priority:** P0
**File:** `internal/providers/azure/disks.go`

### Acceptance Criteria
- [ ] `FetchDisksSDK(ctx, subscription) ([]core.Disk, error)`:
  - Use `armcompute.DisksClient.NewListPager()`.
  - Map: `name`, `id`, `diskState → State`, `diskSizeGb → SizeGB`, `sku.name → Type`, `location → Zone`, `managedBy → AttachedToVM` (extract VM name from resource ID).
  - Extract `resourceGroup` from the `id` URL path.

---

## VM-6: AWS Snapshot Fetching (SDK Only)
**Priority:** P1
**File:** `internal/providers/aws/snapshots.go`

### Acceptance Criteria
- [ ] `FetchSnapshotsSDK(ctx, profile, region) ([]core.Snapshot, error)`:
  - Use `ec2.DescribeSnapshots` with `OwnerIds: ["self"]` and paginator.
  - Map: `SnapshotId → ID`, `State`, `VolumeSize → SizeGB`, `VolumeId → SourceDiskID`, `StartTime → CreatedAt`, `Description`.
  - Parse `Tags` for `Name` and `Labels`.

---

## VM-7: GCP Snapshot Fetching (SDK Only)
**Priority:** P1
**File:** `internal/providers/gcp/snapshots.go`

### Acceptance Criteria
- [ ] `FetchSnapshotsSDK(ctx, project) ([]core.Snapshot, error)`:
  - Use `compute.SnapshotsService.List(project)`.
  - Map: `name`, `id`, `status → State`, `diskSizeGb → SizeGB`, `sourceDisk → SourceDiskName` (parse URL), `creationTimestamp → CreatedAt`.

---

## VM-8: Azure Snapshot Fetching (SDK Only)
**Priority:** P1
**File:** `internal/providers/azure/snapshots.go`

### Acceptance Criteria
- [ ] `FetchSnapshotsSDK(ctx, subscription) ([]core.Snapshot, error)`:
  - Use `armcompute.SnapshotsClient.NewListPager()`.
  - Map: `name`, `id`, `provisioningState → State`, `diskSizeGb → SizeGB`, `creationData.sourceResourceId → SourceDiskID`, `timeCreated → CreatedAt`.

---

## VM-9: Disk Actions (SDK Only)
**Priority:** P1
**Files:** `internal/providers/aws/disks.go`, `gcp/disks.go`, `azure/disks.go`

### Acceptance Criteria
- [ ] **Detach:** AWS `ec2.DetachVolume`, GCP `Instances.DetachDisk`, Azure `VirtualMachinesClient.BeginUpdate` (remove from data disks).
- [ ] **Delete:** AWS `ec2.DeleteVolume` (must be detached), GCP `Disks.Delete`, Azure `DisksClient.BeginDelete`.
- [ ] **Resize:** AWS `ec2.ModifyVolume --size` (only increase), GCP `Disks.Resize`, Azure `DisksClient.BeginUpdate` (VM must be deallocated).
- [ ] **Create Snapshot:** AWS `ec2.CreateSnapshot`, GCP `Disks.CreateSnapshot`, Azure `SnapshotsClient.BeginCreateOrUpdate`.
- [ ] All destructive actions flow through the confirmation dialog (`Dangerous: true`).

---

## VM-10: Disks TUI View
**Priority:** P0
**File:** `internal/views/disks/view.go`

### Acceptance Criteria
- [ ] Implement `ui.View` interface: `Init`, `Update`, `Render`, `Title → "Disks"`, `ShortHelp`, `Resize`.
- [ ] Same pane pattern as VMsView: `paneTable`, `paneActions`, `paneDescribe`, `paneColumnConfig`, `paneSortConfig`, `paneConfirm`.
- [ ] Table columns driven by `core.DefaultDiskColumns` and `DiskColumns` in `AppConfig`.
- [ ] Support search, sort, column config (same patterns as VMsView).
- [ ] Actions from `core.DiskActions()`.
- [ ] Handle `Resize` action: prompt for new size via `textinput`, validate > current.
- [ ] If provider doesn't implement `DiskProvider`, show: *"Switch to SDK backend (press 'c') for disk management."*

---

## VM-11: Snapshots TUI View
**Priority:** P1
**File:** `internal/views/snapshots/view.go`

### Acceptance Criteria
- [ ] Same pattern as Disks view.
- [ ] Actions from `core.SnapshotActions()`.
- [ ] `Create Disk` action: prompt for disk name.

---

## VM-12: VM Detail Drill-Down View
**Priority:** P1
**File:** `internal/views/vms/detail.go`

### Acceptance Criteria
- [ ] Show a sectioned viewport when user selects "Describe":
  - **General:** Name, ID, Type, State, Zone, Labels.
  - **Network:** Each NIC with Private IP, Public IP, Network, Subnet.
  - **Disks:** Table of attached disks (fetch via `DiskProvider`, filter by `AttachedToVMID`).
  - **Tags/Labels:** Full key=value list.
  - **Metrics:** If available from `MetricsProvider` (added later).
- [ ] Selecting a disk and pressing Enter → `PushViewMsg` to Disks view filtered to that disk.
- [ ] `Esc` → back to table.

---

## VM-13: Resource Type Tab Bar
**Priority:** P0
**File:** `internal/ui/app.go`

### Acceptance Criteria
- [ ] Add `resourceViews map[string]ui.View` to `App` containing pre-initialized views (VMs, Disks, Snapshots, and later Networks, Firewalls).
- [ ] Render tab bar at top: `[1] VMs  [2] Disks  [3] Snapshots  [4] Networks  [5] Firewalls`.
- [ ] Number keys `1-5` switch the active view (replace `viewStack[0]`).
- [ ] Active tab highlighted with `ui.Highlight` color.
- [ ] On switch, call `Init()` with current `CloudContext`.
- [ ] Update `main.go` to register all views.
- [ ] If no context selected, show placeholder in all views.

---

## Dependency Graph

```
ARCH-1 (Provider refactor) ── BLOCKING for everything below
ARCH-2 (Tests) ── can run in parallel with ARCH-1

VM-1 (Disk model) ──→ VM-3, VM-4, VM-5 (SDK fetch) ──→ VM-9 (Actions) ──→ VM-10 (TUI)
VM-2 (Snap model) ──→ VM-6, VM-7, VM-8 (SDK fetch) ──→ VM-11 (TUI)
VM-13 (Tab Bar) ──→ needs VM-10, VM-11 to exist
VM-12 (Detail view) ──→ VM-3/4/5 (needs disk fetch)
```
