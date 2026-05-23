package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"

	"github.com/vyoogam/cloudmanager/internal/core"
)

func runAwsJSON(profile, region string, args ...string) ([]byte, error) {
	fullArgs := append([]string{"--no-cli-pager"}, args...)
	if profile != "" {
		fullArgs = append(fullArgs, "--profile", profile)
	}
	if region != "" {
		fullArgs = append(fullArgs, "--region", region)
	}
	fullArgs = append(fullArgs, "--output", "json")
	cmd := exec.Command("aws", fullArgs...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("aws cli failed: %w", err)
	}
	return output, nil
}

func FetchClustersCLI(profile, region string) ([]core.Cluster, error) {
	out, err := runAwsJSON(profile, region, "eks", "list-clusters")
	if err != nil {
		return nil, err
	}

	var listData struct {
		Clusters []string `json:"clusters"`
	}
	if err := json.Unmarshal(out, &listData); err != nil {
		return nil, err
	}

	var clusters []core.Cluster
	for _, name := range listData.Clusters {
		descOut, err := runAwsJSON(profile, region, "eks", "describe-cluster", "--name", name)
		if err != nil {
			continue
		}

		var descData struct {
			Cluster struct {
				Name    string            `json:"name"`
				Arn     string            `json:"arn"`
				Status  string            `json:"status"`
				Version string            `json:"version"`
				Tags    map[string]string `json:"tags"`
			} `json:"cluster"`
		}
		if err := json.Unmarshal(descOut, &descData); err != nil {
			continue
		}

		c := descData.Cluster
		var labels []string
		for k, v := range c.Tags {
			labels = append(labels, fmt.Sprintf("%s=%s", k, v))
		}

		clusters = append(clusters, core.Cluster{
			ID:       c.Arn,
			Name:     c.Name,
			Location: region,
			Status:   c.Status,
			Version:  c.Version,
			Labels:   strings.Join(labels, ", "),
		})
	}

	return clusters, nil
}

func FetchClustersSDK(ctx context.Context, profile, region string) ([]core.Cluster, error) {
	cfg, err := getAWSConfig(ctx, profile, region)
	if err != nil {
		return nil, err
	}
	client := eks.NewFromConfig(cfg)

	listOut, err := client.ListClusters(ctx, &eks.ListClustersInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list eks clusters: %w", err)
	}

	var clusters []core.Cluster
	for _, name := range listOut.Clusters {
		descOut, err := client.DescribeCluster(ctx, &eks.DescribeClusterInput{
			Name: awssdk.String(name),
		})
		if err != nil {
			continue
		}
		c := descOut.Cluster
		status := string(c.Status)
		version := awssdk.ToString(c.Version)

		var labels []string
		for k, v := range c.Tags {
			labels = append(labels, fmt.Sprintf("%s=%s", k, v))
		}

		clusters = append(clusters, core.Cluster{
			ID:       awssdk.ToString(c.Arn),
			Name:     awssdk.ToString(c.Name),
			Location: region,
			Status:   status,
			Version:  version,
			Labels:   strings.Join(labels, ", "),
		})
	}

	return clusters, nil
}
