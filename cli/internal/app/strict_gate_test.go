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

func TestStrictGateAllGreen(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--project", "testdata/gate-project.yaml",
		"--sources", "testdata/gate-sources.yaml",
		"--matrix", "testdata/gate-matrix.yaml",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "PASS") {
		t.Fatalf("expected PASS, got %q", stdout.String())
	}
}

func TestStrictGateAllGreenWithExceptions(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--project", "testdata/gate-project.yaml",
		"--sources", "testdata/gate-sources.yaml",
		"--matrix", "testdata/gate-matrix.yaml",
		"--exceptions", "testdata/gate-exceptions.yaml",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "PASS") {
		t.Fatalf("expected PASS, got %q", stdout.String())
	}
}

func TestStrictGateSourcesInvalid(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--sources", "../checks/testdata/missing-owner.yaml",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), "FAIL") {
		t.Fatalf("expected FAIL, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "NO_OWNER") {
		t.Fatalf("expected NO_OWNER, got %q", stdout.String())
	}
}

func TestStrictGateCrossCheckDangling(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--sources", "testdata/gate-sources.yaml",
		"--matrix", "../strict/testdata/matrix-dangling-ref.yaml",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), "DANGLING_SOURCE_REF") {
		t.Fatalf("expected DANGLING_SOURCE_REF, got %q", stdout.String())
	}
}

func TestStrictGateProductInvalid(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--project", "../productcheck/testdata/no-owners.yaml",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), "NO_OWNERS") {
		t.Fatalf("expected NO_OWNERS, got %q", stdout.String())
	}
}

func TestStrictGateExceptionsInvalid(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--sources", "testdata/gate-sources.yaml",
		"--exceptions", "../exceptions/testdata/no-owner.yaml",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), "NO_OWNER") {
		t.Fatalf("expected NO_OWNER, got %q", stdout.String())
	}
}

func TestStrictGateJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--format", "json",
		"--project", "testdata/gate-project.yaml",
		"--sources", "testdata/gate-sources.yaml",
		"--matrix", "testdata/gate-matrix.yaml",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	var result GateResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v\n%s", err, stdout.String())
	}
	if !result.Valid {
		t.Fatalf("expected valid=true")
	}
	if len(result.Sections) == 0 {
		t.Fatal("expected sections")
	}
}

func TestStrictGateJSONFailed(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--format", "json",
		"--sources", "../checks/testdata/missing-owner.yaml",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected 1, got %d", code)
	}
	var result GateResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
	if result.Valid {
		t.Fatal("expected valid=false")
	}
}

func TestStrictGateNoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := StrictGateCommand(nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected 2, got %d", code)
	}
}

func TestStrictGateMissingFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--project", "/nonexistent.yaml",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected 1, got %d", code)
	}
}

func TestStrictGateSectionsPresent(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--format", "json",
		"--project", "testdata/gate-project.yaml",
		"--sources", "testdata/gate-sources.yaml",
		"--matrix", "testdata/gate-matrix.yaml",
		"--exceptions", "testdata/gate-exceptions.yaml",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stdout=%q", code, stdout.String())
	}
	var result GateResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}

	names := make(map[string]bool)
	for _, s := range result.Sections {
		names[s.Name] = true
	}
	for _, expected := range []string{"product-check", "sources-check", "matrix-check", "contracts-check", "cross-check", "exceptions-check"} {
		if !names[expected] {
			t.Fatalf("expected section %q, got %v", expected, names)
		}
	}
}

func TestStrictGateProjectOnlyRuns(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--format", "json",
		"--project", "testdata/gate-project.yaml",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stdout=%q", code, stdout.String())
	}
	var result GateResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
	if len(result.Sections) != 2 {
		t.Fatalf("expected 2 sections for project-only, got %d", len(result.Sections))
	}
}

// SFI-08 (#346) — corpus integrity wiring tests.

