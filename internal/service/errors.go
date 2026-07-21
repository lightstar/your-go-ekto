package service

import (
	"errors"
)

var (
	ErrStorageRequired = errors.New("storage required")

	ErrInvalidEvidenceName = errors.New("invalid evidence name")
	ErrEvidenceNotExists   = errors.New("evidence not exists")
	ErrEntityNotExists     = errors.New("entity not exists")

	ErrNoDossier        = errors.New("no dossier")
	ErrNoEvidences      = errors.New("no evidences")
	ErrDuplicateDossier = errors.New("duplicate dossier")
	ErrTooManyEvidences = errors.New("too many evidences")

	ErrMalformedEntity     = errors.New("malformed entity")
	ErrMalformedDossier    = errors.New("malformed dossier")
	ErrDossierExtraData    = errors.New("dossier extra JSON data")
	ErrEvidenceEmpty       = errors.New("evidence is empty")
	ErrDossierTooLarge     = errors.New("dossier too large")
	ErrEvidenceTooLarge    = errors.New("evidence too large")
	ErrReadEvidence        = errors.New("failed to read evidence")
	ErrReadEvidenceMIME    = errors.New("failed to read evidence MIME")
	ErrInvalidEvidenceType = errors.New("invalid evidence type")
	ErrStorage             = errors.New("storage error")
)

// UnknownFieldError представляет собой ошибку неизвестного поля формы с заданным именем.
type UnknownFieldError struct {
	Field string
}

func (e UnknownFieldError) Error() string {
	return "unknown field: " + e.Field
}

// unknownFieldError возвращает ошибку неизвестного поля формы с заданным именем.
func unknownFieldError(field string) error {
	return &UnknownFieldError{Field: field}
}

// DossierValidationError представляет собой ошибку валидации досье с заданным сообщением.
type DossierValidationError struct {
	Message string
}

func (e DossierValidationError) Error() string {
	return "invalid dossier: " + e.Message
}

// dossierValidationError возвращает ошибку валидации досье с заданным сообщением.
// Ошибка автоматически оборачивается в InvalidEntityError для классификации.
func dossierValidationError(message string) error {
	return invalidEntityError(&DossierValidationError{Message: message})
}

// InvalidEntityError представляет собой ошибку невалидной сущности с заданным сообщением.
// Сообщение как правило является текстом внутренней ошибки и может безопасно возвращаться клиенту.
// Данная ошибка оборачивает все конкретные ошибки валидации компонентов сущности и прочие проблемы
// при парсинге ее multipart-тела.
// Вызывающий код в итоге может проверять именно ее, и извлекать текст для клиента из Message,
// вместо того, чтобы проверять все конкретные варианты ошибки некорректного тела сущности и
// валидации ее компонентов (досье и улик).
type InvalidEntityError struct {
	Message string
	Cause   error
}

func (e *InvalidEntityError) Error() string {
	return e.Message
}

func (e *InvalidEntityError) Unwrap() error {
	return e.Cause
}

// invalidEntityError возвращает ошибку невалидной сущности с заданной исходной ошибкой.
// В качестве Message, который может вернуться клиенту, устанавливается текст этой исходной ошибки.
func invalidEntityError(inner error) *InvalidEntityError {
	return &InvalidEntityError{Message: inner.Error(), Cause: inner}
}
