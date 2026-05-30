package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"time"
)

// errBodyCap is the maximum number of body bytes captured for error logging.
// Enough to surface a JSON {"error": "..."} response without bloating logs.
const errBodyCap = 512

type statusRecorder struct {
	http.ResponseWriter
	status  int
	errBody bytes.Buffer
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	// Capture response body bytes for error responses so 500s can be debugged
	// from access logs without needing to reproduce the request.
	if r.status >= 400 && r.errBody.Len() < errBodyCap {
		remaining := errBodyCap - r.errBody.Len()
		if remaining > len(b) {
			remaining = len(b)
		}
		r.errBody.Write(b[:remaining])
	}
	return r.ResponseWriter.Write(b)
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap exposes the wrapped ResponseWriter so http.NewResponseController can
// reach the underlying connection (e.g. for SetWriteDeadline on SSE streams).
func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func Logger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: 200}
			next.ServeHTTP(rec, r)
			attrs := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", GetRequestID(r.Context()),
				"remote_ip", r.RemoteAddr,
			}
			if rec.status >= 500 {
				attrs = append(attrs, "error_body", rec.errBody.String())
				logger.Error("request", attrs...)
			} else if rec.status >= 400 {
				attrs = append(attrs, "error_body", rec.errBody.String())
				logger.Warn("request", attrs...)
			} else {
				logger.Info("request", attrs...)
			}
		})
	}
}
