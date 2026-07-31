package core

// AccessMethod describes one way to reach a VM or machine.
type AccessMethod struct {
	ID        string
	Kind      string
	Label     string
	Priority  int
	Command   []string
	CopyText  string
	Available bool
	Reason    string
}

// TunnelMode represents the tunneling strategy for port forwarding and SCP.
type TunnelMode string

const (
	TunnelModeDirect TunnelMode = "direct" // Standard SSH -L (requires public IP)
	TunnelModeIAP    TunnelMode = "iap"    // GCP Identity-Aware Proxy tunnel
	TunnelModeSSM    TunnelMode = "ssm"    // AWS Systems Manager Session Manager
)

// PortForwardSpec describes a single port forwarding rule.
type PortForwardSpec struct {
	LocalPort  int    // Local port to bind
	RemotePort int    // Remote port on VM
	LocalHost  string // Default "localhost" or "0.0.0.0"
	RemoteHost string // Default "localhost" on remote side
}

// SCPTransfer describes a file transfer operation.
type SCPTransfer struct {
	Source      string // Local or remote path
	Destination string // Local or remote path
	Direction   string // "push" (local→remote) or "pull" (remote→local)
	Recursive   bool   // -r flag for directories
}
