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

// entityMultipartProcessor занимается чтением и обработкой multipart-тела данных сущности
// (досье и улик).
//
// В качестве источника использует абстрактный интерфейс EntityReader вместо явного
// *multipart.Reader, чтобы не зависеть от него.
type entityMultipartProcessor struct {
	// От Storage нам тут нужен только 1 метод, так что сужаем интерфейс для меньшей связности.
	storage interface {
		SaveEvidence(ctx context.Context, src io.Reader, name string, maxSize int64) (int64, error)
	}
	dossier          dossier
	dossierProcessed bool

	totalEvidences    int
	savedEvidences    []string
	savedEvidenceSize int64
	failedEvidences   []FailedEvidence
}

// EntityReader представляет собой интерфейс для чтения multipart-тела сущности (досье и улик).
//
// Является абстракцией над *multipart.Reader, чтобы не зависеть от него и облегчить потенциальное
// тестирование.
//
// Делается допущение, что NextPart не заблочится и будет уважать контекст запроса, что справедливо
// для *multipart.Reader.
type EntityReader interface {
	NextPart() (EntityPartReader, error)
}

// EntityPartReader представляет собой интерфейс для чтения отдельной части multipart-тела сущности.
//
// Является абстракцией над *multipart.Part, чтобы не зависеть от него.
//
// Делается допущение, что Read не заблочится и будет уважать контекст запроса, что справедливо
// для *multipart.Reader.
type EntityPartReader interface {
	FormName() string
	FileName() string
	Read(p []byte) (n int, err error)
}

// process читает multipart-тело сущности (досье и улик) и обрабатывает его.
//
// Улики при этом сразу же сохраняются в хранилище, чтобы не тратить память и время на
// промежуточное состояние - они предполагаются довольно большими. Валидация их размера происходит
// уже после записи, т.к. мы не можем заранее знать этот размер до полного вычитывания.
//
// Любые ошибки чтения и нарушения структуры считаются фатальными и приводят к немедленному
// завершению с ошибкой. К немедленному завершению также приводят ошибки валидации досье, т.к.
// оно является обязательным, а при ошибках валидации улик - отбрасывается только эта конкретная
// улика.
//
// Т.о. ошибка возвращается в случае:
// 1) Нарушен формат multipart-тела (NextPart() или part.Read() вернули ошибку).
// 2) Улики не были найдены в теле.
// 3) Количество улик превышает максимум.
// 4) Досье не было найдено в теле, или было найдено в количестве больше 1.
// 5) Произошла ошибка валидации досье (например, отсутствуют какие-либо обязательные поля).
// 6) Размер тела улики или досье превышает максимум.
// 7) Найдено поле с неизвестным названием.
// Все такие ошибки оборачиваются в InvalidEntityError для облегчения классификации.
//
// Кроме того, обработка немедленно прервется при любой ошибке хранилища (при вызове
// storage.SaveEvidence()), либо при отмене переданного контекста.
func (p *entityMultipartProcessor) process(ctx context.Context, r EntityReader) error {
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
			if err := p.processDossier(part); err != nil {
				return fmt.Errorf("process dossier: %w", err)
			}
		case evidenceField:
			if err := p.processEvidence(ctx, part.FileName(), part); err != nil {
				return fmt.Errorf("process evidence: %w", err)
			}
		default:
			return invalidEntityError(unknownFieldError(part.FormName()))
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if !p.dossierProcessed {
		return invalidEntityError(ErrNoDossier)
	}

	if p.totalEvidences == 0 {
		return invalidEntityError(ErrNoEvidences)
	}

	return nil
}

