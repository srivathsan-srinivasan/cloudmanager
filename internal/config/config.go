package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
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
	ContextName           string   `mapstructure:"context_name" json:"context_name"`
	Provider              string   `mapstructure:"provider" json:"provider"`
	AccountID             string   `mapstructure:"account_id" json:"account_id"`
	AccountName           string   `mapstructure:"account_name" json:"account_name"`
	Tenant                string   `mapstructure:"tenant" json:"tenant"`
	AuthMode              string   `mapstructure:"auth_mode" json:"auth_mode"`
	CredentialPersistence string   `mapstructure:"credential_persistence" json:"credential_persistence"`
	CredentialProfile     string   `mapstructure:"credential_profile" json:"credential_profile"`
	Regions               []string `mapstructure:"regions" json:"regions"`
}

type ManualHost struct {
	Name          string   `mapstructure:"name" json:"name"`
	Provider      string   `mapstructure:"provider" json:"provider"`
	Host          string   `mapstructure:"host" json:"host"`
	Username      string   `mapstructure:"username" json:"username"`
	Connection    string   `mapstructure:"connection" json:"connection"`
	SSHConfigHost string   `mapstructure:"ssh_config_host" json:"ssh_config_host"`
	KeyPath       string   `mapstructure:"key_path" json:"key_path"`
	KeyRef        string   `mapstructure:"key_ref" json:"key_ref"`
	PasswordRef   string   `mapstructure:"password_ref" json:"password_ref"`
	Tags          []string `mapstructure:"tags" json:"tags"`
}

// ResourceTag stores CloudManager-only tags for resources without mutating
// provider metadata.
type ResourceTag struct {
	Provider     string   `mapstructure:"provider" json:"provider"`
	AccountID    string   `mapstructure:"account_id" json:"account_id"`
	AccountName  string   `mapstructure:"account_name" json:"account_name"`
	Region       string   `mapstructure:"region" json:"region"`
	ResourceKind string   `mapstructure:"resource_kind" json:"resource_kind"`
	ResourceID   string   `mapstructure:"resource_id" json:"resource_id"`
	ResourceName string   `mapstructure:"resource_name" json:"resource_name"`
	PrivateIP    string   `mapstructure:"private_ip" json:"private_ip"`
	PublicIP     string   `mapstructure:"public_ip" json:"public_ip"`
	Tags         []string `mapstructure:"tags" json:"tags"`
}

// ResourceTagTarget is the reusable identity CloudManager tags attach to.
// It is intentionally provider-neutral so new resource views can opt in
// without adding new config schema.
type ResourceTagTarget struct {
	Kind      string
	ID        string
	Name      string
	PrivateIP string
	PublicIP  string
}

const (
	AuthModeNativeCLI = "native-cli"
	AuthModeJIT       = "jit-session"
	AuthModeAWSume    = "awsume"
	AuthModeVault     = "vault"
	AuthModeManual    = "manual"

	CredentialPersistenceMemory    = "memory"
	CredentialPersistenceKeychain  = "keychain"
	CredentialPersistenceNativeCLI = "native-cli"
	CredentialPersistenceVault     = "vault"
	CredentialPersistenceNone      = "none"
)

