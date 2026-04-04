package sse

import (
	"encoding/json"
	"net/http"
)

// ConflictResponse is the body returned on a 409 Conflict.
type ConflictResponse struct {
	Error   string `json:"error"`
	Current any    `json:"current,omitempty"` // current card state for the client to merge
}

// WriteConflict writes a 409 response with the current state of the conflicting resource.
func WriteConflict(w http.ResponseWriter, current any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusConflict)
	_ = json.NewEncoder(w).Encode(ConflictResponse{
		Error:   "version conflict: card was modified by another user",
		Current: current,
	})
}
