package sse

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteConflict(t *testing.T) {
	w := httptest.NewRecorder()

	currentCard := map[string]any{
		"id":      "card-1",
		"title":   "Updated Title",
		"version": 3,
	}
	WriteConflict(w, currentCard)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}

	var resp ConflictResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error == "" {
		t.Fatal("expected non-empty error message")
	}

	// Verify current card is included
	currentMap, ok := resp.Current.(map[string]any)
	if !ok {
		t.Fatal("expected current to be a map")
	}
	if currentMap["id"] != "card-1" {
		t.Fatalf("expected card-1, got %v", currentMap["id"])
	}
	if currentMap["title"] != "Updated Title" {
		t.Fatalf("expected Updated Title, got %v", currentMap["title"])
	}
	if version, ok := currentMap["version"].(float64); !ok || version != 3 {
		t.Fatalf("expected version 3, got %v", currentMap["version"])
	}
}

func TestWriteConflict_NilCurrent(t *testing.T) {
	w := httptest.NewRecorder()
	WriteConflict(w, nil)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}

	var resp ConflictResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Current != nil {
		t.Fatalf("expected nil current, got %v", resp.Current)
	}
}
