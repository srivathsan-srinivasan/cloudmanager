package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/vyoogam/cloudmanager/internal/core"
)

type awsBucketList struct {
	Buckets []struct {
		Name         string `json:"Name"`
		CreationDate string `json:"CreationDate"`
	} `json:"Buckets"`
}

func FetchStorageBucketsCLI(ctx context.Context, profile string) ([]core.StorageBucket, error) {
	args := []string{"--no-cli-pager", "s3api", "list-buckets", "--output", "json"}
	if strings.TrimSpace(profile) != "" {
		args = append(args, "--profile", profile)
	}
	cmd := exec.CommandContext(ctx, "aws", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("aws s3api list-buckets failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	var data awsBucketList
	if err := json.Unmarshal(output, &data); err != nil {
		return nil, fmt.Errorf("failed to parse aws buckets: %w", err)
	}
	buckets := make([]core.StorageBucket, 0, len(data.Buckets))
	for _, bucket := range data.Buckets {
		buckets = append(buckets, core.StorageBucket{
			Name:         bucket.Name,
			ID:           bucket.Name,
			ProviderType: "S3 bucket",
			Region:       "global",
			Access:       "unknown",
			Encrypted:    "unknown",
			Versioning:   "unknown",
			CreatedAt:    bucket.CreationDate,
		})
	}
	return buckets, nil
}
