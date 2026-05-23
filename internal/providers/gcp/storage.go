package gcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vyoogam/cloudmanager/internal/core"
	"github.com/vyoogam/cloudmanager/internal/logging"
)

type gcpBucketCLI struct {
	Name             string            `json:"name"`
	ID               string            `json:"id"`
	Location         string            `json:"location"`
	StorageClass     string            `json:"storageClass"`
	TimeCreated      string            `json:"timeCreated"`
	Labels           map[string]string `json:"labels"`
	IAMConfiguration struct {
		PublicAccessPrevention string `json:"publicAccessPrevention"`
	} `json:"iamConfiguration"`
	Encryption struct {
		DefaultKMSKeyName string `json:"defaultKmsKeyName"`
	} `json:"encryption"`
	Versioning struct {
		Enabled bool `json:"enabled"`
	} `json:"versioning"`
}

func FetchStorageBucketsCLI(project string) ([]core.StorageBucket, error) {
	output, err := runGCloudJSON("storage", "buckets", "list", "--project", project)
	if err != nil {
		return nil, err
	}
	var data []gcpBucketCLI
	if err := json.Unmarshal(output, &data); err != nil {
		return nil, fmt.Errorf("failed to parse gcp storage buckets: %w", err)
	}
	buckets := make([]core.StorageBucket, 0, len(data))
	for _, bucket := range data {
		encrypted := "default"
		if bucket.Encryption.DefaultKMSKeyName != "" {
			encrypted = "kms"
		}
		versioning := "off"
		if bucket.Versioning.Enabled {
			versioning = "on"
		}
		access := bucket.IAMConfiguration.PublicAccessPrevention
		if access == "" {
			access = "unknown"
		}
		buckets = append(buckets, core.StorageBucket{
			Name:         bucket.Name,
			ID:           bucketID(bucket.ID, bucket.Name),
			ProviderType: "Cloud Storage bucket",
			Region:       bucket.Location,
			StorageClass: bucket.StorageClass,
			Access:       access,
			Encrypted:    encrypted,
			Versioning:   versioning,
			CreatedAt:    bucket.TimeCreated,
			Labels:       joinLabelMap(bucket.Labels),
		})
	}
	return buckets, nil
}

func FetchStorageBucketsCLIWithSDKFallback(ctx context.Context, project string) ([]core.StorageBucket, error) {
	buckets, err := FetchStorageBucketsCLI(project)
	if err == nil {
		return buckets, nil
	}
	logging.Warnf("component=gcp resource=storage mode=cli fallback=sdk project=%s err=%v", project, err)
	sdkBuckets, sdkErr := FetchStorageBucketsSDK(ctx, project)
	if sdkErr != nil {
		return nil, fmt.Errorf("%w. SDK fallback also failed: %v", err, sdkErr)
	}
	return sdkBuckets, nil
}

func FetchStorageBucketsSDK(ctx context.Context, project string) ([]core.StorageBucket, error) {
	service, err := newStorageService(ctx)
	if err != nil {
		return nil, err
	}
	out, err := service.Buckets.List(project).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list gcp storage buckets: %w", err)
	}
	buckets := make([]core.StorageBucket, 0, len(out.Items))
	for _, bucket := range out.Items {
		encrypted := "default"
		if bucket.Encryption != nil && bucket.Encryption.DefaultKmsKeyName != "" {
			encrypted = "kms"
		}
		versioning := "off"
		if bucket.Versioning != nil && bucket.Versioning.Enabled {
			versioning = "on"
		}
		access := "unknown"
		if bucket.IamConfiguration != nil && bucket.IamConfiguration.PublicAccessPrevention != "" {
			access = bucket.IamConfiguration.PublicAccessPrevention
		}
		buckets = append(buckets, core.StorageBucket{
			Name:         bucket.Name,
			ID:           bucketID(bucket.Id, bucket.Name),
			ProviderType: "Cloud Storage bucket",
			Region:       bucket.Location,
			StorageClass: bucket.StorageClass,
			Access:       access,
			Encrypted:    encrypted,
			Versioning:   versioning,
			CreatedAt:    bucket.TimeCreated,
			Labels:       joinLabelMap(bucket.Labels),
		})
	}
	return buckets, nil
}

func FetchStorageBucketsSDKWithCLIAuthFallback(ctx context.Context, project string) ([]core.StorageBucket, error) {
	buckets, err := FetchStorageBucketsSDK(ctx, project)
	if err == nil {
		return buckets, nil
	}
	logging.Warnf("component=gcp resource=storage mode=sdk fallback=cli project=%s err=%v", project, err)
	return FetchStorageBucketsCLI(project)
}

func bucketID(id, name string) string {
	if id != "" {
		return id
	}
	return name
}
