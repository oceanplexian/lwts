package db

import (
	"context"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var migrationFileRe = regexp.MustCompile(`^(\d+)_.+\.up\.sql$`)

// MigrationEntry represents a single migration file.
type MigrationEntry struct {
	Version  int
	Name     string
	SQL      string
}

// Migrate runs all pending .up.sql migrations from the given filesystem.
// Migrations are applied in version order. Already-applied versions are skipped.
func Migrate(ctx context.Context, ds Datasource, migrations fs.FS) error {
	entries, err := parseMigrations(migrations)
	if err != nil {
		return fmt.Errorf("parse migrations: %w", err)
	}
	if len(entries) == 0 {
		return nil
	}

	// Acquire lock
	if err := acquireLock(ctx, ds); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer releaseLock(ctx, ds)

	// Ensure schema_migrations table exists
	createTable := `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INT NOT NULL PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT ''
	)`
	if ds.DBType() == "postgres" {
		createTable = `CREATE TABLE IF NOT EXISTS schema_migrations (
			version INT NOT NULL PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`
	}
	if _, err := ds.Exec(ctx, createTable); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	// Get applied versions
	applied, err := getAppliedVersions(ctx, ds)
	if err != nil {
		return fmt.Errorf("get applied versions: %w", err)
	}

	for _, entry := range entries {
		if applied[entry.Version] {
			continue
		}

		sqlText := entry.SQL
		if ds.DBType() == "sqlite" {
			sqlText = adaptForSQLite(sqlText)
		}

		tx, err := ds.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx for migration %d: %w", entry.Version, err)
		}

		// Execute migration SQL (may contain multiple statements)
		for _, stmt := range splitStatements(sqlText) {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := tx.Exec(ctx, stmt); err != nil {
				tx.Rollback(ctx)
				return fmt.Errorf("migration %d (%s): %w", entry.Version, entry.Name, err)
			}
		}

		// Record version
		now := "datetime('now')"
		insertSQL := `INSERT INTO schema_migrations (version, applied_at) VALUES ($1, datetime('now'))`
		if ds.DBType() == "postgres" {
			insertSQL = `INSERT INTO schema_migrations (version, applied_at) VALUES ($1, now())`
			now = "now()"
		}
		_ = now
		if _, err := tx.Exec(ctx, insertSQL, entry.Version); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("record migration %d: %w", entry.Version, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %d: %w", entry.Version, err)
		}
	}

	return nil
}

func parseMigrations(migrations fs.FS) ([]MigrationEntry, error) {
	var entries []MigrationEntry

	err := fs.WalkDir(migrations, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		name := d.Name()
		m := migrationFileRe.FindStringSubmatch(name)
		if m == nil {
			return nil
		}

		version, _ := strconv.Atoi(m[1])
		data, err := fs.ReadFile(migrations, path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		entries = append(entries, MigrationEntry{
			Version: version,
			Name:    name,
			SQL:     string(data),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Version < entries[j].Version
	})
	return entries, nil
}

func getAppliedVersions(ctx context.Context, ds Datasource) (map[int]bool, error) {
	rows, err := ds.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

func acquireLock(ctx context.Context, ds Datasource) error {
	if ds.DBType() == "postgres" {
		// Advisory lock with a fixed key
		_, err := ds.Exec(ctx, "SELECT pg_advisory_lock(42)")
		return err
	}
	// SQLite: single-writer by nature with busy_timeout, no extra lock needed
	return nil
}

func releaseLock(ctx context.Context, ds Datasource) {
	if ds.DBType() == "postgres" {
		ds.Exec(ctx, "SELECT pg_advisory_unlock(42)")
	}
}

// splitStatements splits SQL text on semicolons, handling basic cases.
func splitStatements(sql string) []string {
	var stmts []string
	var current strings.Builder
	inString := false
	inDollarQuote := false
	dollarTag := ""

	runes := []rune(sql)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]

		if inDollarQuote {
			current.WriteRune(ch)
			// Check for closing dollar quote
			if ch == '$' {
				rest := string(runes[i:])
				if strings.HasPrefix(rest, dollarTag) {
					current.WriteString(dollarTag[1:]) // already wrote the $
					i += len([]rune(dollarTag)) - 1
					inDollarQuote = false
				}
			}
			continue
		}

		if inString {
			current.WriteRune(ch)
			if ch == '\'' {
				// Check for escaped quote
				if i+1 < len(runes) && runes[i+1] == '\'' {
					current.WriteRune(runes[i+1])
					i++
				} else {
					inString = false
				}
			}
			continue
		}

		switch ch {
		case '\'':
			inString = true
			current.WriteRune(ch)
		case '$':
			// Check for dollar-quoted string
			rest := string(runes[i:])
			if idx := strings.Index(rest[1:], "$"); idx >= 0 {
				tag := rest[:idx+2]
				// Simple validation: tag should be $...$ with optional identifier
				if len(tag) >= 2 {
					dollarTag = tag
					inDollarQuote = true
					current.WriteString(tag)
					i += len([]rune(tag)) - 1
					continue
				}
			}
			current.WriteRune(ch)
		case ';':
			s := strings.TrimSpace(current.String())
			if s != "" {
				stmts = append(stmts, s)
			}
			current.Reset()
		case '-':
			// Line comment
			if i+1 < len(runes) && runes[i+1] == '-' {
				for i < len(runes) && runes[i] != '\n' {
					i++
				}
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}

	s := strings.TrimSpace(current.String())
	if s != "" {
		stmts = append(stmts, s)
	}
	return stmts
}

// adaptForSQLite transforms Postgres SQL to be SQLite-compatible.
func adaptForSQLite(sql string) string {
	r := sql

	// Remove gen_random_uuid() defaults
	r = regexp.MustCompile(`(?i)\bDEFAULT\s+gen_random_uuid\(\)`).ReplaceAllString(r, "")

	// UUID → TEXT
	r = regexp.MustCompile(`(?i)\bUUID\b`).ReplaceAllString(r, "TEXT")

	// TIMESTAMPTZ → TEXT
	r = regexp.MustCompile(`(?i)\bTIMESTAMPTZ\b`).ReplaceAllString(r, "TEXT")

	// JSONB → TEXT
	r = regexp.MustCompile(`(?i)\bJSONB\b`).ReplaceAllString(r, "TEXT")

	// BOOLEAN → INTEGER
	r = regexp.MustCompile(`(?i)\bBOOLEAN\b`).ReplaceAllString(r, "INTEGER")

	// DEFAULT now() → DEFAULT (datetime('now'))
	r = regexp.MustCompile(`(?i)\bDEFAULT\s+now\(\)`).ReplaceAllString(r, "DEFAULT (datetime('now'))")

	// DEFAULT true → DEFAULT 1, DEFAULT false → DEFAULT 0
	r = regexp.MustCompile(`(?i)\bDEFAULT\s+true\b`).ReplaceAllString(r, "DEFAULT 1")
	r = regexp.MustCompile(`(?i)\bDEFAULT\s+false\b`).ReplaceAllString(r, "DEFAULT 0")

	// Remove WHERE clauses on CREATE INDEX (partial indexes not well supported)
	r = regexp.MustCompile(`(?i)(CREATE\s+INDEX\s+\S+\s+ON\s+\S+\s*\([^)]+\))\s+WHERE\s+[^;]+`).ReplaceAllString(r, "$1")

	// DATE → TEXT
	r = regexp.MustCompile(`(?i)\bDATE\b`).ReplaceAllString(r, "TEXT")

	return r
}
