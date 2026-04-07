package gcp

import (
	"context"
	"fmt"
	"os/exec"
	"cloudmanager/internal/core"
	
)

func GetK9sCmd(ctx context.Context, cluster core.Cluster, cloudCtx core.CloudContext) (*exec.Cmd, error) {
	if cluster.Name == "" || cluster.Location == "" {
		return nil, fmt.Errorf("cluster name and location are required")
	}
	expectedCtx := fmt.Sprintf("gke_%s_%s_%s", cloudCtx.AccountID, cluster.Location, cluster.Name)
	fetchCmd := fmt.Sprintf("gcloud container clusters get-credentials %s --region %s --project %s && k9s", cluster.Name, cluster.Location, cloudCtx.AccountID)
	return core.EnsureKubeContext(ctx, expectedCtx, fetchCmd)
}
