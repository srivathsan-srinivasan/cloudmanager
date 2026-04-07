package azure

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/costmanagement/armcostmanagement"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"

	"cloudmanager/internal/core"
)

// FetchAccountCostSDK fetches Azure cost management data.
func FetchAccountCostSDK(ctx context.Context, subscriptionID string) (*core.AccountCost, error) {
	cred, err := getAzureCreds()
	if err != nil {
		return nil, fmt.Errorf("failed to get Azure credential: %w", err)
	}

	client, err := armcostmanagement.NewQueryClient(cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create QueryClient: %w", err)
	}

	scope := fmt.Sprintf("/subscriptions/%s", subscriptionID)

	res, err := client.Usage(ctx, scope, armcostmanagement.QueryDefinition{
		Type: to.Ptr(armcostmanagement.ExportTypeActualCost),
		Timeframe: to.Ptr(armcostmanagement.TimeframeTypeMonthToDate),
		Dataset: &armcostmanagement.QueryDataset{
			Granularity: to.Ptr(armcostmanagement.GranularityTypeDaily),
			Grouping: []*armcostmanagement.QueryGrouping{
				{
					Type: to.Ptr(armcostmanagement.QueryColumnTypeDimension),
					Name: to.Ptr("ServiceName"),
				},
			},
		},
	}, nil)

	if err != nil {
		return nil, fmt.Errorf("Azure Cost Management API error: %w", err)
	}

	var currentCost float64
	var services []core.ServiceCost

	if res.Properties != nil && res.Properties.Rows != nil {
		for _, row := range res.Properties.Rows {
			if len(row) > 0 {
				if costVal, ok := row[0].(float64); ok {
					currentCost += costVal
					serviceName := "Unknown"
					if len(row) > 1 {
						if nameStr, ok := row[1].(string); ok {
							serviceName = nameStr
						}
					}
					services = append(services, core.ServiceCost{
						ServiceName: serviceName,
						Cost:        costVal,
					})
				}
			}
		}
	}

	return &core.AccountCost{
		Provider:          "Azure",
		AccountID:         subscriptionID,
		CurrentMonthCost:  currentCost,
		TopServices:       services,
		LastUpdated:       time.Now(),
	}, nil
}

// FetchVMCostSDK fetches the specific VM cost.
func FetchVMCostSDK(ctx context.Context, subscriptionID, vmResourceID string) (*core.ResourceCost, error) {
	cred, err := getAzureCreds()
	if err != nil {
		return nil, fmt.Errorf("failed to get Azure credential: %w", err)
	}

	client, err := armcostmanagement.NewQueryClient(cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create QueryClient: %w", err)
	}

	scope := fmt.Sprintf("/subscriptions/%s", subscriptionID)
	
	res, err := client.Usage(ctx, scope, armcostmanagement.QueryDefinition{
		Type: to.Ptr(armcostmanagement.ExportTypeActualCost),
		Timeframe: to.Ptr(armcostmanagement.TimeframeTypeMonthToDate),
		Dataset: &armcostmanagement.QueryDataset{
			Granularity: to.Ptr(armcostmanagement.GranularityTypeDaily),
			Filter: &armcostmanagement.QueryFilter{
				Dimensions: &armcostmanagement.QueryComparisonExpression{
					Name: to.Ptr("ResourceId"),
					Operator: to.Ptr(armcostmanagement.QueryOperatorTypeIn),
					Values: []*string{to.Ptr(vmResourceID)},
				},
			},
		},
	}, nil)

	if err != nil {
		return &core.ResourceCost{
			ResourceID:       vmResourceID,
			Provider:         "Azure",
			CurrentMonthCost: 0, // Fallback placeholder
			Currency:         "USD",
			LastUpdated:      time.Now(),
		}, nil
	}

	var currentCost float64
	if res.Properties != nil && res.Properties.Rows != nil {
		for _, row := range res.Properties.Rows {
			if len(row) > 0 {
				if costVal, ok := row[0].(float64); ok {
					currentCost += costVal
				}
			}
		}
	}

	return &core.ResourceCost{
		ResourceID:        vmResourceID,
		Provider:          "Azure",
		CurrentMonthCost:  currentCost,
		Currency:          "USD",
		LastUpdated:       time.Now(),
	}, nil
}
