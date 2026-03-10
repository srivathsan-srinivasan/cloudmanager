# CloudManager TUI - Development Tasks

This document outlines the roadmap for upgrading CloudManager from a prototype to a fully functional, production-ready TUI for managing VMs across AWS, GCP, and Azure.

## Phase 1: Context & Configuration Discovery (Current Focus)
The goal of this phase is to dynamically populate the left sidebar ("Cloud Contexts") with the user's actual local configurations instead of mock data.

- [ ] **AWS Configuration Parser**
  - [ ] Parse `~/.aws/config` and `~/.aws/credentials` to extract available profiles.
  - [ ] For each profile, identify configured regions. If no region is strictly defined per profile, provide a mechanism (or default list) to query the 17 target regions.
  - [ ] Map profiles and regions to the `contextItem` struct.
- [ ] **GCP Configuration Parser**
  - [ ] Parse `~/.config/gcloud/configurations/` (or use `gcloud config configurations list --format=json`).
  - [ ] Extract project IDs and default compute regions/zones.
  - [ ] Map projects and regions to the `contextItem` struct.
- [ ] **Azure Configuration Parser**
  - [ ] Parse Azure CLI configuration (typically `~/.azure/azureProfile.json` or via `az account list --output json`).
  - [ ] Extract Subscription IDs, names, and default locations.
  - [ ] Map subscriptions and locations to the `contextItem` struct.
- [ ] **Integration**
  - [ ] Update `main.go` -> `initialModel()` to call these parsers concurrently on startup.
  - [ ] Handle missing configurations gracefully (e.g., if Azure is not installed, only show AWS/GCP).

## Phase 2: Read-Only Integration (Data Fetching)
Replace the `fetchVMsMock` function with actual CLI calls to fetch live VM states.

- [x] **AWS EC2 Fetcher**
  - [x] Implement `aws ec2 describe-instances --profile <profile> --region <region> --output json`.
  - [x] Parse JSON output into `[]table.Row` (Name, ID, Type, State, IPs).
- [x] **GCP Compute Fetcher**
  - [x] Implement `gcloud compute instances list --project <project> --format=json`.
  - [x] Parse JSON output into `[]table.Row`.
- [x] **Azure VM Fetcher**
  - [x] Implement `az vm list -d --subscription <sub-id> --output json`.
  - [x] Parse JSON output into `[]table.Row`.
- [x] **Async Loading & Error Handling**
  - [x] Ensure slow CLI calls do not block the UI.
  - [x] Display loading spinners or progress indicators in the VMs pane.
  - [x] Handle authentication expiration or CLI errors and display them as alerts in the TUI.

## Phase 3: Action Execution (State Mutation)
Wire up the Action Dropdown to perform actual operations on the VMs.

- [x] **Start/Stop/Restart/Terminate Interfaces**
  - [x] AWS: `aws ec2 start-instances`, `stop-instances`, `reboot-instances`, `terminate-instances`.
  - [x] GCP: `gcloud compute instances start`, `stop`, `reset`, `delete`.
  - [x] Azure: `az vm start`, `stop`, `restart`, `delete`.
- [ ] **Confirmation Dialogs**
  - [ ] Implement a TUI confirmation modal (Yes/No) before executing destructive actions like "Terminate".
- [x] **Command Execution Feedback**
  - [x] Stream stdout/stderr from the CLI commands to the status bar or a dedicated log pane.
  - [ ] Auto-refresh the VM list for the current context after an action completes.

## Phase 4: SSH & Interactive Sessions
- [x] **SSH Integration**
  - [x] Implement SSH action logic.
  - [x] AWS: Use SSM Session Manager (`aws ssm start-session`) or standard SSH with key pairs.
  - [x] GCP: Use Identity-Aware Proxy (`gcloud compute ssh`).
  - [x] Azure: Use Bastion or standard SSH (`az ssh vm`).
  - [x] Suspend the Bubble Tea application (`tea.Exec`), drop the user into the native terminal SSH session, and restore the TUI when they exit.

## Phase 5: Polish & Performance
- [x] **Caching**
  - [x] Cache instance lists locally for a few minutes to speed up context switching.
  - [x] Add a manual "Refresh" shortcut (e.g., `r`).
- [x] **Global Search / Filtering**
  - [x] Allow users to press `/` to search across all VMs in all contexts.
- [x] **Custom Keybindings**
  - [x] Read a `~/.cloudmanager.json` (or yaml) file to allow users to override default keybindings, caching ttl, and themes via Viper.
