package middleware

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLogger_LogsRequestFields(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest("POST", "/api/test", nil)
	ctx := context.WithValue(req.Context(), RequestIDKey, "test-req-id")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	output := buf.String()
	for _, field := range []string{"method=POST", "path=/api/test", "status=201", "duration_ms=", "request_id=test-req-id"} {
		if !bytes.Contains([]byte(output), []byte(field)) {
			t.Errorf("log output missing %q, got: %s", field, output)
		}
	}
}
