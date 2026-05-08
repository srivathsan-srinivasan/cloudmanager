package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
)

type azureStorageAccountCLI struct {
	Name               string            `json:"name"`
	ID                 string            `json:"id"`
	Location           string            `json:"location"`
	Kind               string            `json:"kind"`
	ResourceGroup      string            `json:"resourceGroup"`
	Tags               map[string]string `json:"tags"`
	AllowBlobPublic    *bool             `json:"allowBlobPublicAccess"`
	EnableHTTPSTraffic *bool             `json:"enableHttpsTrafficOnly"`
	Encryption         struct {
		KeySource string `json:"keySource"`
	} `json:"encryption"`
}

type azureBlobContainerCLI struct {
	Name       string            `json:"name"`
	ID         string            `json:"id"`
	Metadata   map[string]string `json:"metadata"`
	Properties struct {
		PublicAccess string `json:"publicAccess"`
		LastModified string `json:"lastModified"`
		LeaseState   string `json:"leaseState"`
	} `json:"properties"`
}

func FetchStorageAccountsCLI(ctx context.Context, subscriptionID string) ([]core.StorageBucket, error) {
	cmd := exec.CommandContext(ctx, "az", "storage", "account", "list", "--subscription", subscriptionID, "--output", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("az storage account list failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	var data []azureStorageAccountCLI
	if err := json.Unmarshal(output, &data); err != nil {
		return nil, fmt.Errorf("failed to parse azure storage accounts: %w", err)
	}
	buckets := make([]core.StorageBucket, 0, len(data))
	for _, account := range data {
		access := "unknown"
		if account.AllowBlobPublic != nil {
			if *account.AllowBlobPublic {
				access = "public allowed"
			} else {
				access = "public blocked"
			}
		}
		encrypted := account.Encryption.KeySource
		if encrypted == "" {
			encrypted = "default"
		}
		buckets = append(buckets, core.StorageBucket{
			Name:          account.Name,
			ID:            account.ID,
			ProviderType:  "Storage account",
			Region:        account.Location,
			Access:        access,
			Encrypted:     encrypted,
			Versioning:    "unknown",
			ResourceGroup: account.ResourceGroup,
			Labels:        joinStringMap(account.Tags),
		})
		containers, err := fetchBlobContainersCLI(ctx, subscriptionID, account)
		if err == nil {
			buckets = append(buckets, containers...)
		}
	}
	return buckets, nil
}

func fetchBlobContainersCLI(ctx context.Context, subscriptionID string, account azureStorageAccountCLI) ([]core.StorageBucket, error) {
	cmd := exec.CommandContext(ctx, "az", "storage", "container", "list",
		"--account-name", account.Name,
		"--auth-mode", "login",
		"--subscription", subscriptionID,
		"--output", "json",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("az storage container list failed for %s: %w: %s", account.Name, err, strings.TrimSpace(string(output)))
	}
	var data []azureBlobContainerCLI
	if err := json.Unmarshal(output, &data); err != nil {
		return nil, fmt.Errorf("failed to parse azure blob containers for %s: %w", account.Name, err)
	}
	containers := make([]core.StorageBucket, 0, len(data))
	for _, container := range data {
		access := container.Properties.PublicAccess
		if access == "" {
			access = "private"
		}
		id := container.ID
		if id == "" {
			id = fmt.Sprintf("%s/blobServices/default/containers/%s", strings.TrimRight(account.ID, "/"), container.Name)
		}
		containers = append(containers, core.StorageBucket{
			Name:          fmt.Sprintf("%s/%s", account.Name, container.Name),
			ID:            id,
			ProviderType:  "Blob container",
			Region:        account.Location,
			Access:        access,
			Encrypted:     account.Encryption.KeySource,
			Versioning:    "account-level",
			CreatedAt:     container.Properties.LastModified,
			ResourceGroup: account.ResourceGroup,
			Labels:        joinStringMap(container.Metadata),
		})
	}
	return containers, nil
}

func joinStringMap(values map[string]string) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, 0, len(values))
	for k, v := range values {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, ", ")
}
