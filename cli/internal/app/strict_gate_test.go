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

func TestStrictGateCorpusIntegrity_ComputeOnTheFlyFailsOnUnsupportedHTML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "unsupported.md"), []byte("<div>unsupported raw html</div>\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--format", "json",
		"--corpus-integrity-source", dir,
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit on unsupported HTML source; got 0; stdout=%q",
			stdout.String())
	}
	var result GateResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
	if result.Valid {
		t.Fatalf("expected valid=false on unsupported HTML source")
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

// FSQ-05 (#368): an --corpus-body-ledger that reports uncovered bytes
// for an admitted+atomized source must fail the integrity check with a
// BODY_LEDGER_UNCOVERED_TEXT_SOURCE finding, even though the on-the-fly
// SFI-04 source-integrity gate sees no SOURCE_UNCOVERED_RANGE finding
// (the ledger evidence trumps the in-memory scan).
func TestStrictGateBodyLedger_UncoveredFails(t *testing.T) {
	dir := t.TempDir()
	cleanDoc := "# Heading\n\nClean paragraph used by the FSQ-05 body-ledger test.\n"
	if err := os.WriteFile(filepath.Join(dir, "doc.md"), []byte(cleanDoc), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	ledger := corpus.CorpusBodyLedger{
		Format:        corpus.BodyLedgerFormat,
		GeneratedAt:   "2026-05-04T00:00:00Z",
		SourceCount:   1,
		AdmittedCount: 1,
		Sources: []corpus.BodyLedgerSource{{
			SourceID:          "doc.md",
			Path:              "doc.md",
			SizeBytes:         100,
			AdmissionStatus:   corpus.AdmissionAdmitted,
			AtomizationStatus: corpus.AtomizationAtomized,
			SourceRole:        corpus.AdmissionRoleCanonical,
			FormatSupport:     corpus.FormatSupported,
			ByteCoverage: corpus.ByteCoverageReport{
				TotalBytes:     100,
				SemanticBytes:  60,
				UncoveredBytes: 40,
			},
		}},
		CoverageSummary: corpus.CoverageSummary{
			TotalBytes:     100,
			SemanticBytes:  60,
			UncoveredBytes: 40,
			BySourceRole:   map[string]int64{corpus.AdmissionRoleCanonical: 100},
			BySourceStatus: map[string]int64{corpus.AdmissionAdmitted: 100},
		},
	}
	ledgerPath := filepath.Join(dir, "body-ledger.json")
	raw, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		t.Fatalf("marshal ledger: %v", err)
	}
	if err := os.WriteFile(ledgerPath, raw, 0o600); err != nil {
		t.Fatalf("write ledger: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--format", "json",
		"--corpus-integrity-source", dir,
		"--corpus-body-ledger", ledgerPath,
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit when body ledger reports uncovered bytes; got 0; stdout=%q",
			stdout.String())
	}
	var result GateResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v\nstdout=%q", err, stdout.String())
	}
	if result.Valid {
		t.Fatalf("expected valid=false")
	}
	if result.CorpusIntegrityCheck == nil ||
		result.CorpusIntegrityCheck.Status != "fail" {
		t.Fatalf("expected corpus_integrity_check.status=fail; got %+v",
			result.CorpusIntegrityCheck)
	}
	if len(result.CorpusIntegrityCheck.BodyLedgerFindings) == 0 {
		t.Fatalf("expected at least one body_ledger_findings entry; got %+v",
			result.CorpusIntegrityCheck)
	}
	got := result.CorpusIntegrityCheck.BodyLedgerFindings[0]
	if got.Code != FindingBodyLedgerUncoveredTextSource {
		t.Fatalf("finding code = %q, want %q", got.Code, FindingBodyLedgerUncoveredTextSource)
	}
	if got.SourceID != "doc.md" {
		t.Fatalf("finding source_id = %q, want %q", got.SourceID, "doc.md")
	}
	if !strings.Contains(result.CorpusIntegrityCheck.Summary, "body_ledger=fail") {
		t.Fatalf("expected summary to mention body_ledger=fail; got %q",
			result.CorpusIntegrityCheck.Summary)
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

// FSQ-06 (#369): the semantic feed quality gate is wired into
// CorpusIntegrityCheck and flips the strict gate to fail on a single
// blocking finding. These three tests cover the failing path, the
// passing path, and the profile-override path.

// fsq06SemanticFixtureFeed builds a feed JSON envelope. units carries the
// FeedUnit slice; if any unit has a non-empty SourceID, that source id is
// also added to the Sources list with admission=admitted+atomized so
// FSQ-06's SOURCE_ZERO_UNIT_NO_REASON does not fire spuriously.
func fsq06SemanticFixtureFeed(units []corpus.FeedUnit) []byte {
	srcSet := map[string]struct{}{}
	for _, u := range units {
		if u.SourceID != "" {
			srcSet[u.SourceID] = struct{}{}
		}
	}
	feed := corpus.Feed{Units: units}
	for sid := range srcSet {
		feed.Sources = append(feed.Sources, corpus.FeedSource{
			ID:                sid,
			Path:              "fixture/" + sid + ".md",
			Status:            "active",
			AdmissionStatus:   corpus.AdmissionAdmitted,
			AtomizationStatus: corpus.AtomizationAtomized,
		})
	}
	data, _ := json.Marshal(feed)
	return data
}

func fsq06SemanticFixtureRAG() []byte {
	// Empty array is a valid RAG file. The semantic gate inspects the
	// feed, not the chunks; what we care about is that --rag is supplied
	// so the FSQ-06 gate runs (it requires both --feed AND --rag).
	return []byte("[]")
}

func fsq06SourceDir(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "doc.md"), []byte(content), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return dir
}

func TestStrictGateSemanticQuality_FailsOnBlocking(t *testing.T) {
	dir := fsq06SourceDir(t, "# Heading\n\nClean source paragraph used by the FSQ-06 wiring test.\n")
	feed := []corpus.FeedUnit{
		// One-token paragraph — trips FEED_UNIT_BELOW_TOKEN_MIN (blocking).
		{
			UnitID:             "U-NOISY",
			Domain:             "rbok",
			UnitType:           "rule",
			BusinessRule:       "x",
			SourceIDs:          []string{"doc.md"},
			SourceSegmentID:    "seg:doc.md:10-11:paragraph",
			SourceID:           "doc.md",
			SourcePath:         "doc.md",
			StartByte:          10,
			EndByte:            11,
			StartLine:          3,
			EndLine:            3,
			NormalizedTextHash: "norm-noisy",
		},
	}
	feedPath := filepath.Join(t.TempDir(), "feed.json")
	if err := os.WriteFile(feedPath, fsq06SemanticFixtureFeed(feed), 0o600); err != nil {
		t.Fatalf("write feed: %v", err)
	}
	ragPath := filepath.Join(t.TempDir(), "rag.json")
	if err := os.WriteFile(ragPath, fsq06SemanticFixtureRAG(), 0o600); err != nil {
		t.Fatalf("write rag: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--format", "json",
		"--corpus-integrity-source", dir,
		"--corpus-integrity-feed", feedPath,
		"--corpus-integrity-rag", ragPath,
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1 on noisy feed; got %d; stdout=%q stderr=%q",
			code, stdout.String(), stderr.String())
	}
	var result GateResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected JSON: %v", err)
	}
	if result.CorpusIntegrityCheck == nil ||
		result.CorpusIntegrityCheck.SemanticQuality == nil {
		t.Fatalf("expected semantic_quality to be populated; got %+v", result.CorpusIntegrityCheck)
	}
	if result.CorpusIntegrityCheck.SemanticQuality.Status != "fail" {
		t.Fatalf("expected semantic_quality.status=fail; got %q",
			result.CorpusIntegrityCheck.SemanticQuality.Status)
	}
	if result.CorpusIntegrityCheck.Status != "fail" {
		t.Fatalf("expected corpus_integrity_check.status=fail; got %q",
			result.CorpusIntegrityCheck.Status)
	}
	var sawAggregate bool
	for _, f := range result.CorpusIntegrityCheck.AggregateFindings {
		if f.Code == FindingSemanticQualityFailed {
			sawAggregate = true
		}
	}
	if !sawAggregate {
		t.Fatalf("expected SEMANTIC_QUALITY_FAILED in aggregate_findings; got %+v",
			result.CorpusIntegrityCheck.AggregateFindings)
	}
}

