# Tasks: Networking Resources (Refined)

> **Scope:** VPCs/VNets, Subnets, Network→VM cross-navigation. Load Balancers, Route Tables, Static IPs cut from v1.
> **Owner:** Agent Network
> **Backend:** SDK only.
> **Dependencies:** `core.Resource` (exists), `ui.View` (exists), `NetworkProvider` (from ARCH-1).

---

## NET-1: Network (VPC) Resource Model
**Priority:** P0
**File:** `internal/core/network.go`

### Acceptance Criteria
- [ ] Define `Network` struct:
  ```
  Name, ID, State, CIDRBlock string
  IsDefault bool
  SubnetCount int
  Provider, Region, ResourceGroup, Labels string
  ```
- [ ] Implement `Resource` interface. `GetKind() → "Network"`.
- [ ] Define `DefaultNetworkColumns`: `["Name", "ID", "State", "CIDR", "Default", "Subnets", "Region"]`.
- [ ] Define `NetworkActions()`: `[View Subnets, Describe]`. Read-only — VPC mutations are IaC territory.
- [ ] Unit test.

---

## NET-2: Subnet Resource Model
**Priority:** P0
**File:** `internal/core/subnet.go`

### Acceptance Criteria
- [ ] Define `Subnet` struct:
  ```
  Name, ID, State, CIDRBlock, AvailabilityZone string
  NetworkID, NetworkName string
  AvailableIPs int
  MapPublicIPOnLaunch bool
  ResourceGroup, Labels string
  ```
- [ ] Implement `Resource` interface. `GetKind() → "Subnet"`.
- [ ] Define `DefaultSubnetColumns`: `["Name", "ID", "CIDR", "AZ", "Network", "Available IPs"]`.
- [ ] Define `SubnetActions()`: `[View VMs, Describe]`.
- [ ] Unit test.

---

## NET-3: AWS VPC + Subnet Fetching
**Priority:** P0
**File:** `internal/providers/aws/networks.go`

### Acceptance Criteria
- [ ] `FetchNetworksSDK(ctx, profile, region) ([]core.Network, error)`:
  - `ec2.DescribeVpcs` with paginator.
  - Map: `VpcId → ID`, `State`, `CidrBlock → CIDRBlock`, `IsDefault`.
  - Parse Tags for Name and Labels.
  - Batch-fetch all subnets and count per VPC (one `DescribeSubnets` call, group in Go).
- [ ] `FetchSubnetsSDK(ctx, profile, region) ([]core.Subnet, error)`:
  - `ec2.DescribeSubnets` with paginator.
  - Map: `SubnetId → ID`, `State`, `CidrBlock`, `AvailabilityZone`, `VpcId → NetworkID`, `AvailableIpAddressCount → AvailableIPs`, `MapPublicIpOnLaunch`.
  - Resolve `NetworkName` from VPC Tags.

---

## NET-4: GCP VPC + Subnet Fetching
**Priority:** P0
**File:** `internal/providers/gcp/networks.go`

### Acceptance Criteria
- [ ] `FetchNetworksSDK(ctx, project) ([]core.Network, error)`:
  - `compute.NetworksService.List(project)`.
  - GCP networks are global. Set `Region = "global"`.
  - GCP VPCs don't have a single CIDR — set `CIDRBlock` to `"auto-mode"` or `"custom-mode"`.
  - `subnetworks` array length → `SubnetCount`.
- [ ] `FetchSubnetsSDK(ctx, project) ([]core.Subnet, error)`:
  - `compute.SubnetworksService.AggregatedList(project)`.
  - Map: `name`, `id`, `ipCidrRange → CIDRBlock`, `region` (extract last segment), `network → NetworkName` (extract last segment).

---

## NET-5: Azure VNet + Subnet Fetching
**Priority:** P0
**File:** `internal/providers/azure/networks.go`

### Acceptance Criteria
- [ ] Add dependency: `github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6`.
- [ ] `FetchNetworksSDK(ctx, subscription) ([]core.Network, error)`:
  - `armnetwork.VirtualNetworksClient.NewListAllPager()`.
  - Map: `name`, `id`, `provisioningState → State`, `addressSpace.addressPrefixes[0] → CIDRBlock`, `subnets` length → `SubnetCount`, `location → Region`.
  - Extract `resourceGroup` from `id` URL.
- [ ] `FetchSubnetsSDK(ctx, subscription) ([]core.Subnet, error)`:
  - Azure subnets are nested under VNets. Fetch all VNets, extract subnets.
  - Map: `name`, `id`, `provisioningState → State`, `addressPrefix → CIDRBlock`. Derive AZ from VNet location.

---

## NET-6: Networks TUI View + Cross-Navigation
**Priority:** P0
**File:** `internal/views/networks/view.go`

### Acceptance Criteria
- [ ] Implement `ui.View` interface.
- [ ] Table from `core.DefaultNetworkColumns`.
- [ ] Search, sort, column config.
- [ ] `View Subnets` action → `PushViewMsg` with Subnets view filtered to selected VPC ID.
- [ ] If provider doesn't implement `NetworkProvider`, show SDK switch message.
- [ ] Subnets child view: Implement in same file or separate `subnets_view.go`.
  - Accept `filterNetworkID string`.
  - `View VMs` action → push VMsView filtered by subnet/network.
  - Breadcrumbs: `AWS › profile › region › vpc-123 › subnets`.
  - `Esc` → `PopViewMsg`.

---

## Dependency Graph

```
NET-1 (Network model) ──→ NET-3, 4, 5 (SDK fetch) ──→ NET-6 (TUI + cross-nav)
NET-2 (Subnet model) ──→ NET-3, 4, 5 (Subnet fetch) ──→ NET-6
```
