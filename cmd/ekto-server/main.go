package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/lightstar/your-go-ekto/internal/app"
	"github.com/lightstar/your-go-ekto/internal/handler"
	"github.com/lightstar/your-go-ekto/internal/server"
	"github.com/lightstar/your-go-ekto/internal/service"
	"github.com/lightstar/your-go-ekto/internal/storage"
)

func main() {
	cfg, err := app.ParseConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		flag.Usage()
		os.Exit(1)
	}

	logger := app.Logger(cfg)

	ektoStorage, err := storage.New(cfg.StoragePath)
	if err != nil {
		logger.Error("Failed to create storage", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("Storage inititalized", slog.String("path", cfg.StoragePath))

	ektoService, err := service.New(ektoStorage)
	if err != nil {
		logger.Error("Failed to create service", slog.String("error", err.Error()))
		os.Exit(1)
	}

	ektoHandler, err := handler.New(ektoService, logger, cfg.APIPrefix)
	if err != nil {
		logger.Error("Failed to create http handler", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("Handler inititalized", slog.String("api_prefix", cfg.APIPrefix))

	ektoServer, err := server.New(cfg.ServerAddress, logger,
		slog.NewLogLogger(logger.Handler(), cfg.LogLevel))
	if err != nil {
		logger.Error("Failed to create server", slog.String("error", err.Error()))
		os.Exit(1)
	}

	app.SetupMiddlewares(ektoServer)
	app.SetupRoutes(ektoServer, ektoHandler)

	if err := app.RunServer(ektoServer); err != nil {
		logger.Error("Server run finished with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
