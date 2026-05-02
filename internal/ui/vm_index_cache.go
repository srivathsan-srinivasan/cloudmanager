package ui

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"cloudmanager/internal/config"
	"cloudmanager/internal/core"
)

const vmIndexCacheVersion = 1
const resourceIndexCacheVersion = 1

type vmIndexCacheFile struct {
	Version   int                  `json:"version"`
	UpdatedAt time.Time            `json:"updated_at"`
	Records   []vmIndexCacheRecord `json:"records"`
}

type vmIndexCacheRecord struct {
	VM      core.VM           `json:"vm"`
	Context core.CloudContext `json:"context"`
	SeenAt  time.Time         `json:"seen_at"`
}

type resourceIndexCacheFile struct {
	Version           int                          `json:"version"`
	UpdatedAt         time.Time                    `json:"updated_at"`
	Clusters          []clusterIndexCacheRecord    `json:"clusters"`
	Databases         []databaseIndexCacheRecord   `json:"databases"`
	Disks             []diskIndexCacheRecord       `json:"disks"`
	Snapshots         []snapshotIndexCacheRecord   `json:"snapshots"`
	Networks          []networkIndexCacheRecord    `json:"networks"`
	Subnets           []subnetIndexCacheRecord     `json:"subnets"`
	Firewalls         []firewallIndexCacheRecord   `json:"firewalls"`
	Storage           []storageIndexCacheRecord    `json:"storage"`
	DiskSummaries     []resourceSummaryCacheRecord `json:"disk_summaries"`
	SnapshotSummaries []resourceSummaryCacheRecord `json:"snapshot_summaries"`
	NetworkSummaries  []resourceSummaryCacheRecord `json:"network_summaries"`
	FirewallSummaries []resourceSummaryCacheRecord `json:"firewall_summaries"`
	StorageSummaries  []resourceSummaryCacheRecord `json:"storage_summaries"`
}

type resourceIndexCacheBundle struct {
	Clusters          map[string]clusterIndexRecord
	Databases         map[string]databaseIndexRecord
	Disks             map[string]diskIndexRecord
	Snapshots         map[string]snapshotIndexRecord
	Networks          map[string]networkIndexRecord
	Subnets           map[string]subnetIndexRecord
	Firewalls         map[string]firewallIndexRecord
	Storage           map[string]storageIndexRecord
	DiskSummaries     map[string]resourceSummaryRecord
	SnapshotSummaries map[string]resourceSummaryRecord
	NetworkSummaries  map[string]resourceSummaryRecord
	FirewallSummaries map[string]resourceSummaryRecord
	StorageSummaries  map[string]resourceSummaryRecord
	Total             int
}

type clusterIndexCacheRecord struct {
	Cluster core.Cluster      `json:"cluster"`
	Context core.CloudContext `json:"context"`
	SeenAt  time.Time         `json:"seen_at"`
}

type databaseIndexCacheRecord struct {
	Database core.Database     `json:"database"`
	Context  core.CloudContext `json:"context"`
	SeenAt   time.Time         `json:"seen_at"`
}

type diskIndexCacheRecord struct {
	Disk    core.Disk         `json:"disk"`
	Context core.CloudContext `json:"context"`
	SeenAt  time.Time         `json:"seen_at"`
}

type snapshotIndexCacheRecord struct {
	Snapshot core.Snapshot     `json:"snapshot"`
	Context  core.CloudContext `json:"context"`
	SeenAt   time.Time         `json:"seen_at"`
}

type networkIndexCacheRecord struct {
	Network core.Network      `json:"network"`
	Context core.CloudContext `json:"context"`
	SeenAt  time.Time         `json:"seen_at"`
}

type subnetIndexCacheRecord struct {
	Subnet  core.Subnet       `json:"subnet"`
	Context core.CloudContext `json:"context"`
	SeenAt  time.Time         `json:"seen_at"`
}

type firewallIndexCacheRecord struct {
	Group   core.SecurityGroup `json:"group"`
	Context core.CloudContext  `json:"context"`
	SeenAt  time.Time          `json:"seen_at"`
}

type storageIndexCacheRecord struct {
	Bucket  core.StorageBucket `json:"bucket"`
	Context core.CloudContext  `json:"context"`
	SeenAt  time.Time          `json:"seen_at"`
}

type resourceSummaryCacheRecord struct {
	Context core.CloudContext `json:"context"`
	Count   int               `json:"count"`
	Extra   int               `json:"extra"`
}

