# Tasks: Billing & FinOps Intelligence (Refined)

> **Scope:** Cost data, cloud provider recommendations, and the V2 Gemini prompt with full context.
> **Owner:** Agent Billing
> **Backend:** SDK only.
> **Dependencies:** Monitoring module (tasks-monitoring.md) is a **hard dependency** for MON-9 (metrics in Gemini prompt). BILL-8 (Gemini V2) is the convergence point.

---

## BILL-1: Cost Data Model
**Priority:** P0
**File:** `internal/core/cost.go`

### Acceptance Criteria
- [ ] Define `ResourceCost`:
  ```go
  ResourceID, ResourceName, ResourceType, Provider string
  CurrentMonthCost   float64  // MTD spend in USD
  PreviousMonthCost  float64
  ForecastedCost     float64
  Currency           string   // "USD"
  LastUpdated        time.Time
  ```
- [ ] Define `AccountCost`:
  ```go
  Provider, AccountID, AccountName string
  CurrentMonthCost, PreviousMonthCost, ForecastedCost float64
  TopServices []ServiceCost
  LastUpdated time.Time
  ```
- [ ] `ServiceCost`: `ServiceName string`, `Cost float64`.
- [ ] Helpers: `FormatCost(amount) string` → `"$1,234.56"`, `"$12.3K"`.
- [ ] `CostChangePercent(current, previous) string` → `"+15.3%"`, `"-8.2%"`.
- [ ] Unit tests.

---

## BILL-2: Recommendation Data Model
**Priority:** P0
**File:** `internal/core/recommendation.go`

### Acceptance Criteria
- [ ] Define `Recommendation`:
  ```go
  ResourceID, ResourceName, ResourceType, Provider string
  Type             string  // "Rightsizing", "Idle", "Scheduling", "Purchase"
  Severity         string  // "Critical", "Warning", "Info"
  Summary          string  // "Overprovisioned: downsize to t3.micro"
  Detail           string
  EstimatedSavings float64 // Monthly USD
  CurrentConfig    string  // e.g., "t3.large"
  RecommendedConfig string // e.g., "t3.micro"
  Source           string  // "AWS Compute Optimizer", "GCP Recommender", "Azure Advisor"
  ```
- [ ] Implement `Resource` interface. `GetKind() → "Recommendation"`.
- [ ] Unit tests.

---

## BILL-3: AWS Cost Explorer Fetching
**Priority:** P0
**File:** `internal/providers/aws/billing.go`

### Acceptance Criteria
- [ ] Add dependency: `github.com/aws/aws-sdk-go-v2/service/costexplorer`.
- [ ] `FetchAccountCostSDK(ctx, profile) (*core.AccountCost, error)`:
  - `costexplorer.GetCostAndUsage` with `MONTHLY` granularity, grouped by `SERVICE`.
  - Current month MTD + previous month.
  - `costexplorer.GetCostForecast` for projected cost.
- [ ] `FetchVMCostSDK(ctx, profile, instanceID) (*core.ResourceCost, error)`:
  - Filter by `RESOURCE_ID`. Note: requires resource-level data in Cost Explorer.
  - Fallback if not available: estimate from instance type pricing.
- [ ] Handle `AccessDeniedException` gracefully.
- [ ] **Warn user**: Cost Explorer API costs $0.01/request. Gate behind config toggle.

---

## BILL-4: GCP Billing Fetching
**Priority:** P0
**File:** `internal/providers/gcp/billing.go`

### Acceptance Criteria
- [ ] **Primary:** BigQuery billing export (accurate, requires user setup):
  - Add dependency: `cloud.google.com/go/bigquery`.
  - Query billing export table for current month, grouped by service.
  - Config: `gcp_billing_dataset`, `gcp_billing_table` in `AppConfig`.
- [ ] **Fallback:** Pricing estimation from machine type × hours running × on-demand price.
  - Use `cloudbilling.googleapis.com` API for pricing data.
- [ ] Handle missing BigQuery export: `"GCP billing export not configured. Showing estimated costs."`.

---

## BILL-5: Azure Cost Management Fetching
**Priority:** P0
**File:** `internal/providers/azure/billing.go`

### Acceptance Criteria
- [ ] Add dependency: `github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/costmanagement/armcostmanagement`.
- [ ] `FetchAccountCostSDK(ctx, subscription) (*core.AccountCost, error)`:
  - `armcostmanagement.QueryClient.Usage()` with `ActualCost`, `MonthToDate`, grouped by `ServiceName`.
- [ ] `FetchVMCostSDK(ctx, subscription, vmResourceID) (*core.ResourceCost, error)`:
  - Filter by resource ID dimension.
- [ ] Handle permission errors gracefully.

---

## BILL-6: AWS Compute Optimizer Recommendations
**Priority:** P1
**File:** `internal/providers/aws/recommendations.go`

### Acceptance Criteria
- [ ] Add dependency: `github.com/aws/aws-sdk-go-v2/service/computeoptimizer`.
- [ ] `FetchRecommendationsSDK(ctx, profile, region) ([]core.Recommendation, error)`:
  - `computeoptimizer.GetEC2InstanceRecommendations`.
  - Map: `finding` → `Type`/`Severity`, `recommendationOptions[0].instanceType` → `RecommendedConfig`, `estimatedMonthlySavings` → `EstimatedSavings`.
  - `Source = "AWS Compute Optimizer"`.
- [ ] Handle `OptInRequiredException` → "Enable Compute Optimizer in AWS Console."

---

