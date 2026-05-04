package gcp

import (
	"encoding/json"
	"fmt"

	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
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

func bucketID(id, name string) string {
	if id != "" {
		return id
	}
	return name
}
