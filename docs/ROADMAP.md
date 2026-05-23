# CloudManager TUI — Development Roadmap (Refined)

> **Total: ~49 tasks** (down from 93). Organized into 6 sprints with clear parallelism.
>
> | File | Scope | Tasks |
> |------|-------|-------|
> | [`tasks-vm.md`](./tasks-vm.md) | Architecture fix, Disks, Snapshots, Tab Bar, VM Detail | ARCH-1,2 + VM-1 → VM-13 |
> | [`tasks-monitoring.md`](./tasks-monitoring.md) | CloudWatch, Cloud Monitoring, Azure Monitor, Gemini bridge | MON-1 → MON-10 |
> | [`tasks-billing.md`](./tasks-billing.md) | Cost APIs, Recommendations, Gemini V2 | BILL-1 → BILL-10 |
> | [`tasks-firewalls.md`](./tasks-firewalls.md) | Security Groups, NSGs, Rules, Audit | FW-1 → FW-8 |
> | [`tasks-network.md`](./tasks-network.md) | VPCs, Subnets, Cross-navigation | NET-1 → NET-6 |

---

## Architecture Principles (Refined)

1. **Small interfaces with type assertions** — not one God interface
2. **SDK-only for new resources** — CLI maintained only for existing VM operations
3. **Async enrichment** — metrics and cost never block the initial table render
4. **FinOps is the differentiator** — not resource browsing
5. **Small core, optional components** — CloudManager core stays fast; deeper services ship as installable components

## Product Thesis

CloudManager is the terminal control plane for fast, auditable cloud operations.

It is not a replacement for cloud consoles, SDKs, Terraform, Steampipe,
CloudQuery, Prowler, or Kubernetes-native tools. It is the operator runtime that
brings frequently used cloud resources into one keyboard-first workflow.

CloudManager is built for:

- speed
- firefighting
- instantaneous access
- traceability
- local inventory
- provider-aware actions
- terminal-native auditability

The target users are NOC engineers, SOC engineers, Red Teamers, DevOps
engineers, SecOps engineers, and Platform engineers who need to move across
cloud resources faster than a browser console allows.

Core must stay focused:

- contexts/profiles/auth
- inventory and index
- dashboard
- scoped Find Resources
- access resolver
- CloudManager-local tags
- audit/event log
- provider capability registry
- TUI shell

Core UX backlog:

- Make Find and every resource viewport sort by any visible column, with a
  shared keyboard flow instead of one fixed/default sort per view.

Everything else should be a component unless it is essential to the core
operator workflow.

---

## Sprint Plan

### Sprint 1: Architecture Fix (1-2 days) — BLOCKING
| Task | Description |
|------|-------------|
| ARCH-1 | Refactor Provider into composable interfaces (`DiskProvider`, `FirewallProvider`, `MetricsProvider`, `BillingProvider`) |
| ARCH-2 | Test infrastructure: MockProvider, model unit tests |

### Sprint 2: VM Resources (1 week)
| Task | Description |
|------|-------------|
| VM-1,2 | Disk + Snapshot models |
| VM-3,4,5 | Disk SDK fetch (AWS, GCP, Azure) |
| VM-6,7,8 | Snapshot SDK fetch |
| VM-9 | Disk actions |
| VM-10,11 | Disks + Snapshots TUI views |
| VM-12 | VM Detail drill-down |
| VM-13 | Tab bar in App |

### Sprint 3: Metrics Pipeline (1 week) — **PARALLEL WITH SPRINT 2**
| Task | Description |
|------|-------------|
| MON-1 | Metrics data model |
| MON-2,3,4 | CloudWatch + Cloud Monitoring + Azure Monitor SDK fetch |
| MON-5 | VM struct metrics fields |
| MON-6 | Async enrichment pipeline |
| MON-7 | Metrics caching |
| MON-8 | VM table metrics columns |
| MON-9 | **Gemini bridge — pass real metrics to prompt** |
| MON-10 | Alerting indicators |

### Sprint 4: Billing + Gemini V2 (1 week) — **THE DIFFERENTIATOR**
| Task | Description |
|------|-------------|
| BILL-1,2 | Cost + Recommendation models |
| BILL-3,4,5 | Cost SDK fetch (AWS CE, GCP BigQuery, Azure Cost Mgmt) |
| BILL-6,7 | Recommendation APIs (Compute Optimizer, Recommender, Advisor) |
| BILL-8 | **GEMINI V2 PROMPT — metadata + metrics + cost + recommendations** |
| BILL-9 | Cost enrichment pipeline |
| BILL-10 | Billing configuration |

