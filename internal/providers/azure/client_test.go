package azure

import (
	"context"
	"reflect"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"

	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
)

func TestGetSSHCmdCLIUsesSubscriptionAndResourceGroup(t *testing.T) {
	cmd, err := GetSSHCmdCLI(context.Background(), core.VM{Name: "vm-1", ResourceGroup: "rg-main"}, core.CloudContext{
		Provider:  "Azure",
		AccountID: "sub-123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"az", "ssh", "vm", "--name", "vm-1", "--resource-group", "rg-main", "--subscription", "sub-123"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("unexpected ssh command args:\nwant %#v\ngot  %#v", want, cmd.Args)
	}
}

func TestFetchAzureVMNetworkDetailsResolvesNICMetadata(t *testing.T) {
	primary := true
	nicID := "/subscriptions/sub-123/resourceGroups/rg-main/providers/Microsoft.Network/networkInterfaces/nic-main"
	nsgID := "/subscriptions/sub-123/resourceGroups/rg-main/providers/Microsoft.Network/networkSecurityGroups/nsg-app"
	subnetID := "/subscriptions/sub-123/resourceGroups/rg-main/providers/Microsoft.Network/virtualNetworks/vnet-main/subnets/subnet-app"
	publicIPID := "/subscriptions/sub-123/resourceGroups/rg-main/providers/Microsoft.Network/publicIPAddresses/pip-main"
	privateIP := "10.0.0.4"
	publicIP := "52.1.2.3"

	profile := &armcompute.NetworkProfile{
		NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
			{
				ID: &nicID,
				Properties: &armcompute.NetworkInterfaceReferenceProperties{
					Primary: &primary,
				},
			},
		},
	}

	nicClient := fakeAzureNICGetter{
		response: armnetwork.InterfacesClientGetResponse{
			Interface: armnetwork.Interface{
				Properties: &armnetwork.InterfacePropertiesFormat{
					NetworkSecurityGroup: &armnetwork.SecurityGroup{
						ID: &nsgID,
					},
					IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
						{
							Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
								Primary:          &primary,
								PrivateIPAddress: &privateIP,
								Subnet: &armnetwork.Subnet{
									ID: &subnetID,
								},
								PublicIPAddress: &armnetwork.PublicIPAddress{
									ID: &publicIPID,
								},
							},
						},
					},
				},
			},
		},
	}
	publicIPClient := fakeAzurePublicIPGetter{
		response: armnetwork.PublicIPAddressesClientGetResponse{
			PublicIPAddress: armnetwork.PublicIPAddress{
				Properties: &armnetwork.PublicIPAddressPropertiesFormat{
					IPAddress: &publicIP,
				},
			},
		},
	}

	got, err := fetchAzureVMNetworkDetails(context.Background(), profile, nicClient, publicIPClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.privateIP != privateIP || got.publicIP != publicIP {
		t.Fatalf("unexpected IP data: %#v", got)
	}
	if got.network != "vnet-main" || got.subnet != "subnet-app" {
		t.Fatalf("unexpected network data: %#v", got)
	}
	if !reflect.DeepEqual(got.securityGroups, []string{"nsg-app"}) {
		t.Fatalf("unexpected security groups: %#v", got.securityGroups)
	}
}

type fakeAzureNICGetter struct {
	response armnetwork.InterfacesClientGetResponse
	err      error
}

func (f fakeAzureNICGetter) Get(context.Context, string, string, *armnetwork.InterfacesClientGetOptions) (armnetwork.InterfacesClientGetResponse, error) {
	return f.response, f.err
}

type fakeAzurePublicIPGetter struct {
	response armnetwork.PublicIPAddressesClientGetResponse
	err      error
}

func (f fakeAzurePublicIPGetter) Get(context.Context, string, string, *armnetwork.PublicIPAddressesClientGetOptions) (armnetwork.PublicIPAddressesClientGetResponse, error) {
	return f.response, f.err
}
