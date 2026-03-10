package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// VM struct representing the common denominator for the UI
type VM struct {
	Name          string
	ID            string
	Type          string
	State         string
	PrivateIP     string
	PublicIP      string
	Zone          string // Useful for GCP
	ResourceGroup string // Useful for Azure
	Network       string
	Subnet        string
	Labels        string
}

// AWS JSON Output Structures
type awsDescribeInstancesOutput struct {
	Reservations []struct {
		Instances []struct {
			InstanceId   string `json:"InstanceId"`
			InstanceType string `json:"InstanceType"`
			VpcId        string `json:"VpcId"`
			SubnetId     string `json:"SubnetId"`
			State        struct {
				Name string `json:"Name"`
			} `json:"State"`
			PrivateIpAddress string `json:"PrivateIpAddress"`
			PublicIpAddress  string `json:"PublicIpAddress"`
			Tags             []struct {
				Key   string `json:"Key"`
				Value string `json:"Value"`
			} `json:"Tags"`
		} `json:"Instances"`
	} `json:"Reservations"`
}

func fetchAWSVMs(profile, region string) ([]VM, error) {
	cmd := exec.Command("aws", "ec2", "describe-instances", "--profile", profile, "--region", region, "--output", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("aws cli error: %w", err)
	}

	var data awsDescribeInstancesOutput
	if err := json.Unmarshal(output, &data); err != nil {
		return nil, fmt.Errorf("json parse error: %w", err)
	}

	var vms []VM
	for _, res := range data.Reservations {
		for _, inst := range res.Instances {
			name := "-"
			var labelPairs []string
			for _, tag := range inst.Tags {
				if tag.Key == "Name" {
					name = tag.Value
				}
				labelPairs = append(labelPairs, fmt.Sprintf("%s=%s", tag.Key, tag.Value))
			}
			labelsStr := strings.Join(labelPairs, ", ")

			privIP := inst.PrivateIpAddress
			if privIP == "" {
				privIP = "-"
			}
			pubIP := inst.PublicIpAddress
			if pubIP == "" {
				pubIP = "-"
			}
			vpc := inst.VpcId
			if vpc == "" {
				vpc = "-"
			}
			subnet := inst.SubnetId
			if subnet == "" {
				subnet = "-"
			}

			vms = append(vms, VM{
				Name:      name,
				ID:        inst.InstanceId,
				Type:      inst.InstanceType,
				State:     inst.State.Name,
				PrivateIP: privIP,
				PublicIP:  pubIP,
				Zone:      region,
				Network:   vpc,
				Subnet:    subnet,
				Labels:    labelsStr,
			})
		}
	}
	return vms, nil
}

// GCP JSON Output Structures
type gcpInstancesOutput []struct {
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

func fetchGCPVMs(project string) ([]VM, error) {
	cmd := exec.Command("gcloud", "compute", "instances", "list", "--project", project, "--format=json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gcloud cli error: %w", err)
	}

	var data gcpInstancesOutput
	if err := json.Unmarshal(output, &data); err != nil {
		return nil, fmt.Errorf("json parse error: %w", err)
	}

	var vms []VM
	for _, inst := range data {
		machineParts := strings.Split(inst.MachineType, "/")
		mType := machineParts[len(machineParts)-1]

		zoneParts := strings.Split(inst.Zone, "/")
		zone := zoneParts[len(zoneParts)-1]

		privIP := "-"
		pubIP := "-"
		network := "-"
		subnet := "-"
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
		labelsStr := strings.Join(labelPairs, ", ")

		vms = append(vms, VM{
			Name:      inst.Name,
			ID:        inst.Id,
			Type:      mType,
			State:     inst.Status,
			PrivateIP: privIP,
			PublicIP:  pubIP,
			Zone:      zone,
			Network:   network,
			Subnet:    subnet,
			Labels:    labelsStr,
		})
	}
	return vms, nil
}

// Azure JSON Output Structures
type azureVMsOutput []struct {
	Name            string `json:"name"`
	Id              string `json:"id"` // Full resource ID
	ResourceGroup   string `json:"resourceGroup"`
	HardwareProfile struct {
		VmSize string `json:"vmSize"`
	} `json:"hardwareProfile"`
	PowerState string            `json:"powerState"`
	PrivateIps string            `json:"privateIps"`
	PublicIps  string            `json:"publicIps"`
	Tags       map[string]string `json:"tags"`
}

func fetchAzureVMs(subscription string) ([]VM, error) {
	cmd := exec.Command("az", "vm", "list", "-d", "--subscription", subscription, "--output", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("az cli error: %w", err)
	}

	var data azureVMsOutput
	if err := json.Unmarshal(output, &data); err != nil {
		return nil, fmt.Errorf("json parse error: %w", err)
	}

	var vms []VM
	for _, inst := range data {
		privIP := inst.PrivateIps
		if privIP == "" {
			privIP = "-"
		}
		pubIP := inst.PublicIps
		if pubIP == "" {
			pubIP = "-"
		}

		status := inst.PowerState
		if strings.HasPrefix(status, "VM ") {
			status = strings.TrimPrefix(status, "VM ")
		}

		var labelPairs []string
		for k, v := range inst.Tags {
			labelPairs = append(labelPairs, fmt.Sprintf("%s=%s", k, v))
		}
		labelsStr := strings.Join(labelPairs, ", ")

		// Use the last part of the ID as a short identifier if needed, or just Name
		vms = append(vms, VM{
			Name:          inst.Name,
			ID:            inst.Name,
			Type:          inst.HardwareProfile.VmSize,
			State:         status,
			PrivateIP:     privIP,
			PublicIP:      pubIP,
			ResourceGroup: inst.ResourceGroup,
			Network:       "-", // Requires additional query in Azure
			Subnet:        "-",
			Labels:        labelsStr,
		})
	}
	return vms, nil
}