// writeIntegrityJSON marshals v to a file under t.TempDir() and returns the path.
func writeIntegrityJSON(t *testing.T, name string, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %s: %v", name, err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestStrictGateCorpusIntegrity_NoFlagsOmitsSection(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--format", "json",
		"--project", "testdata/gate-project.yaml",
		"--sources", "testdata/gate-sources.yaml",
		"--matrix", "testdata/gate-matrix.yaml",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
	if _, present := raw["corpus_integrity_check"]; present {
		t.Fatalf("expected corpus_integrity_check to be omitted when no integrity flags are passed; got: %s", stdout.String())
	}
}

func TestStrictGateCorpusIntegrity_PrecomputedFailingReport(t *testing.T) {
	failing := corpus.IntegrityReport{
		Status:                      "fail",
		SourceCount:                 1,
		SegmentCount:                1,
		SemanticSegmentCount:        1,
		UncoveredRanges:             []corpus.ByteRange{},
		DuplicateSemanticRanges:     []corpus.ByteRange{},
		JunkSemanticSegments:        []string{"seg:bad"},
		UnsupportedBlockingSegments: []string{},
		Findings: []corpus.IntegrityFinding{
			{
				Code:      corpus.FindingSourceJunkSemantic,
				SegmentID: "seg:bad",
				Message:   "synthetic junk finding",
			},
		},
	}
	path := writeIntegrityJSON(t, "report.json", failing)

	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--format", "json",
		"--project", "testdata/gate-project.yaml",
		"--corpus-integrity-report", path,
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1 on failing integrity report; got %d; stdout=%q",
			code, stdout.String())
	}
	var result GateResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v\n%s", err, stdout.String())
	}
	if result.Valid {
		t.Fatalf("expected valid=false")
	}
	if result.CorpusIntegrityCheck == nil {
		t.Fatalf("expected corpus_integrity_check to be present in JSON")
	}
	if result.CorpusIntegrityCheck.Status != "fail" {
		t.Fatalf("expected corpus integrity status=fail; got %q",
			result.CorpusIntegrityCheck.Status)
	}
	if result.CorpusIntegrityCheck.SourceIntegrity == nil {
		t.Fatalf("expected source_integrity to be parsed from single-shape JSON")
	}
	if result.CorpusIntegrityCheck.FeedQuality != nil {
		t.Fatalf("expected feed_quality to be nil when only source-integrity report supplied")
	}
}

func TestStrictGateCorpusIntegrity_PrecomputedPassingAggregate(t *testing.T) {
	passingSource := corpus.IntegrityReport{
		Status:                      "pass",
		SourceCount:                 1,
		SegmentCount:                3,
		SemanticSegmentCount:        2,
		UncoveredRanges:             []corpus.ByteRange{},
		DuplicateSemanticRanges:     []corpus.ByteRange{},
		JunkSemanticSegments:        []string{},
		UnsupportedBlockingSegments: []string{},
		Findings:                    []corpus.IntegrityFinding{},
	}
	passingFeed := corpus.FeedQualityReport{
		Status:                 "pass",
		FeedUnitCount:          0,
		SourceDerivedUnitCount: 0,
		ChunkCount:             0,
		DuplicateSpanCount:     0,
		Findings:               []corpus.FeedQualityFinding{},
	}
	agg := map[string]any{
		"source_integrity": passingSource,
		"feed_quality":     passingFeed,
	}
	path := writeIntegrityJSON(t, "agg.json", agg)

	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--format", "json",
		"--project", "testdata/gate-project.yaml",
		"--corpus-integrity-report", path,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0 on passing aggregate; got %d; stdout=%q stderr=%q",
			code, stdout.String(), stderr.String())
	}
	var result GateResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid=true on passing aggregate")
	}
	if result.CorpusIntegrityCheck == nil {
		t.Fatalf("expected corpus_integrity_check to be present")
	}
	if result.CorpusIntegrityCheck.Status != "pass" {
		t.Fatalf("expected status=pass; got %q", result.CorpusIntegrityCheck.Status)
	}
	if result.CorpusIntegrityCheck.SourceIntegrity == nil ||
		result.CorpusIntegrityCheck.FeedQuality == nil {
		t.Fatalf("expected both sub-reports parsed from aggregate")
	}
}

