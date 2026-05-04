package aws

import (
	"context"
	"fmt"
	"sort"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"golang.org/x/sync/errgroup"

	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
)

func FetchVMMetricsSDK(ctx context.Context, profile, region, instanceID string, period time.Duration) (*core.VMMetrics, error) {
	cfg, err := getAWSConfig(ctx, profile, region)
	if err != nil {
		return nil, fmt.Errorf("failed to load aws config: %w", err)
	}
	client := cloudwatch.NewFromConfig(cfg)

	metrics := &core.VMMetrics{
		VMID:      instanceID,
		FetchedAt: time.Now(),
		Period:    period,
	}

	endTime := time.Now()
	startTime := endTime.Add(-period)

	// AWS requires period to be a multiple of 60 for > 3 hours ago
	// 24 points over 24 hours means 1 hour period (3600s)
	intervalSecs := int32(period.Seconds() / 24)
	if intervalSecs < 60 {
		intervalSecs = 60
	}
	// Round to multiple of 60
	intervalSecs = (intervalSecs / 60) * 60

	g, ctx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, 5)

	fetchMetric := func(namespace, metricName string, unit types.StandardUnit, stat types.Statistic) (*core.MetricSummary, error) {
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		defer func() { <-sem }()

		input := &cloudwatch.GetMetricStatisticsInput{
			Namespace:  awssdk.String(namespace),
			MetricName: awssdk.String(metricName),
			Dimensions: []types.Dimension{
				{
					Name:  awssdk.String("InstanceId"),
					Value: awssdk.String(instanceID),
				},
			},
			StartTime:  awssdk.Time(startTime),
			EndTime:    awssdk.Time(endTime),
			Period:     awssdk.Int32(intervalSecs),
			Statistics: []types.Statistic{stat},
			Unit:       unit,
		}

		resp, err := client.GetMetricStatistics(ctx, input)
		if err != nil {
			return nil, err
		}

		if len(resp.Datapoints) == 0 {
			return nil, nil
		}

		// Convert to core.MetricDataPoint
		var dps []core.MetricDataPoint
		for _, dp := range resp.Datapoints {
			val := 0.0
			switch stat {
			case types.StatisticAverage:
				val = awssdk.ToFloat64(dp.Average)
			case types.StatisticMaximum:
				val = awssdk.ToFloat64(dp.Maximum)
			case types.StatisticMinimum:
				val = awssdk.ToFloat64(dp.Minimum)
			case types.StatisticSum:
				val = awssdk.ToFloat64(dp.Sum)
			}
			dps = append(dps, core.MetricDataPoint{
				Timestamp: awssdk.ToTime(dp.Timestamp),
				Value:     val,
			})
		}

		// Sort by timestamp
		sort.Slice(dps, func(i, j int) bool {
			return dps[i].Timestamp.Before(dps[j].Timestamp)
		})

		return core.BuildSummary(metricName, string(unit), dps), nil
	}

	g.Go(func() error {
		summary, err := fetchMetric("AWS/EC2", "CPUUtilization", types.StandardUnitPercent, types.StatisticAverage)
		if err == nil {
			metrics.CPUUtilization = summary
		}
		return nil
	})

	g.Go(func() error {
		// Memory requires CloudWatch Agent (CWAgent namespace)
		summary, err := fetchMetric("CWAgent", "mem_used_percent", types.StandardUnitPercent, types.StatisticAverage)
		if err == nil {
			metrics.MemoryUtilization = summary
		}
		return nil
	})

	g.Go(func() error {
		summary, err := fetchMetric("AWS/EC2", "DiskReadBytes", types.StandardUnitBytes, types.StatisticSum)
		if err == nil {
			metrics.DiskReadBytes = summary
		}
		return nil
	})

	g.Go(func() error {
		summary, err := fetchMetric("AWS/EC2", "DiskWriteBytes", types.StandardUnitBytes, types.StatisticSum)
		if err == nil {
			metrics.DiskWriteBytes = summary
		}
		return nil
	})

	g.Go(func() error {
		summary, err := fetchMetric("AWS/EC2", "NetworkIn", types.StandardUnitBytes, types.StatisticSum)
		if err == nil {
			metrics.NetworkInBytes = summary
		}
		return nil
	})

	g.Go(func() error {
		summary, err := fetchMetric("AWS/EC2", "NetworkOut", types.StandardUnitBytes, types.StatisticSum)
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
