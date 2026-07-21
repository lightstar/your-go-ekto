package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/lightstar/your-go-ekto/internal/httpx"
)

const statusClientClosedRequest = 499

// Logging - middleware для логирования HTTP-запросов.
func Logging(next http.Handler, logger httpx.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w}

		next.ServeHTTP(rw, r)

		durationMs := time.Since(start).Milliseconds()
		status := rw.StatusCode()

		if err := r.Context().Err(); err != nil && errors.Is(err, context.Canceled) {
			status = statusClientClosedRequest
		}

		logger.Info("Request handled",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", status),
			slog.Int64("duration_ms", durationMs),
			slog.String("user_agent", r.UserAgent()),
			slog.String("remote_addr", httpx.RemoteAddr(r)),
		)
	})
}

// responseWriter оборачивает http.ResponseWriter для логирования статуса ответа.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.statusCode == 0 {
		rw.statusCode = code
	}
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	return rw.ResponseWriter.Write(b)
}

func (rw *responseWriter) StatusCode() int {
	if rw.statusCode == 0 {
		return http.StatusOK // Если ничего не писали, по умолчанию это 200-ый статус
	}
	return rw.statusCode
}
