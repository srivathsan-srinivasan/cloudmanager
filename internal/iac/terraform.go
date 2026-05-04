package iac

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
)

const (
	ProviderUnknown = ""
	KindVM          = "vm"
	KindDatabase    = "database"
	KindCluster     = "cluster"
)

type TerraformResource struct {
	Address   string
	Type      string
	Provider  string
	Kind      string
	ID        string
	Name      string
	PrivateIP string
	PublicIP  string
}

type TerraformIndex struct {
	Resources []TerraformResource
}

type cachedState struct {
	modTime time.Time
	size    int64
	index   TerraformIndex
}

var stateCache = struct {
	sync.Mutex
	byPath map[string]cachedState
}{byPath: make(map[string]cachedState)}

type tfState struct {
	Resources []tfStateResource `json:"resources"`
}

type tfStateResource struct {
	Mode      string            `json:"mode"`
	Module    string            `json:"module"`
	Type      string            `json:"type"`
	Name      string            `json:"name"`
	Provider  string            `json:"provider"`
	Address   string            `json:"address"`
	Instances []tfStateInstance `json:"instances"`
}

type tfStateInstance struct {
	IndexKey   any            `json:"index_key"`
	Attributes map[string]any `json:"attributes"`
}

func LoadTerraformStateFile(path string) (TerraformIndex, error) {
	path = expandPath(strings.TrimSpace(path))
	info, err := os.Stat(path)
	if err != nil {
		return TerraformIndex{}, err
	}
	stateCache.Lock()
	if cached, ok := stateCache.byPath[path]; ok && cached.modTime.Equal(info.ModTime()) && cached.size == info.Size() {
		stateCache.Unlock()
		return cached.index, nil
	}
	stateCache.Unlock()
	data, err := os.ReadFile(path)
	if err != nil {
		return TerraformIndex{}, err
	}
	var state tfState
	if err := json.Unmarshal(data, &state); err != nil {
		return TerraformIndex{}, err
	}
	var index TerraformIndex
	for _, resource := range state.Resources {
		if strings.EqualFold(resource.Mode, "data") {
			continue
		}
		for _, instance := range resource.Instances {
			attrs := instance.Attributes
			if len(attrs) == 0 {
				continue
			}
			item := TerraformResource{
				Address:   instanceAddress(resource, instance),
				Type:      strings.TrimSpace(resource.Type),
				Provider:  inferProvider(resource.Type, resource.Provider),
				Kind:      inferKind(resource.Type),
				ID:        firstString(attrs, "id", "self_link", "arn"),
				Name:      firstString(attrs, "name", "instance_name", "db_name", "cluster_id", "identifier"),
				PrivateIP: firstString(attrs, "private_ip", "private_ip_address", "network_ip"),
				PublicIP:  firstString(attrs, "public_ip", "public_ip_address", "nat_ip"),
			}
			if item.Name == "" {
				item.Name = tagName(attrs)
			}
			if item.Address == "" {
				item.Address = strings.TrimSpace(resource.Type + "." + resource.Name)
			}
			index.Resources = append(index.Resources, item)
		}
	}
	sort.SliceStable(index.Resources, func(i, j int) bool {
		return index.Resources[i].Address < index.Resources[j].Address
	})
	stateCache.Lock()
	stateCache.byPath[path] = cachedState{modTime: info.ModTime(), size: info.Size(), index: index}
	stateCache.Unlock()
	return index, nil
}

func LoadTerraformStateFiles(paths []string) (TerraformIndex, []error) {
	var merged TerraformIndex
	var errs []error
	for _, path := range uniquePaths(paths) {
		index, err := LoadTerraformStateFile(path)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		merged.Resources = append(merged.Resources, index.Resources...)
	}
	return merged, errs
}

