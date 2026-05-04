package localdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"cloudmanager/internal/core"
)

type AccessProfile struct {
	Provider        string
	ContextKey      string
	Region          string
	ResourceID      string
	ResourceName    string
	Username        string
	IPKind          string
	IP              string
	KeyPath         string
	DefaultKeyReady bool
	LastSuccessAt   time.Time
}

func Path() string {
	if path := strings.TrimSpace(os.Getenv("CLOUDMANAGER_DB_PATH")); path != "" {
		return expandPath(path)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".cloudmanager.db"
	}
	return filepath.Join(home, ".cloudmanager.db")
}

func Open(ctx context.Context) (*sql.DB, error) {
	path := Path()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := Migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func Migrate(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS access_profiles (
  provider TEXT NOT NULL,
  context_key TEXT NOT NULL,
  region TEXT NOT NULL,
  resource_id TEXT NOT NULL,
  resource_name TEXT NOT NULL,
  username TEXT NOT NULL,
  ip_kind TEXT NOT NULL,
  ip TEXT NOT NULL,
  key_path TEXT NOT NULL,
  default_key_ready INTEGER NOT NULL DEFAULT 0,
  last_success_at TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (provider, context_key, region, resource_id)
);
CREATE INDEX IF NOT EXISTS idx_access_profiles_resource_name ON access_profiles(provider, context_key, region, resource_name);
`)
	return err
}

func LookupAccessProfile(ctx context.Context, db *sql.DB, cloudCtx core.CloudContext, vm core.VM) (AccessProfile, bool, error) {
	profile, ok, err := lookupAccessProfileByID(ctx, db, cloudCtx, vm)
	if err != nil || ok {
		return profile, ok, err
	}
	return lookupAccessProfileByName(ctx, db, cloudCtx, vm)
}

func UpsertAccessProfile(ctx context.Context, db *sql.DB, cloudCtx core.CloudContext, vm core.VM, profile AccessProfile) error {
	profile = sanitizeAccessProfile(cloudCtx, vm, profile)
	if profile.ResourceID == "" && profile.ResourceName == "" {
		return errors.New("access profile needs resource identity")
	}
	if profile.IP == "" {
		return errors.New("access profile needs ip")
	}
	if profile.Username == "" {
		profile.Username = "ubuntu"
	}
	if profile.LastSuccessAt.IsZero() {
		profile.LastSuccessAt = time.Now()
	}
	_, err := db.ExecContext(ctx, `
INSERT INTO access_profiles (
  provider, context_key, region, resource_id, resource_name,
  username, ip_kind, ip, key_path, default_key_ready, last_success_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(provider, context_key, region, resource_id) DO UPDATE SET
  resource_name=excluded.resource_name,
  username=excluded.username,
  ip_kind=excluded.ip_kind,
  ip=excluded.ip,
  key_path=excluded.key_path,
  default_key_ready=excluded.default_key_ready,
  last_success_at=excluded.last_success_at,
  updated_at=CURRENT_TIMESTAMP
`, profile.Provider, profile.ContextKey, profile.Region, profile.ResourceID, profile.ResourceName,
		profile.Username, profile.IPKind, profile.IP, profile.KeyPath, boolInt(profile.DefaultKeyReady), profile.LastSuccessAt.Format(time.RFC3339))
	return err
}

func LearnedAccessMethod(profile AccessProfile) (core.AccessMethod, bool) {
	profile.Username = strings.TrimSpace(profile.Username)
	profile.IP = strings.TrimSpace(profile.IP)
	if profile.Username == "" || profile.IP == "" {
		return core.AccessMethod{}, false
	}
	target := profile.Username + "@" + profile.IP
	args := []string{"ssh", target}
	label := fmt.Sprintf("Learned SSH (%s)", strings.TrimSpace(profile.IPKind))
	if !profile.DefaultKeyReady {
		keyPath := strings.TrimSpace(profile.KeyPath)
		if keyPath == "" {
			return core.AccessMethod{}, false
		}
		args = []string{"ssh", "-i", keyPath, target}
		label = fmt.Sprintf("Learned SSH with %s (%s)", filepath.Base(keyPath), strings.TrimSpace(profile.IPKind))
	}
	return core.AccessMethod{
		ID:        "learned-ssh",
		Kind:      "learned_ssh",
		Label:     label,
		Priority:  5,
		Command:   args,
		CopyText:  formatCommand(args),
		Available: true,
	}, true
}

func lookupAccessProfileByID(ctx context.Context, db *sql.DB, cloudCtx core.CloudContext, vm core.VM) (AccessProfile, bool, error) {
	if strings.TrimSpace(vm.ID) == "" {
		return AccessProfile{}, false, nil
	}
	return scanAccessProfile(db.QueryRowContext(ctx, `
SELECT provider, context_key, region, resource_id, resource_name, username, ip_kind, ip, key_path, default_key_ready, last_success_at
FROM access_profiles
WHERE provider=? AND context_key=? AND region=? AND resource_id=?
`, strings.TrimSpace(cloudCtx.Provider), cloudCtx.CacheKey(), strings.TrimSpace(cloudCtx.Region), strings.TrimSpace(vm.ID)))
}

func lookupAccessProfileByName(ctx context.Context, db *sql.DB, cloudCtx core.CloudContext, vm core.VM) (AccessProfile, bool, error) {
	if strings.TrimSpace(vm.Name) == "" {
		return AccessProfile{}, false, nil
	}
	return scanAccessProfile(db.QueryRowContext(ctx, `
SELECT provider, context_key, region, resource_id, resource_name, username, ip_kind, ip, key_path, default_key_ready, last_success_at
FROM access_profiles
WHERE provider=? AND context_key=? AND region=? AND resource_name=?
ORDER BY updated_at DESC
LIMIT 1
`, strings.TrimSpace(cloudCtx.Provider), cloudCtx.CacheKey(), strings.TrimSpace(cloudCtx.Region), strings.TrimSpace(vm.Name)))
}

func scanAccessProfile(row *sql.Row) (AccessProfile, bool, error) {
	var profile AccessProfile
	var defaultReady int
	var lastSuccess string
	err := row.Scan(&profile.Provider, &profile.ContextKey, &profile.Region, &profile.ResourceID, &profile.ResourceName,
		&profile.Username, &profile.IPKind, &profile.IP, &profile.KeyPath, &defaultReady, &lastSuccess)
	if errors.Is(err, sql.ErrNoRows) {
		return AccessProfile{}, false, nil
	}
	if err != nil {
		return AccessProfile{}, false, err
	}
	profile.DefaultKeyReady = defaultReady == 1
	if parsed, err := time.Parse(time.RFC3339, lastSuccess); err == nil {
		profile.LastSuccessAt = parsed
	}
	return profile, true, nil
}

func sanitizeAccessProfile(cloudCtx core.CloudContext, vm core.VM, profile AccessProfile) AccessProfile {
	profile.Provider = strings.TrimSpace(cloudCtx.Provider)
	profile.ContextKey = cloudCtx.CacheKey()
	profile.Region = strings.TrimSpace(cloudCtx.Region)
	profile.ResourceID = strings.TrimSpace(vm.ID)
	profile.ResourceName = strings.TrimSpace(vm.Name)
	profile.Username = strings.TrimSpace(profile.Username)
	profile.IPKind = strings.ToLower(strings.TrimSpace(profile.IPKind))
	profile.IP = strings.TrimSpace(profile.IP)
	profile.KeyPath = expandPath(profile.KeyPath)
	return profile
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func expandPath(path string) string {
	path = os.ExpandEnv(strings.TrimSpace(path))
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func formatCommand(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "" {
			quoted = append(quoted, "''")
			continue
		}
		if !strings.ContainsAny(arg, " \t\n'\"\\$&;|<>`(){}[]*?!") {
			quoted = append(quoted, arg)
			continue
		}
		quoted = append(quoted, "'"+strings.ReplaceAll(arg, "'", "'\"'\"'")+"'")
	}
	return strings.Join(quoted, " ")
}
