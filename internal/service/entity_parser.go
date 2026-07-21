package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/lightstar/your-go-ekto/internal/storage"

	"github.com/google/uuid"
)

const (
	maxEvidences    = 10
	maxEvidenceSize = 3 << 20
	maxDossierSize  = 1 << 20
	dossierField    = "dossier"
	evidenceField   = "evidence"
)

var (
	evidenceExt2MIME = map[string]string{
		".jpeg": "image/jpeg",
		".jpg":  "image/jpeg",
		".png":  "image/png",
	}
	evidenceMIME2Ext = map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
	}
)

// entityParser занимается парсингом multipart-тела данных сущности (досье и улик).
// В качестве источника использует абстрактный интерфейс EntityReader вместо явного
// *multipart.Reader, чтобы не зависеть от него.
type entityParser struct {
	// От Storage нам тут нужен только 1 метод, так что сужаем интерфейс для меньшей связности.
	storage interface {
		SaveEvidence(src io.Reader, name string, maxSize int64) (int64, error)
	}
	dossier dossier

	totalEvidences    int
	savedEvidences    []string
	savedEvidenceSize int64
	failedEvidences   []FailedEvidence
}

// EntityReader представляет собой интерфейс для чтения multipart-тела сущности (досье и улик).
// Является абстракцией над *multipart.Reader, чтобы не зависеть от него и облегчить потенциальное
// тестирование.
type EntityReader interface {
	NextPart() (EntityPartReader, error)
}

// EntityPartReader представляет собой интерфейс для чтения отдельной части multipart-тела сущности.
// Является абстракцией над *multipart.Part, опять же, чтобы не зависеть от него.
type EntityPartReader interface {
	FormName() string
	FileName() string
	Read(p []byte) (n int, err error)
}

// parseEntity парсит multipart-тело сущности (досье и улик) и сохраняет его в хранилище.
// Любые ошибки чтения/парсинга считаются фатальными и приводят к немедленному завершению с ошибкой.
// Ошибка также возвращается в случае:
// 1) Улики не были найдены в теле.
// 2) Количество улик превышает максимум.
// 3) Досье не было найдено в теле, или было найдено в количестве больше 1.
// 4) Размер тела улики или досье превышает максимум.
// 5) Найдено поле с неизвестным названием.
// Все такие ошибки оборачиваются в InvalidEntityError для облегчения классификации.
func (p *entityParser) parseEntity(ctx context.Context, r EntityReader) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		part, err := r.NextPart()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("%w: %w", invalidEntityError(ErrMalformedEntity), err)
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		switch part.FormName() {
		case dossierField:
			if err := p.parseDossier(part); err != nil {
				return fmt.Errorf("parse dossier: %w", err)
			}
		case evidenceField:
			if p.totalEvidences >= maxEvidences {
				return invalidEntityError(ErrTooManyEvidences)
			}

			if err := p.parseEvidence(part.FileName(), part); err != nil {
				return fmt.Errorf("parse evidence: %w", err)
			}
		default:
			return invalidEntityError(unknownFieldError(part.FormName()))
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if p.dossier.Name == "" {
		return invalidEntityError(ErrNoDossier)
	}

	if p.totalEvidences == 0 {
		return invalidEntityError(ErrNoEvidences)
	}

	return nil
}

// parseDossier парсит тело досье и сохраняет его в хранилище.
// Выполняет следующие проверки в порядке приоритетности:
// 1) Досье должно быть только одно (т.е. пока не должно существовать).
// 2) Тело досье не должно превышать установленного лимита.
// 3) Тело досье должно содержать валидный JSON установленного формата и не содержать никакого
// мусора в конце.
// 4) Полученные данные досье должны быть валидными (выполняется dossier.validate()).
func (p *entityParser) parseDossier(r io.Reader) error {
	if p.dossier.Name != "" {
		return invalidEntityError(ErrDuplicateDossier)
	}

	// LimitedReader создаем с размером большим требуемого на 1, чтобы можно было однозначно
	// проверить, что размер тела был превышен.
	limitedPart := &io.LimitedReader{R: r, N: maxDossierSize + 1}
	decoder := json.NewDecoder(limitedPart)
	decoder.DisallowUnknownFields()

	// Ошибку парсинга сперва просто фиксируем, но не возвращаем, т.к. она могла в теории возникнуть
	// из-за того, что LimitedReader обрезал поле (limitedPart.N тогда будет равен 0), и тогда
	// нужно проигнорировать эту ошибку и вернуть ErrDossierTooLarge.
	var parseErr error
	if err := decoder.Decode(&p.dossier); err != nil {
		parseErr = fmt.Errorf("%w: %w", invalidEntityError(ErrMalformedDossier), err)
	} else if err := decoder.Decode(&struct{}{}); err == nil {
		parseErr = invalidEntityError(ErrDossierExtraData)
	} else if !errors.Is(err, io.EOF) {
		parseErr = fmt.Errorf("%w: %w", invalidEntityError(ErrDossierExtraData), err)
	}

	// Дочитываем остаток тела досье, чтобы наверняка поймать ErrDossierTooLarge.
	_, err := io.Copy(io.Discard, limitedPart)
	if err != nil {
		parseErr = errors.Join(parseErr,
			fmt.Errorf("%w: %w", invalidEntityError(ErrMalformedEntity), err))
	}

	if limitedPart.N == 0 {
		return invalidEntityError(ErrDossierTooLarge)
	}

	if parseErr != nil {
		return parseErr
	}

	p.dossier.normalize()
	if err := p.dossier.validate(); err != nil {
		return err
	}

	return nil
}

