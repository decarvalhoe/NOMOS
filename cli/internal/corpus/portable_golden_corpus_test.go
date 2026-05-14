package corpus

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RBOKproject/Nomos/cli/internal/fidelity"
)

var portableGoldenNow = time.Date(2026, 5, 14, 8, 0, 0, 0, time.UTC)

type portableGoldenFixture struct {
	ID     string
	Domain string
}

type portableLexiconStatus struct {
	Format string `json:"format"`
	Domain string `json:"domain"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type portableFidelityReport struct {
	Format               string            `json:"format"`
	Domain               string            `json:"domain"`
	Status               string            `json:"status"`
	FeedQuality          FeedQualityReport `json:"feed_quality"`
	UncoveredBodyBytes   int64             `json:"uncovered_body_bytes"`
	StrictGatePass       bool              `json:"strict_gate_pass"`
	LicenseSafeFixture   bool              `json:"license_safe_fixture"`
	NoExternalClaimMade  bool              `json:"no_external_claim_made"`
	GeneratedArtifactSet []string          `json:"generated_artifact_set"`
}

func TestPortableMultiDomainGoldenCorpusPack(t *testing.T) {
	fixtures := []portableGoldenFixture{
		{ID: "gxp", Domain: "gxp_csv"},
		{ID: "medical-samd", Domain: "medical_samd"},
		{ID: "ai-governance", Domain: "ai_governance"},
		{ID: "finance-regtech", Domain: "finance_regtech"},
		{ID: "legal-ediscovery", Domain: "legal_ediscovery"},
		{ID: "six-sigma-capa", Domain: "six_sigma_capa"},
		{ID: "provenance", Domain: "provenance"},
		{ID: "cyber-supplier", Domain: "cyber_supplier"},
		{ID: "high-assurance", Domain: "high_assurance"},
	}

	fixtureRoot := filepath.Join("testdata", "portable-golden-corpus")
	outRoot := t.TempDir()
	for _, fixture := range fixtures {
		t.Run(fixture.ID, func(t *testing.T) {
			sourcePath := filepath.Join(fixtureRoot, fixture.ID, "source.md")
			content, err := os.ReadFile(sourcePath)
			if err != nil {
				t.Fatalf("read portable fixture source %s: %v", sourcePath, err)
			}
			outDir := filepath.Join(outRoot, fixture.ID)
			artifacts := buildPortableGoldenArtifacts(t, fixture, sourcePath, content, outDir)
			for _, name := range []string{
				"feed.json",
				"toc.json",
				"no-lexicon.json",
				"body-ledger.json",
				"fidelity-report.json",
				"strict-gate.json",
			} {
				if _, ok := artifacts[name]; !ok {
					t.Fatalf("artifact %s was not emitted; got %v", name, sortedArtifactNames(artifacts))
				}
				assertJSONFileNonEmpty(t, filepath.Join(outDir, name))
			}
		})
	}
}

func buildPortableGoldenArtifacts(
	t *testing.T,
	fixture portableGoldenFixture,
	sourcePath string,
	content []byte,
	outDir string,
) map[string]bool {
	t.Helper()
	sourceHash := sha256Hex(content)
	sourceID := "PORTABLE-" + strings.ToUpper(strings.ReplaceAll(fixture.ID, "-", "_"))
	source := ManifestSource{
		ID:                sourceID,
		Path:              filepath.ToSlash(sourcePath),
		Type:              "markdown",
		Domain:            fixture.Domain,
		Priority:          "primary",
		Status:            "active",
		Hash:              sourceHash,
		Owner:             "NOMOS",
		License:           "license_safe_fixture_authored_for_tests",
		Confidentiality:   "public",
		AllowedUses:       []string{"test_fixture", "corpus_regression"},
		AdmissionStatus:   AdmissionAdmitted,
		AtomizationStatus: AtomizationAtomized,
		SourceRole:        AdmissionRoleCanonical,
		FormatSupport:     FormatSupported,
	}

	segments, err := ScanMarkdown(source.ID, source.Path, content)
	if err != nil {
		t.Fatalf("ScanMarkdown(%s): %v", source.Path, err)
	}
	extracted, err := markdownFeedUnitsFromBytes(content, source, map[string]int{})
	if err != nil {
		t.Fatalf("markdownFeedUnitsFromBytes(%s): %v", source.Path, err)
	}
	if len(extracted) == 0 {
		t.Fatalf("%s produced zero feed units", fixture.ID)
	}

	segByID := make(map[string]SourceSegment, len(segments))
	for _, segment := range segments {
		segByID[segment.SegmentID] = segment
	}

	feedUnits := make([]FeedUnit, 0, len(extracted))
	ragInputs := make([]RAGBuildInput, 0, len(extracted))
	for _, unit := range extracted {
		feedUnits = append(feedUnits, unit.FeedUnit)
		ragInputs = append(ragInputs, RAGBuildInput{
			Unit:       unit.FeedUnit,
			Content:    unit.Content,
			SourceHash: source.Hash,
			Domain:     fixture.Domain,
			Priority:   unit.Priority,
			Status:     unit.SourceStatus,
			Confidence: "medium",
			Locator:    unit.Locator,
		})
	}
	chunks, err := BuildRAGMetadata(ragInputs, segByID, EnrichConfig{Now: portableGoldenNow})
	if err != nil {
		t.Fatalf("BuildRAGMetadata(%s): %v", source.Path, err)
	}

	feedSource := FeedSource{
		ID:                source.ID,
		Path:              source.Path,
		Domain:            source.Domain,
		Type:              source.Type,
		Owner:             source.Owner,
		Confidentiality:   source.Confidentiality,
		Hash:              source.Hash,
		Status:            source.Status,
		AdmissionStatus:   source.AdmissionStatus,
		AtomizationStatus: source.AtomizationStatus,
		SourceRole:        source.SourceRole,
		FormatSupport:     source.FormatSupport,
	}
	if err := feedSource.Validate(); err != nil {
		t.Fatalf("feed source validation: %v", err)
	}
	feed := Feed{
		Format:      FeedFormat,
		GeneratedAt: portableGoldenNow.Format(time.RFC3339),
		UnitCount:   len(feedUnits),
		SourceCount: 1,
		Units:       feedUnits,
		Sources:     []FeedSource{feedSource},
		RAGMetadata: chunks,
	}
	feed.ContentHash = computeFeedHash(feed)

	ledger, err := BuildCorpusBodyLedger(BodyLedgerInput{
		CorpusRoot:  filepath.Dir(sourcePath),
		GeneratedAt: portableGoldenNow.Format(time.RFC3339),
		Sources: []BodyLedgerSourceInput{{
			Source:   source,
			Content:  content,
			Segments: segments,
		}},
	})
	if err != nil {
		t.Fatalf("BuildCorpusBodyLedger(%s): %v", source.Path, err)
	}
	if ledger.CoverageSummary.UncoveredBytes != 0 {
		t.Fatalf("%s has uncovered body bytes: %+v", fixture.ID, ledger.CoverageSummary)
	}

	toc := fidelity.GenerateTOCFromHeadings(
		portableHeadingsFromSegments(content, segments),
		source.ID,
		source.Hash,
		fidelity.DefaultTOCConfig(),
	)
	if !fidelity.VerifyTOCArtifact(toc) {
		t.Fatalf("%s TOC artifact hash did not verify", fixture.ID)
	}

	quality := CheckFeedQuality(FeedQualityInput{
		FeedUnits: feedUnits,
		Chunks:    chunks,
		Segments:  segments,
	})
	if quality.Status != "pass" {
		t.Fatalf("%s feed quality status = %s: %+v", fixture.ID, quality.Status, quality.Findings)
	}

	strict := fidelity.RunStrictFidelityGate(fidelity.StrictGateInput{
		TOC:        &toc,
		DocumentID: source.ID,
		SourceLen:  len(content),
	})
	if !strict.Pass {
		t.Fatalf("%s strict gate failed: %+v", fixture.ID, strict.Findings)
	}
	if !fidelity.VerifyStrictGate(strict) {
		t.Fatalf("%s strict gate hash did not verify", fixture.ID)
	}

	lexicon := portableLexiconStatus{
		Format: "nomos.portable-golden-lexicon-status.v1",
		Domain: fixture.Domain,
		Status: "no_lexicon_required",
		Reason: "Fixture text uses domain prose without standalone governed terms requiring a lexicon artifact.",
	}
	report := portableFidelityReport{
		Format:              "nomos.portable-golden-fidelity-report.v1",
		Domain:              fixture.Domain,
		Status:              "pass",
		FeedQuality:         quality,
		UncoveredBodyBytes:  ledger.CoverageSummary.UncoveredBytes,
		StrictGatePass:      strict.Pass,
		LicenseSafeFixture:  true,
		NoExternalClaimMade: true,
		GeneratedArtifactSet: []string{
			"feed.json",
			"toc.json",
			"no-lexicon.json",
			"body-ledger.json",
			"fidelity-report.json",
			"strict-gate.json",
		},
	}

	artifacts := map[string]any{
		"feed.json":            feed,
		"toc.json":             toc,
		"no-lexicon.json":      lexicon,
		"body-ledger.json":     ledger,
		"fidelity-report.json": report,
		"strict-gate.json":     strict,
	}
	emitted := map[string]bool{}
	for name, artifact := range artifacts {
		writePortableJSON(t, filepath.Join(outDir, name), artifact)
		emitted[name] = true
	}
	return emitted
}

func portableHeadingsFromSegments(content []byte, segments []SourceSegment) []fidelity.HeadingInput {
	headings := make([]fidelity.HeadingInput, 0)
	for _, segment := range segments {
		if segment.Kind != KindHeading || segment.ParentSegmentID != "" {
			continue
		}
		level, title := parseHeadingLevelTitle(string(content[segment.StartByte:segment.EndByte]))
		if level == 0 || strings.TrimSpace(title) == "" {
			continue
		}
		headings = append(headings, fidelity.HeadingInput{
			ID:    segment.SegmentID,
			Title: title,
			Level: level,
			Hash:  segment.NormalizedTextHash,
		})
	}
	return headings
}

func writePortableJSON(t *testing.T, path string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", filepath.Base(path), err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertJSONFileNonEmpty(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read emitted artifact %s: %v", path, err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		t.Fatalf("emitted artifact %s is empty", path)
	}
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("emitted artifact %s is not JSON: %v", path, err)
	}
}

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func sortedArtifactNames(artifacts map[string]bool) []string {
	names := make([]string, 0, len(artifacts))
	for name := range artifacts {
		names = append(names, name)
	}
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j] < names[j-1]; j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}
	return names
}
