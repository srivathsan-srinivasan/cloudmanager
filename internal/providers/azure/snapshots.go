package azure

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"cloudmanager/internal/core"
)

// FetchSnapshotsSDK fetches snapshots using the Azure SDK.
func FetchSnapshotsSDK(ctx context.Context, subscription string) ([]core.Snapshot, error) {
	client, err := getSnapshotsClient(subscription)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshots client: %w", err)
	}

	pager := client.NewListPager(nil)
	var snapshots []core.Snapshot

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list snapshots: %w", err)
		}

		for _, snap := range page.Value {
			name := "-"
			if snap.Name != nil {
				name = *snap.Name
			}

			id := "-"
			resourceGroup := "-"
			if snap.ID != nil {
				id = *snap.ID
				parts := strings.Split(*snap.ID, "/")
				for i, p := range parts {
					if strings.ToLower(p) == "resourcegroups" && i+1 < len(parts) {
						resourceGroup = parts[i+1]
						break
					}
				}
			}

			state := "-"
			if snap.Properties != nil && snap.Properties.ProvisioningState != nil {
				state = *snap.Properties.ProvisioningState
			}

			sizeGB := 0
			if snap.Properties != nil && snap.Properties.DiskSizeGB != nil {
				sizeGB = int(*snap.Properties.DiskSizeGB)
			}

			sourceDiskID := "-"
			sourceDiskName := "-"
			if snap.Properties != nil && snap.Properties.CreationData != nil && snap.Properties.CreationData.SourceResourceID != nil {
				sourceDiskID = *snap.Properties.CreationData.SourceResourceID
				parts := strings.Split(sourceDiskID, "/")
				sourceDiskName = parts[len(parts)-1]
			}

			createdAt := "-"
			if snap.Properties != nil && snap.Properties.TimeCreated != nil {
				createdAt = snap.Properties.TimeCreated.Format("2006-01-02T15:04:05Z")
			}

			zone := "-"
			if snap.Location != nil {
				zone = *snap.Location
			}

			var labelPairs []string
			if snap.Tags != nil {
				for k, v := range snap.Tags {
					if v != nil {
						labelPairs = append(labelPairs, fmt.Sprintf("%s=%s", k, *v))
					}
				}
			}

			snapshots = append(snapshots, core.Snapshot{
				Name:           name,
				ID:             id,
				State:          state,
				SizeGB:         sizeGB,
				SourceDiskName: sourceDiskName,
				SourceDiskID:   sourceDiskID,
				CreatedAt:      createdAt,
				Zone:           zone,
				ResourceGroup:  resourceGroup,
				Description:    "-",
				Labels:         strings.Join(labelPairs, ", "),
			})
		}
	}

	return snapshots, nil
}

// ExecuteSnapshotActionSDK executes an action on an Azure snapshot.
func ExecuteSnapshotActionSDK(ctx context.Context, action string, snap core.Snapshot, cloudCtx core.CloudContext) (string, error) {
	client, err := getSnapshotsClient(cloudCtx.AccountID)
	if err != nil {
		return "", fmt.Errorf("failed to create snapshots client: %w", err)
	}

	switch action {
	case "Delete":
		poller, err := client.BeginDelete(ctx, snap.ResourceGroup, snap.Name, nil)
		if err != nil {
			return "", err
		}
		_, err = poller.PollUntilDone(ctx, nil)
	case "Describe":
		resp, err := client.Get(ctx, snap.ResourceGroup, snap.Name, nil)
		if err != nil {
			return "", err
		}
		s := resp.Snapshot
		return fmt.Sprintf("Name: %s\nID: %s\nLocation: %s\nState: %v\nSize: %v GB\nSource: %v\n",
			*s.Name, *s.ID, *s.Location, s.Properties.ProvisioningState, s.Properties.DiskSizeGB,
			s.Properties.CreationData.SourceResourceID,
		), nil
	default:
		return "", fmt.Errorf("action %s not supported for Azure Snapshots", action)
	}

	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Successfully executed '%s' on %s", action, snap.Name), nil
}

func getSnapshotsClient(subscription string) (*armcompute.SnapshotsClient, error) {
	cred, err := getAzureCreds()
	if err != nil {
		return nil, err
	}
	clientFactory, err := armcompute.NewClientFactory(subscription, cred, nil)
	if err != nil {
		return nil, err
	}
	return clientFactory.NewSnapshotsClient(), nil
}
