# CloudManager Architecture

CloudManager is built in Go using the Bubble Tea framework. It follows the **Model-View-Update (MVU)** pattern (often called the Elm architecture).

## Core Design Principles

### 1. The Elm Architecture (Bubble Tea)
- **Model:** The application state is stored in a single struct (`models.go`), encompassing the currently selected cloud context, the list of VMs, the UI focus state (which pane is active), and caching data.
- **Update:** All state mutations happen via pure functions in `update.go`. When an event occurs (a keypress, a successful network request), a `Msg` is generated. The `Update` function receives the `Msg`, updates the `Model`, and optionally returns a `Cmd` (a side-effect, like a network request).
- **View:** The `View` function simply renders the current state of the `Model` into string format using Lipgloss for styling.

### 2. The Hybrid Backend Model (Pluggable Architecture)
To balance ease of onboarding with ultimate performance, CloudManager implements a `Provider` interface (`provider.go`).

```go
type Provider interface {
	FetchVMs(ctx context.Context, cloudCtx core.CloudContext) ([]core.VM, error)
	ExecuteAction(ctx context.Context, action string, vm core.VM, cloudCtx core.CloudContext) (string, error)
	GetSSHCmd(ctx context.Context, vm core.VM, cloudCtx core.CloudContext) (*exec.Cmd, error)
}
```

#### Composable Interfaces Pattern
As the application expands to manage more resources (Disks, Snapshots, Firewalls, etc.), the `Provider` interface has been refactored to use Go's type assertion pattern instead of a monolithic God-interface. 

New, domain-specific interfaces are defined alongside `Provider`:
- `DiskProvider`
- `SnapshotProvider`
- `FirewallProvider`
- `NetworkProvider`
- `MetricsProvider`
- `BillingProvider`

Views interact with these capabilities by checking if the current provider implements the necessary interface:
```go
provider := providers.GetProvider(cfg)
if dp, ok := provider.(providers.DiskProvider); ok {
    disks, err := dp.FetchDisks(ctx, cloudCtx)
} else {
    // Show "Switch to SDK backend for disk management"
}
```

**Backend 1: The CLI Provider (`cli`)**
- Relies on `os/exec` to wrap `aws`, `gcloud`, and `az` commands.
- **Pros:** Zero authentication code required. Inherits all complex SSO, MFA, and profile configurations the user already has set up in their terminal.
- **Cons:** Slower. Requires spawning heavy external OS processes. Prone to parsing breakage if the upstream CLI JSON output changes.
- **Scope:** Implements ONLY the base `Provider` interface (VM operations). Maintained for legacy compatibility.

**Backend 2: The Native SDK Provider (`sdk`)**
- Uses official Go SDKs (`aws-sdk-go-v2`, etc.).
- **Pros:** Blazing fast. Communicates directly via gRPC/HTTP. Provides compile-time safety and granular error handling. Allows for easy contextual cancellation (e.g., stopping a request when the user hits `Esc`).
- **Cons:** Requires more development effort to map all API interactions perfectly.
- **Scope:** Implements ALL interfaces (`DiskProvider`, `SnapshotProvider`, etc.). This is the primary backend for all new features.

Users can toggle between these backends dynamically using the `--configure` TUI or the `--backend` flag.

### 3. Asynchronous Data Fetching & Caching
All network operations (fetching VMs, executing actions) return a `tea.Cmd`. This ensures the UI never blocks. 
To improve responsiveness, a local cache (`vmCache` map) is utilized. When a user switches back to a previously viewed region, data is loaded instantly from the cache while a background refresh can (optionally) occur.

### 4. Future FinOps Mode
In the future, a new major architectural module will be introduced: the `Recommender`. This will integrate with:
- Cloud-native recommendation APIs (e.g., AWS Compute Optimizer, GCP Recommender).
- The Gemini API, passing contextual VM usage data and cost metrics to an LLM to generate plain-text, actionable cost-saving recommendations directly in the TUI.
