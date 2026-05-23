# CloudManager

[![GitHub release downloads](https://img.shields.io/github/downloads/vyoogam/cloudmanager/total?style=flat-square&label=release%20downloads)](https://github.com/vyoogam/cloudmanager/releases)
![README visitors](https://visitor-badge.laobi.icu/badge?page_id=vyoogam.cloudmanager&left_text=repo%20visits)

CloudManager is a fast, terminal control plane for cloud operations.

It is built for **speed, firefighting, and instantaneous access** to the cloud
resources operators touch every day. Browser consoles are broad, slow,
click-heavy, and inconsistent across providers. SDKs are powerful, but they
expect every team to script its own workflows. CloudManager sits in the middle:
a keyboard-first terminal runtime for cloud inventory, access, search, tagging,
actions, and auditability.

CloudManager is not trying to replace cloud consoles or cloud SDKs. It uses the
provider-native CLIs and SDKs where they are strongest, then gives NOC, SOC,
Red Team, DevOps, SecOps, and Platform engineers a single terminal workflow for
fast, traceable, multi-cloud operations.

Think of it as **[k9s](https://github.com/derailed/k9s) for cloud operations**:
provider-aware, terminal-native, indexed, auditable, and built for the moments
when clicking through five browser tabs is too slow.

## The Soul Of CloudManager

CloudManager exists to make frequently used cloud resources immediately
reachable from the terminal, with enough context to act safely.

Core promises:

- **Speed first:** fast startup, keyboard-first navigation, cached local indexes,
  and scoped find flows.
- **Firefighting first:** surface the resources, access paths, metadata,
  health, and actions operators need during incidents.
- **Auditability first:** show what context is active, what command/action is
  being run, what was accessed, and what changed.
- **Provider-aware, not provider-blind:** AWS, GCP, Azure, DigitalOcean, manual
  hosts, and future providers keep their native identity and behavior.
- **Local control:** CloudManager-owned tags, access memory, and inventory can
  work across clouds without mutating provider metadata by default.
- **Small core, extensible edge:** the core binary stays focused; optional
  components add cloud services such as Pub/Sub, queues, DNS, WAF, secrets,
  IAM analysis, and incident-response packs.

The goal is simple: when something breaks, CloudManager should get the right
engineer to the right resource faster than a browser console can.

---

## Features

### Operator Cockpit

- Multi-cloud dashboard for AWS, GCP, Azure, DigitalOcean, Kubernetes, and manual hosts.
- Keyboard-first Bubble Tea TUI with tabs for VMs, Disks, Snapshots, Firewalls, Clusters, Databases, Networks, Storage, and Hosts.
- Home dashboard with indexed counts, running/stopped state, public IPs, database health, Kubernetes visibility, storage, networks, firewalls, and manual hosts.
- In-app Help view with `?`, `F1`, and `:help`.
- Application logs view with `:logs`.
- Adaptive table colors for light and dark terminals.
- Configurable visible columns and sortable headers across resource tables and Find.

### Cloud Inventory And Search

- Local SQLite-backed VM and resource inventory cache.
- JSON cache fallback during migration from older installs.
- Scoped Find flows: `:find-vms`, `:find-dbs`, `:find-k8s`, `:find-storage`, `:find-hosts`, and `:find-all`.
- Public IP drill-down from dashboard to filtered VM Find results.
- CSV export for known indexed public endpoints.
- Terraform state cross-reference for VMs, databases, and Kubernetes clusters.
- CloudManager-local tags across clouds without mutating provider metadata by default.

### Provider Coverage

- AWS VM, disk, snapshot, firewall/security group, database, cluster, network, subnet, storage, billing, metrics, and recommendation surfaces.
- GCP VM, disk, snapshot, firewall, Cloud SQL, GKE, network, storage, billing, metrics, and recommendation surfaces.
- Azure VM, disk, snapshot, firewall, PostgreSQL, AKS, network, storage, billing, metrics, and recommendation surfaces.
- DigitalOcean VM/database/cluster parser support where available.
- Manual SSH/RDP-style host inventory for resources outside the major cloud APIs.

### Access And Actions

- Provider-native VM actions: start, stop, restart, terminate, describe, and open console where supported.
- Direct SSH access through native provider tools where available.
- Learned SSH access memory stored locally in SQLite.
- Manual host reachability checks with SSH first, then ping fallback.
- Firewall rule editing with provider-aware guardrails.
- Add current public IP to firewall rules when SDK mutation mode is available.
- Copy IDs, console URLs, public endpoints, and detail output from resource views.

### FinOps And Health

- VM-level metrics and utilization indicators.
- Monthly cost enrichment and per-resource cost display.
- Provider recommendation hooks for rightsizing and cost cleanup.
- Dashboard summaries for database and Kubernetes health.
- Cache-aware async enrichment to keep the TUI responsive.

### Local-First Safety

- Uses native CLIs and SDKs; credentials stay in the user's environment.
- CloudManager-owned config lives under the user's home directory.
- Tags, access memory, inventory cache, and logs are local by default.
- Mutating actions are explicit and provider-aware.
- Component roadmap keeps the core binary focused while allowing service-specific expansion.

## Core vs Components

CloudManager core should remain small and reliable:

- contexts, profiles, and provider auth
- inventory index and local cache
- dashboard and scoped Find Resources
- VM/resource access resolution
- CloudManager-local tags and annotations
- audit/event log
- provider capability registry
- TUI shell

Optional components should add cloud-service depth without bloating the core
binary:

- Pub/Sub, queues, functions, DNS, CDN, WAF
- IAM access analysis and security posture views
- load balancers and target health
- secrets/key vault references
- incident-response packs
- external data engines such as Steampipe, CloudQuery, Cloudlist, and Prowler

The intended contributor contract is narrow: a component declares what it can
list, find, describe, render, and safely act on. CloudManager supplies the
terminal workflow, active context, audit path, and provider-aware guardrails.

## Installation

Ensure you have [Go](https://golang.org/doc/install) 1.25 or newer installed.

```bash
go install github.com/vyoogam/cloudmanager@latest
```

`go get` is no longer the right way to install Go binaries on modern Go. Use
`go install ...@version`. To pin this release:

```bash
go install github.com/vyoogam/cloudmanager@v1.0.0
```

From source:

```bash
git clone https://github.com/vyoogam/cloudmanager.git
cd cloudmanager
go build -o cloudmanager .
```

Homebrew tap:

```bash
brew tap vyoogam/cloudmanager https://github.com/vyoogam/cloudmanager
brew install cloudmanager
```

### Release Automation

Releases can be created from GitHub Actions:

1. Open **Actions** in GitHub.
2. Select **Release Go Module**.
3. Click **Run workflow**.
4. Enter a stable version such as `v1.0.2`.
5. Run it from the release branch.

The workflow runs tests, updates `VERSION`, the README install pin, and the
Homebrew formula, commits the release bump, creates the tag, pushes it, and
publishes GitHub release artifacts with GoReleaser.

Local releases are also supported:

```bash
scripts/release v1.0.2
```

The script requires a clean worktree, runs `go test ./...`, updates `VERSION`,
the pinned Go install command, and the Homebrew formula, commits the release
bump, creates an annotated tag, then pushes the branch and tag. Pushing the tag
triggers GoReleaser to publish GitHub release artifacts.

### Prerequisites
CloudManager wraps the native CLI tools for the respective cloud providers. Ensure you have the following installed and authenticated if you intend to manage resources in those clouds:
- **AWS:** [`aws-cli`](https://aws.amazon.com/cli/) + `aws configure`
- **GCP:** [`gcloud`](https://cloud.google.com/sdk/gcloud) + `gcloud auth login --no-browser`
- **Azure:** [`az`](https://learn.microsoft.com/en-us/cli/azure/install-azure-cli) + `az login`
- **DigitalOcean:** [`doctl`](https://docs.digitalocean.com/reference/doctl/) + `doctl auth init`

You can check, install, or update common prerequisites with:

```bash
scripts/install-prereqs --check --all
scripts/install-prereqs --install --core --cloud
scripts/install-prereqs --update --all
```

The script supports Homebrew first-class on macOS and best-effort `apt` installs
on Linux. It never runs login flows; authenticate through CloudManager `:login`
or the native CLIs after installation.

Inside CloudManager, run `:login` to launch wrapped native login flows and refresh discovered contexts afterward.

## Usage

Run the binary in your terminal:

```bash
cloudmanager
```

### Profiles / Contexts

CloudManager stores human-friendly context names on top of provider-native accounts, projects, and subscriptions. For Azure this avoids `az account set` side effects; commands can resolve the selected context and pass `--subscription` explicitly.

The TUI is the primary place to manage these. Open `,` Settings, then `Profiles / Contexts`, or run `:profiles`.

- `a`: add profile/context
- `e` / `Enter`: edit selected profile/context
- `u`: set selected profile/context as current
- `l`: login for the selected profile/context
- `z`: import Azure subscriptions from `az account list`
- `d`: remove selected profile/context, with confirmation
- `r`: discover contexts from installed cloud CLIs
- `b`: back up config

General discovery is selective: `:discover` scans installed provider CLIs, opens a checked import list, and imports only the contexts you explicitly select. This avoids onboarding every visible GCP project or Azure subscription by accident.

Azure import opens a subscription picker. New subscriptions are selected by default; already saved subscriptions are marked `saved`. Re-importing an existing subscription refreshes its subscription name and tenant but keeps your CloudManager-friendly context name.

```bash
cloudmanager profile add eng \
  --provider azure \
  --tenant example.com \
  --subscription-id 00000000-0000-0000-0000-000000000000 \
  --subscription-name "Azure Sponsorship - Engineering"

cloudmanager profile list
cloudmanager profile use eng
cloudmanager profile current
cloudmanager login eng
cloudmanager tui
```

`profile` and `context` are aliases. AWS profiles discovered from local config become CloudManager contexts using the AWS profile name.

### Default Keybindings
- `1` - `9`: Switch between resource views (VMs, Disks, Snapshots, Firewalls, Clusters, Databases, Networks, Storage, Hosts)
- `↑` / `↓` / `k` / `j`: Navigate lists
- `Enter`: Select an item or execute an action
- `?` / `F1`: Open the in-app help / shortcuts page
- `/` in Help: Filter shortcuts
- `Tab`: Switch focus between Sidebar (Contexts) and Main View
- `b`: Toggle Sidebar visibility
- `H`: Return to the home dashboard
- `K`: Toggle Kubernetes worker nodes in VM lists/search/dashboard
- `t`: Add CloudManager-only tags to the selected resource where supported
- `,`: Open Settings
- `g`: Open the scoped Find picker
- `:help`: Open the in-app help / shortcuts page
- `:logs`: Open application logs
- `:dashboard`: Open the home dashboard
- `:find-vms`, `:find-dbs`, `:find-k8s`, `:find-storage`, `:find-hosts`, `:find-all`: Find across indexed resources by scope
- `:summary`: Refresh database and Kubernetes dashboard summary counts
- `:login`: Run wrapped provider CLI login flows
- `:add-provider`: Add a managed provider context with auth mode and persistence policy
- `:add-host`: Add a manual SSH/RDP host without cloud provider credentials
- `:discover`: Scan local cloud CLIs for contexts on demand
- `:index`: Refresh the persisted VM index
- `:index-db`: Refresh the database index
- `:index-storage`: Refresh the storage index
- `:creds`: Manage CloudManager managed contexts
- `c`: Copy the current VM detail/remediation output when the detail pane is open
- `C`: Configure visible table columns
- `S`: Select a column to sort by
- `/`: Search/filter the current list
- `r`: Force refresh the current view
- `Esc` / `q`: Go back or quit the application


## 🛠️ Configuration
Upon first run, a default configuration file will be created at `~/.cloudmanager.json`. 

```json
{
  "current_context": "eng",
  "gcp_configured": true,
  "gcp_projects": ["my-production-project"],
  "cloud_contexts": [
    {
      "context_name": "prod-admin",
      "provider": "AWS",
      "account_id": "000000000000",
      "account_name": "prod",
      "auth_mode": "native-cli",
      "credential_persistence": "native-cli",
      "credential_profile": "prod-admin",
      "regions": ["us-east-1"]
    },
    {
      "context_name": "eng",
      "provider": "Azure",
      "account_id": "00000000-0000-0000-0000-000000000000",
      "account_name": "Azure Sponsorship - Engineering",
      "tenant": "example.com",
      "auth_mode": "native-cli",
      "credential_persistence": "native-cli",
      "regions": ["global"]
    }
  ],
  "manual_hosts": [
    {
      "name": "hetzner-web-01",
      "provider": "Hetzner",
      "host": "203.0.113.10",
      "username": "root",
      "connection": "ssh",
      "ssh_config_host": "",
      "key_path": "~/.ssh/hetzner",
      "key_ref": "",
      "password_ref": "",
      "tags": ["hetzner", "prod"]
    }
  ],
  "resource_tags": [
    {
      "provider": "GCP",
      "account_id": "project-a",
      "region": "us-central1",
      "resource_kind": "VM",
      "resource_id": "gce-instance-id",
      "resource_name": "vf-web-1",
      "private_ip": "10.10.0.5",
      "public_ip": "",
      "tags": ["VFWEB"]
    }
  ],
  "terraform_state_paths": [
    "./terraform.tfstate",
    "/secure/path/prod.tfstate"
  ],
  "vm_columns": ["Name", "Instance ID", "State", "Private IP", "Public IP"],
  "storage_columns": ["Name", "Provider Type", "Region", "Access", "Encrypted", "Versioning", "Created At"],
  "cache_ttl_minutes": 5,
  "global_search_enabled": true,
  "discover_contexts_on_start": false,
  "prefetch_on_start": false,
  "prefetch_resources": ["vms", "databases", "storage"],
  "prefetch_concurrency": 4,
  "vm_index_persistence_enabled": true,
  "vm_index_cache_ttl_hours": 24,
  "resource_index_persistence_enabled": true,
  "resource_index_cache_ttl_hours": 24,
  "hide_kubernetes_nodes": true,
  "dashboard_widgets": [
    "contexts",
    "indexed_vms",
    "running_vms",
    "public_ips",
    "disks",
    "snapshots",
    "networks",
    "subnets",
    "firewalls",
    "databases",
    "storage",
    "kubernetes",
    "terraform",
    "manual_hosts"
  ],
  "dashboard_theme": "btop",
  "theme": {
    "subtle": "#D9DCCF",
    "highlight": "#874BFD",
    "special": "#43BF6D",
    "info": "#38BDF8",
    "amber": "#F59E0B",
    "status_running": "#22C55E",
    "status_available": "#06B6D4",
    "status_ready": "#A3E635",
    "status_in_use": "#60A5FA",
    "status_starting": "#38BDF8",
    "status_stopping": "#FB923C",
    "status_stopped": "#F59E0B",
    "status_terminated": "#EF4444",
    "status_deallocated": "#A78BFA",
    "status_unknown": "#737373",
    "status_reachable": "#10B981",
    "status_unreachable": "#F97316",
    "column_name": "#A855F7",
    "column_id": "#818CF8",
    "column_ip": "#06B6D4",
    "column_provider": "#F472B6",
    "column_region": "#2DD4BF",
    "column_type": "#FBBF24",
    "column_meta": "#94A3B8",
    "alert": "#FF5F87"
  },
  "keybindings": {
    "refresh": "r",
    "search": "/"
  }
}
```
*Note: The configuration file can also be formatted as YAML (`~/.cloudmanager.yaml`).*

Terraform state cross-reference is read-only. CloudManager reads the configured local state files, caches parsed metadata by file timestamp, and marks matched VMs, databases, and Kubernetes clusters with labels like `iac:terraform` and `tf:<address>`. It does not run Terraform, copy the state file, mutate Terraform state, or change cloud resources.

A fake state file for local testing is available at `examples/terraform/fake-cloudmanager.tfstate`. Add that path to `terraform_state_paths`, then use matching fake resource names/IDs such as `cm-demo-web-01`, `cm-demo-worker-01`, `cm-demo-orders-db`, or `cm-demo-gke` in test fixtures to see Terraform labels applied.

## Roadmap
See [docs/ROADMAP.md](docs/ROADMAP.md) for the current product and engineering roadmap.

Currently, we are focusing on unifying cloud context parsers and expanding our Bubble Tea implementation.

**🚀 Upcoming Feature: K9s Integration**
We are working on direct integration to **jump directly into [K9s](https://github.com/derailed/k9s) from the existing terminal!** This means you can seamlessly bridge VM management and Kubernetes cluster management without context switching.

## Contributing & Feature Requests
Contributions, issues, and feature requests are welcome!

CloudManager is designed for community components. Cloud platforms expose
hundreds of services; the core project should not hard-code every one of them.
Contributors should be able to add focused components for the services they
operate every day while inheriting CloudManager's terminal UX, auditability, and
context model.

Useful contribution areas:

- new resource components
- provider support
- incident-response workflows
- IAM/access visibility
- local inventory and SQLite indexing
- audit and telemetry
- terminal UX polish

👉 **[Request a Feature or Open an Issue](https://github.com/vyoogam/cloudmanager/issues)**

Please review the [Contributing Guide](CONTRIBUTING.md) and the [Code of Conduct](CODE_OF_CONDUCT.md) before submitting PRs.

## License
This project is [MIT](LICENSE) licensed.
