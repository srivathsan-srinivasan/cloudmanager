package localdb

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"cloudmanager/internal/core"
)

func TestAccessProfileRoundTrip(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	cloudCtx := core.CloudContext{Provider: "AWS", AccountName: "main", AccountID: "123", Region: "ap-south-1"}
	vm := core.VM{Name: "web", ID: "i-123", PublicIP: "203.0.113.10", PrivateIP: "10.0.0.5"}

	err := UpsertAccessProfile(ctx, db, cloudCtx, vm, AccessProfile{
		Username:      "ubuntu",
		IPKind:        "public",
		IP:            "203.0.113.10",
		KeyPath:       "~/sshkeys/prod.pem",
		LastSuccessAt: time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	profile, ok, err := LookupAccessProfile(ctx, db, cloudCtx, vm)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if !ok {
		t.Fatal("expected profile")
	}
	if profile.Username != "ubuntu" || profile.IPKind != "public" || profile.IP != "203.0.113.10" {
		t.Fatalf("unexpected profile: %+v", profile)
	}
	if !strings.HasSuffix(profile.KeyPath, filepath.Join("sshkeys", "prod.pem")) {
		t.Fatalf("expected expanded key path, got %q", profile.KeyPath)
	}
}

func TestLearnedAccessMethodUsesDefaultKeyReady(t *testing.T) {
	method, ok := LearnedAccessMethod(AccessProfile{
		Username:        "ec2-user",
		IPKind:          "private",
		IP:              "10.0.0.5",
		DefaultKeyReady: true,
	})
	if !ok {
		t.Fatal("expected learned method")
	}
	if method.CopyText != "ssh ec2-user@10.0.0.5" {
		t.Fatalf("expected direct ssh without key, got %q", method.CopyText)
	}
}

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "cloudmanager.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}
