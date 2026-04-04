package db

import (
	"context"
	"database/sql"
	"fmt"
)

// Datasource abstracts database operations for both Postgres and SQLite.
type Datasource interface {
	Exec(ctx context.Context, sql string, args ...any) (int64, error)
	Query(ctx context.Context, sql string, args ...any) (*Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) *Row
	Begin(ctx context.Context) (Tx, error)
	Close() error
	Ping(ctx context.Context) error
	DBType() string // "postgres" or "sqlite"
}

// Tx represents a database transaction.
type Tx interface {
	Exec(ctx context.Context, sql string, args ...any) (int64, error)
	Query(ctx context.Context, sql string, args ...any) (*Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) *Row
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// Row wraps a single row result.
type Row struct {
	scanFunc func(dest ...any) error
}

func NewRow(f func(dest ...any) error) *Row {
	return &Row{scanFunc: f}
}

func (r *Row) Scan(dest ...any) error {
	return r.scanFunc(dest...)
}

// Rows wraps a multi-row result set.
type Rows struct {
	columns  func() ([]string, error)
	next     func() bool
	scanFunc func(dest ...any) error
	closeFunc func() error
	errFunc  func() error
}

func NewRows(columns func() ([]string, error), next func() bool, scan func(dest ...any) error, close func() error, err func() error) *Rows {
	return &Rows{columns: columns, next: next, scanFunc: scan, closeFunc: close, errFunc: err}
}

func (r *Rows) Columns() ([]string, error) { return r.columns() }
func (r *Rows) Next() bool                 { return r.next() }
func (r *Rows) Scan(dest ...any) error      { return r.scanFunc(dest...) }
func (r *Rows) Close() error                { return r.closeFunc() }
func (r *Rows) Err() error                  { return r.errFunc() }

// WrapSQLRows wraps a *sql.Rows into our Rows type.
func WrapSQLRows(rows *sql.Rows) *Rows {
	return NewRows(rows.Columns, rows.Next, rows.Scan, rows.Close, rows.Err)
}

// ErrNoRows is returned when a query expects a single row but finds none.
var ErrNoRows = fmt.Errorf("no rows in result set")
