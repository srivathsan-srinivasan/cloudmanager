package hosts

import (
	"strings"

	"github.com/vyoogam/cloudmanager/internal/config"
	"github.com/vyoogam/cloudmanager/internal/core"
)

func FromConfig(cfg config.AppConfig) []core.ManualHost {
	hosts := make([]core.ManualHost, 0, len(cfg.ManualHosts))
	for _, raw := range cfg.ManualHosts {
		raw = config.SanitizeManualHost(raw)
		if strings.TrimSpace(raw.Host) == "" {
			continue
		}
		hosts = append(hosts, core.ManualHost{
			Name:          raw.Name,
			Provider:      raw.Provider,
			Host:          raw.Host,
			Username:      raw.Username,
			Connection:    raw.Connection,
			SSHConfigHost: raw.SSHConfigHost,
			KeyPath:       raw.KeyPath,
			KeyRef:        raw.KeyRef,
			PasswordRef:   raw.PasswordRef,
			Tags:          raw.Tags,
		})
	}
	return hosts
}

func ToVMs(cfg config.AppConfig) []core.VM {
	manualHosts := FromConfig(cfg)
	vms := make([]core.VM, 0, len(manualHosts))
	for _, host := range manualHosts {
		vms = append(vms, host.ToVM())
	}
	return vms
}
