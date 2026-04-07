package providers

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	"cloudmanager/internal/core"
	"cloudmanager/internal/providers/aws"
	"cloudmanager/internal/providers/azure"
	"cloudmanager/internal/providers/digitalocean"
	"cloudmanager/internal/providers/gcp"
)

type Capability string

const (
	CapabilityVMs       Capability = "vms"
	CapabilityDisks     Capability = "disks"
	CapabilitySnapshots Capability = "snapshots"
	CapabilityNetworks  Capability = "networks"
	CapabilityFirewalls Capability = "firewalls"
	CapabilityClusters  Capability = "clusters"
	CapabilityDatabases Capability = "databases"
	CapabilitySSH       Capability = "ssh"
	CapabilityCostGuide Capability = "cost_guide"
	CapabilityMetrics   Capability = "metrics"
)

type ProviderMetadata struct {
	ID              string
	DisplayName     string
	Order           int
	Aliases         []string
	GlobalLeafLabel string
	Capabilities    map[Capability]bool
}

func (m ProviderMetadata) Supports(capability Capability) bool {
	return m.Capabilities[capability]
}

type backendBindings struct {
	Compute   Provider
	Disks     DiskProvider
	Snapshots SnapshotProvider
	Firewalls FirewallProvider
	Networks  NetworkProvider
	Clusters  ClusterProvider
	Databases DatabaseProvider
	Metrics   MetricsProvider
	Billing   BillingProvider
}

type RegisteredProvider struct {
	Metadata     ProviderMetadata
	LoadContexts func() ([]core.CloudContext, []string)
	CLI          backendBindings
	SDK          backendBindings
}

type computeFuncs struct {
	fetch   func(ctx context.Context, cloudCtx core.CloudContext) ([]core.VM, error)
	execute func(ctx context.Context, action string, vm core.VM, cloudCtx core.CloudContext) (string, error)
	ssh     func(ctx context.Context, vm core.VM, cloudCtx core.CloudContext) (*exec.Cmd, error)
}

func (c computeFuncs) FetchVMs(ctx context.Context, cloudCtx core.CloudContext) ([]core.VM, error) {
	return c.fetch(ctx, cloudCtx)
}

func (c computeFuncs) ExecuteAction(ctx context.Context, action string, vm core.VM, cloudCtx core.CloudContext) (string, error) {
	return c.execute(ctx, action, vm, cloudCtx)
}

func (c computeFuncs) GetSSHCmd(ctx context.Context, vm core.VM, cloudCtx core.CloudContext) (*exec.Cmd, error) {
	return c.ssh(ctx, vm, cloudCtx)
}

type diskFuncs struct {
	fetch   func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Disk, error)
	execute func(ctx context.Context, action string, disk core.Disk, cloudCtx core.CloudContext) (string, error)
}

func (d diskFuncs) FetchDisks(ctx context.Context, cloudCtx core.CloudContext) ([]core.Disk, error) {
	return d.fetch(ctx, cloudCtx)
}

func (d diskFuncs) ExecuteDiskAction(ctx context.Context, action string, disk core.Disk, cloudCtx core.CloudContext) (string, error) {
	return d.execute(ctx, action, disk, cloudCtx)
}

type snapshotFuncs struct {
	fetch   func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Snapshot, error)
	execute func(ctx context.Context, action string, snap core.Snapshot, cloudCtx core.CloudContext) (string, error)
}

func (s snapshotFuncs) FetchSnapshots(ctx context.Context, cloudCtx core.CloudContext) ([]core.Snapshot, error) {
	return s.fetch(ctx, cloudCtx)
}

func (s snapshotFuncs) ExecuteSnapshotAction(ctx context.Context, action string, snap core.Snapshot, cloudCtx core.CloudContext) (string, error) {
	return s.execute(ctx, action, snap, cloudCtx)
}

type databaseFuncs struct {
	fetch func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Database, error)
}

func (d databaseFuncs) FetchDatabases(ctx context.Context, cloudCtx core.CloudContext) ([]core.Database, error) {
	return d.fetch(ctx, cloudCtx)
}

