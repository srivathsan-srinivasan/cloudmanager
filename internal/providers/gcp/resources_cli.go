package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"google.golang.org/api/compute/v1"

	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
	"github.com/srivathsan-srinivasan/cloudmanager/internal/logging"
)

type gcpDiskCLI struct {
	Name              string            `json:"name"`
	Id                string            `json:"id"`
	SizeGb            string            `json:"sizeGb"`
	Type              string            `json:"type"`
	Status            string            `json:"status"`
	Zone              string            `json:"zone"`
	Users             []string          `json:"users"`
	CreationTimestamp string            `json:"creationTimestamp"`
	Labels            map[string]string `json:"labels"`
}

type gcpSnapshotCLI struct {
	Name              string            `json:"name"`
	Id                string            `json:"id"`
	Status            string            `json:"status"`
	DiskSizeGb        string            `json:"diskSizeGb"`
	SourceDisk        string            `json:"sourceDisk"`
	CreationTimestamp string            `json:"creationTimestamp"`
	Description       string            `json:"description"`
	Labels            map[string]string `json:"labels"`
}

type gcpClusterCLI struct {
	Name                 string            `json:"name"`
	Location             string            `json:"location"`
	Status               string            `json:"status"`
	CurrentMasterVersion string            `json:"currentMasterVersion"`
	CurrentNodeCount     int               `json:"currentNodeCount"`
	SelfLink             string            `json:"selfLink"`
	ResourceLabels       map[string]string `json:"resourceLabels"`
}

type gcpDatabaseSettingsCLI struct {
	Tier       string            `json:"tier"`
	UserLabels map[string]string `json:"userLabels"`
}

type gcpDatabaseCLI struct {
	Name            string                 `json:"name"`
	State           string                 `json:"state"`
	DatabaseVersion string                 `json:"databaseVersion"`
	Region          string                 `json:"region"`
	SelfLink        string                 `json:"selfLink"`
	Settings        gcpDatabaseSettingsCLI `json:"settings"`
}

func FetchClustersCLI(project string) ([]core.Cluster, error) {
	output, err := runGCloudJSON("container", "clusters", "list", "--project", project)
	if err != nil {
		return nil, err
	}
	var data []gcpClusterCLI
	if err := json.Unmarshal(output, &data); err != nil {
		return nil, fmt.Errorf("failed to parse gcp clusters: %w", err)
	}

	clusters := make([]core.Cluster, 0, len(data))
	for _, c := range data {
		clusters = append(clusters, core.Cluster{
			ID:        c.SelfLink,
			Name:      c.Name,
			Location:  c.Location,
			Status:    c.Status,
			Version:   c.CurrentMasterVersion,
			NodeCount: fmt.Sprintf("%d", c.CurrentNodeCount),
			Labels:    joinLabelMap(c.ResourceLabels),
		})
	}
	return clusters, nil
}

func FetchDatabasesCLI(project string) ([]core.Database, error) {
	output, err := runGCloudJSON("sql", "instances", "list", "--project", project)
	if err != nil {
		return nil, err
	}
	var data []gcpDatabaseCLI
	if err := json.Unmarshal(output, &data); err != nil {
		return nil, fmt.Errorf("failed to parse gcp databases: %w", err)
	}

	databases := make([]core.Database, 0, len(data))
	for _, db := range data {
		databases = append(databases, core.Database{
			ID:      db.SelfLink,
			Name:    db.Name,
			Engine:  db.DatabaseVersion,
			Version: db.DatabaseVersion,
			Status:  db.State,
			Region:  db.Region,
			Size:    db.Settings.Tier,
			Labels:  joinLabelMap(db.Settings.UserLabels),
		})
	}
	return databases, nil
}

func FetchDisksCLI(project string) ([]core.Disk, error) {
	output, err := runGCloudJSON("compute", "disks", "list", "--project", project)
	if err != nil {
		return nil, err
	}
	return parseGCPDisksCLI(output)
}

func FetchSnapshotsCLI(project string) ([]core.Snapshot, error) {
	output, err := runGCloudJSON("compute", "snapshots", "list", "--project", project)
	if err != nil {
		return nil, err
	}
	return parseGCPSnapshotsCLI(output)
}

func FetchSecurityGroupsCLI(project string) ([]core.SecurityGroup, error) {
	networks, err := listGCPNetworksCLI(project)
	if err != nil {
		return nil, err
	}
	firewalls, err := listGCPFirewallRulesCLI(project)
	if err != nil {
		return nil, err
	}
	attachments, err := gcpNetworkAttachmentCountsCLI(project)
	if err != nil {
		return nil, err
	}
	return gcpSecurityGroupsFromNetworkData(networks, firewalls, attachments), nil
}

