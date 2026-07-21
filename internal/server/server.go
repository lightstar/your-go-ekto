package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/lightstar/your-go-ekto/internal/httpx"
)

const (
	readHeaderTimeout = 2 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 120 * time.Second
	shutdownTimeout   = 60 * time.Second
	maxHeaderBytes    = 1 << 20
)

// Server представляет собой HTTP-сервер, который умеет присоединять к себе
// middleware и http-хендлеры.
type Server struct {
	srv    *http.Server
	mux    *http.ServeMux
	logger httpx.Logger
}

// New создает новый экземпляр сервера. Автоматически настраиваются таймауты из констант.
func New(addr string, logger httpx.Logger, logLogger *log.Logger) (*Server, error) {
	if addr == "" {
		return nil, ErrAddrRequired
	}

	if logger == nil {
		return nil, ErrLoggerRequired
	}

	mux := http.NewServeMux()

	return &Server{
		srv: &http.Server{
			Addr:    addr,
			Handler: mux,

			ReadHeaderTimeout: readHeaderTimeout,
			ReadTimeout:       readTimeout,
			WriteTimeout:      writeTimeout,
			IdleTimeout:       idleTimeout,
			MaxHeaderBytes:    maxHeaderBytes,
			ErrorLog:          logLogger,
		},
		mux:    mux,
		logger: logger,
	}, nil
}

// Mux возвращает мультиплексор сервера, требуемый для настройки маршрутов.
func (s *Server) Mux() *http.ServeMux {
	return s.mux
}

// AddMiddleware добавляет middleware к серверу. Автоматически этому middleware будет передан
// логгер, который задан серверу при его создании.
func (s *Server) AddMiddleware(middleware func(http.Handler, httpx.Logger) http.Handler) {
	s.srv.Handler = middleware(s.srv.Handler, s.logger)
}

// SetupHealthRoute настраивает маршрут для проверки состояния сервера. Он может быть полезен
// для проверки состояния docker'ом.
func (s *Server) SetupHealthRoute(path string) {
	s.mux.HandleFunc("GET "+path, func(w http.ResponseWriter, r *http.Request) {
		response := map[string]string{"status": "ok"}
		if err := httpx.WriteJSONResponse(w, r, response, http.StatusOK); err != nil {
			s.logger.Error("write health response", slog.String("error", err.Error()))
		}
	})
}

// Run запускает сервер. При отмене контекста сервер пытается мягко (gracefully) завершиться.
func (s *Server) Run(ctx context.Context) error {
	s.logger.Info("Starting server", slog.String("address", s.srv.Addr))

	listenAndServeCh := make(chan error, 1)

	go func() {
		listenAndServeCh <- s.srv.ListenAndServe()
	}()

	select {
	case err := <-listenAndServeCh:
		return fmt.Errorf("listen and serve: %w", err)
	case <-ctx.Done():
	}

	s.logger.Info("Received termination signal, trying to gracefully shutdown server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	var shutdownErr, closeErr error

	if shutdownErr = s.srv.Shutdown(shutdownCtx); shutdownErr != nil {
		shutdownErr = fmt.Errorf("shutdown: %w", shutdownErr)

		if closeErr = s.srv.Close(); closeErr != nil {
			closeErr = fmt.Errorf("close: %w", closeErr)
		}
	}

	listenAndServeErr := <-listenAndServeCh
	if errors.Is(listenAndServeErr, http.ErrServerClosed) {
		listenAndServeErr = nil
	}

	if listenAndServeErr != nil {
		listenAndServeErr = fmt.Errorf("listen and serve: %w", listenAndServeErr)
	}

	if err := errors.Join(shutdownErr, closeErr, listenAndServeErr); err != nil {
		return err
	}

	s.logger.Info("Graceful shutdown complete")

	return nil
}
