package gcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
	"google.golang.org/api/compute/v1"
)

// FetchDisksSDK fetches Persistent Disks using the GCP SDK.
func FetchDisksSDK(ctx context.Context, project string) ([]core.Disk, error) {
	service, err := newComputeService(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create compute service: %w", err)
	}

	req := service.Disks.AggregatedList(project)
	var disks []core.Disk

	if err := req.Pages(ctx, func(page *compute.DiskAggregatedList) error {
		for _, scopedList := range page.Items {
			for _, disk := range scopedList.Disks {
				// Parse Type (URL -> name)
				parts := strings.Split(disk.Type, "/")
				diskType := parts[len(parts)-1]

				// Parse Zone (URL -> name)
				zone := "-"
				if disk.Zone != "" {
					zParts := strings.Split(disk.Zone, "/")
					zone = zParts[len(zParts)-1]
				}

				// Attached To
				attachedToVM := "-"
				if len(disk.Users) > 0 {
					// Users is a list of instance URLs
					uParts := strings.Split(disk.Users[0], "/")
					attachedToVM = uParts[len(uParts)-1]
				}

				// Labels
				var labelPairs []string
				for k, v := range disk.Labels {
					labelPairs = append(labelPairs, fmt.Sprintf("%s=%s", k, v))
				}

				disks = append(disks, core.Disk{
					Name:           disk.Name,
					ID:             fmt.Sprintf("%d", disk.Id),
					State:          disk.Status,
					SizeGB:         int(disk.SizeGb),
					Type:           diskType,
					Zone:           zone,
					AttachedToVM:   attachedToVM,
					AttachedToVMID: attachedToVM, // Using name for ID as well since GCP mostly uses names
					IOPS:           "-",          // Not directly exposed on all disk types
					Throughput:     "-",
					Encrypted:      true, // PDs are encrypted by default
					CreatedAt:      disk.CreationTimestamp,
					Labels:         strings.Join(labelPairs, ", "),
				})
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to list disks: %w", err)
	}

	return disks, nil
}

// ExecuteDiskActionSDK executes an action on a GCP disk.
func ExecuteDiskActionSDK(ctx context.Context, action string, disk core.Disk, cloudCtx core.CloudContext) (string, error) {
	service, err := newComputeService(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to create compute service: %w", err)
	}

	project := cloudCtx.AccountID
	zone := disk.Zone

	switch action {
	case "Detach":
		if disk.AttachedToVM == "-" {
			return "", fmt.Errorf("disk is not attached")
		}
		_, err = service.Instances.DetachDisk(project, zone, disk.AttachedToVM, disk.Name).Context(ctx).Do()
	case "Delete":
		_, err = service.Disks.Delete(project, zone, disk.Name).Context(ctx).Do()
	case "Create Snapshot":
		_, err = service.Disks.CreateSnapshot(project, zone, disk.Name, &compute.Snapshot{
			Name:        fmt.Sprintf("%s-snap", disk.Name),
			Description: "Created by CloudManager",
		}).Context(ctx).Do()
	case "Describe":
		d, descErr := service.Disks.Get(project, zone, disk.Name).Context(ctx).Do()
		if descErr != nil {
			return "", descErr
		}
		return fmt.Sprintf("Name: %s\nType: %s\nSize: %d GB\nStatus: %s\nZone: %s\n",
			d.Name, d.Type, d.SizeGb, d.Status, d.Zone,
		), nil
	default:
		// Handle Resize:150
		parts := strings.Split(action, ":")
		if len(parts) == 2 && parts[0] == "Resize" {
			var size int64
			fmt.Sscanf(parts[1], "%d", &size)
			_, err = service.Disks.Resize(project, zone, disk.Name, &compute.DisksResizeRequest{
				SizeGb: size,
			}).Context(ctx).Do()
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("Successfully initiated resize of %s to %d GB", disk.Name, size), nil
		}
		return "", fmt.Errorf("action %s not supported for GCP Disks", action)
	}

	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Successfully executed '%s' on %s", action, disk.Name), nil
}
