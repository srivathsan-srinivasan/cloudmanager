package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"

	"cloudmanager/internal/core"
)

func TestManagedContextsExpandsRegions(t *testing.T) {
	cfg := AppConfig{
		CloudContexts: []ManagedCloudContext{
			{
				Provider:          "AWS",
				AccountID:         "9431",
				AccountName:       "main",
				CredentialProfile: "aws-main-9431",
				Regions:           []string{"eu-central-1", "ap-south-1"},
			},
			{
				Provider:    "Azure",
				AccountID:   "sub-1",
				AccountName: "core",
			},
		},
	}

	contexts := ManagedContexts(cfg)
	if len(contexts) != 3 {
		t.Fatalf("expected 3 contexts, got %d", len(contexts))
	}

	if contexts[0].Provider != "AWS" || contexts[0].Region != "ap-south-1" {
		t.Fatalf("expected first context to be sorted AWS ap-south-1, got %+v", contexts[0])
	}
	if contexts[2].Provider != "Azure" || contexts[2].Region != "global" {
		t.Fatalf("expected Azure default global region, got %+v", contexts[2])
	}
}

func TestLoadDefaultsResourceIndexPersistenceForExistingConfig(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	home := t.TempDir()
	t.Setenv("HOME", home)
	configPath := filepath.Join(home, ".cloudmanager.json")
	if err := os.WriteFile(configPath, []byte(`{"backend":"cli","vm_index_persistence_enabled":true}`), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := Load()

	if !cfg.ResourceIndexPersistence {
		t.Fatal("expected resource index persistence to default on for existing configs")
	}
	if cfg.ResourceIndexCacheTTL != 24 {
		t.Fatalf("expected default resource index ttl 24, got %d", cfg.ResourceIndexCacheTTL)
	}
}

func TestMergeDiscoveredContextsImportsOnlyUnmanagedProviders(t *testing.T) {
	cfg := AppConfig{
		CloudContexts: []ManagedCloudContext{
			{
				Provider:          "AWS",
				AccountID:         "9431",
				AccountName:       "main",
				CredentialProfile: "aws-main-9431",
				Regions:           []string{"ap-south-1"},
			},
		},
	}

	discovered := []core.CloudContext{
		{Provider: "AWS", AccountID: "9999", AccountName: "ignored", Region: "us-east-1", CredentialProfile: "aws-ignored"},
		{Provider: "GCP", AccountID: "project-a", AccountName: "project-a", Region: "global"},
		{Provider: "GCP", AccountID: "project-a", AccountName: "project-a", Region: "global"},
		{Provider: "Azure", AccountID: "sub-1", AccountName: "core", Region: "global"},
	}

	updated, changed := MergeDiscoveredContexts(cfg, discovered)
	if !changed {
		t.Fatal("expected discovered GCP/Azure contexts to be imported")
	}
	if len(updated.CloudContexts) != 3 {
		t.Fatalf("expected 3 managed groups after merge, got %d", len(updated.CloudContexts))
	}

	if updated.CloudContexts[1].Provider != "Azure" && updated.CloudContexts[2].Provider != "Azure" {
		t.Fatalf("expected Azure managed context to be present, got %+v", updated.CloudContexts)
	}
}

func TestReplaceManagedContextsForProvider(t *testing.T) {
	cfg := AppConfig{
		CloudContexts: []ManagedCloudContext{
			{Provider: "AWS", AccountID: "9431", Regions: []string{"ap-south-1"}},
			{Provider: "GCP", AccountID: "project-a", Regions: []string{"global"}},
		},
	}

	cfg = ReplaceManagedContextsForProvider(cfg, "GCP", []ManagedCloudContext{
		{Provider: "GCP", AccountID: "project-b"},
	})

	if len(cfg.CloudContexts) != 2 {
		t.Fatalf("expected 2 managed contexts after replace, got %d", len(cfg.CloudContexts))
	}
	if cfg.CloudContexts[1].AccountID != "project-b" || cfg.CloudContexts[1].Regions[0] != "global" {
		t.Fatalf("expected GCP replacement to default to global, got %+v", cfg.CloudContexts[1])
	}
}

func TestManagedContextsCarryAuthModeAndPersistence(t *testing.T) {
	cfg := AppConfig{
		CloudContexts: []ManagedCloudContext{
			{
				ContextName:           "prod-aws",
				Provider:              "AWS",
				AccountID:             "9431",
				AccountName:           "main",
				Tenant:                "tenant-a",
				AuthMode:              AuthModeAWSume,
				CredentialPersistence: CredentialPersistenceMemory,
				CredentialProfile:     "main-admin",
				Regions:               []string{"ap-south-1"},
			},
		},
	}

	contexts := ManagedContexts(cfg)
	if len(contexts) != 1 {
		t.Fatalf("expected one context, got %+v", contexts)
	}
	if contexts[0].AuthMode != AuthModeAWSume || contexts[0].CredentialScope != CredentialPersistenceMemory {
		t.Fatalf("expected auth metadata on context, got %+v", contexts[0])
	}
	if contexts[0].ContextName != "prod-aws" || contexts[0].Tenant != "tenant-a" {
		t.Fatalf("expected context alias and tenant on context, got %+v", contexts[0])
	}
}

func TestSanitizeManagedCloudContextDefaultsSecurely(t *testing.T) {
	ctx := SanitizeManagedCloudContext(ManagedCloudContext{Provider: "AWS", AccountName: "Prod Account"})
	if ctx.AuthMode != AuthModeNativeCLI {
		t.Fatalf("expected native CLI default, got %q", ctx.AuthMode)
	}
	if ctx.CredentialPersistence != CredentialPersistenceNativeCLI {
		t.Fatalf("expected native CLI persistence default, got %q", ctx.CredentialPersistence)
	}
	if ctx.ContextName != "prod-account" {
		t.Fatalf("expected default context name, got %q", ctx.ContextName)
	}

	jit := SanitizeManagedCloudContext(ManagedCloudContext{Provider: "AWS", AuthMode: AuthModeJIT})
	if jit.CredentialPersistence != CredentialPersistenceMemory {
		t.Fatalf("expected JIT auth to default to memory persistence, got %q", jit.CredentialPersistence)
	}
}

func TestResolveManagedContextAndCurrentContext(t *testing.T) {
	cfg := AppConfig{
		CurrentContext: "eng",
		CloudContexts: []ManagedCloudContext{
			{ContextName: "eng", Provider: "Azure", AccountID: "sub-1", AccountName: "Engineering", Tenant: "firecompass.com", Regions: []string{"global"}},
		},
	}

	managed, idx, ok := ResolveManagedContext(cfg, "eng")
	if !ok || idx != 0 {
		t.Fatalf("expected to resolve eng managed context, ok=%t idx=%d", ok, idx)
	}
	if managed.Tenant != "firecompass.com" {
		t.Fatalf("expected tenant on managed context, got %+v", managed)
	}
	current, ok := CurrentCloudContext(cfg)
	if !ok || current.ContextName != "eng" || current.AccountID != "sub-1" {
		t.Fatalf("expected current cloud context, got ok=%t ctx=%+v", ok, current)
	}
}

func TestSanitizeVMColumnsDeduplicates(t *testing.T) {
	columns := SanitizeVMColumns([]string{"Name", "Cost", "State", "Cost", "Name", "Labels"})

	expected := []string{"Name", "Cost", "State", "Labels"}
	if len(columns) != len(expected) {
		t.Fatalf("expected %d columns after sanitization, got %d: %+v", len(expected), len(columns), columns)
	}
	for i := range expected {
		if columns[i] != expected[i] {
			t.Fatalf("expected sanitized column %d to be %q, got %q", i, expected[i], columns[i])
		}
	}
}

func TestSanitizeDashboardWidgets(t *testing.T) {
	widgets := SanitizeDashboardWidgets([]string{"running_vms", "bad", "backend", "running_vms", " KUBERNETES "})
	expected := []string{"running_vms", "backend", "kubernetes"}
	if len(widgets) != len(expected) {
		t.Fatalf("expected %d dashboard widgets, got %d: %+v", len(expected), len(widgets), widgets)
	}
	for i := range expected {
		if widgets[i] != expected[i] {
			t.Fatalf("expected widget %d to be %q, got %q", i, expected[i], widgets[i])
		}
	}
}

func TestSanitizeDashboardWidgetsDefaultsWhenEmpty(t *testing.T) {
	widgets := SanitizeDashboardWidgets([]string{"bad"})
	if len(widgets) != len(DefaultDashboardWidgets) {
		t.Fatalf("expected default dashboard widgets, got %+v", widgets)
	}
}

func TestSanitizeDashboardWidgetsUpgradesLegacyDefaults(t *testing.T) {
	widgets := SanitizeDashboardWidgets([]string{
		"contexts",
		"indexed_vms",
		"running_vms",
		"stopped_vms",
		"public_ips",
		"backend",
		"databases",
		"kubernetes",
		"db_contexts",
	})
	for _, expected := range []string{"disks", "snapshots", "networks", "subnets", "firewalls", "terraform", "manual_hosts"} {
		found := false
		for _, widget := range widgets {
			if widget == expected {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected legacy default widgets to gain %q, got %+v", expected, widgets)
		}
	}
}

func TestSanitizeDashboardTheme(t *testing.T) {
	if got := SanitizeDashboardTheme(" NEON "); got != "neon" {
		t.Fatalf("expected neon dashboard theme, got %q", got)
	}
	if got := SanitizeDashboardTheme("unknown"); got != "btop" {
		t.Fatalf("expected unknown dashboard theme to default to btop, got %q", got)
	}
}

func TestSanitizeManualHostDefaultsAndTags(t *testing.T) {
	host := SanitizeManualHost(ManualHost{
		Host:       " 203.0.113.10 ",
		Connection: "",
		Tags:       []string{" Prod ", "prod", "Hetzner"},
	})

	if host.Provider != "Manual" {
		t.Fatalf("expected default Manual provider, got %q", host.Provider)
	}
	if host.Connection != "ssh" {
		t.Fatalf("expected default SSH connection, got %q", host.Connection)
	}
	expectedTags := []string{"prod", "hetzner"}
	if len(host.Tags) != len(expectedTags) {
		t.Fatalf("expected tags %+v, got %+v", expectedTags, host.Tags)
	}
	for i := range expectedTags {
		if host.Tags[i] != expectedTags[i] {
			t.Fatalf("expected tag %d to be %q, got %q", i, expectedTags[i], host.Tags[i])
		}
	}
}

func TestManagedContextsIncludesManualContextWhenHostsExist(t *testing.T) {
	cfg := AppConfig{
		ManualHosts: []ManualHost{{Name: "hetzner-web", Host: "203.0.113.10", Username: "root"}},
	}

	contexts := ManagedContexts(cfg)
	if len(contexts) != 1 {
		t.Fatalf("expected one manual context, got %+v", contexts)
	}
	if contexts[0].Provider != "Manual" || contexts[0].AccountID != "manual-hosts" || contexts[0].AuthMode != AuthModeManual {
		t.Fatalf("expected synthetic manual context, got %+v", contexts[0])
	}
}

func TestApplyResourceTagsToVMMergesCloudManagerTags(t *testing.T) {
	ctx := core.CloudContext{Provider: "GCP", AccountID: "project-a", AccountName: "project-a", Region: "us-central1"}
	vm := core.VM{Name: "vf-web-1", ID: "gce-1", PublicIP: "203.0.113.10", Labels: "env=prod"}
	cfg := AppConfig{
		ResourceTags: []ResourceTag{
			{Provider: "GCP", AccountID: "project-a", ResourceID: "gce-1", Tags: []string{"VFWEB", "vfweb"}},
			{Provider: "AWS", AccountID: "1234", ResourceID: "gce-1", Tags: []string{"wrong"}},
		},
	}

	tagged := ApplyResourceTagsToVM(cfg, ctx, vm)
	if tagged.Labels != "env=prod,cm:VFWEB" {
		t.Fatalf("expected CloudManager tag overlay, got %q", tagged.Labels)
	}
}

func TestUpsertVMResourceTagsMergesForSameResource(t *testing.T) {
	ctx := core.CloudContext{Provider: "AWS", AccountID: "1234", AccountName: "prod", Region: "us-east-1"}
	vm := core.VM{Name: "web", ID: "i-123", PrivateIP: "10.0.0.5"}

	cfg := UpsertVMResourceTags(AppConfig{}, ctx, vm, []string{"VFWEB"})
	cfg = UpsertVMResourceTags(cfg, ctx, vm, []string{"prod", "vfweb"})

	if len(cfg.ResourceTags) != 1 {
		t.Fatalf("expected one resource tag entry, got %+v", cfg.ResourceTags)
	}
	expected := []string{"VFWEB", "prod"}
	if len(cfg.ResourceTags[0].Tags) != len(expected) {
		t.Fatalf("expected tags %+v, got %+v", expected, cfg.ResourceTags[0].Tags)
	}
	for i := range expected {
		if cfg.ResourceTags[0].Tags[i] != expected[i] {
			t.Fatalf("expected tag %d to be %q, got %q", i, expected[i], cfg.ResourceTags[0].Tags[i])
		}
	}
}

func TestResourceTagsAreKindScopedAndReusable(t *testing.T) {
	ctx := core.CloudContext{Provider: "AWS", AccountID: "1234", AccountName: "prod", Region: "us-east-1"}
	cfg := AppConfig{}
	cfg = UpsertResourceTags(cfg, ctx, ResourceTagTarget{Kind: "disk", ID: "shared-id", Name: "data"}, []string{"VFWEB"})
	cfg = UpsertResourceTags(cfg, ctx, ResourceTagTarget{Kind: "storage", ID: "bucket-1", Name: "logs"}, []string{"archive"})

	vm := ApplyResourceTagsToVM(cfg, ctx, core.VM{Name: "web", ID: "shared-id", Labels: "env=prod"})
	if vm.Labels != "env=prod" {
		t.Fatalf("disk tag should not apply to VM with same ID, got %q", vm.Labels)
	}

	disks := ApplyResourceTagsToDisks(cfg, ctx, []core.Disk{{Name: "data", ID: "shared-id", Labels: "tier=ssd"}})
	if len(disks) != 1 || disks[0].Labels != "tier=ssd,cm:VFWEB" {
		t.Fatalf("expected disk CloudManager tag overlay, got %+v", disks)
	}

	buckets := ApplyResourceTagsToStorageBuckets(cfg, ctx, []core.StorageBucket{{Name: "logs", ID: "bucket-1"}})
	if len(buckets) != 1 || buckets[0].Labels != "cm:archive" {
		t.Fatalf("expected storage CloudManager tag overlay, got %+v", buckets)
	}
}
