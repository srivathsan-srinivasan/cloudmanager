package digitalocean

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/vyoogam/cloudmanager/internal/core"
)

// LoadContexts discovers the current DigitalOcean account from doctl.
func LoadContexts() ([]core.CloudContext, []string) {
	if _, err := exec.LookPath("doctl"); err != nil {
		return nil, []string{"DigitalOcean: 'doctl' CLI not found (install from https://docs.digitalocean.com/reference/doctl/)"}
	}

	cmd := exec.Command("doctl", "account", "get", "--format", "UUID,Team,Email", "--no-header")
	output, err := cmd.Output()
	if err != nil {
		return nil, []string{fmt.Sprintf("DigitalOcean: failed to inspect account (run 'doctl auth init'): %v", err)}
	}

	line := strings.TrimSpace(string(bytes.TrimSpace(output)))
	if line == "" {
		return nil, []string{"DigitalOcean: no account information returned by doctl"}
	}

	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil, []string{"DigitalOcean: could not parse account information"}
	}

	accountID := fields[0]
	accountName := accountID
	if len(fields) >= 2 {
		accountName = fields[1]
	} else if len(fields) >= 3 {
		accountName = fields[2]
	}

	return []core.CloudContext{
		{
			Provider:    "DigitalOcean",
			AccountID:   accountID,
			AccountName: accountName,
			Region:      "global",
		},
	}, nil
}
