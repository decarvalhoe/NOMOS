package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CKM-H3 follow-up (#540): prove that the knowledge lens now changes engine
// behaviour through a production CLI path, not just in unit tests.
//
// The fixture has a paragraph (atom nature "rule", via AtomRule) and a fenced
// code block (atom nature "evidence", via AtomCodeBlock). A lens that excludes
// nature=evidence must therefore drop exactly the code-block chunk while keeping
// the rule chunk — and the same corpus WITHOUT the lens must keep both.
const lensFixtureMD = "# Permit Rule\n\n" +
	"An applicant must submit the cantonal permit dossier before any site works begin.\n\n" +
	"```text\n" +
	"EVIDENCE-EXHIBIT-A: raw permit response payload\n" +
	"```\n"

// excludeEvidenceLens drops any candidate whose facet nature is "evidence".
const excludeEvidenceLensYAML = `id: LENS-NO-EVIDENCE
description: Drop raw evidence atoms from retrieval scope.
default_behavior: include_all_when_no_lens
exclude:
  any_of:
    - nature: evidence
`

func writeFixture(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

// TestRunAtomizeIsReachable proves `nomos atomize` is wired into the binary's
// command map (it was previously implemented but never registered, so --facets
// and the lens were unreachable outside package tests).
func TestRunAtomizeIsReachable(t *testing.T) {
	mdPath := writeFixture(t, "doc.md", lensFixtureMD)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"atomize", "chunks", "--doc-ref", "permit", mdPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("atomize unreachable via Run: code=%d stderr=%q", code, stderr.String())
	}

	var chunks []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &chunks); err != nil {
		t.Fatalf("atomize chunks did not emit a chunk array: %v\n%s", err, stdout.String())
	}
	if len(chunks) == 0 {
		t.Fatal("expected chunks from reachable atomize command")
	}

	// The help text must also advertise the command.
	var help bytes.Buffer
	if Run(nil, &help, &help) != 0 {
		t.Fatal("help command failed")
	}
	if !strings.Contains(help.String(), "atomize") {
		t.Fatalf("help text omits the registered atomize command:\n%s", help.String())
	}
}

// scopedChunks mirrors the lens-mode output shape of `atomize chunks --lens`.
type scopedChunks struct {
	LensID string `json:"lens_id"`
	Mode   string `json:"mode"`
	Chunks []struct {
		ChunkID      string `json:"chunk_id"`
		AtomID       string `json:"atom_id"`
		CanonicalRef string `json:"canonical_ref"`
		Type         string `json:"type"`
		Text         string `json:"text"`
	} `json:"chunks"`
	Excluded []struct {
		ChunkID      string `json:"chunk_id"`
		CanonicalRef string `json:"canonical_ref"`
		Reason       string `json:"reason"`
	} `json:"excluded"`
}

