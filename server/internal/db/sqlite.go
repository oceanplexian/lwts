package db

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var placeholderRe = regexp.MustCompile(`\$(\d+)`)

// convertPlaceholders rewrites $1, $2, ... to ? for SQLite.
func convertPlaceholders(query string) string {
	return placeholderRe.ReplaceAllString(query, "?")
}

// convertTimeArgs converts time.Time args to ISO8601 strings for SQLite.
func convertTimeArgs(args []any) []any {
	out := make([]any, len(args))
	for i, a := range args {
		if t, ok := a.(time.Time); ok {
			out[i] = t.Format(time.RFC3339Nano)
		} else {
			out[i] = a
		}
	}
	return out
}

// timeFormats to try when parsing SQLite time strings back to time.Time.
var timeFormats = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

// wrapSQLiteScan wraps a scan function to auto-parse time strings into *time.Time destinations.
func wrapSQLiteScan(baseScan func(dest ...any) error) func(dest ...any) error {
	return func(dest ...any) error {
		// Create shadow destinations: for *time.Time targets, scan into *string instead
		shadow := make([]any, len(dest))
		timeIdxs := map[int]*string{}
		for i, d := range dest {
			if _, ok := d.(*time.Time); ok {
				s := new(string)
				shadow[i] = s
				timeIdxs[i] = s
			} else {
				shadow[i] = d
			}
		}

		if err := baseScan(shadow...); err != nil {
			return err
		}

		// Convert scanned strings back to time.Time
		for i, s := range timeIdxs {
			tp := dest[i].(*time.Time)
			if *s == "" {
				*tp = time.Time{}
				continue
			}
			var parsed bool
			for _, fmt := range timeFormats {
				if t, err := time.Parse(fmt, *s); err == nil {
					*tp = t
					parsed = true
					break
				}
			}
			if !parsed {
				return fmt.Errorf("cannot parse time %q", *s)
			}
		}
		return nil
	}
}

type SQLiteDatasource struct {
	db *sql.DB
}

func NewSQLiteDatasource(dsn string) (*SQLiteDatasource, error) {
	dsn = strings.TrimPrefix(dsn, "sqlite://")
	if dsn == "" {
		dsn = ":memory:"
	}

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=30000",
	} {
		if _, err := sqlDB.Exec(pragma); err != nil {
			sqlDB.Close()
			return nil, fmt.Errorf("sqlite pragma %q: %w", pragma, err)
		}
	}

	return &SQLiteDatasource{db: sqlDB}, nil
}

func (s *SQLiteDatasource) DBType() string { return "sqlite" }

func (s *SQLiteDatasource) Exec(ctx context.Context, query string, args ...any) (int64, error) {
	result, err := s.db.ExecContext(ctx, convertPlaceholders(query), convertTimeArgs(args)...)
	if err != nil {
		return 0, err
	}
	n, _ := result.RowsAffected()
	return n, nil
}

func (s *SQLiteDatasource) Query(ctx context.Context, query string, args ...any) (*Rows, error) {
	rows, err := s.db.QueryContext(ctx, convertPlaceholders(query), convertTimeArgs(args)...)
	if err != nil {
		return nil, err
	}
	return wrapSQLiteRows(rows), nil
}

func (s *SQLiteDatasource) QueryRow(ctx context.Context, query string, args ...any) *Row {
	row := s.db.QueryRowContext(ctx, convertPlaceholders(query), convertTimeArgs(args)...)
	return NewRow(func(dest ...any) error {
		scan := wrapSQLiteScan(row.Scan)
		err := scan(dest...)
		if err != nil && err.Error() == "sql: no rows in result set" {
			return ErrNoRows
		}
		return err
	})
}

func (s *SQLiteDatasource) Begin(ctx context.Context) (Tx, error) {
	// Use IMMEDIATE isolation so the write lock is acquired at BEGIN, not
	// at the first write. This lets concurrent writers queue via busy_timeout
	// instead of getting SQLITE_BUSY mid-transaction.
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		conn.Close()
		return nil, err
	}
	return &sqliteImmediateTx{conn: conn}, nil
}

func (s *SQLiteDatasource) Close() error {
	return s.db.Close()
}

func (s *SQLiteDatasource) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *SQLiteDatasource) DB() *sql.DB {
	return s.db
}

// sqliteTx wraps sql.Tx to implement the Tx interface.
type sqliteTx struct {
	tx *sql.Tx
}

func (t *sqliteTx) Exec(ctx context.Context, query string, args ...any) (int64, error) {
	result, err := t.tx.ExecContext(ctx, convertPlaceholders(query), convertTimeArgs(args)...)
	if err != nil {
		return 0, err
	}
	n, _ := result.RowsAffected()
	return n, nil
}

func (t *sqliteTx) Query(ctx context.Context, query string, args ...any) (*Rows, error) {
	rows, err := t.tx.QueryContext(ctx, convertPlaceholders(query), convertTimeArgs(args)...)
	if err != nil {
		return nil, err
	}
	return wrapSQLiteRows(rows), nil
}

func (t *sqliteTx) QueryRow(ctx context.Context, query string, args ...any) *Row {
	row := t.tx.QueryRowContext(ctx, convertPlaceholders(query), convertTimeArgs(args)...)
	return NewRow(func(dest ...any) error {
		scan := wrapSQLiteScan(row.Scan)
		err := scan(dest...)
		if err != nil && err.Error() == "sql: no rows in result set" {
			return ErrNoRows
		}
		return err
	})
}

func (t *sqliteTx) Commit(ctx context.Context) error   { return t.tx.Commit() }
func (t *sqliteTx) Rollback(ctx context.Context) error  { return t.tx.Rollback() }

// sqliteImmediateTx uses a raw conn with BEGIN IMMEDIATE for proper
// write-lock queuing under concurrent writes.
type sqliteImmediateTx struct {
	conn *sql.Conn
}

func (t *sqliteImmediateTx) Exec(ctx context.Context, query string, args ...any) (int64, error) {
	result, err := t.conn.ExecContext(ctx, convertPlaceholders(query), convertTimeArgs(args)...)
	if err != nil {
		return 0, err
	}
	n, _ := result.RowsAffected()
	return n, nil
}

func (t *sqliteImmediateTx) Query(ctx context.Context, query string, args ...any) (*Rows, error) {
	rows, err := t.conn.QueryContext(ctx, convertPlaceholders(query), convertTimeArgs(args)...)
	if err != nil {
		return nil, err
	}
	return wrapSQLiteRows(rows), nil
}

func (t *sqliteImmediateTx) QueryRow(ctx context.Context, query string, args ...any) *Row {
	row := t.conn.QueryRowContext(ctx, convertPlaceholders(query), convertTimeArgs(args)...)
	return NewRow(func(dest ...any) error {
		scan := wrapSQLiteScan(row.Scan)
		err := scan(dest...)
		if err != nil && err.Error() == "sql: no rows in result set" {
			return ErrNoRows
		}
		return err
	})
}

func (t *sqliteImmediateTx) Commit(ctx context.Context) error {
	_, err := t.conn.ExecContext(ctx, "COMMIT")
	t.conn.Close()
	return err
}

func (t *sqliteImmediateTx) Rollback(ctx context.Context) error {
	_, err := t.conn.ExecContext(ctx, "ROLLBACK")
	t.conn.Close()
	return err
}

// wrapSQLiteRows wraps sql.Rows with time-parsing scan.
func wrapSQLiteRows(rows *sql.Rows) *Rows {
	return NewRows(
		rows.Columns,
		rows.Next,
		wrapSQLiteScan(rows.Scan),
		rows.Close,
		rows.Err,
	)
}