// AppConfig holds all persistent configuration.
type AppConfig struct {
	Backend                  string                `mapstructure:"backend" json:"backend"`
	CurrentContext           string                `mapstructure:"current_context" json:"current_context"`
	CloudContexts            []ManagedCloudContext `mapstructure:"cloud_contexts" json:"cloud_contexts"`
	ManualHosts              []ManualHost          `mapstructure:"manual_hosts" json:"manual_hosts"`
	ResourceTags             []ResourceTag         `mapstructure:"resource_tags" json:"resource_tags"`
	TerraformStatePaths      []string              `mapstructure:"terraform_state_paths" json:"terraform_state_paths"`
	GCPConfigured            bool                  `mapstructure:"gcp_configured" json:"gcp_configured"`
	GCPProjects              []string              `mapstructure:"gcp_projects" json:"gcp_projects"`
	VMColumns                []string              `mapstructure:"vm_columns" json:"vm_columns"`
	DiskColumns              []string              `mapstructure:"disk_columns" json:"disk_columns"`
	SnapshotColumns          []string              `mapstructure:"snapshot_columns" json:"snapshot_columns"`
	FirewallColumns          []string              `mapstructure:"firewall_columns" json:"firewall_columns"`
	ClusterColumns           []string              `mapstructure:"cluster_columns" json:"cluster_columns"`
	DatabaseColumns          []string              `mapstructure:"database_columns" json:"database_columns"`
	StorageColumns           []string              `mapstructure:"storage_columns" json:"storage_columns"`
	CacheTTL                 int                   `mapstructure:"cache_ttl_minutes" json:"cache_ttl_minutes"`
	Theme                    ThemeConfig           `mapstructure:"theme" json:"theme"`
	Keybindings              map[string]string     `mapstructure:"keybindings" json:"keybindings"`
	BillingEnabled           bool                  `mapstructure:"billing_enabled" json:"billing_enabled"`
	BillingCacheTTL          int                   `mapstructure:"billing_cache_ttl_minutes" json:"billing_cache_ttl_minutes"`
	GCPBillingDataset        string                `mapstructure:"gcp_billing_dataset" json:"gcp_billing_dataset"`
	GeminiModel              string                `mapstructure:"gemini_model" json:"gemini_model"`
	MetricsEnabled           bool                  `mapstructure:"metrics_enabled" json:"metrics_enabled"`
	MetricsPeriodHours       int                   `mapstructure:"metrics_period_hours" json:"metrics_period_hours"`
	MetricsCacheTTL          int                   `mapstructure:"metrics_cache_ttl_minutes" json:"metrics_cache_ttl_minutes"`
	GlobalSearch             bool                  `mapstructure:"global_search_enabled" json:"global_search_enabled"`
	DiscoverOnStart          bool                  `mapstructure:"discover_contexts_on_start" json:"discover_contexts_on_start"`
	PrefetchOnStart          bool                  `mapstructure:"prefetch_on_start" json:"prefetch_on_start"`
	PrefetchResources        []string              `mapstructure:"prefetch_resources" json:"prefetch_resources"`
	PrefetchConcurrency      int                   `mapstructure:"prefetch_concurrency" json:"prefetch_concurrency"`
	VMIndexPersistence       bool                  `mapstructure:"vm_index_persistence_enabled" json:"vm_index_persistence_enabled"`
	VMIndexCacheTTL          int                   `mapstructure:"vm_index_cache_ttl_hours" json:"vm_index_cache_ttl_hours"`
	ResourceIndexPersistence bool                  `mapstructure:"resource_index_persistence_enabled" json:"resource_index_persistence_enabled"`
	ResourceIndexCacheTTL    int                   `mapstructure:"resource_index_cache_ttl_hours" json:"resource_index_cache_ttl_hours"`
	HideKubernetesNodes      bool                  `mapstructure:"hide_kubernetes_nodes" json:"hide_kubernetes_nodes"`
	DashboardWidgets         []string              `mapstructure:"dashboard_widgets" json:"dashboard_widgets"`
	DashboardTheme           string                `mapstructure:"dashboard_theme" json:"dashboard_theme"`
}

// DefaultVMColumns is the default set of visible VM table columns.
var DefaultVMColumns = []string{
	"Name", "Instance ID", "Type", "State", "Private IP", "Public IP",
	"Network", "Subnet", "Labels",
}

var DefaultDashboardWidgets = []string{
	"contexts",
	"indexed_vms",
	"running_vms",
	"stopped_vms",
	"public_ips",
	"disks",
	"snapshots",
	"networks",
	"subnets",
	"firewalls",
	"backend",
	"databases",
	"storage",
	"kubernetes",
	"terraform",
	"manual_hosts",
	"db_contexts",
}

var legacyDefaultDashboardWidgets = []string{
	"contexts",
	"indexed_vms",
	"running_vms",
	"stopped_vms",
	"public_ips",
	"backend",
	"databases",
	"kubernetes",
	"db_contexts",
}

// GetConfigPath returns the full path to the config file.
func GetConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cloudmanager.json")
}

func GetVMIndexPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cloudmanager-vm-index.json")
}

func GetResourceIndexPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cloudmanager-resource-index.json")
}

func BackupConfig() (string, error) {
	path := GetConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	backupPath := filepath.Join(filepath.Dir(path), fmt.Sprintf(".cloudmanager.backup-%s.json", time.Now().Format("20060102-150405")))
	if err := os.WriteFile(backupPath, data, 0600); err != nil {
		return "", err
	}
	return backupPath, nil
}

