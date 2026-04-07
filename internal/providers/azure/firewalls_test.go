package azure

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
)

func TestAzureSecurityRuleToCore(t *testing.T) {
	name := "allow-ssh"
	desc := "ssh access"
	direction := armnetwork.SecurityRuleDirectionInbound
	protocol := armnetwork.SecurityRuleProtocolTCP
	access := armnetwork.SecurityRuleAccessAllow
	priority := int32(100)
	source := "0.0.0.0/0"
	destPort := "22"

	rule := &armnetwork.SecurityRule{
		Name: &name,
		Properties: &armnetwork.SecurityRulePropertiesFormat{
			Description:          &desc,
			Direction:            &direction,
			Protocol:             &protocol,
			Access:               &access,
			Priority:             &priority,
			SourceAddressPrefix:  &source,
			DestinationPortRange: &destPort,
		},
	}

	got := azureSecurityRuleToCore(rule, "nsg-main", "/subscriptions/x/resourceGroups/rg/providers/Microsoft.Network/networkSecurityGroups/nsg-main", false)
	if got.Direction != "Inbound" || got.Protocol != "tcp" || got.PortRange != "22" || got.Action != "Allow" {
		t.Fatalf("unexpected mapped rule: %#v", got)
	}
	if got.Source != "0.0.0.0/0" {
		t.Fatalf("unexpected source: %s", got.Source)
	}
}

func TestAzureRuleOpensPort(t *testing.T) {
	direction := armnetwork.SecurityRuleDirectionInbound
	protocol := armnetwork.SecurityRuleProtocolTCP
	access := armnetwork.SecurityRuleAccessAllow
	source := "0.0.0.0/0"
	port := "3389"

	rule := &armnetwork.SecurityRule{
		Properties: &armnetwork.SecurityRulePropertiesFormat{
			Direction:            &direction,
			Protocol:             &protocol,
			Access:               &access,
			SourceAddressPrefix:  &source,
			DestinationPortRange: &port,
		},
	}

	if !azureRuleOpensPort(rule, 3389) {
		t.Fatal("expected rule to be marked as internet-exposed rdp")
	}
	if azureRuleOpensPort(rule, 22) {
		t.Fatal("did not expect rule to be marked as ssh")
	}
}

func TestAzureResourceGroupFromID(t *testing.T) {
	id := "/subscriptions/sub-1/resourceGroups/rg-main/providers/Microsoft.Network/networkSecurityGroups/nsg-main"
	if got := azureResourceGroupFromID(&id); got != "rg-main" {
		t.Fatalf("unexpected resource group: %s", got)
	}
}

func TestAzureResourceNameFromID(t *testing.T) {
	id := "/subscriptions/sub-1/resourceGroups/rg-main/providers/Microsoft.Network/virtualNetworks/vnet-main/subnets/subnet-a"
	if got := azureResourceNameFromID(&id, "virtualNetworks"); got != "vnet-main" {
		t.Fatalf("unexpected virtual network: %s", got)
	}
	if got := azureResourceNameFromID(&id, "subnets"); got != "subnet-a" {
		t.Fatalf("unexpected subnet: %s", got)
	}
}