func TestStrictGateSemanticQuality_PassWithCleanFixture(t *testing.T) {
	dir := fsq06SourceDir(t, "# Heading\n\nClean and meaningful paragraph used by the FSQ-06 pass test.\n")
	feed := []corpus.FeedUnit{
		{
			UnitID:             "U-OK",
			Domain:             "rbok",
			UnitType:           "rule",
			BusinessRule:       "Clean and meaningful paragraph used by the FSQ-06 pass test.",
			SourceIDs:          []string{"doc.md"},
			SourceSegmentID:    "seg:doc.md:11-72:paragraph",
			SourceID:           "doc.md",
			SourcePath:         "doc.md",
			StartByte:          11,
			EndByte:            72,
			StartLine:          3,
			EndLine:            3,
			NormalizedTextHash: "norm-ok",
		},
	}
	feedPath := filepath.Join(t.TempDir(), "feed.json")
	if err := os.WriteFile(feedPath, fsq06SemanticFixtureFeed(feed), 0o600); err != nil {
		t.Fatalf("write feed: %v", err)
	}
	ragPath := filepath.Join(t.TempDir(), "rag.json")
	if err := os.WriteFile(ragPath, fsq06SemanticFixtureRAG(), 0o600); err != nil {
		t.Fatalf("write rag: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--format", "json",
		"--corpus-integrity-source", dir,
		"--corpus-integrity-feed", feedPath,
		"--corpus-integrity-rag", ragPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0 on clean feed; got %d; stdout=%q stderr=%q",
			code, stdout.String(), stderr.String())
	}
	var result GateResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected JSON: %v", err)
	}
	if result.CorpusIntegrityCheck == nil ||
		result.CorpusIntegrityCheck.SemanticQuality == nil {
		t.Fatalf("expected semantic_quality to be populated; got %+v", result.CorpusIntegrityCheck)
	}
	if result.CorpusIntegrityCheck.SemanticQuality.Status != "pass" {
		t.Fatalf("expected semantic_quality.status=pass; got %q (%+v)",
			result.CorpusIntegrityCheck.SemanticQuality.Status,
			result.CorpusIntegrityCheck.SemanticQuality.Findings)
	}
}

