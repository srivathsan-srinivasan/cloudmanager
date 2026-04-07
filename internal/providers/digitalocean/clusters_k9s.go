package digitalocean

import (
	"context"
	"fmt"
	"os/exec"
	"cloudmanager/internal/core"
	
)

func GetK9sCmd(ctx context.Context, cluster core.Cluster, cloudCtx core.CloudContext) (*exec.Cmd, error) {
	if cluster.ID == "" {
		return nil, fmt.Errorf("cluster ID is required")
	}
	expectedCtx := fmt.Sprintf("do-%s-%s", cloudCtx.Region, cluster.Name)
	fetchCmd := fmt.Sprintf("doctl kubernetes cluster kubeconfig save %s && k9s", cluster.ID)
	return core.EnsureKubeContext(ctx, expectedCtx, fetchCmd)
}
