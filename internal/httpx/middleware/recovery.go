package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/lightstar/your-go-ekto/internal/httpx"
)

// Recovery - middleware для восстановления от паники.
func Recovery(next http.Handler, logger httpx.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logger.Error("panic",
					slog.Any("error", err),
					slog.String("stack", string(debug.Stack())),
				)

				err := httpx.WriteJSONError(w, r, "internal server error",
					http.StatusInternalServerError)
				if err != nil {
					if errors.Is(err, httpx.ErrMarshalResponse) {
						logger.Error("marshal http response", slog.String("error", err.Error()))
						http.Error(w, "Internal server error", http.StatusInternalServerError)
					} else if errors.Is(err, httpx.ErrWriteResponse) {
						logger.Error("write http response", slog.String("error", err.Error()))
					}
				}
			}
		}()

		next.ServeHTTP(w, r)
	})
}
