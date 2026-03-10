# CloudManager Vision & Strategy

## Why does CloudManager exist?
Cloud computing environments have become incredibly complex. Developers, SysAdmins, and DevOps engineers often manage infrastructure spanning across AWS, Google Cloud Platform (GCP), and Azure. Switching between different web consoles or remembering distinct CLI commands (`aws ec2...`, `gcloud compute...`, `az vm...`) just to perform basic operations like starting a VM or fetching an IP address introduces massive cognitive load and friction.

CloudManager exists to unify multi-cloud virtual machine management into a single, lightning-fast Terminal User Interface (TUI). By bringing the cloud to the terminal—where engineers already spend their time—it minimizes context switching and accelerates daily operational workflows.

## Who needs this?
- **DevOps/Platform Engineers:** Managing disparate environments across providers. Need a fast way to bounce boxes, check states, or SSH into instances without navigating clunky web UIs.
- **Backend Developers:** Needing to quickly start up testing environments, locate dynamic IP addresses, or tail logs without leaving their IDE terminal.
- **FinOps Practitioners & Team Leads:** Looking for a quick, unified overview of running resources to identify waste (zombie VMs) across the entire company portfolio.
- **Freelancers/Consultants:** Working with multiple clients, each using a different cloud provider. A single pane of glass saves hours of re-authenticating and switching contexts.

## How useful is it?
CloudManager is designed for **speed and utility**. 
1. **Zero Context Switching:** View AWS EC2s, GCP Compute Engines, and Azure VMs side-by-side using unified keybindings.
2. **Instant Actions:** Start, Stop, Terminate, or SSH into any instance across any cloud with just two keystrokes (`Enter` -> `Enter`).
3. **Hybrid Authentication:** By supporting both a CLI-wrapper backend and a native Go SDK backend, users can either bring their existing complex CLI authentication profiles for zero setup, or use the native SDKs for maximum performance.
4. **Intelligent FinOps (Upcoming):** Integrating LLMs (Gemini) and cloud provider recommendation APIs to not just manage infrastructure, but actively analyze it for cost-saving opportunities directly in the terminal.
