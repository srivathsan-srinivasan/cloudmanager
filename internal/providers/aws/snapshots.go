package aws

import (
	"context"
	"fmt"
	"strings"

	"cloudmanager/internal/core"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

// FetchSnapshotsSDK fetches EBS snapshots using the AWS SDK.
func FetchSnapshotsSDK(ctx context.Context, profile, region string) ([]core.Snapshot, error) {
	cfg, err := getAWSConfig(ctx, profile, region)
	if err != nil {
		return nil, fmt.Errorf("failed to load aws config: %w", err)
	}

	client := ec2.NewFromConfig(cfg)
	paginator := ec2.NewDescribeSnapshotsPaginator(client, &ec2.DescribeSnapshotsInput{
		OwnerIds: []string{"self"},
	})

	var snapshots []core.Snapshot

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe snapshots: %w", err)
		}

		for _, snap := range page.Snapshots {
			name := "-"
			var labelPairs []string
			for _, tag := range snap.Tags {
				if aws.ToString(tag.Key) == "Name" {
					name = aws.ToString(tag.Value)
				}
				labelPairs = append(labelPairs, fmt.Sprintf("%s=%s", aws.ToString(tag.Key), aws.ToString(tag.Value)))
			}

			createdAt := "-"
			if snap.StartTime != nil {
				createdAt = snap.StartTime.Format("2006-01-02T15:04:05Z")
			}

			snapshots = append(snapshots, core.Snapshot{
				Name:           name,
				ID:             aws.ToString(snap.SnapshotId),
				State:          string(snap.State),
				SizeGB:         int(aws.ToInt32(snap.VolumeSize)),
				SourceDiskName: "-", // Would need to fetch volume tags to get name
				SourceDiskID:   aws.ToString(snap.VolumeId),
				CreatedAt:      createdAt,
				Zone:           region, // Snapshots are regional in AWS
				Description:    aws.ToString(snap.Description),
				Labels:         strings.Join(labelPairs, ", "),
			})
		}
	}

	return snapshots, nil
}

// ExecuteSnapshotActionSDK executes an action on an EBS snapshot.
func ExecuteSnapshotActionSDK(ctx context.Context, action string, snap core.Snapshot, cloudCtx core.CloudContext) (string, error) {
	cfg, err := getAWSConfig(ctx, cloudCtx.AuthRef(), cloudCtx.Region)
	if err != nil {
		return "", fmt.Errorf("failed to load aws config: %w", err)
	}

	client := ec2.NewFromConfig(cfg)

	switch action {
	case "Delete":
		_, err = client.DeleteSnapshot(ctx, &ec2.DeleteSnapshotInput{
			SnapshotId: aws.String(snap.ID),
		})
	case "Describe":
		resp, descErr := client.DescribeSnapshots(ctx, &ec2.DescribeSnapshotsInput{
			SnapshotIds: []string{snap.ID},
		})
		if descErr != nil {
			return "", descErr
		}
		if len(resp.Snapshots) > 0 {
			s := resp.Snapshots[0]
			return fmt.Sprintf("Snapshot ID: %s\nState: %s\nSize: %d GB\nSource Volume: %s\nDescription: %s\n",
				aws.ToString(s.SnapshotId), s.State, aws.ToInt32(s.VolumeSize),
				aws.ToString(s.VolumeId), aws.ToString(s.Description),
			), nil
		}
		return "", fmt.Errorf("snapshot not found")
	default:
		// Handle "Create Disk:name" or similar if needed.
		parts := strings.Split(action, ":")
		if len(parts) == 2 && parts[0] == "Create Disk" {
			az := cloudCtx.Region + "a" // Default to 'a' zone, ideally passed in
			_, err = client.CreateVolume(ctx, &ec2.CreateVolumeInput{
				SnapshotId:       aws.String(snap.ID),
				AvailabilityZone: aws.String(az),
				// Add tags for name if needed
			})
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("Successfully initiated disk creation from %s", snap.ID), nil
		}
		return "", fmt.Errorf("action %s not supported for AWS Snapshots", action)
	}

	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Successfully executed '%s' on %s", action, snap.ID), nil
}
