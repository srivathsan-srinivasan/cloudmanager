package gcp

import (
	"context"
	"cloudmanager/internal/core"
)

func FetchDatabasesCLI(project string) ([]core.Database, error) { return nil, nil }
func FetchDatabasesSDK(ctx context.Context, project string) ([]core.Database, error) { return nil, nil }
func FetchDatabasesSDKWithCLIAuthFallback(ctx context.Context, project string) ([]core.Database, error) { return nil, nil }
