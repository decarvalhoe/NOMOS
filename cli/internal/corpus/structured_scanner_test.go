package corpus

import (
	"strings"
	"testing"
)

func TestScanStructuredScalarsYAMLProducesCoverageAndCanonicalSegments(t *testing.T) {
	source := ManifestSource{
		ID:   "SRC-YAML",
		Path: "config/rule.yaml",
		Type: "source_code",
	}
	content := []byte("rule:\n  id: R1\n  text: La regle impose une trace auditable.\n")

	scan, err := ScanStructuredScalars(source, content, StructuredFormatYAML)
	if err != nil {
		t.Fatalf("ScanStructuredScalars: %v", err)
	}
	if len(scan.Scalars) != 2 {
		t.Fatalf("expected 2 YAML value scalars, got %d: %#v", len(scan.Scalars), scan.Scalars)
	}
	if scan.Scalars[1].Path != "rule.text" {
		t.Fatalf("second scalar path = %q", scan.Scalars[1].Path)
	}
	if scan.Scalars[1].DecodedValue != "La regle impose une trace auditable." {
		t.Fatalf("second scalar decoded value = %q", scan.Scalars[1].DecodedValue)
	}
	assertStructuredScanCoversWholeFile(t, scan.Segments, len(content))
	assertHasCanonicalKind(t, scan.Segments, KindYAMLScalar)
}

func TestScanStructuredScalarsYAMLBlockScalarSpanIncludesBody(t *testing.T) {
	source := ManifestSource{
		ID:   "SRC-YAML-BLOCK",
		Path: "config/block.yaml",
		Type: "source_code",
	}
	content := []byte("module:\n  ai_instructions: |\n    Le module explique la regle.\n    Il garde la preuve source.\n  status: active\n")

	scan, err := ScanStructuredScalars(source, content, StructuredFormatYAML)
	if err != nil {
		t.Fatalf("ScanStructuredScalars: %v", err)
	}
	var block *StructuredScalar
	for i := range scan.Scalars {
		if scan.Scalars[i].Path == "module.ai_instructions" {
			block = &scan.Scalars[i]
			break
		}
	}
	if block == nil {
		t.Fatalf("block scalar not found: %#v", scan.Scalars)
	}
	raw := string(content[block.StartByte:block.EndByte])
	if !strings.Contains(raw, "Le module explique la regle.") ||
		!strings.Contains(raw, "Il garde la preuve source.") {
		t.Fatalf("block raw span does not include body: %q", raw)
	}
}

func TestScanStructuredScalarsJSONProducesExactPathSegments(t *testing.T) {
	source := ManifestSource{
		ID:   "SRC-JSON",
		Path: "contracts/rule.json",
		Type: "source_code",
	}
	content := []byte(`{"rules":[{"id":"R1","text":"La regle JSON reste atomisable."}],"enabled":true}`)

	scan, err := ScanStructuredScalars(source, content, StructuredFormatJSON)
	if err != nil {
		t.Fatalf("ScanStructuredScalars: %v", err)
	}
	var textScalar *StructuredScalar
	for i := range scan.Scalars {
		if scan.Scalars[i].Path == "rules[0].text" {
			textScalar = &scan.Scalars[i]
			break
		}
	}
	if textScalar == nil {
		t.Fatalf("rules[0].text scalar not found: %#v", scan.Scalars)
	}
	if textScalar.DecodedValue != "La regle JSON reste atomisable." {
		t.Fatalf("decoded value = %q", textScalar.DecodedValue)
	}
	if got := string(content[textScalar.StartByte:textScalar.EndByte]); got != `"La regle JSON reste atomisable."` {
		t.Fatalf("raw span = %q", got)
	}
	assertStructuredScanCoversWholeFile(t, scan.Segments, len(content))
	assertHasCanonicalKind(t, scan.Segments, KindJSONScalar)
}

