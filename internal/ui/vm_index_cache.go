package ui

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/vyoogam/cloudmanager/internal/config"
	"github.com/vyoogam/cloudmanager/internal/core"
	"github.com/vyoogam/cloudmanager/internal/localdb"
)

const vmIndexCacheVersion = 1
const resourceIndexCacheVersion = 1

var resourceIndexTypes = []string{"cluster", "database", "disk", "snapshot", "network", "subnet", "firewall", "storage"}
var resourceSummaryTypes = []string{"disks", "snapshots", "networks", "firewalls", "storage"}

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
	payload := vmIndexPayloadFromMap(index)
	sqliteErr := saveVMIndexSQLite(payload.Records)
	jsonErr := writeVMIndexJSON(payload)
	if sqliteErr != nil && jsonErr != nil {
		return sqliteErr
	}
	return nil
}

func vmIndexPayloadFromMap(index map[string]vmSearchRecord) vmIndexCacheFile {
	records := make([]vmIndexCacheRecord, 0, len(index))
	for _, rec := range index {
		records = append(records, vmIndexCacheRecord{
			VM:      rec.VM,
			Context: rec.Context,
			SeenAt:  rec.SeenAt,
		})
	}
	return vmIndexCacheFile{
		Version:   vmIndexCacheVersion,
		UpdatedAt: time.Now(),
		Records:   records,
	}
}

func writeVMIndexJSON(payload vmIndexCacheFile) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(config.GetVMIndexPath(), data, 0600)
}

func loadVMIndexCache(ttl time.Duration) (map[string]vmSearchRecord, int, error) {
	if index, total, err := loadVMIndexSQLite(ttl); err == nil && total > 0 {
		return index, total, nil
	}
	index, total, err := loadVMIndexJSON(ttl)
	if err != nil {
		return nil, 0, err
	}
	if total > 0 {
		_ = saveVMIndexCache(index)
	}
	return index, total, nil
}

func loadVMIndexJSON(ttl time.Duration) (map[string]vmSearchRecord, int, error) {
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

func saveVMIndexSQLite(records []vmIndexCacheRecord) error {
	db, err := localdb.Open(context.Background())
	if err != nil {
		return err
	}
	defer db.Close()
	rows := make([]localdb.ResourceRow, 0, len(records))
	for _, rec := range records {
		payload, err := json.Marshal(rec)
		if err != nil {
			return err
		}
		rows = append(rows, localdb.ResourceRow{
			ResourceType:   "vm",
			CacheKey:       vmIndexKey(rec.Context, rec.VM),
			Provider:       rec.Context.Provider,
			ContextKey:     rec.Context.CacheKey(),
			AccountID:      rec.Context.AccountID,
			AccountName:    rec.Context.AccountName,
			Region:         rec.Context.Region,
			ResourceID:     rec.VM.ID,
			ResourceName:   rec.VM.Name,
			SearchableText: strings.Join(nonEmptyStrings(rec.VM.Name, rec.VM.ID, rec.VM.PrivateIP, rec.VM.PublicIP, rec.VM.State, rec.VM.Zone, rec.VM.Network, rec.VM.Subnet, rec.VM.SecurityGroups, rec.VM.Labels), " "),
			Tags:           rec.VM.Labels,
			PayloadJSON:    string(payload),
			SeenAt:         rec.SeenAt,
		})
	}
	return localdb.ReplaceResourceRows(context.Background(), db, []string{"vm"}, rows)
}

func loadVMIndexSQLite(ttl time.Duration) (map[string]vmSearchRecord, int, error) {
	db, err := localdb.Open(context.Background())
	if err != nil {
		return nil, 0, err
	}
	defer db.Close()
	rows, total, err := localdb.LoadResourceRows(context.Background(), db, []string{"vm"}, ttl)
	if err != nil {
		return nil, 0, err
	}
	index := make(map[string]vmSearchRecord, len(rows))
	for _, row := range rows {
		var rec vmIndexCacheRecord
		if err := json.Unmarshal([]byte(row.PayloadJSON), &rec); err != nil {
			continue
		}
		key := vmIndexKey(rec.Context, rec.VM)
		index[key] = vmSearchRecord{VM: rec.VM, Context: rec.Context, SeenAt: rec.SeenAt}
	}
	return index, total, nil
}

func saveResourceIndexCache(a App) error {
	payload := resourceIndexPayloadFromApp(a)
	sqliteErr := saveResourceIndexSQLite(payload)
	jsonErr := writeResourceIndexJSON(payload)
	if sqliteErr != nil && jsonErr != nil {
		return sqliteErr
	}
	return nil
}

func resourceIndexPayloadFromApp(a App) resourceIndexCacheFile {
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
	return payload
}

func writeResourceIndexJSON(payload resourceIndexCacheFile) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(config.GetResourceIndexPath(), data, 0600)
}

