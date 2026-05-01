package corpus

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const regressionDir = "testdata/regression"

var regressionTime = time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

// --- minimal corpus: clean pipeline end-to-end ---

func TestRegressionMinimalPipeline(t *testing.T) {
	root := filepath.Join(regressionDir, "minimal")

	// Step 1: Binary policy scan — all files should be text/allow.
	policy := DefaultPolicy()
	results, err := policy.EvaluateDir(root)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	for _, r := range results {
		if r.Class != ClassText {
			t.Fatalf("minimal corpus should be all text, got %q for %s", r.Class, r.Path)
		}
		if r.Action != ActionAllow {
			t.Fatalf("expected allow for %s, got %s", r.Path, r.Action)
		}
	}

	// Step 2: Sidecar validation (patch hash to actual content).
	corpusDir := filepath.Join(root, "docs")
	manifestBytes := readFixture(t, filepath.Join(root, "source-manifest.yaml"))
	actualHash := hashFile(t, filepath.Join(corpusDir, "contract.md"))
	manifestBytes = []byte(strings.Replace(
		string(manifestBytes), "sha256:placeholder", "sha256:"+actualHash, 1,
	))
	manifest, err := ParseSidecarManifestBytes(manifestBytes)
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	sidecarResult := ValidateSidecar(manifest, corpusDir)
	if !sidecarResult.Valid {
		t.Fatalf("sidecar should be valid, got errors: %v", sidecarResult.Errors)
	}

	// Step 3: Feed generation.
	matrixBytes := readFixture(t, filepath.Join(root, "canonical-matrix.yaml"))
	feed, err := GenerateFeed(FeedInput{
		MatrixYAML:   matrixBytes,
		ManifestYAML: manifestBytes,
		GeneratedAt:  regressionTime,
	})
	if err != nil {
		t.Fatalf("generate feed: %v", err)
	}
	assertEqual(t, 1, feed.UnitCount)
	assertEqual(t, 1, feed.SourceCount)
	assertEqual(t, "INS-HOME-WATER", feed.Units[0].UnitID)
	assertEqual(t, "covered", feed.Units[0].Status)
	if !strings.HasPrefix(feed.ContentHash, "sha256:") {
		t.Fatalf("expected content hash, got %q", feed.ContentHash)
	}

	// Feed should round-trip through JSON.
	data, err := MarshalFeed(feed)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty JSON output")
	}
}

// --- with-binaries: binary policy blocks .bin, pipeline still produces feed ---

func TestRegressionWithBinaries(t *testing.T) {
	root := filepath.Join(regressionDir, "with-binaries")

	corpusDir := filepath.Join(root, "docs")

	// Plant a binary file in the corpus.
	binPath := filepath.Join(corpusDir, "firmware.bin")
	binContent := make([]byte, 64)
	binContent[0] = 0x00
	if err := os.WriteFile(binPath, binContent, 0o644); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	t.Cleanup(func() { os.Remove(binPath) })

	// Step 1: Scan should detect the binary.
	policy := DefaultPolicy()
	results, err := policy.EvaluateDir(corpusDir)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	var blocked int
	var textCount int
	for _, r := range results {
		if r.Action == ActionBlock {
			blocked++
		}
		if r.Class == ClassText {
			textCount++
		}
	}
	if blocked == 0 {
		t.Fatal("expected at least 1 blocked binary file")
	}
	if textCount == 0 {
		t.Fatal("expected at least 1 text file")
	}

	// Step 2: Sidecar validation — binary is undeclared → error.
	manifestBytes := readFixture(t, filepath.Join(root, "source-manifest.yaml"))
	actualHash := hashFile(t, filepath.Join(corpusDir, "spec.md"))
	manifestBytes = []byte(strings.Replace(
		string(manifestBytes), "sha256:placeholder", "sha256:"+actualHash, 1,
	))
	manifest, err := ParseSidecarManifestBytes(manifestBytes)
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	sidecarResult := ValidateSidecar(manifest, corpusDir)
	if sidecarResult.Valid {
		t.Fatal("sidecar should fail: undeclared binary file")
	}
	assertHasCode(t, sidecarResult, CodeFileUndeclared)

	// Step 3: Feed still generates (feed doesn't enforce sidecar validity).
	matrixBytes := readFixture(t, filepath.Join(root, "canonical-matrix.yaml"))
	feed, err := GenerateFeed(FeedInput{
		MatrixYAML:   matrixBytes,
		ManifestYAML: manifestBytes,
		GeneratedAt:  regressionTime,
	})
	if err != nil {
		t.Fatalf("feed: %v", err)
	}
	assertEqual(t, 1, feed.UnitCount)
	assertEqual(t, "partial", feed.Units[0].Status)
}

// --- with-conflicts: multiple sidecar errors detected ---

