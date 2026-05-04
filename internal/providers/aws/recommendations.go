package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/computeoptimizer"

	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
)

// FetchRecommendationsSDK fetches EC2 instance recommendations from AWS Compute Optimizer.
func FetchRecommendationsSDK(ctx context.Context, profile, region string) ([]core.Recommendation, error) {
	cfg, err := getAWSConfig(ctx, profile, region)
	if err != nil {
		return nil, err
	}
	client := computeoptimizer.NewFromConfig(cfg)

	input := &computeoptimizer.GetEC2InstanceRecommendationsInput{}
	out, err := client.GetEC2InstanceRecommendations(ctx, input)
	if err != nil {
		// Handle OptInRequiredException gracefully
		if strings.Contains(err.Error(), "OptInRequiredException") {
			return nil, fmt.Errorf("Compute Optimizer not enabled. Enable it in AWS Console")
		}
		return nil, fmt.Errorf("Compute Optimizer API error: %w", err)
	}

	var recs []core.Recommendation
	for _, rec := range out.InstanceRecommendations {
		finding := string(rec.Finding)
		if finding == "OPTIMIZED" {
			continue
		}

		severity := "Info"
		if finding == "UNDER_PROVISIONED" {
			severity = "Warning"
		} else if finding == "OVER_PROVISIONED" {
			severity = "Info"
		}

		instanceID := instanceIDFromARN(awssdk.ToString(rec.InstanceArn))
		currentType := awssdk.ToString(rec.CurrentInstanceType)

		recommendedConfig := "-"
		estimatedSavings := 0.0
		summary := fmt.Sprintf("%s: current %s", finding, currentType)

		if len(rec.RecommendationOptions) > 0 {
			opt := rec.RecommendationOptions[0]
			recommendedConfig = awssdk.ToString(opt.InstanceType)
			summary = fmt.Sprintf("%s: %s -> %s", finding, currentType, recommendedConfig)

			if opt.SavingsOpportunity != nil {
				estimatedSavings = opt.SavingsOpportunity.EstimatedMonthlySavings.Value
			}
		}

		recs = append(recs, core.Recommendation{
			ResourceID:        instanceID,
			ResourceName:      awssdk.ToString(rec.InstanceName),
			ResourceType:      "EC2 Instance",
			Provider:          "AWS",
			Type:              "Rightsizing",
			Severity:          severity,
			Summary:           summary,
			EstimatedSavings:  estimatedSavings,
			CurrentConfig:     currentType,
			RecommendedConfig: recommendedConfig,
			Source:            "AWS Compute Optimizer",
			LastUpdated:       time.Now(),
		})
	}

	return recs, nil
}

func instanceIDFromARN(arn string) string {
	parts := strings.Split(arn, "/")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return arn
}
