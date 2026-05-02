package main

import (
	"testing"

	"cloudmanager/internal/config"
)

func TestAddAndUseAzureProfile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := config.AppConfig{}

	addProfile(cfg, []string{
		"eng",
		"--provider", "azure",
		"--tenant", "firecompass.com",
		"--subscription-id", "34e5c3ad-42a8-420f-9999-474a02d91149",
		"--subscription-name", "Azure Sponsorship - Engineering",
	})

	loaded := config.Load()
	if len(loaded.CloudContexts) != 1 {
		t.Fatalf("expected one profile, got %+v", loaded.CloudContexts)
	}
	ctx := loaded.CloudContexts[0]
	if ctx.ContextName != "eng" || ctx.Provider != "Azure" || ctx.AccountID != "34e5c3ad-42a8-420f-9999-474a02d91149" {
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
