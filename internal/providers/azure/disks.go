package azure

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
)

// FetchDisksSDK fetches Managed Disks using the Azure SDK.
func FetchDisksSDK(ctx context.Context, subscription string) ([]core.Disk, error) {
	client, err := getDisksClient(subscription)
	if err != nil {
		return nil, fmt.Errorf("failed to create disks client: %w", err)
	}

	pager := client.NewListPager(&armcompute.DisksClientListOptions{})
	var disks []core.Disk

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list disks: %w", err)
		}

		for _, disk := range page.Value {
			name := "-"
			if disk.Name != nil {
				name = *disk.Name
			}

			id := "-"
			resourceGroup := "-"
			if disk.ID != nil {
				id = *disk.ID
				parts := strings.Split(*disk.ID, "/")
				for i, p := range parts {
					if strings.ToLower(p) == "resourcegroups" && i+1 < len(parts) {
						resourceGroup = parts[i+1]
						break
					}
				}
			}

			state := "-"
			if disk.Properties != nil && disk.Properties.DiskState != nil {
				state = string(*disk.Properties.DiskState)
			}

			sizeGB := 0
			if disk.Properties != nil && disk.Properties.DiskSizeGB != nil {
				sizeGB = int(*disk.Properties.DiskSizeGB)
			}

			diskType := "-"
			if disk.SKU != nil && disk.SKU.Name != nil {
				diskType = string(*disk.SKU.Name)
			}

			zone := "-"
			if disk.Location != nil {
				zone = *disk.Location
			}

			attachedToVM := "-"
			attachedToVMID := "-"
			if disk.ManagedBy != nil {
				attachedToVMID = *disk.ManagedBy
				parts := strings.Split(*disk.ManagedBy, "/")
				attachedToVM = parts[len(parts)-1]
			}

			createdAt := "-"
			if disk.Properties != nil && disk.Properties.TimeCreated != nil {
				createdAt = disk.Properties.TimeCreated.Format("2006-01-02T15:04:05Z")
			}

			var labelPairs []string
			if disk.Tags != nil {
				for k, v := range disk.Tags {
					if v != nil {
						labelPairs = append(labelPairs, fmt.Sprintf("%s=%s", k, *v))
					}
				}
			}

			disks = append(disks, core.Disk{
				Name:           name,
				ID:             id,
				State:          state,
				SizeGB:         sizeGB,
				Type:           diskType,
				Zone:           zone,
				AttachedToVM:   attachedToVM,
				AttachedToVMID: attachedToVMID,
				IOPS:           "-",
				Throughput:     "-",
				Encrypted:      true, // Managed disks are encrypted at rest by default
				CreatedAt:      createdAt,
				ResourceGroup:  resourceGroup,
				Labels:         strings.Join(labelPairs, ", "),
			})
		}
	}

	return disks, nil
}

// ExecuteDiskActionSDK executes an action on an Azure Managed Disk.
func ExecuteDiskActionSDK(ctx context.Context, action string, disk core.Disk, cloudCtx core.CloudContext) (string, error) {
	client, err := getDisksClient(cloudCtx.AccountID)
	if err != nil {
		return "", fmt.Errorf("failed to create disks client: %w", err)
	}

	switch action {
	case "Detach":
		return "", fmt.Errorf("detach requires VM client in Azure (not fully implemented here)")
	case "Delete":
		poller, err := client.BeginDelete(ctx, disk.ResourceGroup, disk.Name, nil)
		if err != nil {
			return "", err
		}
		_, err = poller.PollUntilDone(ctx, nil)
	case "Describe":
		resp, err := client.Get(ctx, disk.ResourceGroup, disk.Name, nil)
		if err != nil {
			return "", err
		}
		d := resp.Disk
		return fmt.Sprintf("Name: %s\nID: %s\nLocation: %s\nState: %v\nSize: %v GB\n",
			*d.Name, *d.ID, *d.Location, d.Properties.DiskState, d.Properties.DiskSizeGB,
		), nil
	default:
		parts := strings.Split(action, ":")
		if len(parts) == 2 && parts[0] == "Resize" {
			sizeGB := int32(150) // Need parsing
			fmt.Sscanf(parts[1], "%d", &sizeGB)

			poller, err := client.BeginUpdate(ctx, disk.ResourceGroup, disk.Name, armcompute.DiskUpdate{
				Properties: &armcompute.DiskUpdateProperties{
					DiskSizeGB: &sizeGB,
				},
			}, nil)
			if err != nil {
				return "", err
			}
			_, err = poller.PollUntilDone(ctx, nil)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("Successfully resized %s to %d GB", disk.Name, sizeGB), nil
		}
		return "", fmt.Errorf("action %s not supported for Azure Disks", action)
	}

	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Successfully executed '%s' on %s", action, disk.Name), nil
}

func getDisksClient(subscription string) (*armcompute.DisksClient, error) {
	cred, err := getAzureCreds()
	if err != nil {
		return nil, err
	}
	clientFactory, err := armcompute.NewClientFactory(subscription, cred, nil)
	if err != nil {
		return nil, err
	}
	return clientFactory.NewDisksClient(), nil
}
