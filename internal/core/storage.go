package core

import (
	"fmt"
	"net/url"
	"strings"
)

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

func StorageActions() []Action {
	return []Action{
		{"Describe", "Show storage details", false},
		{"Copy URI", "Copy the bucket/account URI", false},
		{"Copy ID", "Copy the provider storage ID", false},
		{"Copy Console URL", "Copy a provider console URL when CloudManager can infer one", false},
		{"Open Console", "Open this storage resource in the provider console", false},
		{"Tag", "Add CloudManager-only tags", false},
	}
}

func DescribeStorageBucket(ctx CloudContext, s StorageBucket) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Name:           %s\n", s.Name)
	fmt.Fprintf(&b, "ID:             %s\n", s.GetID())
	fmt.Fprintf(&b, "Provider Type:  %s\n", s.ProviderType)
	fmt.Fprintf(&b, "Region:         %s\n", s.Region)
	fmt.Fprintf(&b, "Storage Class:  %s\n", s.StorageClass)
	fmt.Fprintf(&b, "Access:         %s\n", s.Access)
	fmt.Fprintf(&b, "Encrypted:      %s\n", s.Encrypted)
	fmt.Fprintf(&b, "Versioning:     %s\n", s.Versioning)
	fmt.Fprintf(&b, "Created At:     %s\n", s.CreatedAt)
	fmt.Fprintf(&b, "Resource Group: %s\n", s.ResourceGroup)
	fmt.Fprintf(&b, "Labels:         %s\n", s.Labels)
	fmt.Fprintf(&b, "\nContext:        %s\n", ctx.DisplayName())
	fmt.Fprintf(&b, "Provider:       %s\n", ctx.Provider)
	fmt.Fprintf(&b, "Account:        %s\n", ctx.AccountID)
	if uri := StorageURI(ctx, s); uri != "" {
		fmt.Fprintf(&b, "\nURI:            %s\n", uri)
	}
	if consoleURL := StorageConsoleURL(ctx, s); consoleURL != "" {
		fmt.Fprintf(&b, "Console:        %s\n", consoleURL)
	}
	return b.String()
}

func StorageURI(ctx CloudContext, s StorageBucket) string {
	name := strings.TrimSpace(s.Name)
	if name == "" {
		return s.GetID()
	}
	switch strings.ToUpper(strings.TrimSpace(ctx.Provider)) {
	case "AWS":
		return "s3://" + name
	case "GCP":
		return "gs://" + name
	case "AZURE":
		return fmt.Sprintf("https://%s.blob.core.windows.net", name)
	default:
		return s.GetID()
	}
}

func StorageConsoleURL(ctx CloudContext, s StorageBucket) string {
	provider := strings.ToUpper(strings.TrimSpace(ctx.Provider))
	region := firstNonEmpty(s.Region, ctx.Region)
	switch provider {
	case "AWS":
		if s.Name == "" {
			return ""
		}
		if region == "" || strings.EqualFold(region, "unknown") {
			return fmt.Sprintf("https://s3.console.aws.amazon.com/s3/buckets/%s", url.PathEscape(s.Name))
		}
		return fmt.Sprintf("https://s3.console.aws.amazon.com/s3/buckets/%s?region=%s&bucketType=general", url.PathEscape(s.Name), url.QueryEscape(region))
	case "GCP":
		if s.Name == "" || ctx.AccountID == "" {
			return ""
		}
		return fmt.Sprintf("https://console.cloud.google.com/storage/browser/%s?project=%s", url.PathEscape(s.Name), url.QueryEscape(ctx.AccountID))
	case "AZURE":
		if strings.HasPrefix(s.ID, "/subscriptions/") {
			return azurePortalResourceURL(ctx, s.ID)
		}
	}
	return ""
}
