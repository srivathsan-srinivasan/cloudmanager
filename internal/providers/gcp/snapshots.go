package gcp

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/api/compute/v1"
	"cloudmanager/internal/core"
)

// FetchSnapshotsSDK fetches snapshots using the GCP SDK.
func FetchSnapshotsSDK(ctx context.Context, project string) ([]core.Snapshot, error) {
	service, err := compute.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create compute service: %w", err)
	}

	req := service.Snapshots.List(project)
	var snapshots []core.Snapshot

	if err := req.Pages(ctx, func(page *compute.SnapshotList) error {
		for _, snap := range page.Items {
			// Parse Source Disk
			sourceDiskName := "-"
			if snap.SourceDisk != "" {
				parts := strings.Split(snap.SourceDisk, "/")
				sourceDiskName = parts[len(parts)-1]
			}

			// Labels
			var labelPairs []string
			for k, v := range snap.Labels {
				labelPairs = append(labelPairs, fmt.Sprintf("%s=%s", k, v))
			}

			snapshots = append(snapshots, core.Snapshot{
				Name:           snap.Name,
				ID:             fmt.Sprintf("%d", snap.Id),
				State:          snap.Status,
				SizeGB:         int(snap.DiskSizeGb),
				SourceDiskName: sourceDiskName,
				SourceDiskID:   sourceDiskName,
				CreatedAt:      snap.CreationTimestamp,
				Zone:           "global", // Snapshots are global in GCP by default
				Description:    snap.Description,
				Labels:         strings.Join(labelPairs, ", "),
			})
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to list snapshots: %w", err)
	}

	return snapshots, nil
}

// ExecuteSnapshotActionSDK executes an action on a GCP snapshot.
func ExecuteSnapshotActionSDK(ctx context.Context, action string, snap core.Snapshot, cloudCtx core.CloudContext) (string, error) {
	service, err := compute.NewService(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to create compute service: %w", err)
	}

	project := cloudCtx.AccountID

	switch action {
	case "Delete":
		_, err = service.Snapshots.Delete(project, snap.Name).Context(ctx).Do()
	case "Describe":
		s, descErr := service.Snapshots.Get(project, snap.Name).Context(ctx).Do()
		if descErr != nil {
			return "", descErr
		}
		return fmt.Sprintf("Name: %s\nStatus: %s\nSize: %d GB\nSource Disk: %s\nDescription: %s\n",
			s.Name, s.Status, s.DiskSizeGb, s.SourceDisk, s.Description,
		), nil
	default:
		parts := strings.Split(action, ":")
		if len(parts) == 2 && parts[0] == "Create Disk" {
			diskName := parts[1]
			zone := cloudCtx.Region + "-a" // Default zone, UI handles better
			_, err = service.Disks.Insert(project, zone, &compute.Disk{
				Name:           diskName,
				SourceSnapshot: snap.Name, // This expects a URL, might need full URL but often accepts name
			}).Context(ctx).Do()
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("Successfully initiated disk creation from %s", snap.Name), nil
		}
		return "", fmt.Errorf("action %s not supported for GCP Snapshots", action)
	}

	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Successfully executed '%s' on %s", action, snap.Name), nil
}
