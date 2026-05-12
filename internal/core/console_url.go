package core

import (
	"fmt"
	"net/url"
	"strings"
)

func VMConsoleURL(ctx CloudContext, vm VM) string {
	provider := strings.ToUpper(strings.TrimSpace(ctx.Provider))
	region := firstNonEmpty(ctx.Region)
	switch provider {
	case "AWS":
		if region == "" || vm.ID == "" {
			return ""
		}
		return fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/home?region=%s#InstanceDetails:instanceId=%s", url.QueryEscape(region), url.QueryEscape(region), url.QueryEscape(vm.ID))
	case "GCP":
		zone := zoneSegment(vm.Zone)
		if ctx.AccountID == "" || zone == "" || vm.Name == "" {
			return ""
		}
		return fmt.Sprintf("https://console.cloud.google.com/compute/instancesDetail/zones/%s/instances/%s?project=%s", url.PathEscape(zone), url.PathEscape(vm.Name), url.QueryEscape(ctx.AccountID))
	case "AZURE":
		if strings.HasPrefix(vm.ID, "/subscriptions/") {
			return azurePortalResourceURL(ctx, vm.ID)
		}
		if ctx.AccountID == "" || vm.ResourceGroup == "" || vm.Name == "" {
			return ""
		}
		return azurePortalResourceURL(ctx, fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/virtualMachines/%s", ctx.AccountID, vm.ResourceGroup, vm.Name))
	}
	return ""
}

func DiskConsoleURL(ctx CloudContext, disk Disk) string {
	provider := strings.ToUpper(strings.TrimSpace(ctx.Provider))
	region := firstNonEmpty(ctx.Region)
	switch provider {
	case "AWS":
		if region == "" || disk.ID == "" {
			return ""
		}
		return fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/home?region=%s#VolumeDetails:volumeId=%s", url.QueryEscape(region), url.QueryEscape(region), url.QueryEscape(disk.ID))
	case "GCP":
		zone := zoneSegment(disk.Zone)
		name := firstNonEmpty(disk.Name, disk.ID)
		if ctx.AccountID == "" || zone == "" || name == "" {
			return ""
		}
		return fmt.Sprintf("https://console.cloud.google.com/compute/disksDetail/zones/%s/disks/%s?project=%s", url.PathEscape(zone), url.PathEscape(name), url.QueryEscape(ctx.AccountID))
	case "AZURE":
		if strings.HasPrefix(disk.ID, "/subscriptions/") {
			return azurePortalResourceURL(ctx, disk.ID)
		}
		if ctx.AccountID == "" || disk.ResourceGroup == "" || disk.Name == "" {
			return ""
		}
		return azurePortalResourceURL(ctx, fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/disks/%s", ctx.AccountID, disk.ResourceGroup, disk.Name))
	}
	return ""
}

func SnapshotConsoleURL(ctx CloudContext, snap Snapshot) string {
	provider := strings.ToUpper(strings.TrimSpace(ctx.Provider))
	region := firstNonEmpty(ctx.Region)
	switch provider {
	case "AWS":
		if region == "" || snap.ID == "" {
			return ""
		}
		return fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/home?region=%s#SnapshotDetails:snapshotId=%s", url.QueryEscape(region), url.QueryEscape(region), url.QueryEscape(snap.ID))
	case "GCP":
		name := firstNonEmpty(snap.Name, snap.ID)
		if ctx.AccountID == "" || name == "" {
			return ""
		}
		return fmt.Sprintf("https://console.cloud.google.com/compute/snapshotsDetail/global/snapshots/%s?project=%s", url.PathEscape(name), url.QueryEscape(ctx.AccountID))
	case "AZURE":
		if strings.HasPrefix(snap.ID, "/subscriptions/") {
			return azurePortalResourceURL(ctx, snap.ID)
		}
		if ctx.AccountID == "" || snap.ResourceGroup == "" || snap.Name == "" {
			return ""
		}
		return azurePortalResourceURL(ctx, fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/snapshots/%s", ctx.AccountID, snap.ResourceGroup, snap.Name))
	}
	return ""
}

func NetworkConsoleURL(ctx CloudContext, n Network) string {
	provider := strings.ToUpper(strings.TrimSpace(ctx.Provider))
	region := firstNonEmpty(n.Region, ctx.Region)
	switch provider {
	case "AWS":
		if region == "" || n.ID == "" {
			return ""
		}
		return fmt.Sprintf("https://%s.console.aws.amazon.com/vpcconsole/home?region=%s#VpcDetails:VpcId=%s", url.QueryEscape(region), url.QueryEscape(region), url.QueryEscape(n.ID))
	case "GCP":
		name := firstNonEmpty(n.Name, n.ID)
		if ctx.AccountID == "" || name == "" {
			return ""
		}
		return fmt.Sprintf("https://console.cloud.google.com/networking/networks/details/%s?project=%s", url.PathEscape(name), url.QueryEscape(ctx.AccountID))
	case "AZURE":
		if strings.HasPrefix(n.ID, "/subscriptions/") {
			return azurePortalResourceURL(ctx, n.ID)
		}
	}
	return ""
}

func SubnetConsoleURL(ctx CloudContext, s Subnet) string {
	provider := strings.ToUpper(strings.TrimSpace(ctx.Provider))
	region := firstNonEmpty(s.Region, ctx.Region)
	switch provider {
	case "AWS":
		if region == "" || s.ID == "" {
			return ""
		}
		return fmt.Sprintf("https://%s.console.aws.amazon.com/vpcconsole/home?region=%s#SubnetDetails:SubnetId=%s", url.QueryEscape(region), url.QueryEscape(region), url.QueryEscape(s.ID))
	case "GCP":
		name := firstNonEmpty(s.Name, s.ID)
		if ctx.AccountID == "" || name == "" {
			return ""
		}
		return fmt.Sprintf("https://console.cloud.google.com/networking/subnetworks/details/%s?project=%s", url.PathEscape(name), url.QueryEscape(ctx.AccountID))
	case "AZURE":
		if strings.HasPrefix(s.ID, "/subscriptions/") {
			return azurePortalResourceURL(ctx, s.ID)
		}
	}
	return ""
}

func SecurityGroupConsoleURL(ctx CloudContext, s SecurityGroup) string {
	provider := strings.ToUpper(strings.TrimSpace(firstNonEmpty(s.Provider, ctx.Provider)))
	region := firstNonEmpty(s.Region, ctx.Region)
	switch provider {
	case "AWS":
		if region == "" || s.ID == "" {
			return ""
		}
		return fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/home?region=%s#SecurityGroup:groupId=%s", url.QueryEscape(region), url.QueryEscape(region), url.QueryEscape(s.ID))
	case "GCP":
		name := firstNonEmpty(s.Name, s.ID)
		if ctx.AccountID == "" || name == "" {
			return ""
		}
		return fmt.Sprintf("https://console.cloud.google.com/networking/firewalls/details/%s?project=%s", url.PathEscape(name), url.QueryEscape(ctx.AccountID))
	case "AZURE":
		if strings.HasPrefix(s.ID, "/subscriptions/") {
			return azurePortalResourceURL(ctx, s.ID)
		}
	}
	return ""
}

func FirewallRuleConsoleURL(ctx CloudContext, r FirewallRule) string {
	provider := strings.ToUpper(strings.TrimSpace(firstNonEmpty(r.Provider, ctx.Provider)))
	switch provider {
	case "AWS":
		groupID := firstNonEmpty(r.ResourceID, r.NetworkID)
		if !strings.HasPrefix(groupID, "sg-") {
			return ""
		}
		return SecurityGroupConsoleURL(ctx, SecurityGroup{ID: groupID, Provider: provider})
	case "GCP":
		name := firstNonEmpty(r.ResourceName, r.ID)
		if ctx.AccountID == "" || name == "" {
			return ""
		}
		return fmt.Sprintf("https://console.cloud.google.com/networking/firewalls/details/%s?project=%s", url.PathEscape(name), url.QueryEscape(ctx.AccountID))
	case "AZURE":
		if strings.HasPrefix(r.ResourceID, "/subscriptions/") {
			return azurePortalResourceURL(ctx, r.ResourceID)
		}
	}
	return ""
}

func DetailWithConsoleURL(content, consoleURL string) string {
	consoleURL = strings.TrimSpace(consoleURL)
	if consoleURL == "" {
		return content
	}
	return fmt.Sprintf("Console:        %s\n\n%s", consoleURL, content)
}

func zoneSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parts := strings.Split(value, "/")
	return strings.TrimSpace(parts[len(parts)-1])
}
