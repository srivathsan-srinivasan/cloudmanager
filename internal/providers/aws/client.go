package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"cloudmanager/internal/core"
)

// --- CLI Backend ---

type awsDescribeInstancesOutput struct {
	Reservations []struct {
		Instances []awsInstanceCLI `json:"Instances"`
	} `json:"Reservations"`
}

type awsTagCLI struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

type awsSecurityGroupCLI struct {
	GroupId   string `json:"GroupId"`
	GroupName string `json:"GroupName"`
}

type awsBlockDeviceMappingCLI struct {
	DeviceName string `json:"DeviceName"`
	Ebs        struct {
		VolumeId            string `json:"VolumeId"`
		Status              string `json:"Status"`
		DeleteOnTermination bool   `json:"DeleteOnTermination"`
	} `json:"Ebs"`
}

type awsNetworkInterfaceCLI struct {
	NetworkInterfaceId string `json:"NetworkInterfaceId"`
	SubnetId           string `json:"SubnetId"`
	VpcId              string `json:"VpcId"`
	PrivateIpAddress   string `json:"PrivateIpAddress"`
	Association        struct {
		PublicIp string `json:"PublicIp"`
	} `json:"Association"`
	Groups []awsSecurityGroupCLI `json:"Groups"`
}

type awsInstanceCLI struct {
	InstanceId          string                            `json:"InstanceId"`
	InstanceType        string                            `json:"InstanceType"`
	ImageId             string                            `json:"ImageId"`
	Architecture        string                            `json:"Architecture"`
	PlatformDetails     string                            `json:"PlatformDetails"`
	InstanceLifecycle   string                            `json:"InstanceLifecycle"`
	LaunchTime          string                            `json:"LaunchTime"`
	KeyName             string                            `json:"KeyName"`
	VpcId               string                            `json:"VpcId"`
	SubnetId            string                            `json:"SubnetId"`
	PrivateDnsName      string                            `json:"PrivateDnsName"`
	PublicDnsName       string                            `json:"PublicDnsName"`
	RootDeviceType      string                            `json:"RootDeviceType"`
	RootDeviceName      string                            `json:"RootDeviceName"`
	VirtualizationType  string                            `json:"VirtualizationType"`
	Hypervisor          string                            `json:"Hypervisor"`
	EbsOptimized        bool                              `json:"EbsOptimized"`
	SourceDestCheck     bool                              `json:"SourceDestCheck"`
	SecurityGroups      []awsSecurityGroupCLI             `json:"SecurityGroups"`
	State               struct{ Name string }             `json:"State"`
	Placement           struct{ AvailabilityZone string } `json:"Placement"`
	IamInstanceProfile  struct{ Arn string }              `json:"IamInstanceProfile"`
	BlockDeviceMappings []awsBlockDeviceMappingCLI        `json:"BlockDeviceMappings"`
	NetworkInterfaces   []awsNetworkInterfaceCLI          `json:"NetworkInterfaces"`
	PrivateIpAddress    string                            `json:"PrivateIpAddress"`
	PublicIpAddress     string                            `json:"PublicIpAddress"`
	Tags                []awsTagCLI                       `json:"Tags"`
}

type awsDescribeVpcsOutput struct {
	Vpcs []struct {
		VpcId     string `json:"VpcId"`
		CidrBlock string `json:"CidrBlock"`
		IsDefault bool   `json:"IsDefault"`
	} `json:"Vpcs"`
}

type awsDescribeSubnetsOutput struct {
	Subnets []struct {
		SubnetId         string `json:"SubnetId"`
		VpcId            string `json:"VpcId"`
		CidrBlock        string `json:"CidrBlock"`
		AvailabilityZone string `json:"AvailabilityZone"`
	} `json:"Subnets"`
}