func FetchFirewallRulesByNetworkCLI(project, networkName string) ([]core.FirewallRule, error) {
	firewalls, err := listGCPFirewallRulesCLI(project)
	if err != nil {
		return nil, err
	}

	rules := gcpClassicFirewallRulesToCore(firewalls, networkName)
	if effective, err := getGCPEffectiveFirewallsCLI(project, networkName); err == nil {
		rules = append(rules, gcpEffectivePolicyRulesToCore(effective, networkName)...)
	}

	rules = dedupeFirewallRules(rules)
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].Priority != rules[j].Priority {
			return rules[i].Priority < rules[j].Priority
		}
		return rules[i].ID < rules[j].ID
	})
	return rules, nil
}

func parseGCPDisksCLI(output []byte) ([]core.Disk, error) {
	var data []gcpDiskCLI
	if err := json.Unmarshal(output, &data); err != nil {
		return nil, fmt.Errorf("failed to parse gcp disks: %w", err)
	}
	disks := make([]core.Disk, 0, len(data))
	for _, disk := range data {
		disks = append(disks, gcpDiskCLIToCore(disk))
	}
	return disks, nil
}

func parseGCPSnapshotsCLI(output []byte) ([]core.Snapshot, error) {
	var data []gcpSnapshotCLI
	if err := json.Unmarshal(output, &data); err != nil {
		return nil, fmt.Errorf("failed to parse gcp snapshots: %w", err)
	}
	snaps := make([]core.Snapshot, 0, len(data))
	for _, snap := range data {
		snaps = append(snaps, gcpSnapshotCLIToCore(snap))
	}
	return snaps, nil
}

func gcpDiskCLIToCore(disk gcpDiskCLI) core.Disk {
	return core.Disk{
		Name:           disk.Name,
		ID:             disk.Id,
		State:          disk.Status,
		SizeGB:         parseGCPSizeGB(disk.SizeGb),
		Type:           shortName("", disk.Type),
		Zone:           shortName("", disk.Zone),
		AttachedToVM:   shortName("", firstOrDash(disk.Users)),
		AttachedToVMID: shortName("", firstOrDash(disk.Users)),
		IOPS:           "-",
		Throughput:     "-",
		Encrypted:      true,
		CreatedAt:      disk.CreationTimestamp,
		Labels:         joinLabelMap(disk.Labels),
	}
}

func gcpSnapshotCLIToCore(snap gcpSnapshotCLI) core.Snapshot {
	sourceDiskName := shortName("", snap.SourceDisk)
	return core.Snapshot{
		Name:           snap.Name,
		ID:             snap.Id,
		State:          snap.Status,
		SizeGB:         parseGCPSizeGB(snap.DiskSizeGb),
		SourceDiskName: sourceDiskName,
		SourceDiskID:   sourceDiskName,
		CreatedAt:      snap.CreationTimestamp,
		Zone:           "global",
		Description:    snap.Description,
		Labels:         joinLabelMap(snap.Labels),
	}
}

func gcpNetworkAttachmentCountsCLI(project string) (map[string]int, error) {
	output, err := runGCloudJSON("compute", "instances", "list", "--project", project)
	if err != nil {
		return nil, err
	}
	var data []gcpInstance
	if err := json.Unmarshal(output, &data); err != nil {
		return nil, fmt.Errorf("failed to parse gcp instances for firewall attachment counts: %w", err)
	}
	counts := make(map[string]int)
	for _, inst := range data {
		for _, nic := range inst.NetworkInterfaces {
			networkName := shortName("", nic.Network)
			if networkName == "" || networkName == "-" {
				continue
			}
			counts[networkName]++
		}
	}
	return counts, nil
}

func listGCPNetworksCLI(project string) ([]*compute.Network, error) {
	output, err := runGCloudJSON("compute", "networks", "list", "--project", project)
	if err != nil {
		return nil, err
	}
	var networks []*compute.Network
	if err := json.Unmarshal(output, &networks); err != nil {
		return nil, fmt.Errorf("failed to parse gcp networks: %w", err)
	}
	return networks, nil
}

func listGCPFirewallRulesCLI(project string) ([]*compute.Firewall, error) {
	output, err := runGCloudJSON("compute", "firewall-rules", "list", "--project", project)
	if err != nil {
		return nil, err
	}
	var rules []*compute.Firewall
	if err := json.Unmarshal(output, &rules); err != nil {
		return nil, fmt.Errorf("failed to parse gcp firewall rules: %w", err)
	}
	return rules, nil
}

func getGCPEffectiveFirewallsCLI(project, networkName string) (*compute.NetworksGetEffectiveFirewallsResponse, error) {
	output, err := runGCloudJSON("compute", "networks", "get-effective-firewalls", networkName, "--project", project)
	if err != nil {
		return nil, err
	}
	var response compute.NetworksGetEffectiveFirewallsResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("failed to parse gcp effective firewalls: %w", err)
	}
	return &response, nil
}

