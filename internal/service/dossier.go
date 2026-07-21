package service

import (
	"fmt"
	"slices"
	"strings"
)

const (
	minThreatLevel = 1
	maxThreatLevel = 10
)

// dossier - структура, представляющая досье, распарсенное при парсинге сущности из EntityReader.
// Его методы позволяют нормализовать данные досье и валидировать их.
type dossier struct {
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	ThreatLevel     int      `json:"threat_level"`
	Vulnerabilities []string `json:"vulnerabilities"`
}

func (d *dossier) normalize() {
	d.Name = strings.TrimSpace(d.Name)
	d.Description = strings.TrimSpace(d.Description)

	d.Vulnerabilities = slices.Clone(d.Vulnerabilities)
	for i, v := range d.Vulnerabilities {
		d.Vulnerabilities[i] = strings.TrimSpace(v)
	}
}

func (d dossier) validate() error {
	if d.Name == "" {
		return dossierValidationError("name is required")
	}

	if d.Description == "" {
		return dossierValidationError("description is required")
	}

	if d.ThreatLevel < minThreatLevel || d.ThreatLevel > maxThreatLevel {
		return dossierValidationError(fmt.Sprintf("threat level must be between %d and %d",
			minThreatLevel, maxThreatLevel))
	}

	if len(d.Vulnerabilities) == 0 {
		return dossierValidationError("no vulnerabilities")
	}

	if slices.Contains(d.Vulnerabilities, "") {
		return dossierValidationError("has empty vulnerability")
	}

	return nil
}
