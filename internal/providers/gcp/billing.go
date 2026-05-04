package gcp

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"

	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
)

// FetchAccountCostSDK fetches GCP billing export from BigQuery.
func FetchAccountCostSDK(ctx context.Context, projectID string, dataset, table string) (*core.AccountCost, error) {
	if dataset == "" || table == "" {
		return nil, fmt.Errorf("GCP billing export not configured. Showing estimated costs.")
	}

	client, err := newBigQueryClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create BigQuery client: %w", err)
	}
	defer client.Close()

	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	startOfPrevMonth := startOfMonth.AddDate(0, -1, 0)

	// Query current month cost grouped by service
	qStr := fmt.Sprintf(`
		SELECT service.description as service_name, SUM(cost) as total_cost
		FROM `+"`%s.%s.%s`"+`
		WHERE usage_start_time >= TIMESTAMP(@start_time)
		GROUP BY service.description
		ORDER BY total_cost DESC
	`, projectID, dataset, table)

	q := client.Query(qStr)
	q.Parameters = []bigquery.QueryParameter{
		{Name: "start_time", Value: startOfMonth},
	}

	it, err := q.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("BigQuery query failed: %w", err)
	}

	var currentCost float64
	var services []core.ServiceCost

	for {
		var row struct {
			ServiceName string  `bigquery:"service_name"`
			TotalCost   float64 `bigquery:"total_cost"`
		}
		err := it.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		currentCost += row.TotalCost
		services = append(services, core.ServiceCost{
			ServiceName: row.ServiceName,
			Cost:        row.TotalCost,
		})
	}

	// Query previous month total
	qPrevStr := fmt.Sprintf(`
		SELECT SUM(cost) as total_cost
		FROM `+"`%s.%s.%s`"+`
		WHERE usage_start_time >= TIMESTAMP(@start_prev) AND usage_start_time < TIMESTAMP(@start_time)
	`, projectID, dataset, table)
	qPrev := client.Query(qPrevStr)
	qPrev.Parameters = []bigquery.QueryParameter{
		{Name: "start_prev", Value: startOfPrevMonth},
		{Name: "start_time", Value: startOfMonth},
	}
	itPrev, err := qPrev.Read(ctx)
	var prevCost float64
	if err == nil {
		var row struct {
			TotalCost float64 `bigquery:"total_cost"`
		}
		if err := itPrev.Next(&row); err == nil {
			prevCost = row.TotalCost
		}
	}

	return &core.AccountCost{
		Provider:          "GCP",
		AccountID:         projectID,
		CurrentMonthCost:  currentCost,
		PreviousMonthCost: prevCost,
		TopServices:       services,
		LastUpdated:       time.Now(),
	}, nil
}

// FetchVMCostSDK fetches the specific VM cost.
func FetchVMCostSDK(ctx context.Context, projectID, instanceID, dataset, table string) (*core.ResourceCost, error) {
	if dataset == "" || table == "" {
		return &core.ResourceCost{
			ResourceID:       instanceID,
			Provider:         "GCP",
			CurrentMonthCost: 0.0,
			Currency:         "USD",
			LastUpdated:      time.Now(),
		}, nil
	}

	client, err := newBigQueryClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create BigQuery client: %w", err)
	}
	defer client.Close()

	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	startOfPrevMonth := startOfMonth.AddDate(0, -1, 0)

	// Try to match on resource name or ID. We use name since ID might not always be in billing perfectly.
	qStr := fmt.Sprintf(`
		SELECT SUM(cost) as total_cost
		FROM `+"`%s.%s.%s`"+`
		WHERE usage_start_time >= TIMESTAMP(@start_time)
		AND (resource.name = @instance_id OR resource.global_name LIKE CONCAT('%%', @instance_id))
	`, projectID, dataset, table)

	q := client.Query(qStr)
	q.Parameters = []bigquery.QueryParameter{
		{Name: "start_time", Value: startOfMonth},
		{Name: "instance_id", Value: instanceID},
	}

	it, err := q.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("BigQuery query failed: %w", err)
	}

	var currentCost float64
	var row struct {
		TotalCost float64 `bigquery:"total_cost"`
	}
	if err := it.Next(&row); err == nil {
		currentCost = row.TotalCost
	}

	// Prev month
	qPrevStr := fmt.Sprintf(`
		SELECT SUM(cost) as total_cost
		FROM `+"`%s.%s.%s`"+`
		WHERE usage_start_time >= TIMESTAMP(@start_prev) AND usage_start_time < TIMESTAMP(@start_time)
		AND (resource.name = @instance_id OR resource.global_name LIKE CONCAT('%%', @instance_id))
	`, projectID, dataset, table)
	qPrev := client.Query(qPrevStr)
	qPrev.Parameters = []bigquery.QueryParameter{
		{Name: "start_prev", Value: startOfPrevMonth},
		{Name: "start_time", Value: startOfMonth},
		{Name: "instance_id", Value: instanceID},
	}
	itPrev, err := qPrev.Read(ctx)
	var prevCost float64
	if err == nil {
		if err := itPrev.Next(&row); err == nil {
			prevCost = row.TotalCost
		}
	}

	return &core.ResourceCost{
		ResourceID:        instanceID,
		Provider:          "GCP",
		CurrentMonthCost:  currentCost,
		PreviousMonthCost: prevCost,
		Currency:          "USD",
		LastUpdated:       time.Now(),
	}, nil
}