func TestRegressionWithConflicts(t *testing.T) {
	root := filepath.Join(regressionDir, "with-conflicts")
	corpusDir := filepath.Join(root, "docs")

	// Step 1: Scan — all text, no blocks.
	policy := DefaultPolicy()
	results, err := policy.EvaluateDir(corpusDir)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	for _, r := range results {
		if r.Action == ActionBlock {
			t.Fatalf("unexpected block on %s", r.Path)
		}
	}

	// Step 2: Sidecar validation — expect multiple errors.
	manifestBytes := readFixture(t, filepath.Join(root, "source-manifest.yaml"))
	manifest, err := ParseSidecarManifestBytes(manifestBytes)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	sidecarResult := ValidateSidecar(manifest, corpusDir)
	if sidecarResult.Valid {
		t.Fatal("expected invalid sidecar with conflicts")
	}

	codes := errorCodes(sidecarResult)
	// Duplicate ID
	if !codes[CodeIDDuplicate] {
		t.Fatal("expected ID_DUPLICATE")
	}
	// Hash mismatch (wrong hash for alpha.md)
	if !codes[CodeHashMismatch] {
		t.Fatal("expected HASH_MISMATCH")
	}
	// Missing owner on second entry
	if !codes[CodeOwnerMissing] {
		t.Fatal("expected OWNER_MISSING")
	}
	// Invalid confidentiality "top-secret"
	if !codes[CodeConfidentialityBad] {
		t.Fatal("expected CONFIDENTIALITY_INVALID")
	}

	// Step 3: Feed still generates from matrix.
	matrixBytes := readFixture(t, filepath.Join(root, "canonical-matrix.yaml"))
	feed, err := GenerateFeed(FeedInput{
		MatrixYAML:   matrixBytes,
		ManifestYAML: manifestBytes,
		GeneratedAt:  regressionTime,
	})
	if err != nil {
		t.Fatalf("feed: %v", err)
	}
	assertEqual(t, 1, feed.UnitCount)
	assertEqual(t, "missing", feed.Units[0].Status)
}

// --- with-units: multi-unit pipeline with mixed statuses ---

func TestRegressionWithUnits(t *testing.T) {
	root := filepath.Join(regressionDir, "with-units")
	corpusDir := filepath.Join(root, "docs")

	// Step 1: Scan.
	policy := DefaultPolicy()
	results, err := policy.EvaluateDir(corpusDir)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	for _, r := range results {
		if r.Action == ActionBlock {
			t.Fatalf("unexpected block on %s", r.Path)
		}
	}

	// Step 2: Sidecar validation.
	manifestBytes := readFixture(t, filepath.Join(root, "source-manifest.yaml"))
	actualHash := hashFile(t, filepath.Join(corpusDir, "contract.md"))
	manifestBytes = []byte(strings.Replace(
		string(manifestBytes), "sha256:placeholder", "sha256:"+actualHash, 1,
	))
	manifest, err := ParseSidecarManifestBytes(manifestBytes)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	sidecarResult := ValidateSidecar(manifest, corpusDir)
	if !sidecarResult.Valid {
		t.Fatalf("sidecar should be valid, got: %v", sidecarResult.Errors)
	}

	// Step 3: Feed with 3 units.
	matrixBytes := readFixture(t, filepath.Join(root, "canonical-matrix.yaml"))
	feed, err := GenerateFeed(FeedInput{
		MatrixYAML:   matrixBytes,
		ManifestYAML: manifestBytes,
		GeneratedAt:  regressionTime,
	})
	if err != nil {
		t.Fatalf("feed: %v", err)
	}
	assertEqual(t, 3, feed.UnitCount)
	assertEqual(t, 1, feed.SourceCount)

	// Verify unit statuses.
	statusMap := map[string]string{}
	for _, u := range feed.Units {
		statusMap[u.UnitID] = u.Status
	}
	assertEqual(t, "covered", statusMap["INS-HOME-WATER"])
	assertEqual(t, "partial", statusMap["INS-HOME-ROOF"])
	assertEqual(t, "deprecated", statusMap["DEC-DEDUCTIBLE"])

	// Covered unit should have contract ref.
	var waterUnit FeedUnit
	for _, u := range feed.Units {
		if u.UnitID == "INS-HOME-WATER" {
			waterUnit = u
			break
		}
	}
	if waterUnit.Contract == nil {
		t.Fatal("expected contract ref on covered unit")
	}
	assertEqual(t, "INS-HOME-WATER", waterUnit.Contract.ObjectID)

	// Partial unit should have gaps.
	var roofUnit FeedUnit
	for _, u := range feed.Units {
		if u.UnitID == "INS-HOME-ROOF" {
			roofUnit = u
			break
		}
	}
	if len(roofUnit.Gaps) == 0 {
		t.Fatal("expected gaps on partial unit")
	}

	// All units should reference the same source.
	for _, u := range feed.Units {
		if len(u.SourceIDs) == 0 {
			t.Fatalf("unit %s has no source_ids", u.UnitID)
		}
		assertEqual(t, "SRC-FULL-CONTRACT", u.SourceIDs[0])
	}

	// Content hash should be stable.
	feed2, _ := GenerateFeed(FeedInput{
		MatrixYAML:   matrixBytes,
		ManifestYAML: manifestBytes,
		GeneratedAt:  regressionTime,
	})
	assertEqual(t, feed.ContentHash, feed2.ContentHash)
}

// --- helpers ---

func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func hashFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("hash %s: %v", path, err)
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func errorCodes(r SidecarResult) map[string]bool {
	m := map[string]bool{}
	for _, e := range r.Errors {
		m[e.Code] = true
	}
	return m
}