// processDossier читает тело досье и выполняет следующие проверки:
// 1) Досье должно быть первым и единственным.
// 2) Тело досье не должно превышать установленного лимита.
// 3) Тело досье должно содержать валидный JSON установленного формата и не содержать никакого
// мусора в конце.
// 4) Полученные данные досье должны быть валидными (выполняется dossier.validate()).
//
// Готовое досье записывается при этом во внутреннее состояние processor'а.
func (p *entityMultipartProcessor) processDossier(r io.Reader) error {
	if p.dossierProcessed {
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

	p.dossierProcessed = true

	return nil
}

// processEvidence читает тело улики, валидирует его и сохраняет в хранилище.
//
// Возвращает ошибку при проблемах записи в хранилище или других неожиданных проблемах с IO/памятью.
// Также ошибка немедленно возвращается при превышении общего допустимого количества улик.
//
// При возникновении же ошибки в самих данных улики (формат/размер/...), она просто сохраняется в
// failedEvidences, функция ошибки не возвращает.
//
// При успехе улика сохраняется в savedEvidences состояния processor'а.
func (p *entityMultipartProcessor) processEvidence(
	ctx context.Context, fileName string, r io.Reader,
) error {
	if p.totalEvidences >= maxEvidences {
		return invalidEntityError(ErrTooManyEvidences)
	}

	name, size, err := p.ingestEvidence(ctx, r)
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

// ingestEvidence читает содержимое улики из тела, проверяет его корректность и сохраняет
// в хранилище, чтобы в том числе проверить и его размер.
//
// В качестве имени при этом используется UUID v7 с расширением, которое определяется по
// содержимому тела улики.
//
// Возвращает ошибку при невалидном типе, превышении размера, невозможности создать UUID,
// а также проблемах со чтением или записью.
func (p *entityMultipartProcessor) ingestEvidence(
	ctx context.Context, r io.Reader,
) (string, int64, error) {
	ext, header, err := p.validateEvidenceType(r)
	if err != nil {
		return "", 0, err
	}

	// r не имеет метода Seek, поэтому необходимо вернуть в поток данные заголовка, прочитанные для
	// определения типа. io.MultiReader именно это и делает - объединяет уже прочитанные данные
	// с потоком, из которого эти данные были прочитаны.
	r = io.MultiReader(bytes.NewReader(header), r)

	nameUUID, err := uuid.NewV7()
	if err != nil {
		return "", 0, fmt.Errorf("generate uuid: %w", err)
	}

	name := nameUUID.String() + ext

	// Проверку на превышение размера будет делать хранилище при записи, т.к. заранее размер
	// неизвестен. При превышении хранилище вернет storage.ErrTooLarge, что будет обернуто
	// в ErrEvidenceTooLarge.
	size, err := p.storage.SaveEvidence(ctx, r, name, maxEvidenceSize)
	if err != nil {
		return "", 0, p.translateSaveEvidenceError(err)
	}

	return name, size, nil
}

// translateSaveEvidenceError оборачивает ошибку сохранения в хранилище, если это необходимо,
// чтобы клиенты сервиса не зависели от ошибок storage.
//
// В итоге storage.ErrStorageOp оборачивается в ErrStorage, storage.ErrTooLarge -
// в ErrEvidenceTooLarge, а storage.ErrRead - в ErrReadEvidence.
func (p *entityMultipartProcessor) translateSaveEvidenceError(err error) error {
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

// validateEvidenceType определяет и проверяет тип данных улики на валидность, и
// при успехе возвращает расширение изображения, вычисленное по MIME-типу, а также
// прочитанные для определения типа заголовочные данные.
//
// Проверяется следующее: поток не пуст, его можно читать (читаются только первые 512 байт),
// и что его вычисленное MIME в списке разрешенных.
func (p *entityMultipartProcessor) validateEvidenceType(r io.Reader) (string, []byte, error) {
	header := make([]byte, 512)

	n, err := io.ReadFull(r, header)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", nil, fmt.Errorf("%w: %w", invalidEntityError(ErrReadEvidenceMIME), err)
	}

	if n == 0 {
		return "", nil, invalidEntityError(ErrEvidenceEmpty)
	}

	header = header[:n]
	contentType := http.DetectContentType(header)
	ext, ok := evidenceMIME2Ext[contentType]
	if !ok {
		return "", nil, invalidEntityError(ErrInvalidEvidenceType)
	}

	return ext, header, nil
}

// evidenceErrToReason преобразует ошибку в причину неудачи, которая возвращается клиенту.
//
// Если ошибка фатальная (ошибка хранилища, либо нераспознанная), то возвращается "", false, иначе -
// строка с причиной и true.
func (p *entityMultipartProcessor) evidenceErrToReason(err error) (FailReason, bool) {
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
