package aws

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"cloudmanager/internal/core"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

// FetchDisksSDK fetches EBS volumes using the AWS SDK.
func FetchDisksSDK(ctx context.Context, profile, region string) ([]core.Disk, error) {
	cfg, err := getAWSConfig(ctx, profile, region)
	if err != nil {
		return nil, fmt.Errorf("failed to load aws config: %w", err)
	}

	client := ec2.NewFromConfig(cfg)
	paginator := ec2.NewDescribeVolumesPaginator(client, &ec2.DescribeVolumesInput{})

	var disks []core.Disk

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe volumes: %w", err)
		}

		for _, vol := range page.Volumes {
			name := "-"
			var labelPairs []string
			for _, tag := range vol.Tags {
				if aws.ToString(tag.Key) == "Name" {
					name = aws.ToString(tag.Value)
				}
				labelPairs = append(labelPairs, fmt.Sprintf("%s=%s", aws.ToString(tag.Key), aws.ToString(tag.Value)))
			}

			attachedToVM := "-"
			attachedToVMID := "-"
			if len(vol.Attachments) > 0 {
				attachedToVMID = aws.ToString(vol.Attachments[0].InstanceId)
				// Note: We don't have the VM name here easily without another API call.
				// For now, we'll just use the ID as the name or leave it as "-".
				attachedToVM = attachedToVMID
			}

			iops := "-"
			if vol.Iops != nil {
				iops = fmt.Sprintf("%d", aws.ToInt32(vol.Iops))
			}

			throughput := "-"
			if vol.Throughput != nil {
				throughput = fmt.Sprintf("%d", aws.ToInt32(vol.Throughput))
			}

			createdAt := "-"
			if vol.CreateTime != nil {
				createdAt = vol.CreateTime.Format("2006-01-02T15:04:05Z")
			}

			disks = append(disks, core.Disk{
				Name:           name,
				ID:             aws.ToString(vol.VolumeId),
				State:          string(vol.State),
				SizeGB:         int(aws.ToInt32(vol.Size)),
				Type:           string(vol.VolumeType),
				Zone:           aws.ToString(vol.AvailabilityZone),
				AttachedToVM:   attachedToVM,
				AttachedToVMID: attachedToVMID,
				IOPS:           iops,
				Throughput:     throughput,
				Encrypted:      aws.ToBool(vol.Encrypted),
				CreatedAt:      createdAt,
				Labels:         strings.Join(labelPairs, ", "),
			})
		}
	}

	return disks, nil
}

// ExecuteDiskActionSDK executes an action on an EBS volume.
func ExecuteDiskActionSDK(ctx context.Context, action string, disk core.Disk, cloudCtx core.CloudContext) (string, error) {
	cfg, err := getAWSConfig(ctx, cloudCtx.AuthRef(), cloudCtx.Region)
	if err != nil {
		return "", fmt.Errorf("failed to load aws config: %w", err)
	}

	client := ec2.NewFromConfig(cfg)

	switch action {
	case "Detach":
		_, err = client.DetachVolume(ctx, &ec2.DetachVolumeInput{
			VolumeId: aws.String(disk.ID),
		})
	case "Delete":
		_, err = client.DeleteVolume(ctx, &ec2.DeleteVolumeInput{
			VolumeId: aws.String(disk.ID),
		})
	case "Resize":
		// This requires a size parameter. For now, we'll just return an error
		// or hardcode a bump for demonstration if needed, but typically UI handles this.
		// The prompt for size will come from the TUI. We assume the size is passed via cloudCtx or similar?
		// Wait, the interface is ExecuteDiskAction(ctx, action, disk, cloudCtx).
		// If action is "Resize:150" we can parse it.
		parts := strings.Split(action, ":")
		if len(parts) == 2 && parts[0] == "Resize" {
			size, err := strconv.Atoi(parts[1])
			if err != nil {
				return "", fmt.Errorf("invalid size: %w", err)
			}
			_, err = client.ModifyVolume(ctx, &ec2.ModifyVolumeInput{
				VolumeId: aws.String(disk.ID),
				Size:     aws.Int32(int32(size)),
			})
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("Successfully initiated resize of %s to %d GB", disk.ID, size), nil
		}
		return "", fmt.Errorf("resize requires size parameter in action string (e.g., Resize:150)")
	case "Create Snapshot":
		_, err = client.CreateSnapshot(ctx, &ec2.CreateSnapshotInput{
			VolumeId:    aws.String(disk.ID),
			Description: aws.String(fmt.Sprintf("Created by CloudManager from %s", disk.ID)),
		})
	case "Describe":
		resp, descErr := client.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{
			VolumeIds: []string{disk.ID},
		})
		if descErr != nil {
			return "", descErr
		}
		if len(resp.Volumes) > 0 {
			vol := resp.Volumes[0]
			return fmt.Sprintf("Volume ID: %s\nType: %s\nState: %s\nSize: %d GB\nZone: %s\nEncrypted: %v\n",
				aws.ToString(vol.VolumeId), vol.VolumeType, vol.State,
				aws.ToInt32(vol.Size), aws.ToString(vol.AvailabilityZone), aws.ToBool(vol.Encrypted),
			), nil
		}
		return "", fmt.Errorf("volume not found")
	default:
		return "", fmt.Errorf("action %s not supported for AWS Disks", action)
	}

	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Successfully executed '%s' on %s", action, disk.ID), nil
}
