package corpus

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractParcoursFullFile(t *testing.T) {
	result, err := ExtractParcours(filepath.Join("testdata", "parcours-assurance-habitation.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ParcoursID != "assurance-habitation" {
		t.Fatalf("expected assurance-habitation, got %q", result.ParcoursID)
	}
	if result.Domain != "insurance-home" {
		t.Fatalf("expected insurance-home, got %q", result.Domain)
	}
	if result.TotalEtapes != 3 {
		t.Fatalf("expected 3 etapes, got %d", result.TotalEtapes)
	}
	// 4 critères souscription + 3 sinistre + 1 résiliation = 8
	if result.TotalUnits != 8 {
		t.Fatalf("expected 8 units, got %d", result.TotalUnits)
	}
}

func TestExtractParcoursUnitIDFormat(t *testing.T) {
	result, err := ExtractParcours(filepath.Join("testdata", "parcours-assurance-habitation.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range result.Units {
		if !strings.HasPrefix(unit.UnitID, "RBOK-PARCOURS-") {
			t.Fatalf("expected RBOK-PARCOURS- prefix, got %q", unit.UnitID)
		}
		// Verify all uppercase + digits + dashes only
		for _, r := range unit.UnitID {
			if !((r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-') {
				t.Fatalf("ID %q has invalid char %q", unit.UnitID, string(r))
			}
		}
	}
}

func TestExtractParcoursUnitIDsUnique(t *testing.T) {
	result, err := ExtractParcours(filepath.Join("testdata", "parcours-assurance-habitation.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for _, unit := range result.Units {
		if seen[unit.UnitID] {
			t.Fatalf("duplicate unit ID: %q", unit.UnitID)
		}
		seen[unit.UnitID] = true
	}
}

func TestExtractParcoursUnitTypes(t *testing.T) {
	result, err := ExtractParcours(filepath.Join("testdata", "parcours-assurance-habitation.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	hasRule := false
	hasFormula := false
	for _, unit := range result.Units {
		switch unit.UnitType {
		case "rule":
			hasRule = true
		case "formula":
			hasFormula = true
		}
	}
	if !hasRule {
		t.Fatal("expected at least one rule unit")
	}
	if !hasFormula {
		t.Fatal("expected at least one formula unit")
	}
}

func TestExtractParcoursCriticality(t *testing.T) {
	result, err := ExtractParcours(filepath.Join("testdata", "parcours-assurance-habitation.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	hasCritical := false
	for _, unit := range result.Units {
		if unit.Criticality == "critical" {
			hasCritical = true
		}
		// All criticalities must be valid
		switch unit.Criticality {
		case "low", "medium", "high", "critical":
			// ok
		default:
			t.Fatalf("unexpected criticality %q for %s", unit.Criticality, unit.UnitID)
		}
	}
	if !hasCritical {
		t.Fatal("expected at least one critical unit")
	}
}

func TestExtractParcoursTraceability(t *testing.T) {
	result, err := ExtractParcours(filepath.Join("testdata", "parcours-assurance-habitation.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range result.Units {
		if unit.ParcoursID != "assurance-habitation" {
			t.Fatalf("expected parcours_id assurance-habitation, got %q", unit.ParcoursID)
		}
		if unit.EtapeID == "" {
			t.Fatalf("expected non-empty etape_id for %s", unit.UnitID)
		}
		if unit.ObjectifID == "" {
			t.Fatalf("expected non-empty objectif_id for %s", unit.UnitID)
		}
		if unit.Owner == "" {
			t.Fatalf("expected non-empty owner for %s", unit.UnitID)
		}
		if unit.Domain != "insurance-home" {
			t.Fatalf("expected domain insurance-home for %s, got %q", unit.UnitID, unit.Domain)
		}
	}
}

func TestExtractParcoursMinimal(t *testing.T) {
	result, err := ExtractParcours(filepath.Join("testdata", "parcours-minimal.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalUnits != 1 {
		t.Fatalf("expected 1 unit, got %d", result.TotalUnits)
	}
	if result.Units[0].UnitType != "rule" {
		t.Fatalf("expected rule, got %q", result.Units[0].UnitType)
	}
}

func TestExtractParcoursEmptyEtapes(t *testing.T) {
	result, err := ExtractParcours(filepath.Join("testdata", "parcours-empty-etapes.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalUnits != 0 {
		t.Fatalf("expected 0 units, got %d", result.TotalUnits)
	}
	if result.TotalEtapes != 0 {
		t.Fatalf("expected 0 etapes, got %d", result.TotalEtapes)
	}
}

func TestExtractParcoursFromBytes(t *testing.T) {
	data := []byte(`
parcours:
  id: inline
  name: Inline Test
  domain: test
  version: "1"
  owner: test@test.com
  status: active
  etapes:
    - id: step1
      name: Step 1
      description: First step
      ordre: 1
      objectifs:
        - id: obj1
          description: Objective 1
          criteres:
            - id: crit1
              description: Criterion 1
              type: rule
              criticality: high
            - id: crit2
              description: Criterion 2
              type: formula
              criticality: critical
`)
	result, err := ExtractParcoursFromBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalUnits != 2 {
		t.Fatalf("expected 2 units, got %d", result.TotalUnits)
	}
	if result.Units[0].UnitType != "rule" {
		t.Fatalf("expected rule, got %q", result.Units[0].UnitType)
	}
	if result.Units[1].UnitType != "formula" {
		t.Fatalf("expected formula, got %q", result.Units[1].UnitType)
	}
}

func TestExtractParcoursMissingID(t *testing.T) {
	data := []byte(`
parcours:
  name: No ID
  domain: test
  etapes: []
`)
	_, err := ExtractParcoursFromBytes(data)
	if err == nil {
		t.Fatal("expected error for missing parcours id")
	}
}

func TestExtractParcoursInvalidYAML(t *testing.T) {
	_, err := ExtractParcoursFromBytes([]byte(`{broken`))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestExtractParcoursNonexistentFile(t *testing.T) {
	_, err := ExtractParcours("/nonexistent/parcours.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestExtractParcoursUnknownCritereType(t *testing.T) {
	data := []byte(`
parcours:
  id: unknown-type
  name: Unknown
  domain: test
  version: "1"
  owner: test@test.com
  status: active
  etapes:
    - id: s1
      name: S1
      description: Step
      ordre: 1
      objectifs:
        - id: o1
          description: Obj
          criteres:
            - id: c1
              description: Crit
              type: custom_type
              criticality: extreme
`)
	result, err := ExtractParcoursFromBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	// Unknown type defaults to "rule"
	if result.Units[0].UnitType != "rule" {
		t.Fatalf("expected default rule, got %q", result.Units[0].UnitType)
	}
	// Unknown criticality defaults to "medium"
	if result.Units[0].Criticality != "medium" {
		t.Fatalf("expected default medium, got %q", result.Units[0].Criticality)
	}
}

func TestExtractParcoursSpecificUnitID(t *testing.T) {
	result, err := ExtractParcours(filepath.Join("testdata", "parcours-assurance-habitation.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	// First critère: souscription → eligibilite → surface-habitable
	expected := "RBOK-PARCOURS-ASSURANCE-HABITATION-SOUSCRIPTION-SURFACE-HABITABLE"
	if result.Units[0].UnitID != expected {
		t.Fatalf("expected %q, got %q", expected, result.Units[0].UnitID)
	}
}
