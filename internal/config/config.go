package config

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cloudmanager/internal/core"
	"github.com/spf13/viper"
)

// ThemeConfig defines color overrides for the TUI.
type ThemeConfig struct {
	Subtle    string `mapstructure:"subtle" json:"subtle"`
	Highlight string `mapstructure:"highlight" json:"highlight"`
	Special   string `mapstructure:"special" json:"special"`
	Alert     string `mapstructure:"alert" json:"alert"`
}

// ManagedCloudContext stores app-owned cloud context metadata independent of CLI naming quirks.
type ManagedCloudContext struct {
	Provider          string   `mapstructure:"provider" json:"provider"`
	AccountID         string   `mapstructure:"account_id" json:"account_id"`
	AccountName       string   `mapstructure:"account_name" json:"account_name"`
	CredentialProfile string   `mapstructure:"credential_profile" json:"credential_profile"`
	Regions           []string `mapstructure:"regions" json:"regions"`
}

// AppConfig holds all persistent configuration.
type AppConfig struct {
	Backend            string                `mapstructure:"backend" json:"backend"`
	CloudContexts      []ManagedCloudContext `mapstructure:"cloud_contexts" json:"cloud_contexts"`
	GCPConfigured      bool                  `mapstructure:"gcp_configured" json:"gcp_configured"`
	GCPProjects        []string              `mapstructure:"gcp_projects" json:"gcp_projects"`
	VMColumns          []string              `mapstructure:"vm_columns" json:"vm_columns"`
	DiskColumns        []string              `mapstructure:"disk_columns" json:"disk_columns"`
	SnapshotColumns    []string              `mapstructure:"snapshot_columns" json:"snapshot_columns"`
	FirewallColumns    []string              `mapstructure:"firewall_columns" json:"firewall_columns"`
	ClusterColumns     []string              `mapstructure:"cluster_columns" json:"cluster_columns"`
	DatabaseColumns    []string              `mapstructure:"database_columns" json:"database_columns"`
	CacheTTL           int                   `mapstructure:"cache_ttl_minutes" json:"cache_ttl_minutes"`
	Theme              ThemeConfig           `mapstructure:"theme" json:"theme"`
	Keybindings        map[string]string     `mapstructure:"keybindings" json:"keybindings"`
	BillingEnabled     bool                  `mapstructure:"billing_enabled" json:"billing_enabled"`
	BillingCacheTTL    int                   `mapstructure:"billing_cache_ttl_minutes" json:"billing_cache_ttl_minutes"`
	GCPBillingDataset  string                `mapstructure:"gcp_billing_dataset" json:"gcp_billing_dataset"`
	GeminiModel        string                `mapstructure:"gemini_model" json:"gemini_model"`
	MetricsEnabled     bool                  `mapstructure:"metrics_enabled" json:"metrics_enabled"`
	MetricsPeriodHours int                   `mapstructure:"metrics_period_hours" json:"metrics_period_hours"`
	MetricsCacheTTL    int                   `mapstructure:"metrics_cache_ttl_minutes" json:"metrics_cache_ttl_minutes"`
}

// DefaultVMColumns is the default set of visible VM table columns.
var DefaultVMColumns = []string{
	"Name", "Instance ID", "Type", "State", "Private IP", "Public IP",
	"Network", "Subnet", "Labels",
}

// GetConfigPath returns the full path to the config file.
func GetConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cloudmanager.json")
}

// Load reads the config from disk, applying defaults for missing values.
func Load() AppConfig {
	home, _ := os.UserHomeDir()
	viper.AddConfigPath(home)
	viper.SetConfigName(".cloudmanager")
	viper.SetConfigType("json")

	viper.SetDefault("backend", "cli")
	viper.SetDefault("cloud_contexts", []ManagedCloudContext{})
	viper.SetDefault("gcp_configured", false)
	viper.SetDefault("gcp_projects", []string{})
	viper.SetDefault("vm_columns", DefaultVMColumns)
	viper.SetDefault("disk_columns", []string{"Name", "ID", "State", "Size (GB)", "Type", "Attached To", "Zone", "Encrypted"})
	viper.SetDefault("snapshot_columns", []string{"Name", "ID", "State", "Size (GB)", "Source Disk", "Created At"})
	viper.SetDefault("firewall_columns", core.DefaultSecurityGroupColumns)
	viper.SetDefault("cluster_columns", core.DefaultClusterColumns)
	viper.SetDefault("database_columns", core.DefaultDatabaseColumns)
	viper.SetDefault("cache_ttl_minutes", 5)
	viper.SetDefault("billing_enabled", true)
	viper.SetDefault("billing_cache_ttl_minutes", 30)
	viper.SetDefault("gcp_billing_dataset", "")
	viper.SetDefault("gemini_model", "gemini-1.5-flash")
	viper.SetDefault("metrics_enabled", false)
	viper.SetDefault("metrics_period_hours", 24)
	viper.SetDefault("metrics_cache_ttl_minutes", 15)
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

	if len(cfg.VMColumns) == 0 {
		cfg.VMColumns = DefaultVMColumns
	}
	cfg.VMColumns = SanitizeVMColumns(cfg.VMColumns)
	if cfg.CloudContexts == nil {
		cfg.CloudContexts = []ManagedCloudContext{}
	}
	if len(cfg.DiskColumns) == 0 {
		cfg.DiskColumns = []string{"Name", "ID", "State", "Size (GB)", "Type", "Attached To", "Zone", "Encrypted"}
	}
	if len(cfg.SnapshotColumns) == 0 {
		cfg.SnapshotColumns = []string{"Name", "ID", "State", "Size (GB)", "Source Disk", "Created At"}
	}
	if len(cfg.FirewallColumns) == 0 {
		cfg.FirewallColumns = core.DefaultSecurityGroupColumns
	}
	if len(cfg.ClusterColumns) == 0 {
		cfg.ClusterColumns = core.DefaultClusterColumns
	}
	if len(cfg.DatabaseColumns) == 0 {
		cfg.DatabaseColumns = core.DefaultDatabaseColumns
	}
	return cfg
}