func saveVMIndexCache(index map[string]vmSearchRecord) error {
	records := make([]vmIndexCacheRecord, 0, len(index))
	for _, rec := range index {
		records = append(records, vmIndexCacheRecord{
			VM:      rec.VM,
			Context: rec.Context,
			SeenAt:  rec.SeenAt,
		})
	}
	payload := vmIndexCacheFile{
		Version:   vmIndexCacheVersion,
		UpdatedAt: time.Now(),
		Records:   records,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(config.GetVMIndexPath(), data, 0600)
}

func loadVMIndexCache(ttl time.Duration) (map[string]vmSearchRecord, int, error) {
	data, err := os.ReadFile(config.GetVMIndexPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, nil
		}
		return nil, 0, err
	}
	var payload vmIndexCacheFile
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, 0, err
	}
	if payload.Version != vmIndexCacheVersion {
		return nil, 0, nil
	}
	index := make(map[string]vmSearchRecord, len(payload.Records))
	now := time.Now()
	for _, rec := range payload.Records {
		if ttl > 0 && now.Sub(rec.SeenAt) > ttl {
			continue
		}
		key := vmIndexKey(rec.Context, rec.VM)
		index[key] = vmSearchRecord{VM: rec.VM, Context: rec.Context, SeenAt: rec.SeenAt}
	}
	return index, len(payload.Records), nil
}