func TestGenerateFeedAtomizesExplicitJSONStructuredSource(t *testing.T) {
	root := t.TempDir()
	writeFeedTestFile(t, root, "contracts/rules.json", `{"rules":[{"id":"R1","text":"La regle JSON reste atomisable dans Nomos."}],"enabled":true}`)
	manifest := `
schema_version: "0.1.0"
sources:
  - id: SRC-JSON
    path: contracts/rules.json
    type: source_code
    domain: rbok
    priority: primary
    status: active
    hash: "sha256:json"
    owner: Alice
    license: internal
    confidentiality: internal
    allowed_uses:
      - structured_contract
      - vector_index
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
	if feed.UnitCount != 1 {
		t.Fatalf("expected one semantic JSON feed unit, got %d: %#v", feed.UnitCount, feed.Units)
	}
	unit := feed.Units[0]
	if unit.StructuredFormat != StructuredFormatJSON || unit.StructuredPath != "rules[0].text" {
		t.Fatalf("structured metadata not populated: %#v", unit)
	}
	if unit.SourceSegmentID == "" || unit.StartByte == 0 || unit.EndByte == 0 {
		t.Fatalf("source linkage missing from JSON unit: %#v", unit)
	}
	if len(feed.RAGMetadata) != 1 {
		t.Fatalf("expected one JSON RAG chunk, got %d: %#v", len(feed.RAGMetadata), feed.RAGMetadata)
	}
	chunk := feed.RAGMetadata[0]
	if chunk.SourceSegmentID != unit.SourceSegmentID || chunk.SegmentKind != KindJSONScalar {
		t.Fatalf("JSON chunk source linkage mismatch: %#v", chunk)
	}
	if chunk.ChunkCompositionStrategy != string(ChunkStrategyStructured) {
		t.Fatalf("JSON chunk strategy = %q", chunk.ChunkCompositionStrategy)
	}
	if chunk.ChunkText == "" || chunk.ContextYAMLPath != "rules[0].text" {
		t.Fatalf("JSON chunk contextual fields missing: %#v", chunk)
	}
}

func TestGenerateFeedSourceBacksYAMLParcoursRAGMetadata(t *testing.T) {
	root := t.TempDir()
	yamlDoc := `parcours:
  id: PAR_TEST
  name: Parcours test
  domain: rbok
  modules:
    - code: MOD1
      name: Module stratégique
      type: guided
      description: Module de cadrage stratégique.
      objectives:
        - key: OBJ1
          titre: Objectif un
          description: Comprendre le besoin métier.
          questions:
            - key: Q1
              label: Quelle est la priorité métier actuelle ?
              type: text
              help_text: Répondre avec la priorité métier la plus importante.
`
	writeFeedTestFile(t, root, "03_parcours/PAR_TEST.yaml", yamlDoc)
	manifest := `
schema_version: "0.1.0"
sources:
  - id: RBOK-YAML
    path: 03_parcours/PAR_TEST.yaml
    type: source_code
    domain: rbok
    priority: primary
    status: active
    hash: "sha256:yaml"
    owner: Alice
    license: internal
    confidentiality: internal
    allowed_uses:
      - structured_contract
      - vector_index
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
	if feed.UnitCount == 0 {
		t.Fatal("expected YAML parcours units")
	}
	for _, unit := range feed.Units {
		if unit.SourceSegmentID == "" || unit.StructuredPath == "" || unit.StructuredFormat != StructuredFormatYAML {
			t.Fatalf("YAML unit missing structured source linkage: %#v", unit)
		}
	}
	if len(feed.RAGMetadata) != feed.UnitCount {
		t.Fatalf("RAG metadata count=%d, unit count=%d", len(feed.RAGMetadata), feed.UnitCount)
	}
	for _, chunk := range feed.RAGMetadata {
		if chunk.SourceSegmentID == "" || chunk.SegmentKind != KindYAMLScalar {
			t.Fatalf("YAML chunk missing source-backed yaml_scalar linkage: %#v", chunk)
		}
		if chunk.ChunkCompositionStrategy != string(ChunkStrategyYAMLScalar) {
			t.Fatalf("YAML chunk strategy=%q", chunk.ChunkCompositionStrategy)
		}
		if chunk.ChunkText == "" || chunk.ContextYAMLPath == "" {
			t.Fatalf("YAML chunk contextual fields missing: %#v", chunk)
		}
	}
}

func TestGenerateFeedSkipsShortYAMLParcoursScalars(t *testing.T) {
	root := t.TempDir()
	yamlDoc := `parcours:
  id: PAR_SHORT
  name: Parcours short scalar test
  domain: rbok
  modules:
    - code: MOD1
      name: Module stratégique
      type: guided
      description: Module description with enough semantic content.
      objectives:
        - key: OBJ1
          titre: Objectif un
          description: Comprendre le besoin métier avec assez de contexte.
          questions:
            - key: Q1
              label: Quelle est la priorité métier actuelle ?
              type: text
              help_text: Une phrase suffit.
            - key: Q2
              label: Quelle est la contrainte principale actuelle ?
              type: text
              help_text: Répondre avec la contrainte principale et le contexte métier utile.
`
	writeFeedTestFile(t, root, "03_parcours/PAR_SHORT.yaml", yamlDoc)
	manifest := `
schema_version: "0.1.0"
sources:
  - id: RBOK-YAML-SHORT
    path: 03_parcours/PAR_SHORT.yaml
    type: source_code
    domain: rbok
    priority: primary
    status: active
    hash: "sha256:yaml-short"
    owner: Alice
    license: internal
    confidentiality: internal
    allowed_uses:
      - structured_contract
      - vector_index
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
	var shortFound, longFound bool
	for _, unit := range feed.Units {
		switch unit.BusinessRule {
		case "Une phrase suffit.":
			shortFound = true
		case "Répondre avec la contrainte principale et le contexte métier utile.":
			longFound = true
		}
	}
	if shortFound {
		t.Fatalf("short YAML parcours scalar must not enter feed: %#v", feed.Units)
	}
	if !longFound {
		t.Fatalf("expected longer YAML parcours scalar to remain in feed: %#v", feed.Units)
	}
}

func assertStructuredScanCoversWholeFile(t *testing.T, segments []SourceSegment, contentLen int) {
	t.Helper()
	gaps := computeGaps(make([]byte, contentLen), segments)
	if len(gaps) != 0 {
		t.Fatalf("structured scan left gaps: %#v\nsegments=%#v", gaps, segments)
	}
}

func assertHasCanonicalKind(t *testing.T, segments []SourceSegment, kind string) {
	t.Helper()
	for _, seg := range segments {
		if seg.Kind == kind && seg.Disposition == DispositionCanonicalAtom {
			return
		}
	}
	t.Fatalf("no canonical segment of kind %q in %#v", kind, segments)
}
