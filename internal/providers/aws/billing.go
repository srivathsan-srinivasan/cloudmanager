package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"

	"cloudmanager/internal/core"
)

// FetchAccountCostSDK fetches the account cost using Cost Explorer.
func FetchAccountCostSDK(ctx context.Context, profile string) (*core.AccountCost, error) {
	cfg, err := getAWSConfig(ctx, profile, "us-east-1") // CE requires us-east-1 or default
	if err != nil {
		return nil, err
	}

	client := costexplorer.NewFromConfig(cfg)
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	endOfMonth := startOfMonth.AddDate(0, 1, 0)
	startOfPrevMonth := startOfMonth.AddDate(0, -1, 0)

	input := &costexplorer.GetCostAndUsageInput{
		TimePeriod: &types.DateInterval{
			Start: aws.String(startOfPrevMonth.Format("2006-01-02")),
			End:   aws.String(endOfMonth.Format("2006-01-02")),
		},
		Granularity: types.GranularityMonthly,
		Metrics:     []string{"UnblendedCost"},
		GroupBy: []types.GroupDefinition{
			{
				Type: types.GroupDefinitionTypeDimension,
				Key:  aws.String("SERVICE"),
			},
		},
	}

	out, err := client.GetCostAndUsage(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("Cost Explorer API error (ensure billing permissions): %w", err)
	}

	var currentCost, prevCost float64
	var services []core.ServiceCost

	for _, res := range out.ResultsByTime {
		if res.TimePeriod == nil || res.TimePeriod.Start == nil {
			continue
		}
		isCurrentMonth := *res.TimePeriod.Start == startOfMonth.Format("2006-01-02")
		for _, group := range res.Groups {
			if len(group.Keys) == 0 {
				continue
			}
			serviceName := group.Keys[0]
			costStr := *group.Metrics["UnblendedCost"].Amount
			var cost float64
			fmt.Sscanf(costStr, "%f", &cost)

			if isCurrentMonth {
				currentCost += cost
				services = append(services, core.ServiceCost{
					ServiceName: serviceName,
					Cost:        cost,
				})
			} else {
				prevCost += cost
			}
		}
	}

	// Try forecast
	var forecastedCost float64
	forecastInput := &costexplorer.GetCostForecastInput{
		TimePeriod: &types.DateInterval{
			Start: aws.String(now.Format("2006-01-02")),
			End:   aws.String(endOfMonth.Format("2006-01-02")),
		},
		Metric:      types.MetricUnblendedCost,
		Granularity: types.GranularityMonthly,
	}
	fOut, fErr := client.GetCostForecast(ctx, forecastInput)
	if fErr == nil && fOut.Total != nil && fOut.Total.Amount != nil {
		fmt.Sscanf(*fOut.Total.Amount, "%f", &forecastedCost)
	}

	return &core.AccountCost{
		Provider:          "AWS",
		AccountID:         profile,
		CurrentMonthCost:  currentCost,
		PreviousMonthCost: prevCost,
		ForecastedCost:    forecastedCost,
		TopServices:       services,
		LastUpdated:       time.Now(),
	}, nil
}

// FetchVMCostSDK fetches the specific VM cost.
func FetchVMCostSDK(ctx context.Context, profile, instanceID string) (*core.ResourceCost, error) {
	cfg, err := getAWSConfig(ctx, profile, "us-east-1")
	if err != nil {
		return nil, err
	}
	client := costexplorer.NewFromConfig(cfg)
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	endOfMonth := startOfMonth.AddDate(0, 1, 0)
	startOfPrevMonth := startOfMonth.AddDate(0, -1, 0)

	input := &costexplorer.GetCostAndUsageInput{
		TimePeriod: &types.DateInterval{
			Start: aws.String(startOfPrevMonth.Format("2006-01-02")),
			End:   aws.String(endOfMonth.Format("2006-01-02")),
		},
		Granularity: types.GranularityMonthly,
		Metrics:     []string{"UnblendedCost"},
		Filter: &types.Expression{
			Dimensions: &types.DimensionValues{
				Key:    types.DimensionResourceId,
				Values: []string{instanceID},
			},
		},
	}

	out, err := client.GetCostAndUsage(ctx, input)
	if err != nil {
		return &core.ResourceCost{
			ResourceID:       instanceID,
			Provider:         "AWS",
			CurrentMonthCost: 0, // Fallback placeholder
			Currency:         "USD",
			LastUpdated:      time.Now(),
		}, nil
	}

	var currentCost, prevCost float64
	for _, res := range out.ResultsByTime {
		if res.TimePeriod == nil || res.TimePeriod.Start == nil {
			continue
		}
		isCurrentMonth := *res.TimePeriod.Start == startOfMonth.Format("2006-01-02")
		if res.Total != nil {
			if metric, ok := res.Total["UnblendedCost"]; ok && metric.Amount != nil {
				var cost float64
				fmt.Sscanf(*metric.Amount, "%f", &cost)
				if isCurrentMonth {
					currentCost = cost
				} else {
					prevCost = cost
				}
			}
		}
	}

	return &core.ResourceCost{
		ResourceID:        instanceID,
		Provider:          "AWS",
		CurrentMonthCost:  currentCost,
		PreviousMonthCost: prevCost,
		Currency:          "USD",
		LastUpdated:       time.Now(),
	}, nil
}