// Save writes the config to disk.
func Save(cfg AppConfig) error {
	cfg.VMColumns = SanitizeVMColumns(cfg.VMColumns)
	viper.Set("backend", cfg.Backend)
	viper.Set("cloud_contexts", cfg.CloudContexts)
	viper.Set("gcp_configured", cfg.GCPConfigured)
	viper.Set("gcp_projects", cfg.GCPProjects)
	viper.Set("vm_columns", cfg.VMColumns)
	viper.Set("disk_columns", cfg.DiskColumns)
	viper.Set("snapshot_columns", cfg.SnapshotColumns)
	viper.Set("firewall_columns", cfg.FirewallColumns)
	viper.Set("cluster_columns", cfg.ClusterColumns)
	viper.Set("database_columns", cfg.DatabaseColumns)
	viper.Set("cache_ttl_minutes", cfg.CacheTTL)
	viper.Set("billing_enabled", cfg.BillingEnabled)
	viper.Set("billing_cache_ttl_minutes", cfg.BillingCacheTTL)
	viper.Set("gcp_billing_dataset", cfg.GCPBillingDataset)
	viper.Set("gemini_model", cfg.GeminiModel)
	viper.Set("metrics_enabled", cfg.MetricsEnabled)
	viper.Set("metrics_period_hours", cfg.MetricsPeriodHours)
	viper.Set("metrics_cache_ttl_minutes", cfg.MetricsCacheTTL)
	viper.Set("theme", cfg.Theme)
	viper.Set("keybindings", cfg.Keybindings)
	return viper.WriteConfigAs(GetConfigPath())
}

func SanitizeVMColumns(columns []string) []string {
	if len(columns) == 0 {
		sanitized := make([]string, len(DefaultVMColumns))
		copy(sanitized, DefaultVMColumns)
		return sanitized
	}

	seen := make(map[string]bool, len(columns))
	sanitized := make([]string, 0, len(columns))
	for _, col := range columns {
		col = strings.TrimSpace(col)
		if col == "" {
			continue
		}
		if seen[col] {
			continue
		}
		seen[col] = true
		sanitized = append(sanitized, col)
	}

	if len(sanitized) == 0 {
		sanitized = make([]string, len(DefaultVMColumns))
		copy(sanitized, DefaultVMColumns)
	}

	return sanitized
}

// IsGCPProjectSelected checks if a given project ID is in the config's selected list.
func IsGCPProjectSelected(projectId string, cfg AppConfig) bool {
	if !cfg.GCPConfigured {
		return true
	}
	for _, p := range cfg.GCPProjects {
		if p == projectId {
			return true
		}
	}
	return false
}

func ManagedContexts(cfg AppConfig) []core.CloudContext {
	var contexts []core.CloudContext
	for _, managed := range cfg.CloudContexts {
		provider := normalizeProvider(managed.Provider)
		if provider == "" {
			continue
		}

		regions := normalizeRegions(provider, managed.Regions)
		for _, region := range regions {
			contexts = append(contexts, core.CloudContext{
				Provider:          provider,
				AccountID:         strings.TrimSpace(managed.AccountID),
				AccountName:       strings.TrimSpace(managed.AccountName),
				Region:            region,
				CredentialProfile: strings.TrimSpace(managed.CredentialProfile),
			})
		}
	}

	sort.Slice(contexts, func(i, j int) bool {
		if contexts[i].Provider != contexts[j].Provider {
			return contexts[i].Provider < contexts[j].Provider
		}
		if contexts[i].AccountID != contexts[j].AccountID {
			return contexts[i].AccountID < contexts[j].AccountID
		}
		if contexts[i].AccountName != contexts[j].AccountName {
			return contexts[i].AccountName < contexts[j].AccountName
		}
		return contexts[i].Region < contexts[j].Region
	})

	return contexts
}