func ApplyTerraformToVMs(paths []string, ctx core.CloudContext, vms []core.VM) []core.VM {
	if len(paths) == 0 || len(vms) == 0 {
		return vms
	}
	index, _ := LoadTerraformStateFiles(paths)
	if len(index.Resources) == 0 {
		return vms
	}
	out := make([]core.VM, len(vms))
	for i, vm := range vms {
		out[i] = vm
		if resource, ok := index.MatchVM(ctx, vm); ok {
			out[i].Labels = appendTerraformLabels(vm.Labels, resource.Address)
		}
	}
	return out
}

func ApplyTerraformToDatabases(paths []string, ctx core.CloudContext, databases []core.Database) []core.Database {
	if len(paths) == 0 || len(databases) == 0 {
		return databases
	}
	index, _ := LoadTerraformStateFiles(paths)
	if len(index.Resources) == 0 {
		return databases
	}
	out := make([]core.Database, len(databases))
	for i, db := range databases {
		out[i] = db
		if resource, ok := index.MatchDatabase(ctx, db); ok {
			out[i].Labels = appendTerraformLabels(db.Labels, resource.Address)
		}
	}
	return out
}

func ApplyTerraformToClusters(paths []string, ctx core.CloudContext, clusters []core.Cluster) []core.Cluster {
	if len(paths) == 0 || len(clusters) == 0 {
		return clusters
	}
	index, _ := LoadTerraformStateFiles(paths)
	if len(index.Resources) == 0 {
		return clusters
	}
	out := make([]core.Cluster, len(clusters))
	for i, cluster := range clusters {
		out[i] = cluster
		if resource, ok := index.MatchCluster(ctx, cluster); ok {
			out[i].Labels = appendTerraformLabels(cluster.Labels, resource.Address)
		}
	}
	return out
}

func (i TerraformIndex) MatchVM(ctx core.CloudContext, vm core.VM) (TerraformResource, bool) {
	for _, resource := range i.Resources {
		if resource.Kind != KindVM || !providerMatches(resource.Provider, ctx.Provider) {
			continue
		}
		if matchesResource(resource, vm.ID, vm.Name, vm.PrivateIP, vm.PublicIP) {
			return resource, true
		}
	}
	return TerraformResource{}, false
}

func (i TerraformIndex) MatchDatabase(ctx core.CloudContext, db core.Database) (TerraformResource, bool) {
	for _, resource := range i.Resources {
		if resource.Kind != KindDatabase || !providerMatches(resource.Provider, ctx.Provider) {
			continue
		}
		if matchesResource(resource, db.ID, db.Name, "", "") {
			return resource, true
		}
	}
	return TerraformResource{}, false
}

func (i TerraformIndex) MatchCluster(ctx core.CloudContext, cluster core.Cluster) (TerraformResource, bool) {
	for _, resource := range i.Resources {
		if resource.Kind != KindCluster || !providerMatches(resource.Provider, ctx.Provider) {
			continue
		}
		if matchesResource(resource, cluster.ID, cluster.Name, "", "") {
			return resource, true
		}
	}
	return TerraformResource{}, false
}

func matchesResource(resource TerraformResource, id, name, privateIP, publicIP string) bool {
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	privateIP = strings.TrimSpace(privateIP)
	publicIP = strings.TrimSpace(publicIP)
	if id != "" && stringMatch(resource.ID, id) {
		return true
	}
	if name != "" && (stringMatch(resource.Name, name) || pathEndsWithName(resource.ID, name)) {
		return true
	}
	if privateIP != "" && resource.PrivateIP == privateIP {
		return true
	}
	if publicIP != "" && resource.PublicIP == publicIP {
		return true
	}
	return false
}

func stringMatch(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	return left != "" && right != "" && strings.EqualFold(left, right)
}

func pathEndsWithName(value, name string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	name = strings.ToLower(strings.TrimSpace(name))
	return value != "" && name != "" && (strings.HasSuffix(value, "/"+name) || strings.Contains(value, "/instances/"+name))
}

