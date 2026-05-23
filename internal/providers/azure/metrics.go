package azure

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"

	"github.com/vyoogam/cloudmanager/internal/core"
)

func FetchVMMetricsSDK(ctx context.Context, subscriptionID, resourceID string, period time.Duration) (*core.VMMetrics, error) {
	cred, err := getAzureCreds()
	if err != nil {
		return nil, fmt.Errorf("failed to get azure credentials: %w", err)
	}
	client, err := armmonitor.NewMetricsClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create azure monitor client: %w", err)
	}

	metrics := &core.VMMetrics{
		VMID:      resourceID,
		FetchedAt: time.Now(),
		Period:    period,
	}

	endTime := time.Now()
	startTime := endTime.Add(-period)
	timespan := fmt.Sprintf("%s/%s", startTime.Format(time.RFC3339), endTime.Format(time.RFC3339))

	// Azure supports comma-separated metrics in one call
	metricNames := []string{
		"Percentage CPU",
		"Network In Total",
		"Network Out Total",
		"Disk Read Bytes",
		"Disk Write Bytes",
	}

	// Interval logic: 24 points over period
	intervalSecs := int(period.Seconds() / 24)
	if intervalSecs < 60 {
		intervalSecs = 60
	}
	// Round to multiple of 60
	intervalSecs = (intervalSecs / 60) * 60
	interval := fmt.Sprintf("PT%dM", intervalSecs/60)

	res, err := client.List(ctx, resourceID, &armmonitor.MetricsClientListOptions{
		Metricnames: to.Ptr(strings.Join(metricNames, ",")),
		Timespan:    to.Ptr(timespan),
		Interval:    to.Ptr(interval),
		Aggregation: to.Ptr("Average,Total"),
	})
	if err != nil {
		return nil, err
	}

	for _, m := range res.Value {
		if m.Name == nil || m.Name.Value == nil {
			continue
		}
		name := *m.Name.Value
		unit := ""
		if m.Unit != nil {
			unit = string(*m.Unit)
		}
		if len(m.Timeseries) == 0 {
			continue
		}
		var dps []core.MetricDataPoint
		for _, dp := range m.Timeseries[0].Data {
			val := 0.0
			if name == "Percentage CPU" {
				if dp.Average != nil {
					val = *dp.Average
				} else if dp.Total != nil {
					val = *dp.Total
				}
			} else {
				if dp.Total != nil {
					val = *dp.Total
				} else if dp.Average != nil {
					val = *dp.Average
				}
			}
			if dp.TimeStamp != nil {
				dps = append(dps, core.MetricDataPoint{
					Timestamp: *dp.TimeStamp,
					Value:     val,
				})
			}
		}

		summary := core.BuildSummary(name, unit, dps)
		switch name {
		case "Percentage CPU":
			metrics.CPUUtilization = summary
		case "Network In Total":
			metrics.NetworkInBytes = summary
		case "Network Out Total":
			metrics.NetworkOutBytes = summary
		case "Disk Read Bytes":
			metrics.DiskReadBytes = summary
		case "Disk Write Bytes":
			metrics.DiskWriteBytes = summary
		}
	}

	return metrics, nil
}