type clusterFuncs struct {
	fetch func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Cluster, error)
	k9s func(ctx context.Context, cluster core.Cluster, cloudCtx core.CloudContext) (*exec.Cmd, error)
}

func (c clusterFuncs) FetchClusters(ctx context.Context, cloudCtx core.CloudContext) ([]core.Cluster, error) {
	return c.fetch(ctx, cloudCtx)
}

func (c clusterFuncs) GetK9sCmd(ctx context.Context, cluster core.Cluster, cloudCtx core.CloudContext) (*exec.Cmd, error) {
	if c.k9s == nil { return nil, fmt.Errorf("k9s not supported") }
	return c.k9s(ctx, cluster, cloudCtx)
}

type metricsFuncs struct {
	fetch func(ctx context.Context, vm core.VM, cloudCtx core.CloudContext, period time.Duration) (*core.VMMetrics, error)
}

func (m metricsFuncs) FetchVMMetrics(ctx context.Context, vm core.VM, cloudCtx core.CloudContext, period time.Duration) (*core.VMMetrics, error) {
	return m.fetch(ctx, vm, cloudCtx, period)
}

type firewallFuncs struct {
	groups func(ctx context.Context, cloudCtx core.CloudContext) ([]core.SecurityGroup, error)
	rules  func(ctx context.Context, groupID string, cloudCtx core.CloudContext) ([]core.FirewallRule, error)
	execute func(ctx context.Context, action string, rule core.FirewallRule, cloudCtx core.CloudContext) (string, error)
}

func (f firewallFuncs) FetchSecurityGroups(ctx context.Context, cloudCtx core.CloudContext) ([]core.SecurityGroup, error) {
	return f.groups(ctx, cloudCtx)
}

func (f firewallFuncs) FetchFirewallRules(ctx context.Context, groupID string, cloudCtx core.CloudContext) ([]core.FirewallRule, error) {
	return f.rules(ctx, groupID, cloudCtx)
}

func (f firewallFuncs) ExecuteFirewallAction(ctx context.Context, action string, rule core.FirewallRule, cloudCtx core.CloudContext) (string, error) {
	if f.execute == nil { return "", fmt.Errorf("firewall modification not supported") }
	return f.execute(ctx, action, rule, cloudCtx)
}

type networkFuncs struct {
	networks func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Network, error)
	subnets  func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Subnet, error)
}

func (n networkFuncs) FetchNetworks(ctx context.Context, cloudCtx core.CloudContext) ([]core.Network, error) {
	return n.networks(ctx, cloudCtx)
}

func (n networkFuncs) FetchSubnets(ctx context.Context, cloudCtx core.CloudContext) ([]core.Subnet, error) {
	return n.subnets(ctx, cloudCtx)
}

type billingFuncs struct {
	account        func(ctx context.Context, cloudCtx core.CloudContext) (*core.AccountCost, error)
	resource       func(ctx context.Context, resourceID string, cloudCtx core.CloudContext) (*core.ResourceCost, error)
	recommendation func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Recommendation, error)
}

func (b billingFuncs) FetchAccountCost(ctx context.Context, cloudCtx core.CloudContext) (*core.AccountCost, error) {
	return b.account(ctx, cloudCtx)
}

func (b billingFuncs) FetchResourceCost(ctx context.Context, resourceID string, cloudCtx core.CloudContext) (*core.ResourceCost, error) {
	return b.resource(ctx, resourceID, cloudCtx)
}

func (b billingFuncs) FetchRecommendations(ctx context.Context, cloudCtx core.CloudContext) ([]core.Recommendation, error) {
	return b.recommendation(ctx, cloudCtx)
}

var providerRegistry = map[string]RegisteredProvider{}

func init() {
	registerBuiltins()
}

