package core

import (
	"fmt"
	"time"
)

// Metric name constants
const (
	MetricCPU        = "CPUUtilization"
	MetricMemory     = "MemoryUtilization"
	MetricDiskRead   = "DiskReadBytes"
	MetricDiskWrite  = "DiskWriteBytes"
	MetricNetworkIn  = "NetworkInBytes"
	MetricNetworkOut = "NetworkOutBytes"
)

// MetricDataPoint represents a single metric value at a point in time.
type MetricDataPoint struct {
	Timestamp time.Time
	Value     float64
}

// MetricSummary provides aggregated statistics and recent history for a metric.
type MetricSummary struct {
	MetricName string
	Unit       string
	Average    float64
	Maximum    float64
	Minimum    float64
	Current    float64
	DataPoints []MetricDataPoint
}

// VMMetrics aggregates all monitored metrics for a single VM.
type VMMetrics struct {
	VMID, VMName      string
	CPUUtilization    *MetricSummary
	MemoryUtilization *MetricSummary
	DiskReadBytes     *MetricSummary
	DiskWriteBytes    *MetricSummary
	NetworkInBytes    *MetricSummary
	NetworkOutBytes   *MetricSummary
	FetchedAt         time.Time
	Period            time.Duration
	Error             string
}

// FormatMetricValue returns a human-readable string for a metric value and unit.
func FormatMetricValue(value float64, unit string) string {
	if unit == "Percent" {
		return fmt.Sprintf("%.1f%%", value)
	}
	if unit == "Bytes" || unit == "Bytes/Second" {
		suffix := ""
		if unit == "Bytes/Second" {
			suffix = "/s"
		}
		if value < 1024 {
			return fmt.Sprintf("%.0f B%s", value, suffix)
		}
		if value < 1024*1024 {
			return fmt.Sprintf("%.1f KB%s", value/1024, suffix)
		}
		if value < 1024*1024*1024 {
			return fmt.Sprintf("%.1f MB%s", value/(1024*1024), suffix)
		}
		return fmt.Sprintf("%.1f GB%s", value/(1024*1024*1024), suffix)
	}
	if unit == "Count/Second" {
		return fmt.Sprintf("%.1f IOPS", value)
	}
	return fmt.Sprintf("%.1f %s", value, unit)
}

// BuildSummary creates a MetricSummary from raw data points.
func BuildSummary(name, unit string, datapoints []MetricDataPoint) *MetricSummary {
	if len(datapoints) == 0 {
		return nil
	}
	summary := &MetricSummary{
		MetricName: name,
		Unit:       unit,
		DataPoints: datapoints,
	}
	var sum float64
	summary.Minimum = datapoints[0].Value
	summary.Maximum = datapoints[0].Value
	summary.Current = datapoints[len(datapoints)-1].Value

	for _, dp := range datapoints {
		sum += dp.Value
		if dp.Value < summary.Minimum {
			summary.Minimum = dp.Value
		}
		if dp.Value > summary.Maximum {
			summary.Maximum = dp.Value
		}
	}
	summary.Average = sum / float64(len(datapoints))
	return summary
}
