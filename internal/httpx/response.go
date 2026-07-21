package httpx

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// ErrorResponse представляет собой структуру ответа об ошибке.
type ErrorResponse struct {
	Error string `json:"error"`
}

// WriteJSONResponse записывает http-ответ клиенту в формате JSON. Возвращает одну из ошибок -
// ErrMarshalResponse (при ошибке маршалинга в JSON), либо ErrWriteResponse (при ошибки записи
// ответа).
func WriteJSONResponse(w http.ResponseWriter, r *http.Request, response any, status int) error {
	if r.Context().Err() != nil {
		return nil
	}

	data, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrMarshalResponse, err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("%w: %w", ErrWriteResponse, err)
	}

	return nil
}

// WriteJSONError - обертка над WriteJSONResponse, отправляющая клиенту простой JSON с одним
// сообщением в поле "error".
func WriteJSONError(w http.ResponseWriter, r *http.Request, message string, status int) error {
	return WriteJSONResponse(w, r, ErrorResponse{Error: message}, status)
}
