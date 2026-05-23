package main

import (
	"testing"

	"github.com/vyoogam/cloudmanager/internal/config"
)

func TestAddAndUseAzureProfile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := config.AppConfig{}

	addProfile(cfg, []string{
		"eng",
		"--provider", "azure",
		"--tenant", "example.com",
		"--subscription-id", "00000000-0000-0000-0000-000000000000",
		"--subscription-name", "Azure Sponsorship - Engineering",
	})

	loaded := config.Load()
	if len(loaded.CloudContexts) != 1 {
		t.Fatalf("expected one profile, got %+v", loaded.CloudContexts)
	}
	ctx := loaded.CloudContexts[0]
	if ctx.ContextName != "eng" || ctx.Provider != "Azure" || ctx.AccountID != "00000000-0000-0000-0000-000000000000" {
		t.Fatalf("unexpected saved profile: %+v", ctx)
	}
	if loaded.CurrentContext != "eng" {
		t.Fatalf("expected first added profile to become current, got %q", loaded.CurrentContext)
	}

	useProfile(loaded, "eng")
	loaded = config.Load()
	if loaded.CurrentContext != "eng" {
		t.Fatalf("expected profile use to persist current context, got %q", loaded.CurrentContext)
	}
}