type awsInstanceDetail struct {
	Name              string
	InstanceID        string
	InstanceType      string
	State             string
	Lifecycle         string
	ImageID           string
	Architecture      string
	Platform          string
	LaunchTime        string
	Uptime            string
	KeyName           string
	VpcID             string
	VpcCIDR           string
	SubnetID          string
	SubnetCIDR        string
	AvailabilityZone  string
	PrivateIP         string
	PublicIP          string
	PrivateDNS        string
	PublicDNS         string
	SecurityGroups    []string
	IAMProfile        string
	RootDeviceType    string
	RootDeviceName    string
	Virtualization    string
	Hypervisor        string
	EBSOptimized      string
	SourceDestCheck   string
	Tags              []string
	Volumes           []string
	NetworkInterfaces []string
	Warnings          []string
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
		args = []string{"ec2", "describe-instances", "--instance-ids", vm.ID, "--output", "json"}
	default:
		return "", fmt.Errorf("action %s not supported for AWS", action)
	}
	cmd := awsCLICommand(ctx, cloudCtx, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to %s: %w\n%s", action, err, string(output))
	}
	if action == "Describe" {
		formatted, formatErr := formatAWSDescribeCLI(ctx, cloudCtx, output)
		if formatErr != nil {
			return string(output), nil
		}
		return formatted, nil
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
			detail := awsInstanceDetailFromSDK(inst)
			enrichAWSInstanceDetailSDK(ctx, client, &detail)
			return formatAWSInstanceDetail(detail), nil
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

func formatAWSDescribeCLI(ctx context.Context, cloudCtx core.CloudContext, output []byte) (string, error) {
	var data awsDescribeInstancesOutput
	if err := json.Unmarshal(output, &data); err != nil {
		return "", err
	}
	for _, res := range data.Reservations {
		for _, inst := range res.Instances {
			detail := awsInstanceDetailFromCLI(inst)
			enrichAWSInstanceDetailCLI(ctx, cloudCtx, &detail)
			return formatAWSInstanceDetail(detail), nil
		}
	}
	return "", fmt.Errorf("instance not found")
}

func awsInstanceDetailFromCLI(inst awsInstanceCLI) awsInstanceDetail {
	detail := awsInstanceDetail{
		Name:              tagValueCLI(inst.Tags, "Name"),
		InstanceID:        inst.InstanceId,
		InstanceType:      inst.InstanceType,
		State:             inst.State.Name,
		Lifecycle:         inst.InstanceLifecycle,
		ImageID:           inst.ImageId,
		Architecture:      inst.Architecture,
		Platform:          inst.PlatformDetails,
		KeyName:           inst.KeyName,
		VpcID:             inst.VpcId,
		SubnetID:          inst.SubnetId,
		AvailabilityZone:  inst.Placement.AvailabilityZone,
		PrivateIP:         inst.PrivateIpAddress,
		PublicIP:          inst.PublicIpAddress,
		PrivateDNS:        inst.PrivateDnsName,
		PublicDNS:         inst.PublicDnsName,
		IAMProfile:        inst.IamInstanceProfile.Arn,
		RootDeviceType:    inst.RootDeviceType,
		RootDeviceName:    inst.RootDeviceName,
		Virtualization:    inst.VirtualizationType,
		Hypervisor:        inst.Hypervisor,
		EBSOptimized:      fmt.Sprint(inst.EbsOptimized),
		SourceDestCheck:   fmt.Sprint(inst.SourceDestCheck),
		LaunchTime:        inst.LaunchTime,
		Uptime:            uptimeFromString(inst.LaunchTime),
		SecurityGroups:    securityGroupsCLI(inst.SecurityGroups),
		Tags:              tagsCLI(inst.Tags),
		Volumes:           volumesCLI(inst.BlockDeviceMappings),
		NetworkInterfaces: networkInterfacesCLI(inst.NetworkInterfaces),
	}
	if detail.Name == "" {
		detail.Name = "-"
	}
	return detail
}

func enrichAWSInstanceDetailCLI(ctx context.Context, cloudCtx core.CloudContext, detail *awsInstanceDetail) {
	if detail.VpcID != "" {
		output, err := awsCLICommand(ctx, cloudCtx, "ec2", "describe-vpcs", "--vpc-ids", detail.VpcID, "--output", "json").CombinedOutput()
		if err != nil {
			detail.Warnings = append(detail.Warnings, fmt.Sprintf("VPC CIDR lookup failed: %v", err))
		} else {
			var data awsDescribeVpcsOutput
			if err := json.Unmarshal(output, &data); err != nil {
				detail.Warnings = append(detail.Warnings, fmt.Sprintf("VPC CIDR parse failed: %v", err))
			} else if len(data.Vpcs) > 0 {
				detail.VpcCIDR = data.Vpcs[0].CidrBlock
			}
		}
	}
	if detail.SubnetID != "" {
		output, err := awsCLICommand(ctx, cloudCtx, "ec2", "describe-subnets", "--subnet-ids", detail.SubnetID, "--output", "json").CombinedOutput()
		if err != nil {
			detail.Warnings = append(detail.Warnings, fmt.Sprintf("Subnet CIDR lookup failed: %v", err))
		} else {
			var data awsDescribeSubnetsOutput
			if err := json.Unmarshal(output, &data); err != nil {
				detail.Warnings = append(detail.Warnings, fmt.Sprintf("Subnet CIDR parse failed: %v", err))
			} else if len(data.Subnets) > 0 {
				detail.SubnetCIDR = data.Subnets[0].CidrBlock
				if detail.AvailabilityZone == "" {
					detail.AvailabilityZone = data.Subnets[0].AvailabilityZone
				}
			}
		}
	}
}

func awsInstanceDetailFromSDK(inst ec2types.Instance) awsInstanceDetail {
	launch := ""
	uptime := "-"
	if inst.LaunchTime != nil {
		launch = inst.LaunchTime.Local().Format(time.RFC3339)
		uptime = compactDuration(time.Since(*inst.LaunchTime))
	}
	detail := awsInstanceDetail{
		Name:              tagValueSDK(inst.Tags, "Name"),
		InstanceID:        awssdk.ToString(inst.InstanceId),
		InstanceType:      string(inst.InstanceType),
		State:             string(inst.State.Name),
		Lifecycle:         string(inst.InstanceLifecycle),
		ImageID:           awssdk.ToString(inst.ImageId),
		Architecture:      string(inst.Architecture),
		Platform:          awssdk.ToString(inst.PlatformDetails),
		KeyName:           awssdk.ToString(inst.KeyName),
		VpcID:             awssdk.ToString(inst.VpcId),
		SubnetID:          awssdk.ToString(inst.SubnetId),
		AvailabilityZone:  awssdk.ToString(inst.Placement.AvailabilityZone),
		PrivateIP:         awssdk.ToString(inst.PrivateIpAddress),
		PublicIP:          awssdk.ToString(inst.PublicIpAddress),
		PrivateDNS:        awssdk.ToString(inst.PrivateDnsName),
		PublicDNS:         awssdk.ToString(inst.PublicDnsName),
		IAMProfile:        awssdk.ToString(inst.IamInstanceProfile.Arn),
		RootDeviceType:    string(inst.RootDeviceType),
		RootDeviceName:    awssdk.ToString(inst.RootDeviceName),
		Virtualization:    string(inst.VirtualizationType),
		Hypervisor:        string(inst.Hypervisor),
		EBSOptimized:      fmt.Sprint(awssdk.ToBool(inst.EbsOptimized)),
		SourceDestCheck:   fmt.Sprint(awssdk.ToBool(inst.SourceDestCheck)),
		LaunchTime:        launch,
		Uptime:            uptime,
		SecurityGroups:    securityGroupsSDK(inst.SecurityGroups),
		Tags:              tagsSDK(inst.Tags),
		Volumes:           volumesSDK(inst.BlockDeviceMappings),
		NetworkInterfaces: networkInterfacesSDK(inst.NetworkInterfaces),
	}
	if detail.Name == "" {
		detail.Name = "-"
	}
	return detail
}

func enrichAWSInstanceDetailSDK(ctx context.Context, client *ec2.Client, detail *awsInstanceDetail) {
	if detail.VpcID != "" {
		resp, err := client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{VpcIds: []string{detail.VpcID}})
		if err != nil {
			detail.Warnings = append(detail.Warnings, fmt.Sprintf("VPC CIDR lookup failed: %v", err))
		} else if len(resp.Vpcs) > 0 {
			detail.VpcCIDR = awssdk.ToString(resp.Vpcs[0].CidrBlock)
		}
	}
	if detail.SubnetID != "" {
		resp, err := client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{SubnetIds: []string{detail.SubnetID}})
		if err != nil {
			detail.Warnings = append(detail.Warnings, fmt.Sprintf("Subnet CIDR lookup failed: %v", err))
		} else if len(resp.Subnets) > 0 {
			detail.SubnetCIDR = awssdk.ToString(resp.Subnets[0].CidrBlock)
			if detail.AvailabilityZone == "" {
				detail.AvailabilityZone = awssdk.ToString(resp.Subnets[0].AvailabilityZone)
			}
		}
	}
}

