package providers

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"cloudmanager/internal/core"
	applog "cloudmanager/internal/logging"
)

// CLIProvider wraps the native CLI tools (aws, gcloud, az) for cloud operations.
type CLIProvider struct{}

func (p *CLIProvider) FetchVMs(ctx context.Context, cloudCtx core.CloudContext) ([]core.VM, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "cli")
	if !ok || bindings.Compute == nil {
		return nil, fmt.Errorf("provider %s not supported", cloudCtx.Provider)
	}
	return bindings.Compute.FetchVMs(ctx, cloudCtx)
}

func (p *CLIProvider) ExecuteAction(ctx context.Context, action string, vm core.VM, cloudCtx core.CloudContext) (string, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "cli")
	if !ok || bindings.Compute == nil {
		return "", fmt.Errorf("action %s not supported for %s", action, cloudCtx.Provider)
	}
	return bindings.Compute.ExecuteAction(ctx, action, vm, cloudCtx)
}

func (p *CLIProvider) GetSSHCmd(ctx context.Context, vm core.VM, cloudCtx core.CloudContext) (*exec.Cmd, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "cli")
	if !ok || bindings.Compute == nil {
		return nil, fmt.Errorf("SSH not supported for %s", cloudCtx.Provider)
	}
	return bindings.Compute.GetSSHCmd(ctx, vm, cloudCtx)
}

// --- SDK Fallbacks for New Resources ---
// The CLI provider uses the SDK for new resources (Disks, Snapshots, Billing)
// as per the roadmap.

func (p *CLIProvider) FetchDisks(ctx context.Context, cloudCtx core.CloudContext) ([]core.Disk, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "cli")
	if !ok || bindings.Disks == nil {
		return nil, fmt.Errorf("disks not supported for %s", cloudCtx.Provider)
	}
	return bindings.Disks.FetchDisks(ctx, cloudCtx)
}

func (p *CLIProvider) ExecuteDiskAction(ctx context.Context, action string, disk core.Disk, cloudCtx core.CloudContext) (string, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "cli")
	if !ok || bindings.Disks == nil {
		return "", fmt.Errorf("disk action %s not supported for %s", action, cloudCtx.Provider)
	}
	return bindings.Disks.ExecuteDiskAction(ctx, action, disk, cloudCtx)
}

func (p *CLIProvider) FetchSnapshots(ctx context.Context, cloudCtx core.CloudContext) ([]core.Snapshot, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "cli")
	if !ok || bindings.Snapshots == nil {
		return nil, fmt.Errorf("snapshots not supported for %s", cloudCtx.Provider)
	}
	return bindings.Snapshots.FetchSnapshots(ctx, cloudCtx)
}

func (p *CLIProvider) ExecuteSnapshotAction(ctx context.Context, action string, snap core.Snapshot, cloudCtx core.CloudContext) (string, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "cli")
	if !ok || bindings.Snapshots == nil {
		return "", fmt.Errorf("snapshot action %s not supported for %s", action, cloudCtx.Provider)
	}
	return bindings.Snapshots.ExecuteSnapshotAction(ctx, action, snap, cloudCtx)
}

func (p *CLIProvider) FetchSecurityGroups(ctx context.Context, cloudCtx core.CloudContext) ([]core.SecurityGroup, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "cli")
	if !ok || bindings.Firewalls == nil {
		return nil, fmt.Errorf("firewalls not supported for %s", cloudCtx.Provider)
	}
	return bindings.Firewalls.FetchSecurityGroups(ctx, cloudCtx)
}

func (p *CLIProvider) FetchFirewallRules(ctx context.Context, groupID string, cloudCtx core.CloudContext) ([]core.FirewallRule, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "cli")
	if !ok || bindings.Firewalls == nil {
		return nil, fmt.Errorf("firewalls not supported for %s", cloudCtx.Provider)
	}
	return bindings.Firewalls.FetchFirewallRules(ctx, groupID, cloudCtx)
}

