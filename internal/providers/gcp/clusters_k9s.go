package gcp

import (
	"context"
	"fmt"
	"github.com/vyoogam/cloudmanager/internal/core"
	"os/exec"
)

func GetK9sCmd(ctx context.Context, cluster core.Cluster, cloudCtx core.CloudContext) (*exec.Cmd, error) {
	if cluster.Name == "" || cluster.Location == "" {
		return nil, fmt.Errorf("cluster name and location are required")
	}
	expectedCtx := fmt.Sprintf("gke_%s_%s_%s", cloudCtx.AccountID, cluster.Location, cluster.Name)
	return core.EnsureKubeContextForTarget(
		ctx,
		core.KubeContextTarget{
			ExpectedContext: expectedCtx,
			ClusterName:     cluster.Name,
			ClusterID:       cluster.ID,
			AccountID:       cloudCtx.AccountID,
			Location:        cluster.Location,
		},
		"gcloud",
		"container",
		"clusters",
		"get-credentials",
		cluster.Name,
		"--region",
		cluster.Location,
		"--project",
		cloudCtx.AccountID,
	)
}
