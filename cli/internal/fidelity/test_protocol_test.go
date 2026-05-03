package fidelity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type testEvidenceProtocol struct {
	SchemaVersion string `yaml:"schema_version"`
	DocumentID    string `yaml:"document_id"`
	Title         string `yaml:"title"`
	Status        string `yaml:"status"`
	Purpose       string `yaml:"purpose"`
	Evidence      []struct {
		ID          string   `yaml:"id"`
		Name        string   `yaml:"name"`
		Format      string   `yaml:"format"`
		Required    bool     `yaml:"required"`
		Fields      []string `yaml:"fields"`
		Retention   int      `yaml:"retention_days"`
	} `yaml:"evidence_requirements"`
	Collection struct {
		Steps []struct {
			ID     string `yaml:"id"`
			Action string `yaml:"action"`
			Output string `yaml:"output"`
		} `yaml:"steps"`
	} `yaml:"collection_protocol"`
	Integrity struct {
		HashAlgorithm string `yaml:"hash_algorithm"`
		ChainRequired bool   `yaml:"chain_required"`
	} `yaml:"integrity_controls"`
}

type releaseChecklist struct {
	SchemaVersion string `yaml:"schema_version"`
	DocumentID    string `yaml:"document_id"`
	Title         string `yaml:"title"`
	Checklist     []struct {
		ID       string `yaml:"id"`
		Category string `yaml:"category"`
		Item     string `yaml:"item"`
		Gate     string `yaml:"gate"`
		Blocking bool   `yaml:"blocking"`
		Status   string `yaml:"status"`
	} `yaml:"checklist"`
	Completion struct {
		AllBlocking    bool `yaml:"all_blocking_verified"`
		ApprovalValid  bool `yaml:"approval_chain_valid"`
		EvidenceUploaded bool `yaml:"evidence_pack_uploaded"`
	} `yaml:"completion_criteria"`
}

func TestTestEvidenceProtocolValid(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "regulated", "validation-pack", "test-evidence-protocol.yaml"))
	if err != nil {
		t.Skipf("protocol file not found: %v", err)
	}

	var proto testEvidenceProtocol
	if err := yaml.Unmarshal(data, &proto); err != nil {
		t.Fatalf("parse protocol: %v", err)
	}

	if proto.SchemaVersion != "0.1.0" {
		t.Fatalf("expected schema_version 0.1.0, got %s", proto.SchemaVersion)
	}
	if proto.DocumentID == "" {
		t.Fatal("missing document_id")
	}
	if len(proto.Evidence) < 5 {
		t.Fatalf("expected >= 5 evidence requirements, got %d", len(proto.Evidence))
	}
	for _, evd := range proto.Evidence {
		if !strings.HasPrefix(evd.ID, "EVD-") {
			t.Fatalf("evidence ID should start with EVD-, got %s", evd.ID)
		}
		if len(evd.Fields) == 0 {
			t.Fatalf("evidence %s has no fields", evd.ID)
		}
		if evd.Retention < 1 {
			t.Fatalf("evidence %s has invalid retention: %d", evd.ID, evd.Retention)
		}
	}
	if len(proto.Collection.Steps) < 6 {
		t.Fatalf("expected >= 6 collection steps, got %d", len(proto.Collection.Steps))
	}
	if proto.Integrity.HashAlgorithm != "sha256" {
		t.Fatalf("expected sha256, got %s", proto.Integrity.HashAlgorithm)
	}
	if !proto.Integrity.ChainRequired {
		t.Fatal("expected chain_required true")
	}
}

func TestReleaseChecklistValid(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "regulated", "validation-pack", "release-checklist.yaml"))
	if err != nil {
		t.Skipf("checklist file not found: %v", err)
	}

	var cl releaseChecklist
	if err := yaml.Unmarshal(data, &cl); err != nil {
		t.Fatalf("parse checklist: %v", err)
	}

	if cl.DocumentID == "" {
		t.Fatal("missing document_id")
	}
	if len(cl.Checklist) < 10 {
		t.Fatalf("expected >= 10 checklist items, got %d", len(cl.Checklist))
	}

	categories := map[string]bool{}
	for _, item := range cl.Checklist {
		if !strings.HasPrefix(item.ID, "RC-") {
			t.Fatalf("checklist ID should start with RC-, got %s", item.ID)
		}
		categories[item.Category] = true
		if item.Status != "not_verified" {
			t.Fatalf("item %s should be not_verified in template, got %s", item.ID, item.Status)
		}
	}

	required := []string{"evidence", "fidelity", "integrity", "governance", "approval"}
	for _, cat := range required {
		if !categories[cat] {
			t.Fatalf("missing category %s in checklist", cat)
		}
	}
}

func TestReleaseChecklistBlockingCount(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "regulated", "validation-pack", "release-checklist.yaml"))
	if err != nil {
		t.Skipf("checklist file not found: %v", err)
	}

	var cl releaseChecklist
	yaml.Unmarshal(data, &cl)

	blocking := 0
	for _, item := range cl.Checklist {
		if item.Blocking {
			blocking++
		}
	}
	if blocking < 10 {
		t.Fatalf("expected >= 10 blocking items, got %d", blocking)
	}
}

func TestEvidenceProtocolCoversAllGates(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "regulated", "validation-pack", "test-evidence-protocol.yaml"))
	if err != nil {
		t.Skipf("protocol file not found: %v", err)
	}

	var proto testEvidenceProtocol
	yaml.Unmarshal(data, &proto)

	requiredEvidenceIDs := []string{"EVD-001", "EVD-002", "EVD-003", "EVD-004", "EVD-005", "EVD-006"}
	ids := map[string]bool{}
	for _, evd := range proto.Evidence {
		ids[evd.ID] = true
	}
	for _, required := range requiredEvidenceIDs {
		if !ids[required] {
			t.Fatalf("missing required evidence %s", required)
		}
	}
}
