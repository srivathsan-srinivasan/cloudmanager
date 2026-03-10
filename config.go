package main

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type ThemeConfig struct {
	Subtle    string `mapstructure:"subtle" json:"subtle"`
	Highlight string `mapstructure:"highlight" json:"highlight"`
	Special   string `mapstructure:"special" json:"special"`
	Alert     string `mapstructure:"alert" json:"alert"`
}

type AppConfig struct {
	GCPConfigured bool              `mapstructure:"gcp_configured" json:"gcp_configured"`
	GCPProjects   []string          `mapstructure:"gcp_projects" json:"gcp_projects"`
	VMColumns     []string          `mapstructure:"vm_columns" json:"vm_columns"`
	CacheTTL      int               `mapstructure:"cache_ttl_minutes" json:"cache_ttl_minutes"`
	Theme         ThemeConfig       `mapstructure:"theme" json:"theme"`
	Keybindings   map[string]string `mapstructure:"keybindings" json:"keybindings"`
}

var defaultVMColumns = []string{"Name", "Instance ID", "Type", "State", "Private IP", "Public IP", "Network", "Subnet", "Labels"}

func getConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cloudmanager.json")
}

func loadAppConfig() AppConfig {
	home, _ := os.UserHomeDir()
	viper.AddConfigPath(home)
	viper.SetConfigName(".cloudmanager")
	viper.SetConfigType("json")

	// Set Defaults
	viper.SetDefault("gcp_configured", false)
	viper.SetDefault("gcp_projects", []string{})
	viper.SetDefault("vm_columns", defaultVMColumns)
	viper.SetDefault("cache_ttl_minutes", 5)

	// Lipgloss adaptive colors are supported via theme overrides if needed,
	// but keeping the standard hex defaults here.
	viper.SetDefault("theme.subtle", "#D9DCCF")
	viper.SetDefault("theme.highlight", "#874BFD")
	viper.SetDefault("theme.special", "#43BF6D")
	viper.SetDefault("theme.alert", "#FF5F87")

	viper.SetDefault("keybindings", map[string]string{
		"refresh": "r",
		"search":  "/",
	})

	_ = viper.ReadInConfig()

	var cfg AppConfig
	_ = viper.Unmarshal(&cfg)

	// Ensure defaults if empty array was unmarshalled for some reason
	if len(cfg.VMColumns) == 0 {
		cfg.VMColumns = defaultVMColumns
	}

	return cfg
}

func saveAppConfig(cfg AppConfig) error {
	viper.Set("gcp_configured", cfg.GCPConfigured)
	viper.Set("gcp_projects", cfg.GCPProjects)
	viper.Set("vm_columns", cfg.VMColumns)
	viper.Set("cache_ttl_minutes", cfg.CacheTTL)
	viper.Set("theme", cfg.Theme)
	viper.Set("keybindings", cfg.Keybindings)

	return viper.WriteConfigAs(getConfigPath())
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
