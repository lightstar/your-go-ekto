package app

import (
	"flag"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"unicode"
)

const (
	defAPIPrefix     = "/api/v1/"
	defStoragePath   = "storage"
	defServerAddress = ":8080"
	defLogLevel      = "debug"
)

// Config представляет собой конфигурацию приложения.
type Config struct {
	APIPrefix     string
	StoragePath   string
	ServerAddress string
	LogLevel      slog.Level
}

// ParseConfig парсит аргументы командной строки и возвращает конфигурацию приложения.
// При отсутствии какого-либо флага используются значения по-умолчанию, а если значение невалидно -
// возвращается ошибка.
func ParseConfig() (Config, error) {
	var cfg Config

	flag.StringVar(&cfg.APIPrefix, "api", defAPIPrefix, "API prefix")
	flag.StringVar(&cfg.StoragePath, "storage", defStoragePath, "Storage path")
	flag.StringVar(&cfg.ServerAddress, "address", defServerAddress, "Server address")

	var logLevel string
	flag.StringVar(&logLevel, "log", defLogLevel, "Log level (debug/info/warn/error)")

	flag.Parse()

	if !apiPrefixValid(cfg.APIPrefix) {
		return Config{}, fmt.Errorf("invalid API prefix: %s", cfg.APIPrefix)
	}

	switch logLevel {
	case "debug":
		cfg.LogLevel = slog.LevelDebug
	case "info":
		cfg.LogLevel = slog.LevelInfo
	case "warn":
		cfg.LogLevel = slog.LevelWarn
	case "error":
		cfg.LogLevel = slog.LevelError
	default:
		return Config{}, fmt.Errorf("invalid log level: %s", logLevel)
	}

	return cfg, nil
}

// apiPrefixValid проверяет валидность префикса API.
func apiPrefixValid(prefix string) bool {
	if !strings.HasPrefix(prefix, "/") {
		return false
	}

	if prefix != "/" && !strings.HasSuffix(prefix, "/") {
		return false
	}

	if strings.ContainsAny(prefix, "{}?#%\\") {
		return false

	}

	for _, r := range prefix {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}

	canonical := path.Clean(prefix)
	if canonical != "/" {
		canonical += "/"
	}

	if canonical != prefix {
		return false
	}

	return true
}
