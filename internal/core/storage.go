package core

import "strings"

var DefaultStorageColumns = []string{"Name", "Provider Type", "Region", "Access", "Encrypted", "Versioning", "Created At"}

type StorageBucket struct {
	Name          string
	ID            string
	ProviderType  string
	Region        string
	StorageClass  string
	Access        string
	Encrypted     string
	Versioning    string
	CreatedAt     string
	ResourceGroup string
	Labels        string
}

func (s StorageBucket) GetID() string {
	if strings.TrimSpace(s.ID) != "" {
		return s.ID
	}
	return s.Name
}

func (s StorageBucket) GetName() string { return s.Name }
func (s StorageBucket) GetKind() string { return "Storage" }

func (s StorageBucket) GetField(col string) string {
	switch col {
	case "Name":
		return s.Name
	case "ID":
		return s.GetID()
	case "Provider Type":
		return s.ProviderType
	case "Region":
		return s.Region
	case "Storage Class":
		return s.StorageClass
	case "Access":
		return s.Access
	case "Encrypted":
		return s.Encrypted
	case "Versioning":
		return s.Versioning
	case "Created At":
		return s.CreatedAt
	case "Resource Group":
		return s.ResourceGroup
	case "Labels":
		return s.Labels
	default:
		return "-"
	}
}