// TestAtomizeChunksLensExcludesFacet is the adversarial proof that facets now
// change engine behaviour: with a nature=evidence exclusion lens the code-block
// chunk is ABSENT from the output (and recorded as excluded with the lens's
// reason), while the same corpus WITHOUT the lens INCLUDES it.
func TestAtomizeChunksLensExcludesFacet(t *testing.T) {
	mdPath := writeFixture(t, "doc.md", lensFixtureMD)
	lensPath := writeFixture(t, "lens.yaml", excludeEvidenceLensYAML)

	// --- Run WITHOUT the lens: the evidence (code-block) chunk is present. ---
	var baseOut, baseErr bytes.Buffer
	if code := AtomizeCommand([]string{"chunks", "--doc-ref", "permit", mdPath}, &baseOut, &baseErr); code != 0 {
		t.Fatalf("baseline chunks failed: code=%d stderr=%q", code, baseErr.String())
	}
	var baseChunks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(baseOut.Bytes(), &baseChunks); err != nil {
		t.Fatalf("baseline chunks JSON: %v\n%s", err, baseOut.String())
	}
	if countCodeBlocks(baseChunks) == 0 {
		t.Fatalf("fixture invalid: baseline produced no code_block chunk to exclude\n%s", baseOut.String())
	}

	// --- Run WITH the lens: the evidence chunk is dropped, the rule kept. ---
	var lensOut, lensErr bytes.Buffer
	if code := AtomizeCommand([]string{"chunks", "--doc-ref", "permit", "--lens", lensPath, mdPath}, &lensOut, &lensErr); code != 0 {
		t.Fatalf("lensed chunks failed: code=%d stderr=%q", code, lensErr.String())
	}
	var scoped scopedChunks
	if err := json.Unmarshal(lensOut.Bytes(), &scoped); err != nil {
		t.Fatalf("lensed chunks JSON: %v\n%s", err, lensOut.String())
	}

	if scoped.LensID != "LENS-NO-EVIDENCE" || scoped.Mode != "lens" {
		t.Fatalf("expected lens-mode output, got lens_id=%q mode=%q", scoped.LensID, scoped.Mode)
	}

	// PROOF 1: no surviving chunk is a code_block (the excluded facet value).
	for _, c := range scoped.Chunks {
		if c.Type == "code_block" {
			t.Fatalf("lens failed to exclude code_block chunk %s (facets did not change engine behaviour)", c.ChunkID)
		}
	}
	// PROOF 2: a rule chunk did survive, so the lens excluded selectively, not everything.
	if !hasRuleChunk(scoped) {
		t.Fatal("lens dropped every chunk; expected the rule chunk to survive a nature=evidence exclusion")
	}
	// PROOF 3: the dropped unit is recorded with the lens's own reason.
	if len(scoped.Excluded) == 0 {
		t.Fatal("lens excluded no units; expected the evidence chunk to be recorded as excluded")
	}
	foundEvidenceReason := false
	for _, e := range scoped.Excluded {
		if e.Reason != "excluded_by_facets.nature" {
			t.Fatalf("excluded unit %s carried unexpected reason %q", e.ChunkID, e.Reason)
		}
		foundEvidenceReason = true
	}
	if !foundEvidenceReason {
		t.Fatal("expected an excluded unit with reason excluded_by_facets.nature")
	}

	// PROOF 4 (differential): the count of surviving chunks under the lens is
	// strictly smaller than the baseline — the lens removed real units.
	if len(scoped.Chunks) >= len(baseChunks) {
		t.Fatalf("lens did not reduce chunk count: baseline=%d lensed=%d", len(baseChunks), len(scoped.Chunks))
	}
}

// TestAtomizeChunksWithoutLensIsUnchanged guards the default path: no --lens
// means the output is still a bare chunk array (no lens envelope), so existing
// consumers are byte-shape-compatible (zero regression).
func TestAtomizeChunksWithoutLensIsUnchanged(t *testing.T) {
	mdPath := writeFixture(t, "doc.md", lensFixtureMD)

	var out, errBuf bytes.Buffer
	if code := AtomizeCommand([]string{"chunks", "--doc-ref", "permit", mdPath}, &out, &errBuf); code != 0 {
		t.Fatalf("chunks failed: code=%d stderr=%q", code, errBuf.String())
	}
	// Must decode as a bare array, not an object with lens_id/excluded.
	var asArray []map[string]any
	if err := json.Unmarshal(out.Bytes(), &asArray); err != nil {
		t.Fatalf("default chunks output is not a bare array (regression): %v\n%s", err, out.String())
	}
	// And facets must be absent without --facets/--lens (additive opt-in).
	for _, c := range asArray {
		if _, ok := c["facets"]; ok {
			t.Fatalf("default chunks carried facets without --facets/--lens (regression): %v", c)
		}
	}
}

func countCodeBlocks(chunks []struct {
	Type string `json:"type"`
	Text string `json:"text"`
}) int {
	n := 0
	for _, c := range chunks {
		if c.Type == "code_block" {
			n++
		}
	}
	return n
}

func hasRuleChunk(s scopedChunks) bool {
	for _, c := range s.Chunks {
		if c.Type == "rule" {
			return true
		}
	}
	return false
}
