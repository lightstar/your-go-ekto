package app

import (
	"log/slog"
	"os"
)

// Logger создает и возвращает экземпляр логгера на основе конфигурации.
func Logger(cfg Config) *slog.Logger {
	logOptions := &slog.HandlerOptions{Level: cfg.LogLevel}
	return slog.New(slog.NewTextHandler(os.Stdout, logOptions))
}
