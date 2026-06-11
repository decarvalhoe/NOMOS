package corpus

import (
	"errors"
	"strings"
	"testing"
)

// FSQ-07 (#370) — context-rich RAG chunk composer.

// fsq07ParagraphSegment returns a canonical_atom paragraph segment with the
// raw / normalized hashes set so SFI-04 invariants would pass.
func fsq07ParagraphSegment() SourceSegment {
	return SourceSegment{
		SegmentID:          "seg:RBOK-RULE:0-58:paragraph",
		SourceID:           "RBOK-RULE",
		SourcePath:         "docs/rule.md",
		Kind:               KindParagraph,
		Disposition:        DispositionCanonicalAtom,
		StartByte:          0,
		EndByte:            58,
		StartLine:          2,
		EndLine:            2,
		RawTextHash:        "rawhash",
		NormalizedTextHash: "normhash-paragraph",
		CanonicalUnitID:    "RBOK-MD-RBOK-RULE-RULE-A",
	}
}

func fsq07ParagraphUnit(seg SourceSegment) FeedUnit {
	return FeedUnit{
		UnitID:               "RBOK-MD-RBOK-RULE-RULE-A",
		Name:                 "Rule A",
		Domain:               "rbok",
		UnitType:             "rule",
		Status:               "partial",
		BusinessRule:         "L'assure doit declarer le sinistre dans les 5 jours ouvrables.",
		SourceIDs:            []string{seg.SourceID},
		SourceSegmentID:      seg.SegmentID,
		SourceID:             seg.SourceID,
		SourcePath:           seg.SourcePath,
		StartByte:            seg.StartByte,
		EndByte:              seg.EndByte,
		StartLine:            seg.StartLine,
		EndLine:              seg.EndLine,
		NormalizedTextHash:   seg.NormalizedTextHash,
		HeadingPath:          []string{"Rules", "Rule A"},
		BodyLedgerSegmentIDs: []string{seg.SegmentID},
	}
}

func fsq07TableRowSegments() (SourceSegment, []SourceSegment) {
	row := SourceSegment{
		SegmentID:          "seg:RBOK-OFFERS:120-200:table_row",
		SourceID:           "RBOK-OFFERS",
		SourcePath:         "docs/offers.md",
		Kind:               KindTableRow,
		Disposition:        DispositionCanonicalAtom,
		StartByte:          120,
		EndByte:            200,
		StartLine:          5,
		EndLine:            5,
		RawTextHash:        "rawhash-row",
		NormalizedTextHash: "normhash-row",
		CanonicalUnitID:    "RBOK-MD-RBOK-OFFERS-PROGRAMME-LANCEMENT-R0",
		TableID:            "table:RBOK-OFFERS:100-200:table",
		ColumnHeaders:      []string{"Offre", "Prix", "Durée", "Contenu"},
		RowIndex:           0,
		RowCanonicalText:   "Offre=Programme Lancement; Prix=980 CHF; Durée=3 mois; Contenu=Accès plateforme",
	}
	cells := []SourceSegment{
		{SegmentID: "seg:cell:120-145", ParentSegmentID: row.SegmentID, Kind: KindTableCell, StartByte: 120, EndByte: 145, Disposition: DispositionCoverageOnly},
		{SegmentID: "seg:cell:146-160", ParentSegmentID: row.SegmentID, Kind: KindTableCell, StartByte: 146, EndByte: 160, Disposition: DispositionCoverageOnly},
		{SegmentID: "seg:cell:161-175", ParentSegmentID: row.SegmentID, Kind: KindTableCell, StartByte: 161, EndByte: 175, Disposition: DispositionCoverageOnly},
		{SegmentID: "seg:cell:176-200", ParentSegmentID: row.SegmentID, Kind: KindTableCell, StartByte: 176, EndByte: 200, Disposition: DispositionCoverageOnly},
	}
	return row, cells
}

