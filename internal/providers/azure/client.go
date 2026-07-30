package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"

	"github.com/vyoogam/cloudmanager/internal/core"
)

// --- CLI Backend ---

type azureVMOutput struct {
	Name            string `json:"name"`
	Id              string `json:"id"`
	ResourceGroup   string `json:"resourceGroup"`
	Location        string `json:"location"`
	HardwareProfile struct {
		VmSize string `json:"vmSize"`
	} `json:"hardwareProfile"`
	PowerState string            `json:"powerState"`
	PrivateIps string            `json:"privateIps"`
	PublicIps  string            `json:"publicIps"`
	Tags       map[string]string `json:"tags"`
}

func FetchVMsCLI(subscription string) ([]core.VM, error) {
	subscription = strings.TrimSpace(subscription)
	cmd := exec.Command("az", "vm", "list", "-d", "--subscription", subscription, "--output", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("az vm list failed for subscription %s: %w\n%s", subscription, err, strings.TrimSpace(string(output)))
	}
	var data []azureVMOutput
	if err := json.Unmarshal(output, &data); err != nil {
		return nil, fmt.Errorf("json parse error: %w", err)
	}
	var vms []core.VM
	for _, inst := range data {
		vms = append(vms, azureVMOutputToVM(inst))
	}
	return vms, nil
}

func azureVMOutputToVM(inst azureVMOutput) core.VM {
	status := inst.PowerState
	if strings.HasPrefix(status, "VM ") {
		status = strings.TrimPrefix(status, "VM ")
	}
	var lp []string
	for k, v := range inst.Tags {
		lp = append(lp, fmt.Sprintf("%s=%s", k, v))
	}
	return core.VM{
		Name: inst.Name, ID: inst.Id,
		Type: inst.HardwareProfile.VmSize, State: status,
		PrivateIP: orDash(inst.PrivateIps), PublicIP: orDash(inst.PublicIps),
		Zone:          orDash(inst.Location),
		ResourceGroup: inst.ResourceGroup,
		Network:       "-", Subnet: "-", Labels: strings.Join(lp, ", "),
	}
}

func ExecuteActionCLI(ctx context.Context, action string, vm core.VM, cloudCtx core.CloudContext) (string, error) {
	var cmd *exec.Cmd
	switch action {
	case "Start":
		cmd = exec.CommandContext(ctx, "az", "vm", "start", "--name", vm.Name, "--resource-group", vm.ResourceGroup, "--subscription", cloudCtx.AccountID)
	case "Stop":
		cmd = exec.CommandContext(ctx, "az", "vm", "stop", "--name", vm.Name, "--resource-group", vm.ResourceGroup, "--subscription", cloudCtx.AccountID)
	case "Restart":
		cmd = exec.CommandContext(ctx, "az", "vm", "restart", "--name", vm.Name, "--resource-group", vm.ResourceGroup, "--subscription", cloudCtx.AccountID)
	case "Terminate":
		cmd = exec.CommandContext(ctx, "az", "vm", "delete", "--name", vm.Name, "--resource-group", vm.ResourceGroup, "--subscription", cloudCtx.AccountID, "--yes")
	case "Describe":
		cmd = exec.CommandContext(ctx, "az", "vm", "show", "--name", vm.Name, "--resource-group", vm.ResourceGroup, "--subscription", cloudCtx.AccountID)
	default:
		return "", fmt.Errorf("action %s not supported for Azure", action)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to %s: %w\n%s", action, err, string(output))
	}
	if action == "Describe" {
		return string(output), nil
	}
	return fmt.Sprintf("Successfully executed '%s' on %s", action, vm.Name), nil
}

func GetSSHCmdCLI(ctx context.Context, vm core.VM, cloudCtx core.CloudContext) (*exec.Cmd, error) {
	return exec.CommandContext(ctx, "az", "ssh", "vm", "--name", vm.Name, "--resource-group", vm.ResourceGroup, "--subscription", cloudCtx.AccountID), nil
}

// --- SDK Backend ---

