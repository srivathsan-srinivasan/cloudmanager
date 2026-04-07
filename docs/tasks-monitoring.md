# Tasks: Monitoring & Metrics (Refined)

> **Scope:** CPU, Memory, Disk I/O, Network I/O metrics from CloudWatch, Cloud Monitoring, Azure Monitor. The critical bridge to Gemini FinOps.
> **Owner:** Agent Monitoring
> **Backend:** SDK only. CLI metric fetching is too slow (6-8 subprocess spawns per VM).
> **Dependencies:** `core.VM` (exists), `MetricsProvider` interface (from ARCH-1).
> **Can start in parallel with Sprint 2 (VM Resources).**

---

## MON-1: Metrics Data Model
**Priority:** P0
**File:** `internal/core/metrics.go`

### Acceptance Criteria
- [ ] Define `MetricDataPoint` struct: `Timestamp time.Time`, `Value float64`.
- [ ] Define `MetricSummary` struct:
  ```go
  MetricName string     // "CPUUtilization", "MemoryUsed", etc.
  Unit       string     // "Percent", "Bytes", "Bytes/Second", "Count/Second"
  Average    float64
  Maximum    float64
  Minimum    float64
  Current    float64    // Most recent data point
  DataPoints []MetricDataPoint // Last 24 points for sparkline
  ```
- [ ] Define `VMMetrics` struct:
  ```go
  VMID, VMName           string
  CPUUtilization         *MetricSummary
  MemoryUtilization      *MetricSummary  // nil if CloudWatch Agent not installed
  DiskReadBytes          *MetricSummary
  DiskWriteBytes         *MetricSummary
  NetworkInBytes         *MetricSummary
  NetworkOutBytes        *MetricSummary
  FetchedAt              time.Time
  Period                 time.Duration   // lookback window (e.g., 24h)
  Error                  string          // if fetch failed
  ```
- [ ] Define metric name constants: `MetricCPU`, `MetricMemory`, `MetricDiskRead`, `MetricDiskWrite`, `MetricNetworkIn`, `MetricNetworkOut`.
- [ ] Helper: `FormatMetricValue(value float64, unit string) string` — `72.3%`, `1.2 GB/s`, `340 IOPS`.
- [ ] Helper: `BuildSummary(name, unit string, datapoints []MetricDataPoint) *MetricSummary`.
- [ ] Unit tests for both helpers.

---

## MON-2: AWS CloudWatch Metrics Fetching
**Priority:** P0
**File:** `internal/providers/aws/metrics.go`

### Acceptance Criteria
- [ ] `FetchVMMetricsSDK(ctx, profile, region, instanceID, period) (*core.VMMetrics, error)`:
  - Add dependency: `github.com/aws/aws-sdk-go-v2/service/cloudwatch`.
  - Use `cloudwatch.GetMetricStatistics` for each metric.
  - Metrics (namespace `AWS/EC2`): `CPUUtilization`, `NetworkIn`, `NetworkOut`, `DiskReadBytes`, `DiskWriteBytes`.
  - Memory (namespace `CWAgent`): `mem_used_percent`. If fails → `MemoryUtilization = nil`, `Error = "Memory requires CloudWatch Agent"`.
  - Default period: 24h. Granularity: 3600s (1h) → 24 data points.
  - `Current` = last datapoint value.
- [ ] Run all metric fetches concurrently using `errgroup` with semaphore (max 5 concurrent).
- [ ] Handle `InsufficientData` / empty datapoints gracefully (instance might be stopped).
- [ ] Use `context.Context` for cancellation.

---

## MON-3: GCP Cloud Monitoring Metrics Fetching
**Priority:** P0
**File:** `internal/providers/gcp/metrics.go`

### Acceptance Criteria
- [ ] `FetchVMMetricsSDK(ctx, project, zone, instanceID, period) (*core.VMMetrics, error)`:
  - Add dependency: `cloud.google.com/go/monitoring/apiv3/v2`.
  - Use `MetricClient.ListTimeSeries` for each metric.
  - Metrics: `compute.googleapis.com/instance/cpu/utilization` (×100 for percent), `instance/network/received_bytes_count`, `sent_bytes_count`, `disk/read_bytes_count`, `write_bytes_count`.
  - Memory: `agent.googleapis.com/memory/percent_used` (requires Ops Agent). Graceful fallback.
  - Filter: `metric.type = "<type>" AND resource.labels.instance_id = "<id>"`.
- [ ] Concurrent fetching with `errgroup`.
- [ ] Handle rate limits with retry + backoff (6000 reads/min quota).

---

## MON-4: Azure Monitor Metrics Fetching
**Priority:** P0
**File:** `internal/providers/azure/metrics.go`

### Acceptance Criteria
- [ ] `FetchVMMetricsSDK(ctx, subscription, rg, vmName, period) (*core.VMMetrics, error)`:
  - Add dependency: `github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor`.
  - Use `armmonitor.MetricsClient.List()` with comma-separated metrics (Azure supports multi-metric in one call).
  - Metrics: `Percentage CPU`, `Network In Total`, `Network Out Total`, `Disk Read Bytes`, `Disk Write Bytes`.
  - Memory: `Available Memory Bytes` (requires Azure Monitor Agent). Calculate percent from VM size if possible.
