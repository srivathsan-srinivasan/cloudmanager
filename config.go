package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type AppConfig struct {
	GCPConfigured bool     `json:"gcp_configured"`
	GCPProjects   []string `json:"gcp_projects"`
	VMColumns     []string `json:"vm_columns"`
}

var defaultVMColumns = []string{"Name", "Instance ID", "Type", "State", "Private IP", "Public IP", "Network", "Subnet", "Labels"}

func getConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cloudmanager.json")
}

func loadAppConfig() AppConfig {
	var cfg AppConfig
	data, err := os.ReadFile(getConfigPath())
	if err == nil {
		json.Unmarshal(data, &cfg)
	}
	if len(cfg.VMColumns) == 0 {
		cfg.VMColumns = defaultVMColumns
	}
	return cfg
}

func saveAppConfig(cfg AppConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(getConfigPath(), data, 0644)
}

func isGCPProjectSelected(projectId string, cfg AppConfig) bool {
	if !cfg.GCPConfigured {
		return true // Default to all if not configured
	}
	for _, p := range cfg.GCPProjects {
		if p == projectId {
			return true
		}
	}
	return false
}
