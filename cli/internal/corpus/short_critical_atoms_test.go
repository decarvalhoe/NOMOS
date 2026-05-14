package corpus

import (
	"strings"
	"testing"
)

func TestGenerateFeedEmitsShortCriticalAtomsReportAcrossFormats(t *testing.T) {
	root := t.TempDir()
	writeFeedTestFile(t, root, "legal/legal.md", `# Legal Controls

Critical clauses mention GxP and ALCOA+ only with parent evidence context.

| Control | Value | Status |
| --- | --- | --- |
| 21 CFR | Yes | P0 |
`)
	writeFeedTestFile(t, root, "technical/controls.yaml", `controls:
  - id: SOP-01
    priority: P0
    status: Yes
    rule: Technical release control requires independent evidence review before approval.
`)
	writeFeedTestFile(t, root, "game/rules.json", `{
  "game": {
    "mode": "PvP",
    "approved": false,
    "rule": "Business game rule requires parent context before retrieval indexing."
  }
}`)
	manifest := `
schema_version: "0.1.0"
sources:
  - id: SRC-LEGAL
    path: legal/legal.md
    type: markdown
    domain: legal
    priority: primary
    status: active
    hash: "sha256:legal"
    owner: Alice
    confidentiality: internal
    admission_status: admitted
    atomization_status: atomized
    source_role: canonical
    format_support: supported
  - id: SRC-TECH
    path: technical/controls.yaml
    type: source_code
    domain: technical
    priority: primary
    status: active
    hash: "sha256:technical"
    owner: Alice
    confidentiality: internal
    admission_status: admitted
    atomization_status: atomized
    source_role: canonical
    format_support: supported
  - id: SRC-GAME
    path: game/rules.json
    type: source_code
    domain: game
    priority: primary
    status: active
    hash: "sha256:game"
    owner: Alice
    confidentiality: internal
    admission_status: admitted
    atomization_status: atomized
    source_role: canonical
    format_support: supported
`

	feed, err := GenerateFeed(FeedInput{
		ManifestYAML: []byte(manifest),
		Root:         root,
		GeneratedAt:  fixedTime,
	})
	if err != nil {
		t.Fatalf("GenerateFeed: %v", err)
	}

	if feed.ShortCriticalAtoms == nil {
		t.Fatal("expected short critical atom report")
	}
	if feed.ShortCriticalAtoms.AtomCount == 0 {
		t.Fatal("expected non-empty short critical atom inventory")
	}
	if feed.ShortCriticalAtoms.UnresolvedCount != 0 {
		t.Fatalf("expected no unresolved short atoms, got %#v", feed.ShortCriticalAtoms.Atoms)
	}

	assertShortAtom(t, feed.ShortCriticalAtoms, "GxP", ShortCriticalContextualizedInParent, "legal/legal.md")
	assertShortAtom(t, feed.ShortCriticalAtoms, "ALCOA+", ShortCriticalContextualizedInParent, "legal/legal.md")
	assertShortAtom(t, feed.ShortCriticalAtoms, "21 CFR", ShortCriticalIdentifierAtom, "legal/legal.md")
	assertShortAtom(t, feed.ShortCriticalAtoms, "SOP-01", ShortCriticalIdentifierAtom, "technical/controls.yaml")
	assertShortAtom(t, feed.ShortCriticalAtoms, "P0", ShortCriticalNormativeValueAtom, "technical/controls.yaml")
	assertShortAtom(t, feed.ShortCriticalAtoms, "Yes", ShortCriticalNormativeValueAtom, "technical/controls.yaml")
	assertShortAtom(t, feed.ShortCriticalAtoms, "false", ShortCriticalNormativeValueAtom, "game/rules.json")

	for _, chunk := range feed.RAGMetadata {
		if runeCount(strings.TrimSpace(chunk.ChunkText)) <= 10 {
			t.Fatalf("RAG emitted orphan <=10-character chunk: %#v", chunk)
		}
		if strings.Contains(chunk.ChunkText, "21 CFR") && !strings.Contains(chunk.ChunkText, "Control=21 CFR") {
			t.Fatalf("RAG chunk containing 21 CFR lacks table parent context: %#v", chunk)
		}
		if strings.Contains(chunk.ChunkText, "SOP-01") {
			t.Fatalf("short standalone identifier must not become an orphan RAG chunk: %#v", chunk)
		}
	}
}

