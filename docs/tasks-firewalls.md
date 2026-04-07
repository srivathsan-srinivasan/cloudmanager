# Tasks: Firewalls & Security Groups (Refined)

> **Scope:** Security Groups, NSGs, GCP Firewall Rules, security audit highlighting.
> **Owner:** Agent Firewall
> **Backend:** SDK only.
> **Dependencies:** `core.Resource` (exists), `ui.View` (exists), `FirewallProvider` (from ARCH-1).
> **Can run in parallel with Sprint 4 (Billing).**

---

## FW-1: Security Group Resource Model
**Priority:** P0
**File:** `internal/core/firewall.go`

### Acceptance Criteria
- [ ] Define `SecurityGroup` struct:
  ```
  Name, ID, Description, NetworkID, NetworkName string
  InboundRuleCount, OutboundRuleCount, AttachedResources int
  HasOpenSSH, HasOpenRDP bool  // for audit highlighting
  Provider, Region, ResourceGroup, Labels string
  ```
- [ ] Implement `Resource` interface. `GetKind() → "Security Group"`.
- [ ] Define `DefaultSecurityGroupColumns`: `["Name", "ID", "Network", "Inbound Rules", "Outbound Rules", "Attached", "Description"]`.
- [ ] Define `SecurityGroupActions()`: `[View Rules, Describe, Delete]`. `Delete` is dangerous.
- [ ] Unit test for `GetField`.

---

## FW-2: Firewall Rule Resource Model
**Priority:** P0
**File:** `internal/core/firewall_rule.go`

### Acceptance Criteria
- [ ] Define `FirewallRule` struct:
  ```
  ID, Direction (Inbound/Outbound), Protocol (tcp/udp/icmp/all),
  PortRange (22 / 80-443 / All), Source, Destination,
  Action (Allow/Deny), Priority int, Description,
  ParentGroupID, ParentGroupName string
  ```
- [ ] Implement `Resource` interface. `GetKind() → "Firewall Rule"`.
- [ ] Define `DefaultFirewallRuleColumns`: `["Direction", "Protocol", "Port Range", "Source", "Destination", "Action", "Priority", "Description"]`.
- [ ] Helper: `IsRuleRisky(rule FirewallRule) bool` — true if Source is `0.0.0.0/0` and port includes 22 or 3389 or protocol is "All".
- [ ] Unit test.

---

## FW-3: AWS Security Group Fetching
**Priority:** P0
**File:** `internal/providers/aws/firewalls.go`

### Acceptance Criteria
- [ ] `FetchSecurityGroupsSDK(ctx, profile, region) ([]core.SecurityGroup, error)`:
  - `ec2.DescribeSecurityGroups` with paginator.
  - Map: `GroupId → ID`, `GroupName → Name`, `Description`, `VpcId → NetworkID`, `IpPermissions` length → `InboundRuleCount`, `IpPermissionsEgress` length → `OutboundRuleCount`.
  - Check inbound rules for `0.0.0.0/0` on port 22/3389 → set `HasOpenSSH`/`HasOpenRDP`.
  - `AttachedResources = 0` (populated on Describe to avoid N+1).
- [ ] `FetchFirewallRulesSDK(ctx, profile, region, sgID) ([]core.FirewallRule, error)`:
  - `ec2.DescribeSecurityGroupRules` filtered by group-id.
  - Map: `IsEgress → Direction`, `IpProtocol` (`-1` = All), `FromPort-ToPort → PortRange`, `CidrIpv4 → Source/Destination`.
  - AWS SG rules are all Allow (deny-by-default). `Action = "Allow"`, `Priority = 0`.

---

## FW-4: GCP Firewall Rule Fetching
**Priority:** P0
**File:** `internal/providers/gcp/firewalls.go`

### Acceptance Criteria
- [ ] `FetchSecurityGroupsSDK(ctx, project) ([]core.SecurityGroup, error)`:
  - `compute.FirewallsService.List(project)`.
  - **Group by network**: create one `SecurityGroup` per network with rule counts.
  - Check for `0.0.0.0/0` + port 22/3389 → `HasOpenSSH`/`HasOpenRDP`.