func formatAWSInstanceDetail(d awsInstanceDetail) string {
	var b strings.Builder
	writeSection := func(title string) {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(title)
		b.WriteString("\n")
	}
	writeKV := func(key, value string) {
		b.WriteString("  ")
		b.WriteString(key)
		b.WriteString(": ")
		b.WriteString(orDash(value))
		b.WriteString("\n")
	}
	writeList := func(key string, values []string) {
		b.WriteString("  ")
		b.WriteString(key)
		b.WriteString(": ")
		if len(values) == 0 {
			b.WriteString("-\n")
			return
		}
		b.WriteString(strings.Join(values, "\n    - "))
		b.WriteString("\n")
	}

	b.WriteString("AWS INSTANCE DETAILS\n\n")
	writeSection("Identity")
	writeKV("Name", d.Name)
	writeKV("Instance ID", d.InstanceID)
	writeKV("Type", d.InstanceType)
	writeKV("State", d.State)
	writeKV("Lifecycle", d.Lifecycle)
	writeKV("AMI", d.ImageID)
	writeKV("Architecture", d.Architecture)
	writeKV("Platform", d.Platform)

	writeSection("Runtime")
	writeKV("Launch Time / Last Start", d.LaunchTime)
	writeKV("Uptime", d.Uptime)
	writeKV("SSH Key Pair", d.KeyName)
	writeKV("IAM Profile", d.IAMProfile)

	writeSection("Network")
	writeKV("VPC ID", d.VpcID)
	writeKV("VPC CIDR", d.VpcCIDR)
	writeKV("Subnet ID", d.SubnetID)
	writeKV("Subnet CIDR", d.SubnetCIDR)
	writeKV("Availability Zone", d.AvailabilityZone)
	writeKV("Private IP", d.PrivateIP)
	writeKV("Public IP", d.PublicIP)
	writeKV("Private DNS", d.PrivateDNS)
	writeKV("Public DNS", d.PublicDNS)
	writeList("Security Groups", d.SecurityGroups)
	writeList("Network Interfaces", d.NetworkInterfaces)

	writeSection("Storage And Placement")
	writeKV("Root Device Type", d.RootDeviceType)
	writeKV("Root Device Name", d.RootDeviceName)
	writeKV("Virtualization", d.Virtualization)
	writeKV("Hypervisor", d.Hypervisor)
	writeKV("EBS Optimized", d.EBSOptimized)
	writeKV("Source/Dest Check", d.SourceDestCheck)
	writeList("Volumes", d.Volumes)

	writeSection("Tags")
	writeList("Tags", d.Tags)

	if len(d.Warnings) > 0 {
		writeSection("Warnings")
		writeList("Warnings", d.Warnings)
	}
	return strings.TrimRight(b.String(), "\n")
}

