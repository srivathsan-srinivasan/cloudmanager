package gcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
)

// FetchNetworksSDK fetches VPCs from GCP.
func FetchNetworksSDK(ctx context.Context, projectID string) ([]core.Network, error) {
	service, err := newComputeService(ctx)
	if err != nil {
		return nil, err
	}

	out, err := service.Networks.List(projectID).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}

	var networks []core.Network
	for _, n := range out.Items {
		networks = append(networks, core.Network{
			ID:          fmt.Sprintf("%d", n.Id),
			Name:        n.Name,
			State:       "READY",     // GCP networks don't really have a status like EC2
			CIDRBlock:   "auto-mode", // GCP VPCs don't have a single CIDR
			SubnetCount: len(n.Subnetworks),
			Provider:    "GCP",
			Region:      "global",
		})
	}

	return networks, nil
}

// FetchSubnetsSDK fetches Subnets from GCP using AggregatedList.
func FetchSubnetsSDK(ctx context.Context, projectID string) ([]core.Subnet, error) {
	service, err := newComputeService(ctx)
	if err != nil {
		return nil, err
	}

	out, err := service.Subnetworks.AggregatedList(projectID).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list subnets: %w", err)
	}

	var subnets []core.Subnet
	for _, scope := range out.Items {
		for _, s := range scope.Subnetworks {
			region := ""
			parts := strings.Split(s.Region, "/")
			if len(parts) > 0 {
				region = parts[len(parts)-1]
			}

			networkName := ""
			nParts := strings.Split(s.Network, "/")
			if len(nParts) > 0 {
				networkName = nParts[len(nParts)-1]
			}

			subnets = append(subnets, core.Subnet{
				ID:           fmt.Sprintf("%d", s.Id),
				Name:         s.Name,
				State:        "READY",
				CIDRBlock:    s.IpCidrRange,
				NetworkID:    networkName, // ID is often same as name in GCP URLs
				NetworkName:  networkName,
				AvailableIPs: 0, // GCP doesn't easily expose this in standard list
				Provider:     "GCP",
				Region:       region,
			})
		}
	}

	return subnets, nil
}
