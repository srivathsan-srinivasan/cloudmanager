package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"

	"gopkg.in/ini.v1"
)

var defaultAWSRegions = []string{
	"us-east-1", "us-east-2", "us-west-1", "us-west-2",
	"ca-central-1", "eu-central-1", "eu-west-1", "eu-west-2",
	"eu-west-3", "eu-north-1", "ap-northeast-1", "ap-northeast-2",
	"ap-northeast-3", "ap-southeast-1", "ap-southeast-2", "ap-south-1",
	"sa-east-1",
}

func loadAWSContexts() []contextItem {
	var contexts []contextItem
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return contexts
	}

	configPath := filepath.Join(homeDir, ".aws", "config")
	cfg, err := ini.Load(configPath)
	if err != nil {
		return contexts
	}

	for _, section := range cfg.Sections() {
		name := section.Name()
		if name == "DEFAULT" {
			continue
		}

		profileName := name
		if len(name) > 8 && name[:8] == "profile " {
			profileName = name[8:]
		}

		regionKey, err := section.GetKey("region")
		if err == nil && regionKey.String() != "" {
			// If a region is explicitly set for the profile, use only that region
			contexts = append(contexts, contextItem{
				provider:    "AWS",
				accountID:   "N/A",
				accountName: profileName,
				region:      regionKey.String(),
			})
		} else {
			// If no region is set, fall back to checking all default regions
			for _, region := range defaultAWSRegions {
				contexts = append(contexts, contextItem{
					provider:    "AWS",
					accountID:   "N/A",
					accountName: profileName,
					region:      region,
				})
			}
		}
	}
	return contexts
}

func loadGCPContexts() []contextItem {
	var contexts []contextItem

	cmd := exec.Command("gcloud", "projects", "list", "--format=json(projectId)")
	output, err := cmd.Output()
	if err != nil {
		return contexts
	}

	var projects []struct {
		ProjectId string `json:"projectId"`
	}

	if err := json.Unmarshal(output, &projects); err != nil {
		return contexts
	}

	cfg := loadAppConfig()

	for _, p := range projects {
		if isGCPProjectSelected(p.ProjectId, cfg) {
			contexts = append(contexts, contextItem{
				provider:    "GCP",
				accountID:   p.ProjectId,
				accountName: p.ProjectId,
				region:      "global",
			})
		}
	}
	return contexts
}

func loadAzureContexts() []contextItem {
	var contexts []contextItem

	cmd := exec.Command("az", "account", "list", "--output", "json")
	output, err := cmd.Output()
	if err != nil {
		return contexts
	}

	var accounts []struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	}

	if err := json.Unmarshal(output, &accounts); err != nil {
		return contexts
	}

	for _, acc := range accounts {
		contexts = append(contexts, contextItem{
			provider:    "Azure",
			accountID:   acc.Id,
			accountName: acc.Name,
			region:      "global",
		})
	}
	return contexts
}
