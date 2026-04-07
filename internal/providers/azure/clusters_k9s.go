package azure

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"cloudmanager/internal/core"
	
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
	fetchCmd := fmt.Sprintf("az aks get-credentials --resource-group %s --name %s --subscription %s && k9s", rg, cluster.Name, cloudCtx.AccountID)
	return core.EnsureKubeContext(ctx, expectedCtx, fetchCmd)
}
