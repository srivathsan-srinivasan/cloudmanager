package core

import (
	"fmt"
	"net/url"
	"strings"
)

var DefaultDatabaseColumns = []string{"Name", "Engine", "Version", "Status", "Region", "Size"}

type Database struct {
	Name    string
	ID      string
	Engine  string
	Version string
	Status  string
	Region  string
	Size    string
	Labels  string
}

func (d Database) GetID() string   { return d.ID }
func (d Database) GetName() string { return d.Name }
func (d Database) GetKind() string { return "Database" }
func (d Database) GetField(col string) string {
	switch col {
	case "Name":
		return d.Name
	case "ID":
		return d.ID
	case "Engine":
		return d.Engine
	case "Version":
		return d.Version
	case "Status":
		return d.Status
	case "Region":
		return d.Region
	case "Size":
		return d.Size
	case "Labels":
		return d.Labels
	default:
		return "-"
	}
}

func DatabaseActions() []Action {
	return []Action{
		{"Describe", "Show database details", false},
		{"Copy ID", "Copy the provider database ID", false},
		{"Copy Console URL", "Copy a provider console URL when CloudManager can infer one", false},
		{"Tag", "Add CloudManager-only tags", false},
	}
}

func DescribeDatabase(ctx CloudContext, d Database) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Name:      %s\n", d.Name)
	fmt.Fprintf(&b, "ID:        %s\n", d.ID)
	fmt.Fprintf(&b, "Engine:    %s\n", d.Engine)
	fmt.Fprintf(&b, "Version:   %s\n", d.Version)
	fmt.Fprintf(&b, "Status:    %s\n", d.Status)
	fmt.Fprintf(&b, "Region:    %s\n", d.Region)
	fmt.Fprintf(&b, "Size:      %s\n", d.Size)
	fmt.Fprintf(&b, "Labels:    %s\n", d.Labels)
	fmt.Fprintf(&b, "\nContext:   %s\n", ctx.DisplayName())
	fmt.Fprintf(&b, "Provider:  %s\n", ctx.Provider)
	fmt.Fprintf(&b, "Account:   %s\n", ctx.AccountID)
	if ctx.Tenant != "" {
		fmt.Fprintf(&b, "Tenant:    %s\n", ctx.Tenant)
	}
	if consoleURL := DatabaseConsoleURL(ctx, d); consoleURL != "" {
		fmt.Fprintf(&b, "\nConsole:   %s\n", consoleURL)
	}
	return b.String()
}

func DatabaseConsoleURL(ctx CloudContext, d Database) string {
	provider := strings.ToUpper(strings.TrimSpace(ctx.Provider))
	region := firstNonEmpty(d.Region, ctx.Region)
	id := firstNonEmpty(d.ID, d.Name)
	switch provider {
	case "AWS":
		dbIdentifier := firstNonEmpty(d.Name, d.ID)
		if region == "" || dbIdentifier == "" {
			return ""
		}
		return fmt.Sprintf("https://%s.console.aws.amazon.com/rds/home?region=%s#database:id=%s;is-cluster=false", url.QueryEscape(region), url.QueryEscape(region), url.QueryEscape(dbIdentifier))
	case "GCP":
		if d.Name == "" || ctx.AccountID == "" {
			return ""
		}
		return fmt.Sprintf("https://console.cloud.google.com/sql/instances/%s/overview?project=%s", url.PathEscape(d.Name), url.QueryEscape(ctx.AccountID))
	case "AZURE":
		if strings.HasPrefix(id, "/subscriptions/") {
			return azurePortalResourceURL(ctx, id)
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func azurePortalResourceURL(ctx CloudContext, resourceID string) string {
	if strings.TrimSpace(resourceID) == "" {
		return ""
	}
	tenant := strings.TrimSpace(ctx.Tenant)
	if tenant != "" {
		return fmt.Sprintf("https://portal.azure.com/#@%s/resource%s", url.PathEscape(tenant), resourceID)
	}
	return fmt.Sprintf("https://portal.azure.com/#resource%s", resourceID)
}
