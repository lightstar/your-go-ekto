package service

import (
	"context"
	"io"
	"time"

	"github.com/lightstar/your-go-ekto/internal/model"

	"github.com/google/uuid"
)

// Storage - интерфейс для хранилища сущностей и улик.
type Storage interface {
	GetEntity(ctx context.Context, dossierID uuid.UUID) (model.Entity, error)
	SaveEntity(ctx context.Context, entity model.Entity) error
	RemoveEntity(ctx context.Context, dossierID uuid.UUID) error
	SaveEvidence(ctx context.Context, src io.Reader, name string, maxSize int64) (int64, error)
	GetEvidence(ctx context.Context, name string) (time.Time, io.ReadSeekCloser, error)
	RemoveEvidence(ctx context.Context, name string) error
}

// Service - сервис для работы с сущностями и уликами.
type Service struct {
	storage Storage
}

// New создает новый экземпляр Service.
func New(storage Storage) (*Service, error) {
	if storage == nil {
		return nil, ErrStorageRequired
	}

	return &Service{
		storage: storage,
	}, nil
}
