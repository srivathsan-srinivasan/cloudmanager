package core

import (
	"fmt"
	"time"
)

// ResourceCost represents the cost of a single resource.
type ResourceCost struct {
	ResourceID        string
	ResourceName      string
	ResourceType      string
	Provider          string
	CurrentMonthCost  float64
	PreviousMonthCost float64
	ForecastedCost    float64
	Currency          string
	LastUpdated       time.Time
}

// ServiceCost represents the cost of a specific service within an account.
type ServiceCost struct {
	ServiceName string
	Cost        float64
}

// AccountCost represents the aggregate cost for a cloud account/subscription.
type AccountCost struct {
	Provider          string
	AccountID         string
	AccountName       string
	CurrentMonthCost  float64
	PreviousMonthCost float64
	ForecastedCost    float64
	TopServices       []ServiceCost
	LastUpdated       time.Time
}

// FormatCost formats a float amount as a string, e.g., "$1,234.56" or "$12.3K"
func FormatCost(amount float64) string {
	if amount >= 1000 {
		return fmt.Sprintf("$%.1fK", amount/1000)
	}
	return fmt.Sprintf("$%.2f", amount)
}

// CostChangePercent calculates the percentage change between current and previous costs.
func CostChangePercent(current, previous float64) string {
	if previous == 0 {
		return "+0.0%"
	}
	change := ((current - previous) / previous) * 100
	if change >= 0 {
		return fmt.Sprintf("+%.1f%%", change)
	}
	return fmt.Sprintf("%.1f%%", change)
}
