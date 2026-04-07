package core

import (
	"testing"
	"time"
)

func TestFormatMetricValue(t *testing.T) {
	tests := []struct {
		value    float64
		unit     string
		expected string
	}{
		{72.34, "Percent", "72.3%"},
		{512, "Bytes", "512 B"},
		{2048, "Bytes", "2.0 KB"},
		{1048576, "Bytes", "1.0 MB"},
		{1073741824, "Bytes", "1.0 GB"},
		{1024, "Bytes/Second", "1.0 KB/s"},
		{340.5, "Count/Second", "340.5 IOPS"},
		{10.5, "Count", "10.5 Count"},
	}

	for _, tt := range tests {
		result := FormatMetricValue(tt.value, tt.unit)
		if result != tt.expected {
			t.Errorf("FormatMetricValue(%f, %s) = %s; want %s", tt.value, tt.unit, result, tt.expected)
		}
	}
}

func TestBuildSummary(t *testing.T) {
	now := time.Now()
	datapoints := []MetricDataPoint{
		{Timestamp: now.Add(-2 * time.Hour), Value: 10.0},
		{Timestamp: now.Add(-1 * time.Hour), Value: 30.0},
		{Timestamp: now, Value: 20.0},
	}

	summary := BuildSummary("CPU", "Percent", datapoints)

	if summary == nil {
		t.Fatal("BuildSummary returned nil")
	}
	if summary.Average != 20.0 {
		t.Errorf("Expected average 20.0, got %f", summary.Average)
	}
	if summary.Minimum != 10.0 {
		t.Errorf("Expected minimum 10.0, got %f", summary.Minimum)
	}
	if summary.Maximum != 30.0 {
		t.Errorf("Expected maximum 30.0, got %f", summary.Maximum)
	}
	if summary.Current != 20.0 {
		t.Errorf("Expected current 20.0, got %f", summary.Current)
	}
	if len(summary.DataPoints) != 3 {
		t.Errorf("Expected 3 datapoints, got %d", len(summary.DataPoints))
	}

	emptySummary := BuildSummary("CPU", "Percent", []MetricDataPoint{})
	if emptySummary != nil {
		t.Error("Expected nil for empty datapoints")
	}
}