// Load reads the config from disk, applying defaults for missing values.
func Load() AppConfig {
	home, _ := os.UserHomeDir()
	viper.AddConfigPath(home)
	viper.SetConfigName(".cloudmanager")
	viper.SetConfigType("json")

	viper.SetDefault("backend", "cli")
	viper.SetDefault("current_context", "")
	viper.SetDefault("cloud_contexts", []ManagedCloudContext{})
	viper.SetDefault("manual_hosts", []ManualHost{})
	viper.SetDefault("resource_tags", []ResourceTag{})
	viper.SetDefault("terraform_state_paths", []string{})
	viper.SetDefault("gcp_configured", false)
	viper.SetDefault("gcp_projects", []string{})
	viper.SetDefault("vm_columns", DefaultVMColumns)
	viper.SetDefault("disk_columns", []string{"Name", "ID", "State", "Size (GB)", "Type", "Attached To", "Zone", "Encrypted"})
	viper.SetDefault("snapshot_columns", []string{"Name", "ID", "State", "Size (GB)", "Source Disk", "Created At"})
	viper.SetDefault("firewall_columns", core.DefaultSecurityGroupColumns)
	viper.SetDefault("cluster_columns", core.DefaultClusterColumns)
	viper.SetDefault("database_columns", core.DefaultDatabaseColumns)
	viper.SetDefault("storage_columns", core.DefaultStorageColumns)
	viper.SetDefault("cache_ttl_minutes", 5)
	viper.SetDefault("billing_enabled", true)
	viper.SetDefault("billing_cache_ttl_minutes", 30)
	viper.SetDefault("gcp_billing_dataset", "")
	viper.SetDefault("gemini_model", "gemini-1.5-flash")
	viper.SetDefault("metrics_enabled", false)
	viper.SetDefault("metrics_period_hours", 24)
	viper.SetDefault("metrics_cache_ttl_minutes", 15)
	viper.SetDefault("global_search_enabled", true)
	viper.SetDefault("discover_contexts_on_start", false)
	viper.SetDefault("prefetch_on_start", false)
	viper.SetDefault("prefetch_resources", []string{"vms"})
	viper.SetDefault("prefetch_concurrency", 4)
	viper.SetDefault("vm_index_persistence_enabled", true)
	viper.SetDefault("vm_index_cache_ttl_hours", 24)
	viper.SetDefault("resource_index_persistence_enabled", true)
	viper.SetDefault("resource_index_cache_ttl_hours", 24)
	viper.SetDefault("hide_kubernetes_nodes", true)
	viper.SetDefault("dashboard_widgets", DefaultDashboardWidgets)
	viper.SetDefault("dashboard_theme", "btop")
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
	cfg.CurrentContext = strings.TrimSpace(cfg.CurrentContext)
	cfg.VMColumns = SanitizeVMColumns(cfg.VMColumns)
	if cfg.CloudContexts == nil {
		cfg.CloudContexts = []ManagedCloudContext{}
	}
	if cfg.ManualHosts == nil {
		cfg.ManualHosts = []ManualHost{}
	}
	for i := range cfg.ManualHosts {
		cfg.ManualHosts[i] = SanitizeManualHost(cfg.ManualHosts[i])
	}
	if cfg.ResourceTags == nil {
		cfg.ResourceTags = []ResourceTag{}
	}
	for i := range cfg.ResourceTags {
		cfg.ResourceTags[i] = SanitizeResourceTag(cfg.ResourceTags[i])
	}
	cfg.TerraformStatePaths = normalizePathList(cfg.TerraformStatePaths)
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
	if len(cfg.StorageColumns) == 0 {
		cfg.StorageColumns = core.DefaultStorageColumns
	}
	if len(cfg.PrefetchResources) == 0 {
		cfg.PrefetchResources = []string{"vms"}
	}
	cfg.PrefetchResources = normalizeResourceList(cfg.PrefetchResources)
	cfg.DashboardWidgets = SanitizeDashboardWidgets(cfg.DashboardWidgets)
	cfg.DashboardTheme = SanitizeDashboardTheme(cfg.DashboardTheme)
	if cfg.PrefetchConcurrency <= 0 {
		cfg.PrefetchConcurrency = 4
	}
	if cfg.VMIndexCacheTTL <= 0 {
		cfg.VMIndexCacheTTL = 24
	}
	if !viper.InConfig("resource_index_persistence_enabled") {
		cfg.ResourceIndexPersistence = true
	}
	if cfg.ResourceIndexCacheTTL <= 0 {
		cfg.ResourceIndexCacheTTL = 24
	}
	return cfg
}

