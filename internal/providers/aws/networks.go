package aws

import (
	"context"
	"fmt"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"cloudmanager/internal/core"
)

// FetchNetworksSDK fetches VPCs from AWS.
func FetchNetworksSDK(ctx context.Context, profile, region string) ([]core.Network, error) {
	cfg, err := getAWSConfig(ctx, profile, region)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	// Fetch VPCs
	vpcOut, err := client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe vpcs: %w", err)
	}

	// Fetch all subnets to count per VPC
	subnetOut, err := client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe subnets: %w", err)
	}

	subnetCounts := make(map[string]int)
	for _, s := range subnetOut.Subnets {
		subnetCounts[awssdk.ToString(s.VpcId)]++
	}

	var networks []core.Network
	for _, v := range vpcOut.Vpcs {
		name := "-"
		var labels []string
		for _, tag := range v.Tags {
			if awssdk.ToString(tag.Key) == "Name" {
				name = awssdk.ToString(tag.Value)
			}
			labels = append(labels, fmt.Sprintf("%s=%s", awssdk.ToString(tag.Key), awssdk.ToString(tag.Value)))
		}

		networks = append(networks, core.Network{
			ID:          awssdk.ToString(v.VpcId),
			Name:        name,
			State:       string(v.State),
			CIDRBlock:   awssdk.ToString(v.CidrBlock),
			IsDefault:   awssdk.ToBool(v.IsDefault),
			SubnetCount: subnetCounts[awssdk.ToString(v.VpcId)],
			Provider:    "AWS",
			Region:      region,
			Labels:      strings.Join(labels, ", "),
		})
	}

	return networks, nil
}

// FetchSubnetsSDK fetches Subnets from AWS.
func FetchSubnetsSDK(ctx context.Context, profile, region string) ([]core.Subnet, error) {
	cfg, err := getAWSConfig(ctx, profile, region)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	// Fetch VPCs to resolve names
	vpcOut, err := client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{})
	vpcNames := make(map[string]string)
	if err == nil {
		for _, v := range vpcOut.Vpcs {
			name := awssdk.ToString(v.VpcId)
			for _, tag := range v.Tags {
				if awssdk.ToString(tag.Key) == "Name" {
					name = awssdk.ToString(tag.Value)
					break
				}
			}
			vpcNames[awssdk.ToString(v.VpcId)] = name
		}
	}

	out, err := client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe subnets: %w", err)
	}

	var subnets []core.Subnet
	for _, s := range out.Subnets {
		name := "-"
		var labels []string
		for _, tag := range s.Tags {
			if awssdk.ToString(tag.Key) == "Name" {
				name = awssdk.ToString(tag.Value)
			}
			labels = append(labels, fmt.Sprintf("%s=%s", awssdk.ToString(tag.Key), awssdk.ToString(tag.Value)))
		}

		subnets = append(subnets, core.Subnet{
			ID:                  awssdk.ToString(s.SubnetId),
			Name:                name,
			State:               string(s.State),
			CIDRBlock:           awssdk.ToString(s.CidrBlock),
			AvailabilityZone:    awssdk.ToString(s.AvailabilityZone),
			NetworkID:           awssdk.ToString(s.VpcId),
			NetworkName:         vpcNames[awssdk.ToString(s.VpcId)],
			AvailableIPs:        int(awssdk.ToInt32(s.AvailableIpAddressCount)),
			MapPublicIPOnLaunch: awssdk.ToBool(s.MapPublicIpOnLaunch),
			Provider:            "AWS",
			Region:              region,
			Labels:              strings.Join(labels, ", "),
		})
	}

	return subnets, nil
}
