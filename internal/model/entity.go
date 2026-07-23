package model

import "github.com/google/uuid"

// Entity - структура с описанием паранормальной сущности, включая досье и список улик.
// Ее экземпляры сохраняются в хранилище и отдаются из него.
type Entity struct {
	DossierID       uuid.UUID `json:"dossier_id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	ThreatLevel     int       `json:"threat_level"`
	Vulnerabilities []string  `json:"vulnerabilities"`
	Evidence        []string  `json:"evidence"`
}
