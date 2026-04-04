package db

import (
	"context"
	"fmt"
	"strings"
)

// NewDatasource creates a Datasource from a database URL.
// Supported schemes: postgres://, postgresql://, sqlite://
func NewDatasource(ctx context.Context, dbURL string) (Datasource, error) {
	switch {
	case strings.HasPrefix(dbURL, "postgres://"), strings.HasPrefix(dbURL, "postgresql://"):
		return NewPostgresDatasource(ctx, dbURL)
	case strings.HasPrefix(dbURL, "sqlite://"):
		return NewSQLiteDatasource(dbURL)
	default:
		return nil, fmt.Errorf("unsupported database URL scheme: %s", dbURL)
	}
}