func RegisterProvider(provider RegisteredProvider) {
	normalizedID := NormalizeProviderName(provider.Metadata.ID)
	if normalizedID == "" {
		panic("provider metadata id cannot be empty")
	}
	provider.Metadata.ID = normalizedID
	if provider.Metadata.DisplayName == "" {
		provider.Metadata.DisplayName = normalizedID
	}
	providerRegistry[normalizedID] = provider
}

func RegisteredProviders() []RegisteredProvider {
	names := make([]string, 0, len(providerRegistry))
	for name := range providerRegistry {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		left := providerRegistry[names[i]].Metadata
		right := providerRegistry[names[j]].Metadata
		if left.Order != right.Order {
			return left.Order < right.Order
		}
		return left.DisplayName < right.DisplayName
	})

	registered := make([]RegisteredProvider, 0, len(names))
	for _, name := range names {
		registered = append(registered, providerRegistry[name])
	}
	return registered
}

func RegisteredProviderNames() []string {
	registered := RegisteredProviders()
	names := make([]string, 0, len(registered))
	for _, provider := range registered {
		names = append(names, provider.Metadata.DisplayName)
	}
	return names
}

func MetadataFor(name string) (ProviderMetadata, bool) {
	provider, ok := providerRegistry[NormalizeProviderName(name)]
	if !ok {
		return ProviderMetadata{}, false
	}
	return provider.Metadata, true
}

func Supports(name string, capability Capability) bool {
	meta, ok := MetadataFor(name)
	if !ok {
		return false
	}
	return meta.Supports(capability)
}

func DiscoverContexts(name string) ([]core.CloudContext, []string) {
	provider, ok := providerRegistry[NormalizeProviderName(name)]
	if !ok || provider.LoadContexts == nil {
		displayName := strings.TrimSpace(name)
		if displayName == "" {
			displayName = "unknown"
		}
		return nil, []string{fmt.Sprintf("%s: provider discovery not available", displayName)}
	}
	return provider.LoadContexts()
}

func NormalizeProviderName(name string) string {
	normalized := strings.TrimSpace(name)
	if normalized == "" {
		return ""
	}
	upper := strings.ToUpper(normalized)
	for _, provider := range providerRegistry {
		if strings.EqualFold(provider.Metadata.ID, normalized) || strings.EqualFold(provider.Metadata.DisplayName, normalized) {
			return provider.Metadata.DisplayName
		}
		for _, alias := range provider.Metadata.Aliases {
			if strings.EqualFold(alias, normalized) {
				return provider.Metadata.DisplayName
			}
		}
	}

	switch upper {
	case "AWS":
		return "AWS"
	case "GCP":
		return "GCP"
	case "AZURE":
		return "Azure"
	case "DIGITALOCEAN", "DIGITAL_OCEAN", "DO":
		return "DigitalOcean"
	default:
		return normalized
	}
}

func bindingsFor(providerName, backend string) (backendBindings, bool) {
	provider, ok := providerRegistry[NormalizeProviderName(providerName)]
	if !ok {
		return backendBindings{}, false
	}

	switch strings.ToLower(strings.TrimSpace(backend)) {
	case "sdk":
		return provider.SDK, true
	default:
		return provider.CLI, true
	}
}

func backendForProvider(providerName, backend string) (interface{}, bool) {
	bindings, ok := bindingsFor(providerName, backend)
	if !ok {
		return nil, false
	}
	switch strings.ToLower(strings.TrimSpace(backend)) {
	case "sdk":
		if bindings.Compute != nil {
			return bindings.Compute, true
		}
	default:
		if bindings.Compute != nil {
			return bindings.Compute, true
		}
	}
	return nil, false
}

