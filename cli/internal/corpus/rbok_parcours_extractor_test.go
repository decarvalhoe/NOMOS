package corpus

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixtureCompleteParcours = `
version: "2.1.0"
owner: "equipe-formation"
status: "active"
domain: "assurance-vie"
parcours:
  - id: parcours-01
    name: "Initiation assurance vie"
    source_rbok: "RBOK:assurance/vie/base"
    contenu_reference: "RBOK:contenu/init-vie"
    modules:
      - id: module-01
        name: "Fondamentaux"
        source_rbok: "RBOK:assurance/vie/fondamentaux"
        contenu_reference: "RBOK:contenu/fondamentaux"
        questions:
          - id: q-01
            text: "Quelle est la difference entre UC et fonds euro?"
            source_rbok: "RBOK:assurance/vie/uc-vs-euro"
            contenu_reference: "RBOK:contenu/uc-euro"
          - id: q-02
            text: "Quels sont les frais applicables?"
            source_rbok: "RBOK:assurance/vie/frais"
            contenu_reference: "RBOK:contenu/frais-vie"
`

const fixtureMissingGovernance = `
parcours:
  - id: parcours-orphan
    name: "Sans gouvernance"
    source_rbok: "rbok:orphan/path"
    modules: []
`

const fixtureUnresolvedRefs = `
version: "1.0.0"
owner: "team-x"
status: "draft"
domain: "prevoyance"
parcours:
  - id: parcours-prev
    name: "Prevoyance"
    source_rbok: "RBOK:prevoyance/base"
    contenu_reference: ""
    modules:
      - id: mod-01
        name: "Module 1"
        source_rbok: ""
        contenu_reference: "RBOK:contenu/prev-mod1"
        questions:
          - id: q-01
            text: "See RBOK:prevoyance/hidden-ref for details"
            source_rbok: ""
            contenu_reference: ""
`

func TestExtractParcoursComplete(t *testing.T) {
	result, err := ExtractParcoursFromBytes("parcours/vie.yaml", []byte(fixtureCompleteParcours))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	m := result.Manifest
	if m.Version != "2.1.0" {
		t.Fatalf("expected version 2.1.0, got %s", m.Version)
	}
	if m.Owner != "equipe-formation" {
		t.Fatalf("expected owner equipe-formation, got %s", m.Owner)
	}
	if m.Status != "active" {
		t.Fatalf("expected status active, got %s", m.Status)
	}
	if m.Domain != "assurance-vie" {
		t.Fatalf("expected domain assurance-vie, got %s", m.Domain)
	}

	if len(m.Parcours) != 1 {
		t.Fatalf("expected 1 parcours, got %d", len(m.Parcours))
	}
	if len(m.Parcours[0].Modules) != 1 {
		t.Fatalf("expected 1 module, got %d", len(m.Parcours[0].Modules))
	}
	if len(m.Parcours[0].Modules[0].Questions) != 2 {
		t.Fatalf("expected 2 questions, got %d", len(m.Parcours[0].Modules[0].Questions))
	}

	// No governance findings for complete manifest.
	for _, f := range result.Findings {
		if f.Code == "corpus_partial" {
			t.Fatalf("unexpected corpus_partial finding: %s", f.Message)
		}
	}
}

func TestExtractParcoursNormalizesReferences(t *testing.T) {
	result, _ := ExtractParcoursFromBytes("test.yaml", []byte(fixtureCompleteParcours))

	// Check references are collected and normalized.
	if len(result.References) == 0 {
		t.Fatal("expected collected references")
	}

	for _, ref := range result.References {
		if !strings.HasPrefix(ref, "rbok") {
			t.Fatalf("expected normalized ref starting with 'rbok', got %q", ref)
		}
	}

	// Check that source_rbok values are normalized in manifest.
	p := result.Manifest.Parcours[0]
	if !strings.HasPrefix(p.SourceRBOK, "rbok") {
		t.Fatalf("expected normalized source_rbok, got %q", p.SourceRBOK)
	}
}

