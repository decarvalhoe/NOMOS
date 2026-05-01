package corpus

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParcoursFile is the top-level YAML structure for a parcours file.
type ParcoursFile struct {
	Parcours Parcours `yaml:"parcours"`
}

// Parcours describes a structured learning or business path.
type Parcours struct {
	ID      string  `yaml:"id"`
	Name    string  `yaml:"name"`
	Domain  string  `yaml:"domain"`
	Version string  `yaml:"version"`
	Owner   string  `yaml:"owner"`
	Status  string  `yaml:"status"`
	Etapes  []Etape `yaml:"etapes"`
}

// Etape is a stage within a parcours.
type Etape struct {
	ID          string     `yaml:"id"`
	Name        string     `yaml:"name"`
	Description string     `yaml:"description"`
	Ordre       int        `yaml:"ordre"`
	Objectifs   []Objectif `yaml:"objectifs"`
}

// Objectif is a goal within an étape.
type Objectif struct {
	ID          string    `yaml:"id"`
	Description string    `yaml:"description"`
	Criteres    []Critere `yaml:"criteres"`
}

// Critere is a single verifiable criterion within an objectif.
type Critere struct {
	ID          string `yaml:"id"`
	Description string `yaml:"description"`
	Type        string `yaml:"type"`
	Criticality string `yaml:"criticality"`
}

// ParcoursUnit is a canonical unit extracted from a parcours.
// Each critère becomes one unit, traceable to its étape and objectif.
type ParcoursUnit struct {
	UnitID       string `json:"unit_id"       yaml:"unit_id"`
	UnitType     string `json:"unit_type"     yaml:"unit_type"`
	Name         string `json:"name"          yaml:"name"`
	Domain       string `json:"domain"        yaml:"domain"`
	Criticality  string `json:"criticality"   yaml:"criticality"`
	BusinessRule string `json:"business_rule" yaml:"business_rule"`
	EtapeID      string `json:"etape_id"      yaml:"etape_id"`
	EtapeName    string `json:"etape_name"    yaml:"etape_name"`
	ObjectifID   string `json:"objectif_id"   yaml:"objectif_id"`
	ParcoursID   string `json:"parcours_id"   yaml:"parcours_id"`
	Owner        string `json:"owner"         yaml:"owner"`
	Status       string `json:"status"        yaml:"status"`
}

// ExtractResult holds extracted units and metadata.
type ExtractResult struct {
	ParcoursID   string         `json:"parcours_id"   yaml:"parcours_id"`
	ParcoursName string         `json:"parcours_name" yaml:"parcours_name"`
	Domain       string         `json:"domain"        yaml:"domain"`
	TotalEtapes  int            `json:"total_etapes"  yaml:"total_etapes"`
	TotalUnits   int            `json:"total_units"   yaml:"total_units"`
	Units        []ParcoursUnit `json:"units"         yaml:"units"`
}

// ExtractParcours reads a parcours YAML file and extracts canonical units.
func ExtractParcours(path string) (ExtractResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ExtractResult{}, fmt.Errorf("read parcours: %w", err)
	}
	return ExtractParcoursFromBytes(data)
}

// ExtractParcoursFromBytes extracts canonical units from parcours YAML bytes.
func ExtractParcoursFromBytes(data []byte) (ExtractResult, error) {
	var file ParcoursFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return ExtractResult{}, fmt.Errorf("parse parcours: %w", err)
	}

	p := file.Parcours
	if p.ID == "" {
		return ExtractResult{}, fmt.Errorf("parcours.id is required")
	}

	var units []ParcoursUnit
	for _, etape := range p.Etapes {
		for _, objectif := range etape.Objectifs {
			for _, critere := range objectif.Criteres {
				units = append(units, ParcoursUnit{
					UnitID:       makeParcoursUnitID(p.ID, etape.ID, critere.ID),
					UnitType:     mapCritereType(critere.Type),
					Name:         critere.Description,
					Domain:       p.Domain,
					Criticality:  normalizeCriticality(critere.Criticality),
					BusinessRule: critere.Description,
					EtapeID:      etape.ID,
					EtapeName:    etape.Name,
					ObjectifID:   objectif.ID,
					ParcoursID:   p.ID,
					Owner:        p.Owner,
					Status:       "partial",
				})
			}
		}
	}

	return ExtractResult{
		ParcoursID:   p.ID,
		ParcoursName: p.Name,
		Domain:       p.Domain,
		TotalEtapes:  len(p.Etapes),
		TotalUnits:   len(units),
		Units:        units,
	}, nil
}

func makeParcoursUnitID(parcoursID string, etapeID string, critereID string) string {
	slug := fmt.Sprintf("RBOK-PARCOURS-%s-%s-%s", parcoursID, etapeID, critereID)
	return toUpperSlug(slug)
}

func toUpperSlug(s string) string {
	s = strings.ToUpper(s)
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func mapCritereType(t string) string {
	switch strings.ToLower(t) {
	case "rule":
		return "rule"
	case "formula":
		return "formula"
	case "exception":
		return "exception"
	case "scenario":
		return "scenario"
	case "term":
		return "term"
	case "decision":
		return "decision"
	default:
		return "rule"
	}
}

func normalizeCriticality(c string) string {
	switch strings.ToLower(c) {
	case "low", "medium", "high", "critical":
		return strings.ToLower(c)
	default:
		return "medium"
	}
}
