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
	project = strings.TrimSpace(project)
	cmd := exec.Command("gcloud", "compute", "instances", "list", "--project", project, "--format=json", "--quiet")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gcloud compute instances list failed for project %s: %w\n%s", project, err, strings.TrimSpace(string(output)))
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
	project := strings.TrimSpace(cloudCtx.AccountID)
	zone, err := resolveGCPVMZone(ctx, project, vm, func(ctx context.Context, project string) ([]core.VM, error) {
		_ = ctx
		return FetchVMsCLI(project)
	})
	if err != nil {
		return "", err
	}
	switch action {
	case "Start":
		cmd = exec.CommandContext(ctx, "gcloud", "compute", "instances", "start", vm.Name, "--project", project, "--zone", zone, "--quiet")
	case "Stop":
		cmd = exec.CommandContext(ctx, "gcloud", "compute", "instances", "stop", vm.Name, "--project", project, "--zone", zone, "--quiet")
	case "Restart":
		cmd = exec.CommandContext(ctx, "gcloud", "compute", "instances", "reset", vm.Name, "--project", project, "--zone", zone, "--quiet")
	case "Terminate":
		cmd = exec.CommandContext(ctx, "gcloud", "compute", "instances", "delete", vm.Name, "--project", project, "--zone", zone, "--quiet")
	case "Describe":
		cmd = exec.CommandContext(ctx, "gcloud", "compute", "instances", "describe", vm.Name, "--project", project, "--zone", zone, "--quiet")
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
	return exec.CommandContext(ctx, "gcloud", "compute", "ssh", vm.Name, "--project", cloudCtx.AccountID, "--zone", normalizeGCPZone(vm.Zone)), nil
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
	project := strings.TrimSpace(cloudCtx.AccountID)
	zone, err := resolveGCPVMZone(ctx, project, vm, FetchVMsSDKWithCLIAuthFallback)
	if err != nil {
		return "", err
	}
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
		output, marshalErr := json.MarshalIndent(inst, "", "  ")
		if marshalErr != nil {
			return "", marshalErr
		}
		return string(output), nil
	default:
		return "", fmt.Errorf("action %s not supported for GCP SDK", action)
	}
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Successfully %sed %s", strings.ToLower(action), vm.Name), nil
}

func resolveGCPVMZone(ctx context.Context, project string, vm core.VM, fetch func(context.Context, string) ([]core.VM, error)) (string, error) {
	if zone := normalizeGCPZone(vm.Zone); zone != "" {
		return zone, nil
	}
	project = strings.TrimSpace(project)
	if project == "" {
		return "", fmt.Errorf("gcp project is empty")
	}
	name := strings.TrimSpace(vm.Name)
	id := strings.TrimSpace(vm.ID)
	if name == "" && id == "" {
		return "", fmt.Errorf("gcp vm name/id is empty")
	}
	vms, err := fetch(ctx, project)
	if err != nil {
		return "", fmt.Errorf("gcp vm zone is empty and zone discovery failed for project %s: %w", project, err)
	}
	for _, candidate := range vms {
		if (name != "" && strings.EqualFold(strings.TrimSpace(candidate.Name), name)) ||
			(id != "" && strings.EqualFold(strings.TrimSpace(candidate.ID), id)) {
			if zone := normalizeGCPZone(candidate.Zone); zone != "" {
				return zone, nil
			}
		}
	}
	if name != "" {
		return "", fmt.Errorf("gcp vm zone is empty and instance %q was not found in project %s", name, project)
	}
	return "", fmt.Errorf("gcp vm zone is empty and instance id %q was not found in project %s", id, project)
}

func normalizeGCPZone(zone string) string {
	zone = strings.TrimSpace(zone)
	if zone == "" || zone == "-" {
		return ""
	}
	parts := strings.Split(zone, "/")
	return strings.TrimSpace(parts[len(parts)-1])
}

func GetSSHCmdSDK(ctx context.Context, vm core.VM, cloudCtx core.CloudContext) (*exec.Cmd, error) {
	return exec.CommandContext(ctx, "gcloud", "compute", "ssh", vm.Name, "--project", cloudCtx.AccountID, "--zone", normalizeGCPZone(vm.Zone)), nil
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