// Save writes the config to disk.
func Save(cfg AppConfig) error {
	for i := range cfg.CloudContexts {
		cfg.CloudContexts[i] = SanitizeManagedCloudContext(cfg.CloudContexts[i])
	}
	cfg.VMColumns = SanitizeVMColumns(cfg.VMColumns)
	if len(cfg.PrefetchResources) == 0 {
		cfg.PrefetchResources = []string{"vms"}
	}
	cfg.PrefetchResources = normalizeResourceList(cfg.PrefetchResources)
	cfg.DashboardWidgets = SanitizeDashboardWidgets(cfg.DashboardWidgets)
	cfg.DashboardTheme = SanitizeDashboardTheme(cfg.DashboardTheme)
	if cfg.PrefetchConcurrency <= 0 {
		cfg.PrefetchConcurrency = 4
	}
	if cfg.VMIndexCacheTTL <= 0 {
		cfg.VMIndexCacheTTL = 24
	}
	if cfg.ResourceIndexCacheTTL <= 0 {
		cfg.ResourceIndexCacheTTL = 24
	}
	viper.Set("backend", cfg.Backend)
	viper.Set("current_context", strings.TrimSpace(cfg.CurrentContext))
	viper.Set("cloud_contexts", cfg.CloudContexts)
	for i := range cfg.ManualHosts {
		cfg.ManualHosts[i] = SanitizeManualHost(cfg.ManualHosts[i])
	}
	viper.Set("manual_hosts", cfg.ManualHosts)
	for i := range cfg.ResourceTags {
		cfg.ResourceTags[i] = SanitizeResourceTag(cfg.ResourceTags[i])
	}
	viper.Set("resource_tags", cfg.ResourceTags)
	cfg.TerraformStatePaths = normalizePathList(cfg.TerraformStatePaths)
	viper.Set("terraform_state_paths", cfg.TerraformStatePaths)
	viper.Set("gcp_configured", cfg.GCPConfigured)
	viper.Set("gcp_projects", cfg.GCPProjects)
	viper.Set("vm_columns", cfg.VMColumns)
	viper.Set("disk_columns", cfg.DiskColumns)
	viper.Set("snapshot_columns", cfg.SnapshotColumns)
	viper.Set("firewall_columns", cfg.FirewallColumns)
	viper.Set("cluster_columns", cfg.ClusterColumns)
	viper.Set("database_columns", cfg.DatabaseColumns)
	viper.Set("storage_columns", cfg.StorageColumns)
	viper.Set("cache_ttl_minutes", cfg.CacheTTL)
	viper.Set("billing_enabled", cfg.BillingEnabled)
	viper.Set("billing_cache_ttl_minutes", cfg.BillingCacheTTL)
	viper.Set("gcp_billing_dataset", cfg.GCPBillingDataset)
	viper.Set("gemini_model", cfg.GeminiModel)
	viper.Set("metrics_enabled", cfg.MetricsEnabled)
	viper.Set("metrics_period_hours", cfg.MetricsPeriodHours)
	viper.Set("metrics_cache_ttl_minutes", cfg.MetricsCacheTTL)
	viper.Set("global_search_enabled", cfg.GlobalSearch)
	viper.Set("discover_contexts_on_start", cfg.DiscoverOnStart)
	viper.Set("prefetch_on_start", cfg.PrefetchOnStart)
	viper.Set("prefetch_resources", cfg.PrefetchResources)
	viper.Set("prefetch_concurrency", cfg.PrefetchConcurrency)
	viper.Set("vm_index_persistence_enabled", cfg.VMIndexPersistence)
	viper.Set("vm_index_cache_ttl_hours", cfg.VMIndexCacheTTL)
	viper.Set("resource_index_persistence_enabled", cfg.ResourceIndexPersistence)
	viper.Set("resource_index_cache_ttl_hours", cfg.ResourceIndexCacheTTL)
	viper.Set("hide_kubernetes_nodes", cfg.HideKubernetesNodes)
	viper.Set("dashboard_widgets", cfg.DashboardWidgets)
	viper.Set("dashboard_theme", cfg.DashboardTheme)
	viper.Set("theme", cfg.Theme)
	viper.Set("keybindings", cfg.Keybindings)
	return viper.WriteConfigAs(GetConfigPath())
}

func SanitizeManagedCloudContext(ctx ManagedCloudContext) ManagedCloudContext {
	ctx.ContextName = sanitizeContextName(ctx.ContextName)
	ctx.Provider = normalizeProvider(ctx.Provider)
	ctx.Tenant = strings.TrimSpace(ctx.Tenant)
	ctx.AuthMode = SanitizeAuthMode(ctx.AuthMode)
	ctx.CredentialPersistence = SanitizeCredentialPersistence(ctx.CredentialPersistence, ctx.AuthMode)
	ctx.Regions = normalizeRegions(ctx.Provider, ctx.Regions)
	if ctx.ContextName == "" {
		ctx.ContextName = DefaultContextName(ctx)
	}
	return ctx
}

func DefaultContextName(ctx ManagedCloudContext) string {
	for _, value := range []string{ctx.CredentialProfile, ctx.AccountName, ctx.AccountID} {
		if name := sanitizeContextName(value); name != "" {
			return name
		}
	}
	return ""
}

func SanitizeManualHost(host ManualHost) ManualHost {
	host.Name = strings.TrimSpace(host.Name)
	host.Provider = strings.TrimSpace(host.Provider)
	if host.Provider == "" {
		host.Provider = "Manual"
	}
	host.Host = strings.TrimSpace(host.Host)
	host.Username = strings.TrimSpace(host.Username)
	host.Connection = strings.ToLower(strings.TrimSpace(host.Connection))
	if host.Connection == "" {
		host.Connection = "ssh"
	}
	host.SSHConfigHost = strings.TrimSpace(host.SSHConfigHost)
	host.KeyPath = strings.TrimSpace(host.KeyPath)
	host.KeyRef = strings.TrimSpace(host.KeyRef)
	host.PasswordRef = strings.TrimSpace(host.PasswordRef)
	host.Tags = normalizeStringList(host.Tags)
	return host
}

func SanitizeResourceTag(tag ResourceTag) ResourceTag {
	tag.Provider = normalizeProvider(tag.Provider)
	tag.AccountID = strings.TrimSpace(tag.AccountID)
	tag.AccountName = strings.TrimSpace(tag.AccountName)
	tag.Region = strings.TrimSpace(tag.Region)
	tag.ResourceKind = normalizeResourceKind(tag.ResourceKind)
	tag.ResourceID = strings.TrimSpace(tag.ResourceID)
	tag.ResourceName = strings.TrimSpace(tag.ResourceName)
	tag.PrivateIP = strings.TrimSpace(tag.PrivateIP)
	tag.PublicIP = strings.TrimSpace(tag.PublicIP)
	tag.Tags = normalizeDisplayTagList(tag.Tags)
	return tag
}