func runGCloudJSON(args ...string) ([]byte, error) {
	fullArgs := append(args, "--format=json")
	cmd := exec.Command("gcloud", fullArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gcloud %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func parseGCPSizeGB(value string) int {
	size, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return size
}

func joinLabelMap(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, labels[key]))
	}
	return strings.Join(parts, ", ")
}

func firstOrDash(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return values[0]
}

func gcpSecurityGroupsFromNetworkData(networks []*compute.Network, firewalls []*compute.Firewall, attachments map[string]int) []core.SecurityGroup {
	type networkSummary struct {
		network       *compute.Network
		inboundRules  int
		outboundRules int
		hasOpenSSH    bool
		hasOpenRDP    bool
	}

	summaries := make(map[string]*networkSummary, len(networks))
	for _, network := range networks {
		if network == nil {
			continue
		}
		name := shortName(network.Name, network.SelfLink)
		summaries[name] = &networkSummary{network: network}
	}

	for _, rule := range firewalls {
		if rule == nil {
			continue
		}
		networkName := shortName("", rule.Network)
		summary, ok := summaries[networkName]
		if !ok {
			summary = &networkSummary{network: &compute.Network{Name: networkName, SelfLink: rule.Network}}
			summaries[networkName] = summary
		}

		switch strings.ToUpper(rule.Direction) {
		case "EGRESS":
			summary.outboundRules++
		default:
			summary.inboundRules++
		}

		if gcpRuleOpensPort(rule, 22) {
			summary.hasOpenSSH = true
		}
		if gcpRuleOpensPort(rule, 3389) {
			summary.hasOpenRDP = true
		}
	}

	names := make([]string, 0, len(summaries))
	for name := range summaries {
		names = append(names, name)
	}
	sort.Strings(names)

	groups := make([]core.SecurityGroup, 0, len(names))
	for _, name := range names {
		summary := summaries[name]
		network := summary.network
		groups = append(groups, core.SecurityGroup{
			Name:              name,
			ID:                name,
			Description:       orDash(network.Description),
			NetworkID:         shortName("", network.SelfLink),
			NetworkName:       name,
			InboundRuleCount:  summary.inboundRules,
			OutboundRuleCount: summary.outboundRules,
			AttachedResources: attachments[name],
			HasOpenSSH:        summary.hasOpenSSH,
			HasOpenRDP:        summary.hasOpenRDP,
			Provider:          "GCP",
			Region:            "global",
		})
	}

	return groups
}

func gcpClassicFirewallRulesToCore(firewalls []*compute.Firewall, networkName string) []core.FirewallRule {
	var rules []core.FirewallRule
	for _, rule := range firewalls {
		if shortName("", rule.Network) != networkName {
			continue
		}
		rules = append(rules, gcpFirewallRuleToCore(rule, networkName)...)
	}
	return rules
}

func FetchDisksSDKWithCLIAuthFallback(ctx context.Context, project string) ([]core.Disk, error) {
	disks, err := FetchDisksSDK(ctx, project)
	if err == nil {
		return disks, nil
	}
	logging.Warnf("component=gcp resource=disks mode=sdk fallback=cli project=%s err=%v", project, err)
	return FetchDisksCLI(project)
}

func FetchSnapshotsSDKWithCLIAuthFallback(ctx context.Context, project string) ([]core.Snapshot, error) {
	snaps, err := FetchSnapshotsSDK(ctx, project)
	if err == nil {
		return snaps, nil
	}
	logging.Warnf("component=gcp resource=snapshots mode=sdk fallback=cli project=%s err=%v", project, err)
	return FetchSnapshotsCLI(project)
}

func FetchSecurityGroupsSDKWithCLIAuthFallback(ctx context.Context, project string) ([]core.SecurityGroup, error) {
	groups, err := FetchSecurityGroupsSDK(ctx, project)
	if err == nil {
		return groups, nil
	}
	logging.Warnf("component=gcp resource=firewalls mode=sdk fallback=cli project=%s err=%v", project, err)
	return FetchSecurityGroupsCLI(project)
}

func FetchFirewallRulesByNetworkSDKWithCLIAuthFallback(ctx context.Context, project, networkName string) ([]core.FirewallRule, error) {
	rules, err := FetchFirewallRulesByNetworkSDK(ctx, project, networkName)
	if err == nil {
		return rules, nil
	}
	logging.Warnf("component=gcp resource=firewall_rules mode=sdk fallback=cli project=%s network=%s err=%v", project, networkName, err)
	return FetchFirewallRulesByNetworkCLI(project, networkName)
}