- [ ] `FetchFirewallRulesByNetworkSDK(ctx, project, networkName) ([]core.FirewallRule, error)`:
  - Filter by network in Go.
  - Map: `direction → Direction`, `allowed[]` → `Protocol/PortRange/Action=Allow`, `denied[]` → `Action=Deny`, `sourceRanges → Source`, `priority → Priority`.

---

## FW-5: Azure NSG Fetching
**Priority:** P0
**File:** `internal/providers/azure/firewalls.go`

### Acceptance Criteria
- [ ] `FetchSecurityGroupsSDK(ctx, subscription) ([]core.SecurityGroup, error)`:
  - Add dependency: `github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6`.
  - `armnetwork.SecurityGroupsClient.NewListAllPager()`.
  - Map: `name`, `securityRules` → count inbound/outbound, `networkInterfaces` length → `AttachedResources`.
  - Check rules for `0.0.0.0/0` on ports 22/3389 → `HasOpenSSH`/`HasOpenRDP`.
- [ ] `FetchFirewallRulesSDK(ctx, subscription, rg, nsgName) ([]core.FirewallRule, error)`:
  - `armnetwork.SecurityRulesClient.NewListPager(rg, nsgName)`.
  - Also include default rules (priority >= 65000) rendered dimmed.
  - Map: `direction`, `protocol` (`*` = All), `destinationPortRange → PortRange`, `sourceAddressPrefix → Source`, `access → Action`, `priority`.

---

## FW-6: Firewalls TUI View
**Priority:** P0
**File:** `internal/views/firewalls/view.go`

### Acceptance Criteria
- [ ] Implement `ui.View` interface.
- [ ] Table from `core.DefaultSecurityGroupColumns`.
- [ ] Search, sort, column config (same pattern as VMsView).
- [ ] `View Rules` action → `PushViewMsg` with rules drill-down view (FW-7).
- [ ] If provider doesn't implement `FirewallProvider`, show SDK switch message.
- [ ] **Audit highlighting**: rows with `HasOpenSSH || HasOpenRDP` → render in Alert color with ⚠ prefix.

---

## FW-7: Firewall Rules Drill-Down View
**Priority:** P0
**File:** `internal/views/firewalls/rules_view.go`

### Acceptance Criteria
- [ ] Implement `ui.View` interface.
- [ ] Accept `parentGroupID`, `parentGroupName` (+ `resourceGroup` for Azure, `networkName` for GCP).
- [ ] Table from `core.DefaultFirewallRuleColumns`.
- [ ] **Color-coded rows**:
  - Source `0.0.0.0/0` or `::/0` → Alert color (red).
  - Protocol `All` + Port `All` → Alert color.
  - Action `Deny` → dimmed style.
- [ ] Sort by Priority (default).
- [ ] Actions: `[Describe]` only. Rule mutations are IaC territory.
- [ ] `Esc` → `PopViewMsg` back to SG list.

---

## FW-8: VM → Security Group Cross-Navigation
**Priority:** P2
**Files:** `internal/core/vm.go`, `internal/views/vms/view.go`

### Acceptance Criteria
- [ ] Add `SecurityGroups string` to `core.VM` (comma-separated SG IDs).
- [ ] Populate during VM fetch:
  - AWS: `SecurityGroups[].GroupId` from describe-instances.
  - GCP: `tags.items` (network tags that firewall rules target).
  - Azure: resolve NIC → NSG associations.
- [ ] Add `"View Security Groups"` action in `VMActions()`.
- [ ] Push `FirewallsView` filtered to the VM's SG IDs.

---

## Dependency Graph

```
FW-1 (SG model) ──→ FW-3, 4, 5 (SDK fetch) ──→ FW-6 (TUI)
FW-2 (Rule model) ──→ FW-3, 4, 5 (Rule fetch) ──→ FW-7 (Rules TUI)
FW-6 ──→ FW-7 (View Rules action)
FW-8 (Cross-nav) ── independent, P2
```
