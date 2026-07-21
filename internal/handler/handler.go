package handler

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/lightstar/your-go-ekto/internal/httpx"
	"github.com/lightstar/your-go-ekto/internal/model"
	"github.com/lightstar/your-go-ekto/internal/service"

	"time"

	"github.com/google/uuid"
)

const (
	maxUploadBodySize = 20 << 20
	evidencePrefix    = "evidence/"
	entitiesPrefix    = "entities/"
)

// Service представляет собой интерфейс для работы с сервисом сущностей и улик.
type Service interface {
	CreateEntity(ctx context.Context, r service.EntityReader) (service.CreateEntityResult, error)
	GetEntity(id uuid.UUID) (model.Entity, error)
	GetEvidence(fileName string) (time.Time, io.ReadSeekCloser, error)
}

// Handler обрабатывает HTTP-запросы для работы с сущностями и уликами.
type Handler struct {
	service   Service
	logger    *httpx.RequestLogger
	apiPrefix string
}

// New создает новый экземпляр Handler.
// apiPrefix - префикс url-пути, используемый для доступа к обработчикам. Здесь он не валидируется,
// проверяется только пустота, вызывающий код должен его валидировать предварительно сам.
func New(
	service Service, logger httpx.Logger, apiPrefix string,
) (*Handler, error) {
	if service == nil {
		return nil, ErrServiceRequired
	}

	if logger == nil {
		return nil, ErrLoggerRequired
	}

	if apiPrefix == "" {
		return nil, ErrAPIPrefixRequired
	}

	return &Handler{
		service:   service,
		logger:    httpx.NewRequestLogger(logger),
		apiPrefix: apiPrefix,
	}, nil
}

// APIPrefix возвращает префикс API обработчика, что может пригодиться при настройке роутинга.
func (h *Handler) APIPrefix() string {
	return h.apiPrefix
}

// CreateEntity обрабатывает запрос на создание сущности и улик из multipart-формы.
func (h *Handler) CreateEntity(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBodySize)
	defer r.Body.Close()

	reader, err := r.MultipartReader()
	if err != nil {
		h.handleCreateEntityError(w, r, err)
		return
	}

	createResult, err := h.service.CreateEntity(r.Context(), &entityReader{r: reader})
	if err != nil {
		h.handleCreateEntityError(w, r, err)
		return
	}

	// service.StatusFailed означает, что ничего не было сохранено, хотя форма и была в целом
	// распарсена. Значит мы должны вернуть 400 Bad Request и сообщение об ошибке.
	// Но вместе с этим в качестве дополнительной информации мы возвращаем еще и список улик,
	// которые не удалось сохранить.
	if createResult.Status == service.StatusFailed {
		msg, code := "no valid evidences", http.StatusBadRequest
		h.logger.Debug(r, msg, slog.Any("result", createResult))
		h.writeJSONResponse(w, r, h.createEntityFailedResponse(createResult, msg), code)
		return
	}

	h.logger.Info(r, fmt.Sprintf("[ANALYTICS] Dossier ID: %s | Paranormal Index (P): %.2f",
		createResult.DossierID, createResult.ParanormalIndex))

	code := http.StatusMultiStatus
	if createResult.Status == service.StatusSuccess {
		// Location требуется только для статуса 201 Created.
		w.Header().Set("Location", h.apiPrefix+entitiesPrefix+createResult.DossierID)
		code = http.StatusCreated
	}

	h.writeJSONResponse(w, r, h.createEntityResponse(createResult), code)
}

// GetEntity обрабатывает запрос на получение досье на сущность по ID.
func (h *Handler) GetEntity(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.error(w, r, "invalid dossier id", http.StatusBadRequest)
		return
	}

	entity, err := h.service.GetEntity(id)
	if err != nil {
		h.handleGetEntityError(w, r, err)
		return
	}

	h.writeJSONResponse(w, r, h.getEntityResponse(entity), http.StatusOK)
}

// GetEvidence обрабатывает запрос на получение файла с уликой по имени.
// Для отдачи тела улики клиенту используется http.ServeContent, который автоматически генерирует
// корректные заголовки вроде Content-Type и Content-Length, а также обрабатывает Range-запросы.
func (h *Handler) GetEvidence(w http.ResponseWriter, r *http.Request) {
	fileName := r.PathValue("filename")

	modTime, body, err := h.service.GetEvidence(fileName)
	if err != nil {
		h.handleGetEvidenceError(w, r, fileName, err)
		return
	}
	defer body.Close()

	http.ServeContent(w, r, fileName, modTime, body)
}

// writeJSONResponse - вспомогательный метод для отправки JSON-ответа клиенту.
func (h *Handler) writeJSONResponse(w http.ResponseWriter, r *http.Request, response any, code int) {
	if err := httpx.WriteJSONResponse(w, r, response, code); err != nil {
		h.handleWriteError(w, r, err)
	}
}
