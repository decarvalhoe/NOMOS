package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RBOKproject/Nomos/cli/internal/corpus"
)

// VRC-23 (#566) — the `export evidence-bom` CLI surface: emits a BOM for a
// Merkle-verified body ledger and fails closed (exit 1) on a tampered hash.

func writeEvidenceLedger(t *testing.T, dir string, tamper bool) string {
	t.Helper()
	content := []byte("# Rule\n\nBody paragraph.\n\n## Sub\n\nSecond paragraph.\n")
	segments, err := corpus.ScanMarkdown("SRC-1", "rule.md", content)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	source := corpus.ManifestSource{ID: "SRC-1", Path: "rule.md", Hash: corpus.ComputeRawTextHash(content), Owner: "o@example.com"}
	adm := source.Admission()
	corpus.BackfillAdmission(&adm, source.Path)
	source.AdmissionStatus = adm.AdmissionStatus
	source.AtomizationStatus = adm.AtomizationStatus
	source.SourceRole = adm.SourceRole
	source.FormatSupport = adm.FormatSupport
	ledger, err := corpus.BuildCorpusBodyLedger(corpus.BodyLedgerInput{
		CorpusRoot: "corpus",
		Sources:    []corpus.BodyLedgerSourceInput{{Source: source, Content: content, Segments: segments, SizeBytes: int64(len(content))}},
	})
	if err != nil {
		t.Fatalf("build ledger: %v", err)
	}
	if tamper {
		ledger.Sources[0].Hash = strings.Repeat("b", 64) // forged after proof generation
	}
	raw, err := corpus.MarshalCorpusBodyLedger(ledger)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path := filepath.Join(dir, "body-ledger.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func TestExportEvidenceBOM_EmitsCycloneDXForVerifiedLedger(t *testing.T) {
	path := writeEvidenceLedger(t, t.TempDir(), false)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"export", "evidence-bom", "--body-ledger", path, "--format", "cyclonedx"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}
	var bom map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &bom); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	if bom["bomFormat"] != "CycloneDX" {
		t.Fatalf("not a CycloneDX BOM: %v", bom["bomFormat"])
	}
}

func TestExportEvidenceBOM_EmitsSPDXForVerifiedLedger(t *testing.T) {
	path := writeEvidenceLedger(t, t.TempDir(), false)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"export", "evidence-bom", "--body-ledger", path, "--format", "spdx"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "SPDX-2.3") {
		t.Fatalf("not an SPDX 2.3 document: %s", stdout.String()[:120])
	}
}

func TestExportEvidenceBOM_FailsClosedOnTamperedLedger(t *testing.T) {
	path := writeEvidenceLedger(t, t.TempDir(), true)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"export", "evidence-bom", "--body-ledger", path}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("a tampered ledger must exit 1, got %d", code)
	}
	if strings.TrimSpace(stdout.String()) != "" {
		t.Fatalf("no BOM may be written on refusal, got: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "merkle verification") {
		t.Fatalf("expected a merkle-verification refusal: %s", stderr.String())
	}
}

func TestExportEvidenceBOM_RequiresLedgerFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"export", "evidence-bom"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("missing --body-ledger must exit 2, got %d", code)
	}
}
