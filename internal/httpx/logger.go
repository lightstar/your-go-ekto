package httpx

import (
	"log/slog"
	"net/http"
)

// Logger представляет собой интерфейс для логирования.
// Используется в http-хендлерах и middleware.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

// RequestLogger - обертка логгера для упрощения логирования событий, происходящих во время
// обработки http-запросов.
type RequestLogger struct {
	logger Logger
}

// NewRequestLogger создает новый экземпляр RequestLogger.
func NewRequestLogger(logger Logger) *RequestLogger {
	return &RequestLogger{logger: logger}
}

// Error логирует ошибку, произошедшую во время обработки запроса. К сообщению будут автоматически
// добавлены важные атрибуты запроса.
func (rl *RequestLogger) Error(r *http.Request, message string, attrs ...any) {
	log(rl.logger.Error, r, message, attrs...)
}

// Info логирует информационное сообщение, связанное с обработкой запроса. К сообщению будут
// автоматически добавлены важные атрибуты запроса.
func (rl *RequestLogger) Info(r *http.Request, message string, attrs ...any) {
	log(rl.logger.Info, r, message, attrs...)
}

// Debug логирует отладочное сообщение, связанное с обработкой запроса. К сообщению будут
// автоматически добавлены важные атрибуты запроса.
func (rl *RequestLogger) Debug(r *http.Request, message string, attrs ...any) {
	log(rl.logger.Debug, r, message, attrs...)
}

// log вспомогательная функция для логирования, которая автоматически добавляет
// важные атрибуты запроса к сообщению.
func log(logFn func(string, ...any), r *http.Request, message string, attrs ...any) {
	logFn(message, append([]any{
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.String("remote_addr", RemoteAddr(r)),
	}, attrs...)...)
}
