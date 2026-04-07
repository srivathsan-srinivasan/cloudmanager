package aws

import (
	"context"
	"cloudmanager/internal/core"
)

func FetchDatabasesCLI(profile, region string) ([]core.Database, error) { return nil, nil }
func FetchDatabasesSDK(ctx context.Context, profile, region string) ([]core.Database, error) { return nil, nil }
