# CloudManager ☁️

CloudManager is a fast, terminal-based user interface (TUI) for managing virtual machines across multiple cloud providers (AWS, GCP, Azure). Think of it as **[k9s](https://github.com/derailed/k9s) for your cloud infrastructure**.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea), CloudManager provides a modern, keyboard-centric interface to view, filter, start, stop, and SSH into your cloud instances without leaving your terminal.

---

## ✨ Features

- **Multi-Cloud Dashboard:** View all your VMs, Disks, Snapshots, and Networks across AWS, Google Cloud, and Azure.
- **FinOps Intelligence:** Get actionable AI-powered cost and architecture recommendations via Gemini 1.5 Flash.
- **Live Metrics:** Real-time CPU and memory utilization indicators (🟢🟡🔴) integrated into your resource tables.
- **Cost Transparency:** Track monthly spending and cost trends for individual instances and entire accounts.
- **Direct SSH Access:** Drop directly into an interactive SSH session with your instances using native tools.
- **Lightning Fast:** Powered by Go and Bubble Tea with smart async enrichment and caching.

## 🚀 Installation

Ensure you have [Go](https://golang.org/doc/install) (1.20+) installed.

```bash
git clone https://github.com/srivathsan-srinivasan/cloudmanager.git
cd cloudmanager
go build -o cloudmanager
sudo mv cloudmanager /usr/local/bin/
```

### Prerequisites
CloudManager wraps the native CLI tools for the respective cloud providers. Ensure you have the following installed and authenticated if you intend to manage resources in those clouds:
- **AWS:** [`aws-cli`](https://aws.amazon.com/cli/) + `aws configure`
- **GCP:** [`gcloud`](https://cloud.google.com/sdk/gcloud) + `gcloud auth login`
- **Azure:** [`az`](https://learn.microsoft.com/en-us/cli/azure/install-azure-cli) + `az login`

## ⌨️ Usage

Run the binary in your terminal:

```bash
cloudmanager
```

### Default Keybindings
- `1` - `6`: Switch between resource views (VMs, Disks, Snapshots, Firewalls, Clusters, Networks)
- `↑` / `↓` / `k` / `j`: Navigate lists
- `Enter`: Select an item or execute an action
- `Tab`: Switch focus between Sidebar (Contexts) and Main View
- `b`: Toggle Sidebar visibility
- `C`: Configure visible table columns
- `S`: Select a column to sort by
- `/`: Search/filter the current list
- `r`: Force refresh the current view
- `Esc` / `q`: Go back or quit the application


## 🛠️ Configuration
Upon first run, a default configuration file will be created at `~/.cloudmanager.json`. 

```json
{
  "gcp_configured": true,
  "gcp_projects": ["my-production-project"],
  "vm_columns": ["Name", "Instance ID", "State", "Private IP", "Public IP"],
  "cache_ttl_minutes": 5,
  "theme": {
    "subtle": "#D9DCCF",
    "highlight": "#874BFD",
    "special": "#43BF6D",
    "alert": "#FF5F87"
  },
  "keybindings": {
    "refresh": "r",
    "search": "/"
  }
}
```
*Note: The configuration file can also be formatted as YAML (`~/.cloudmanager.yaml`).*

## 🗺️ Roadmap
See [tasks.md](tasks.md) for our detailed development roadmap. 

Currently, we are focusing on unifying cloud context parsers and expanding our Bubble Tea implementation.

**🚀 Upcoming Feature: K9s Integration**
We are working on direct integration to **jump directly into [K9s](https://github.com/derailed/k9s) from the existing terminal!** This means you can seamlessly bridge VM management and Kubernetes cluster management without context switching.

## 🤝 Contributing & Feature Requests
Contributions, issues, and feature requests are welcome!

**We want to hear from you!** What feature would make your life significantly easier? What tool or workflow are you missing? 
Drop your ideas to help shape the future of CloudManager. If something would help you immensely, please request it!

👉 **[Request a Feature or Open an Issue](https://github.com/srivathsan-srinivasan/cloudmanager/issues)**

Please review the [Contributing Guide](CONTRIBUTING.md) and the [Code of Conduct](CODE_OF_CONDUCT.md) before submitting PRs.

## 📝 License
This project is [MIT](LICENSE) licensed.