func HasManagedContextsForProvider(cfg AppConfig, provider string) bool {
	provider = normalizeProvider(provider)
	if provider == "" {
		return false
	}
	for _, managed := range cfg.CloudContexts {
		if normalizeProvider(managed.Provider) == provider {
			return true
		}
	}
	return false
}

func IsManagedAccountSelected(cfg AppConfig, provider, accountID string) bool {
	provider = normalizeProvider(provider)
	accountID = strings.TrimSpace(accountID)
	for _, managed := range cfg.CloudContexts {
		if normalizeProvider(managed.Provider) == provider && strings.TrimSpace(managed.AccountID) == accountID {
			return true
		}
	}
	return false
}

func MergeDiscoveredContexts(cfg AppConfig, discovered []core.CloudContext) (AppConfig, bool) {
	type groupKey struct {
		provider string
		id       string
		name     string
		auth     string
	}

	managedProviders := make(map[string]bool)
	for _, managed := range cfg.CloudContexts {
		provider := normalizeProvider(managed.Provider)
		if provider != "" {
			managedProviders[provider] = true
		}
	}

	groups := make(map[groupKey]map[string]bool)
	for _, ctx := range discovered {
		provider := normalizeProvider(ctx.Provider)
		if provider == "" || managedProviders[provider] {
			continue
		}
		key := groupKey{
			provider: provider,
			id:       strings.TrimSpace(ctx.AccountID),
			name:     strings.TrimSpace(ctx.AccountName),
			auth:     strings.TrimSpace(ctx.CredentialProfile),
		}
		if _, ok := groups[key]; !ok {
			groups[key] = make(map[string]bool)
		}
		region := strings.TrimSpace(ctx.Region)
		if region == "" {
			region = defaultRegionForProvider(provider)
		}
		groups[key][region] = true
	}

	if len(groups) == 0 {
		return cfg, false
	}

	keys := make([]groupKey, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].provider != keys[j].provider {
			return keys[i].provider < keys[j].provider
		}
		if keys[i].id != keys[j].id {
			return keys[i].id < keys[j].id
		}
		if keys[i].name != keys[j].name {
			return keys[i].name < keys[j].name
		}
		return keys[i].auth < keys[j].auth
	})

	for _, key := range keys {
		regions := make([]string, 0, len(groups[key]))
		for region := range groups[key] {
			regions = append(regions, region)
		}
		sort.Strings(regions)
		cfg.CloudContexts = append(cfg.CloudContexts, ManagedCloudContext{
			Provider:          key.provider,
			AccountID:         key.id,
			AccountName:       key.name,
			CredentialProfile: key.auth,
			Regions:           regions,
		})
	}

	return cfg, true
}

func ReplaceManagedContextsForProvider(cfg AppConfig, provider string, replacements []ManagedCloudContext) AppConfig {
	provider = normalizeProvider(provider)
	var kept []ManagedCloudContext
	for _, managed := range cfg.CloudContexts {
		if normalizeProvider(managed.Provider) != provider {
			kept = append(kept, managed)
		}
	}
	for _, replacement := range replacements {
		replacement.Provider = normalizeProvider(replacement.Provider)
		replacement.Regions = normalizeRegions(replacement.Provider, replacement.Regions)
		kept = append(kept, replacement)
	}
	cfg.CloudContexts = kept
	return cfg
}

func normalizeProvider(provider string) string {
	provider = strings.TrimSpace(provider)
	switch strings.ToUpper(provider) {
	case "AWS":
		return "AWS"
	case "GCP":
		return "GCP"
	case "AZURE":
		return "Azure"
	case "DIGITALOCEAN", "DIGITAL_OCEAN", "DO":
		return "DigitalOcean"
	default:
		return provider
	}
}

func normalizeRegions(provider string, regions []string) []string {
	seen := make(map[string]bool)
	var normalized []string
	for _, region := range regions {
		region = strings.TrimSpace(region)
		if region == "" {
			continue
		}
		if !seen[region] {
			seen[region] = true
			normalized = append(normalized, region)
		}
	}
	if len(normalized) == 0 {
		normalized = append(normalized, defaultRegionForProvider(provider))
	}
	sort.Strings(normalized)
	return normalized
}

func defaultRegionForProvider(provider string) string {
	switch normalizeProvider(provider) {
	case "AWS":
		return "us-east-1"
	case "GCP", "Azure", "DigitalOcean":
		return "global"
	default:
		return "global"
	}
}
