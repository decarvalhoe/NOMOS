package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The `nomos rag` CLI surface: export a governed feed into records any RAG
// stack can index, and fingerprint what was handed out.

const feedFixture = `{
  "format": "nomos-corpus-feed",
  "generated_at": "2026-06-01T00:00:00Z",
  "content_hash": "sha256:feed",
  "unit_count": 2,
  "source_count": 1,
  "rag_metadata": [
    {
      "chunk_id": "chunk:S1:100-220",
      "source_id": "S1",
      "source_path": "corpus/rulebook.md",
      "source_hash": "sha256:aaaa",
      "domain": "aec",
      "locator": "corpus/rulebook.md:L10-L14",
      "priority": "primary",
      "status": "active",
      "confidence": "high",
      "normalized_text_hash": "sha256:bbbb",
      "source_segment_ids": ["seg-1"],
      "context_heading_path": ["Titre I", "Chapitre 3"],
      "context_source_role": "reference",
      "chunk_text": "Titre I/Chapitre 3\n\nLe gabarit retient une hauteur de neuf metres."
    },
    {
      "chunk_id": "chunk:S1:300-380",
      "source_id": "S1",
      "source_hash": "sha256:aaaa",
      "domain": "aec",
      "context_heading_path": ["Titre I", "Chapitre 4"],
      "chunk_text": "Titre I/Chapitre 4\n\nLe recul minimal est de cinq metres."
    }
  ]
}`

// unciteableFeedFixture carries a chunk with no source_hash: its freshness
// could never be proved, so it must not reach an index.
const unciteableFeedFixture = `{
  "format": "nomos-corpus-feed",
  "generated_at": "2026-06-01T00:00:00Z",
  "content_hash": "sha256:feed",
  "rag_metadata": [
    {
      "chunk_id": "chunk:S1:100-220",
      "source_id": "S1",
      "source_hash": "sha256:aaaa",
      "context_heading_path": ["Titre I"],
      "chunk_text": "Titre I\n\nUn alinea correctement source."
    },
    {
      "chunk_id": "chunk:S1:900-999",
      "source_id": "S1",
      "context_heading_path": ["Titre I"],
      "chunk_text": "Titre I\n\nUn alinea sans hash de source."
    }
  ]
}`

func writeFeed(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "feed.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write feed: %v", err)
	}
	return path
}