## BILL-7: GCP Recommender + Azure Advisor
**Priority:** P1
**Files:** `internal/providers/gcp/recommendations.go`, `internal/providers/azure/recommendations.go`

### Acceptance Criteria
- [ ] **GCP:** `cloud.google.com/go/recommender/apiv1`.
  - `RecommenderClient.ListRecommendations` for `google.compute.instance.MachineTypeRecommender` and `IdleResourceRecommender`.
  - Iterate zones where VMs exist.
- [ ] **Azure:** `github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/advisor/armadvisor`.
  - `armadvisor.RecommendationsClient.NewListPager()` filtered by `Category eq 'Cost'`.
  - Map: `shortDescription.problem` → `Summary`, `extendedProperties.annualSavingsAmount/12` → `EstimatedSavings`.

---

## BILL-8: Gemini FinOps Prompt V2 (Full Context)
**Priority:** P0 — **THE CONVERGENCE POINT of the entire project.**
**File:** `internal/views/vms/view.go` (refactor `finopsRecommendCmd`)

### Acceptance Criteria
- [ ] When user triggers `FinOps` on a VM, gather all data (with timeouts):
  1. VM metadata (already have).
  2. Metrics: check cache, fetch if needed (max 10s timeout).
  3. Cost: check cache, fetch if needed (max 5s timeout).
  4. Provider recommendation: check cache, fetch if needed (max 5s timeout).
- [ ] Build the V2 prompt:
  ```
  You are an expert Cloud FinOps Architect.

  === VM METADATA ===
  Provider: %s | Name: %s | ID: %s
  Instance Type: %s | State: %s | Zone: %s
  Network: %s / Subnet: %s | Labels: %s

  === UTILIZATION METRICS (Last %s) ===
  CPU:    Avg=%.1f%% | Max=%.1f%% | Current=%.1f%%
  Memory: Avg=%.1f%% | Max=%.1f%% | Current=%.1f%%  [%s]
  Disk Read: Avg=%s | Max=%s
  Disk Write: Avg=%s | Max=%s
  Net In: Avg=%s | Max=%s
  Net Out: Avg=%s | Max=%s

  === COST DATA ===
  Current Month (MTD): %s
  Previous Month: %s | Forecasted: %s | Trend: %s

  === PROVIDER RECOMMENDATION ===
  Source: %s | Finding: %s
  Current: %s → Recommended: %s
  Estimated Savings: %s/month

  === YOUR TASK ===
  1. Rightsizing Verdict: Is this VM over/under-provisioned? Cite CPU/memory averages.
  2. Specific Action: Recommend a concrete instance type with expected savings.
  3. Scheduling: Could this VM be stopped off-hours?
  4. Purchase: Should this be Reserved/Committed Use/Savings Plan?
  5. Risk: Any risks to the recommendation (e.g., occasional CPU spikes)?

  Be concise (5-8 sentences). Use specific numbers. No markdown.
  ```
- [ ] Handle missing data: replace each section with `"<type> unavailable (<reason>)."`.
- [ ] Config: `gemini_model` (default `"gemini-1.5-flash"`).
- [ ] Show spinner while gathering data + waiting for response.
- [ ] Display in viewport (same as Describe pane).

---

## BILL-9: Cost Enrichment Pipeline
**Priority:** P0
**File:** `internal/views/vms/view.go`

### Acceptance Criteria
- [ ] Add fields to `core.VM`: `MonthlyCost string`, `CostTrend string`, `Recommendation string`, `EstimatedSavings string`.
- [ ] Update `VM.GetField()` for: `"Cost"`, `"Cost Trend"`, `"Recommendation"`, `"Est. Savings"`.
- [ ] After VM fetch, if billing enabled:
  1. Fetch account cost once (one API call).
  2. Fetch recommendations once (one API call).
  3. Match recommendations to VMs by resource ID.
  4. Update matched VMs' fields.
- [ ] Cache costs with TTL 30 minutes.
- [ ] Config: `billing_enabled bool` (default `false`), `billing_cache_ttl_minutes int` (default `30`).

---

## BILL-10: Billing Configuration
**Priority:** P0
**File:** `internal/config/config.go`

### Acceptance Criteria
- [ ] Add to `AppConfig`:
  ```go
  BillingEnabled     bool   `mapstructure:"billing_enabled"`
  BillingCacheTTL    int    `mapstructure:"billing_cache_ttl_minutes"`
  GCPBillingDataset  string `mapstructure:"gcp_billing_dataset"`
  GeminiModel        string `mapstructure:"gemini_model"`
  MetricsEnabled     bool   `mapstructure:"metrics_enabled"`
  MetricsPeriodHours int    `mapstructure:"metrics_period_hours"`
  MetricsCacheTTL    int    `mapstructure:"metrics_cache_ttl_minutes"`
  ```
- [ ] Set defaults via Viper.
- [ ] Validate: `billing_enabled && !GEMINI_API_KEY` → warning.
- [ ] Update `--configure` TUI to include billing/metrics toggles.

---

## Dependency Graph

```
BILL-1 (Cost model) ──→ BILL-3, 4, 5 (Cost fetch) ──→ BILL-9 (Enrichment)
BILL-2 (Rec model) ──→ BILL-6, 7 (Rec fetch) ──→ BILL-9

MON-9 (Metrics for Gemini) ──→ BILL-8 (Gemini V2) ← THE CONVERGENCE POINT
BILL-3,4,5 (Cost) ──→ BILL-8
BILL-6,7 (Recs) ──→ BILL-8
BILL-10 (Config) ──→ everything
```