func loadResourceIndexCache(ttl time.Duration) (resourceIndexCacheBundle, error) {
	if bundle, total, err := loadResourceIndexSQLite(ttl); err == nil && total > 0 {
		return bundle, nil
	}
	bundle, err := loadResourceIndexJSON(ttl)
	if err != nil {
		return bundle, err
	}
	return bundle, nil
}

func loadResourceIndexJSON(ttl time.Duration) (resourceIndexCacheBundle, error) {
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

func saveResourceIndexSQLite(payload resourceIndexCacheFile) error {
	db, err := localdb.Open(context.Background())
	if err != nil {
		return err
	}
	defer db.Close()
	rows, err := resourceRowsFromPayload(payload)
	if err != nil {
		return err
	}
	if err := localdb.ReplaceResourceRows(context.Background(), db, resourceIndexTypes, rows); err != nil {
		return err
	}
	return localdb.ReplaceSummaryRows(context.Background(), db, resourceSummaryTypes, summaryRowsFromPayload(payload))
}

func loadResourceIndexSQLite(ttl time.Duration) (resourceIndexCacheBundle, int, error) {
	db, err := localdb.Open(context.Background())
	if err != nil {
		return resourceIndexCacheBundle{}, 0, err
	}
	defer db.Close()
	rows, rowTotal, err := localdb.LoadResourceRows(context.Background(), db, resourceIndexTypes, ttl)
	if err != nil {
		return resourceIndexCacheBundle{}, 0, err
	}
	summaryRows, summaryTotal, err := localdb.LoadSummaryRows(context.Background(), db, resourceSummaryTypes, ttl)
	if err != nil {
		return resourceIndexCacheBundle{}, 0, err
	}
	bundle := emptyResourceIndexBundle()
	for _, row := range rows {
		switch row.ResourceType {
		case "cluster":
			var rec clusterIndexCacheRecord
			if json.Unmarshal([]byte(row.PayloadJSON), &rec) == nil {
				key := fmtResourceIndexKey(rec.Context, rec.Cluster.ID, rec.Cluster.Name)
				bundle.Clusters[key] = clusterIndexRecord{Cluster: rec.Cluster, Context: rec.Context, SeenAt: rec.SeenAt}
			}
		case "database":
			var rec databaseIndexCacheRecord
			if json.Unmarshal([]byte(row.PayloadJSON), &rec) == nil {
				key := fmtResourceIndexKey(rec.Context, rec.Database.ID, rec.Database.Name)
				bundle.Databases[key] = databaseIndexRecord{Database: rec.Database, Context: rec.Context, SeenAt: rec.SeenAt}
			}
		case "disk":
			var rec diskIndexCacheRecord
			if json.Unmarshal([]byte(row.PayloadJSON), &rec) == nil {
				key := fmtResourceIndexKey(rec.Context, rec.Disk.ID, rec.Disk.Name)
				bundle.Disks[key] = diskIndexRecord{Disk: rec.Disk, Context: rec.Context, SeenAt: rec.SeenAt}
			}
		case "snapshot":
			var rec snapshotIndexCacheRecord
			if json.Unmarshal([]byte(row.PayloadJSON), &rec) == nil {
				key := fmtResourceIndexKey(rec.Context, rec.Snapshot.ID, rec.Snapshot.Name)
				bundle.Snapshots[key] = snapshotIndexRecord{Snapshot: rec.Snapshot, Context: rec.Context, SeenAt: rec.SeenAt}
			}
		case "network":
			var rec networkIndexCacheRecord
			if json.Unmarshal([]byte(row.PayloadJSON), &rec) == nil {
				key := fmtResourceIndexKey(rec.Context, rec.Network.ID, rec.Network.Name)
				bundle.Networks[key] = networkIndexRecord{Network: rec.Network, Context: rec.Context, SeenAt: rec.SeenAt}
			}
		case "subnet":
			var rec subnetIndexCacheRecord
			if json.Unmarshal([]byte(row.PayloadJSON), &rec) == nil {
				key := fmtResourceIndexKey(rec.Context, rec.Subnet.ID, rec.Subnet.Name)
				bundle.Subnets[key] = subnetIndexRecord{Subnet: rec.Subnet, Context: rec.Context, SeenAt: rec.SeenAt}
			}
		case "firewall":
			var rec firewallIndexCacheRecord
			if json.Unmarshal([]byte(row.PayloadJSON), &rec) == nil {
				key := fmtResourceIndexKey(rec.Context, rec.Group.ID, rec.Group.Name)
				bundle.Firewalls[key] = firewallIndexRecord{Group: rec.Group, Context: rec.Context, SeenAt: rec.SeenAt}
			}
		case "storage":
			var rec storageIndexCacheRecord
			if json.Unmarshal([]byte(row.PayloadJSON), &rec) == nil {
				key := fmtResourceIndexKey(rec.Context, rec.Bucket.ID, rec.Bucket.Name)
				bundle.Storage[key] = storageIndexRecord{Bucket: rec.Bucket, Context: rec.Context, SeenAt: rec.SeenAt}
			}
		}
	}
	for _, row := range summaryRows {
		rec := resourceSummaryRecord{Count: row.Count, Extra: row.Extra}
		switch row.SummaryType {
		case "disks":
			bundle.DiskSummaries[row.ContextKey] = rec
		case "snapshots":
			bundle.SnapshotSummaries[row.ContextKey] = rec
		case "networks":
			bundle.NetworkSummaries[row.ContextKey] = rec
		case "firewalls":
			bundle.FirewallSummaries[row.ContextKey] = rec
		case "storage":
			bundle.StorageSummaries[row.ContextKey] = rec
		}
	}
	bundle.Total = len(bundle.Clusters) + len(bundle.Databases) + len(bundle.Disks) + len(bundle.Snapshots) + len(bundle.Networks) + len(bundle.Subnets) + len(bundle.Firewalls) + len(bundle.Storage) + len(bundle.DiskSummaries) + len(bundle.SnapshotSummaries) + len(bundle.NetworkSummaries) + len(bundle.FirewallSummaries) + len(bundle.StorageSummaries)
	return bundle, rowTotal + summaryTotal, nil
}

func resourceRowsFromPayload(payload resourceIndexCacheFile) ([]localdb.ResourceRow, error) {
	rows := make([]localdb.ResourceRow, 0, len(payload.Clusters)+len(payload.Databases)+len(payload.Disks)+len(payload.Snapshots)+len(payload.Networks)+len(payload.Subnets)+len(payload.Firewalls)+len(payload.Storage))
	for _, rec := range payload.Clusters {
		row, err := cacheResourceRow("cluster", rec.Context, rec.Cluster.ID, rec.Cluster.Name, rec.Cluster.Labels, rec.SeenAt, strings.Join(nonEmptyStrings(rec.Cluster.Name, rec.Cluster.ID, rec.Cluster.Location, rec.Cluster.Status, rec.Cluster.Version, rec.Cluster.Labels), " "), rec)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	for _, rec := range payload.Databases {
		row, err := cacheResourceRow("database", rec.Context, rec.Database.ID, rec.Database.Name, rec.Database.Labels, rec.SeenAt, strings.Join(nonEmptyStrings(rec.Database.Name, rec.Database.ID, rec.Database.Engine, rec.Database.Version, rec.Database.Status, rec.Database.Region, rec.Database.Labels), " "), rec)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	for _, rec := range payload.Disks {
		row, err := cacheResourceRow("disk", rec.Context, rec.Disk.ID, rec.Disk.Name, rec.Disk.Labels, rec.SeenAt, strings.Join(nonEmptyStrings(rec.Disk.Name, rec.Disk.ID, rec.Disk.State, rec.Disk.Type, rec.Disk.Zone, rec.Disk.AttachedToVM, rec.Disk.Labels), " "), rec)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	for _, rec := range payload.Snapshots {
		row, err := cacheResourceRow("snapshot", rec.Context, rec.Snapshot.ID, rec.Snapshot.Name, rec.Snapshot.Labels, rec.SeenAt, strings.Join(nonEmptyStrings(rec.Snapshot.Name, rec.Snapshot.ID, rec.Snapshot.State, rec.Snapshot.Zone, rec.Snapshot.SourceDiskID, rec.Snapshot.SourceDiskName, rec.Snapshot.Labels), " "), rec)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	for _, rec := range payload.Networks {
		row, err := cacheResourceRow("network", rec.Context, rec.Network.ID, rec.Network.Name, rec.Network.Labels, rec.SeenAt, strings.Join(nonEmptyStrings(rec.Network.Name, rec.Network.ID, rec.Network.State, rec.Network.CIDRBlock, rec.Network.Region, rec.Network.ResourceGroup, rec.Network.Labels), " "), rec)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	for _, rec := range payload.Subnets {
		row, err := cacheResourceRow("subnet", rec.Context, rec.Subnet.ID, rec.Subnet.Name, rec.Subnet.Labels, rec.SeenAt, strings.Join(nonEmptyStrings(rec.Subnet.Name, rec.Subnet.ID, rec.Subnet.State, rec.Subnet.CIDRBlock, rec.Subnet.AvailabilityZone, rec.Subnet.NetworkID, rec.Subnet.NetworkName, rec.Subnet.Region, rec.Subnet.Labels), " "), rec)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	for _, rec := range payload.Firewalls {
		row, err := cacheResourceRow("firewall", rec.Context, rec.Group.ID, rec.Group.Name, rec.Group.Labels, rec.SeenAt, strings.Join(nonEmptyStrings(rec.Group.Name, rec.Group.ID, rec.Group.Description, rec.Group.NetworkID, rec.Group.NetworkName, rec.Group.Region, rec.Group.ResourceGroup, rec.Group.Labels), " "), rec)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	for _, rec := range payload.Storage {
		row, err := cacheResourceRow("storage", rec.Context, rec.Bucket.ID, rec.Bucket.Name, rec.Bucket.Labels, rec.SeenAt, strings.Join(nonEmptyStrings(rec.Bucket.Name, rec.Bucket.ID, rec.Bucket.ProviderType, rec.Bucket.Region, rec.Bucket.StorageClass, rec.Bucket.Access, rec.Bucket.ResourceGroup, rec.Bucket.Labels), " "), rec)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func cacheResourceRow(resourceType string, ctx core.CloudContext, id, name, tags string, seenAt time.Time, searchable string, record any) (localdb.ResourceRow, error) {
	payload, err := json.Marshal(record)
	if err != nil {
		return localdb.ResourceRow{}, err
	}
	return localdb.ResourceRow{
		ResourceType:   resourceType,
		CacheKey:       fmtResourceIndexKey(ctx, id, name),
		Provider:       ctx.Provider,
		ContextKey:     ctx.CacheKey(),
		AccountID:      ctx.AccountID,
		AccountName:    ctx.AccountName,
		Region:         ctx.Region,
		ResourceID:     id,
		ResourceName:   name,
		SearchableText: searchable,
		Tags:           tags,
		PayloadJSON:    string(payload),
		SeenAt:         seenAt,
	}, nil
}

func summaryRowsFromPayload(payload resourceIndexCacheFile) []localdb.SummaryRow {
	total := len(payload.DiskSummaries) + len(payload.SnapshotSummaries) + len(payload.NetworkSummaries) + len(payload.FirewallSummaries) + len(payload.StorageSummaries)
	rows := make([]localdb.SummaryRow, 0, total)
	rows = append(rows, summaryRows("disks", payload.DiskSummaries, payload.UpdatedAt)...)
	rows = append(rows, summaryRows("snapshots", payload.SnapshotSummaries, payload.UpdatedAt)...)
	rows = append(rows, summaryRows("networks", payload.NetworkSummaries, payload.UpdatedAt)...)
	rows = append(rows, summaryRows("firewalls", payload.FirewallSummaries, payload.UpdatedAt)...)
	rows = append(rows, summaryRows("storage", payload.StorageSummaries, payload.UpdatedAt)...)
	return rows
}

func summaryRows(summaryType string, records []resourceSummaryCacheRecord, updatedAt time.Time) []localdb.SummaryRow {
	rows := make([]localdb.SummaryRow, 0, len(records))
	for _, rec := range records {
		rows = append(rows, localdb.SummaryRow{
			SummaryType: summaryType,
			ContextKey:  rec.Context.CacheKey(),
			Provider:    rec.Context.Provider,
			AccountID:   rec.Context.AccountID,
			AccountName: rec.Context.AccountName,
			Region:      rec.Context.Region,
			Count:       rec.Count,
			Extra:       rec.Extra,
			UpdatedAt:   updatedAt,
		})
	}
	return rows
}

func emptyResourceIndexBundle() resourceIndexCacheBundle {
	return resourceIndexCacheBundle{
		Clusters:          make(map[string]clusterIndexRecord),
		Databases:         make(map[string]databaseIndexRecord),
		Disks:             make(map[string]diskIndexRecord),
		Snapshots:         make(map[string]snapshotIndexRecord),
		Networks:          make(map[string]networkIndexRecord),
		Subnets:           make(map[string]subnetIndexRecord),
		Firewalls:         make(map[string]firewallIndexRecord),
		Storage:           make(map[string]storageIndexRecord),
		DiskSummaries:     make(map[string]resourceSummaryRecord),
		SnapshotSummaries: make(map[string]resourceSummaryRecord),
		NetworkSummaries:  make(map[string]resourceSummaryRecord),
		FirewallSummaries: make(map[string]resourceSummaryRecord),
		StorageSummaries:  make(map[string]resourceSummaryRecord),
	}
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