- [ ] Handle deallocated VMs (no metrics) gracefully.

---

## MON-5: Extend VM Struct with Metrics Fields
**Priority:** P0
**File:** `internal/core/vm.go`

### Acceptance Criteria
- [ ] Add string fields: `CPUPercent`, `MemoryPercent`, `DiskIORead`, `DiskIOWrite`, `NetworkIn`, `NetworkOut` (formatted strings like `"72.3%"` or `"-"`).
- [ ] Update `GetField()` to handle: `"CPU %"`, `"Memory %"`, `"Disk Read"`, `"Disk Write"`, `"Net In"`, `"Net Out"`.
- [ ] These columns are **off by default** — users opt in via column config (press `C`).
- [ ] All VM fetch functions initialize these to `"-"`.

---

## MON-6: Async Metrics Enrichment
**Priority:** P0
**File:** `internal/views/vms/view.go`

### Description
Metrics are slow (1-3s per VM, 6+ API calls). They must NOT block the initial VM table load. Fetch asynchronously and update rows incrementally.

### Acceptance Criteria
- [ ] After `vmFetchMsg` arrives, if metrics columns are enabled, fire a `tea.Cmd` that fetches metrics for each VM.
- [ ] Fetch in batches of 3-5 VMs concurrently (avoid rate limits).
- [ ] New message: `vmMetricsMsg { vmID string; metrics *core.VMMetrics; err error }`.
- [ ] On receive, update that VM's `CPUPercent`, `MemoryPercent`, etc. and re-render only that row.
- [ ] Status bar: `"Loading metrics (3/12)..."`.
- [ ] Press `m` to toggle metrics on/off.
- [ ] Config: `metrics_enabled bool` (default `false`), `metrics_period_hours int` (default `24`).

---

## MON-7: Metrics Caching
**Priority:** P1
**File:** `internal/core/metrics_cache.go`

### Acceptance Criteria
- [ ] Thread-safe `MetricsCache` with `sync.RWMutex`:
  - `Get(vmID) (*VMMetrics, bool)` — returns cached if not expired.
  - `Set(vmID, *VMMetrics)`.
  - `InvalidateAll()`.
- [ ] Default TTL: 15 minutes (config: `metrics_cache_ttl_minutes`).
- [ ] Unit tests.

---

## MON-8: VM Table Metrics Columns
**Priority:** P1
**File:** `internal/views/vms/view.go`

### Acceptance Criteria
- [ ] New columns in config: `"CPU %"` (width 7), `"Memory %"` (width 9), `"Disk Read"` / `"Disk Write"` (width 10), `"Net In"` / `"Net Out"` (width 10).
- [ ] Color coding: `<30%` → green, `30-70%` → default, `>70%` → yellow, `>90%` → red.
- [ ] Update `createVMTable` weights for new columns.
- [ ] `"-"` (not fetched) renders in dim/subtle.

---

## MON-9: Metrics for Gemini FinOps Context
**Priority:** P0 — **This is the reason the monitoring module exists.**
**File:** `internal/views/vms/view.go` (update `finopsRecommendCmd`)

### Acceptance Criteria
- [ ] When user triggers `FinOps` action:
  1. Check metrics cache. If hit, use cached.
  2. If miss, fetch metrics synchronously (status: "Fetching metrics for Gemini analysis...").
  3. Pass metrics into the Gemini prompt (the actual V2 prompt lives in `tasks-billing.md` BILL-8).
- [ ] If metrics fetch fails entirely, fall back to metadata-only prompt with note: `"Utilization metrics unavailable."`.
- [ ] Handle nil metrics gracefully — write `"N/A"` for unavailable metrics.

---

## MON-10: Alerting Indicators
**Priority:** P1
**File:** `internal/views/vms/view.go`

### Acceptance Criteria
- [ ] Configurable thresholds: `cpu_warning: 70`, `cpu_critical: 90`, `mem_warning: 70`, `mem_critical: 90`.
- [ ] In the VM table, prepend status icon to VM name:
  - 🟢 if all metrics below warning.
  - 🟡 if any > warning but < critical.
  - 🔴 if any > critical.
  - No icon if metrics not fetched.

---

## Dependency Graph

```
MON-1 (Model) ──→ MON-2, MON-3, MON-4 (SDK fetch)
                ──→ MON-7 (Cache)

MON-2,3,4 ──→ MON-5 (VM struct extension)
MON-5 ──→ MON-6 (Async enrichment)
MON-6 ──→ MON-8 (Table columns)
MON-6 ──→ MON-9 (Gemini bridge) ← CRITICAL PATH
MON-8 ──→ MON-10 (Alerting)
```

### Critical Notes
- **Memory metrics require agents.** AWS: CloudWatch Agent. GCP: Ops Agent. Azure: Azure Monitor Agent. Always nil-handle gracefully.
- **Rate limits:** AWS 400 calls/s, GCP 6000 reads/min, Azure 12000 reads/hr. Use `errgroup` with semaphore.
- **Performance:** 50 VMs × 6 metrics = 300 API calls. With concurrency (5 parallel) ≈ 10-30s. This is why MON-6 (async enrichment) is non-negotiable.
