package corpus

import (
	"strings"
	"testing"
)

// FSQ-03 (#366): tables emit row-level canonical_atom segments (with
// RowCanonicalText / ColumnHeaders / RowIndex / TableID) and downgrade
// every table_cell to coverage_only. Metadata tables (Field/Champ vs
// Value/Valeur) stay coverage_only end to end.

func TestFSQ03_PlainDataTable_RowCanonicalText(t *testing.T) {
	t.Parallel()
	content := "" +
		"| Offre | Prix | Durée |\n" +
		"| --- | --- | --- |\n" +
		"| Programme Lancement | 980 CHF | 3 mois |\n" +
		"| Programme Avancé | 1980 CHF | 6 mois |\n"
	segs := scanOK(t, content)
	assertCoverageAndIntegrity(t, content, segs)

	// Every table_cell must be coverage_only.
	cellCount := 0
	for _, s := range segs {
		if s.Kind != KindTableCell {
			continue
		}
		cellCount++
		if s.Disposition != DispositionCoverageOnly {
			t.Fatalf("table_cell %q must be coverage_only, got %s",
				s.SegmentID, s.Disposition)
		}
	}
	if cellCount != 9 {
		t.Fatalf("expected 9 table_cell segments (3 cols × 3 rows), got %d", cellCount)
	}

	// Header row is structure_only; both data rows are canonical_atom.
	var headerSeen bool
	var dataRows []SourceSegment
	for _, s := range segs {
		if s.Kind != KindTableRow {
			continue
		}
		// Header rows live under table_header, not as KindTableRow, but
		// guard regardless: only data rows reach KindTableRow.
		dataRows = append(dataRows, s)
	}
	if len(dataRows) != 2 {
		t.Fatalf("expected 2 data table_row segments, got %d", len(dataRows))
	}

	for _, s := range segs {
		if s.Kind == KindTableHeader {
			headerSeen = true
			if s.Disposition != DispositionStructureOnly {
				t.Fatalf("table_header must be structure_only, got %s", s.Disposition)
			}
		}
	}
	if !headerSeen {
		t.Fatal("expected a table_header segment")
	}

	wantTexts := []string{
		"Offre=Programme Lancement; Prix=980 CHF; Durée=3 mois",
		"Offre=Programme Avancé; Prix=1980 CHF; Durée=6 mois",
	}
	for i, row := range dataRows {
		if row.Disposition != DispositionCanonicalAtom {
			t.Fatalf("data row %d must be canonical_atom, got %s", i, row.Disposition)
		}
		if row.RowCanonicalText != wantTexts[i] {
			t.Fatalf("row %d RowCanonicalText = %q, want %q",
				i, row.RowCanonicalText, wantTexts[i])
		}
		if row.RowIndex != i {
			t.Fatalf("row %d RowIndex = %d, want %d", i, row.RowIndex, i)
		}
		if got := row.ColumnHeaders; len(got) != 3 ||
			got[0] != "Offre" || got[1] != "Prix" || got[2] != "Durée" {
			t.Fatalf("row %d ColumnHeaders = %v, want [Offre Prix Durée]", i, got)
		}
		if row.TableID == "" {
			t.Fatalf("row %d missing TableID", i)
		}
		if row.TableRole != "" {
			t.Fatalf("row %d TableRole = %q, want empty (non-metadata table)", i, row.TableRole)
		}
		if row.NormalizedTextHash == "" {
			t.Fatalf("row %d canonical_atom must carry NormalizedTextHash", i)
		}
	}

	// Both rows share the same TableID.
	if dataRows[0].TableID != dataRows[1].TableID {
		t.Fatalf("rows in same table must share TableID: %q vs %q",
			dataRows[0].TableID, dataRows[1].TableID)
	}
}

func TestFSQ03_MetadataTable_FrenchHeaders(t *testing.T) {
	t.Parallel()
	content := "" +
		"| Champ | Valeur |\n" +
		"| --- | --- |\n" +
		"| Auteur | Eric |\n" +
		"| Version | 1.2 |\n"
	segs := scanOK(t, content)
	assertCoverageAndIntegrity(t, content, segs)

	tableSeen := false
	for _, s := range segs {
		switch s.Kind {
		case KindTable, KindTableHeader, KindTableRow:
			if s.TableRole != "metadata_table" {
				t.Fatalf("%s segment %q must carry TableRole=metadata_table, got %q",
					s.Kind, s.SegmentID, s.TableRole)
			}
			if s.Kind == KindTable {
				tableSeen = true
			}
		}
		if s.Kind == KindTableRow && s.Disposition != DispositionCoverageOnly {
			t.Fatalf("metadata-table row %q must be coverage_only, got %s",
				s.SegmentID, s.Disposition)
		}
	}
	if !tableSeen {
		t.Fatal("expected a table container segment")
	}
}

func TestFSQ03_MetadataTable_EnglishHeaders(t *testing.T) {
	t.Parallel()
	content := "" +
		"| Field | Value |\n" +
		"| --- | --- |\n" +
		"| Author | Eric |\n"
	segs := scanOK(t, content)
	assertCoverageAndIntegrity(t, content, segs)

	for _, s := range segs {
		if s.Kind == KindTableRow && s.Disposition != DispositionCoverageOnly {
			t.Fatalf("Field/Value table row must be coverage_only, got %s", s.Disposition)
		}
	}
}

