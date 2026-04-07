package aws

import (
	"context"
	"fmt"
	"os/exec"
	"cloudmanager/internal/core"
	
)

func GetK9sCmd(ctx context.Context, cluster core.Cluster, cloudCtx core.CloudContext) (*exec.Cmd, error) {
	if cluster.Name == "" || cloudCtx.Region == "" {
		return nil, fmt.Errorf("cluster name and region are required")
	}
	expectedCtx := fmt.Sprintf("arn:aws:eks:%s:%s:cluster/%s", cloudCtx.Region, cloudCtx.AccountID, cluster.Name)
	fetchCmd := fmt.Sprintf("aws eks update-kubeconfig --name %s --region %s", cluster.Name, cloudCtx.Region)
	if cloudCtx.CredentialProfile != "" {
		fetchCmd += fmt.Sprintf(" --profile %s", cloudCtx.CredentialProfile)
	}
	fetchCmd += " && k9s"
	return core.EnsureKubeContext(ctx, expectedCtx, fetchCmd)
}
