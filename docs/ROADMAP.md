# CloudManager TUI - Development Roadmap

This document outlines the strategic roadmap for CloudManager, evolving it from a multi-cloud VM dashboard to a high-performance, intelligent FinOps terminal environment.

## Phase 1: Context & Configuration Discovery (Completed)
- [x] AWS Configuration Parser (`~/.aws/config`, `~/.aws/credentials`).
- [x] GCP Configuration Parser (`~/.config/gcloud/configurations/`).
- [x] Azure Configuration Parser (Azure CLI config).
- [x] Concurrent initialization and graceful fallback.

## Phase 2: Read-Only Integration & Hybrid Architecture (Completed)
- [x] Implemented CLI-wrapper backend for AWS, GCP, Azure via `os/exec`.
- [x] Implemented asynchronous loading, error handling, and TUI spinners.
- [x] Introduced the `Provider` interface (`provider.go`) to decouple UI from backend logic.
- [x] Built the `--configure` TUI to toggle between `cli` and `sdk` backends dynamically.

## Phase 3: Action Execution & Interactive Sessions (Completed)
- [x] Implemented Start/Stop/Restart/Terminate mapping for CLI backends.
- [x] TUI Confirmation dialogs for destructive actions.
- [x] Interactive SSH Integration (AWS SSM, GCP IAP, Azure standard SSH) via `tea.ExecProcess`.

## Phase 4: Native Go SDK Implementation (In Progress / Next)
*The transition to Native SDKs for the "sdk" backend to provide order-of-magnitude performance improvements and static typing safety.*
- [ ] **AWS SDK v2 Integration**
  - [ ] Implement `FetchVMs` using `github.com/aws/aws-sdk-go-v2/service/ec2`.
  - [ ] Implement `ExecuteAction` using native SDK calls.
  - [ ] Handle AWS SSO credential resolution explicitly if needed.
- [ ] **GCP SDK Integration**
  - [ ] Implement `FetchVMs` using `google.golang.org/api/compute/v1`.
  - [ ] Implement `ExecuteAction` using native SDK calls.
- [ ] **Azure SDK Integration**
  - [ ] Implement `FetchVMs` using `github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute`.
  - [ ] Use `azidentity.NewAzureCLICredential()` for seamless auth handoff.

## Phase 5: Intelligent FinOps & Gemini Integration
*Elevating the tool from purely operational to strategic cost management.*
- [ ] **Cost Data Fetching**
  - [ ] Integrate with AWS Cost Explorer, GCP Billing API, and Azure Cost Management to fetch real-time and estimated monthly costs for selected VMs or overall projects.
  - [ ] Add a new `Cost` column to the `vmTable`.
- [ ] **Cloud Provider Recommender APIs**
  - [ ] Pull data from AWS Compute Optimizer (e.g., "Overprovisioned: downsize to t3.micro").
  - [ ] Pull data from GCP Recommender (e.g., "Idle VM: terminate").
- [ ] **Gemini "FinOps Mode"**
  - [ ] Create a new UI Pane or Mode (`Alt+F` to toggle FinOps Mode).
  - [ ] Integrate the official Gemini Go SDK.
  - [ ] **Prompt Engineering:** Pass the currently selected VM's metadata (CPU, RAM, tags, uptime, provider recommendations, and cost) to Gemini to generate actionable, human-readable insights.
  - [ ] *Example output in TUI:* "This GCP e2-standard-4 has been running for 30 days but GCP Recommender suggests it's idle. You are spending $98/mo. I recommend stopping it immediately or creating an automated schedule."

## Phase 6: Polish & Advanced Features
- [ ] **Bulk Actions:** Multi-select VMs (using `Space`) to start/stop an entire environment simultaneously.
- [ ] **Advanced Querying:** Replace simple fuzzy search with a query language (e.g., `status:running provider:aws env:prod`).
- [ ] **Background Sync:** Implement a background ticker to periodically fetch state updates without blocking the UI.
- [ ] **In-App Logs:** Action to stream CloudWatch/Serial Console logs directly into a Bubble Tea viewport.
