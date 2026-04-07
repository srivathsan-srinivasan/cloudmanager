package azure

import (
	"encoding/json"
	"fmt"
	"os/exec"

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
		Id   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(output, &accounts); err != nil {
		return contexts, []string{fmt.Sprintf("Azure: failed to parse subscription list: %v", err)}
	}

	for _, acc := range accounts {
		contexts = append(contexts, core.CloudContext{
			Provider: "Azure", AccountID: acc.Id,
			AccountName: acc.Name, Region: "global",
		})
	}
	return contexts, warnings
}