func fsq07TableRowUnit(row SourceSegment, cells []SourceSegment) FeedUnit {
	ids := []string{row.SegmentID}
	for _, c := range cells {
		ids = append(ids, c.SegmentID)
	}
	return FeedUnit{
		UnitID:               "RBOK-MD-RBOK-OFFERS-PROGRAMME-LANCEMENT",
		Name:                 "Catalogue",
		Domain:               "rbok",
		UnitType:             "table_row",
		Status:               "partial",
		BusinessRule:         row.RowCanonicalText,
		SourceIDs:            []string{row.SourceID},
		SourceSegmentID:      row.SegmentID,
		SourceID:             row.SourceID,
		SourcePath:           row.SourcePath,
		StartByte:            row.StartByte,
		EndByte:              row.EndByte,
		StartLine:            row.StartLine,
		EndLine:              row.EndLine,
		NormalizedTextHash:   row.NormalizedTextHash,
		HeadingPath:          []string{"Offres", "Catalogue"},
		TableID:              row.TableID,
		RowIndex:             row.RowIndex,
		ColumnHeaders:        append([]string(nil), row.ColumnHeaders...),
		BodyLedgerSegmentIDs: ids,
	}
}

func fsq07YAMLUnit() FeedUnit {
	return FeedUnit{
		UnitID:               "RBOK-PARCOURS-Q-HELP",
		Name:                 "parcours.modules[2].questions[7].help_text",
		Domain:               "rbok",
		UnitType:             "rule",
		Status:               "partial",
		BusinessRule:         "Renseignez la date du sinistre au format YYYY-MM-DD.",
		SourceIDs:            []string{"RBOK-PARCOURS"},
		SourceSegmentID:      "seg:yaml:42-180:scalar",
		SourceID:             "RBOK-PARCOURS",
		SourcePath:           "configs/parcours.yaml",
		StartByte:            42,
		EndByte:              180,
		StartLine:            48,
		EndLine:              48,
		NormalizedTextHash:   "normhash-yaml",
		HeadingPath:          []string{"Parcours", "Module 2"},
		YAMLPath:             "parcours.modules[2].questions[7].help_text",
		DecodedValue:         "Renseignez la date du sinistre au format YYYY-MM-DD.",
		NodeKind:             "scalar",
		BusinessRuleMode:     "decoded",
		BodyLedgerSegmentIDs: []string{"seg:yaml:42-180:scalar"},
	}
}

func fsq07Sources() []FeedSource {
	return []FeedSource{
		{ID: "RBOK-RULE", Path: "docs/rule.md", Domain: "rbok", Type: "markdown", Hash: "sha256:abc", SourceRole: "canonical"},
		{ID: "RBOK-OFFERS", Path: "docs/offers.md", Domain: "rbok", Type: "markdown", Hash: "sha256:def", SourceRole: "canonical"},
		{ID: "RBOK-PARCOURS", Path: "configs/parcours.yaml", Domain: "rbok", Type: "yaml", Hash: "sha256:ghi", SourceRole: "canonical"},
	}
}

func fsq07Profile() SemanticQualityProfile {
	return DefaultRBOKProfile()
}

func fsq07BaseConfig() RAGBuildInput {
	return RAGBuildInput{
		SourceHash: "",
		Domain:     "",
		Priority:   "primary",
		Status:     "active",
		Confidence: "medium",
	}
}