func runRAG(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(append([]string{"rag"}, args...), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestRAGExport_EmitsOneRecordPerLine(t *testing.T) {
	feed := writeFeed(t, feedFixture)
	code, stdout, stderr := runRAG(t, "export", "--feed", feed)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stderr: %s)", code, stderr)
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 records, got %d", len(lines))
	}
	var rec struct {
		ChunkID       string `json:"chunk_id"`
		EmbeddingText string `json:"embedding_text"`
		BodyText      string `json:"body_text"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("decode record: %v", err)
	}
	if rec.ChunkID != "chunk:S1:100-220" {
		t.Fatalf("records must be ordered by chunk id, got %q first", rec.ChunkID)
	}
	if !strings.Contains(rec.EmbeddingText, "Titre I › Chapitre 3") {
		t.Fatalf("embedding text lost its structural context: %q", rec.EmbeddingText)
	}
	if strings.Contains(rec.BodyText, "Titre I") {
		t.Fatalf("citable body must not carry the context prefix: %q", rec.BodyText)
	}
}

// Rejections are reported on stderr even when the export succeeds: a silently
// dropped chunk is a retrieval gap nobody would notice.
func TestRAGExport_ReportsRejectionsAndStrictFailsClosed(t *testing.T) {
	feed := writeFeed(t, unciteableFeedFixture)

	code, stdout, stderr := runRAG(t, "export", "--feed", feed)
	if code != 0 {
		t.Fatalf("without --strict the export continues, got exit %d", code)
	}
	if !strings.Contains(stderr, "missing_source_hash") {
		t.Fatalf("rejection was not reported: %s", stderr)
	}
	if strings.Contains(stdout, "chunk:S1:900-999") {
		t.Fatal("an unciteable chunk reached the index")
	}

	strictCode, _, _ := runRAG(t, "export", "--feed", feed, "--strict")
	if strictCode != 1 {
		t.Fatalf("--strict must fail closed on rejection, got exit %d", strictCode)
	}
}

func TestRAGExport_IsDeterministicAcrossRuns(t *testing.T) {
	feed := writeFeed(t, feedFixture)
	_, first, _ := runRAG(t, "export", "--feed", feed)
	_, second, _ := runRAG(t, "export", "--feed", feed)
	if first != second {
		t.Fatal("two exports of the same feed differ: a CI gate could not diff them")
	}
}

func TestRAGExport_RejectsUnknownFormat(t *testing.T) {
	feed := writeFeed(t, feedFixture)
	code, _, stderr := runRAG(t, "export", "--feed", feed, "--format", "faiss")
	if code != 2 {
		t.Fatalf("an unknown format must be a usage error, got exit %d", code)
	}
	if !strings.Contains(stderr, "unknown format") {
		t.Fatalf("unexpected stderr: %s", stderr)
	}
}

func TestRAGExport_WritesToOutputFile(t *testing.T) {
	feed := writeFeed(t, feedFixture)
	out := filepath.Join(t.TempDir(), "chunks.jsonl")
	code, stdout, stderr := runRAG(t, "export", "--feed", feed, "--output", out)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stderr: %s)", code, stderr)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Fatal("with --output nothing must go to stdout")
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.Contains(string(data), "chunk:S1:100-220") {
		t.Fatal("output file is missing the exported records")
	}
}

func TestRAGManifest_FingerprintsTheExport(t *testing.T) {
	feed := writeFeed(t, feedFixture)
	code, stdout, stderr := runRAG(t, "manifest", "--feed", feed)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stderr: %s)", code, stderr)
	}
	var m struct {
		SchemaVersion   string `json:"schema_version"`
		ChunkCount      int    `json:"chunk_count"`
		ChunkDigest     string `json:"chunk_digest"`
		FeedContentHash string `json:"feed_content_hash"`
		Sources         []struct {
			SourceID   string `json:"source_id"`
			ChunkCount int    `json:"chunk_count"`
		} `json:"sources"`
	}
	if err := json.Unmarshal([]byte(stdout), &m); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if m.SchemaVersion == "" || m.ChunkDigest == "" {
		t.Fatalf("manifest is missing its contract fields: %+v", m)
	}
	if m.ChunkCount != 2 {
		t.Fatalf("expected 2 chunks, got %d", m.ChunkCount)
	}
	if m.FeedContentHash != "sha256:feed" {
		t.Fatalf("manifest must bind to the feed it fingerprints, got %q", m.FeedContentHash)
	}
	if len(m.Sources) != 1 || m.Sources[0].SourceID != "S1" || m.Sources[0].ChunkCount != 2 {
		t.Fatalf("per-source rollup is wrong: %+v", m.Sources)
	}
}

// The manifest exists to make staleness provable; if an edited corpus produced
// the same digest, an index could never be shown to be out of date.
func TestRAGManifest_DigestMovesWhenTheCorpusMoves(t *testing.T) {
	before := writeFeed(t, feedFixture)
	_, beforeOut, _ := runRAG(t, "manifest", "--feed", before)

	after := writeFeed(t, strings.Replace(
		feedFixture, "hauteur de neuf metres", "hauteur de douze metres", 1))
	_, afterOut, _ := runRAG(t, "manifest", "--feed", after)

	if beforeOut == afterOut {
		t.Fatal("the manifest survived a corpus edit: staleness would be undetectable")
	}
}

func TestRAGCommand_UsageErrors(t *testing.T) {
	if code, _, _ := runRAG(t); code != 2 {
		t.Fatalf("bare `rag` must be a usage error, got %d", code)
	}
	if code, _, stderr := runRAG(t, "reindex"); code != 2 || !strings.Contains(stderr, "unknown rag subcommand") {
		t.Fatalf("unknown subcommand must be a usage error, got %d / %s", code, stderr)
	}
	if code, _, _ := runRAG(t, "export"); code != 2 {
		t.Fatal("missing --feed must be a usage error")
	}
	if code, stdout, _ := runRAG(t, "help"); code != 0 || !strings.Contains(stdout, "export") {
		t.Fatal("`rag help` must list the subcommands")
	}
}

// The wiring matrix fails a registered command that the help text does not
// advertise (the #543 class). This keeps that contract enforced from Go too.
func TestRAGCommand_IsAdvertisedInHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("help exited %d", code)
	}
	if !strings.Contains(stdout.String(), "  rag ") {
		t.Fatal("`rag` is registered but not advertised in the help text")
	}
}

// --- delta / verify ---------------------------------------------------------

// ragManifestFile builds a manifest from a feed fixture, the way a consumer
// records what it indexed.
func ragManifestFile(t *testing.T, feedBody string) string {
	t.Helper()
	feed := writeFeed(t, feedBody)
	out := filepath.Join(t.TempDir(), "manifest.json")
	code, _, stderr := runRAG(t, "manifest", "--feed", feed, "--output", out)
	if code != 0 {
		t.Fatalf("manifest exited %d: %s", code, stderr)
	}
	return out
}

type ragPlan struct {
	Stale       bool `json:"stale"`
	FullReindex bool `json:"full_reindex"`
	Chunks      []struct {
		ChunkID string `json:"chunk_id"`
		Action  string `json:"action"`
		Reason  string `json:"reason"`
	} `json:"chunks"`
	Summary struct {
		Unchanged int `json:"unchanged"`
		Embed     int `json:"embed"`
	} `json:"summary"`
	FullReindexReasons []string `json:"full_reindex_reasons"`
}

func decodePlan(t *testing.T, stdout string) ragPlan {
	t.Helper()
	var p ragPlan
	if err := json.Unmarshal([]byte(stdout), &p); err != nil {
		t.Fatalf("decode plan: %v\n%s", err, stdout)
	}
	return p
}

func TestRAGVerify_FreshIndexPasses(t *testing.T) {
	manifest := ragManifestFile(t, feedFixture)
	feed := writeFeed(t, feedFixture)

	code, stdout, stderr := runRAG(t, "verify", "--manifest", manifest, "--feed", feed)
	if code != 0 {
		t.Fatalf("a fresh index must exit 0, got %d (stderr: %s)", code, stderr)
	}
	p := decodePlan(t, stdout)
	if p.Stale || len(p.Chunks) != 0 || p.Summary.Unchanged != 2 {
		t.Fatalf("fresh index reported work: %+v", p)
	}
	if !strings.Contains(stderr, "index fresh") {
		t.Fatalf("verdict missing from stderr: %s", stderr)
	}
}

// The gate: an edited corpus turns the exit code red and the plan names
// exactly the chunk to re-embed — not the whole index.
func TestRAGVerify_StaleIndexFailsClosed(t *testing.T) {
	manifest := ragManifestFile(t, feedFixture)
	edited := writeFeed(t, strings.Replace(
		feedFixture, "hauteur de neuf metres", "hauteur de douze metres", 1))

	code, stdout, stderr := runRAG(t, "verify", "--manifest", manifest, "--feed", edited)
	if code != 1 {
		t.Fatalf("a stale index must exit 1, got %d (stderr: %s)", code, stderr)
	}
	p := decodePlan(t, stdout)
	if !p.Stale || p.FullReindex {
		t.Fatalf("expected stale without full reindex, got %+v", p)
	}
	if len(p.Chunks) != 1 || p.Chunks[0].ChunkID != "chunk:S1:100-220" ||
		p.Chunks[0].Action != "embed" || p.Chunks[0].Reason != "body_changed" {
		t.Fatalf("plan must name exactly the edited chunk as embed/body_changed, got %+v", p.Chunks)
	}
	if p.Summary.Unchanged != 1 {
		t.Fatalf("the untouched chunk must stay unchanged, got %+v", p.Summary)
	}
	if !strings.Contains(stderr, "index stale") {
		t.Fatalf("verdict missing from stderr: %s", stderr)
	}
}

// A manifest edited by hand keeps its digest but not its chunk list: it must
// be refused as a baseline, or a forged manifest could certify a stale index.
func TestRAGVerify_TamperedManifestIsRefused(t *testing.T) {
	manifest := ragManifestFile(t, feedFixture)
	raw, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	chunks := m["chunks"].([]any)
	chunks[0].(map[string]any)["embedding_hash"] = "sha256:" + strings.Repeat("0", 64)
	forged, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, forged, 0o644); err != nil {
		t.Fatal(err)
	}

	feed := writeFeed(t, feedFixture)
	code, stdout, _ := runRAG(t, "verify", "--manifest", manifest, "--feed", feed)
	if code != 1 {
		t.Fatalf("a tampered manifest must exit 1, got %d", code)
	}
	p := decodePlan(t, stdout)
	if !p.FullReindex || !strings.Contains(stdout, "old_manifest_digest_mismatch") {
		t.Fatalf("tampering not reported as a digest mismatch: %+v", p)
	}
}

func TestRAGVerify_RefusesNonManifestInput(t *testing.T) {
	feed := writeFeed(t, feedFixture)
	code, _, stderr := runRAG(t, "verify", "--manifest", feed, "--feed", feed)
	if code != 1 || !strings.Contains(stderr, "is not a nomos-rag-index-manifest-v1 document") {
		t.Fatalf("a feed passed as manifest must be refused, got %d / %s", code, stderr)
	}
}

// `rag delta` is a plan, not a gate: it exits 0 and lists exactly the work.
func TestRAGDelta_PlansExactlyTheChangedChunks(t *testing.T) {
	oldManifest := ragManifestFile(t, feedFixture)
	newManifest := ragManifestFile(t, strings.Replace(
		feedFixture, "recul minimal est de cinq metres", "recul minimal est de six metres", 1))

	code, stdout, stderr := runRAG(t, "delta", "--old", oldManifest, "--new", newManifest)
	if code != 0 {
		t.Fatalf("delta is informational and must exit 0, got %d (stderr: %s)", code, stderr)
	}
	p := decodePlan(t, stdout)
	if !p.Stale || len(p.Chunks) != 1 || p.Chunks[0].ChunkID != "chunk:S1:300-380" || p.Chunks[0].Reason != "body_changed" {
		t.Fatalf("plan must name exactly the edited chunk, got %+v", p)
	}
	if p.Summary.Embed != 1 || p.Summary.Unchanged != 1 {
		t.Fatalf("summary drifted: %+v", p.Summary)
	}
}

// --- bundle input / lens scoping / retrieval contract ------------------------

// bundleFixture is a minimal CKM bundle: three faceted nodes over three
// source documents, the shape `nomos bundle` emits from the AEC golden corpus.
const bundleFixture = `{
  "schema_version": "ckm-bundle-v1",
  "bundle_id": "aec-test",
  "generated_at": "2026-06-01T00:00:00Z",
  "producer": "nomos",
  "claim_boundary": "test",
  "feeds": [
    {
      "feed_id": "aec-test-feed",
      "format": "nomos.canonical-knowledge-feed.v1",
      "nodes": [
        {"node_id": "A-1", "text": "Le gabarit retient une hauteur de neuf metres au faite.", "source_path": "conception.md", "source_hash": "sha256:c1", "span": {"start_line": 3, "end_line": 3}, "parent_chain": [], "facets": {"nature": "rule", "scope_level": "atom", "trust_tier": "unverified", "provenance": "source_backed"}},
        {"node_id": "A-2", "text": "La mise a l'enquete publique dure trente jours.", "source_path": "permis.md", "source_hash": "sha256:p1", "span": {"start_line": 5, "end_line": 5}, "parent_chain": [], "facets": {"nature": "rule", "scope_level": "atom", "trust_tier": "unverified", "provenance": "source_backed"}},
        {"node_id": "A-3", "text": "Appreciation interne confidentielle du dossier.", "source_path": "journal-interne.md", "source_hash": "sha256:j1", "span": {"start_line": 2, "end_line": 2}, "parent_chain": [], "facets": {"nature": "rule", "scope_level": "atom", "trust_tier": "unverified", "provenance": "source_backed"}}
      ]
    }
  ],
  "rag_metadata": [
    {"node_id": "A-1", "chunk_id": "chunk:A-1", "source_path": "conception.md", "source_hash": "sha256:c1", "parent_chain": []},
    {"node_id": "A-2", "chunk_id": "chunk:A-2", "source_path": "permis.md", "source_hash": "sha256:p1", "parent_chain": []},
    {"node_id": "A-3", "chunk_id": "chunk:A-3", "source_path": "journal-interne.md", "source_hash": "sha256:j1", "parent_chain": []}
  ],
  "trace_manifest": {},
  "attestation": {}
}`

const permisLensFixture = `id: LENS-TEST-PERMIS
include:
  all_of:
    - activity:
        - aec.permis
      confidentiality: public
exclude:
  any_of:
    - confidentiality: confidential
`

const conceptionLensFixture = `id: LENS-TEST-CONCEPTION
include:
  all_of:
    - activity:
        - aec.conception
`

// documentFacetsFixture has the shape of a pack retrieval harness file: the
// open axes live under a top-level document_facets map keyed by source path.
const documentFacetsFixture = `record_type: pack_retrieval_harness
document_facets:
  conception.md:
    activity: [aec.conception]
    confidentiality: public
    applicability: applicable
  permis.md:
    activity: [aec.permis]
    confidentiality: public
    applicability: applicable
  journal-interne.md:
    activity: [aec.permis]
    confidentiality: confidential
    applicability: applicable
`

func ragFixtureFile(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestRAGExport_BundleInputCarriesFacets(t *testing.T) {
	b := ragFixtureFile(t, "bundle.json", bundleFixture)
	code, stdout, stderr := runRAG(t, "export", "--bundle", b)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stderr: %s)", code, stderr)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 records, got %d", len(lines))
	}
	var rec struct {
		ChunkID    string `json:"chunk_id"`
		BodyText   string `json:"body_text"`
		Provenance struct {
			SourceID   string `json:"source_id"`
			SourceHash string `json:"source_hash"`
		} `json:"provenance"`
		Metadata struct {
			Facets map[string]any `json:"facets"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rec.ChunkID != "chunk:A-1" || rec.Provenance.SourceID != "conception.md" || rec.Provenance.SourceHash != "sha256:c1" {
		t.Fatalf("bundle provenance drifted: %+v", rec)
	}
	if rec.Metadata.Facets["nature"] != "rule" {
		t.Fatalf("bundle records must carry the node facets, got %+v", rec.Metadata.Facets)
	}
	if !strings.Contains(rec.BodyText, "neuf metres") {
		t.Fatalf("node text must be the citable body, got %q", rec.BodyText)
	}
}

// Lens at the base level: only the in-scope chunk is handed out; the
// exclusions are named on stderr and are not rejections (--strict passes).
func TestRAGExport_LensScopesTheExportAndReportsExclusions(t *testing.T) {
	b := ragFixtureFile(t, "bundle.json", bundleFixture)
	lens := ragFixtureFile(t, "permis.lens.yaml", permisLensFixture)
	facets := ragFixtureFile(t, "harness.yaml", documentFacetsFixture)

	code, stdout, stderr := runRAG(t, "export", "--bundle", b, "--lens", lens, "--document-facets", facets, "--strict")
	if code != 0 {
		t.Fatalf("exclusions are not rejections: expected exit 0, got %d (stderr: %s)", code, stderr)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], `"chunk_id":"chunk:A-2"`) {
		t.Fatalf("only the permis chunk is in scope, got %d line(s): %s", len(lines), stdout)
	}
	if strings.Contains(stdout, "chunk:A-3") || strings.Contains(stdout, "confidentielle") {
		t.Fatal("the confidential chunk leaked into the export")
	}
	for _, want := range []string{"excluded chunk:A-1 [lens_excluded]", "excluded chunk:A-3 [lens_excluded]", "lens LENS-TEST-PERMIS enforced: 2 chunk(s) excluded"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr must report %q, got: %s", want, stderr)
		}
	}
	if !strings.Contains(lines[0], `"activity":["aec.permis"]`) {
		t.Fatalf("exported record must carry the merged document facets: %s", lines[0])
	}
}

