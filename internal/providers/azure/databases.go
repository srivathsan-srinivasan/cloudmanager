package azure

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers"

	"cloudmanager/internal/core"
)

func FetchDatabasesCLI(subscription string) ([]core.Database, error) { return nil, nil }

func FetchDatabasesSDK(ctx context.Context, subscription string) ([]core.Database, error) {
	cred, err := getAzureCreds()
	if err != nil {
		return nil, err
	}
	client, err := armpostgresqlflexibleservers.NewServersClient(subscription, cred, nil)
	if err != nil {
		return nil, err
	}

	pager := client.NewListPager(nil)
	var databases []core.Database

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list postgresql servers: %w", err)
		}
		for _, db := range page.Value {
			version := "-"
			if db.Properties != nil && db.Properties.Version != nil {
				version = string(*db.Properties.Version)
			}
			status := "-"
			if db.Properties != nil && db.Properties.State != nil {
				status = string(*db.Properties.State)
			}
			size := "-"
			if db.SKU != nil && db.SKU.Name != nil {
				size = *db.SKU.Name
			}

			var labels []string
			if db.Tags != nil {
				for k, v := range db.Tags {
					labels = append(labels, fmt.Sprintf("%s=%s", k, *v))
				}
			}

			databases = append(databases, core.Database{
				ID:      orPtr(db.ID),
				Name:    orPtr(db.Name),
				Engine:  "PostgreSQL",
				Version: version,
				Status:  status,
				Region:  orPtr(db.Location),
				Size:    size,
				Labels:  strings.Join(labels, ", "),
			})
		}
	}

	return databases, nil
}
