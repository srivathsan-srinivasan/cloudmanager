package gcp

import (
	"context"
	"fmt"
	"strings"

	"cloudmanager/internal/core"
	"cloudmanager/internal/logging"
)

func FetchClustersSDK(ctx context.Context, project string) ([]core.Cluster, error) {
	service, err := newContainerService(ctx)
	if err != nil {
		return nil, err
	}

	// GKE clusters can be regional or zonal. List uses parent format "projects/ID/locations/-"
	parent := fmt.Sprintf("projects/%s/locations/-", project)
	out, err := service.Projects.Locations.Clusters.List(parent).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list gke clusters: %w", err)
	}

	var clusters []core.Cluster
	for _, c := range out.Clusters {
		var labels []string
		for k, v := range c.ResourceLabels {
			labels = append(labels, fmt.Sprintf("%s=%s", k, v))
		}

		clusters = append(clusters, core.Cluster{
			ID:        c.SelfLink,
			Name:      c.Name,
			Location:  c.Location,
			Status:    c.Status,
			Version:   c.CurrentMasterVersion,
			NodeCount: fmt.Sprintf("%d", c.CurrentNodeCount),
			Labels:    strings.Join(labels, ", "),
		})
	}

	return clusters, nil
}

func FetchClustersSDKWithCLIAuthFallback(ctx context.Context, project string) ([]core.Cluster, error) {
	clusters, err := FetchClustersSDK(ctx, project)
	if err != nil {
		logging.Warnf("component=gcp resource=clusters mode=sdk fallback=cli project=%s err=%v", project, err)
		return FetchClustersCLI(project)
	}
	return clusters, nil
}
