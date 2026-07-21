package httpx

import (
	"errors"
	"fmt"
)

var (
	ErrMarshalResponse = errors.New("marshal HTTP response")
	ErrWriteResponse   = errors.New("write HTTP response")
)

// MarshalResponseError - ошибка при маршалинге http-ответа клиенту.
type MarshalResponseError struct {
	Cause error
}

func (e *MarshalResponseError) Error() string {
	return fmt.Sprintf("marshal HTTP response: %v", e.Cause)
}

// WriteResponseError - ошибка при записи http-ответа клиенту.
type WriteResponseError struct {
	Cause error
}

func (e *WriteResponseError) Error() string {
	return fmt.Sprintf("write HTTP response: %v", e.Cause)
}
