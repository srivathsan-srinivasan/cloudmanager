package providers

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"cloudmanager/internal/core"
)

// SDKProvider uses native Go SDKs for cloud operations.
type SDKProvider struct{}

func (p *SDKProvider) FetchVMs(ctx context.Context, cloudCtx core.CloudContext) ([]core.VM, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "sdk")
	if !ok || bindings.Compute == nil {
		return nil, fmt.Errorf("SDK backend not implemented for %s", cloudCtx.Provider)
	}
	return bindings.Compute.FetchVMs(ctx, cloudCtx)
}

func (p *SDKProvider) ExecuteAction(ctx context.Context, action string, vm core.VM, cloudCtx core.CloudContext) (string, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "sdk")
	if !ok || bindings.Compute == nil {
		return "", fmt.Errorf("SDK backend not implemented for %s", cloudCtx.Provider)
	}
	return bindings.Compute.ExecuteAction(ctx, action, vm, cloudCtx)
}

func (p *SDKProvider) GetSSHCmd(ctx context.Context, vm core.VM, cloudCtx core.CloudContext) (*exec.Cmd, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "sdk")
	if !ok || bindings.Compute == nil {
		return nil, fmt.Errorf("SSH not supported for %s via SDK", cloudCtx.Provider)
	}
	return bindings.Compute.GetSSHCmd(ctx, vm, cloudCtx)
}

func (p *SDKProvider) FetchDisks(ctx context.Context, cloudCtx core.CloudContext) ([]core.Disk, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "sdk")
	if !ok || bindings.Disks == nil {
		return nil, fmt.Errorf("SDK backend not implemented for %s", cloudCtx.Provider)
	}
	return bindings.Disks.FetchDisks(ctx, cloudCtx)
}

func (p *SDKProvider) ExecuteDiskAction(ctx context.Context, action string, disk core.Disk, cloudCtx core.CloudContext) (string, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "sdk")
	if !ok || bindings.Disks == nil {
		return "", fmt.Errorf("SDK backend not implemented for %s", cloudCtx.Provider)
	}
	return bindings.Disks.ExecuteDiskAction(ctx, action, disk, cloudCtx)
}

func (p *SDKProvider) FetchSnapshots(ctx context.Context, cloudCtx core.CloudContext) ([]core.Snapshot, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "sdk")
	if !ok || bindings.Snapshots == nil {
		return nil, fmt.Errorf("SDK backend not implemented for %s", cloudCtx.Provider)
	}
	return bindings.Snapshots.FetchSnapshots(ctx, cloudCtx)
}

func (p *SDKProvider) ExecuteSnapshotAction(ctx context.Context, action string, snap core.Snapshot, cloudCtx core.CloudContext) (string, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "sdk")
	if !ok || bindings.Snapshots == nil {
		return "", fmt.Errorf("SDK backend not implemented for %s", cloudCtx.Provider)
	}
	return bindings.Snapshots.ExecuteSnapshotAction(ctx, action, snap, cloudCtx)
}

func (p *SDKProvider) FetchSecurityGroups(ctx context.Context, cloudCtx core.CloudContext) ([]core.SecurityGroup, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "sdk")
	if !ok || bindings.Firewalls == nil {
		return nil, fmt.Errorf("SDK backend not implemented for %s", cloudCtx.Provider)
	}
	return bindings.Firewalls.FetchSecurityGroups(ctx, cloudCtx)
}

func (p *SDKProvider) FetchFirewallRules(ctx context.Context, groupID string, cloudCtx core.CloudContext) ([]core.FirewallRule, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "sdk")
	if !ok || bindings.Firewalls == nil {
		return nil, fmt.Errorf("SDK backend not implemented for %s", cloudCtx.Provider)
	}
	return bindings.Firewalls.FetchFirewallRules(ctx, groupID, cloudCtx)
}

func (p *SDKProvider) FetchNetworks(ctx context.Context, cloudCtx core.CloudContext) ([]core.Network, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "sdk")
	if !ok || bindings.Networks == nil {
		return nil, fmt.Errorf("SDK backend not implemented for networks on %s", cloudCtx.Provider)
	}
	return bindings.Networks.FetchNetworks(ctx, cloudCtx)
}

func (p *SDKProvider) FetchSubnets(ctx context.Context, cloudCtx core.CloudContext) ([]core.Subnet, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "sdk")
	if !ok || bindings.Networks == nil {
		return nil, fmt.Errorf("SDK backend not implemented for subnets on %s", cloudCtx.Provider)
	}
	return bindings.Networks.FetchSubnets(ctx, cloudCtx)
}

func (p *SDKProvider) FetchVMMetrics(ctx context.Context, vm core.VM, cloudCtx core.CloudContext, period time.Duration) (*core.VMMetrics, error) {
	return nil, fmt.Errorf("not implemented")
}

func (p *SDKProvider) FetchAccountCost(ctx context.Context, cloudCtx core.CloudContext) (*core.AccountCost, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "sdk")
	if !ok || bindings.Billing == nil {
		return nil, fmt.Errorf("SDK backend not implemented for %s", cloudCtx.Provider)
	}
	return bindings.Billing.FetchAccountCost(ctx, cloudCtx)
}

func (p *SDKProvider) FetchResourceCost(ctx context.Context, resourceID string, cloudCtx core.CloudContext) (*core.ResourceCost, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "sdk")
	if !ok || bindings.Billing == nil {
		return nil, fmt.Errorf("SDK backend not implemented for %s", cloudCtx.Provider)
	}
	return bindings.Billing.FetchResourceCost(ctx, resourceID, cloudCtx)
}

func (p *SDKProvider) FetchRecommendations(ctx context.Context, cloudCtx core.CloudContext) ([]core.Recommendation, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "sdk")
	if !ok || bindings.Billing == nil {
		return nil, fmt.Errorf("SDK backend not implemented for %s", cloudCtx.Provider)
	}
	return bindings.Billing.FetchRecommendations(ctx, cloudCtx)
}

func (p *SDKProvider) FetchClusters(ctx context.Context, cloudCtx core.CloudContext) ([]core.Cluster, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "sdk")
	if !ok || bindings.Clusters == nil {
		return nil, fmt.Errorf("clusters not supported for %s", cloudCtx.Provider)
	}
	return bindings.Clusters.FetchClusters(ctx, cloudCtx)
}

func (p *SDKProvider) FetchDatabases(ctx context.Context, cloudCtx core.CloudContext) ([]core.Database, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "sdk")
	if !ok || bindings.Databases == nil {
		return nil, fmt.Errorf("databases not supported for %s", cloudCtx.Provider)
	}
	return bindings.Databases.FetchDatabases(ctx, cloudCtx)
}

func (p *SDKProvider) GetK9sCmd(ctx context.Context, cluster core.Cluster, cloudCtx core.CloudContext) (*exec.Cmd, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "sdk")
	if !ok || bindings.Clusters == nil {
		return nil, fmt.Errorf("k9s not supported for %s", cloudCtx.Provider)
	}
	return bindings.Clusters.GetK9sCmd(ctx, cluster, cloudCtx)
}

func (p *SDKProvider) ExecuteFirewallAction(ctx context.Context, action string, rule core.FirewallRule, cloudCtx core.CloudContext) (string, error) {
	bindings, ok := bindingsFor(cloudCtx.Provider, "sdk")
	if !ok || bindings.Firewalls == nil {
		return "", fmt.Errorf("firewall actions not supported for %s", cloudCtx.Provider)
	}
	return bindings.Firewalls.ExecuteFirewallAction(ctx, action, rule, cloudCtx)
}