func providerMatches(tfProvider, ctxProvider string) bool {
	tfProvider = strings.TrimSpace(tfProvider)
	if tfProvider == "" {
		return true
	}
	return strings.EqualFold(tfProvider, strings.TrimSpace(ctxProvider))
}

func inferProvider(resourceType, provider string) string {
	value := strings.ToLower(resourceType + " " + provider)
	switch {
	case strings.Contains(value, "azurerm_"):
		return "Azure"
	case strings.Contains(value, "google_"):
		return "GCP"
	case strings.Contains(value, "aws_"):
		return "AWS"
	default:
		return ProviderUnknown
	}
}

func inferKind(resourceType string) string {
	t := strings.ToLower(strings.TrimSpace(resourceType))
	switch {
	case strings.Contains(t, "rds") || strings.Contains(t, "db_") || strings.Contains(t, "database") || strings.Contains(t, "postgres") || strings.Contains(t, "mysql") || strings.Contains(t, "mssql") || strings.Contains(t, "sql"):
		return KindDatabase
	case strings.Contains(t, "cluster") || strings.Contains(t, "kubernetes"):
		return KindCluster
	case strings.Contains(t, "instance") || strings.Contains(t, "virtual_machine") || strings.Contains(t, "compute"):
		return KindVM
	default:
		return ""
	}
}

func instanceAddress(resource tfStateResource, instance tfStateInstance) string {
	address := strings.TrimSpace(resource.Address)
	if address == "" {
		parts := []string{}
		if strings.TrimSpace(resource.Module) != "" {
			parts = append(parts, strings.TrimSpace(resource.Module))
		}
		parts = append(parts, strings.TrimSpace(resource.Type)+"."+strings.TrimSpace(resource.Name))
		address = strings.Join(parts, ".")
	}
	if instance.IndexKey == nil {
		return address
	}
	switch value := instance.IndexKey.(type) {
	case string:
		return address + "[" + value + "]"
	case float64:
		return address + "[" + strconv.FormatFloat(value, 'f', -1, 64) + "]"
	default:
		return address
	}
}

func firstString(attrs map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := findString(attrs, key); value != "" {
			return value
		}
	}
	return ""
}

func findString(value any, key string) string {
	switch typed := value.(type) {
	case map[string]any:
		for k, v := range typed {
			if strings.EqualFold(k, key) {
				if s, ok := v.(string); ok {
					return strings.TrimSpace(s)
				}
			}
		}
		for _, v := range typed {
			if s := findString(v, key); s != "" {
				return s
			}
		}
	case []any:
		for _, v := range typed {
			if s := findString(v, key); s != "" {
				return s
			}
		}
	}
	return ""
}

func tagName(attrs map[string]any) string {
	for _, key := range []string{"tags", "labels"} {
		raw, ok := attrs[key].(map[string]any)
		if !ok {
			continue
		}
		for _, tagKey := range []string{"Name", "name"} {
			if value, ok := raw[tagKey].(string); ok {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func appendTerraformLabels(labels, address string) string {
	parts := splitLabels(labels)
	for _, tag := range []string{"iac:terraform", "tf:" + sanitizeLabelValue(address)} {
		if tag == "tf:" {
			continue
		}
		if !containsLabel(parts, tag) {
			parts = append(parts, tag)
		}
	}
	return strings.Join(parts, ",")
}

func splitLabels(labels string) []string {
	var out []string
	for _, part := range strings.Split(labels, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func containsLabel(parts []string, want string) bool {
	for _, part := range parts {
		if strings.EqualFold(strings.TrimSpace(part), want) {
			return true
		}
	}
	return false
}

func sanitizeLabelValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, ",", "_")
	return value
}

func uniquePaths(paths []string) []string {
	seen := make(map[string]bool, len(paths))
	var out []string
	for _, path := range paths {
		path = expandPath(strings.TrimSpace(path))
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, path)
	}
	return out
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}
