package azure

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/advisor/armadvisor"

	"github.com/vyoogam/cloudmanager/internal/core"
)

func FetchRecommendationsSDK(ctx context.Context, subscriptionID string) ([]core.Recommendation, error) {
	cred, err := getAzureCreds()
	if err != nil {
		return nil, fmt.Errorf("failed to get azure credentials: %w", err)
	}
	clientFactory, err := armadvisor.NewClientFactory(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create advisor client factory: %w", err)
	}
	client := clientFactory.NewRecommendationsClient()

	pager := client.NewListPager(&armadvisor.RecommendationsClientListOptions{
		Filter: to.Ptr("Category eq 'Cost'"),
	})

	var recs []core.Recommendation
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get azure recommendations: %w", err)
		}
		for _, rec := range page.Value {
			if rec.Properties == nil {
				continue
			}

			severity := "Info"
			if rec.Properties.Impact != nil {
				impact := string(*rec.Properties.Impact)
				if impact == "High" {
					severity = "Critical"
				} else if impact == "Medium" {
					severity = "Warning"
				}
			}

			summary := "-"
			if rec.Properties.ShortDescription != nil && rec.Properties.ShortDescription.Problem != nil {
				summary = *rec.Properties.ShortDescription.Problem
			}

			estimatedSavings := 0.0
			if rec.Properties.ExtendedProperties != nil {
				if savingsStr, ok := rec.Properties.ExtendedProperties["annualSavingsAmount"]; ok && savingsStr != nil {
					if val, err := strconv.ParseFloat(*savingsStr, 64); err == nil {
						estimatedSavings = val / 12.0
					}
				}
			}

			resourceID := "-"
			if rec.Properties.ResourceMetadata != nil && rec.Properties.ResourceMetadata.ResourceID != nil {
				resourceID = *rec.Properties.ResourceMetadata.ResourceID
			}

			recs = append(recs, core.Recommendation{
				ResourceID:       resourceID,
				Provider:         "Azure",
				Type:             "Rightsizing",
				Severity:         severity,
				Summary:          summary,
				Detail:           orPtr(rec.Properties.Description),
				EstimatedSavings: estimatedSavings,
				Source:           "Azure Advisor",
				LastUpdated:      orTime(rec.Properties.LastUpdated),
			})
		}
	}
	return recs, nil
}

func orTime(t *time.Time) time.Time {
	if t == nil {
		return time.Now()
	}
	return *t
}
