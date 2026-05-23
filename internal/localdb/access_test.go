package localdb

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/vyoogam/cloudmanager/internal/core"
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

func TestResourceRowsRoundTrip(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	seenAt := time.Now().UTC().Truncate(time.Second)

	err := ReplaceResourceRows(ctx, db, []string{"vm"}, []ResourceRow{{
		ResourceType:   "vm",
		CacheKey:       "AWS|123|us-east-1|prod|i-123",
		Provider:       "AWS",
		ContextKey:     "AWS|123|us-east-1|prod",
		AccountID:      "123",
		AccountName:    "prod",
		Region:         "us-east-1",
		ResourceID:     "i-123",
		ResourceName:   "api",
		SearchableText: "api i-123 203.0.113.10",
		Tags:           "env=prod",
		PayloadJSON:    `{"name":"api"}`,
		SeenAt:         seenAt,
	}})
	if err != nil {
		t.Fatalf("replace resource rows: %v", err)
	}

	rows, total, err := LoadResourceRows(ctx, db, []string{"vm"}, 24*time.Hour)
	if err != nil {
		t.Fatalf("load resource rows: %v", err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("expected one row, total=%d len=%d", total, len(rows))
	}
	if rows[0].ResourceID != "i-123" || rows[0].PayloadJSON != `{"name":"api"}` || !rows[0].SeenAt.Equal(seenAt) {
		t.Fatalf("unexpected resource row: %+v", rows[0])
	}
}

func TestSummaryRowsRoundTrip(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)

	err := ReplaceSummaryRows(ctx, db, []string{"disks"}, []SummaryRow{{
		SummaryType: "disks",
		ContextKey:  "AWS|123|us-east-1|prod",
		Provider:    "AWS",
		AccountID:   "123",
		AccountName: "prod",
		Region:      "us-east-1",
		Count:       7,
		Extra:       2,
		UpdatedAt:   time.Now().UTC().Truncate(time.Second),
	}})
	if err != nil {
		t.Fatalf("replace summary rows: %v", err)
	}

	rows, total, err := LoadSummaryRows(ctx, db, []string{"disks"}, 24*time.Hour)
	if err != nil {
		t.Fatalf("load summary rows: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].Count != 7 || rows[0].Extra != 2 {
		t.Fatalf("unexpected summary rows total=%d rows=%+v", total, rows)
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
