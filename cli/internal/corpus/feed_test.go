package corpus

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

var testMatrix = []byte(`
schema_version: "0.1.0"
units:
  - unit_id: INS-HOME-WATER
    unit_type: catalog_entry
    name: Water damage warranty
    domain: insurance-home
    criticality: high
    status: covered
    business_rule: Water damage is covered.
    source_refs:
      - source_id: SRC-CONTRACT-2026
    canonical_contract:
      path: data/warranties.yaml
      object_id: INS-HOME-WATER
      status: present
    test_refs:
      - tests/water_test.go
    gaps: []
  - unit_id: INS-HOME-ROOF
    unit_type: exception
    name: Roof infiltration
    domain: insurance-home
    criticality: medium
    status: partial
    business_rule: Roof excluded without maintenance.
    source_refs:
      - source_id: SRC-CONTRACT-2026
    gaps:
      - Missing exclusion payload.
`)

var testManifest = []byte(`
schema_version: "0.1.0"
sources:
  - id: SRC-CONTRACT-2026
    path: docs/contract-2026.pdf
    type: pdf
    domain: insurance-home
    priority: primary
    status: active
    hash: "sha256:aabbccdd"
    owner: Alice
    license: proprietary
    confidentiality: restricted
    allowed_uses:
      - structured_contract
`)

var fixedTime = time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

func TestGenerateFeedBasic(t *testing.T) {
	feed, err := GenerateFeed(FeedInput{
		MatrixYAML:   testMatrix,
		ManifestYAML: testManifest,
		GeneratedAt:  fixedTime,
	})
	assertNoErr(t, err)

	assertEqual(t, FeedFormat, feed.Format)
	assertEqual(t, 2, feed.UnitCount)
	assertEqual(t, 1, feed.SourceCount)
	assertEqual(t, "2026-05-01T12:00:00Z", feed.GeneratedAt)
}

func TestFeedUnitsContent(t *testing.T) {
	feed, err := GenerateFeed(FeedInput{
		MatrixYAML:   testMatrix,
		ManifestYAML: testManifest,
		GeneratedAt:  fixedTime,
	})
	assertNoErr(t, err)

	u0 := feed.Units[0]
	assertEqual(t, "INS-HOME-WATER", u0.UnitID)
	assertEqual(t, "catalog_entry", u0.UnitType)
	assertEqual(t, "covered", u0.Status)
	assertEqual(t, "high", u0.Criticality)
	if len(u0.SourceIDs) != 1 || u0.SourceIDs[0] != "SRC-CONTRACT-2026" {
		t.Fatalf("expected source_ids [SRC-CONTRACT-2026], got %v", u0.SourceIDs)
	}
	if u0.Contract == nil {
		t.Fatal("expected contract ref")
	}
	assertEqual(t, "INS-HOME-WATER", u0.Contract.ObjectID)
	if len(u0.TestRefs) != 1 {
		t.Fatalf("expected 1 test_ref, got %d", len(u0.TestRefs))
	}

	u1 := feed.Units[1]
	assertEqual(t, "INS-HOME-ROOF", u1.UnitID)
	assertEqual(t, "partial", u1.Status)
	if u1.Contract != nil {
		t.Fatal("expected no contract ref for partial unit")
	}
	if len(u1.Gaps) != 1 {
		t.Fatalf("expected 1 gap, got %d", len(u1.Gaps))
	}
}

func TestFeedSourcesContent(t *testing.T) {
	feed, err := GenerateFeed(FeedInput{
		MatrixYAML:   testMatrix,
		ManifestYAML: testManifest,
		GeneratedAt:  fixedTime,
	})
	assertNoErr(t, err)

	s := feed.Sources[0]
	assertEqual(t, "SRC-CONTRACT-2026", s.ID)
	assertEqual(t, "docs/contract-2026.pdf", s.Path)
	assertEqual(t, "restricted", s.Confidentiality)
	assertEqual(t, "Alice", s.Owner)
	assertEqual(t, "sha256:aabbccdd", s.Hash)
}

