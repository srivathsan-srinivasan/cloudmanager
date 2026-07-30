package providers

import (
	"context"
	"os/exec"
	"time"

	"github.com/vyoogam/cloudmanager/internal/core"
)

// MockProvider is a test implementation of all provider interfaces.
// It uses function fields so tests can easily mock specific methods.
type MockProvider struct {
	FetchVMsFn              func(ctx context.Context, cloudCtx core.CloudContext) ([]core.VM, error)
	ExecuteActionFn         func(ctx context.Context, action string, vm core.VM, cloudCtx core.CloudContext) (string, error)
	GetSSHCmdFn             func(ctx context.Context, vm core.VM, cloudCtx core.CloudContext) (*exec.Cmd, error)
	GetPortForwardCmdFn     func(ctx context.Context, vm core.VM, cloudCtx core.CloudContext, specs []core.PortForwardSpec) (*exec.Cmd, error)
	GetSCPCmdFn             func(ctx context.Context, vm core.VM, cloudCtx core.CloudContext, transfer core.SCPTransfer) (*exec.Cmd, error)
	FetchDisksFn            func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Disk, error)
	ExecuteDiskActionFn     func(ctx context.Context, action string, disk core.Disk, cloudCtx core.CloudContext) (string, error)
	FetchSnapshotsFn        func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Snapshot, error)
	ExecuteSnapshotActionFn func(ctx context.Context, action string, snap core.Snapshot, cloudCtx core.CloudContext) (string, error)
	FetchSecurityGroupsFn   func(ctx context.Context, cloudCtx core.CloudContext) ([]core.SecurityGroup, error)
	FetchFirewallRulesFn    func(ctx context.Context, groupID string, cloudCtx core.CloudContext) ([]core.FirewallRule, error)
	ExecuteFirewallActionFn func(ctx context.Context, action string, rule core.FirewallRule, cloudCtx core.CloudContext) (string, error)
	FetchNetworksFn         func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Network, error)
	FetchSubnetsFn          func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Subnet, error)
	FetchVMMetricsFn        func(ctx context.Context, vm core.VM, cloudCtx core.CloudContext, period time.Duration) (*core.VMMetrics, error)
	FetchAccountCostFn      func(ctx context.Context, cloudCtx core.CloudContext) (*core.AccountCost, error)
	FetchResourceCostFn     func(ctx context.Context, resourceID string, cloudCtx core.CloudContext) (*core.ResourceCost, error)
	FetchRecommendationsFn  func(ctx context.Context, cloudCtx core.CloudContext) ([]core.Recommendation, error)
	FetchStorageBucketsFn   func(ctx context.Context, cloudCtx core.CloudContext) ([]core.StorageBucket, error)
}

func (m *MockProvider) FetchVMs(ctx context.Context, cloudCtx core.CloudContext) ([]core.VM, error) {
	if m.FetchVMsFn != nil {
		return m.FetchVMsFn(ctx, cloudCtx)
	}
	return nil, nil
}

func (m *MockProvider) ExecuteAction(ctx context.Context, action string, vm core.VM, cloudCtx core.CloudContext) (string, error) {
	if m.ExecuteActionFn != nil {
		return m.ExecuteActionFn(ctx, action, vm, cloudCtx)
	}
	return "", nil
}

func (m *MockProvider) GetSSHCmd(ctx context.Context, vm core.VM, cloudCtx core.CloudContext) (*exec.Cmd, error) {
	if m.GetSSHCmdFn != nil {
		return m.GetSSHCmdFn(ctx, vm, cloudCtx)
	}
	return nil, nil
}

func (m *MockProvider) GetPortForwardCmd(ctx context.Context, vm core.VM, cloudCtx core.CloudContext, specs []core.PortForwardSpec) (*exec.Cmd, error) {
	if m.GetPortForwardCmdFn != nil {
		return m.GetPortForwardCmdFn(ctx, vm, cloudCtx, specs)
	}
	return nil, nil
}

func (m *MockProvider) GetSCPCmd(ctx context.Context, vm core.VM, cloudCtx core.CloudContext, transfer core.SCPTransfer) (*exec.Cmd, error) {
	if m.GetSCPCmdFn != nil {
		return m.GetSCPCmdFn(ctx, vm, cloudCtx, transfer)
	}
	return nil, nil
}