func TestFSQ03_TableRow_EmptyCellsCollapsed(t *testing.T) {
	t.Parallel()
	content := "" +
		"| A | B | C |\n" +
		"| --- | --- | --- |\n" +
		"| x |   | z |\n"
	segs := scanOK(t, content)
	assertCoverageAndIntegrity(t, content, segs)

	var row SourceSegment
	for _, s := range segs {
		if s.Kind == KindTableRow {
			row = s
			break
		}
	}
	if row.SegmentID == "" {
		t.Fatal("expected a table_row data segment")
	}
	if row.Disposition != DispositionCanonicalAtom {
		t.Fatalf("expected canonical_atom row, got %s", row.Disposition)
	}
	want := "A=x; C=z"
	if row.RowCanonicalText != want {
		t.Fatalf("RowCanonicalText = %q, want %q (empty cell B dropped)",
			row.RowCanonicalText, want)
	}
}

func TestFSQ03_LedgerPreservedCellsStillEmitted(t *testing.T) {
	t.Parallel()
	content := "" +
		"| Offre | Prix |\n" +
		"| --- | --- |\n" +
		"| Programme Lancement | 980 CHF |\n"
	segs := scanOK(t, content)
	assertCoverageAndIntegrity(t, content, segs)

	// 1 table + 1 table_header + 1 table_separator + 1 data row + 4 cells
	// (2 in header, 2 in data row).
	kinds := collectKinds(segs)
	if kinds[KindTableCell] != 4 {
		t.Fatalf("ledger fidelity: expected 4 table_cell segments, got %d", kinds[KindTableCell])
	}
	if kinds[KindTableHeader] != 1 {
		t.Fatalf("ledger fidelity: expected 1 table_header, got %d", kinds[KindTableHeader])
	}
	if kinds[KindTableRow] != 1 {
		t.Fatalf("ledger fidelity: expected 1 table_row data segment, got %d", kinds[KindTableRow])
	}
	if kinds[KindTableSeparator] != 1 {
		t.Fatalf("ledger fidelity: expected 1 table_separator, got %d", kinds[KindTableSeparator])
	}

	// Every cell is coverage_only AND every cell still has a parent row /
	// header it belongs to.
	rowsByID := map[string]SourceSegment{}
	for _, s := range segs {
		if s.Kind == KindTableRow || s.Kind == KindTableHeader {
			rowsByID[s.SegmentID] = s
		}
	}
	for _, s := range segs {
		if s.Kind != KindTableCell {
			continue
		}
		if s.Disposition != DispositionCoverageOnly {
			t.Fatalf("cell %q disposition = %s, want coverage_only",
				s.SegmentID, s.Disposition)
		}
		if _, ok := rowsByID[s.ParentSegmentID]; !ok {
			t.Fatalf("cell %q parent %q is not a table_row/table_header",
				s.SegmentID, s.ParentSegmentID)
		}
	}
}

func TestFSQ03_ExtractMarkdownUnits_RowYieldsHeadingUnit(t *testing.T) {
	t.Parallel()
	content := []byte("# Catalogue\n\n" +
		"| Offre | Prix |\n" +
		"| --- | --- |\n" +
		"| Programme Lancement | 980 CHF |\n")

	units, err := extractFromBytes("docs/catalogue.md", content)
	if err != nil {
		t.Fatalf("extractFromBytes: %v", err)
	}

	var rowUnits []HeadingUnit
	var cellUnits []HeadingUnit
	for _, u := range units {
		switch u.Kind {
		case KindTableRow:
			rowUnits = append(rowUnits, u)
		case KindTableCell:
			cellUnits = append(cellUnits, u)
		}
	}
	if len(cellUnits) != 0 {
		t.Fatalf("expected 0 table_cell HeadingUnits, got %d", len(cellUnits))
	}
	if len(rowUnits) != 1 {
		t.Fatalf("expected 1 table_row HeadingUnit, got %d", len(rowUnits))
	}
	u := rowUnits[0]
	if u.Content != "Offre=Programme Lancement; Prix=980 CHF" {
		t.Fatalf("row HeadingUnit Content = %q", u.Content)
	}
	if u.RowIndex != 0 {
		t.Fatalf("row HeadingUnit RowIndex = %d, want 0", u.RowIndex)
	}
	if len(u.ColumnHeaders) != 2 || u.ColumnHeaders[0] != "Offre" {
		t.Fatalf("row HeadingUnit ColumnHeaders = %v", u.ColumnHeaders)
	}
	if u.TableID == "" {
		t.Fatal("row HeadingUnit missing TableID")
	}
	if !strings.Contains(strings.Join(u.HeadingPath, "/"), "Catalogue") {
		t.Fatalf("row HeadingUnit HeadingPath = %v, want contains 'Catalogue'", u.HeadingPath)
	}
}

func TestFSQ03_ExtractMarkdownUnits_MetadataTableProducesNoUnits(t *testing.T) {
	t.Parallel()
	content := []byte("# Doc\n\n" +
		"| Champ | Valeur |\n" +
		"| --- | --- |\n" +
		"| Auteur | Eric |\n" +
		"| Version | 1.2 |\n")

	units, err := extractFromBytes("docs/doc.md", content)
	if err != nil {
		t.Fatalf("extractFromBytes: %v", err)
	}
	for _, u := range units {
		if u.Kind == KindTableRow || u.Kind == KindTableCell {
			t.Fatalf("metadata table must not surface table units; got %+v", u)
		}
	}
}
