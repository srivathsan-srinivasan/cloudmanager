package aws

import (
	"context"
	"encoding/json"
	"fmt"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"

	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
)

func FetchDatabasesCLI(profile, region string) ([]core.Database, error) {
	out, err := runAwsJSON(profile, region, "rds", "describe-db-instances")
	if err != nil {
		return nil, err
	}

	var data struct {
		DBInstances []struct {
			DBInstanceIdentifier string `json:"DBInstanceIdentifier"`
			DBInstanceArn        string `json:"DBInstanceArn"`
			Engine               string `json:"Engine"`
			EngineVersion        string `json:"EngineVersion"`
			DBInstanceStatus     string `json:"DBInstanceStatus"`
			DBInstanceClass      string `json:"DBInstanceClass"`
		} `json:"DBInstances"`
	}
	if err := json.Unmarshal(out, &data); err != nil {
		return nil, err
	}

	var databases []core.Database
	for _, db := range data.DBInstances {
		databases = append(databases, core.Database{
			ID:      db.DBInstanceArn,
			Name:    db.DBInstanceIdentifier,
			Engine:  db.Engine,
			Version: db.EngineVersion,
			Status:  db.DBInstanceStatus,
			Region:  region,
			Size:    db.DBInstanceClass,
		})
	}
	return databases, nil
}

func FetchDatabasesSDK(ctx context.Context, profile, region string) ([]core.Database, error) {
	cfg, err := getAWSConfig(ctx, profile, region)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	out, err := client.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe rds instances: %w", err)
	}

	var databases []core.Database
	for _, db := range out.DBInstances {
		name := awssdk.ToString(db.DBInstanceIdentifier)
		engine := awssdk.ToString(db.Engine)
		version := awssdk.ToString(db.EngineVersion)
		status := awssdk.ToString(db.DBInstanceStatus)
		size := awssdk.ToString(db.DBInstanceClass)

		databases = append(databases, core.Database{
			ID:      awssdk.ToString(db.DBInstanceArn),
			Name:    name,
			Engine:  engine,
			Version: version,
			Status:  status,
			Region:  region,
			Size:    size,
		})
	}

	return databases, nil
}
