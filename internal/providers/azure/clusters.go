package azure

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"

	"github.com/vyoogam/cloudmanager/internal/core"
)

func FetchClustersCLI(subscription string) ([]core.Cluster, error) {
	return nil, nil
}

func FetchClustersSDK(ctx context.Context, subscription string) ([]core.Cluster, error) {
	cred, err := getAzureCreds()
	if err != nil {
		return nil, err
	}
	client, err := armcontainerservice.NewManagedClustersClient(subscription, cred, nil)
	if err != nil {
		return nil, err
	}

	pager := client.NewListPager(nil)
	var clusters []core.Cluster

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list aks clusters: %w", err)
		}
		for _, c := range page.Value {
			version := "-"
			if c.Properties != nil && c.Properties.KubernetesVersion != nil {
				version = *c.Properties.KubernetesVersion
			}
			status := "-"
			if c.Properties != nil && c.Properties.ProvisioningState != nil {
				status = *c.Properties.ProvisioningState
			}

			var labels []string
			if c.Tags != nil {
				for k, v := range c.Tags {
					labels = append(labels, fmt.Sprintf("%s=%s", k, *v))
				}
			}

			clusters = append(clusters, core.Cluster{
				ID:       orPtr(c.ID),
				Name:     orPtr(c.Name),
				Location: orPtr(c.Location),
				Status:   status,
				Version:  version,
				Labels:   strings.Join(labels, ", "),
			})
		}
	}

	return clusters, nil
}
