package providers

import (
	"context"
	"os/exec"
	"time"

	"github.com/srivathsan-srinivasan/cloudmanager/internal/config"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
)

// Provider is the main interface for interacting with cloud resources.
// Each cloud (AWS, GCP, Azure) implements this for both CLI and SDK backends.
type Provider interface {
	FetchVMs(ctx context.Context, cloudCtx core.CloudContext) ([]core.VM, error)
	ExecuteAction(ctx context.Context, action string, vm core.VM, cloudCtx core.CloudContext) (string, error)
	GetSSHCmd(ctx context.Context, vm core.VM, cloudCtx core.CloudContext) (*exec.Cmd, error)
}

type DiskProvider interface {
	FetchDisks(ctx context.Context, cloudCtx core.CloudContext) ([]core.Disk, error)
	ExecuteDiskAction(ctx context.Context, action string, disk core.Disk, cloudCtx core.CloudContext) (string, error)
}

type SnapshotProvider interface {
	FetchSnapshots(ctx context.Context, cloudCtx core.CloudContext) ([]core.Snapshot, error)
	ExecuteSnapshotAction(ctx context.Context, action string, snap core.Snapshot, cloudCtx core.CloudContext) (string, error)
}

type ClusterProvider interface {
	FetchClusters(ctx context.Context, cloudCtx core.CloudContext) ([]core.Cluster, error)
	GetK9sCmd(ctx context.Context, cluster core.Cluster, cloudCtx core.CloudContext) (*exec.Cmd, error)
}

type DatabaseProvider interface {
	FetchDatabases(ctx context.Context, cloudCtx core.CloudContext) ([]core.Database, error)
}

type StorageProvider interface {
	FetchStorageBuckets(ctx context.Context, cloudCtx core.CloudContext) ([]core.StorageBucket, error)
}

type FirewallProvider interface {
	FetchSecurityGroups(ctx context.Context, cloudCtx core.CloudContext) ([]core.SecurityGroup, error)
	FetchFirewallRules(ctx context.Context, groupID string, cloudCtx core.CloudContext) ([]core.FirewallRule, error)
	ExecuteFirewallAction(ctx context.Context, action string, rule core.FirewallRule, cloudCtx core.CloudContext) (string, error)
}

type NetworkProvider interface {
	FetchNetworks(ctx context.Context, cloudCtx core.CloudContext) ([]core.Network, error)
	FetchSubnets(ctx context.Context, cloudCtx core.CloudContext) ([]core.Subnet, error)
}

type MetricsProvider interface {
	FetchVMMetrics(ctx context.Context, vm core.VM, cloudCtx core.CloudContext, period time.Duration) (*core.VMMetrics, error)
}

type BillingProvider interface {
	FetchAccountCost(ctx context.Context, cloudCtx core.CloudContext) (*core.AccountCost, error)
	FetchResourceCost(ctx context.Context, resourceID string, cloudCtx core.CloudContext) (*core.ResourceCost, error)
	FetchRecommendations(ctx context.Context, cloudCtx core.CloudContext) ([]core.Recommendation, error)
}

// ContextParser loads CloudContexts from local CLI configuration files.
type ContextParser interface {
	LoadContexts() ([]core.CloudContext, []string)
}

// GetProvider returns the appropriate Provider implementation based on config.
func GetProvider(cfg config.AppConfig) Provider {
	if cfg.Backend == "sdk" {
		return &SDKProvider{}
	}
	return &CLIProvider{}
}
