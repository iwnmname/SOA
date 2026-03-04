package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

func LoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			requestID := uuid.New().String()

			w.Header().Set("X-Request-Id", requestID)

			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			var bodyForLog string
			if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete {
				bodyBytes, _ := io.ReadAll(r.Body)
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				bodyForLog = maskSensitiveFields(string(bodyBytes))
			}

			next.ServeHTTP(rw, r)

			duration := time.Since(start)

			var userID *string
			if id, ok := GetUserID(r.Context()); ok {
				s := id.String()
				userID = &s
			}

			attrs := []slog.Attr{
				slog.String("request_id", requestID),
				slog.String("method", r.Method),
				slog.String("endpoint", r.URL.Path),
				slog.Int("status_code", rw.statusCode),
				slog.Int64("duration_ms", duration.Milliseconds()),
				slog.String("timestamp", start.Format(time.RFC3339)),
			}

			if userID != nil {
				attrs = append(attrs, slog.String("user_id", *userID))
			} else {
				attrs = append(attrs, slog.Any("user_id", nil))
			}

			if bodyForLog != "" {
				attrs = append(attrs, slog.String("request_body", bodyForLog))
			}

			logger.LogAttrs(r.Context(), slog.LevelInfo, "api_request", attrs...)
		})
	}
}

func maskSensitiveFields(body string) string {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return body
	}
	for key := range data {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "password") || strings.Contains(lower, "secret") || strings.Contains(lower, "token") {
			data[key] = "***"
		}
	}
	masked, err := json.Marshal(data)
	if err != nil {
		return body
	}
	return string(masked)
}