// parseEvidence парсит тело улик и сохраняет его в хранилище.
// Возвращает ошибку при проблемах записи в хранилище или других неожиданных проблемах с IO/памятью.
// При возникновении же ошибки в самих данных улик (формат/размер/...), они просто сохраняются в
// failedEvidences, функция ошибки не возвращает.
func (p *entityParser) parseEvidence(fileName string, r io.Reader) error {
	name, size, err := p.saveEvidence(r)
	if err != nil {
		reason, ok := p.evidenceErrToReason(err)
		if !ok {
			return err
		}

		p.failedEvidences = append(p.failedEvidences, FailedEvidence{
			OriginalName: fileName,
			FailReason:   reason,
		})
	} else {
		p.savedEvidences = append(p.savedEvidences, name)
		p.savedEvidenceSize += size
	}

	p.totalEvidences++

	return nil
}

// saveEvidence потоково сохраняет файл с уликой в хранилище.
// В качестве имени при этом используется UUID v7 с расширением, которое определяется п
//
//	содержимому тела улики.
//
// Возвращает ошибку при невалидном типе, превышении размера, невозможностью создать UUID,
// а также проблемах со чтением или записью.
func (p *entityParser) saveEvidence(r io.Reader) (string, int64, error) {
	ext, data, err := p.validateEvidenceBody(r)
	if err != nil {
		return "", 0, err
	}

	// r не имеет метода Seek, поэтому необходимо вернуть в поток данные, прочитанные для
	// определения типа. io.MultiReader именно это и делает - объединяет уже прочитанные данные
	// с потоком, из которого эти данные были прочитаны.
	r = io.MultiReader(bytes.NewReader(data), r)

	nameUUID, err := uuid.NewV7()
	if err != nil {
		return "", 0, fmt.Errorf("generate uuid: %w", err)
	}

	name := nameUUID.String() + ext

	// Проверку на превышение размера будет делать хранилище при записи, т.к. заранее размер
	// неизвестен. При превышении хранилище вернет storage.ErrTooLarge, что будет обернуто
	// в ErrEvidenceTooLarge.
	size, err := p.storage.SaveEvidence(r, name, maxEvidenceSize)
	if err != nil {
		return "", 0, p.saveEvidenceError(err)
	}

	return name, size, nil
}

// saveEvidenceError оборачивает ошибку сохранения в хранилище, если это необходимо, чтобы клиенты
// сервиса не зависели от ошибок storage.
// В итоге storage.ErrStorageOp оборачивается в ErrStorage, storage.ErrTooLarge -
// в ErrEvidenceTooLarge, а storage.ErrRead - в ErrReadEvidence.
func (p *entityParser) saveEvidenceError(err error) error {
	errPrefix := "save to storage"

	switch {
	case errors.Is(err, storage.ErrStorageOp):
		return fmt.Errorf("%s: %w: %w", errPrefix, ErrStorage, err)
	case errors.Is(err, storage.ErrTooLarge):
		return fmt.Errorf("%s: %w: %w", errPrefix, invalidEntityError(ErrEvidenceTooLarge), err)
	case errors.Is(err, storage.ErrRead):
		return fmt.Errorf("%s: %w: %w", errPrefix, invalidEntityError(ErrReadEvidence), err)
	default:
		return fmt.Errorf("%s: %w", errPrefix, err)
	}
}

// validateEvidenceBody проверяет поток-источник данных улики на валидность, и
// при успехе возвращает расширение изображения, вычисленное по MIME-типу, а также
// прочитанные для определения типа данные.
// Проверяется следующее: поток не пуст, его можно читать (читаются только первые 512 байт),
// и что его вычисленное MIME в списке разрешенных.
func (p *entityParser) validateEvidenceBody(r io.Reader) (string, []byte, error) {
	data := make([]byte, 512)

	n, err := io.ReadFull(r, data)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", nil, fmt.Errorf("%w: %w", invalidEntityError(ErrReadEvidenceMIME), err)
	}

	if n == 0 {
		return "", nil, invalidEntityError(ErrEvidenceEmpty)
	}

	data = data[:n]
	contentType := http.DetectContentType(data)
	ext, ok := evidenceMIME2Ext[contentType]
	if !ok {
		return "", nil, invalidEntityError(ErrInvalidEvidenceType)
	}

	return ext, data, nil
}

// evidenceErrToReason преобразует ошибку в причину неудачи, которая возвращается клиенту.
// Если ошибка фатальная (ошибка хранилища, либо нераспознанная), то возвращается "", false, иначе -
// строка с причиной и true.
func (p *entityParser) evidenceErrToReason(err error) (FailReason, bool) {
	// Проверять на фатальные ошибки хранилища нужно в самом начале, т.к. она может быть
	// присоединена после ошибок самих данных улики (например, не удалось удалить временный файл).
	// Если такая ошибка была, то это фатальный сбой, и мы должны отменить и откатить всю операцию.
	if errors.Is(err, ErrStorage) {
		return "", false
	}

	switch {
	case errors.Is(err, ErrEvidenceEmpty):
		return FailReasonFileEmpty, true
	case errors.Is(err, ErrEvidenceTooLarge):
		return FailReasonFileTooLarge, true
	case errors.Is(err, ErrInvalidEvidenceType):
		return FailReasonInvalidMIME, true
	default:
		return "", false
	}
}