func ApplyResourceTagsToLabels(cfg AppConfig, ctx core.CloudContext, target ResourceTagTarget, labels string) string {
	tags := CloudManagerTagsForResource(cfg, ctx, target)
	if len(tags) == 0 {
		return labels
	}
	return appendCloudManagerTags(labels, tags)
}

func CloudManagerTagsForResource(cfg AppConfig, ctx core.CloudContext, target ResourceTagTarget) []string {
	target = sanitizeResourceTagTarget(target)
	var tags []string
	for _, raw := range cfg.ResourceTags {
		tag := SanitizeResourceTag(raw)
		if !resourceTagMatchesContext(tag, ctx) || !resourceTagMatchesTarget(tag, target) {
			continue
		}
		tags = append(tags, tag.Tags...)
	}
	return normalizeDisplayTagList(tags)
}

func UpsertResourceTags(cfg AppConfig, ctx core.CloudContext, target ResourceTagTarget, tags []string) AppConfig {
	tags = normalizeDisplayTagList(tags)
	target = sanitizeResourceTagTarget(target)
	if len(tags) == 0 || !target.hasIdentity() {
		return cfg
	}
	next := SanitizeResourceTag(ResourceTag{
		Provider:     ctx.Provider,
		AccountID:    ctx.AccountID,
		AccountName:  ctx.AccountName,
		Region:       ctx.Region,
		ResourceKind: target.Kind,
		ResourceID:   target.ID,
		ResourceName: target.Name,
		PrivateIP:    target.PrivateIP,
		PublicIP:     target.PublicIP,
		Tags:         tags,
	})
	for i, existing := range cfg.ResourceTags {
		existing = SanitizeResourceTag(existing)
		if sameResourceTagTarget(existing, next) {
			existing.Tags = normalizeDisplayTagList(append(existing.Tags, tags...))
			cfg.ResourceTags[i] = existing
			return cfg
		}
	}
	cfg.ResourceTags = append(cfg.ResourceTags, next)
	return cfg
}

func ApplyResourceTagsToVMs(cfg AppConfig, ctx core.CloudContext, vms []core.VM) []core.VM {
	out := make([]core.VM, len(vms))
	for i, vm := range vms {
		out[i] = ApplyResourceTagsToVM(cfg, ctx, vm)
	}
	return out
}

func ApplyResourceTagsToVM(cfg AppConfig, ctx core.CloudContext, vm core.VM) core.VM {
	vm.Labels = ApplyResourceTagsToLabels(cfg, ctx, vmResourceTagTarget(vm), vm.Labels)
	return vm
}

func UpsertVMResourceTags(cfg AppConfig, ctx core.CloudContext, vm core.VM, tags []string) AppConfig {
	return UpsertResourceTags(cfg, ctx, vmResourceTagTarget(vm), tags)
}

func CloudManagerTagsForVM(cfg AppConfig, ctx core.CloudContext, vm core.VM) []string {
	return CloudManagerTagsForResource(cfg, ctx, vmResourceTagTarget(vm))
}

func ApplyResourceTagsToDisks(cfg AppConfig, ctx core.CloudContext, disks []core.Disk) []core.Disk {
	out := make([]core.Disk, len(disks))
	for i, disk := range disks {
		disk.Labels = ApplyResourceTagsToLabels(cfg, ctx, resourceTagTargetFromResource(disk), disk.Labels)
		out[i] = disk
	}
	return out
}

func ApplyResourceTagsToSnapshots(cfg AppConfig, ctx core.CloudContext, snapshots []core.Snapshot) []core.Snapshot {
	out := make([]core.Snapshot, len(snapshots))
	for i, snap := range snapshots {
		snap.Labels = ApplyResourceTagsToLabels(cfg, ctx, resourceTagTargetFromResource(snap), snap.Labels)
		out[i] = snap
	}
	return out
}

func ApplyResourceTagsToClusters(cfg AppConfig, ctx core.CloudContext, clusters []core.Cluster) []core.Cluster {
	out := make([]core.Cluster, len(clusters))
	for i, cluster := range clusters {
		cluster.Labels = ApplyResourceTagsToLabels(cfg, ctx, resourceTagTargetFromResource(cluster), cluster.Labels)
		out[i] = cluster
	}
	return out
}

func ApplyResourceTagsToDatabases(cfg AppConfig, ctx core.CloudContext, databases []core.Database) []core.Database {
	out := make([]core.Database, len(databases))
	for i, db := range databases {
		db.Labels = ApplyResourceTagsToLabels(cfg, ctx, resourceTagTargetFromResource(db), db.Labels)
		out[i] = db
	}
	return out
}

