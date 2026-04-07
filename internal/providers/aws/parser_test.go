package aws

import (
	"testing"

	"gopkg.in/ini.v1"
)

func TestLoadContextsFromConfigGroupsRegionsUnderAccounts(t *testing.T) {
	cfg, err := ini.Load([]byte(`
[profile 9824-use1]
source_profile = aws-9824-c2shell
region = us-east-1

[profile 9824-aps1]
source_profile = aws-9824-c2shell
region = ap-south-1

[profile main-aps1]
source_profile = aws-main-9431
region = ap-south-1

[profile main-euc1]
source_profile = aws-main-9431
region = eu-central-1

[default]
region = us-east-2
`))
	if err != nil {
		t.Fatalf("failed to load test config: %v", err)
	}

	contexts, warnings := loadContextsFromConfig(cfg)
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(contexts) != 4 {
		t.Fatalf("expected 4 region contexts, got %d", len(contexts))
	}

	expected := map[string]struct {
		name    string
		profile string
	}{
		"9431|ap-south-1":   {name: "main", profile: "aws-main-9431"},
		"9431|eu-central-1": {name: "main", profile: "aws-main-9431"},
		"9824|ap-south-1":   {name: "c2shell", profile: "aws-9824-c2shell"},
		"9824|us-east-1":    {name: "c2shell", profile: "aws-9824-c2shell"},
	}

	for _, ctx := range contexts {
		key := ctx.AccountID + "|" + ctx.Region
		want, ok := expected[key]
		if !ok {
			t.Fatalf("unexpected context returned: %+v", ctx)
		}
		if ctx.AccountName != want.name {
			t.Fatalf("expected account name %q for %s, got %q", want.name, key, ctx.AccountName)
		}
		if ctx.CredentialProfile != want.profile {
			t.Fatalf("expected credential profile %q for %s, got %q", want.profile, key, ctx.CredentialProfile)
		}
	}
}

func TestLoadContextsFromConfigFallsBackToDefaultRegion(t *testing.T) {
	cfg, err := ini.Load([]byte(`
[profile shared]
source_profile = aws-7777-platform

[default]
region = us-west-2
`))
	if err != nil {
		t.Fatalf("failed to load test config: %v", err)
	}

	contexts, warnings := loadContextsFromConfig(cfg)
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(contexts) != 1 {
		t.Fatalf("expected 1 context, got %d", len(contexts))
	}

	ctx := contexts[0]
	if ctx.AccountID != "7777" {
		t.Fatalf("expected account id 7777, got %q", ctx.AccountID)
	}
	if ctx.AccountName != "platform" {
		t.Fatalf("expected account name platform, got %q", ctx.AccountName)
	}
	if ctx.Region != "us-west-2" {
		t.Fatalf("expected region us-west-2, got %q", ctx.Region)
	}
	if ctx.CredentialProfile != "aws-7777-platform" {
		t.Fatalf("expected credential profile aws-7777-platform, got %q", ctx.CredentialProfile)
	}
}