func TestCheckSemanticQualityFailsOnUnresolvedShortCriticalAtom(t *testing.T) {
	report := CheckSemanticQuality(SemanticQualityInput{
		ShortCriticalAtoms: &ShortCriticalAtomsReport{
			Atoms: []ShortCriticalAtom{{
				Fragment:    "TBD",
				SourceID:    "SRC-LEGAL",
				SourcePath:  "legal/legal.md",
				StartLine:   12,
				Disposition: ShortCriticalRequiresReview,
			}},
		},
	})

	if report.Status != "fail" {
		t.Fatalf("expected status=fail, got %q (%+v)", report.Status, report.Findings)
	}
	f, ok := sqgFindingByCode(report, FindingShortCriticalRequiresReview)
	if !ok {
		t.Fatalf("expected %s, got %+v", FindingShortCriticalRequiresReview, report.Findings)
	}
	if f.Severity != SemanticSeverityBlocking {
		t.Fatalf("expected blocking severity, got %q", f.Severity)
	}
}

func TestGenerateFeedAllowsShortOnlyCorpusWhenAtomsAreGoverned(t *testing.T) {
	root := t.TempDir()
	writeFeedTestFile(t, root, "short/values.json", `{
  "release": {
    "id": "SOP-01",
    "priority": "P0",
    "approved": true
  }
}`)
	manifest := `
schema_version: "0.1.0"
sources:
  - id: SRC-SHORT
    path: short/values.json
    type: source_code
    domain: technical
    priority: primary
    status: active
    hash: "sha256:short"
    owner: Alice
    confidentiality: internal
    admission_status: admitted
    atomization_status: atomized
    source_role: canonical
    format_support: supported
`

	feed, err := GenerateFeed(FeedInput{
		ManifestYAML: []byte(manifest),
		Root:         root,
		GeneratedAt:  fixedTime,
	})
	if err != nil {
		t.Fatalf("GenerateFeed should allow short-only governed atoms: %v", err)
	}
	if feed.UnitCount != 0 {
		t.Fatalf("short-only source should not create orphan feed units, got %d", feed.UnitCount)
	}
	if feed.ShortCriticalAtoms == nil || feed.ShortCriticalAtoms.AtomCount != 3 {
		t.Fatalf("expected three governed short atoms, got %#v", feed.ShortCriticalAtoms)
	}
	if feed.ShortCriticalAtoms.UnresolvedCount != 0 {
		t.Fatalf("expected no unresolved short atoms: %#v", feed.ShortCriticalAtoms.Atoms)
	}
}

func assertShortAtom(t *testing.T, report *ShortCriticalAtomsReport, fragment, disposition, sourcePath string) {
	t.Helper()
	for _, atom := range report.Atoms {
		if atom.Fragment != fragment || atom.SourcePath != sourcePath {
			continue
		}
		if atom.Disposition != disposition {
			t.Fatalf("atom %q disposition=%q, want %q: %#v", fragment, atom.Disposition, disposition, atom)
		}
		if atom.SourceID == "" || atom.StartLine == 0 || atom.StartByte == atom.EndByte {
			t.Fatalf("atom %q missing source evidence: %#v", fragment, atom)
		}
		if atom.SurroundingContext == "" {
			t.Fatalf("atom %q missing surrounding context: %#v", fragment, atom)
		}
		if disposition != ShortCriticalContextualizedInParent && atom.PromotedArtifactID == "" {
			t.Fatalf("atom %q missing promoted artifact id: %#v", fragment, atom)
		}
		return
	}
	t.Fatalf("missing short atom fragment=%q source_path=%q in %#v", fragment, sourcePath, report.Atoms)
}
