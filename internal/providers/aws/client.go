package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"cloudmanager/internal/core"
)

// --- CLI Backend ---

type awsDescribeInstancesOutput struct {
	Reservations []struct {
		Instances []struct {
			InstanceId     string `json:"InstanceId"`
			InstanceType   string `json:"InstanceType"`
			VpcId          string `json:"VpcId"`
			SubnetId       string `json:"SubnetId"`
			SecurityGroups []struct {
				GroupId string `json:"GroupId"`
			} `json:"SecurityGroups"`
			State struct {
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

func FetchVMsCLI(profile, region string) ([]core.VM, error) {
	args := []string{"--no-cli-pager", "ec2", "describe-instances", "--region", region, "--output", "json"}
	if profile != "" {
		args = append(args, "--profile", profile)
	}
	cmd := exec.Command("aws", args...)
	output, err := cmd.Output()
	if err != nil {
		var stderrStr string
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderrStr = " " + string(exitErr.Stderr)
		}
		return nil, fmt.Errorf("aws cli error:%s %w", stderrStr, err)
	}
	var data awsDescribeInstancesOutput
	if err := json.Unmarshal(output, &data); err != nil {
		return nil, fmt.Errorf("json parse error: %w", err)
	}
	var vms []core.VM
	for _, res := range data.Reservations {
		for _, inst := range res.Instances {
			name := "-"
			var labelPairs []string
			var securityGroups []string
			for _, tag := range inst.Tags {
				if tag.Key == "Name" {
					name = tag.Value
				}
				labelPairs = append(labelPairs, fmt.Sprintf("%s=%s", tag.Key, tag.Value))
			}
			for _, group := range inst.SecurityGroups {
				if group.GroupId != "" {
					securityGroups = append(securityGroups, group.GroupId)
				}
			}
			vms = append(vms, core.VM{
				Name:           name,
				ID:             inst.InstanceId,
				Type:           inst.InstanceType,
				State:          inst.State.Name,
				PrivateIP:      orDash(inst.PrivateIpAddress),
				PublicIP:       orDash(inst.PublicIpAddress),
				Zone:           region,
				Network:        orDash(inst.VpcId),
				Subnet:         orDash(inst.SubnetId),
				Labels:         strings.Join(labelPairs, ", "),
				SecurityGroups: strings.Join(securityGroups, ","),
			})
		}
	}
	return vms, nil
}

func ExecuteActionCLI(ctx context.Context, action string, vm core.VM, cloudCtx core.CloudContext) (string, error) {
	var args []string
	switch action {
	case "Start":
		args = []string{"ec2", "start-instances", "--instance-ids", vm.ID}
	case "Stop":
		args = []string{"ec2", "stop-instances", "--instance-ids", vm.ID}
	case "Restart":
		args = []string{"ec2", "reboot-instances", "--instance-ids", vm.ID}
	case "Terminate":
		args = []string{"ec2", "terminate-instances", "--instance-ids", vm.ID}
	case "Describe":
		args = []string{"ec2", "describe-instances", "--instance-ids", vm.ID}
	default:
		return "", fmt.Errorf("action %s not supported for AWS", action)
	}
	cmd := awsCLICommand(ctx, cloudCtx, args...)
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
	// If the instance has a public IP, use ec2-instance-connect ssh which is more robust
	// and works like 'gcloud compute ssh' (handles keys and uses standard SSH protocol).
	if vm.PublicIP != "" && vm.PublicIP != "-" {
		return awsCLICommand(ctx, cloudCtx, "ec2-instance-connect", "ssh", "--instance-id", vm.ID), nil
	}
	// Fallback to SSM session manager for private instances.
	return awsCLICommand(ctx, cloudCtx, "ssm", "start-session", "--target", vm.ID), nil
}

// --- SDK Backend ---

func getAWSConfig(ctx context.Context, profile, region string) (awssdk.Config, error) {
	var opts []func(*awscfg.LoadOptions) error
	if profile != "" {
		opts = append(opts, awscfg.WithSharedConfigProfile(profile))
	}
	if region != "" {
		opts = append(opts, awscfg.WithRegion(region))
	}
	return awscfg.LoadDefaultConfig(ctx, opts...)
}

func FetchVMsSDK(ctx context.Context, profile, region string) ([]core.VM, error) {
	cfg, err := getAWSConfig(ctx, profile, region)
	if err != nil {
		return nil, fmt.Errorf("failed to load aws config: %w", err)
	}
	client := ec2.NewFromConfig(cfg)
	paginator := ec2.NewDescribeInstancesPaginator(client, &ec2.DescribeInstancesInput{})
	var vms []core.VM
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe instances: %w", err)
		}
		for _, res := range page.Reservations {
			for _, inst := range res.Instances {
				name := "-"
				var labelPairs []string
				var securityGroups []string
				for _, tag := range inst.Tags {
					if awssdk.ToString(tag.Key) == "Name" {
						name = awssdk.ToString(tag.Value)
					}
					labelPairs = append(labelPairs, fmt.Sprintf("%s=%s", awssdk.ToString(tag.Key), awssdk.ToString(tag.Value)))
				}
				for _, group := range inst.SecurityGroups {
					if id := awssdk.ToString(group.GroupId); id != "" {
						securityGroups = append(securityGroups, id)
					}
				}
				vms = append(vms, core.VM{
					Name:           name,
					ID:             awssdk.ToString(inst.InstanceId),
					Type:           string(inst.InstanceType),
					State:          string(inst.State.Name),
					PrivateIP:      orDash(awssdk.ToString(inst.PrivateIpAddress)),
					PublicIP:       orDash(awssdk.ToString(inst.PublicIpAddress)),
					Zone:           region,
					Network:        orDash(awssdk.ToString(inst.VpcId)),
					Subnet:         orDash(awssdk.ToString(inst.SubnetId)),
					Labels:         strings.Join(labelPairs, ", "),
					SecurityGroups: strings.Join(securityGroups, ","),
				})
			}
		}
	}
	return vms, nil
}

func ExecuteActionSDK(ctx context.Context, action string, vm core.VM, cloudCtx core.CloudContext) (string, error) {
	cfg, err := getAWSConfig(ctx, cloudCtx.CredentialProfile, cloudCtx.Region)
	if err != nil {
		return "", fmt.Errorf("failed to load aws config: %w", err)
	}
	client := ec2.NewFromConfig(cfg)
	switch action {
	case "Start":
		_, err = client.StartInstances(ctx, &ec2.StartInstancesInput{InstanceIds: []string{vm.ID}})
	case "Stop":
		_, err = client.StopInstances(ctx, &ec2.StopInstancesInput{InstanceIds: []string{vm.ID}})
	case "Restart":
		_, err = client.RebootInstances(ctx, &ec2.RebootInstancesInput{InstanceIds: []string{vm.ID}})
	case "Terminate":
		_, err = client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{InstanceIds: []string{vm.ID}})
	case "Describe":
		resp, descErr := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{InstanceIds: []string{vm.ID}})
		if descErr != nil {
			return "", descErr
		}
		if len(resp.Reservations) > 0 && len(resp.Reservations[0].Instances) > 0 {
			inst := resp.Reservations[0].Instances[0]
			return fmt.Sprintf("Instance ID: %s\nType: %s\nState: %s\nPrivate IP: %s\nPublic IP: %s\nVPC ID: %s\nSubnet ID: %s\n",
				awssdk.ToString(inst.InstanceId), inst.InstanceType, inst.State.Name,
				awssdk.ToString(inst.PrivateIpAddress), awssdk.ToString(inst.PublicIpAddress),
				awssdk.ToString(inst.VpcId), awssdk.ToString(inst.SubnetId),
			), nil
		}
		return "", fmt.Errorf("instance not found")
	default:
		return "", fmt.Errorf("action %s not supported for AWS SDK", action)
	}
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Successfully %sed %s", strings.ToLower(action), vm.Name), nil
}

func GetSSHCmdSDK(ctx context.Context, vm core.VM, cloudCtx core.CloudContext) (*exec.Cmd, error) {
	return GetSSHCmdCLI(ctx, vm, cloudCtx)
}

func awsCLICommand(ctx context.Context, cloudCtx core.CloudContext, args ...string) *exec.Cmd {
	fullArgs := append([]string{"--no-cli-pager"}, args...)
	if cloudCtx.CredentialProfile != "" {
		fullArgs = append(fullArgs, "--profile", cloudCtx.CredentialProfile)
	}
	if cloudCtx.Region != "" {
		fullArgs = append(fullArgs, "--region", cloudCtx.Region)
	}
	return exec.CommandContext(ctx, "aws", fullArgs...)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
