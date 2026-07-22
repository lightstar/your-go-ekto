package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/lightstar/your-go-ekto/internal/model"
	"github.com/lightstar/your-go-ekto/internal/storage"

	"github.com/google/uuid"
)

// CreateStatus представляет собой статус создания сущности.
// Может принимать только одно из предустановленных значений.
type CreateStatus string

const (
	StatusSuccess        CreateStatus = "success"
	StatusPartialSuccess CreateStatus = "partial_success"
	StatusFailed         CreateStatus = "failed"
)

// FailReason представляет собой причину неудачного сохранения улики.
// Может принимать только одно из предустановленных значений.
type FailReason string

const (
	FailReasonFileEmpty    FailReason = "file_empty"
	FailReasonFileTooLarge FailReason = "file_too_large"
	FailReasonInvalidMIME  FailReason = "invalid_mime_type"
)

// CreateEntityResult представляет собой данные ответа на запрос создания сущности.
type CreateEntityResult struct {
	DossierID       string
	Status          CreateStatus
	SavedEvidence   []string
	FailedEvidence  []FailedEvidence
	ParanormalIndex float64
}

// FailedEvidence представляет собой данные об улике, которую не удалось сохранить.
type FailedEvidence struct {
	OriginalName string
	FailReason   FailReason
}

// GetEntity возвращает сущность по ее ID.
func (s *Service) GetEntity(ctx context.Context, id uuid.UUID) (model.Entity, error) {
	entity, err := s.storage.GetEntity(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrEntityNotExists) {
			return model.Entity{}, ErrEntityNotExists
		}
		return model.Entity{}, s.getFromStorageError(err)
	}

	return entity, nil
}

// CreateEntity создает новую сущность на основе данных из EntityReader.
// Для парсинга использует отдельную структуру entityParser.
// Выполняет автоматический откат при ошибке.
func (s *Service) CreateEntity(
	ctx context.Context, r EntityReader,
) (resp CreateEntityResult, err error) {
	p := entityParser{storage: s.storage}
	var dossierID uuid.UUID

	defer func() {
		// Нужно откатить (удалить уже сохраненные улики), как при ошибке, так и при панике.
		// Панику при этом нужно пробросить дальше.
		panicErr := recover()

		if err == nil && panicErr == nil {
			return
		}

		const rollbackTimeout = 5 * time.Second
		rollBackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackTimeout)
		defer cancel()

		orphanedEntityRemained := false
		if dossierID != uuid.Nil {
			if removeErr := s.removeOrphanedEntity(rollBackCtx, dossierID); removeErr != nil {
				err = errors.Join(err, removeErr)
				orphanedEntityRemained = true
			}
		}

		// Мы не должны удалять улики, если сущность уже была сохранена, и ее не удалось удалить.
		if !orphanedEntityRemained {
			removeErrs := s.removeOrphanedEvidences(rollBackCtx, p.savedEvidences)
			if removeErrs != nil {
				err = errors.Join(append([]error{err}, removeErrs...)...)
			}
		}

		if panicErr != nil {
			panic(panicErr)
		}
	}()

	if err := p.parseEntity(ctx, r); err != nil {
		return CreateEntityResult{}, fmt.Errorf("parse entity: %w", err)
	}

	var response CreateEntityResult

	if len(p.savedEvidences) > 0 {
		entity, err := s.saveEntity(ctx, p.dossier, p.savedEvidences)
		if err != nil {
			return CreateEntityResult{}, fmt.Errorf("save entity: %w", err)
		}

		dossierID = entity.DossierID
		response.DossierID = dossierID.String()
		response.SavedEvidence = entity.Evidences
		response.ParanormalIndex = s.paranormalIndex(p.savedEvidenceSize, p.totalEvidences)
	}

	response.FailedEvidence = p.failedEvidences
	response.Status = s.createEntityStatus(response)

	return response, nil
}

// saveEntity сохраняет сущность в хранилище (досье и список сохраненных улик).
// В качестве id автоматически генерируется UUID v7.
func (s *Service) saveEntity(
	ctx context.Context, dossier dossier, evidences []string,
) (e model.Entity, err error) {
	id, err := uuid.NewV7()
	if err != nil {
		return model.Entity{}, fmt.Errorf("generate uuid: %w", err)
	}

	entity := model.Entity{
		DossierID:       id,
		Name:            dossier.Name,
		Description:     dossier.Description,
		ThreatLevel:     dossier.ThreatLevel,
		Vulnerabilities: dossier.Vulnerabilities,
		Evidences:       evidences,
	}

	if err := s.storage.SaveEntity(ctx, entity); err != nil {
		return model.Entity{}, s.saveToStorageError(err)
	}

	return entity, nil
}

// getFromStorageError оборачивает ошибку получения из хранилища, если это необходимо, чтобы клиенты
// сервиса не зависели от ошибок storage.
func (s *Service) getFromStorageError(err error) error {
	errPrefix := "get from storage"

	if errors.Is(err, storage.ErrStorageOp) {
		return fmt.Errorf("%s: %w: %w", errPrefix, ErrStorage, err)
	}
	return fmt.Errorf("%s: %w", errPrefix, err)
}

// saveToStorageError оборачивает ошибку сохранения в хранилище, если это необходимо, чтобы клиенты
// сервиса не зависели от ошибок storage.
func (s *Service) saveToStorageError(err error) error {
	errPrefix := "save to storage"

	if errors.Is(err, storage.ErrStorageOp) {
		return fmt.Errorf("%s: %w: %w", errPrefix, ErrStorage, err)
	}
	return fmt.Errorf("%s: %w", errPrefix, err)
}

// createEntityStatus определяет статус создания сущности на основе результатов сохранения улик.
func (s *Service) createEntityStatus(resp CreateEntityResult) CreateStatus {
	switch {
	case len(resp.FailedEvidence) == 0:
		return StatusSuccess
	case len(resp.SavedEvidence) == 0:
		return StatusFailed
	default:
		return StatusPartialSuccess
	}
}

// paranormalIndex рассчитывает паранормальный индекс на основе сохраненного размера улик и общего
// количества улик (даже тех, которые сохранены с ошибкой).
func (s *Service) paranormalIndex(savedEvidenceSize int64, totalEvidenceCount int) float64 {
	return (float64(savedEvidenceSize) / (1 << 20)) * math.Sqrt(float64(totalEvidenceCount))
}

// removeOrphanedEntity удаляет уже сохраненную сущность, если вдруг понадобится откат операции
// сохранения.
func (s *Service) removeOrphanedEntity(ctx context.Context, dossierID uuid.UUID) error {
	if err := s.storage.RemoveEntity(ctx, dossierID); err != nil {
		return fmt.Errorf("remove orphaned entity: %w: %w", ErrStorage, err)
	}

	return nil
}

// removeOrphanedEvidences удаляет уже сохраненные улики, если вдруг понадобится откат
// операции сохранения сущности.
func (s *Service) removeOrphanedEvidences(ctx context.Context, evidences []string) []error {
	var errs []error

	for _, evidence := range evidences {
		if err := s.storage.RemoveEvidence(ctx, evidence); err != nil {
			err = fmt.Errorf("remove orphaned evidence %q: %w: %w", evidence, ErrStorage, err)
			errs = append(errs, err)
		}
	}

	return errs
}