//  1. Single-atom chunk: paragraph FeedUnit produces a single_atom chunk
//     with heading-prefixed ChunkText.
func TestComposeRAGChunks_SingleAtom(t *testing.T) {
	seg := fsq07ParagraphSegment()
	u := fsq07ParagraphUnit(seg)

	chunks, err := ComposeRAGChunks(RAGComposeInput{
		FeedUnits:  []FeedUnit{u},
		Sources:    fsq07Sources(),
		Segments:   []SourceSegment{seg},
		Profile:    fsq07Profile(),
		BaseConfig: fsq07BaseConfig(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	c := chunks[0]
	if c.ChunkCompositionStrategy != string(ChunkStrategySingleAtom) {
		t.Fatalf("strategy = %q, want single_atom", c.ChunkCompositionStrategy)
	}
	wantPrefix := "Rules/Rule A · "
	if !strings.HasPrefix(c.ChunkText, wantPrefix) {
		t.Fatalf("ChunkText %q missing prefix %q", c.ChunkText, wantPrefix)
	}
	if !strings.Contains(c.ChunkText, "L'assure doit declarer le sinistre") {
		t.Fatalf("ChunkText missing body: %q", c.ChunkText)
	}
	if got := c.ContextHeadingPath; len(got) != 2 || got[0] != "Rules" || got[1] != "Rule A" {
		t.Fatalf("ContextHeadingPath = %v, want [Rules Rule A]", got)
	}
	if c.ContextSourceRole != "canonical" {
		t.Fatalf("ContextSourceRole = %q, want 'canonical'", c.ContextSourceRole)
	}
	if c.SourceSegmentID != seg.SegmentID {
		t.Fatalf("SourceSegmentID = %q, want %q", c.SourceSegmentID, seg.SegmentID)
	}
	if got := c.SourceSegmentIDs; len(got) != 1 || got[0] != seg.SegmentID {
		t.Fatalf("SourceSegmentIDs = %v, want [%s]", got, seg.SegmentID)
	}
}

//  2. Table-row chunk: FSQ-03 table_row unit produces a table_row strategy
//     chunk whose ChunkText is `H/H · Col=Val; Col=Val; ...` and whose
//     SourceSegmentIDs lists [rowID, ...cellIDs] in stable order.
func TestComposeRAGChunks_TableRow(t *testing.T) {
	row, cells := fsq07TableRowSegments()
	u := fsq07TableRowUnit(row, cells)

	allSegs := append([]SourceSegment{row}, cells...)
	chunks, err := ComposeRAGChunks(RAGComposeInput{
		FeedUnits:  []FeedUnit{u},
		Sources:    fsq07Sources(),
		Segments:   allSegs,
		Profile:    fsq07Profile(),
		BaseConfig: fsq07BaseConfig(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	c := chunks[0]
	if c.ChunkCompositionStrategy != string(ChunkStrategyTableRow) {
		t.Fatalf("strategy = %q, want table_row", c.ChunkCompositionStrategy)
	}
	wantText := "Offres/Catalogue · Offre=Programme Lancement; Prix=980 CHF; Durée=3 mois; Contenu=Accès plateforme"
	if c.ChunkText != wantText {
		t.Fatalf("ChunkText = %q, want %q", c.ChunkText, wantText)
	}
	if c.ContextTableID != row.TableID {
		t.Fatalf("ContextTableID = %q, want %q", c.ContextTableID, row.TableID)
	}
	if got := c.ContextColumnHeaders; len(got) != 4 || got[0] != "Offre" || got[3] != "Contenu" {
		t.Fatalf("ContextColumnHeaders = %v", got)
	}
	wantSegIDs := []string{row.SegmentID, cells[0].SegmentID, cells[1].SegmentID, cells[2].SegmentID, cells[3].SegmentID}
	if got := c.SourceSegmentIDs; len(got) != len(wantSegIDs) {
		t.Fatalf("SourceSegmentIDs len = %d, want %d (%v)", len(got), len(wantSegIDs), got)
	} else {
		for i := range got {
			if got[i] != wantSegIDs[i] {
				t.Fatalf("SourceSegmentIDs[%d] = %q, want %q", i, got[i], wantSegIDs[i])
			}
		}
	}
}

//  3. YAML scalar chunk: FSQ-04 unit produces a yaml_scalar strategy chunk
//     whose ChunkText embeds the YAML key path.
func TestComposeRAGChunks_YAMLScalar(t *testing.T) {
	u := fsq07YAMLUnit()

	chunks, err := ComposeRAGChunks(RAGComposeInput{
		FeedUnits:  []FeedUnit{u},
		Sources:    fsq07Sources(),
		Profile:    fsq07Profile(),
		BaseConfig: fsq07BaseConfig(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	c := chunks[0]
	if c.ChunkCompositionStrategy != string(ChunkStrategyYAMLScalar) {
		t.Fatalf("strategy = %q, want yaml_scalar", c.ChunkCompositionStrategy)
	}
	if c.ContextYAMLPath != u.YAMLPath {
		t.Fatalf("ContextYAMLPath = %q, want %q", c.ContextYAMLPath, u.YAMLPath)
	}
	if !strings.Contains(c.ChunkText, u.YAMLPath) {
		t.Fatalf("ChunkText %q missing yaml path %q", c.ChunkText, u.YAMLPath)
	}
	if !strings.Contains(c.ChunkText, "Renseignez la date du sinistre") {
		t.Fatalf("ChunkText missing decoded value: %q", c.ChunkText)
	}
}

//  4. Below-token-min: a single-cell row with no real content fails the
//     composer's post-composition threshold check.
func TestComposeRAGChunks_BelowTokenMin(t *testing.T) {
	row, _ := fsq07TableRowSegments()
	row.RowCanonicalText = "Offre=X"
	u := fsq07TableRowUnit(row, nil)
	u.BusinessRule = row.RowCanonicalText
	u.BodyLedgerSegmentIDs = []string{row.SegmentID}

	_, err := ComposeRAGChunks(RAGComposeInput{
		FeedUnits:  []FeedUnit{u},
		Sources:    fsq07Sources(),
		Segments:   []SourceSegment{row},
		Profile:    fsq07Profile(),
		BaseConfig: fsq07BaseConfig(),
	})
	if err == nil {
		t.Fatal("expected RAG_CHUNK_BELOW_TOKEN_MIN, got nil")
	}
	assertComposeRejection(t, err, RAGChunkBelowTokenMin)
}

//  5. Stop-label rejection: a paragraph whose body is exactly "Champ"
//     (case-insensitive denylist hit) is rejected.
func TestComposeRAGChunks_StopLabel(t *testing.T) {
	seg := fsq07ParagraphSegment()
	u := fsq07ParagraphUnit(seg)
	u.BusinessRule = "Champ"

	_, err := ComposeRAGChunks(RAGComposeInput{
		FeedUnits:  []FeedUnit{u},
		Sources:    fsq07Sources(),
		Segments:   []SourceSegment{seg},
		Profile:    fsq07Profile(),
		BaseConfig: fsq07BaseConfig(),
	})
	if err == nil {
		t.Fatal("expected RAG_CHUNK_STOP_LABEL, got nil")
	}
	assertComposeRejection(t, err, RAGChunkStopLabel)
}

//  6. Heading-group strategy is INTENTIONALLY not implemented in FSQ-07.
//     Documented as a stretch goal; FSQ-06 already flags below-threshold
//     paragraphs via the semantic gate so the runtime does not depend on
//     this composer branch. Test pinpoints the documented choice so the
//     next ticket can flip the behaviour deliberately.
func TestComposeRAGChunks_HeadingGroupSkipped(t *testing.T) {
	const want = "heading_group"
	if want == string(ChunkStrategyHeadingGroup) {
		// constant exists; intentionally not selected by pickCompositionStrategy.
	} else {
		t.Fatalf("ChunkStrategyHeadingGroup constant unexpectedly absent")
	}
	// Two adjacent below-min paragraphs under the same heading would, in a
	// future ticket, be merged. For now the composer rejects each on its
	// own threshold and the FSQ-06 gate surfaces them. Verify exactly that.
	seg := fsq07ParagraphSegment()
	short := fsq07ParagraphUnit(seg)
	short.BusinessRule = "Hi."
	_, err := ComposeRAGChunks(RAGComposeInput{
		FeedUnits:  []FeedUnit{short},
		Sources:    fsq07Sources(),
		Segments:   []SourceSegment{seg},
		Profile:    fsq07Profile(),
		BaseConfig: fsq07BaseConfig(),
	})
	if err == nil {
		t.Fatal("expected below-threshold rejection without heading_group merge")
	}
}

//  7. Determinism: composing the same input twice yields a byte-identical
//     chunk list (modulo the IngestedAt timestamp, which is the only
//     non-deterministic field).
func TestComposeRAGChunks_Deterministic(t *testing.T) {
	seg1 := fsq07ParagraphSegment()
	row, cells := fsq07TableRowSegments()
	u1 := fsq07ParagraphUnit(seg1)
	u2 := fsq07TableRowUnit(row, cells)
	yaml := fsq07YAMLUnit()

	// Feed in unsorted order to force the composer's internal sort.
	in := RAGComposeInput{
		FeedUnits:  []FeedUnit{u2, yaml, u1},
		Sources:    fsq07Sources(),
		Segments:   append([]SourceSegment{seg1, row}, cells...),
		Profile:    fsq07Profile(),
		BaseConfig: fsq07BaseConfig(),
	}
	a, err := ComposeRAGChunks(in)
	if err != nil {
		t.Fatalf("compose A: %v", err)
	}
	b, err := ComposeRAGChunks(in)
	if err != nil {
		t.Fatalf("compose B: %v", err)
	}
	if len(a) != len(b) {
		t.Fatalf("len mismatch: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].ChunkID != b[i].ChunkID {
			t.Fatalf("ChunkID order mismatch at %d: %q vs %q", i, a[i].ChunkID, b[i].ChunkID)
		}
		if a[i].ChunkText != b[i].ChunkText {
			t.Fatalf("ChunkText mismatch at %d: %q vs %q", i, a[i].ChunkText, b[i].ChunkText)
		}
	}
	// Output is sorted by UnitID ASC.
	for i := 1; i < len(a); i++ {
		if a[i].UnitIDs[0] < a[i-1].UnitIDs[0] {
			t.Fatalf("output not sorted by unit_id: %v", []string{a[i-1].UnitIDs[0], a[i].UnitIDs[0]})
		}
	}
}

//  8. RBOK architecture spot-check: composing the offers table row produces
//     a chunk that fuses the offer name, price, duration, and content; no
//     chunk in the output is a bare "Champ" / "Valeur" / "980 CHF" / "3 mois".
func TestComposeRAGChunks_RBOKArchitectureSpotCheck(t *testing.T) {
	row, cells := fsq07TableRowSegments()
	u := fsq07TableRowUnit(row, cells)

	chunks, err := ComposeRAGChunks(RAGComposeInput{
		FeedUnits:  []FeedUnit{u},
		Sources:    fsq07Sources(),
		Segments:   append([]SourceSegment{row}, cells...),
		Profile:    fsq07Profile(),
		BaseConfig: fsq07BaseConfig(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bareLabels := []string{"Champ", "Valeur", "980 CHF", "3 mois"}
	for _, c := range chunks {
		body := strings.TrimSpace(c.ChunkText)
		for _, bad := range bareLabels {
			if body == bad {
				t.Fatalf("chunk %q is a bare label %q; should be context-rich", c.ChunkID, bad)
			}
		}
	}
	if len(chunks) == 0 {
		t.Fatal("expected at least one composed chunk")
	}
	c := chunks[0]
	for _, want := range []string{"Programme Lancement", "980 CHF", "3 mois", "Accès plateforme"} {
		if !strings.Contains(c.ChunkText, want) {
			t.Fatalf("chunk text missing %q: %q", want, c.ChunkText)
		}
	}
}

//  9. Retrieval-style: substring search for a unique token returns a chunk
//     whose ContextSourceRole matches the FSQ-02 admission default
//     ("canonical" in our test fixtures).
func TestComposeRAGChunks_RetrievalSubstring(t *testing.T) {
	seg := fsq07ParagraphSegment()
	row, cells := fsq07TableRowSegments()
	u1 := fsq07ParagraphUnit(seg)
	u2 := fsq07TableRowUnit(row, cells)

	chunks, err := ComposeRAGChunks(RAGComposeInput{
		FeedUnits:  []FeedUnit{u1, u2},
		Sources:    fsq07Sources(),
		Segments:   append([]SourceSegment{seg, row}, cells...),
		Profile:    fsq07Profile(),
		BaseConfig: fsq07BaseConfig(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var hits []ChunkMetadata
	for _, c := range chunks {
		if strings.Contains(c.ChunkText, "Programme Lancement") {
			hits = append(hits, c)
		}
	}
	if len(hits) == 0 {
		t.Fatal("substring 'Programme Lancement' not found in any composed chunk")
	}
	for _, h := range hits {
		if h.ContextSourceRole != "canonical" {
			t.Fatalf("hit chunk %q has ContextSourceRole %q, want 'canonical'",
				h.ChunkID, h.ContextSourceRole)
		}
	}
}

//  10. Matrix-derived units (no SourceSegmentID) are silently skipped, not
//     errored. They keep going through Enrich/EnrichBatch in the legacy path.
func TestComposeRAGChunks_SkipsMatrixDerived(t *testing.T) {
	seg := fsq07ParagraphSegment()
	good := fsq07ParagraphUnit(seg)
	matrix := FeedUnit{
		UnitID:       "RBOK-MATRIX-ONLY",
		Name:         "Matrix-only",
		Domain:       "rbok",
		BusinessRule: "Some matrix-derived rule.",
		// SourceSegmentID intentionally empty.
	}

	chunks, err := ComposeRAGChunks(RAGComposeInput{
		FeedUnits:  []FeedUnit{good, matrix},
		Sources:    fsq07Sources(),
		Segments:   []SourceSegment{seg},
		Profile:    fsq07Profile(),
		BaseConfig: fsq07BaseConfig(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk (matrix-derived skipped), got %d", len(chunks))
	}
	if chunks[0].UnitIDs[0] != good.UnitID {
		t.Fatalf("expected matrix-derived skipped, got %q", chunks[0].UnitIDs[0])
	}
}

//  11. SFI-06 BuildRAGMetadata rejection rules are unchanged: the existing
//     RAG_CHUNK_NO_SEGMENT path keeps rejecting matrix-derived units when
//     fed through the legacy entry point. ComposeRAGChunks is additive.
func TestComposeRAGChunks_DoesNotAffectBuildRAGMetadata(t *testing.T) {
	matrix := FeedUnit{UnitID: "M", Domain: "rbok", BusinessRule: "x"}
	in := RAGBuildInput{
		Unit: matrix, Content: "x", Domain: "rbok",
		Priority: "primary", Status: "active", Confidence: "medium",
	}
	_, err := BuildRAGMetadata([]RAGBuildInput{in}, map[string]SourceSegment{}, EnrichConfig{})
	if err == nil {
		t.Fatal("BuildRAGMetadata must still reject matrix-derived units")
	}
	assertComposeRejection(t, err, RAGChunkNoSegment)
}

func assertComposeRejection(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected RAGRejection with code %s, got nil", code)
	}
	var rej *RAGRejection
	if errors.As(err, &rej) {
		if rej.Code != code {
			t.Fatalf("expected rejection code %s, got %s (%v)", code, rej.Code, err)
		}
		return
	}
	if !strings.Contains(err.Error(), code) {
		t.Fatalf("expected error to contain %s, got %v", code, err)
	}
}