func TestStrictGateCorpusIntegrity_ComputeOnTheFlyPasses(t *testing.T) {
	dir := t.TempDir()
	doc := strings.Join([]string{
		"# Heading",
		"",
		"This is a clean paragraph used by the SFI-08 compute-on-the-fly test.",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(dir, "doc.md"), []byte(doc), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--format", "json",
		"--corpus-integrity-source", dir,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0 on clean source dir; got %d; stdout=%q stderr=%q",
			code, stdout.String(), stderr.String())
	}
	var result GateResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
	if result.CorpusIntegrityCheck == nil {
		t.Fatalf("expected corpus_integrity_check to be present")
	}
	if result.CorpusIntegrityCheck.Status != "pass" {
		t.Fatalf("expected status=pass; got %q\nfindings=%+v",
			result.CorpusIntegrityCheck.Status,
			result.CorpusIntegrityCheck.SourceIntegrity)
	}
	if result.CorpusIntegrityCheck.SourceIntegrity == nil {
		t.Fatalf("expected source_integrity report to be populated")
	}
	if result.CorpusIntegrityCheck.SourceIntegrity.SemanticSegmentCount == 0 {
		t.Fatalf("expected at least one semantic segment from one-paragraph fixture")
	}
}

func TestStrictGateCorpusIntegrity_ComputeOnTheFlyFailsOnJunk(t *testing.T) {
	dir := t.TempDir()
	// A line of pipes is not a heading, list, blockquote, or decorative
	// separator (decorative-separator runes are *-_.~+ \t — pipes excluded),
	// so the typed scanner emits it as a canonical_atom paragraph; the SFI-04
	// junk rule then flags it because every non-whitespace rune is in the
	// punctuation/layout set.
	if err := os.WriteFile(filepath.Join(dir, "junk.md"), []byte("||||\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--format", "json",
		"--corpus-integrity-source", dir,
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit on junk-only source; got 0; stdout=%q",
			stdout.String())
	}
	var result GateResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
	if result.Valid {
		t.Fatalf("expected valid=false on junk-only source")
	}
	if result.CorpusIntegrityCheck == nil ||
		result.CorpusIntegrityCheck.Status != "fail" {
		t.Fatalf("expected corpus_integrity_check.status=fail; got %+v",
			result.CorpusIntegrityCheck)
	}
}

func TestStrictGateCorpusIntegrity_OnlyIntegrityFlagIsAllowed(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ok.md"), []byte("# Title\n\nClean body paragraph here.\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--format", "json",
		"--corpus-integrity-source", dir,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0 with only --corpus-integrity-source supplied; got %d; stderr=%q stdout=%q",
			code, stderr.String(), stdout.String())
	}
}

func TestStrictGateCorpusIntegrity_BadReportFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.json")
	if err := os.WriteFile(path, []byte("not-valid-json"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--format", "json",
		"--project", "testdata/gate-project.yaml",
		"--corpus-integrity-report", path,
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1 on unparseable report; got %d", code)
	}
	var result GateResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
	if result.CorpusIntegrityCheck == nil ||
		result.CorpusIntegrityCheck.Status != "fail" {
		t.Fatalf("expected corpus_integrity_check.status=fail on parse error; got %+v",
			result.CorpusIntegrityCheck)
	}
	if !strings.Contains(result.CorpusIntegrityCheck.Summary, "load failed") {
		t.Fatalf("expected summary to mention load failure; got %q",
			result.CorpusIntegrityCheck.Summary)
	}
}
