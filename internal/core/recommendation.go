package core

import "time"

// Recommendation represents a cost-saving or optimization suggestion from a cloud provider.
type Recommendation struct {
	ResourceID        string
	ResourceName      string
	ResourceType      string
	Provider          string
	Type              string // "Rightsizing", "Idle", "Scheduling", "Purchase"
	Severity          string // "Critical", "Warning", "Info"
	Summary           string // "Overprovisioned: downsize to t3.micro"
	Detail            string
	EstimatedSavings  float64 // Monthly USD
	CurrentConfig     string  // e.g., "t3.large"
	RecommendedConfig string // e.g., "t3.micro"
	Source            string  // "AWS Compute Optimizer", "GCP Recommender", "Azure Advisor"
	LastUpdated       time.Time
}

// --- Resource interface implementation ---

func (r Recommendation) GetID() string   { return r.ResourceID }
func (r Recommendation) GetName() string { return r.Summary }
func (r Recommendation) GetKind() string { return "Recommendation" }
