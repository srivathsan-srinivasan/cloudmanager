package gcp

import (
	"context"
	"fmt"
	"strings"

	"cloudmanager/internal/core"
	"cloudmanager/internal/logging"
)

func FetchDatabasesSDK(ctx context.Context, project string) ([]core.Database, error) {
	service, err := newSQLAdminService(ctx)
	if err != nil {
		return nil, err
	}

	out, err := service.Instances.List(project).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list cloud sql instances: %w", err)
	}

	var databases []core.Database
	for _, db := range out.Items {
		var labels []string
		for k, v := range db.Settings.UserLabels {
			labels = append(labels, fmt.Sprintf("%s=%s", k, v))
		}

		databases = append(databases, core.Database{
			ID:      db.SelfLink,
			Name:    db.Name,
			Engine:  db.DatabaseVersion,
			Version: db.DatabaseVersion,
			Status:  db.State,
			Region:  db.Region,
			Size:    db.Settings.Tier,
			Labels:  strings.Join(labels, ", "),
		})
	}

	return databases, nil
}

func FetchDatabasesSDKWithCLIAuthFallback(ctx context.Context, project string) ([]core.Database, error) {
	databases, err := FetchDatabasesSDK(ctx, project)
	if err != nil {
		logging.Warnf("component=gcp resource=databases mode=sdk fallback=cli project=%s err=%v", project, err)
		return FetchDatabasesCLI(project)
	}
	return databases, nil
}