func registerBuiltins() {
	RegisterProvider(RegisteredProvider{
		Metadata: ProviderMetadata{
			ID:              "AWS",
			DisplayName:     "AWS",
			Order:           10,
			Aliases:         []string{"aws"},
			GlobalLeafLabel: "",
			Capabilities: map[Capability]bool{
				CapabilityVMs:       true,
				CapabilityDisks:     true,
				CapabilitySnapshots: true,
				CapabilityFirewalls: true,
				CapabilityNetworks:  true,
				CapabilityClusters:  true,
				CapabilityDatabases: true,
				CapabilitySSH:       true,
				CapabilityCostGuide: true,
				CapabilityMetrics:   true,
			},
		},
		LoadContexts: aws.LoadContexts,
		CLI: backendBindings{
			Compute: computeFuncs{
				fetch: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.VM, error) {
					return aws.FetchVMsCLI(cloudCtx.CredentialProfile, cloudCtx.Region)
				},
				execute: aws.ExecuteActionCLI,
				ssh:     aws.GetSSHCmdCLI,
			},
			Disks: diskFuncs{
				fetch: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Disk, error) {
					return aws.FetchDisksSDK(ctx, cloudCtx.CredentialProfile, cloudCtx.Region)
				},
				execute: aws.ExecuteDiskActionSDK,
			},
			Snapshots: snapshotFuncs{
				fetch: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Snapshot, error) {
					return aws.FetchSnapshotsSDK(ctx, cloudCtx.CredentialProfile, cloudCtx.Region)
				},
				execute: aws.ExecuteSnapshotActionSDK,
			},
			Firewalls: firewallFuncs{
				groups: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.SecurityGroup, error) {
					return aws.FetchSecurityGroupsSDK(ctx, cloudCtx.CredentialProfile, cloudCtx.Region)
				},
				rules: func(ctx context.Context, groupID string, cloudCtx core.CloudContext) ([]core.FirewallRule, error) {
					return aws.FetchFirewallRulesSDK(ctx, cloudCtx.CredentialProfile, cloudCtx.Region, groupID)
				},
				execute: aws.ExecuteFirewallActionSDK,
			},
			Networks: networkFuncs{
				networks: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Network, error) {
					return aws.FetchNetworksSDK(ctx, cloudCtx.CredentialProfile, cloudCtx.Region)
				},
				subnets: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Subnet, error) {
					return aws.FetchSubnetsSDK(ctx, cloudCtx.CredentialProfile, cloudCtx.Region)
				},
			},
			Clusters: clusterFuncs{
				fetch: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Cluster, error) {
					return aws.FetchClustersCLI(cloudCtx.CredentialProfile, cloudCtx.Region)
				},
				k9s: aws.GetK9sCmd,
			},
			Metrics: metricsFuncs{
				fetch: func(ctx context.Context, vm core.VM, cloudCtx core.CloudContext, period time.Duration) (*core.VMMetrics, error) {
					return aws.FetchVMMetricsSDK(ctx, cloudCtx.CredentialProfile, cloudCtx.Region, vm.ID, period)
				},
			},
			Billing: billingFuncs{
				account: func(ctx context.Context, cloudCtx core.CloudContext) (*core.AccountCost, error) {
					return aws.FetchAccountCostSDK(ctx, cloudCtx.CredentialProfile)
				},
				resource: func(ctx context.Context, resourceID string, cloudCtx core.CloudContext) (*core.ResourceCost, error) {
					return aws.FetchVMCostSDK(ctx, cloudCtx.CredentialProfile, resourceID)
				},
				recommendation: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Recommendation, error) {
					return aws.FetchRecommendationsSDK(ctx, cloudCtx.CredentialProfile, cloudCtx.Region)
				},
			},
		},
		SDK: backendBindings{
			Compute: computeFuncs{
				fetch: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.VM, error) {
					return aws.FetchVMsSDK(ctx, cloudCtx.CredentialProfile, cloudCtx.Region)
				},
				execute: aws.ExecuteActionSDK,
				ssh:     aws.GetSSHCmdSDK,
			},
			Disks: diskFuncs{
				fetch: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Disk, error) {
					return aws.FetchDisksSDK(ctx, cloudCtx.CredentialProfile, cloudCtx.Region)
				},
				execute: aws.ExecuteDiskActionSDK,
			},
			Snapshots: snapshotFuncs{
				fetch: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Snapshot, error) {
					return aws.FetchSnapshotsSDK(ctx, cloudCtx.CredentialProfile, cloudCtx.Region)
				},
				execute: aws.ExecuteSnapshotActionSDK,
			},
			Firewalls: firewallFuncs{
				groups: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.SecurityGroup, error) {
					return aws.FetchSecurityGroupsSDK(ctx, cloudCtx.CredentialProfile, cloudCtx.Region)
				},
				rules: func(ctx context.Context, groupID string, cloudCtx core.CloudContext) ([]core.FirewallRule, error) {
					return aws.FetchFirewallRulesSDK(ctx, cloudCtx.CredentialProfile, cloudCtx.Region, groupID)
				},
				execute: aws.ExecuteFirewallActionSDK,
			},
			Networks: networkFuncs{
				networks: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Network, error) {
					return aws.FetchNetworksSDK(ctx, cloudCtx.CredentialProfile, cloudCtx.Region)
				},
				subnets: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Subnet, error) {
					return aws.FetchSubnetsSDK(ctx, cloudCtx.CredentialProfile, cloudCtx.Region)
				},
			},
			Clusters: clusterFuncs{
				fetch: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Cluster, error) {
					return aws.FetchClustersSDK(ctx, cloudCtx.CredentialProfile, cloudCtx.Region)
				},
				k9s: aws.GetK9sCmd,
			},
			Metrics: metricsFuncs{
				fetch: func(ctx context.Context, vm core.VM, cloudCtx core.CloudContext, period time.Duration) (*core.VMMetrics, error) {
					return aws.FetchVMMetricsSDK(ctx, cloudCtx.CredentialProfile, cloudCtx.Region, vm.ID, period)
				},
			},
			Billing: billingFuncs{
				account: func(ctx context.Context, cloudCtx core.CloudContext) (*core.AccountCost, error) {
					return aws.FetchAccountCostSDK(ctx, cloudCtx.CredentialProfile)
				},
				resource: func(ctx context.Context, resourceID string, cloudCtx core.CloudContext) (*core.ResourceCost, error) {
					return aws.FetchVMCostSDK(ctx, cloudCtx.CredentialProfile, resourceID)
				},
				recommendation: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Recommendation, error) {
					return aws.FetchRecommendationsSDK(ctx, cloudCtx.CredentialProfile, cloudCtx.Region)
				},
			},
		},
	})

	RegisterProvider(RegisteredProvider{
		Metadata: ProviderMetadata{
			ID:              "GCP",
			DisplayName:     "GCP",
			Order:           20,
			Aliases:         []string{"gcp", "googlecloud"},
			GlobalLeafLabel: "All Resources",
			Capabilities: map[Capability]bool{
				CapabilityVMs:       true,
				CapabilityDisks:     true,
				CapabilitySnapshots: true,
				CapabilityFirewalls: true,
				CapabilityNetworks:  true,
				CapabilityClusters:  true,
				CapabilityDatabases: true,
				CapabilitySSH:       true,
				CapabilityCostGuide: true,
				CapabilityMetrics:   true,
			},
		},
		LoadContexts: gcp.LoadContexts,
		CLI: backendBindings{
			Compute: computeFuncs{
				fetch: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.VM, error) {
					return gcp.FetchVMsCLI(cloudCtx.AccountID)
				},
				execute: gcp.ExecuteActionCLI,
				ssh:     gcp.GetSSHCmdCLI,
			},
			Disks: diskFuncs{
				fetch: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Disk, error) {
					return gcp.FetchDisksCLI(cloudCtx.AccountID)
				},
				execute: gcp.ExecuteDiskActionSDK,
			},
			Snapshots: snapshotFuncs{
				fetch: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Snapshot, error) {
					return gcp.FetchSnapshotsCLI(cloudCtx.AccountID)
				},
				execute: gcp.ExecuteSnapshotActionSDK,
			},
			Firewalls: firewallFuncs{
				groups: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.SecurityGroup, error) {
					return gcp.FetchSecurityGroupsCLI(cloudCtx.AccountID)
				},
				rules: func(ctx context.Context, groupID string, cloudCtx core.CloudContext) ([]core.FirewallRule, error) {
					return gcp.FetchFirewallRulesByNetworkCLI(cloudCtx.AccountID, groupID)
				},
				execute: gcp.ExecuteFirewallActionSDK,
			},
			Networks: networkFuncs{
				networks: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Network, error) {
					return gcp.FetchNetworksSDK(ctx, cloudCtx.AccountID)
				},
				subnets: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Subnet, error) {
					return gcp.FetchSubnetsSDK(ctx, cloudCtx.AccountID)
				},
			},
			Clusters: clusterFuncs{
				fetch: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Cluster, error) {
					return gcp.FetchClustersCLI(cloudCtx.AccountID)
				},
				k9s: gcp.GetK9sCmd,
			},
			Metrics: metricsFuncs{
				fetch: func(ctx context.Context, vm core.VM, cloudCtx core.CloudContext, period time.Duration) (*core.VMMetrics, error) {
					return gcp.FetchVMMetricsSDK(ctx, cloudCtx.AccountID, vm.Zone, vm.ID, period)
				},
			},
			Billing: billingFuncs{
				account: func(ctx context.Context, cloudCtx core.CloudContext) (*core.AccountCost, error) {
					return gcp.FetchAccountCostSDK(ctx, cloudCtx.AccountID, "billing_dataset", "gcp_billing_export_v1")
				},
				resource: func(ctx context.Context, resourceID string, cloudCtx core.CloudContext) (*core.ResourceCost, error) {
					return gcp.FetchVMCostSDK(ctx, cloudCtx.AccountID, resourceID)
				},
				recommendation: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Recommendation, error) {
					return gcp.FetchRecommendationsSDK(ctx, cloudCtx.AccountID)
				},
			},
		},
		SDK: backendBindings{
			Compute: computeFuncs{
				fetch: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.VM, error) {
					return gcp.FetchVMsSDK(ctx, cloudCtx.AccountID)
				},
				execute: gcp.ExecuteActionSDK,
				ssh:     gcp.GetSSHCmdSDK,
			},
			Disks: diskFuncs{
				fetch: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Disk, error) {
					return gcp.FetchDisksSDKWithCLIAuthFallback(ctx, cloudCtx.AccountID)
				},
				execute: gcp.ExecuteDiskActionSDK,
			},
			Snapshots: snapshotFuncs{
				fetch: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Snapshot, error) {
					return gcp.FetchSnapshotsSDKWithCLIAuthFallback(ctx, cloudCtx.AccountID)
				},
				execute: gcp.ExecuteSnapshotActionSDK,
			},
			Firewalls: firewallFuncs{
				groups: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.SecurityGroup, error) {
					return gcp.FetchSecurityGroupsSDKWithCLIAuthFallback(ctx, cloudCtx.AccountID)
				},
				rules: func(ctx context.Context, groupID string, cloudCtx core.CloudContext) ([]core.FirewallRule, error) {
					return gcp.FetchFirewallRulesByNetworkSDKWithCLIAuthFallback(ctx, cloudCtx.AccountID, groupID)
				},
				execute: gcp.ExecuteFirewallActionSDK,
			},
			Networks: networkFuncs{
				networks: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Network, error) {
					return gcp.FetchNetworksSDK(ctx, cloudCtx.AccountID)
				},
				subnets: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Subnet, error) {
					return gcp.FetchSubnetsSDK(ctx, cloudCtx.AccountID)
				},
			},
			Clusters: clusterFuncs{
				fetch: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Cluster, error) {
					return gcp.FetchClustersSDKWithCLIAuthFallback(ctx, cloudCtx.AccountID)
				},
				k9s: gcp.GetK9sCmd,
			},
			Metrics: metricsFuncs{
				fetch: func(ctx context.Context, vm core.VM, cloudCtx core.CloudContext, period time.Duration) (*core.VMMetrics, error) {
					return gcp.FetchVMMetricsSDK(ctx, cloudCtx.AccountID, vm.Zone, vm.ID, period)
				},
			},
			Billing: billingFuncs{
				account: func(ctx context.Context, cloudCtx core.CloudContext) (*core.AccountCost, error) {
					return gcp.FetchAccountCostSDK(ctx, cloudCtx.AccountID, "billing_dataset", "gcp_billing_export_v1")
				},
				resource: func(ctx context.Context, resourceID string, cloudCtx core.CloudContext) (*core.ResourceCost, error) {
					return gcp.FetchVMCostSDK(ctx, cloudCtx.AccountID, resourceID)
				},
				recommendation: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Recommendation, error) {
					return gcp.FetchRecommendationsSDK(ctx, cloudCtx.AccountID)
				},
			},
		},
	})

	RegisterProvider(RegisteredProvider{
		Metadata: ProviderMetadata{
			ID:              "Azure",
			DisplayName:     "Azure",
			Order:           30,
			Aliases:         []string{"azure"},
			GlobalLeafLabel: "All Resources",
			Capabilities: map[Capability]bool{
				CapabilityVMs:       true,
				CapabilityDisks:     true,
				CapabilitySnapshots: true,
				CapabilityFirewalls: true,
				CapabilityNetworks:  true,
				CapabilityClusters:  true,
				CapabilityDatabases: true,
				CapabilitySSH:       true,
				CapabilityCostGuide: true,
				CapabilityMetrics:   true,
			},
		},
		LoadContexts: azure.LoadContexts,
		CLI: backendBindings{
			Compute: computeFuncs{
				fetch: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.VM, error) {
					return azure.FetchVMsCLI(cloudCtx.AccountID)
				},
				execute: azure.ExecuteActionCLI,
				ssh:     azure.GetSSHCmdCLI,
			},
			Disks: diskFuncs{
				fetch: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Disk, error) {
					return azure.FetchDisksSDK(ctx, cloudCtx.AccountID)
				},
				execute: azure.ExecuteDiskActionSDK,
			},
			Snapshots: snapshotFuncs{
				fetch: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Snapshot, error) {
					return azure.FetchSnapshotsSDK(ctx, cloudCtx.AccountID)
				},
				execute: azure.ExecuteSnapshotActionSDK,
			},
			Firewalls: firewallFuncs{
				groups: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.SecurityGroup, error) {
					return azure.FetchSecurityGroupsSDK(ctx, cloudCtx.AccountID)
				},
				rules: func(ctx context.Context, groupID string, cloudCtx core.CloudContext) ([]core.FirewallRule, error) {
					resourceGroup := azureResourceGroupFromFirewallID(groupID)
					nsgName := azureNameFromFirewallID(groupID)
					return azure.FetchFirewallRulesSDK(ctx, cloudCtx.AccountID, resourceGroup, nsgName)
				},
			},
			Networks: networkFuncs{
				networks: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Network, error) {
					return azure.FetchNetworksSDK(ctx, cloudCtx.AccountID)
				},
				subnets: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Subnet, error) {
					return azure.FetchSubnetsSDK(ctx, cloudCtx.AccountID)
				},
			},
			Clusters: clusterFuncs{
				fetch: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Cluster, error) {
					return azure.FetchClustersSDK(ctx, cloudCtx.AccountID)
				},
				k9s: azure.GetK9sCmd,
			},
			Metrics: metricsFuncs{
				fetch: func(ctx context.Context, vm core.VM, cloudCtx core.CloudContext, period time.Duration) (*core.VMMetrics, error) {
					return azure.FetchVMMetricsSDK(ctx, cloudCtx.AccountID, vm.ID, period)
				},
			},
			Billing: billingFuncs{
				account: func(ctx context.Context, cloudCtx core.CloudContext) (*core.AccountCost, error) {
					return azure.FetchAccountCostSDK(ctx, cloudCtx.AccountID)
				},
				resource: func(ctx context.Context, resourceID string, cloudCtx core.CloudContext) (*core.ResourceCost, error) {
					return azure.FetchVMCostSDK(ctx, cloudCtx.AccountID, resourceID)
				},
				recommendation: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Recommendation, error) {
					return azure.FetchRecommendationsSDK(ctx, cloudCtx.AccountID)
				},
			},
		},
		SDK: backendBindings{
			Compute: computeFuncs{
				fetch: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.VM, error) {
					return azure.FetchVMsSDK(ctx, cloudCtx.AccountID)
				},
				execute: azure.ExecuteActionSDK,
				ssh:     azure.GetSSHCmdSDK,
			},
			Disks: diskFuncs{
				fetch: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Disk, error) {
					return azure.FetchDisksSDK(ctx, cloudCtx.AccountID)
				},
				execute: azure.ExecuteDiskActionSDK,
			},
			Snapshots: snapshotFuncs{
				fetch: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Snapshot, error) {
					return azure.FetchSnapshotsSDK(ctx, cloudCtx.AccountID)
				},
				execute: azure.ExecuteSnapshotActionSDK,
			},
			Firewalls: firewallFuncs{
				groups: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.SecurityGroup, error) {
					return azure.FetchSecurityGroupsSDK(ctx, cloudCtx.AccountID)
				},
				rules: func(ctx context.Context, groupID string, cloudCtx core.CloudContext) ([]core.FirewallRule, error) {
					resourceGroup := azureResourceGroupFromFirewallID(groupID)
					nsgName := azureNameFromFirewallID(groupID)
					return azure.FetchFirewallRulesSDK(ctx, cloudCtx.AccountID, resourceGroup, nsgName)
				},
			},
			Networks: networkFuncs{
				networks: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Network, error) {
					return azure.FetchNetworksSDK(ctx, cloudCtx.AccountID)
				},
				subnets: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Subnet, error) {
					return azure.FetchSubnetsSDK(ctx, cloudCtx.AccountID)
				},
			},
			Metrics: metricsFuncs{
				fetch: func(ctx context.Context, vm core.VM, cloudCtx core.CloudContext, period time.Duration) (*core.VMMetrics, error) {
					return azure.FetchVMMetricsSDK(ctx, cloudCtx.AccountID, vm.ID, period)
				},
			},
			Billing: billingFuncs{
				account: func(ctx context.Context, cloudCtx core.CloudContext) (*core.AccountCost, error) {
					return azure.FetchAccountCostSDK(ctx, cloudCtx.AccountID)
				},
				resource: func(ctx context.Context, resourceID string, cloudCtx core.CloudContext) (*core.ResourceCost, error) {
					return azure.FetchVMCostSDK(ctx, cloudCtx.AccountID, resourceID)
				},
				recommendation: func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Recommendation, error) {
					return azure.FetchRecommendationsSDK(ctx, cloudCtx.AccountID)
				},
			},
		},
	})

	doCompute := computeFuncs{
		fetch:   digitalocean.FetchVMsCLI,
		execute: digitalocean.ExecuteActionCLI,
		ssh:     digitalocean.GetSSHCmdCLI,
	}

	RegisterProvider(RegisteredProvider{
		Metadata: ProviderMetadata{
			ID:              "DigitalOcean",
			DisplayName:     "DigitalOcean",
			Order:           40,
			Aliases:         []string{"digitalocean", "digital_ocean", "do"},
			GlobalLeafLabel: "All Resources",
			Capabilities: map[Capability]bool{
				CapabilityVMs:       true,
				CapabilitySSH:       true,
				CapabilityCostGuide: true,
			},
		},
		LoadContexts: digitalocean.LoadContexts,
		CLI: backendBindings{
			Compute: doCompute,
		},
		SDK: backendBindings{
			Compute: doCompute,
		},
	})
}

func azureResourceGroupFromFirewallID(id string) string {
	parts := strings.Split(id, "/")
	for i, part := range parts {
		if strings.EqualFold(part, "resourceGroups") && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return "-"
}

func azureNameFromFirewallID(id string) string {
	parts := strings.Split(strings.TrimRight(id, "/"), "/")
	if len(parts) == 0 {
		return id
	}
	return parts[len(parts)-1]
}

