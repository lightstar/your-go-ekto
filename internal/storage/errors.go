package storage

import (
	"errors"
	"fmt"
)

var (
	ErrNoRootDir           = errors.New("no root dir")
	ErrEntityNotExists     = errors.New("entity not exists")
	ErrEvidenceNotExists   = errors.New("evidence not exists")
	ErrInvalidEvidenceName = errors.New("invalid evidence name")
	ErrNotARegularFile     = errors.New("not a regular file")
	ErrTooLarge            = errors.New("too large")
	ErrRead                = errors.New("read failed")
	ErrStorageOp           = errors.New("storage op failed")
	ErrSource              = errors.New("source error")
)

// storageOpError оборачивает ошибку в ErrStorageOp и добавляет описание операции.
// Используется для классификации ошибки (errors.Is(err, ErrStorageOp) будет равен true).
func storageOpError(op string, cause error) error {
	return fmt.Errorf("%w: %s: %w", ErrStorageOp, op, cause)
}

// sourceError оборачивает ошибку в ErrSource.
// Используется для классификации ошибки (errors.Is(err, ErrSource) будет равен true).
func sourceError(cause error) error {
	return fmt.Errorf("%w: %w", ErrSource, cause)
}
