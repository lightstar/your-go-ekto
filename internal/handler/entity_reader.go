package handler

import (
	"mime/multipart"

	"github.com/lightstar/your-go-ekto/internal/service"
)

// entityReader реализует интерфейс service.EntityReader для чтения данных из multipart-запроса.
// Для этого необходимо обернуть *multipart.Reader.
type entityReader struct {
	r *multipart.Reader
}

// NextPart возвращает следующую часть multipart-запроса, преобразуя его к интерфейсу
// service.EntityPartReader, требуемому сервисом.
func (er *entityReader) NextPart() (service.EntityPartReader, error) {
	return er.r.NextPart()
}