// A feed carries no facets: under a lens nothing can be proved in scope, so
// nothing is exported, loudly — and --strict refuses to call that a success.
func TestRAGExport_LensOverFeedFailsClosed(t *testing.T) {
	feed := writeFeed(t, feedFixture)
	lens := ragFixtureFile(t, "permis.lens.yaml", permisLensFixture)

	code, stdout, stderr := runRAG(t, "export", "--feed", feed, "--lens", lens)
	if code != 0 || strings.TrimSpace(stdout) != "" {
		t.Fatalf("without --strict: exit 0 and an empty export expected, got %d / %q", code, stdout)
	}
	if !strings.Contains(stderr, "lens_no_facets") || !strings.Contains(stderr, "2 chunk(s) excluded") {
		t.Fatalf("exclusions must be reported: %s", stderr)
	}
	if code, _, stderr := runRAG(t, "export", "--feed", feed, "--lens", lens, "--strict"); code != 1 || !strings.Contains(stderr, "nothing exported") {
		t.Fatalf("--strict must refuse an empty scope, got %d / %s", code, stderr)
	}
}

func TestRAGManifest_BindsLensAndContract(t *testing.T) {
	b := ragFixtureFile(t, "bundle.json", bundleFixture)
	lens := ragFixtureFile(t, "permis.lens.yaml", permisLensFixture)
	facets := ragFixtureFile(t, "harness.yaml", documentFacetsFixture)

	code, stdout, stderr := runRAG(t, "manifest", "--bundle", b, "--lens", lens, "--document-facets", facets)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stderr: %s)", code, stderr)
	}
	var m struct {
		FeedFormat          string `json:"feed_format"`
		FeedContentHash     string `json:"feed_content_hash"`
		ChunkCount          int    `json:"chunk_count"`
		ExcludedByLensCount int    `json:"excluded_by_lens_count"`
		Lens                *struct {
			ID     string `json:"id"`
			Digest string `json:"digest"`
		} `json:"lens"`
		Contract struct {
			Scope        string `json:"scope"`
			FilterFields []struct {
				Field  string   `json:"field"`
				Values []string `json:"values"`
			} `json:"filter_fields"`
			Unsupported []struct {
				Capability string `json:"capability"`
			} `json:"unsupported"`
		} `json:"retrieval_contract"`
	}
	if err := json.Unmarshal([]byte(stdout), &m); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if m.Lens == nil || m.Lens.ID != "LENS-TEST-PERMIS" || !strings.HasPrefix(m.Lens.Digest, "sha256:") {
		t.Fatalf("manifest must bind the lens, got %+v", m.Lens)
	}
	if m.ChunkCount != 1 || m.ExcludedByLensCount != 2 {
		t.Fatalf("counts drifted: chunks=%d excluded=%d", m.ChunkCount, m.ExcludedByLensCount)
	}
	if m.FeedFormat != "ckm-bundle-v1" || !strings.HasPrefix(m.FeedContentHash, "sha256:") {
		t.Fatalf("manifest must bind to the bundle bytes, got %q / %q", m.FeedFormat, m.FeedContentHash)
	}
	if m.Contract.Scope != "lens" {
		t.Fatalf("contract scope must be lens, got %q", m.Contract.Scope)
	}
	var activity []string
	for _, f := range m.Contract.FilterFields {
		if f.Field == "facets.activity" {
			activity = f.Values
		}
	}
	if len(activity) != 1 || activity[0] != "aec.permis" {
		t.Fatalf("contract must list observed activity values, got %v", activity)
	}
	if len(m.Contract.Unsupported) == 0 || m.Contract.Unsupported[0].Capability != "temporal_scoping" {
		t.Fatalf("contract must declare temporal scoping unsupported: %+v", m.Contract.Unsupported)
	}
}

