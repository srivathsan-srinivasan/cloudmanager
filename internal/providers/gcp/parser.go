package gcp

import (
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/vyoogam/cloudmanager/internal/config"
	"github.com/vyoogam/cloudmanager/internal/core"
)

// LoadContexts fetches GCP projects via gcloud CLI.
func LoadContexts() ([]core.CloudContext, []string) {
	var contexts []core.CloudContext
	var warnings []string

	if _, err := exec.LookPath("gcloud"); err != nil {
		return contexts, []string{"GCP: 'gcloud' CLI not found (install from https://cloud.google.com/sdk)"}
	}

	cmd := exec.Command("gcloud", "projects", "list", "--format=json(projectId)")
	output, err := cmd.Output()
	if err != nil {
		return contexts, []string{fmt.Sprintf("GCP: failed to list projects (run 'gcloud auth login'): %v", err)}
	}

	var projects []struct {
		ProjectId string `json:"projectId"`
	}
	if err := json.Unmarshal(output, &projects); err != nil {
		return contexts, []string{fmt.Sprintf("GCP: failed to parse project list: %v", err)}
	}

	cfg := config.Load()
	for _, p := range projects {
		if config.IsGCPProjectSelected(p.ProjectId, cfg) {
			contexts = append(contexts, core.CloudContext{
				Provider: "GCP", AccountID: p.ProjectId,
				AccountName: p.ProjectId, Region: "global",
			})
		}
	}

	if len(contexts) == 0 && len(projects) > 0 {
		warnings = append(warnings, "GCP: projects found but none selected (press 'c' to configure)")
	}
	return contexts, warnings
}