func (p *CLIProvider) FetchAccountCost(ctx context.Context, cloudCtx core.CloudContext) (*core.AccountCost, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "cli")
	if !ok || bindings.Billing == nil {
		return nil, fmt.Errorf("billing not supported for %s", cloudCtx.Provider)
	}
	return bindings.Billing.FetchAccountCost(ctx, cloudCtx)
}

func (p *CLIProvider) FetchResourceCost(ctx context.Context, resourceID string, cloudCtx core.CloudContext) (*core.ResourceCost, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "cli")
	if !ok || bindings.Billing == nil {
		return nil, fmt.Errorf("billing not supported for %s", cloudCtx.Provider)
	}
	return bindings.Billing.FetchResourceCost(ctx, resourceID, cloudCtx)
}

func (p *CLIProvider) FetchRecommendations(ctx context.Context, cloudCtx core.CloudContext) ([]core.Recommendation, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "cli")
	if !ok || bindings.Billing == nil {
		return nil, fmt.Errorf("recommendations not supported for %s", cloudCtx.Provider)
	}
	return bindings.Billing.FetchRecommendations(ctx, cloudCtx)
}

func (p *CLIProvider) FetchClusters(ctx context.Context, cloudCtx core.CloudContext) ([]core.Cluster, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "cli")
	if !ok || bindings.Clusters == nil {
		return nil, fmt.Errorf("clusters not supported for %s", cloudCtx.Provider)
	}
	return bindings.Clusters.FetchClusters(ctx, cloudCtx)
}

func (p *CLIProvider) FetchDatabases(ctx context.Context, cloudCtx core.CloudContext) ([]core.Database, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "cli")
	if !ok || bindings.Databases == nil {
		return nil, fmt.Errorf("databases not supported for %s", cloudCtx.Provider)
	}
	return bindings.Databases.FetchDatabases(ctx, cloudCtx)
}

func (p *CLIProvider) FetchStorageBuckets(ctx context.Context, cloudCtx core.CloudContext) ([]core.StorageBucket, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "cli")
	if !ok || bindings.Storage == nil {
		return nil, fmt.Errorf("storage not supported for %s", cloudCtx.Provider)
	}
	return bindings.Storage.FetchStorageBuckets(ctx, cloudCtx)
}

func (p *CLIProvider) GetK9sCmd(ctx context.Context, cluster core.Cluster, cloudCtx core.CloudContext) (*exec.Cmd, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "cli")
	if !ok || bindings.Clusters == nil {
		return nil, fmt.Errorf("k9s not supported for %s", cloudCtx.Provider)
	}
	return bindings.Clusters.GetK9sCmd(ctx, cluster, cloudCtx)
}

func (p *CLIProvider) ExecuteFirewallAction(ctx context.Context, action string, rule core.FirewallRule, cloudCtx core.CloudContext) (string, error) {
	if strings.TrimSpace(action) != "" {
		applog.Warnf("component=firewall_rules event=action_blocked provider=%s account=%s region=%s mode=CLI action=%s rule=%s err=firewall updates require SDK mode", cloudCtx.Provider, cloudCtx.AccountID, cloudCtx.Region, action, rule.ID)
	}
	return "", fmt.Errorf("firewall updates require SDK mode; switch to SDK and retry")
}

func (p *CLIProvider) FetchNetworks(ctx context.Context, cloudCtx core.CloudContext) ([]core.Network, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "cli")
	if !ok || bindings.Networks == nil {
		return nil, fmt.Errorf("networks not supported for %s", cloudCtx.Provider)
	}
	return bindings.Networks.FetchNetworks(ctx, cloudCtx)
}

func (p *CLIProvider) FetchSubnets(ctx context.Context, cloudCtx core.CloudContext) ([]core.Subnet, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "cli")
	if !ok || bindings.Networks == nil {
		return nil, fmt.Errorf("subnets not supported for %s", cloudCtx.Provider)
	}
	return bindings.Networks.FetchSubnets(ctx, cloudCtx)
}