func (m *MockProvider) FetchDisks(ctx context.Context, cloudCtx core.CloudContext) ([]core.Disk, error) {
	if m.FetchDisksFn != nil {
		return m.FetchDisksFn(ctx, cloudCtx)
	}
	return nil, nil
}

func (m *MockProvider) ExecuteDiskAction(ctx context.Context, action string, disk core.Disk, cloudCtx core.CloudContext) (string, error) {
	if m.ExecuteDiskActionFn != nil {
		return m.ExecuteDiskActionFn(ctx, action, disk, cloudCtx)
	}
	return "", nil
}

func (m *MockProvider) FetchSnapshots(ctx context.Context, cloudCtx core.CloudContext) ([]core.Snapshot, error) {
	if m.FetchSnapshotsFn != nil {
		return m.FetchSnapshotsFn(ctx, cloudCtx)
	}
	return nil, nil
}

func (m *MockProvider) ExecuteSnapshotAction(ctx context.Context, action string, snap core.Snapshot, cloudCtx core.CloudContext) (string, error) {
	if m.ExecuteSnapshotActionFn != nil {
		return m.ExecuteSnapshotActionFn(ctx, action, snap, cloudCtx)
	}
	return "", nil
}

func (m *MockProvider) FetchSecurityGroups(ctx context.Context, cloudCtx core.CloudContext) ([]core.SecurityGroup, error) {
	if m.FetchSecurityGroupsFn != nil {
		return m.FetchSecurityGroupsFn(ctx, cloudCtx)
	}
	return nil, nil
}

func (m *MockProvider) FetchFirewallRules(ctx context.Context, groupID string, cloudCtx core.CloudContext) ([]core.FirewallRule, error) {
	if m.FetchFirewallRulesFn != nil {
		return m.FetchFirewallRulesFn(ctx, groupID, cloudCtx)
	}
	return nil, nil
}

func (m *MockProvider) ExecuteFirewallAction(ctx context.Context, action string, rule core.FirewallRule, cloudCtx core.CloudContext) (string, error) {
	if m.ExecuteFirewallActionFn != nil {
		return m.ExecuteFirewallActionFn(ctx, action, rule, cloudCtx)
	}
	return "", nil
}

func (m *MockProvider) FetchNetworks(ctx context.Context, cloudCtx core.CloudContext) ([]core.Network, error) {
	if m.FetchNetworksFn != nil {
		return m.FetchNetworksFn(ctx, cloudCtx)
	}
	return nil, nil
}

func (m *MockProvider) FetchSubnets(ctx context.Context, cloudCtx core.CloudContext) ([]core.Subnet, error) {
	if m.FetchSubnetsFn != nil {
		return m.FetchSubnetsFn(ctx, cloudCtx)
	}
	return nil, nil
}

func (m *MockProvider) FetchVMMetrics(ctx context.Context, vm core.VM, cloudCtx core.CloudContext, period time.Duration) (*core.VMMetrics, error) {
	if m.FetchVMMetricsFn != nil {
		return m.FetchVMMetricsFn(ctx, vm, cloudCtx, period)
	}
	return nil, nil
}

func (m *MockProvider) FetchAccountCost(ctx context.Context, cloudCtx core.CloudContext) (*core.AccountCost, error) {
	if m.FetchAccountCostFn != nil {
		return m.FetchAccountCostFn(ctx, cloudCtx)
	}
	return nil, nil
}

func (m *MockProvider) FetchResourceCost(ctx context.Context, resourceID string, cloudCtx core.CloudContext) (*core.ResourceCost, error) {
	if m.FetchResourceCostFn != nil {
		return m.FetchResourceCostFn(ctx, resourceID, cloudCtx)
	}
	return nil, nil
}

func (m *MockProvider) FetchRecommendations(ctx context.Context, cloudCtx core.CloudContext) ([]core.Recommendation, error) {
	if m.FetchRecommendationsFn != nil {
		return m.FetchRecommendationsFn(ctx, cloudCtx)
	}
	return nil, nil
}

func (m *MockProvider) FetchStorageBuckets(ctx context.Context, cloudCtx core.CloudContext) ([]core.StorageBucket, error) {
	if m.FetchStorageBucketsFn != nil {
		return m.FetchStorageBucketsFn(ctx, cloudCtx)
	}
	return nil, nil
}