func ApplyResourceTagsToNetworks(cfg AppConfig, ctx core.CloudContext, networks []core.Network) []core.Network {
	out := make([]core.Network, len(networks))
	for i, network := range networks {
		network.Labels = ApplyResourceTagsToLabels(cfg, ctx, resourceTagTargetFromResource(network), network.Labels)
		out[i] = network
	}
	return out
}

func ApplyResourceTagsToSubnets(cfg AppConfig, ctx core.CloudContext, subnets []core.Subnet) []core.Subnet {
	out := make([]core.Subnet, len(subnets))
	for i, subnet := range subnets {
		subnet.Labels = ApplyResourceTagsToLabels(cfg, ctx, resourceTagTargetFromResource(subnet), subnet.Labels)
		out[i] = subnet
	}
	return out
}

func ApplyResourceTagsToSecurityGroups(cfg AppConfig, ctx core.CloudContext, groups []core.SecurityGroup) []core.SecurityGroup {
	out := make([]core.SecurityGroup, len(groups))
	for i, group := range groups {
		group.Labels = ApplyResourceTagsToLabels(cfg, ctx, resourceTagTargetFromResource(group), group.Labels)
		out[i] = group
	}
	return out
}

func ApplyResourceTagsToStorageBuckets(cfg AppConfig, ctx core.CloudContext, buckets []core.StorageBucket) []core.StorageBucket {
	out := make([]core.StorageBucket, len(buckets))
	for i, bucket := range buckets {
		bucket.Labels = ApplyResourceTagsToLabels(cfg, ctx, resourceTagTargetFromResource(bucket), bucket.Labels)
		out[i] = bucket
	}
	return out
}

func sameResourceTagTarget(left, right ResourceTag) bool {
	return strings.EqualFold(left.Provider, right.Provider) &&
		strings.EqualFold(left.AccountID, right.AccountID) &&
		strings.EqualFold(left.AccountName, right.AccountName) &&
		strings.EqualFold(left.Region, right.Region) &&
		matchOptional(left.ResourceKind, right.ResourceKind) &&
		strings.EqualFold(left.ResourceID, right.ResourceID) &&
		strings.EqualFold(left.ResourceName, right.ResourceName) &&
		left.PrivateIP == right.PrivateIP &&
		left.PublicIP == right.PublicIP
}

func resourceTagMatchesContext(tag ResourceTag, ctx core.CloudContext) bool {
	return matchOptional(tag.Provider, ctx.Provider) &&
		matchOptional(tag.AccountID, ctx.AccountID) &&
		matchOptional(tag.AccountName, ctx.AccountName) &&
		matchOptional(tag.Region, ctx.Region)
}

func resourceTagMatchesTarget(tag ResourceTag, target ResourceTagTarget) bool {
	if !matchOptional(tag.ResourceKind, target.Kind) {
		return false
	}
	matched := false
	if tag.ResourceID != "" {
		matched = matched || strings.EqualFold(tag.ResourceID, target.ID)
	}
	if tag.ResourceName != "" {
		matched = matched || strings.EqualFold(tag.ResourceName, target.Name)
	}
	if tag.PrivateIP != "" {
		matched = matched || tag.PrivateIP == target.PrivateIP
	}
	if tag.PublicIP != "" {
		matched = matched || tag.PublicIP == target.PublicIP
	}
	return matched
}

func ResourceTagTargetFromResource(resource core.Resource) ResourceTagTarget {
	if resource == nil {
		return ResourceTagTarget{}
	}
	return sanitizeResourceTagTarget(ResourceTagTarget{
		Kind: resource.GetKind(),
		ID:   resource.GetID(),
		Name: resource.GetName(),
	})
}

func vmResourceTagTarget(vm core.VM) ResourceTagTarget {
	return sanitizeResourceTagTarget(ResourceTagTarget{
		Kind:      vm.GetKind(),
		ID:        vm.ID,
		Name:      vm.Name,
		PrivateIP: vm.PrivateIP,
		PublicIP:  vm.PublicIP,
	})
}

func resourceTagTargetFromResource(resource core.Resource) ResourceTagTarget {
	return ResourceTagTargetFromResource(resource)
}

func sanitizeResourceTagTarget(target ResourceTagTarget) ResourceTagTarget {
	target.Kind = normalizeResourceKind(target.Kind)
	target.ID = strings.TrimSpace(target.ID)
	target.Name = strings.TrimSpace(target.Name)
	target.PrivateIP = strings.TrimSpace(target.PrivateIP)
	target.PublicIP = strings.TrimSpace(target.PublicIP)
	return target
}

func (target ResourceTagTarget) hasIdentity() bool {
	return target.ID != "" || target.Name != "" || target.PrivateIP != "" || target.PublicIP != ""
}

