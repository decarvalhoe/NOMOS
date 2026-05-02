package corpus

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckParcoursGovernanceComplete(t *testing.T) {
	findings, err := CheckParcoursGovernance(filepath.Join("testdata", "parcours-assurance-habitation.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, f := range findings {
		if f.Code == "corpus_partial" {
			t.Fatalf("expected no corpus_partial findings for complete file, got: %s", f.Message)
		}
	}
}

func TestCheckParcoursGovernanceMissingFields(t *testing.T) {
	data := []byte(`
parcours:
  id: no-gov
  name: "No Governance"
  etapes: []
`)
	findings, err := CheckParcoursGovernanceFromBytes(data, "test/no-gov.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fields := map[string]bool{}
	for _, f := range findings {
		if f.Code != "corpus_partial" {
			continue
		}
		if f.Severity != SeverityWarning {
			t.Fatalf("expected warning severity, got %s", f.Severity)
		}
		for _, field := range []string{"version", "owner", "status", "domain"} {
			if strings.Contains(f.Message, field) {
				fields[field] = true
			}
		}
	}

	for _, field := range []string{"version", "owner", "status", "domain"} {
		if !fields[field] {
			t.Fatalf("expected corpus_partial finding for missing %s", field)
		}
	}
}

func TestCheckParcoursGovernancePartialFields(t *testing.T) {
	data := []byte(`
parcours:
  id: partial-gov
  name: "Partial"
  version: "1.0"
  domain: insurance
  etapes: []
`)
	findings, err := CheckParcoursGovernanceFromBytes(data, "test/partial.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// version and domain present → only owner and status missing
	missing := map[string]bool{}
	for _, f := range findings {
		if f.Code == "corpus_partial" {
			for _, field := range []string{"version", "owner", "status", "domain"} {
				if strings.Contains(f.Message, field) {
					missing[field] = true
				}
			}
		}
	}
	if missing["version"] {
		t.Fatal("version is present, should not be reported")
	}
	if missing["domain"] {
		t.Fatal("domain is present, should not be reported")
	}
	if !missing["owner"] {
		t.Fatal("owner is missing, should be reported")
	}
	if !missing["status"] {
		t.Fatal("status is missing, should be reported")
	}
}

func TestCheckParcoursGovernanceFromBytesInvalidYAML(t *testing.T) {
	_, err := CheckParcoursGovernanceFromBytes([]byte(`{broken`), "bad.yaml")
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestCheckParcoursGovernanceFromBytesProductionShape(t *testing.T) {
	data := []byte(`
parcours:
  code: PAR_ACC_ADMIN
  name: Les bases
  version: "2.0"
  owner: compliance@corp.com
  status: active
  domain: admin
  modules:
    - code: MOD1
      name: Module 1
      type: conversational
      objectives: []
`)
	findings, err := CheckParcoursGovernanceFromBytes(data, "prod.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, f := range findings {
		if f.Code == "corpus_partial" {
			t.Fatalf("unexpected corpus_partial for complete production shape: %s", f.Message)
		}
	}
}

func TestCheckParcoursGovernanceFromResultDefaultDomain(t *testing.T) {
	// When domain was empty, ExtractResult shows "rbok" (default fallback).
	result := ExtractResult{
		ParcoursID: "test",
		Domain:     "rbok",
	}
	findings := CheckParcoursGovernanceFromResult(result, "test.yaml")
	hasDomain := false
	for _, f := range findings {
		if strings.Contains(f.Message, "domain") {
			hasDomain = true
		}
	}
	if !hasDomain {
		t.Fatal("expected domain finding when domain is default 'rbok'")
	}
}

func TestCheckParcoursGovernanceFileNotFound(t *testing.T) {
	_, err := CheckParcoursGovernance("/nonexistent-governance.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestCheckParcoursGovernanceFindingPaths(t *testing.T) {
	data := []byte(`
parcours:
  id: path-test
  name: "Path Test"
  etapes: []
`)
	findings, _ := CheckParcoursGovernanceFromBytes(data, "corpus/path-test.yaml")
	for _, f := range findings {
		if f.Path != "corpus/path-test.yaml" {
			t.Fatalf("expected path corpus/path-test.yaml, got %s", f.Path)
		}
		if f.Location != "path-test" {
			t.Fatalf("expected location path-test, got %s", f.Location)
		}
	}
}