### Sprint 5: Security (3-4 days) — **PARALLEL WITH SPRINT 4**
| Task | Description |
|------|-------------|
| FW-1,2 | SecurityGroup + FirewallRule models |
| FW-3,4,5 | SDK fetch (AWS SGs, GCP rules, Azure NSGs) |
| FW-6 | Firewalls TUI view with audit highlighting |
| FW-7 | Rules drill-down view |
| FW-8 | VM → SG cross-navigation |

### Sprint 6: Networking (3-4 days)
| Task | Description |
|------|-------------|
| NET-1,2 | Network + Subnet models |
| NET-3,4,5 | SDK fetch |
| NET-6 | Networks TUI + Subnet drill-down + cross-nav |

---

## What's Cut From V1
- ❌ Images/AMIs, Load Balancers, Route Tables, Static IPs
- ❌ CLI backend for new resources
- ❌ Sparklines, FinOps dashboard, cost anomaly detection
- ❌ Bulk actions, background sync
- ❌ Disk/Network metrics (VM-level only)

These become v2 items after the core FinOps pipeline ships.

## vNext: Lean Integrations, Not Tool Sprawl

CloudManager should remain an operator cockpit, not a replacement for every
inventory, query, security, or governance platform.

### Local Index

- SQLite now backs access memory and the local VM/resource inventory cache.
- Keep JSON cache compatibility temporarily as import/fallback during migration.
- Store normalized indexed rows by provider, context, resource type, resource ID,
  searchable text, tags, and last-seen timestamp.
- Keep provider refresh explicit: startup stays fast unless the user enables
  prefetch.

Remaining build order:

1. Dashboard and `find-*` direct query paths from SQLite instead of in-memory maps.
2. SQLite FTS for fast name/IP/tag/security-group/subnet searches.
3. Remove JSON fallback after a stable migration window.

### Asset Inventory And Tags

- Treat CloudManager as a local asset inventory for indexed cloud resources.
- Support CloudManager-owned tags across clouds for cross-provider labels such as
  `VFWEB`, `prod`, `customer-a`, or `incident-watch`.
- Keep CloudManager tags local by default so Terraform/IaC and provider metadata
  are not mutated accidentally.
- Add explicit provider-tag sync later:
  - `local only`
  - `sync to provider tags/labels`
  - `import provider tags into CloudManager`
- Every tag mutation must show whether it is local-only or provider-mutating.

### Refresh Semantics

- Live resource views refresh when opened or when the user presses refresh.
- Index/cache refresh happens through explicit `:index-*`, `:index-all`, settings
  actions, or configured prefetch.
- No automatic provider event stream exists yet; a resource modified elsewhere is
  not reflected until the relevant view/index refresh runs.
- Future: optional background refresh and provider event ingestion, disabled by
  default for cost and startup-speed reasons.

### Access Memory And Bootstrap

- Remember successful SSH access in SQLite by provider/context/resource/user/IP/key.
- Prefer learned SSH access before generic key selection on the next run.
- Offer explicit authorized_keys bootstrap:
  - use selected working key once
  - append the default public key such as `~/.ssh/id_rsa.pub`
  - then mark the VM as default-key-ready after confirmed success
- Cloud-native paths stay provider-aware:
  - AWS: SSM first, private key fallback
  - GCP: metadata/OS Login path
  - Azure: native SSH/run-command path plus private key fallback
  - Other/manual: generic SSH/RDP with learned local access

### IAM / Show My Access

- Add `Show my access` as a provider-aware view for the active context.
- AWS: STS caller identity plus IAM policy simulation / attached role summaries
  where available.
- Azure: effective role assignments at subscription/resource group/resource scope.
- GCP: IAM policy bindings, project roles, OS Login/metadata SSH hints.
- Surface this as operator guidance, not an authorization engine:
  “you can describe VMs”, “you cannot modify firewall rules”, “SSM missing”.

### Plugin Interfaces

- Add a pluggable integration interface without bloating the core binary.
- Candidate plugin types:
  - MCP servers for tools/agents that need CloudManager inventory and actions.
  - A2A/agent interfaces for incident workflows and triage assistants.
  - Local command adapters for Steampipe, CloudQuery, Cloudlist, Prowler.
  - Read-only inventory providers and action providers.
- Plugins must declare capabilities, permissions, and mutation risk clearly.

### Component System

CloudManager should support optional installable components, similar in spirit
to `gcloud components`, but scoped to CloudManager's terminal workflow.

Example UX:

```text
cloudmanager component list
cloudmanager component install pubsub
cloudmanager component enable pubsub
cloudmanager component disable pubsub
cloudmanager component update
```

Example component manifest:

```yaml
name: pubsub
version: 0.1.0
kind: resource-component
providers:
  gcp:
    resources:
      - pubsub_topics
      - pubsub_subscriptions
capabilities:
  - list
  - find
  - describe
  - dashboard
  - actions
commands:
  - find-pubsub
  - index-pubsub
views:
  - Pub/Sub
```

Component rules:

- Components must not receive raw long-lived credentials by default.
- Components receive active provider/context/account/region and a scoped
  runtime credential handle when available.
- Components must clearly declare read-only vs mutating actions.
- Dangerous actions must use CloudManager confirmation and audit paths.
- Components should emit normalized resources into CloudManager's inventory
  model so dashboard, tags, and Find Resources work consistently.
- Components should be installable locally first; signed/trusted component
  distribution can come later.

Practical build order:

1. Add `components` config section.
2. Add local component registry and manifest parser.
3. Add component list/enable/disable UI.
4. Add read-only component execution contract.
5. Let component resources feed SQLite/index/search.
6. Add dashboard widgets from component manifests.
7. Add action execution with explicit mutation risk.
8. Add signed/trusted component installation.

Candidate first-party components:

- `pubsub`: GCP Pub/Sub topics/subscriptions and equivalent queue surfaces.
- `queues`: SQS, Pub/Sub subscriptions, Azure Service Bus queues.
- `dns`: Route53, Cloud DNS, Azure DNS.
- `waf`: AWS WAF, Cloud Armor, Azure WAF.
- `secrets`: Secrets Manager, Secret Manager, Key Vault references.
- `iam-access`: "Show my access" and effective permission summaries.
- `lb-health`: load balancers, target groups, backend health.
- `incident-pack`: fast triage views for common outage paths.

Rule: a component adds a capability; CloudManager owns the operator experience.

### Incident-Response Services

- Expand beyond compute where fast operator access matters:
  - load balancers and target health
  - DNS records and zones
  - NAT gateways, VPNs, routes, peering
  - IAM users/roles/service accounts
  - secrets/key vault references
  - queues, functions, container services, logs
  - object storage access and public exposure
- Keep the UI scope-first: `find-lbs`, `find-dns`, `find-iam`,
  `find-secrets`, not one noisy global dump.

### Pluggable Data Engines

Optional integrations should feed CloudManager's Find Resources and dashboard,
without bloating the default binary:

- `steampipe`: query cloud resources through SQL and show results in a scoped
  CloudManager search surface.
- `cloudlist`: ingest lightweight asset-discovery output for broad IP/host/asset
  visibility.
- `cloudquery`: import/query an existing cloud asset inventory instead of
  duplicating large-scale sync logic.
- `prowler`: surface security posture findings as contextual hints, not as a
  replacement for Prowler reports.

Proposed UX:

- `cloudmanager --steampipe`: open a Steampipe-backed query/search mode.
- `:engine`: choose local index, Steampipe, Cloudlist, CloudQuery, or disabled.
- `:find-*`: keep the same scoped Find Resources UX regardless of backend.

Rule: external engines are adapters. CloudManager owns the operator workflow.

---

## The Full FinOps Pipeline

```
┌─────────────┐   ┌─────────────┐   ┌──────────────┐   ┌───────────────┐
│ VM Metadata  │   │  Metrics    │   │  Cost Data   │   │ Provider Recs │
│ (exists)     │   │ (Sprint 3)  │   │ (Sprint 4)   │   │ (Sprint 4)    │
└──────┬───────┘   └──────┬──────┘   └──────┬───────┘   └───────┬───────┘
       │                  │                  │                   │
       └──────────────────┴──────────────────┴───────────────────┘
                                    │
                          ┌─────────▼─────────┐
                          │  GEMINI V2 PROMPT  │
                          │  (BILL-8)          │
                          └─────────┬──────────┘
                                    │
                          ┌─────────▼──────────┐
                          │  "This t3.large     │
                          │  averages 8% CPU.   │
                          │  Downsize to t3.small│
                          │  and save $60/mo."  │
                          └────────────────────┘
```

---

## Agent Assignment

| Agent | Files | Start | Blocked By |
|-------|-------|-------|------------|
| **Agent Arch** | `tasks-vm.md` ARCH-1,2 | Immediately | Nothing — do first |
| **Agent VM** | `tasks-vm.md` VM-1→13 | After ARCH-1 | ARCH-1 |
| **Agent Metrics** | `tasks-monitoring.md` | After ARCH-1 | ARCH-1, can parallel with VM |
| **Agent Billing** | `tasks-billing.md` | After MON-9 for BILL-8 | MON-9 for Gemini V2 |
| **Agent Firewall** | `tasks-firewalls.md` | After ARCH-1 | ARCH-1, can parallel with Billing |
| **Agent Network** | `tasks-network.md` | After ARCH-1 | ARCH-1 |