func normalizeResourceKind(kind string) string {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return ""
	}
	key := strings.ToLower(strings.ReplaceAll(kind, "_", " "))
	key = strings.Join(strings.Fields(key), " ")
	switch key {
	case "vm", "vms", "virtual machine", "virtual machines", "instance", "instances":
		return "VM"
	case "disk", "disks", "volume", "volumes":
		return "Disk"
	case "snapshot", "snapshots":
		return "Snapshot"
	case "cluster", "clusters", "k8s", "kubernetes":
		return "Cluster"
	case "database", "databases", "db", "dbs":
		return "Database"
	case "network", "networks", "vpc", "vnet":
		return "Network"
	case "subnet", "subnets":
		return "Subnet"
	case "security group", "security groups", "sg", "firewall", "firewalls":
		return "Security Group"
	case "storage", "bucket", "buckets", "storage bucket", "storage buckets":
		return "Storage"
	default:
		return kind
	}
}

func matchOptional(want, got string) bool {
	want = strings.TrimSpace(want)
	if want == "" || want == "*" {
		return true
	}
	return strings.EqualFold(want, strings.TrimSpace(got))
}

func appendCloudManagerTags(labels string, tags []string) string {
	parts := splitDisplayList(labels)
	seen := make(map[string]bool, len(parts)+len(tags))
	for _, part := range parts {
		seen[strings.ToLower(strings.TrimSpace(part))] = true
	}
	for _, tag := range normalizeDisplayTagList(tags) {
		token := "cm:" + tag
		key := strings.ToLower(token)
		if seen[key] {
			continue
		}
		parts = append(parts, token)
		seen[key] = true
	}
	return strings.Join(parts, ",")
}

func SanitizeAuthMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case AuthModeJIT:
		return AuthModeJIT
	case AuthModeAWSume:
		return AuthModeAWSume
	case AuthModeVault:
		return AuthModeVault
	case AuthModeManual:
		return AuthModeManual
	case AuthModeNativeCLI, "":
		return AuthModeNativeCLI
	default:
		return AuthModeNativeCLI
	}
}

func SanitizeCredentialPersistence(persistence, authMode string) string {
	persistence = strings.ToLower(strings.TrimSpace(persistence))
	authMode = SanitizeAuthMode(authMode)
	switch persistence {
	case CredentialPersistenceMemory, CredentialPersistenceKeychain, CredentialPersistenceNativeCLI, CredentialPersistenceVault, CredentialPersistenceNone:
		return persistence
	}
	switch authMode {
	case AuthModeJIT, AuthModeAWSume:
		return CredentialPersistenceMemory
	case AuthModeVault:
		return CredentialPersistenceVault
	case AuthModeManual:
		return CredentialPersistenceNone
	default:
		return CredentialPersistenceNativeCLI
	}
}

func PrefetchesResource(cfg AppConfig, resource string) bool {
	resource = strings.ToLower(strings.TrimSpace(resource))
	for _, item := range cfg.PrefetchResources {
		item = strings.ToLower(strings.TrimSpace(item))
		if item == "all" || item == resource {
			return true
		}
	}
	return false
}

func normalizeResourceList(resources []string) []string {
	return normalizeStringList(resources)
}

func normalizePathList(values []string) []string {
	seen := make(map[string]bool, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		normalized = append(normalized, value)
	}
	return normalized
}

func sanitizeContextName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", "-")
	var b strings.Builder
	lastDash := false
	for _, r := range value {
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
	return strings.Trim(b.String(), "-")
}

func normalizeStringList(values []string) []string {
	seen := make(map[string]bool, len(values))
	normalized := make([]string, 0, len(values))
	for _, resource := range values {
		resource = strings.ToLower(strings.TrimSpace(resource))
		if resource == "" || seen[resource] {
			continue
		}
		seen[resource] = true
		normalized = append(normalized, resource)
	}
	return normalized
}

func normalizeDisplayTagList(values []string) []string {
	seen := make(map[string]bool, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		normalized = append(normalized, value)
	}
	return normalized
}

func splitDisplayList(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';'
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			out = append(out, field)
		}
	}
	return out
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

func SanitizeDashboardWidgets(widgets []string) []string {
	allowed := map[string]bool{
		"contexts":     true,
		"indexed_vms":  true,
		"running_vms":  true,
		"stopped_vms":  true,
		"public_ips":   true,
		"disks":        true,
		"snapshots":    true,
		"networks":     true,
		"subnets":      true,
		"firewalls":    true,
		"storage":      true,
		"backend":      true,
		"databases":    true,
		"kubernetes":   true,
		"terraform":    true,
		"manual_hosts": true,
		"db_contexts":  true,
		"vm_providers": true,
		"vm_index_age": true,
		"k8s_hidden":   true,
	}
	seen := make(map[string]bool, len(widgets))
	sanitized := make([]string, 0, len(widgets))
	for _, widget := range widgets {
		widget = strings.ToLower(strings.TrimSpace(widget))
		if widget == "" || !allowed[widget] || seen[widget] {
			continue
		}
		seen[widget] = true
		sanitized = append(sanitized, widget)
	}
	if len(sanitized) == 0 {
		sanitized = make([]string, len(DefaultDashboardWidgets))
		copy(sanitized, DefaultDashboardWidgets)
	}
	if containsAllWidgets(sanitized, legacyDefaultDashboardWidgets) {
		sanitized = appendMissingWidgets(sanitized, DefaultDashboardWidgets)
	}
	return sanitized
}

