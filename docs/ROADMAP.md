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