func FetchVMsSDK(ctx context.Context, subscriptionID string) ([]core.VM, error) {
	cred, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get azure credentials: %w", err)
	}
	clientFactory, err := armcompute.NewClientFactory(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create azure client factory: %w", err)
	}
	vmClient := clientFactory.NewVirtualMachinesClient()
	nicClient, err := armnetwork.NewInterfacesClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create azure network interfaces client: %w", err)
	}
	publicIPClient, err := armnetwork.NewPublicIPAddressesClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create azure public IP client: %w", err)
	}
	pager := vmClient.NewListAllPager(&armcompute.VirtualMachinesClientListAllOptions{StatusOnly: to.Ptr("true")})
	var vms []core.VM
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get azure vms: %w", err)
		}
		for _, vm := range page.Value {
			name := orPtr(vm.Name)
			id := orPtr(vm.ID)
			rg := "-"
			parts := strings.Split(id, "/")
			for i, p := range parts {
				if strings.ToLower(p) == "resourcegroups" && i+1 < len(parts) {
					rg = parts[i+1]
					break
				}
			}
			vmSize := "-"
			if vm.Properties != nil && vm.Properties.HardwareProfile != nil && vm.Properties.HardwareProfile.VMSize != nil {
				vmSize = string(*vm.Properties.HardwareProfile.VMSize)
			}
			status := "-"
			if vm.Properties != nil && vm.Properties.InstanceView != nil {
				for _, stat := range vm.Properties.InstanceView.Statuses {
					if stat.Code != nil && strings.HasPrefix(*stat.Code, "PowerState/") {
						status = strings.TrimPrefix(*stat.Code, "PowerState/")
						break
					}
				}
			}
			var lp []string
			for k, v := range vm.Tags {
				if v != nil {
					lp = append(lp, fmt.Sprintf("%s=%s", k, *v))
				}
			}
			var networkProfile *armcompute.NetworkProfile
			if vm.Properties != nil {
				networkProfile = vm.Properties.NetworkProfile
			}
			networkDetails, err := fetchAzureVMNetworkDetails(ctx, networkProfile, nicClient, publicIPClient)
			if err != nil {
				networkDetails = azureNetworkDetails{
					privateIP:      "-",
					publicIP:       "-",
					network:        "-",
					subnet:         "-",
					securityGroups: nil,
				}
			}
			vms = append(vms, core.VM{
				Name: name, ID: id, Type: vmSize, State: status,
				PrivateIP:      networkDetails.privateIP,
				PublicIP:       networkDetails.publicIP,
				Zone:           orPtr(vm.Location),
				ResourceGroup:  rg,
				Network:        networkDetails.network,
				Subnet:         networkDetails.subnet,
				Labels:         strings.Join(lp, ", "),
				SecurityGroups: strings.Join(networkDetails.securityGroups, ","),
			})
		}
	}
	return vms, nil
}

func ExecuteActionSDK(ctx context.Context, action string, vm core.VM, cloudCtx core.CloudContext) (string, error) {
	cred, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return "", fmt.Errorf("failed to get azure credentials: %w", err)
	}
	clientFactory, err := armcompute.NewClientFactory(cloudCtx.AccountID, cred, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create azure client factory: %w", err)
	}
	vmClient := clientFactory.NewVirtualMachinesClient()
	rg := vm.ResourceGroup
	switch action {
	case "Start":
		poller, e := vmClient.BeginStart(ctx, rg, vm.Name, nil)
		if e != nil {
			return "", e
		}
		_, err = poller.PollUntilDone(ctx, nil)
	case "Stop":
		poller, e := vmClient.BeginPowerOff(ctx, rg, vm.Name, &armcompute.VirtualMachinesClientBeginPowerOffOptions{SkipShutdown: to.Ptr(false)})
		if e != nil {
			return "", e
		}
		_, err = poller.PollUntilDone(ctx, nil)
	case "Restart":
		poller, e := vmClient.BeginRestart(ctx, rg, vm.Name, nil)
		if e != nil {
			return "", e
		}
		_, err = poller.PollUntilDone(ctx, nil)
	case "Terminate":
		poller, e := vmClient.BeginDelete(ctx, rg, vm.Name, &armcompute.VirtualMachinesClientBeginDeleteOptions{ForceDeletion: to.Ptr(true)})
		if e != nil {
			return "", e
		}
		_, err = poller.PollUntilDone(ctx, nil)
	case "Describe":
		resp, descErr := vmClient.Get(ctx, rg, vm.Name, nil)
		if descErr != nil {
			return "", descErr
		}
		vmSize := "-"
		if resp.Properties != nil && resp.Properties.HardwareProfile != nil && resp.Properties.HardwareProfile.VMSize != nil {
			vmSize = string(*resp.Properties.HardwareProfile.VMSize)
		}
		return fmt.Sprintf("Instance Name: %s\nType: %s\nResource Group: %s\nLocation: %s\n",
			*resp.Name, vmSize, rg, *resp.Location), nil
	default:
		return "", fmt.Errorf("action %s not supported for Azure SDK", action)
	}
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Successfully %sed %s", strings.ToLower(action), vm.Name), nil
}

func GetSSHCmdSDK(ctx context.Context, vm core.VM, cloudCtx core.CloudContext) (*exec.Cmd, error) {
	return exec.CommandContext(ctx, "az", "ssh", "vm", "--name", vm.Name, "--resource-group", vm.ResourceGroup, "--subscription", cloudCtx.AccountID), nil
}