func containsAllWidgets(widgets, want []string) bool {
	seen := make(map[string]bool, len(widgets))
	for _, widget := range widgets {
		seen[widget] = true
	}
	for _, widget := range want {
		if !seen[widget] {
			return false
		}
	}
	return true
}

func appendMissingWidgets(widgets, defaults []string) []string {
	seen := make(map[string]bool, len(widgets)+len(defaults))
	for _, widget := range widgets {
		seen[widget] = true
	}
	for _, widget := range defaults {
		if seen[widget] {
			continue
		}
		widgets = append(widgets, widget)
		seen[widget] = true
	}
	return widgets
}

func SanitizeDashboardTheme(theme string) string {
	switch strings.ToLower(strings.TrimSpace(theme)) {
	case "classic", "btop", "neon", "mono":
		return strings.ToLower(strings.TrimSpace(theme))
	default:
		return "btop"
	}
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
	if len(cfg.ManualHosts) > 0 {
		contexts = append(contexts, core.CloudContext{
			Provider:        "Manual",
			ContextName:     "hosts",
			AccountID:       "manual-hosts",
			AccountName:     "Manual Hosts",
			Region:          "global",
			AuthMode:        AuthModeManual,
			CredentialScope: CredentialPersistenceNone,
		})
	}
	for _, managed := range cfg.CloudContexts {
		managed = SanitizeManagedCloudContext(managed)
		provider := normalizeProvider(managed.Provider)
		if provider == "" {
			continue
		}

		regions := normalizeRegions(provider, managed.Regions)
		for _, region := range regions {
			contexts = append(contexts, core.CloudContext{
				Provider:          provider,
				ContextName:       strings.TrimSpace(managed.ContextName),
				AccountID:         strings.TrimSpace(managed.AccountID),
				AccountName:       strings.TrimSpace(managed.AccountName),
				Region:            region,
				Tenant:            strings.TrimSpace(managed.Tenant),
				CredentialProfile: strings.TrimSpace(managed.CredentialProfile),
				AuthMode:          managed.AuthMode,
				CredentialScope:   managed.CredentialPersistence,
			})
		}
	}

	sort.Slice(contexts, func(i, j int) bool {
		if contexts[i].Provider != contexts[j].Provider {
			return contexts[i].Provider < contexts[j].Provider
		}
		if contexts[i].ContextName != contexts[j].ContextName {
			return contexts[i].ContextName < contexts[j].ContextName
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

func CurrentCloudContext(cfg AppConfig) (core.CloudContext, bool) {
	name := strings.TrimSpace(cfg.CurrentContext)
	if name == "" {
		return core.CloudContext{}, false
	}
	for _, ctx := range ManagedContexts(cfg) {
		if strings.EqualFold(ctx.ContextName, name) || strings.EqualFold(ctx.DisplayName(), name) {
			return ctx, true
		}
	}
	return core.CloudContext{}, false
}

func ResolveManagedContext(cfg AppConfig, name string) (ManagedCloudContext, int, bool) {
	name = sanitizeContextName(name)
	for i, managed := range cfg.CloudContexts {
		ctx := SanitizeManagedCloudContext(managed)
		if ctx.ContextName == name {
			return ctx, i, true
		}
	}
	return ManagedCloudContext{}, -1, false
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
		context  string
		id       string
		name     string
		tenant   string
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
			context:  strings.TrimSpace(ctx.ContextName),
			id:       strings.TrimSpace(ctx.AccountID),
			name:     strings.TrimSpace(ctx.AccountName),
			tenant:   strings.TrimSpace(ctx.Tenant),
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
		if keys[i].context != keys[j].context {
			return keys[i].context < keys[j].context
		}
		if keys[i].id != keys[j].id {
			return keys[i].id < keys[j].id
		}
		if keys[i].name != keys[j].name {
			return keys[i].name < keys[j].name
		}
		if keys[i].tenant != keys[j].tenant {
			return keys[i].tenant < keys[j].tenant
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
			ContextName:           key.context,
			Provider:              key.provider,
			AccountID:             key.id,
			AccountName:           key.name,
			Tenant:                key.tenant,
			AuthMode:              AuthModeNativeCLI,
			CredentialPersistence: CredentialPersistenceNativeCLI,
			CredentialProfile:     key.auth,
			Regions:               regions,
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
		kept = append(kept, SanitizeManagedCloudContext(replacement))
	}
	cfg.CloudContexts = kept
	return cfg
}

func normalizeProvider(provider string) string {
	provider = strings.TrimSpace(provider)
	switch strings.ToUpper(provider) {
	case "MANUAL", "HOSTS", "HOST", "UNMANAGED":
		return "Manual"
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
