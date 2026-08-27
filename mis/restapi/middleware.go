package restapi

import (
	"bytes"
	"io"
	"net/http"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture status code and response body.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.body.Write(b) // capture for debug logging
	return rw.ResponseWriter.Write(b)
}

// loggingMiddleware wraps a handler with info and debug level logging.
func (m *MisRestAPI) loggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Read and restore request body for debug logging.
		var reqBody []byte
		if r.Body != nil {
			var err error
			reqBody, err = io.ReadAll(r.Body)
			if err != nil {
				m.logger.Error("failed to read request body", "err", err.Error())
			}
			r.Body = io.NopCloser(bytes.NewBuffer(reqBody))
		}

		rw := newResponseWriter(w)
		next(rw, r)

		duration := time.Since(start)

		// Info: method, path, status, duration.
		m.logger.Info(
			"request handled",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.statusCode,
			"duration_ms", duration.Milliseconds(),
		)

		// Debug: additionally log request and response bodies.
		m.logger.Debug(
			"request/response detail",
			"method", r.Method,
			"path", r.URL.Path,
			"request_body", string(reqBody),
			"response_body", rw.body.String(),
		)
	}
}