func TestFeedContentHash(t *testing.T) {
	feed, err := GenerateFeed(FeedInput{
		MatrixYAML:   testMatrix,
		ManifestYAML: testManifest,
		GeneratedAt:  fixedTime,
	})
	assertNoErr(t, err)

	if !strings.HasPrefix(feed.ContentHash, "sha256:") {
		t.Fatalf("expected sha256: prefix, got %q", feed.ContentHash)
	}
	if len(feed.ContentHash) != 71 { // "sha256:" + 64 hex chars
		t.Fatalf("unexpected content hash length: %d", len(feed.ContentHash))
	}
}

func TestFeedContentHashDeterministic(t *testing.T) {
	input := FeedInput{
		MatrixYAML:   testMatrix,
		ManifestYAML: testManifest,
		GeneratedAt:  fixedTime,
	}
	f1, _ := GenerateFeed(input)
	f2, _ := GenerateFeed(input)
	assertEqual(t, f1.ContentHash, f2.ContentHash)
}

func TestFeedContentHashChangesOnInput(t *testing.T) {
	f1, _ := GenerateFeed(FeedInput{
		MatrixYAML:   testMatrix,
		ManifestYAML: testManifest,
		GeneratedAt:  fixedTime,
	})
	// Different timestamp → different hash.
	f2, _ := GenerateFeed(FeedInput{
		MatrixYAML:   testMatrix,
		ManifestYAML: testManifest,
		GeneratedAt:  fixedTime.Add(time.Hour),
	})
	if f1.ContentHash == f2.ContentHash {
		t.Fatal("expected different hashes for different inputs")
	}
}

func TestMarshalFeedJSON(t *testing.T) {
	feed, err := GenerateFeed(FeedInput{
		MatrixYAML:   testMatrix,
		ManifestYAML: testManifest,
		GeneratedAt:  fixedTime,
	})
	assertNoErr(t, err)

	data, err := MarshalFeed(feed)
	assertNoErr(t, err)

	var decoded Feed
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	assertEqual(t, feed.Format, decoded.Format)
	assertEqual(t, feed.UnitCount, decoded.UnitCount)
	assertEqual(t, feed.ContentHash, decoded.ContentHash)
}

func TestGenerateFeedInvalidMatrix(t *testing.T) {
	_, err := GenerateFeed(FeedInput{
		MatrixYAML:   []byte("not: [valid: yaml"),
		ManifestYAML: testManifest,
	})
	if err == nil {
		t.Fatal("expected error for invalid matrix YAML")
	}
}

func TestGenerateFeedInvalidManifest(t *testing.T) {
	_, err := GenerateFeed(FeedInput{
		MatrixYAML:   testMatrix,
		ManifestYAML: []byte("not: [valid: yaml"),
	})
	if err == nil {
		t.Fatal("expected error for invalid manifest YAML")
	}
}

func TestGenerateFeedEmptyUnits(t *testing.T) {
	emptyMatrix := []byte(`
schema_version: "0.1.0"
units: []
`)
	feed, err := GenerateFeed(FeedInput{
		MatrixYAML:   emptyMatrix,
		ManifestYAML: testManifest,
		GeneratedAt:  fixedTime,
	})
	assertNoErr(t, err)
	assertEqual(t, 0, feed.UnitCount)
	assertEqual(t, 1, feed.SourceCount)
}

func TestGenerateFeedDefaultTimestamp(t *testing.T) {
	feed, err := GenerateFeed(FeedInput{
		MatrixYAML:   testMatrix,
		ManifestYAML: testManifest,
	})
	assertNoErr(t, err)
	if feed.GeneratedAt == "" {
		t.Fatal("expected non-empty generated_at")
	}
}

func TestFeedUnitNoSourceRefs(t *testing.T) {
	noRefs := []byte(`
schema_version: "0.1.0"
units:
  - unit_id: ORPHAN-UNIT
    unit_type: rule
    name: Orphan
    domain: test
    criticality: low
    status: missing
    business_rule: No source refs.
    gaps:
      - No sources linked.
`)
	feed, err := GenerateFeed(FeedInput{
		MatrixYAML:   noRefs,
		ManifestYAML: testManifest,
		GeneratedAt:  fixedTime,
	})
	assertNoErr(t, err)
	if len(feed.Units[0].SourceIDs) != 0 {
		t.Fatalf("expected 0 source_ids, got %v", feed.Units[0].SourceIDs)
	}
}
