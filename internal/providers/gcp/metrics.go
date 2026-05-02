package gcp

import (
	"context"
	"fmt"
	"sort"
	"time"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/iterator"
	"google.golang.org/protobuf/types/known/timestamppb"

	"cloudmanager/internal/core"
)

func FetchVMMetricsSDK(ctx context.Context, project, zone, instanceID string, period time.Duration) (*core.VMMetrics, error) {
	client, err := newMonitoringMetricClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create monitoring client: %w", err)
	}
	defer client.Close()

	metrics := &core.VMMetrics{
		VMID:      instanceID,
		FetchedAt: time.Now(),
		Period:    period,
	}

	endTime := time.Now()
	startTime := endTime.Add(-period)

	g, ctx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, 5)

	fetchMetric := func(metricType, unit string, multiplier float64) (*core.MetricSummary, error) {
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		defer func() { <-sem }()

		req := &monitoringpb.ListTimeSeriesRequest{
			Name:   "projects/" + project,
			Filter: fmt.Sprintf(`metric.type = "%s" AND resource.labels.instance_id = "%s"`, metricType, instanceID),
			Interval: &monitoringpb.TimeInterval{
				StartTime: timestamppb.New(startTime),
				EndTime:   timestamppb.New(endTime),
			},
			View: monitoringpb.ListTimeSeriesRequest_FULL,
		}

		it := client.ListTimeSeries(ctx, req)

		// Map to aggregate values by timestamp (seconds)
		aggr := make(map[int64]float64)

		for {
			ts, err := it.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return nil, err
			}
			for _, p := range ts.Points {
				val := 0.0
				switch v := p.Value.Value.(type) {
				case *monitoringpb.TypedValue_DoubleValue:
					val = v.DoubleValue
				case *monitoringpb.TypedValue_Int64Value:
					val = float64(v.Int64Value)
				}
				secs := p.Interval.EndTime.Seconds
				aggr[secs] += val * multiplier
			}
		}

		if len(aggr) == 0 {
			return nil, nil
		}

		var dps []core.MetricDataPoint
		for secs, val := range aggr {
			dps = append(dps, core.MetricDataPoint{
				Timestamp: time.Unix(secs, 0),
				Value:     val,
			})
		}

		// Sort by timestamp
		sort.Slice(dps, func(i, j int) bool {
			return dps[i].Timestamp.Before(dps[j].Timestamp)
		})

		return core.BuildSummary(metricType, unit, dps), nil
	}

	g.Go(func() error {
		summary, err := fetchMetric("compute.googleapis.com/instance/cpu/utilization", "Percent", 100.0)
		if err == nil {
			metrics.CPUUtilization = summary
		}
		return nil
	})

	g.Go(func() error {
		summary, err := fetchMetric("agent.googleapis.com/memory/percent_used", "Percent", 1.0)
		if err == nil {
			metrics.MemoryUtilization = summary
		}
		return nil
	})

	g.Go(func() error {
		summary, err := fetchMetric("compute.googleapis.com/instance/disk/read_bytes_count", "Bytes", 1.0)
		if err == nil {
			metrics.DiskReadBytes = summary
		}
		return nil
	})

	g.Go(func() error {
		summary, err := fetchMetric("compute.googleapis.com/instance/disk/write_bytes_count", "Bytes", 1.0)
		if err == nil {
			metrics.DiskWriteBytes = summary
		}
		return nil
	})

	g.Go(func() error {
		summary, err := fetchMetric("compute.googleapis.com/instance/network/received_bytes_count", "Bytes", 1.0)
		if err == nil {
			metrics.NetworkInBytes = summary
		}
		return nil
	})

	g.Go(func() error {
		summary, err := fetchMetric("compute.googleapis.com/instance/network/sent_bytes_count", "Bytes", 1.0)
		if err == nil {
			metrics.NetworkOutBytes = summary
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		metrics.Error = err.Error()
		return metrics, err
	}

	return metrics, nil
}
