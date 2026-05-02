package digitalocean

import (
	"cloudmanager/internal/core"
	"context"
	"fmt"
	"os/exec"
)

func GetK9sCmd(ctx context.Context, cluster core.Cluster, cloudCtx core.CloudContext) (*exec.Cmd, error) {
	if cluster.ID == "" {
		return nil, fmt.Errorf("cluster ID is required")
	}
	expectedCtx := fmt.Sprintf("do-%s-%s", cloudCtx.Region, cluster.Name)
	return core.EnsureKubeContextForTarget(ctx, core.KubeContextTarget{
		ExpectedContext: expectedCtx,
		ClusterName:     cluster.Name,
		ClusterID:       cluster.ID,
		AccountID:       cloudCtx.AccountID,
		Location:        cloudCtx.Region,
	}, "doctl", "kubernetes", "cluster", "kubeconfig", "save", cluster.ID)
}