// The gate: an index built for one lens is stale for another, and stale when
// the scope is dropped — the consumer's WHERE clause was written for a scope.
func TestRAGVerify_DifferentLensIsStale(t *testing.T) {
	b := ragFixtureFile(t, "bundle.json", bundleFixture)
	permis := ragFixtureFile(t, "permis.lens.yaml", permisLensFixture)
	conception := ragFixtureFile(t, "conception.lens.yaml", conceptionLensFixture)
	facets := ragFixtureFile(t, "harness.yaml", documentFacetsFixture)
	manifest := filepath.Join(t.TempDir(), "manifest.json")
	if code, _, stderr := runRAG(t, "manifest", "--bundle", b, "--lens", permis, "--document-facets", facets, "--output", manifest); code != 0 {
		t.Fatalf("manifest exited %d: %s", code, stderr)
	}

	code, stdout, _ := runRAG(t, "verify", "--manifest", manifest, "--bundle", b, "--lens", conception, "--document-facets", facets)
	if code != 1 || !strings.Contains(stdout, "lens_changed") {
		t.Fatalf("a different lens must be stale with reason lens_changed, got %d / %s", code, stdout)
	}
	code, stdout, _ = runRAG(t, "verify", "--manifest", manifest, "--bundle", b, "--document-facets", facets)
	if code != 1 || !strings.Contains(stdout, "lens_changed") {
		t.Fatalf("dropping the lens must be stale, got %d / %s", code, stdout)
	}
	code, _, stderr := runRAG(t, "verify", "--manifest", manifest, "--bundle", b, "--lens", permis, "--document-facets", facets)
	if code != 0 {
		t.Fatalf("the same lens over the same bundle must be fresh, got %d (stderr: %s)", code, stderr)
	}
}

