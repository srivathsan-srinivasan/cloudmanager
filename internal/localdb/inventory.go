package localdb

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

type ResourceRow struct {
	ResourceType   string
	CacheKey       string
	Provider       string
	ContextKey     string
	AccountID      string
	AccountName    string
	Region         string
	ResourceID     string
	ResourceName   string
	SearchableText string
	Tags           string
	PayloadJSON    string
	SeenAt         time.Time
}

type SummaryRow struct {
	SummaryType string
	ContextKey  string
	Provider    string
	AccountID   string
	AccountName string
	Region      string
	Count       int
	Extra       int
	UpdatedAt   time.Time
}

func ReplaceResourceRows(ctx context.Context, db *sql.DB, resourceTypes []string, rows []ResourceRow) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	for _, resourceType := range resourceTypes {
		if _, err = tx.ExecContext(ctx, `DELETE FROM resource_inventory WHERE resource_type=?`, strings.TrimSpace(resourceType)); err != nil {
			return err
		}
	}
	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO resource_inventory (
  resource_type, cache_key, provider, context_key, account_id, account_name,
  region, resource_id, resource_name, searchable_text, tags, payload_json, seen_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(resource_type, cache_key) DO UPDATE SET
  provider=excluded.provider,
  context_key=excluded.context_key,
  account_id=excluded.account_id,
  account_name=excluded.account_name,
  region=excluded.region,
  resource_id=excluded.resource_id,
  resource_name=excluded.resource_name,
  searchable_text=excluded.searchable_text,
  tags=excluded.tags,
  payload_json=excluded.payload_json,
  seen_at=excluded.seen_at,
  updated_at=CURRENT_TIMESTAMP
`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	now := time.Now()
	for _, row := range rows {
		row = sanitizeResourceRow(row)
		if row.SeenAt.IsZero() {
			row.SeenAt = now
		}
		if _, err = stmt.ExecContext(ctx,
			row.ResourceType, row.CacheKey, row.Provider, row.ContextKey, row.AccountID, row.AccountName,
			row.Region, row.ResourceID, row.ResourceName, row.SearchableText, row.Tags, row.PayloadJSON, row.SeenAt.Format(time.RFC3339Nano),
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func LoadResourceRows(ctx context.Context, db *sql.DB, resourceTypes []string, ttl time.Duration) ([]ResourceRow, int, error) {
	var rows []ResourceRow
	total := 0
	now := time.Now()
	for _, resourceType := range resourceTypes {
		resourceType = strings.TrimSpace(resourceType)
		if resourceType == "" {
			continue
		}
		result, err := db.QueryContext(ctx, `
SELECT resource_type, cache_key, provider, context_key, account_id, account_name,
       region, resource_id, resource_name, searchable_text, tags, payload_json, seen_at
FROM resource_inventory
WHERE resource_type=?
ORDER BY provider, context_key, region, resource_name, resource_id
`, resourceType)
		if err != nil {
			return nil, 0, err
		}
		for result.Next() {
			total++
			var row ResourceRow
			var seenAt string
			if err := result.Scan(&row.ResourceType, &row.CacheKey, &row.Provider, &row.ContextKey, &row.AccountID, &row.AccountName,
				&row.Region, &row.ResourceID, &row.ResourceName, &row.SearchableText, &row.Tags, &row.PayloadJSON, &seenAt); err != nil {
				_ = result.Close()
				return nil, 0, err
			}
			row.SeenAt = parseDBTime(seenAt)
			if ttl > 0 && !row.SeenAt.IsZero() && now.Sub(row.SeenAt) > ttl {
				continue
			}
			rows = append(rows, row)
		}
		if err := result.Err(); err != nil {
			_ = result.Close()
			return nil, 0, err
		}
		if err := result.Close(); err != nil {
			return nil, 0, err
		}
	}
	return rows, total, nil
}

func ReplaceSummaryRows(ctx context.Context, db *sql.DB, summaryTypes []string, rows []SummaryRow) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	for _, summaryType := range summaryTypes {
		if _, err = tx.ExecContext(ctx, `DELETE FROM resource_summaries WHERE summary_type=?`, strings.TrimSpace(summaryType)); err != nil {
			return err
		}
	}
	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO resource_summaries (
  summary_type, context_key, provider, account_id, account_name, region, count, extra, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(summary_type, context_key) DO UPDATE SET
  provider=excluded.provider,
  account_id=excluded.account_id,
  account_name=excluded.account_name,
  region=excluded.region,
  count=excluded.count,
  extra=excluded.extra,
  updated_at=excluded.updated_at
`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	now := time.Now()
	for _, row := range rows {
		row = sanitizeSummaryRow(row)
		if row.UpdatedAt.IsZero() {
			row.UpdatedAt = now
		}
		if _, err = stmt.ExecContext(ctx, row.SummaryType, row.ContextKey, row.Provider, row.AccountID, row.AccountName, row.Region, row.Count, row.Extra, row.UpdatedAt.Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func LoadSummaryRows(ctx context.Context, db *sql.DB, summaryTypes []string, ttl time.Duration) ([]SummaryRow, int, error) {
	var rows []SummaryRow
	total := 0
	now := time.Now()
	for _, summaryType := range summaryTypes {
		summaryType = strings.TrimSpace(summaryType)
		if summaryType == "" {
			continue
		}
		result, err := db.QueryContext(ctx, `
SELECT summary_type, context_key, provider, account_id, account_name, region, count, extra, updated_at
FROM resource_summaries
WHERE summary_type=?
ORDER BY provider, context_key, region
`, summaryType)
		if err != nil {
			return nil, 0, err
		}
		for result.Next() {
			total++
			var row SummaryRow
			var updatedAt string
			if err := result.Scan(&row.SummaryType, &row.ContextKey, &row.Provider, &row.AccountID, &row.AccountName, &row.Region, &row.Count, &row.Extra, &updatedAt); err != nil {
				_ = result.Close()
				return nil, 0, err
			}
			row.UpdatedAt = parseDBTime(updatedAt)
			if ttl > 0 && !row.UpdatedAt.IsZero() && now.Sub(row.UpdatedAt) > ttl {
				continue
			}
			rows = append(rows, row)
		}
		if err := result.Err(); err != nil {
			_ = result.Close()
			return nil, 0, err
		}
		if err := result.Close(); err != nil {
			return nil, 0, err
		}
	}
	return rows, total, nil
}

func sanitizeResourceRow(row ResourceRow) ResourceRow {
	row.ResourceType = strings.TrimSpace(row.ResourceType)
	row.CacheKey = strings.TrimSpace(row.CacheKey)
	row.Provider = strings.TrimSpace(row.Provider)
	row.ContextKey = strings.TrimSpace(row.ContextKey)
	row.AccountID = strings.TrimSpace(row.AccountID)
	row.AccountName = strings.TrimSpace(row.AccountName)
	row.Region = strings.TrimSpace(row.Region)
	row.ResourceID = strings.TrimSpace(row.ResourceID)
	row.ResourceName = strings.TrimSpace(row.ResourceName)
	row.SearchableText = strings.TrimSpace(row.SearchableText)
	row.Tags = strings.TrimSpace(row.Tags)
	row.PayloadJSON = strings.TrimSpace(row.PayloadJSON)
	return row
}

func sanitizeSummaryRow(row SummaryRow) SummaryRow {
	row.SummaryType = strings.TrimSpace(row.SummaryType)
	row.ContextKey = strings.TrimSpace(row.ContextKey)
	row.Provider = strings.TrimSpace(row.Provider)
	row.AccountID = strings.TrimSpace(row.AccountID)
	row.AccountName = strings.TrimSpace(row.AccountName)
	row.Region = strings.TrimSpace(row.Region)
	return row
}

func parseDBTime(value string) time.Time {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}