func saveResourceIndexCache(a App) error {
	payload := resourceIndexCacheFile{
		Version:           resourceIndexCacheVersion,
		UpdatedAt:         time.Now(),
		Clusters:          make([]clusterIndexCacheRecord, 0, len(a.clusterIndex)),
		Databases:         make([]databaseIndexCacheRecord, 0, len(a.databaseIndex)),
		Disks:             make([]diskIndexCacheRecord, 0, len(a.diskIndex)),
		Snapshots:         make([]snapshotIndexCacheRecord, 0, len(a.snapshotIndex)),
		Networks:          make([]networkIndexCacheRecord, 0, len(a.networkIndex)),
		Subnets:           make([]subnetIndexCacheRecord, 0, len(a.subnetIndex)),
		Firewalls:         make([]firewallIndexCacheRecord, 0, len(a.firewallIndex)),
		Storage:           make([]storageIndexCacheRecord, 0, len(a.storageIndex)),
		DiskSummaries:     summaryCacheRecords(a.diskSummaryIndex),
		SnapshotSummaries: summaryCacheRecords(a.snapshotSummaryIndex),
		NetworkSummaries:  summaryCacheRecords(a.networkSummaryIndex),
		FirewallSummaries: summaryCacheRecords(a.firewallSummaryIndex),
		StorageSummaries:  summaryCacheRecords(a.storageSummaryIndex),
	}
	for _, rec := range a.clusterIndex {
		payload.Clusters = append(payload.Clusters, clusterIndexCacheRecord{Cluster: rec.Cluster, Context: rec.Context, SeenAt: rec.SeenAt})
	}
	for _, rec := range a.databaseIndex {
		payload.Databases = append(payload.Databases, databaseIndexCacheRecord{Database: rec.Database, Context: rec.Context, SeenAt: rec.SeenAt})
	}
	for _, rec := range a.diskIndex {
		payload.Disks = append(payload.Disks, diskIndexCacheRecord{Disk: rec.Disk, Context: rec.Context, SeenAt: rec.SeenAt})
	}
	for _, rec := range a.snapshotIndex {
		payload.Snapshots = append(payload.Snapshots, snapshotIndexCacheRecord{Snapshot: rec.Snapshot, Context: rec.Context, SeenAt: rec.SeenAt})
	}
	for _, rec := range a.networkIndex {
		payload.Networks = append(payload.Networks, networkIndexCacheRecord{Network: rec.Network, Context: rec.Context, SeenAt: rec.SeenAt})
	}
	for _, rec := range a.subnetIndex {
		payload.Subnets = append(payload.Subnets, subnetIndexCacheRecord{Subnet: rec.Subnet, Context: rec.Context, SeenAt: rec.SeenAt})
	}
	for _, rec := range a.firewallIndex {
		payload.Firewalls = append(payload.Firewalls, firewallIndexCacheRecord{Group: rec.Group, Context: rec.Context, SeenAt: rec.SeenAt})
	}
	for _, rec := range a.storageIndex {
		payload.Storage = append(payload.Storage, storageIndexCacheRecord{Bucket: rec.Bucket, Context: rec.Context, SeenAt: rec.SeenAt})
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(config.GetResourceIndexPath(), data, 0600)
}

func loadResourceIndexCache(ttl time.Duration) (resourceIndexCacheBundle, error) {
	var bundle resourceIndexCacheBundle
	data, err := os.ReadFile(config.GetResourceIndexPath())
	if err != nil {
		if os.IsNotExist(err) {
			return bundle, nil
		}
		return bundle, err
	}
	var payload resourceIndexCacheFile
	if err := json.Unmarshal(data, &payload); err != nil {
		return bundle, err
	}
	if payload.Version != resourceIndexCacheVersion {
		return bundle, nil
	}
	if ttl > 0 && time.Since(payload.UpdatedAt) > ttl {
		return bundle, nil
	}
	bundle.Clusters = make(map[string]clusterIndexRecord, len(payload.Clusters))
	bundle.Databases = make(map[string]databaseIndexRecord, len(payload.Databases))
	bundle.Disks = make(map[string]diskIndexRecord, len(payload.Disks))
	bundle.Snapshots = make(map[string]snapshotIndexRecord, len(payload.Snapshots))
	bundle.Networks = make(map[string]networkIndexRecord, len(payload.Networks))
	bundle.Subnets = make(map[string]subnetIndexRecord, len(payload.Subnets))
	bundle.Firewalls = make(map[string]firewallIndexRecord, len(payload.Firewalls))
	bundle.Storage = make(map[string]storageIndexRecord, len(payload.Storage))
	for _, rec := range payload.Clusters {
		key := fmtResourceIndexKey(rec.Context, rec.Cluster.ID, rec.Cluster.Name)
		bundle.Clusters[key] = clusterIndexRecord{Cluster: rec.Cluster, Context: rec.Context, SeenAt: rec.SeenAt}
	}
	for _, rec := range payload.Databases {
		key := fmtResourceIndexKey(rec.Context, rec.Database.ID, rec.Database.Name)
		bundle.Databases[key] = databaseIndexRecord{Database: rec.Database, Context: rec.Context, SeenAt: rec.SeenAt}
	}
	for _, rec := range payload.Disks {
		key := fmtResourceIndexKey(rec.Context, rec.Disk.ID, rec.Disk.Name)
		bundle.Disks[key] = diskIndexRecord{Disk: rec.Disk, Context: rec.Context, SeenAt: rec.SeenAt}
	}
	for _, rec := range payload.Snapshots {
		key := fmtResourceIndexKey(rec.Context, rec.Snapshot.ID, rec.Snapshot.Name)
		bundle.Snapshots[key] = snapshotIndexRecord{Snapshot: rec.Snapshot, Context: rec.Context, SeenAt: rec.SeenAt}
	}
	for _, rec := range payload.Networks {
		key := fmtResourceIndexKey(rec.Context, rec.Network.ID, rec.Network.Name)
		bundle.Networks[key] = networkIndexRecord{Network: rec.Network, Context: rec.Context, SeenAt: rec.SeenAt}
	}
	for _, rec := range payload.Subnets {
		key := fmtResourceIndexKey(rec.Context, rec.Subnet.ID, rec.Subnet.Name)
		bundle.Subnets[key] = subnetIndexRecord{Subnet: rec.Subnet, Context: rec.Context, SeenAt: rec.SeenAt}
	}
	for _, rec := range payload.Firewalls {
		key := fmtResourceIndexKey(rec.Context, rec.Group.ID, rec.Group.Name)
		bundle.Firewalls[key] = firewallIndexRecord{Group: rec.Group, Context: rec.Context, SeenAt: rec.SeenAt}
	}
	for _, rec := range payload.Storage {
		key := fmtResourceIndexKey(rec.Context, rec.Bucket.ID, rec.Bucket.Name)
		bundle.Storage[key] = storageIndexRecord{Bucket: rec.Bucket, Context: rec.Context, SeenAt: rec.SeenAt}
	}
	bundle.DiskSummaries = summaryCacheMap(payload.DiskSummaries)
	bundle.SnapshotSummaries = summaryCacheMap(payload.SnapshotSummaries)
	bundle.NetworkSummaries = summaryCacheMap(payload.NetworkSummaries)
	bundle.FirewallSummaries = summaryCacheMap(payload.FirewallSummaries)
	bundle.StorageSummaries = summaryCacheMap(payload.StorageSummaries)
	bundle.Total = len(bundle.Clusters) + len(bundle.Databases) + len(bundle.Disks) + len(bundle.Snapshots) + len(bundle.Networks) + len(bundle.Subnets) + len(bundle.Firewalls) + len(bundle.Storage) + len(bundle.DiskSummaries) + len(bundle.SnapshotSummaries) + len(bundle.NetworkSummaries) + len(bundle.FirewallSummaries) + len(bundle.StorageSummaries)
	return bundle, nil
}

func summaryCacheRecords(index map[string]resourceSummaryRecord) []resourceSummaryCacheRecord {
	records := make([]resourceSummaryCacheRecord, 0, len(index))
	for key, rec := range index {
		records = append(records, resourceSummaryCacheRecord{
			Context: cloudContextFromCacheKey(key),
			Count:   rec.Count,
			Extra:   rec.Extra,
		})
	}
	return records
}

func summaryCacheMap(records []resourceSummaryCacheRecord) map[string]resourceSummaryRecord {
	index := make(map[string]resourceSummaryRecord, len(records))
	for _, rec := range records {
		index[rec.Context.CacheKey()] = resourceSummaryRecord{Count: rec.Count, Extra: rec.Extra}
	}
	return index
}

func fmtResourceIndexKey(ctx core.CloudContext, id, name string) string {
	return ctx.CacheKey() + "|" + orFallback(id, name)
}

func cloudContextFromCacheKey(key string) core.CloudContext {
	parts := strings.Split(key, "|")
	ctx := core.CloudContext{}
	if len(parts) > 0 {
		ctx.Provider = parts[0]
	}
	if len(parts) > 1 {
		ctx.AccountID = parts[1]
	}
	if len(parts) > 2 {
		ctx.Region = parts[2]
	}
	if len(parts) > 3 {
		ctx.AccountName = parts[3]
	}
	return ctx
}
