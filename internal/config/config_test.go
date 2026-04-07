package config

import (
	"testing"

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
