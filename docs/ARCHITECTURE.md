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
	FetchVMs(ctx context.Context, item contextItem) ([]VM, error)
	ExecuteAction(ctx context.Context, action string, vm VM, item contextItem) (string, error)
	GetSSHCmd(ctx context.Context, vm VM, item contextItem) (*exec.Cmd, error)
}
```

**Backend 1: The CLI Provider (`cli`)**
- Relies on `os/exec` to wrap `aws`, `gcloud`, and `az` commands.
- **Pros:** Zero authentication code required. Inherits all complex SSO, MFA, and profile configurations the user already has set up in their terminal.
- **Cons:** Slower. Requires spawning heavy external OS processes. Prone to parsing breakage if the upstream CLI JSON output changes.

**Backend 2: The Native SDK Provider (`sdk`)**
- Uses official Go SDKs (`aws-sdk-go-v2`, etc.).
- **Pros:** Blazing fast. Communicates directly via gRPC/HTTP. Provides compile-time safety and granular error handling. Allows for easy contextual cancellation (e.g., stopping a request when the user hits `Esc`).
- **Cons:** Requires more development effort to map all API interactions perfectly.

Users can toggle between these backends dynamically using the `--configure` TUI or the `--backend` flag.

### 3. Asynchronous Data Fetching & Caching
All network operations (fetching VMs, executing actions) return a `tea.Cmd`. This ensures the UI never blocks. 
To improve responsiveness, a local cache (`vmCache` map) is utilized. When a user switches back to a previously viewed region, data is loaded instantly from the cache while a background refresh can (optionally) occur.

### 4. Future FinOps Mode
In the future, a new major architectural module will be introduced: the `Recommender`. This will integrate with:
- Cloud-native recommendation APIs (e.g., AWS Compute Optimizer, GCP Recommender).
- The Gemini API, passing contextual VM usage data and cost metrics to an LLM to generate plain-text, actionable cost-saving recommendations directly in the TUI.
