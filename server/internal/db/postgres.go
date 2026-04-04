package db

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresDatasource struct {
	pool *pgxpool.Pool
}

func NewPostgresDatasource(ctx context.Context, connString string) (*PostgresDatasource, error) {
	cfg, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("parse pg config: %w", err)
	}

	cfg.MinConns = 2
	cfg.MaxConns = 20
	if v := os.Getenv("DB_MAX_CONNS"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n > 0 {
			cfg.MaxConns = int32(n)
		}
	}
	cfg.MaxConnIdleTime = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pg pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pg ping: %w", err)
	}

	return &PostgresDatasource{pool: pool}, nil
}

func (p *PostgresDatasource) DBType() string { return "postgres" }

func (p *PostgresDatasource) Exec(ctx context.Context, sql string, args ...any) (int64, error) {
	tag, err := p.pool.Exec(ctx, sql, args...)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (p *PostgresDatasource) Query(ctx context.Context, sql string, args ...any) (*Rows, error) {
	rows, err := p.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return wrapPgxRows(rows), nil
}

func (p *PostgresDatasource) QueryRow(ctx context.Context, sql string, args ...any) *Row {
	row := p.pool.QueryRow(ctx, sql, args...)
	return NewRow(func(dest ...any) error {
		err := row.Scan(dest...)
		if err != nil && err.Error() == "no rows in result set" {
			return ErrNoRows
		}
		return err
	})
}

func (p *PostgresDatasource) Begin(ctx context.Context) (Tx, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &pgTx{tx: tx}, nil
}

func (p *PostgresDatasource) Close() error {
	p.pool.Close()
	return nil
}

func (p *PostgresDatasource) Ping(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

func (p *PostgresDatasource) Pool() *pgxpool.Pool {
	return p.pool
}

// pgTx wraps pgx.Tx to implement the Tx interface.
type pgTx struct {
	tx pgx.Tx
}

func (t *pgTx) Exec(ctx context.Context, sql string, args ...any) (int64, error) {
	tag, err := t.tx.Exec(ctx, sql, args...)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (t *pgTx) Query(ctx context.Context, sql string, args ...any) (*Rows, error) {
	rows, err := t.tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return wrapPgxRows(rows), nil
}

func (t *pgTx) QueryRow(ctx context.Context, sql string, args ...any) *Row {
	row := t.tx.QueryRow(ctx, sql, args...)
	return NewRow(func(dest ...any) error {
		err := row.Scan(dest...)
		if err != nil && err.Error() == "no rows in result set" {
			return ErrNoRows
		}
		return err
	})
}

func (t *pgTx) Commit(ctx context.Context) error   { return t.tx.Commit(ctx) }
func (t *pgTx) Rollback(ctx context.Context) error  { return t.tx.Rollback(ctx) }

func wrapPgxRows(rows pgx.Rows) *Rows {
	return NewRows(
		func() ([]string, error) {
			descs := rows.FieldDescriptions()
			cols := make([]string, len(descs))
			for i, d := range descs {
				cols[i] = d.Name
			}
			return cols, nil
		},
		rows.Next,
		rows.Scan,
		func() error { rows.Close(); return nil },
		rows.Err,
	)
}
