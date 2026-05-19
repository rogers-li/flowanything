package configadapter

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	coreconfig "flow-anything/core/config"
)

type SQLDialect string

const (
	SQLDialectSQLite SQLDialect = "sqlite"
	SQLDialectMySQL  SQLDialect = "mysql"
)

// SQLBundleStore stores config bundles in a small relational table. The SQL is
// intentionally boring: one JSON document per bundle id. Runtime semantics stay
// in core/config, not in the database schema.
type SQLBundleStore struct {
	DB      *sql.DB
	Dialect SQLDialect
	NowFn   func() time.Time
}

func NewSQLBundleStore(db *sql.DB, dialect SQLDialect) SQLBundleStore {
	return SQLBundleStore{DB: db, Dialect: dialect, NowFn: time.Now}
}

func (s SQLBundleStore) EnsureSchema(ctx context.Context) error {
	if s.DB == nil {
		return fmt.Errorf("db is required")
	}
	_, err := s.DB.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS runtime_bundles (
  id VARCHAR(191) PRIMARY KEY,
  name TEXT NOT NULL,
  version TEXT NOT NULL,
  data TEXT NOT NULL,
  updated_at TEXT NOT NULL
)`)
	return err
}

func (s SQLBundleStore) LoadBundle(ctx context.Context, id string) (coreconfig.BundleSpec, error) {
	if s.DB == nil {
		return coreconfig.BundleSpec{}, fmt.Errorf("db is required")
	}
	if id == "" {
		return coreconfig.BundleSpec{}, fmt.Errorf("bundle id is required")
	}
	var data string
	err := s.DB.QueryRowContext(ctx, `SELECT data FROM runtime_bundles WHERE id = ?`, id).Scan(&data)
	if err == sql.ErrNoRows {
		return coreconfig.BundleSpec{}, fmt.Errorf("bundle %q not found", id)
	}
	if err != nil {
		return coreconfig.BundleSpec{}, err
	}
	return coreconfig.LoadBundleJSON(bytes.NewBufferString(data))
}

func (s SQLBundleStore) SaveBundle(ctx context.Context, bundle coreconfig.BundleSpec) error {
	if s.DB == nil {
		return fmt.Errorf("db is required")
	}
	if err := validateBundleForStore(bundle); err != nil {
		return err
	}
	buffer := bytes.Buffer{}
	if err := coreconfig.WriteBundleJSON(&buffer, bundle); err != nil {
		return err
	}
	updatedAt := s.now().UTC().Format(time.RFC3339Nano)
	switch s.dialect() {
	case SQLDialectMySQL:
		_, err := s.DB.ExecContext(ctx, `
INSERT INTO runtime_bundles (id, name, version, data, updated_at)
VALUES (?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  version = VALUES(version),
  data = VALUES(data),
  updated_at = VALUES(updated_at)`,
			bundle.ID, bundle.Name, bundle.Version, buffer.String(), updatedAt,
		)
		return err
	default:
		_, err := s.DB.ExecContext(ctx, `
INSERT INTO runtime_bundles (id, name, version, data, updated_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  name = excluded.name,
  version = excluded.version,
  data = excluded.data,
  updated_at = excluded.updated_at`,
			bundle.ID, bundle.Name, bundle.Version, buffer.String(), updatedAt,
		)
		return err
	}
}

func (s SQLBundleStore) DeleteBundle(ctx context.Context, id string) error {
	if s.DB == nil {
		return fmt.Errorf("db is required")
	}
	if id == "" {
		return fmt.Errorf("bundle id is required")
	}
	result, err := s.DB.ExecContext(ctx, `DELETE FROM runtime_bundles WHERE id = ?`, id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err == nil && rows == 0 {
		return fmt.Errorf("bundle %q not found", id)
	}
	return nil
}

func (s SQLBundleStore) ListBundles(ctx context.Context) ([]BundleSummary, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("db is required")
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT data, updated_at FROM runtime_bundles`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []BundleSummary{}
	for rows.Next() {
		var data string
		var updatedAt string
		if err := rows.Scan(&data, &updatedAt); err != nil {
			return nil, err
		}
		bundle, err := coreconfig.LoadBundleJSON(bytes.NewBufferString(data))
		if err != nil {
			return nil, err
		}
		summary := summaryFromBundle(bundle, time.Time{})
		if parsed, err := time.Parse(time.RFC3339Nano, updatedAt); err == nil {
			summary.UpdatedAt = parsed
		}
		out = append(out, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s SQLBundleStore) dialect() SQLDialect {
	if s.Dialect == "" {
		return SQLDialectSQLite
	}
	return s.Dialect
}

func (s SQLBundleStore) now() time.Time {
	if s.NowFn != nil {
		return s.NowFn()
	}
	return time.Now()
}