func TestStrictGateSemanticQuality_ProfileOverride(t *testing.T) {
	dir := fsq06SourceDir(t, "# Heading\n\nClean paragraph for FSQ-06 profile-override test.\n")
	// A bare table_cell unit (no TableID, no ColumnHeaders) would normally
	// trip FEED_TABLE_CELL_NOT_ROW_CONTEXT under DefaultRBOKProfile.
	feed := []corpus.FeedUnit{
		{
			UnitID:             "U-CELL",
			Domain:             "rbok",
			UnitType:           "rule",
			BusinessRule:       "Free-form cell content that would otherwise pass the text rules.",
			SourceIDs:          []string{"doc.md"},
			SourceSegmentID:    "seg:doc.md:11-80:table_cell",
			SourceID:           "doc.md",
			SourcePath:         "doc.md",
			StartByte:          11,
			EndByte:            80,
			StartLine:          3,
			EndLine:            3,
			NormalizedTextHash: "norm-cell",
		},
	}
	feedPath := filepath.Join(t.TempDir(), "feed.json")
	if err := os.WriteFile(feedPath, fsq06SemanticFixtureFeed(feed), 0o600); err != nil {
		t.Fatalf("write feed: %v", err)
	}
	ragPath := filepath.Join(t.TempDir(), "rag.json")
	if err := os.WriteFile(ragPath, fsq06SemanticFixtureRAG(), 0o600); err != nil {
		t.Fatalf("write rag: %v", err)
	}

	// Permissive profile: AllowTableCellWithoutRow=true.
	profile := corpus.DefaultRBOKProfile()
	profile.AllowTableCellWithoutRow = true
	profilePath := filepath.Join(t.TempDir(), "profile.json")
	profileBytes, _ := json.Marshal(profile)
	if err := os.WriteFile(profilePath, profileBytes, 0o600); err != nil {
		t.Fatalf("write profile: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--format", "json",
		"--corpus-integrity-source", dir,
		"--corpus-integrity-feed", feedPath,
		"--corpus-integrity-rag", ragPath,
		"--corpus-semantic-quality-profile", profilePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0 with permissive profile; got %d; stdout=%q stderr=%q",
			code, stdout.String(), stderr.String())
	}
	var result GateResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected JSON: %v", err)
	}
	sq := result.CorpusIntegrityCheck.SemanticQuality
	if sq == nil {
		t.Fatal("expected semantic_quality to be populated")
	}
	for _, f := range sq.Findings {
		if f.Code == corpus.FindingFeedTableCellNotRowContext {
			t.Fatalf("expected NO %s under permissive profile; got %+v",
				corpus.FindingFeedTableCellNotRowContext, sq.Findings)
		}
	}
	if sq.Status != "pass" {
		t.Fatalf("expected semantic_quality.status=pass; got %q (%+v)", sq.Status, sq.Findings)
	}
}
