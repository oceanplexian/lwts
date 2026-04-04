package db

import (
	"context"
	"testing"
)

func TestFactorySQLite(t *testing.T) {
	ds, err := NewDatasource(context.Background(), "sqlite://:memory:")
	if err != nil {
		t.Fatalf("create sqlite: %v", err)
	}
	defer ds.Close()
	if ds.DBType() != "sqlite" {
		t.Fatalf("expected sqlite, got %s", ds.DBType())
	}
}

func TestFactoryUnsupported(t *testing.T) {
	_, err := NewDatasource(context.Background(), "mysql://localhost/test")
	if err == nil {
		t.Fatal("expected error for unsupported scheme")
	}
}
