package core

import "strings"

type ManualHost struct {
	Name          string
	Provider      string
	Host          string
	Username      string
	Connection    string
	SSHConfigHost string
	KeyPath       string
	KeyRef        string
	PasswordRef   string
	Tags          []string
}

var DefaultManualHostColumns = []string{
	"Name", "Host", "User", "Connection", "Provider", "Auth", "Tags",
}

func (h ManualHost) ID() string {
	if h.Name != "" {
		return h.Name
	}
	if h.Username != "" {
		return h.Username + "@" + h.Host
	}
	return h.Host
}

func (h ManualHost) AuthSummary() string {
	switch {
	case strings.TrimSpace(h.SSHConfigHost) != "":
		return "ssh-config:" + strings.TrimSpace(h.SSHConfigHost)
	case strings.TrimSpace(h.KeyRef) != "":
		return "key-ref:" + strings.TrimSpace(h.KeyRef)
	case strings.TrimSpace(h.KeyPath) != "":
		return "key-path"
	case strings.TrimSpace(h.PasswordRef) != "":
		return "password-ref"
	default:
		return "default"
	}
}

func (h ManualHost) ToVM() VM {
	host := strings.TrimSpace(h.Host)
	name := strings.TrimSpace(h.Name)
	if name == "" {
		name = host
	}
	provider := strings.TrimSpace(h.Provider)
	if provider == "" {
		provider = "Manual"
	}
	return VM{
		Name:      name,
		ID:        h.ID(),
		Type:      "manual-host",
		State:     "manual",
		PublicIP:  host,
		Labels:    strings.Join(h.Tags, ","),
		Zone:      "global",
		Network:   provider,
		Subnet:    strings.TrimSpace(h.Username),
		PrivateIP: "",
	}
}
