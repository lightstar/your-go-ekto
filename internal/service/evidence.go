package service

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/lightstar/your-go-ekto/internal/storage"

	"github.com/google/uuid"
)

// GetEvidence возвращает изображение улики по имени.
//
// Возвращается время модификации и поток с телом файла, что может в дальнейшем
// использоваться для вызова http.ServeContent.
func (s *Service) GetEvidence(
	ctx context.Context, name string,
) (time.Time, io.ReadSeekCloser, error) {
	if !s.validEvidenceName(name) {
		return time.Time{}, nil, ErrInvalidEvidenceName
	}

	modTime, body, err := s.storage.GetEvidence(ctx, name)
	if err != nil {
		if errors.Is(err, storage.ErrInvalidEvidenceName) {
			return time.Time{}, nil, ErrInvalidEvidenceName
		}

		if errors.Is(err, storage.ErrEvidenceNotExists) ||
			errors.Is(err, storage.ErrNotARegularFile) {
			return time.Time{}, nil, ErrEvidenceNotExists
		}

		return time.Time{}, nil, s.getFromStorageError(err)
	}

	return modTime, body, nil
}

// validEvidenceName проверяет, что имя запрашиваемой улики валидно, а именно:
// 1) Его расширение в списке разрешенных
// 2) Его имя представляет собой корректный ненулевой UUIDv7 формата RFC4122.
func (s *Service) validEvidenceName(name string) bool {
	baseName, ext, ok := strings.Cut(name, ".")
	if !ok {
		return false
	}

	id, err := uuid.Parse(baseName)
	if err != nil || id == uuid.Nil || id.String() != baseName ||
		id.Version() != uuid.Version(7) || id.Variant() != uuid.RFC4122 {
		return false
	}

	_, ok = evidenceExt2MIME["."+ext]
	return ok
}
