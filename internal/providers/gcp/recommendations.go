package gcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/recommender/apiv1/recommenderpb"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/iterator"
)

// FetchRecommendationsSDK fetches VM recommendations from GCP Recommender.
func FetchRecommendationsSDK(ctx context.Context, projectID string) ([]core.Recommendation, error) {
	client, err := newRecommenderClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create recommender client: %w", err)
	}
	defer client.Close()

	service, err := newComputeService(ctx)
	if err != nil {
		return nil, err
	}

	// Fetch active zones
	zoneReq := service.Zones.List(projectID)
	var zones []string
	if err := zoneReq.Pages(ctx, func(page *compute.ZoneList) error {
		for _, z := range page.Items {
			if z.Status == "UP" {
				zones = append(zones, z.Name)
			}
		}
		return nil
	}); err != nil {
		// Fallback
		zones = []string{"us-central1-a", "us-east1-b", "europe-west1-b"}
	}

	var recs []core.Recommendation
	recommenderIDs := []string{
		"google.compute.instance.MachineTypeRecommender",
		"google.compute.instance.IdleResourceRecommender",
	}

	for _, zone := range zones {
		for _, recommenderID := range recommenderIDs {
			parent := fmt.Sprintf("projects/%s/locations/%s/recommenders/%s", projectID, zone, recommenderID)
			it := client.ListRecommendations(ctx, &recommenderpb.ListRecommendationsRequest{
				Parent: parent,
			})
			for {
				rec, err := it.Next()
				if err == iterator.Done {
					break
				}
				if err != nil {
					break
				}

				severity := "Info"
				if rec.Priority == recommenderpb.Recommendation_P1 {
					severity = "Critical"
				} else if rec.Priority == recommenderpb.Recommendation_P2 {
					severity = "Warning"
				}

				summary := rec.Description
				estimatedSavings := 0.0
				if rec.PrimaryImpact != nil {
					if cp := rec.PrimaryImpact.GetCostProjection(); cp != nil && cp.Cost != nil {
						total := float64(cp.Cost.Units) + float64(cp.Cost.Nanos)/1e9
						if total < 0 {
							estimatedSavings = -total
						}
					}
				}

				resourceID := "-"
				if rec.Content != nil && len(rec.Content.OperationGroups) > 0 {
					og := rec.Content.OperationGroups[0]
					if len(og.Operations) > 0 {
						parts := strings.Split(og.Operations[0].Resource, "/")
						resourceID = parts[len(parts)-1]
					}
				}

				recs = append(recs, core.Recommendation{
					ResourceID:       resourceID,
					Provider:         "GCP",
					Type:             "Rightsizing",
					Severity:         severity,
					Summary:          summary,
					Detail:           rec.Description,
					EstimatedSavings: estimatedSavings,
					Source:           "GCP Recommender",
					LastUpdated:      time.Now(),
				})
			}
		}
	}
	return recs, nil
}