func TestExtractParcoursCollectsAllRefs(t *testing.T) {
	result, _ := ExtractParcoursFromBytes("test.yaml", []byte(fixtureCompleteParcours))

	// 1 parcours x 2 fields + 1 module x 2 fields + 2 questions x 2 fields = 8 unique refs
	expectedRefs := []string{
		"rbok:assurance/vie/base",
		"rbok:contenu/init-vie",
		"rbok:assurance/vie/fondamentaux",
		"rbok:contenu/fondamentaux",
		"rbok:assurance/vie/uc-vs-euro",
		"rbok:contenu/uc-euro",
		"rbok:assurance/vie/frais",
		"rbok:contenu/frais-vie",
	}
	for _, expected := range expectedRefs {
		found := false
		for _, ref := range result.References {
			if ref == expected {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected ref %q in collected references: %v", expected, result.References)
		}
	}
}

func TestExtractParcoursMissingGovernanceFindings(t *testing.T) {
	result, err := ExtractParcoursFromBytes("orphan.yaml", []byte(fixtureMissingGovernance))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	partialCount := 0
	fields := map[string]bool{}
	for _, f := range result.Findings {
		if f.Code == "corpus_partial" {
			partialCount++
			// Extract the field name from message.
			for _, field := range []string{"version", "owner", "status", "domain"} {
				if strings.Contains(f.Message, field) {
					fields[field] = true
				}
			}
		}
	}

	if partialCount != 4 {
		t.Fatalf("expected 4 corpus_partial findings, got %d", partialCount)
	}
	for _, field := range []string{"version", "owner", "status", "domain"} {
		if !fields[field] {
			t.Fatalf("expected finding for missing %s", field)
		}
	}
}

func TestExtractParcoursDetectsUnresolvedRefs(t *testing.T) {
	result, err := ExtractParcoursFromBytes("prev.yaml", []byte(fixtureUnresolvedRefs))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	var unresolved []ExtractionFinding
	for _, f := range result.Findings {
		if f.Code == "corpus_unresolved_ref" {
			unresolved = append(unresolved, f)
		}
	}

	if len(unresolved) == 0 {
		t.Fatal("expected unresolved ref findings for RBOK:prevoyance/hidden-ref")
	}

	found := false
	for _, f := range unresolved {
		if strings.Contains(f.Location, "prevoyance/hidden") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected hidden-ref to be detected as unresolved: %+v", unresolved)
	}
}

func TestValidateReferencesKnownSet(t *testing.T) {
	result, _ := ExtractParcoursFromBytes("test.yaml", []byte(fixtureCompleteParcours))

	// All refs known → no findings.
	knownRefs := make(map[string]bool)
	for _, ref := range result.References {
		knownRefs[ref] = true
	}

	findings := ValidateReferences(result, knownRefs)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings when all refs known, got %d", len(findings))
	}
}

func TestValidateReferencesUnknownSet(t *testing.T) {
	result, _ := ExtractParcoursFromBytes("test.yaml", []byte(fixtureCompleteParcours))

	// Empty known set → all refs are unresolved.
	findings := ValidateReferences(result, map[string]bool{})
	if len(findings) != len(result.References) {
		t.Fatalf("expected %d findings, got %d", len(result.References), len(findings))
	}
	for _, f := range findings {
		if f.Code != "corpus_unresolved_ref" {
			t.Fatalf("expected corpus_unresolved_ref, got %s", f.Code)
		}
		if f.Severity != SeverityError {
			t.Fatalf("expected error severity, got %s", f.Severity)
		}
	}
}

func TestExtractParcoursFromFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "parcours.yaml")
	os.WriteFile(path, []byte(fixtureCompleteParcours), 0o644)

	result, err := ExtractParcours(path)
	if err != nil {
		t.Fatalf("extract from file: %v", err)
	}
	if result.Manifest.Version != "2.1.0" {
		t.Fatalf("expected version 2.1.0, got %s", result.Manifest.Version)
	}
}

func TestExtractParcoursFileNotFound(t *testing.T) {
	_, err := ExtractParcours("/nonexistent-rbok-117.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestExtractParcoursInvalidYAML(t *testing.T) {
	_, err := ExtractParcoursFromBytes("bad.yaml", []byte("{{{{not yaml"))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestNormalizeRef(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"RBOK:assurance/vie", "rbok:assurance/vie"},
		{"  RBOK:path/to/ref  ", "rbok:path/to/ref"},
		{"rbok:already/lower", "rbok:already/lower"},
		{"", ""},
		{"RBOK\\windows\\path", "rbok/windows/path"},
	}
	for _, tc := range cases {
		got := normalizeRef(tc.input)
		if got != tc.expected {
			t.Fatalf("normalizeRef(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestExtractParcoursEmptyParcours(t *testing.T) {
	content := `
version: "1.0.0"
owner: "team"
status: "active"
domain: "test"
parcours: []
`
	result, err := ExtractParcoursFromBytes("empty.yaml", []byte(content))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(result.References) != 0 {
		t.Fatalf("expected 0 references, got %d", len(result.References))
	}
	if len(result.Findings) != 0 {
		t.Fatalf("expected 0 findings for complete governance, got %d", len(result.Findings))
	}
}
