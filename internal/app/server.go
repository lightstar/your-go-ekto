package app

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/lightstar/your-go-ekto/internal/handler"
	"github.com/lightstar/your-go-ekto/internal/httpx/middleware"
	"github.com/lightstar/your-go-ekto/internal/server"
)

// SetupRoutes настраивает маршруты для сервера, связывая их с обработчиком.
// Также настраивает и маршрут для /health, который нужен docker'у для проверки работоспособности
// сервиса.
func SetupRoutes(srv *server.Server, h *handler.Handler) {
	mux := srv.Mux()
	apiPrefix := h.APIPrefix()

	mux.HandleFunc("POST "+apiPrefix+"entities", h.CreateEntity)
	mux.HandleFunc("GET "+apiPrefix+"entities/{id}", h.GetEntity)
	mux.HandleFunc("GET "+apiPrefix+"evidence/{filename}", h.GetEvidence)

	srv.SetupHealthRoute("/health")
}

// SetupMiddlewares настраивает middleware для сервера, включая обработку паники и логирование.
func SetupMiddlewares(srv *server.Server) {
	srv.AddMiddleware(middleware.Recovery)
	srv.AddMiddleware(middleware.Logging)
}

// RunServer запускает сервер, ожидая сигналов прерывания и завершения. При получении этих сигналов
// сервер мягко (gracefully) останавливается, ожидая завершения всех запросов.
func RunServer(srv *server.Server) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// При отмене контекста останавливаем слушание сигналов. Тогда, если нажать Ctrl+C еще раз,
	// сервер будет остановлен уже грубо.
	// Это соответствует ожидаемому поведению в большинстве случаев.
	go func() {
		<-ctx.Done()
		stop()
	}()

	return srv.Run(ctx)
}
