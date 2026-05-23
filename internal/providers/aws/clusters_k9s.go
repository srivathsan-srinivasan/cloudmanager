package aws

import (
	"context"
	"fmt"
	"github.com/vyoogam/cloudmanager/internal/core"
	"os/exec"
)

func GetK9sCmd(ctx context.Context, cluster core.Cluster, cloudCtx core.CloudContext) (*exec.Cmd, error) {
	if cluster.Name == "" || cloudCtx.Region == "" {
		return nil, fmt.Errorf("cluster name and region are required")
	}
	expectedCtx := fmt.Sprintf("arn:aws:eks:%s:%s:cluster/%s", cloudCtx.Region, cloudCtx.AccountID, cluster.Name)
	fetchArgs := []string{"eks", "update-kubeconfig", "--name", cluster.Name, "--region", cloudCtx.Region}
	if cloudCtx.CredentialProfile != "" {
		fetchArgs = append(fetchArgs, "--profile", cloudCtx.CredentialProfile)
	}
	return core.EnsureKubeContextForTarget(ctx, core.KubeContextTarget{
		ExpectedContext: expectedCtx,
		ClusterName:     cluster.Name,
		ClusterID:       cluster.ID,
		AccountID:       cloudCtx.AccountID,
		Location:        cloudCtx.Region,
	}, "aws", fetchArgs...)
}
