package azure

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"cloudmanager/internal/core"
)

// LoadContexts lists Azure subscriptions using the az CLI.
func LoadContexts() ([]core.CloudContext, []string) {
	var contexts []core.CloudContext
	var warnings []string

	if _, err := exec.LookPath("az"); err != nil {
		return contexts, []string{"Azure: 'az' CLI not found (install from https://aka.ms/installazurecli)"}
	}

	cmd := exec.Command("az", "account", "list", "--output", "json")
	output, err := cmd.Output()
	if err != nil {
		return contexts, []string{fmt.Sprintf("Azure: failed to list subscriptions (run 'az login'): %v", err)}
	}

	var accounts []struct {
		Id       string `json:"id"`
		Name     string `json:"name"`
		TenantID string `json:"tenantId"`
	}
	if err := json.Unmarshal(output, &accounts); err != nil {
		return contexts, []string{fmt.Sprintf("Azure: failed to parse subscription list: %v", err)}
	}

	for _, acc := range accounts {
		contexts = append(contexts, core.CloudContext{
			Provider:    "Azure",
			ContextName: azureContextName(acc.Name, acc.Id),
			AccountID:   acc.Id,
			AccountName: acc.Name,
			Tenant:      acc.TenantID,
			Region:      "global",
		})
	}
	return contexts, warnings
}

func azureContextName(name, fallback string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "_", "-")
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteRune('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out != "" {
		return out
	}
	return strings.ToLower(strings.TrimSpace(fallback))
}
