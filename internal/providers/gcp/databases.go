package gcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/vyoogam/cloudmanager/internal/core"
	"github.com/vyoogam/cloudmanager/internal/logging"
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
		tier := ""
		activationPolicy := ""
		if db.Settings != nil {
			tier = db.Settings.Tier
			activationPolicy = db.Settings.ActivationPolicy
			for k, v := range db.Settings.UserLabels {
				labels = append(labels, fmt.Sprintf("%s=%s", k, v))
			}
		}

		databases = append(databases, core.Database{
			ID:      db.SelfLink,
			Name:    db.Name,
			Engine:  db.DatabaseVersion,
			Version: db.DatabaseVersion,
			Status:  gcpCloudSQLDisplayStatus(db.State, activationPolicy),
			Region:  db.Region,
			Size:    tier,
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

func gcpCloudSQLDisplayStatus(state, activationPolicy string) string {
	state = strings.ToUpper(strings.TrimSpace(state))
	activationPolicy = strings.ToUpper(strings.TrimSpace(activationPolicy))
	if state == "" {
		return "-"
	}
	if state == "RUNNABLE" {
		if activationPolicy == "NEVER" {
			return "STOPPED"
		}
		return "RUNNING"
	}
	if state == "SUSPENDED" {
		return "STOPPED"
	}
	return state
}