func tagValueCLI(tags []awsTagCLI, key string) string {
	for _, tag := range tags {
		if tag.Key == key {
			return tag.Value
		}
	}
	return ""
}

func tagsCLI(tags []awsTagCLI) []string {
	values := make([]string, 0, len(tags))
	for _, tag := range tags {
		values = append(values, fmt.Sprintf("%s=%s", tag.Key, tag.Value))
	}
	return values
}

func securityGroupsCLI(groups []awsSecurityGroupCLI) []string {
	values := make([]string, 0, len(groups))
	for _, group := range groups {
		values = append(values, groupName(group.GroupName, group.GroupId))
	}
	return values
}

func volumesCLI(mappings []awsBlockDeviceMappingCLI) []string {
	values := make([]string, 0, len(mappings))
	for _, mapping := range mappings {
		values = append(values, fmt.Sprintf("%s -> %s (%s, delete on termination: %t)", mapping.DeviceName, mapping.Ebs.VolumeId, mapping.Ebs.Status, mapping.Ebs.DeleteOnTermination))
	}
	return values
}

func networkInterfacesCLI(interfaces []awsNetworkInterfaceCLI) []string {
	values := make([]string, 0, len(interfaces))
	for _, iface := range interfaces {
		values = append(values, fmt.Sprintf("%s private=%s public=%s subnet=%s vpc=%s groups=%s",
			iface.NetworkInterfaceId,
			orDash(iface.PrivateIpAddress),
			orDash(iface.Association.PublicIp),
			orDash(iface.SubnetId),
			orDash(iface.VpcId),
			strings.Join(securityGroupsCLI(iface.Groups), ", "),
		))
	}
	return values
}