func getAzureCreds() (*azidentity.AzureCLICredential, error) {
	return azidentity.NewAzureCLICredential(nil)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func orPtr(s *string) string {
	if s == nil {
		return "-"
	}
	return *s
}

func azureResourceGroupFromID(id *string) string {
	return azureResourceNameFromID(id, "resourceGroups")
}

func azureResourceNameFromID(id *string, segment string) string {
	if id == nil {
		return "-"
	}
	parts := strings.Split(*id, "/")
	for i, p := range parts {
		if strings.EqualFold(p, segment) && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return "-"
}

type azureNICGetter interface {
	Get(ctx context.Context, resourceGroupName, networkInterfaceName string, options *armnetwork.InterfacesClientGetOptions) (armnetwork.InterfacesClientGetResponse, error)
}

type azurePublicIPGetter interface {
	Get(ctx context.Context, resourceGroupName, publicIPAddressName string, options *armnetwork.PublicIPAddressesClientGetOptions) (armnetwork.PublicIPAddressesClientGetResponse, error)
}

type azureNetworkDetails struct {
	privateIP      string
	publicIP       string
	network        string
	subnet         string
	securityGroups []string
}

func fetchAzureVMNetworkDetails(ctx context.Context, profile *armcompute.NetworkProfile, nicClient azureNICGetter, publicIPClient azurePublicIPGetter) (azureNetworkDetails, error) {
	details := azureNetworkDetails{
		privateIP: "-",
		publicIP:  "-",
		network:   "-",
		subnet:    "-",
	}
	if profile == nil || len(profile.NetworkInterfaces) == 0 || nicClient == nil {
		return details, nil
	}

	var firstErr error
	successfulFetch := false
	for _, nicRef := range orderedAzureNetworkInterfaceRefs(profile.NetworkInterfaces) {
		if nicRef == nil {
			continue
		}
		resourceGroup := azureResourceGroupFromID(nicRef.ID)
		nicName := azureResourceNameFromID(nicRef.ID, "networkInterfaces")
		if resourceGroup == "-" || nicName == "-" {
			continue
		}

		resp, err := nicClient.Get(ctx, resourceGroup, nicName, nil)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		successfulFetch = true
		mergeAzureNetworkDetails(ctx, &details, &resp.Interface, publicIPClient)
	}

	if !successfulFetch && firstErr != nil {
		return details, firstErr
	}
	return details, nil
}

func mergeAzureNetworkDetails(ctx context.Context, details *azureNetworkDetails, nic *armnetwork.Interface, publicIPClient azurePublicIPGetter) {
	if details == nil || nic == nil || nic.Properties == nil {
		return
	}

	if nic.Properties.NetworkSecurityGroup != nil {
		addUniqueAzureValue(&details.securityGroups, azureResourceNameFromID(nic.Properties.NetworkSecurityGroup.ID, "networkSecurityGroups"))
	}

	for _, ipConfig := range orderedAzureInterfaceIPConfigs(nic.Properties.IPConfigurations) {
		if ipConfig == nil || ipConfig.Properties == nil {
			continue
		}

		props := ipConfig.Properties
		if details.privateIP == "-" {
			details.privateIP = orDashPtr(props.PrivateIPAddress)
		}
		if details.subnet == "-" && props.Subnet != nil {
			details.subnet = azureResourceNameFromID(props.Subnet.ID, "subnets")
		}
		if details.network == "-" && props.Subnet != nil {
			details.network = azureResourceNameFromID(props.Subnet.ID, "virtualNetworks")
		}
		if details.publicIP == "-" {
			details.publicIP = azurePublicIPAddress(ctx, publicIPClient, props.PublicIPAddress)
		}
	}
}

func azurePublicIPAddress(ctx context.Context, publicIPClient azurePublicIPGetter, publicIP *armnetwork.PublicIPAddress) string {
	if publicIP == nil {
		return "-"
	}
	if publicIP.Properties != nil && publicIP.Properties.IPAddress != nil && strings.TrimSpace(*publicIP.Properties.IPAddress) != "" {
		return *publicIP.Properties.IPAddress
	}
	if publicIPClient == nil {
		return "-"
	}

	resourceGroup := azureResourceGroupFromID(publicIP.ID)
	addressName := azureResourceNameFromID(publicIP.ID, "publicIPAddresses")
	if resourceGroup == "-" || addressName == "-" {
		return "-"
	}

	resp, err := publicIPClient.Get(ctx, resourceGroup, addressName, nil)
	if err != nil || resp.Properties == nil || resp.Properties.IPAddress == nil || strings.TrimSpace(*resp.Properties.IPAddress) == "" {
		return "-"
	}
	return *resp.Properties.IPAddress
}

func orderedAzureNetworkInterfaceRefs(refs []*armcompute.NetworkInterfaceReference) []*armcompute.NetworkInterfaceReference {
	if len(refs) <= 1 {
		return refs
	}
	ordered := make([]*armcompute.NetworkInterfaceReference, 0, len(refs))
	for _, ref := range refs {
		if ref != nil && ref.Properties != nil && ref.Properties.Primary != nil && *ref.Properties.Primary {
			ordered = append(ordered, ref)
		}
	}
	for _, ref := range refs {
		if ref == nil || (ref.Properties != nil && ref.Properties.Primary != nil && *ref.Properties.Primary) {
			continue
		}
		ordered = append(ordered, ref)
	}
	return ordered
}

func orderedAzureInterfaceIPConfigs(configs []*armnetwork.InterfaceIPConfiguration) []*armnetwork.InterfaceIPConfiguration {
	if len(configs) <= 1 {
		return configs
	}
	ordered := make([]*armnetwork.InterfaceIPConfiguration, 0, len(configs))
	for _, cfg := range configs {
		if cfg != nil && cfg.Properties != nil && cfg.Properties.Primary != nil && *cfg.Properties.Primary {
			ordered = append(ordered, cfg)
		}
	}
	for _, cfg := range configs {
		if cfg == nil || (cfg.Properties != nil && cfg.Properties.Primary != nil && *cfg.Properties.Primary) {
			continue
		}
		ordered = append(ordered, cfg)
	}
	return ordered
}

func addUniqueAzureValue(values *[]string, value string) {
	if values == nil || value == "" || value == "-" {
		return
	}
	for _, existing := range *values {
		if existing == value {
			return
		}
	}
	*values = append(*values, value)
}

// --- Port Forwarding & SCP ---

// GetPortForwardCmdCLI returns an Azure CLI command for port forwarding using `az ssh vm` with -L flags.
func GetPortForwardCmdCLI(ctx context.Context, vm core.VM, cloudCtx core.CloudContext, specs []core.PortForwardSpec) (*exec.Cmd, error) {
	if len(specs) == 0 {
		return nil, fmt.Errorf("no port forward specs provided")
	}
	args := []string{"ssh", "vm", "--name", vm.Name, "--resource-group", vm.ResourceGroup, "--subscription", cloudCtx.AccountID}
	for _, s := range specs {
		localHost := s.LocalHost
		if localHost == "" {
			localHost = "localhost"
		}
		remoteHost := s.RemoteHost
		if remoteHost == "" {
			remoteHost = "localhost"
		}
		args = append(args, "-L", fmt.Sprintf("%s:%d:%s:%d", localHost, s.LocalPort, remoteHost, s.RemotePort))
	}
	return exec.CommandContext(ctx, "az", args...), nil
}

// GetPortForwardCmdSDK delegates to CLI for port forwarding.
func GetPortForwardCmdSDK(ctx context.Context, vm core.VM, cloudCtx core.CloudContext, specs []core.PortForwardSpec) (*exec.Cmd, error) {
	return GetPortForwardCmdCLI(ctx, vm, cloudCtx, specs)
}

// GetSCPCmdCLI returns an Azure CLI command for SCP file transfer.
// Uses `az ssh vm` with scp subsystem or falls back to manual scp command construction.
func GetSCPCmdCLI(ctx context.Context, vm core.VM, cloudCtx core.CloudContext, transfer core.SCPTransfer) (*exec.Cmd, error) {
	// az ssh vm supports file transfer via the `--` separator to pass ssh/scp flags
	// For scp, we construct the command to run via the SSH connection
	var scpArgs []string
	if transfer.Direction == "pull" {
		// remote -> local
		scpArgs = []string{"scp"}
		if transfer.Recursive {
			scpArgs = append(scpArgs, "-r")
		}
		scpArgs = append(scpArgs, fmt.Sprintf("%s:%s", vm.Name, transfer.Source), transfer.Destination)
	} else {
		// local -> remote
		scpArgs = []string{"scp"}
		if transfer.Recursive {
			scpArgs = append(scpArgs, "-r")
		}
		scpArgs = append(scpArgs, transfer.Source, fmt.Sprintf("%s:%s", vm.Name, transfer.Destination))
	}
	
	args := []string{"ssh", "vm", "--name", vm.Name, "--resource-group", vm.ResourceGroup, "--subscription", cloudCtx.AccountID, "--"}
	args = append(args, scpArgs...)
	return exec.CommandContext(ctx, "az", args...), nil
}

// GetSCPCmdSDK delegates to CLI for SCP.
func GetSCPCmdSDK(ctx context.Context, vm core.VM, cloudCtx core.CloudContext, transfer core.SCPTransfer) (*exec.Cmd, error) {
	return GetSCPCmdCLI(ctx, vm, cloudCtx, transfer)
}
