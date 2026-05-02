package azure

import (
	"cloudmanager/internal/core"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func GetK9sCmd(ctx context.Context, cluster core.Cluster, cloudCtx core.CloudContext) (*exec.Cmd, error) {
	if cluster.Name == "" || cluster.ID == "" {
		return nil, fmt.Errorf("cluster name and ID are required")
	}
	rg := ""
	parts := strings.Split(cluster.ID, "/")
	for i, p := range parts {
		if strings.EqualFold(p, "resourceGroups") && i+1 < len(parts) {
			rg = parts[i+1]
			break
		}
	}
	if rg == "" {
		return nil, fmt.Errorf("could not determine resource group from cluster ID")
	}
	expectedCtx := cluster.Name
	return core.EnsureKubeContextForTarget(
		ctx,
		core.KubeContextTarget{
			ExpectedContext: expectedCtx,
			ClusterName:     cluster.Name,
			ClusterID:       cluster.ID,
			AccountID:       cloudCtx.AccountID,
			Location:        cluster.Location,
		},
		"az",
		"aks",
		"get-credentials",
		"--resource-group",
		rg,
		"--name",
		cluster.Name,
		"--subscription",
		cloudCtx.AccountID,
	)
}
