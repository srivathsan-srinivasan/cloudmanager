package azure

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"

	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
)

// FetchNetworksSDK fetches VNets from Azure.
func FetchNetworksSDK(ctx context.Context, subscriptionID string) ([]core.Network, error) {
	cred, err := getAzureCreds()
	if err != nil {
		return nil, err
	}
	clientFactory, err := armnetwork.NewClientFactory(subscriptionID, cred, nil)
	if err != nil {
		return nil, err
	}
	client := clientFactory.NewVirtualNetworksClient()

	pager := client.NewListAllPager(nil)
	var networks []core.Network
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, v := range page.Value {
			cidr := "-"
			if v.Properties != nil && v.Properties.AddressSpace != nil && len(v.Properties.AddressSpace.AddressPrefixes) > 0 {
				cidr = *v.Properties.AddressSpace.AddressPrefixes[0]
			}

			subnetCount := 0
			if v.Properties != nil {
				subnetCount = len(v.Properties.Subnets)
			}

			networks = append(networks, core.Network{
				ID:            orPtr(v.ID),
				Name:          orPtr(v.Name),
				State:         string(orPtr((*string)(v.Properties.ProvisioningState))),
				CIDRBlock:     cidr,
				SubnetCount:   subnetCount,
				Provider:      "Azure",
				Region:        orPtr(v.Location),
				ResourceGroup: azureResourceGroupFromID(v.ID),
			})
		}
	}
	return networks, nil
}

// FetchSubnetsSDK fetches Subnets from Azure by iterating all VNets.
func FetchSubnetsSDK(ctx context.Context, subscriptionID string) ([]core.Subnet, error) {
	cred, err := getAzureCreds()
	if err != nil {
		return nil, err
	}
	clientFactory, err := armnetwork.NewClientFactory(subscriptionID, cred, nil)
	if err != nil {
		return nil, err
	}
	client := clientFactory.NewVirtualNetworksClient()

	pager := client.NewListAllPager(nil)
	var subnets []core.Subnet
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, v := range page.Value {
			vnetName := orPtr(v.Name)
			vnetID := orPtr(v.ID)
			if v.Properties != nil {
				for _, s := range v.Properties.Subnets {
					subnets = append(subnets, core.Subnet{
						ID:          orPtr(s.ID),
						Name:        orPtr(s.Name),
						State:       string(orPtr((*string)(s.Properties.ProvisioningState))),
						CIDRBlock:   orPtr(s.Properties.AddressPrefix),
						NetworkID:   vnetID,
						NetworkName: vnetName,
						Provider:    "Azure",
						Region:      orPtr(v.Location),
					})
				}
			}
		}
	}
	return subnets, nil
}
