# CloudManager ☁️

CloudManager is a fast, terminal-based user interface (TUI) for managing virtual machines across multiple cloud providers (AWS, GCP, Azure). Think of it as **[k9s](https://github.com/derailed/k9s) for your cloud infrastructure**.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea), CloudManager provides a modern, keyboard-centric interface to view, filter, start, stop, and SSH into your cloud instances without leaving your terminal.

---

## ✨ Features

- **Multi-Cloud Dashboard:** View all your VMs across AWS (EC2), Google Cloud (Compute Engine), and Azure (Virtual Machines) in a single unified interface.
- **Lightning Fast:** Powered by Go and Bubble Tea. Local caching keeps navigation snappy.
- **Direct SSH Access:** Drop directly into an interactive SSH session with your instances using native tools (SSM Session Manager, IAP, Azure Bastion).
- **Vim-like Keybindings:** Easily navigate your infrastructure entirely via keyboard.
- **Highly Configurable:** Customize columns, sort fields, color themes, and keybindings via a central configuration file (`~/.cloudmanager.json`).
- **Safety First:** Actions like "Terminate" require explicit confirmation (coming soon!).

## 🚀 Installation

Ensure you have [Go](https://golang.org/doc/install) (1.20+) installed.

```bash
git clone https://github.com/yourusername/cloudmanager.git
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
- `↑` / `↓` / `k` / `j`: Navigate lists
- `Enter`: Select an item or execute an action
- `Tab`: Switch focus between Sidebar (Contexts) and Main View (VMs)
- `b`: Toggle Sidebar visibility
- `c`: Configure GCP Projects
- `C`: Configure visible VM Table columns
- `S`: Select a column to sort by
- `/`: Search/filter the current VM list
- `r`: Force refresh the VM list (bypasses cache)
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

Currently, we are focusing on **Phase 1: Cloud Context Parsers** (dynamically parsing local `~/.aws/config`, `~/.config/gcloud`, etc., rather than relying on mock configuration).

## 🤝 Contributing
Contributions, issues, and feature requests are welcome!
Feel free to check [issues page](https://github.com/yourusername/cloudmanager/issues).

Please review the [Contributing Guide](CONTRIBUTING.md) and the [Code of Conduct](CODE_OF_CONDUCT.md).

## 📝 License
This project is [MIT](LICENSE) licensed.
