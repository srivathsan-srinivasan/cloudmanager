package ui

import (
	"encoding/csv"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/vyoogam/cloudmanager/internal/config"
	"github.com/vyoogam/cloudmanager/internal/core"
)

type publicEndpointRecord struct {
	SourceType   string
	Provider     string
	Context      string
	Region       string
	ResourceName string
	ResourceID   string
	EndpointType string
	Endpoint     string
	Notes        string
	SeenAt       time.Time
}

func exportPublicEndpointsCSV(a App, path string) (int, string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = config.GetPublicEndpointExportPath()
	}
	records := a.publicEndpointRecords()
	file, err := os.Create(path)
	if err != nil {
		return 0, path, err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{"source_type", "provider", "context", "region", "resource_name", "resource_id", "endpoint_type", "endpoint", "notes", "seen_at"}
	if err := writer.Write(header); err != nil {
		return 0, path, err
	}
	for _, rec := range records {
		row := []string{
			rec.SourceType,
			rec.Provider,
			rec.Context,
			rec.Region,
			rec.ResourceName,
			rec.ResourceID,
			rec.EndpointType,
			rec.Endpoint,
			rec.Notes,
			formatEndpointSeenAt(rec.SeenAt),
		}
		if err := writer.Write(row); err != nil {
			return 0, path, err
		}
	}
	if err := writer.Error(); err != nil {
		return 0, path, err
	}
	return len(records), path, nil
}

func (a App) publicEndpointRecords() []publicEndpointRecord {
	var records []publicEndpointRecord
	seen := map[string]struct{}{}
	add := func(rec publicEndpointRecord) {
		rec.Endpoint = strings.TrimSpace(rec.Endpoint)
		if rec.Endpoint == "" || rec.Endpoint == "-" {
			return
		}
		key := strings.Join([]string{rec.SourceType, rec.Provider, rec.Context, rec.Region, rec.ResourceID, rec.EndpointType, rec.Endpoint}, "\x00")
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		records = append(records, rec)
	}

	for _, rec := range a.vmIndex {
		if !hasUsablePublicIP(rec.VM.PublicIP) {
			continue
		}
		add(publicEndpointRecord{
			SourceType:   vmEndpointSourceType(rec.Context),
			Provider:     rec.Context.Provider,
			Context:      rec.Context.DisplayName(),
			Region:       rec.Context.Region,
			ResourceName: rec.VM.Name,
			ResourceID:   orFallback(rec.VM.ID, rec.VM.Name),
			EndpointType: endpointTypeForHost(rec.VM.PublicIP),
			Endpoint:     rec.VM.PublicIP,
			Notes:        strings.TrimSpace(rec.VM.State),
			SeenAt:       rec.SeenAt,
		})
	}
	for _, rec := range a.storageIndex {
		uri := core.StorageURI(rec.Context, rec.Bucket)
		if strings.TrimSpace(uri) == "" {
			continue
		}
		add(publicEndpointRecord{
			SourceType:   "storage",
			Provider:     rec.Context.Provider,
			Context:      rec.Context.DisplayName(),
			Region:       firstNonEmptyString(rec.Bucket.Region, rec.Context.Region),
			ResourceName: rec.Bucket.Name,
			ResourceID:   rec.Bucket.GetID(),
			EndpointType: "uri",
			Endpoint:     uri,
			Notes:        strings.TrimSpace(rec.Bucket.Access),
			SeenAt:       rec.SeenAt,
		})
	}

	sort.Slice(records, func(i, j int) bool {
		left, right := records[i], records[j]
		for _, cmp := range []int{
			strings.Compare(left.SourceType, right.SourceType),
			strings.Compare(left.Provider, right.Provider),
			strings.Compare(left.Context, right.Context),
			strings.Compare(left.Region, right.Region),
			strings.Compare(left.ResourceName, right.ResourceName),
			strings.Compare(left.Endpoint, right.Endpoint),
		} {
			if cmp != 0 {
				return cmp < 0
			}
		}
		return false
	})
	return records
}

func vmEndpointSourceType(ctx core.CloudContext) string {
	if strings.EqualFold(ctx.Provider, "Manual") {
		return "host"
	}
	return "vm"
}

func endpointTypeForHost(value string) string {
	value = strings.TrimSpace(value)
	if strings.ContainsAny(value, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		return "domain"
	}
	return "public_ip"
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func formatEndpointSeenAt(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
