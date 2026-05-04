package corpus

import (
	"os"
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

func TestExtractParcoursProductionModuleShape(t *testing.T) {
	data := []byte(`
parcours:
  code: PAR_ACC_ADMIN
  name: Les bases administratives
  modules:
    - code: MOD_ADMIN_STATUT
      name: Statut et structure
      type: conversational
      ai_instructions: Verifier le statut juridique.
      objectives:
        - key: statut-juridique
          titre: Statut et inscriptions
          questions:
            - key: statut-choisi
              label: Quel statut juridique avez-vous choisi ?
              type: select
              help_text: Raison individuelle, Sarl ou SA.
`)
	result, err := ExtractParcoursFromBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if result.ParcoursID != "PAR_ACC_ADMIN" {
		t.Fatalf("expected code fallback as parcours id, got %q", result.ParcoursID)
	}
	if result.TotalUnits != 2 {
		t.Fatalf("expected module + question units, got %d", result.TotalUnits)
	}
	if result.Units[0].UnitType != "workflow" {
		t.Fatalf("expected module workflow unit, got %q", result.Units[0].UnitType)
	}
	if result.Units[1].UnitType != "rule" {
		t.Fatalf("expected question rule unit, got %q", result.Units[1].UnitType)
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

// ----------------------------------------------------------------------------
// FSQ-04 (#367) — YAML raw / decoded / key-path provenance.
// ----------------------------------------------------------------------------

// TestExtractParcoursFSQ04QuotedVsUnquoted asserts that a double-quoted
// scalar carries its quotes in RawText while DecodedValue is the unquoted
// string, and that an adjacent unquoted scalar carries no quotes.
func TestExtractParcoursFSQ04QuotedVsUnquoted(t *testing.T) {
	data := []byte("parcours:\n" +
		"  id: q-vs-uq\n" +
		"  domain: test\n" +
		"  owner: t@t.t\n" +
		"  status: active\n" +
		"  etapes:\n" +
		"    - id: e1\n" +
		"      name: E1\n" +
		"      description: Step\n" +
		"      ordre: 1\n" +
		"      objectifs:\n" +
		"        - id: o1\n" +
		"          description: Obj\n" +
		"          criteres:\n" +
		"            - id: c1\n" +
		"              description: \"actif >= 18 ans\"\n" +
		"              type: rule\n" +
		"              criticality: high\n" +
		"            - id: c2\n" +
		"              description: actif >= 18\n" +
		"              type: rule\n" +
		"              criticality: high\n")
	result, err := ExtractParcoursFromBytes(data)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(result.Units) != 2 {
		t.Fatalf("expected 2 units; got %d", len(result.Units))
	}
	c1 := result.Units[0]
	c2 := result.Units[1]
	if c1.RawText != "\"actif >= 18 ans\"" {
		t.Fatalf("expected quoted raw_text; got %q", c1.RawText)
	}
	if c1.DecodedValue != "actif >= 18 ans" {
		t.Fatalf("expected decoded c1=%q; got %q", "actif >= 18 ans", c1.DecodedValue)
	}
	if c2.RawText != "actif >= 18" {
		t.Fatalf("expected unquoted raw_text; got %q", c2.RawText)
	}
	if c2.DecodedValue != "actif >= 18" {
		t.Fatalf("expected decoded c2=%q; got %q", "actif >= 18", c2.DecodedValue)
	}
	if c1.NodeKind != "scalar_string" || c2.NodeKind != "scalar_string" {
		t.Fatalf("expected node_kind=scalar_string for both; got %q %q",
			c1.NodeKind, c2.NodeKind)
	}
}

// TestExtractParcoursFSQ04DuplicateValuesDistinctPaths verifies that two
// scalars with identical decoded values but different YAML key paths bind
// to distinct units. This is the core regression guard against the old
// "first-unused-value" matching that this ticket replaces.
func TestExtractParcoursFSQ04DuplicateValuesDistinctPaths(t *testing.T) {
	data := []byte("parcours:\n" +
		"  id: dup\n" +
		"  domain: test\n" +
		"  owner: t@t.t\n" +
		"  status: active\n" +
		"  etapes:\n" +
		"    - id: e1\n" +
		"      name: E1\n" +
		"      description: Step\n" +
		"      ordre: 1\n" +
		"      objectifs:\n" +
		"        - id: o1\n" +
		"          description: Obj\n" +
		"          criteres:\n" +
		"            - id: c1\n" +
		"              description: same value\n" +
		"              type: rule\n" +
		"              criticality: high\n" +
		"            - id: c2\n" +
		"              description: same value\n" +
		"              type: rule\n" +
		"              criticality: high\n")
	result, err := ExtractParcoursFromBytes(data)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(result.Units) != 2 {
		t.Fatalf("expected 2 distinct units; got %d", len(result.Units))
	}
	if result.Units[0].DecodedValue != result.Units[1].DecodedValue {
		t.Fatalf("expected identical decoded values; got %q vs %q",
			result.Units[0].DecodedValue, result.Units[1].DecodedValue)
	}
	if result.Units[0].YAMLPath == result.Units[1].YAMLPath {
		t.Fatalf("expected distinct yaml_path values; both = %q",
			result.Units[0].YAMLPath)
	}
	if result.Units[0].YAMLPath != "parcours.etapes[0].objectifs[0].criteres[0].description" {
		t.Fatalf("unexpected yaml_path[0] = %q", result.Units[0].YAMLPath)
	}
	if result.Units[1].YAMLPath != "parcours.etapes[0].objectifs[0].criteres[1].description" {
		t.Fatalf("unexpected yaml_path[1] = %q", result.Units[1].YAMLPath)
	}
	if result.Units[0].UnitID == result.Units[1].UnitID {
		t.Fatalf("duplicate value collapsed two units into one: %q",
			result.Units[0].UnitID)
	}
}

// TestExtractParcoursFSQ04EscapeSequences asserts that a YAML scalar with
// escape sequences exposes the raw bytes (with backslashes) in RawText and
// the resolved string in DecodedValue.
func TestExtractParcoursFSQ04EscapeSequences(t *testing.T) {
	data := []byte("parcours:\n" +
		"  id: esc\n" +
		"  domain: test\n" +
		"  owner: t@t.t\n" +
		"  status: active\n" +
		"  etapes:\n" +
		"    - id: e1\n" +
		"      name: E1\n" +
		"      description: Step\n" +
		"      ordre: 1\n" +
		"      objectifs:\n" +
		"        - id: o1\n" +
		"          description: Obj\n" +
		"          criteres:\n" +
		"            - id: c1\n" +
		"              description: \"line\\nwith\\\"escapes\"\n" +
		"              type: rule\n" +
		"              criticality: high\n")
	result, err := ExtractParcoursFromBytes(data)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(result.Units) != 1 {
		t.Fatalf("expected 1 unit; got %d", len(result.Units))
	}
	u := result.Units[0]
	if !strings.Contains(u.RawText, "\\n") || !strings.Contains(u.RawText, "\\\"") {
		t.Fatalf("expected raw_text to retain backslash escapes; got %q", u.RawText)
	}
	if u.DecodedValue != "line\nwith\"escapes" {
		t.Fatalf("expected decoded to resolve escapes; got %q", u.DecodedValue)
	}
}

// TestExtractParcoursFSQ04SequenceIndexedPath checks that yaml_path
// includes [N] sequence indices for nested module/objective/question
// scalars (e.g. parcours.modules[2].questions[7].help_text).
func TestExtractParcoursFSQ04SequenceIndexedPath(t *testing.T) {
	data := []byte("parcours:\n" +
		"  code: PAR_SEQ\n" +
		"  modules:\n" +
		"    - code: M0\n" +
		"      name: Mod 0\n" +
		"      type: conversational\n" +
		"      ai_instructions: zero\n" +
		"      objectives: []\n" +
		"    - code: M1\n" +
		"      name: Mod 1\n" +
		"      type: conversational\n" +
		"      ai_instructions: one\n" +
		"      objectives: []\n" +
		"    - code: M2\n" +
		"      name: Mod 2\n" +
		"      type: conversational\n" +
		"      objectives:\n" +
		"        - key: o0\n" +
		"          titre: Objective 0\n" +
		"          description: Objective desc 0\n" +
		"          questions: []\n" +
		"        - key: o1\n" +
		"          titre: Objective 1\n" +
		"          description: Objective desc 1\n" +
		"          questions:\n" +
		"            - key: q0\n" +
		"              label: Q0\n" +
		"              type: text\n" +
		"              help_text: Help 0\n" +
		"            - key: q1\n" +
		"              label: Q1\n" +
		"              type: text\n" +
		"              help_text: Help 1\n" +
		"            - key: q2\n" +
		"              label: Q2\n" +
		"              type: text\n" +
		"              help_text: Help 2\n")
	result, err := ExtractParcoursFromBytes(data)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	var q1Path string
	for _, u := range result.Units {
		if u.UnitID == "RBOK-PARCOURS-PAR-SEQ-M2-Q1" {
			q1Path = u.YAMLPath
			break
		}
	}
	if q1Path == "" {
		t.Fatalf("could not locate question q1 unit in result.Units (n=%d)",
			len(result.Units))
	}
	expected := "parcours.modules[2].objectives[1].questions[1].help_text"
	if q1Path != expected {
		t.Fatalf("expected yaml_path=%q; got %q", expected, q1Path)
	}
}

// TestExtractParcoursFSQ04BusinessRuleModeAlwaysSet asserts every emitted
// unit declares its BusinessRuleMode. For the parcours flow this is
// always "decoded" — the BusinessRule field carries the YAML-decoded
// scalar value (what yaml.Unmarshal produced).
func TestExtractParcoursFSQ04BusinessRuleModeAlwaysSet(t *testing.T) {
	result, err := ExtractParcours(filepath.Join("testdata", "parcours-assurance-habitation.yaml"))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(result.Units) == 0 {
		t.Fatalf("expected at least one unit")
	}
	for _, u := range result.Units {
		if u.BusinessRuleMode == "" {
			t.Fatalf("unit %s has empty BusinessRuleMode", u.UnitID)
		}
		if u.BusinessRuleMode != "decoded" {
			t.Fatalf("unit %s has unexpected BusinessRuleMode=%q (want decoded)",
				u.UnitID, u.BusinessRuleMode)
		}
		if u.DecodedValue != "" && u.DecodedValue != u.BusinessRule {
			t.Fatalf("unit %s: BusinessRuleMode=decoded should imply DecodedValue==BusinessRule; got %q vs %q",
				u.UnitID, u.DecodedValue, u.BusinessRule)
		}
	}
}

// TestExtractParcoursFSQ04SpanRoundTripsRawText asserts the [start_byte,
// end_byte) slice of the original YAML reconstructs RawText exactly. This
// is the integrity guarantee callers rely on to re-prove which bytes fed
// the unit.
func TestExtractParcoursFSQ04SpanRoundTripsRawText(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "parcours-assurance-habitation.yaml"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	result, err := ExtractParcoursFromBytes(data)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	checked := 0
	for _, u := range result.Units {
		if u.RawText == "" {
			continue
		}
		if u.StartByte < 0 || u.EndByte > len(data) || u.StartByte >= u.EndByte {
			t.Fatalf("unit %s has invalid span [%d,%d) over %d bytes",
				u.UnitID, u.StartByte, u.EndByte, len(data))
		}
		got := string(data[u.StartByte:u.EndByte])
		if got != u.RawText {
			t.Fatalf("unit %s: span slice %q != raw_text %q", u.UnitID, got, u.RawText)
		}
		checked++
	}
	if checked == 0 {
		t.Fatalf("expected at least one unit with raw_text/span populated")
	}
}
