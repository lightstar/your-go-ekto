package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/lightstar/your-go-ekto/internal/httpx"
	"github.com/lightstar/your-go-ekto/internal/service"
)

// handleCreateEntityError обрабатывает ошибку, которая возникла при создании сущности.
func (h *Handler) handleCreateEntityError(w http.ResponseWriter, r *http.Request, err error) {
	var msg string
	var status int

	// Ошибка хранилища - приоритетная, ее наличие означает ошибку сервера, т.е. мы должны вернуть
	// 500 Internal Server Error.
	// Проверить ее нужно сразу, т.к. в теории ошибки запроса также могут присутствовать в цепочке
	// ошибок, но здесь они должны быть проигнорированы.
	if errors.Is(err, service.ErrStorage) {
		h.logger.Error(r, "create entity storage error", slog.String("error", err.Error()))
		h.error(w, r, "internal server error", http.StatusInternalServerError)
		return
	}

	if errors.Is(err, context.Canceled) {
		h.logger.Debug(r, "context canceled", slog.String("error", err.Error()))
		return
	}

	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		msg, status = "request entity too large", http.StatusRequestEntityTooLarge
	} else if errors.Is(err, http.ErrNotMultipart) || errors.Is(err, http.ErrMissingBoundary) {
		msg, status = "bad multipart form", http.StatusBadRequest
	} else if parseErr, ok := errors.AsType[*service.InvalidEntityError](err); ok {
		msg, status = parseErr.Message, http.StatusBadRequest
	} else {
		h.logger.Error(r, "create entity error", slog.String("error", err.Error()))
		h.error(w, r, "internal server error", http.StatusInternalServerError)
		return
	}

	h.logger.Debug(r, msg, slog.String("error", err.Error()))
	h.error(w, r, msg, status)
}

// handleGetEntityError обрабатывает ошибку, которая возникла при получении сущности.
func (h *Handler) handleGetEntityError(w http.ResponseWriter, r *http.Request, err error) {
	var msg string
	var status int

	if errors.Is(err, context.Canceled) {
		h.logger.Debug(r, "context canceled", slog.String("error", err.Error()))
		return
	}

	if errors.Is(err, service.ErrEntityNotExists) {
		msg, status = "entity not exists", http.StatusNotFound
	} else {
		h.logger.Error(r, "get entity error", slog.String("error", err.Error()))
		h.error(w, r, "internal server error", http.StatusInternalServerError)
		return
	}

	h.logger.Debug(r, msg, slog.String("error", err.Error()))
	h.error(w, r, msg, status)
}

// handleGetEvidenceError обрабатывает ошибку, которая возникла при получении файла с уликой.
func (h *Handler) handleGetEvidenceError(
	w http.ResponseWriter, r *http.Request, fileName string, err error,
) {
	var msg string
	var status int

	if errors.Is(err, context.Canceled) {
		h.logger.Debug(r, "context canceled", slog.String("error", err.Error()))
		return
	}

	if errors.Is(err, service.ErrInvalidEvidenceName) {
		msg, status = "invalid evidence name", http.StatusBadRequest
	} else if errors.Is(err, service.ErrEvidenceNotExists) {
		msg, status = "evidence not found", http.StatusNotFound
	} else {
		h.logger.Error(r, "get evidence error", slog.String("error", err.Error()),
			slog.String("filename", fileName))
		h.error(w, r, "internal server error", http.StatusInternalServerError)
		return
	}

	h.logger.Debug(r, msg, slog.String("filename", fileName), slog.String("error", err.Error()))
	h.error(w, r, msg, status)
}

// error - вспомогательный метод для отправки JSON-ответа клиенту с простым сообщением об ошибке.
func (h *Handler) error(w http.ResponseWriter, r *http.Request, message string, code int) {
	if err := httpx.WriteJSONError(w, r, message, code); err != nil {
		h.handleWriteError(w, r, err)
	}
}

// handleWriteError обрабатывает ошибку, которая возникла при записи ответа клиенту.
func (h *Handler) handleWriteError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, httpx.ErrMarshalResponse) {
		h.logger.Error(r, "marshal http response", slog.String("error", err.Error()))
		// Откат к http.Error, возвращающей ответ в формате text/plain. Это может произойти
		// только в случае ошибки json.Marshal, которой в общем случае вообще не должно
		// быть, разве что мы сами где-то сильно напортачили со структурами данных.
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	} else if errors.Is(err, httpx.ErrWriteResponse) {
		h.logger.Error(r, "write http response", slog.String("error", err.Error()))
	}
}