func tagValueSDK(tags []ec2types.Tag, key string) string {
	for _, tag := range tags {
		if awssdk.ToString(tag.Key) == key {
			return awssdk.ToString(tag.Value)
		}
	}
	return ""
}

func tagsSDK(tags []ec2types.Tag) []string {
	values := make([]string, 0, len(tags))
	for _, tag := range tags {
		values = append(values, fmt.Sprintf("%s=%s", awssdk.ToString(tag.Key), awssdk.ToString(tag.Value)))
	}
	return values
}

func securityGroupsSDK(groups []ec2types.GroupIdentifier) []string {
	values := make([]string, 0, len(groups))
	for _, group := range groups {
		values = append(values, groupName(awssdk.ToString(group.GroupName), awssdk.ToString(group.GroupId)))
	}
	return values
}

func volumesSDK(mappings []ec2types.InstanceBlockDeviceMapping) []string {
	values := make([]string, 0, len(mappings))
	for _, mapping := range mappings {
		values = append(values, fmt.Sprintf("%s -> %s (%s, delete on termination: %t)",
			awssdk.ToString(mapping.DeviceName),
			awssdk.ToString(mapping.Ebs.VolumeId),
			string(mapping.Ebs.Status),
			awssdk.ToBool(mapping.Ebs.DeleteOnTermination),
		))
	}
	return values
}

func networkInterfacesSDK(interfaces []ec2types.InstanceNetworkInterface) []string {
	values := make([]string, 0, len(interfaces))
	for _, iface := range interfaces {
		values = append(values, fmt.Sprintf("%s private=%s public=%s subnet=%s vpc=%s groups=%s",
			awssdk.ToString(iface.NetworkInterfaceId),
			orDash(awssdk.ToString(iface.PrivateIpAddress)),
			orDash(awssdk.ToString(iface.Association.PublicIp)),
			orDash(awssdk.ToString(iface.SubnetId)),
			orDash(awssdk.ToString(iface.VpcId)),
			strings.Join(securityGroupsSDK(iface.Groups), ", "),
		))
	}
	return values
}

func groupName(name, id string) string {
	switch {
	case name != "" && id != "":
		return fmt.Sprintf("%s (%s)", name, id)
	case name != "":
		return name
	default:
		return id
	}
}

func uptimeFromString(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return "-"
	}
	return compactDuration(time.Since(parsed))
}

func compactDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
