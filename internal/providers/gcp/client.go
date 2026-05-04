package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"google.golang.org/api/compute/v1"

	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/logging"
)

// --- CLI Backend ---

type gcpInstance struct {
	Name              string `json:"name"`
	Id                string `json:"id"`
	MachineType       string `json:"machineType"`
	Status            string `json:"status"`
	NetworkInterfaces []struct {
		Network       string `json:"network"`
		Subnetwork    string `json:"subnetwork"`
		NetworkIP     string `json:"networkIP"`
		AccessConfigs []struct {
			NatIP string `json:"natIP"`
		} `json:"accessConfigs"`
	} `json:"networkInterfaces"`
	Zone   string            `json:"zone"`
	Labels map[string]string `json:"labels"`
}

func FetchVMsCLI(project string) ([]core.VM, error) {
	cmd := exec.Command("gcloud", "compute", "instances", "list", "--project", project, "--format=json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gcloud cli error: %w", err)
	}
	var data []gcpInstance
	if err := json.Unmarshal(output, &data); err != nil {
		return nil, fmt.Errorf("json parse error: %w", err)
	}
	var vms []core.VM
	for _, inst := range data {
		vms = append(vms, gcpInstanceToVM(inst))
	}
	return vms, nil
}

func ExecuteActionCLI(ctx context.Context, action string, vm core.VM, cloudCtx core.CloudContext) (string, error) {
	var cmd *exec.Cmd
	switch action {
	case "Start":
		cmd = exec.CommandContext(ctx, "gcloud", "compute", "instances", "start", vm.Name, "--project", cloudCtx.AccountID, "--zone", vm.Zone)
	case "Stop":
		cmd = exec.CommandContext(ctx, "gcloud", "compute", "instances", "stop", vm.Name, "--project", cloudCtx.AccountID, "--zone", vm.Zone)
	case "Restart":
		cmd = exec.CommandContext(ctx, "gcloud", "compute", "instances", "reset", vm.Name, "--project", cloudCtx.AccountID, "--zone", vm.Zone)
	case "Terminate":
		cmd = exec.CommandContext(ctx, "gcloud", "compute", "instances", "delete", vm.Name, "--project", cloudCtx.AccountID, "--zone", vm.Zone, "--quiet")
	case "Describe":
		cmd = exec.CommandContext(ctx, "gcloud", "compute", "instances", "describe", vm.Name, "--project", cloudCtx.AccountID, "--zone", vm.Zone)
	default:
		return "", fmt.Errorf("action %s not supported for GCP", action)
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
	return exec.CommandContext(ctx, "gcloud", "compute", "ssh", vm.Name, "--project", cloudCtx.AccountID, "--zone", vm.Zone), nil
}

// --- SDK Backend ---

func FetchVMsSDK(ctx context.Context, project string) ([]core.VM, error) {
	service, err := newComputeService(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create gcp compute service: %w", err)
	}
	req := service.Instances.AggregatedList(project)
	var vms []core.VM
	if err := req.Pages(ctx, func(page *compute.InstanceAggregatedList) error {
		for zoneUrl, scopedList := range page.Items {
			for _, inst := range scopedList.Instances {
				zoneParts := strings.Split(zoneUrl, "/")
				zone := zoneParts[len(zoneParts)-1]
				machineParts := strings.Split(inst.MachineType, "/")
				mType := machineParts[len(machineParts)-1]
				privIP, pubIP, network, subnet := "-", "-", "-", "-"
				if len(inst.NetworkInterfaces) > 0 {
					privIP = inst.NetworkInterfaces[0].NetworkIP
					if len(inst.NetworkInterfaces[0].AccessConfigs) > 0 {
						pubIP = inst.NetworkInterfaces[0].AccessConfigs[0].NatIP
					}
					nParts := strings.Split(inst.NetworkInterfaces[0].Network, "/")
					network = nParts[len(nParts)-1]
					sParts := strings.Split(inst.NetworkInterfaces[0].Subnetwork, "/")
					subnet = sParts[len(sParts)-1]
				}
				var lp []string
				for k, v := range inst.Labels {
					lp = append(lp, fmt.Sprintf("%s=%s", k, v))
				}
				vms = append(vms, core.VM{
					Name: inst.Name, ID: fmt.Sprintf("%d", inst.Id),
					Type: mType, State: inst.Status,
					PrivateIP: privIP, PublicIP: pubIP, Zone: zone,
					Network: network, Subnet: subnet, Labels: strings.Join(lp, ", "), SecurityGroups: network,
				})
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to fetch gcp instances: %w", err)
	}
	return vms, nil
}

func ExecuteActionSDK(ctx context.Context, action string, vm core.VM, cloudCtx core.CloudContext) (string, error) {
	service, err := newComputeService(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to create gcp compute service: %w", err)
	}
	project, zone := cloudCtx.AccountID, vm.Zone
	switch action {
	case "Start":
		_, err = service.Instances.Start(project, zone, vm.Name).Context(ctx).Do()
	case "Stop":
		_, err = service.Instances.Stop(project, zone, vm.Name).Context(ctx).Do()
	case "Restart":
		_, err = service.Instances.Reset(project, zone, vm.Name).Context(ctx).Do()
	case "Terminate":
		_, err = service.Instances.Delete(project, zone, vm.Name).Context(ctx).Do()
	case "Describe":
		inst, descErr := service.Instances.Get(project, zone, vm.Name).Context(ctx).Do()
		if descErr != nil {
			return "", descErr
		}
		machineParts := strings.Split(inst.MachineType, "/")
		return fmt.Sprintf("Instance Name: %s\nID: %d\nType: %s\nStatus: %s\nZone: %s\n",
			inst.Name, inst.Id, machineParts[len(machineParts)-1], inst.Status, inst.Zone), nil
	default:
		return "", fmt.Errorf("action %s not supported for GCP SDK", action)
	}
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Successfully %sed %s", strings.ToLower(action), vm.Name), nil
}

func GetSSHCmdSDK(ctx context.Context, vm core.VM, cloudCtx core.CloudContext) (*exec.Cmd, error) {
	return exec.CommandContext(ctx, "gcloud", "compute", "ssh", vm.Name, "--project", cloudCtx.AccountID, "--zone", vm.Zone), nil
}

// --- Helpers ---

func gcpInstanceToVM(inst gcpInstance) core.VM {
	machineParts := strings.Split(inst.MachineType, "/")
	mType := machineParts[len(machineParts)-1]
	zoneParts := strings.Split(inst.Zone, "/")
	zone := zoneParts[len(zoneParts)-1]
	privIP, pubIP, network, subnet := "-", "-", "-", "-"
	if len(inst.NetworkInterfaces) > 0 {
		privIP = inst.NetworkInterfaces[0].NetworkIP
		if len(inst.NetworkInterfaces[0].AccessConfigs) > 0 {
			pubIP = inst.NetworkInterfaces[0].AccessConfigs[0].NatIP
		}
		nParts := strings.Split(inst.NetworkInterfaces[0].Network, "/")
		if len(nParts) > 0 {
			network = nParts[len(nParts)-1]
		}
		sParts := strings.Split(inst.NetworkInterfaces[0].Subnetwork, "/")
		if len(sParts) > 0 {
			subnet = sParts[len(sParts)-1]
		}
	}
	var labelPairs []string
	for k, v := range inst.Labels {
		labelPairs = append(labelPairs, fmt.Sprintf("%s=%s", k, v))
	}
	return core.VM{
		Name: inst.Name, ID: inst.Id, Type: mType, State: inst.Status,
		PrivateIP: privIP, PublicIP: pubIP, Zone: zone,
		Network: network, Subnet: subnet, Labels: strings.Join(labelPairs, ", "), SecurityGroups: network,
	}
}

func FetchVMsSDKWithCLIAuthFallback(ctx context.Context, project string) ([]core.VM, error) {
	vms, err := FetchVMsSDK(ctx, project)
	if err != nil {
		logging.Warnf("component=gcp resource=vms mode=sdk fallback=cli project=%s err=%v", project, err)
		return FetchVMsCLI(project)
	}
	return vms, nil
}
