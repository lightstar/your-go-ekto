package handler

import (
	"github.com/lightstar/your-go-ekto/internal/model"
	"github.com/lightstar/your-go-ekto/internal/service"
)

// CreateEntityResponse представляет собой ответ с результатом создания сущности.
// Возвращается, когда форму удалось распарсить, и валидным оказалось как минимум досье и хотя бы
// одна улика.
type CreateEntityResponse struct {
	DossierID      string               `json:"dossier_id"`
	Status         service.CreateStatus `json:"status"`
	SavedEvidence  []string             `json:"saved_evidence"`
	FailedEvidence []FailedEvidence     `json:"failed_evidence"`
}

// CreateEntityFailedResponse представляет собой ответ на запрос на создание сущности, в случае,
// когда форму удалось распарсить, досье оказалось валидным, но ни одну улику сохранить не удалось.
type CreateEntityFailedResponse struct {
	FailedEvidence []FailedEvidence `json:"failed_evidence"`
	Error          string           `json:"error"`
}

// FailedEvidence представляет собой информацию об улике, которую не удалось сохранить.
type FailedEvidence struct {
	OriginalName string             `json:"original_name"`
	FailReason   service.FailReason `json:"reason"`
}

// GetEntityResponse представляет собой ответ на запрос на получение сущности.
type GetEntityResponse struct {
	DossierID       string   `json:"dossier_id"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	ThreatLevel     int      `json:"threat_level"`
	Vulnerabilities []string `json:"vulnerabilities"`
	EvidenceURLs    []string `json:"evidence_urls"`
}

// createEntityResponse создает ответ с результатом создания сущности по результату, возвращенному
// сервисом. Конвертация происходит почти один-в-один, но используется собственный тип данных
// с тегами JSON, а также имена сохраненных улик преобразуются в полные URL-пути к ним.
func (h *Handler) createEntityResponse(result service.CreateEntityResult) CreateEntityResponse {
	return CreateEntityResponse{
		DossierID:      result.DossierID,
		Status:         result.Status,
		SavedEvidence:  h.evidenceURLs(result.SavedEvidence),
		FailedEvidence: h.failedEvidences(result.FailedEvidence),
	}
}

// createEntityFailedResponse создает ответ с результатом создания сущности по результату,
// возвращенному сервисом. Возвращается в случае, когда форму удалось распарсить, досье оказалось
// валидным, но ни одну улику сохранить не удалось.
func (h *Handler) createEntityFailedResponse(
	result service.CreateEntityResult, message string,
) CreateEntityFailedResponse {
	return CreateEntityFailedResponse{
		FailedEvidence: h.failedEvidences(result.FailedEvidence),
		Error:          message,
	}
}

// failedEvidences преобразует список улик, которые не удалось сохранить, в формат, подходящий
// для ответа на запрос на создание сущности.
func (h *Handler) failedEvidences(inEvidences []service.FailedEvidence) []FailedEvidence {
	outEvidences := make([]FailedEvidence, len(inEvidences))

	for i, evidence := range inEvidences {
		outEvidences[i] = FailedEvidence{
			OriginalName: evidence.OriginalName,
			FailReason:   evidence.FailReason,
		}
	}

	return outEvidences
}

// getEntityResponse создает ответ с результатом получения сущности. Доменная структура Entity здесь
// преобразуется в собственный тип данных ответа с JSON-тегами, а также имена улик преобразуются
// в полные URL-пути к ним.
func (h *Handler) getEntityResponse(entity model.Entity) GetEntityResponse {
	return GetEntityResponse{
		DossierID:       entity.DossierID.String(),
		Name:            entity.Name,
		Description:     entity.Description,
		ThreatLevel:     entity.ThreatLevel,
		Vulnerabilities: entity.Vulnerabilities,
		EvidenceURLs:    h.evidenceURLs(entity.Evidences),
	}
}

// evidenceURLs преобразует список имен сохраненных улик в полные URL-пути к ним.
func (h *Handler) evidenceURLs(evidences []string) []string {
	evidenceURLs := make([]string, len(evidences))

	for i, savedEvidence := range evidences {
		evidenceURLs[i] = h.apiPrefix + evidencePrefix + savedEvidence
	}

	return evidenceURLs
}
