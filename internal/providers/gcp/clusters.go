package gcp

import (
	"context"
	"cloudmanager/internal/core"
)

func FetchClustersCLI(project string) ([]core.Cluster, error) {
	return nil, nil
}

func FetchClustersSDK(ctx context.Context, project string) ([]core.Cluster, error) {
	return nil, nil
}

func FetchClustersSDKWithCLIAuthFallback(ctx context.Context, project string) ([]core.Cluster, error) {
	return nil, nil
}