func TestRAGInputs_FeedAndBundleAreMutuallyExclusive(t *testing.T) {
	feed := writeFeed(t, feedFixture)
	b := ragFixtureFile(t, "bundle.json", bundleFixture)
	if code, _, stderr := runRAG(t, "export", "--feed", feed, "--bundle", b); code != 2 || !strings.Contains(stderr, "mutually exclusive") {
		t.Fatalf("both inputs must be a usage error, got %d / %s", code, stderr)
	}
	if code, _, stderr := runRAG(t, "manifest"); code != 2 || !strings.Contains(stderr, "one of --feed or --bundle") {
		t.Fatalf("no input must be a usage error, got %d / %s", code, stderr)
	}
	if code, _, stderr := runRAG(t, "export", "--bundle", b, "--lens", filepath.Join(t.TempDir(), "missing.yaml")); code != 1 || !strings.Contains(stderr, "lens") {
		t.Fatalf("a missing lens file must fail, got %d / %s", code, stderr)
	}
}

func TestRAGDeltaVerify_UsageErrors(t *testing.T) {
	if code, _, _ := runRAG(t, "delta", "--old", "only-one.json"); code != 2 {
		t.Fatalf("delta without --new must be a usage error, got %d", code)
	}
	if code, _, _ := runRAG(t, "verify", "--manifest", "only-one.json"); code != 2 {
		t.Fatalf("verify without --feed must be a usage error, got %d", code)
	}
	if code, _, _ := runRAG(t, "delta", "--old", "missing.json", "--new", "missing.json"); code != 1 {
		t.Fatalf("delta on missing files must fail, got %d", code)
	}
	code, stdout, _ := runRAG(t, "help")
	if code != 0 || !strings.Contains(stdout, "delta") || !strings.Contains(stdout, "verify") {
		t.Fatal("`rag help` must list delta and verify")
	}
}
