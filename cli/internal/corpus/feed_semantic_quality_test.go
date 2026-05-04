package corpus

import (
	"encoding/json"
	"sort"
	"testing"
)

// FSQ-06 (#369) — semantic feed quality gate tests.
//
// Each negative case isolates one rule by hand-crafting the minimal
// inputs that trip it. The clean-fixture and severity-mix tests exercise
// the gate's status semantics and deterministic sort order.

const sqgSourceID = "SRC-SQG"
const sqgSourcePath = "docs/sqg.md"

// sqgValidUnit returns a fully-populated source-derived feed unit that
// passes every FSQ-06 rule under DefaultRBOKProfile(). Tests mutate one
// field per case to isolate the rule under test.
func sqgValidUnit() FeedUnit {
	return FeedUnit{
		UnitID:             "RBOK-MD-SQG-RULE-A",
		Name:               "Rule A",
		Domain:             "rbok",
		UnitType:           "rule",
		BusinessRule:       "The system must validate all source bytes before producing a feed unit.",
		SourceIDs:          []string{sqgSourceID},
		SourceSegmentID:    "seg:SRC-SQG:10-90:paragraph",
		SourceID:           sqgSourceID,
		SourcePath:         sqgSourcePath,
		StartByte:          10,
		EndByte:            90,
		StartLine:          3,
		EndLine:            3,
		NormalizedTextHash: "norm-rule-a",
		HeadingPath:        []string{"Rule A"},
	}
}

func sqgUnitOfKind(kind, text, hash string, line int) FeedUnit {
	u := sqgValidUnit()
	u.UnitID = "RBOK-MD-SQG-" + kind + "-L" + itoa(line)
	u.BusinessRule = text
	u.NormalizedTextHash = hash
	u.SourceSegmentID = "seg:SRC-SQG:" + itoa(line*100) + "-" + itoa(line*100+len(text)) + ":" + kind
	u.StartByte = line * 100
	u.EndByte = line*100 + len(text)
	u.StartLine = line
	u.EndLine = line
	return u
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	if neg {
		return "-" + string(d)
	}
	return string(d)
}

func sqgFindingByCode(report SemanticQualityReport, code string) (SemanticQualityFinding, bool) {
	for _, f := range report.Findings {
		if f.Code == code {
			return f, true
		}
	}
	return SemanticQualityFinding{}, false
}

// ----------------------------------------------------------------------------
// 1. One-token chunk → FEED_UNIT_BELOW_TOKEN_MIN blocking.
// ----------------------------------------------------------------------------

func TestCheckSemanticQuality_OneTokenChunkIsBelowTokenMin(t *testing.T) {
	t.Parallel()
	u := sqgUnitOfKind("paragraph", "x", "norm-x", 1)
	report := CheckSemanticQuality(SemanticQualityInput{Feed: []FeedUnit{u}})
	if report.Status != "fail" {
		t.Fatalf("expected status=fail, got %q (%+v)", report.Status, report.Findings)
	}
	if _, ok := sqgFindingByCode(report, FindingFeedUnitBelowTokenMin); !ok {
		t.Fatalf("expected %s, got %+v", FindingFeedUnitBelowTokenMin, report.Findings)
	}
	if report.BlockingFindingCount == 0 {
		t.Fatal("expected at least one blocking finding")
	}
}

// ----------------------------------------------------------------------------
// 2. Stop-label "Champ" → FEED_STOP_LABEL blocking.
// ----------------------------------------------------------------------------

func TestCheckSemanticQuality_StopLabelChamp(t *testing.T) {
	t.Parallel()
	u := sqgUnitOfKind("table_cell", "Champ", "norm-champ", 4)
	u.TableID = "T1"
	u.ColumnHeaders = []string{"Champ", "Valeur"}
	report := CheckSemanticQuality(SemanticQualityInput{Feed: []FeedUnit{u}})
	if _, ok := sqgFindingByCode(report, FindingFeedStopLabel); !ok {
		t.Fatalf("expected %s, got %+v", FindingFeedStopLabel, report.Findings)
	}
	if report.Status != "fail" {
		t.Fatalf("expected status=fail, got %q", report.Status)
	}
}

// ----------------------------------------------------------------------------
// 3. Duplicate normalized text 3× → blocking.
// ----------------------------------------------------------------------------

func TestCheckSemanticQuality_DuplicateThree_Blocking(t *testing.T) {
	t.Parallel()
	feed := []FeedUnit{
		sqgUnitOfKind("paragraph", "Identical statement that appears three times across the feed.", "norm-dup", 1),
		sqgUnitOfKind("paragraph", "Identical statement that appears three times across the feed.", "norm-dup", 2),
		sqgUnitOfKind("paragraph", "Identical statement that appears three times across the feed.", "norm-dup", 3),
	}
	report := CheckSemanticQuality(SemanticQualityInput{Feed: feed})
	if report.Status != "fail" {
		t.Fatalf("expected status=fail, got %q (%+v)", report.Status, report.Findings)
	}
	f, ok := sqgFindingByCode(report, FindingFeedDuplicateNormalizedText)
	if !ok {
		t.Fatalf("expected %s, got %+v", FindingFeedDuplicateNormalizedText, report.Findings)
	}
	if f.Severity != SemanticSeverityBlocking {
		t.Fatalf("expected blocking severity, got %q", f.Severity)
	}
}

// ----------------------------------------------------------------------------
// 4. Duplicate normalized text 2× → warning.
// ----------------------------------------------------------------------------

func TestCheckSemanticQuality_DuplicateTwo_Warning(t *testing.T) {
	t.Parallel()
	feed := []FeedUnit{
		sqgUnitOfKind("paragraph", "Statement that appears twice across the curated feed.", "norm-dup-2x", 1),
		sqgUnitOfKind("paragraph", "Statement that appears twice across the curated feed.", "norm-dup-2x", 2),
	}
	report := CheckSemanticQuality(SemanticQualityInput{Feed: feed})
	if report.Status != "warn" {
		t.Fatalf("expected status=warn, got %q (%+v)", report.Status, report.Findings)
	}
	f, ok := sqgFindingByCode(report, FindingFeedDuplicateNormalizedText)
	if !ok {
		t.Fatalf("expected %s, got %+v", FindingFeedDuplicateNormalizedText, report.Findings)
	}
	if f.Severity != SemanticSeverityWarning {
		t.Fatalf("expected warning severity, got %q", f.Severity)
	}
	if report.WarningFindingCount == 0 {
		t.Fatal("expected at least one warning")
	}
}

// ----------------------------------------------------------------------------
// 5. Bare table_cell unit → FEED_TABLE_CELL_NOT_ROW_CONTEXT blocking.
// ----------------------------------------------------------------------------

func TestCheckSemanticQuality_BareTableCellMissingRowContext(t *testing.T) {
	t.Parallel()
	// Use a non-stop-label, long-enough text so other rules don't fire.
	u := sqgUnitOfKind("table_cell", "Free-form value text that would otherwise pass.", "norm-cell", 6)
	report := CheckSemanticQuality(SemanticQualityInput{Feed: []FeedUnit{u}})
	if _, ok := sqgFindingByCode(report, FindingFeedTableCellNotRowContext); !ok {
		t.Fatalf("expected %s, got %+v", FindingFeedTableCellNotRowContext, report.Findings)
	}
	if report.Status != "fail" {
		t.Fatalf("expected status=fail, got %q", report.Status)
	}
}

// ----------------------------------------------------------------------------
// 6. metadata_table TableRole → FEED_METADATA_TABLE_LEAKED blocking.
// ----------------------------------------------------------------------------

func TestCheckSemanticQuality_MetadataTableLeak(t *testing.T) {
	t.Parallel()
	u := sqgUnitOfKind("table_row", "Document version 1.0", "norm-meta", 7)
	u.TableID = "T-meta"
	u.TableRole = "metadata_table"
	u.ColumnHeaders = []string{"Field", "Value"}
	report := CheckSemanticQuality(SemanticQualityInput{Feed: []FeedUnit{u}})
	if _, ok := sqgFindingByCode(report, FindingFeedMetadataTableLeaked); !ok {
		t.Fatalf("expected %s, got %+v", FindingFeedMetadataTableLeaked, report.Findings)
	}
	if report.Status != "fail" {
		t.Fatalf("expected status=fail, got %q", report.Status)
	}
}

// ----------------------------------------------------------------------------
// 7. YAML raw vs decoded mismatch → FEED_RAW_DECODED_MISMATCH informational.
// ----------------------------------------------------------------------------

func TestCheckSemanticQuality_YAMLRawDecodedMismatch(t *testing.T) {
	t.Parallel()
	u := sqgValidUnit()
	u.RawText = "\"yes please\""
	u.DecodedValue = "yes please"
	u.BusinessRuleMode = "decoded"
	u.YAMLPath = "parcours.modules[0].help_text"
	report := CheckSemanticQuality(SemanticQualityInput{Feed: []FeedUnit{u}})
	f, ok := sqgFindingByCode(report, FindingFeedRawDecodedMismatch)
	if !ok {
		t.Fatalf("expected %s, got %+v", FindingFeedRawDecodedMismatch, report.Findings)
	}
	if f.Severity != SemanticSeverityInformational {
		t.Fatalf("expected informational, got %q", f.Severity)
	}
	// Default profile: informational findings do not flip the status.
	if report.Status != "pass" {
		t.Fatalf("expected status=pass with informational only, got %q", report.Status)
	}
	if report.InformationalFindingCount == 0 {
		t.Fatal("expected at least one informational finding")
	}
}

// ----------------------------------------------------------------------------
// 8. Admitted+atomized source with zero units and empty reason → blocking.
// ----------------------------------------------------------------------------

func TestCheckSemanticQuality_ZeroUnitNoReason(t *testing.T) {
	t.Parallel()
	src := FeedSource{
		ID:                "SRC-ORPHAN",
		Path:              "docs/orphan.md",
		Status:            "active",
		AdmissionStatus:   AdmissionAdmitted,
		AtomizationStatus: AtomizationAtomized,
		// ExclusionReason intentionally empty.
	}
	report := CheckSemanticQuality(SemanticQualityInput{Sources: []FeedSource{src}})
	f, ok := sqgFindingByCode(report, FindingSourceZeroUnitNoReason)
	if !ok {
		t.Fatalf("expected %s, got %+v", FindingSourceZeroUnitNoReason, report.Findings)
	}
	if f.Severity != SemanticSeverityBlocking {
		t.Fatalf("expected blocking, got %q", f.Severity)
	}
	if report.Status != "fail" {
		t.Fatalf("expected status=fail, got %q", report.Status)
	}
}

// ----------------------------------------------------------------------------
// 9. Clean RBOK-style fixture → pass with zero blocking and zero warning.
// ----------------------------------------------------------------------------

func TestCheckSemanticQuality_CleanFixture(t *testing.T) {
	t.Parallel()
	feed := []FeedUnit{
		sqgUnitOfKind("paragraph",
			"First paragraph that is clearly long enough and unique in normalized form.",
			"norm-p1", 1),
		sqgUnitOfKind("paragraph",
			"Second distinct paragraph with sufficient tokens and characters.",
			"norm-p2", 2),
		sqgUnitOfKind("list_item",
			"List item with enough content.",
			"norm-li1", 3),
	}
	src := FeedSource{
		ID:                sqgSourceID,
		Path:              sqgSourcePath,
		Status:            "active",
		AdmissionStatus:   AdmissionAdmitted,
		AtomizationStatus: AtomizationAtomized,
	}
	report := CheckSemanticQuality(SemanticQualityInput{
		Feed:    feed,
		Sources: []FeedSource{src},
	})
	if report.Status != "pass" {
		t.Fatalf("expected status=pass on clean fixture, got %q (%+v)", report.Status, report.Findings)
	}
	if report.BlockingFindingCount != 0 || report.WarningFindingCount != 0 {
		t.Fatalf("expected zero blocking/warning, got blocking=%d warning=%d info=%d",
			report.BlockingFindingCount, report.WarningFindingCount, report.InformationalFindingCount)
	}
}

// ----------------------------------------------------------------------------
// 10. Severity sort + status semantics: a mix of blocking + warning +
//     informational → status=fail, counters correct, deterministic order.
// ----------------------------------------------------------------------------

func TestCheckSemanticQuality_SeveritySortAndStatus(t *testing.T) {
	t.Parallel()
	feed := []FeedUnit{
		// Blocking: one-token paragraph.
		sqgUnitOfKind("paragraph", "x", "norm-x", 1),
		// Warning: 2× duplicate normalized text.
		sqgUnitOfKind("paragraph", "Repeated phrase used as a warning trigger.", "norm-dup-warn", 2),
		sqgUnitOfKind("paragraph", "Repeated phrase used as a warning trigger.", "norm-dup-warn", 3),
		// Informational: YAML raw/decoded mismatch.
		func() FeedUnit {
			u := sqgValidUnit()
			u.UnitID = "U-YAML"
			u.NormalizedTextHash = "norm-yaml"
			u.RawText = "\"hello\""
			u.DecodedValue = "hello"
			u.BusinessRuleMode = "decoded"
			return u
		}(),
	}
	report := CheckSemanticQuality(SemanticQualityInput{Feed: feed})
	if report.Status != "fail" {
		t.Fatalf("expected fail, got %q", report.Status)
	}
	if report.BlockingFindingCount == 0 {
		t.Fatal("expected ≥1 blocking")
	}
	if report.WarningFindingCount == 0 {
		t.Fatal("expected ≥1 warning")
	}
	if report.InformationalFindingCount == 0 {
		t.Fatal("expected ≥1 informational")
	}

	// Findings must be sorted: severity DESC (blocking first), then code ASC.
	prev := -1
	for _, f := range report.Findings {
		rank := severityRank(f.Severity)
		if rank < prev {
			t.Fatalf("findings not sorted by severity DESC: %+v", report.Findings)
		}
		prev = rank
	}

	// Same input → same finding order across runs (determinism).
	a := CheckSemanticQuality(SemanticQualityInput{Feed: feed}).Findings
	b := CheckSemanticQuality(SemanticQualityInput{Feed: feed}).Findings
	if !sameFindingOrder(a, b) {
		t.Fatal("non-deterministic finding order across runs")
	}

	// Report serialises cleanly.
	if _, err := json.Marshal(report); err != nil {
		t.Fatalf("marshal: %v", err)
	}
}

func sameFindingOrder(a, b []SemanticQualityFinding) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Code != b[i].Code || a[i].UnitID != b[i].UnitID || a[i].Severity != b[i].Severity {
			return false
		}
	}
	return true
}

// ----------------------------------------------------------------------------
// 11. Profile override: AllowTableCellWithoutRow=true → bare cell no longer fails.
// ----------------------------------------------------------------------------

func TestCheckSemanticQuality_ProfileOverride_AllowsBareTableCell(t *testing.T) {
	t.Parallel()
	u := sqgUnitOfKind("table_cell", "Bare cell text without row context plumbing.", "norm-cell-allow", 8)
	profile := DefaultRBOKProfile()
	profile.AllowTableCellWithoutRow = true
	report := CheckSemanticQuality(SemanticQualityInput{
		Feed:    []FeedUnit{u},
		Profile: profile,
	})
	if _, found := sqgFindingByCode(report, FindingFeedTableCellNotRowContext); found {
		t.Fatalf("expected NO %s under override, got %+v", FindingFeedTableCellNotRowContext, report.Findings)
	}
	// The raised denylist might not match this benign content, and the cell
	// is long enough → status should be pass.
	if report.Status != "pass" {
		t.Fatalf("expected status=pass under permissive profile, got %q (%+v)", report.Status, report.Findings)
	}
}

// ----------------------------------------------------------------------------
// Bonus determinism: re-emitting the same severity-sorted slice via a stable
// secondary sort by code and unit_id matches what the gate emitted.
// ----------------------------------------------------------------------------

func TestCheckSemanticQuality_FindingOrderingIsStable(t *testing.T) {
	t.Parallel()
	feed := []FeedUnit{
		sqgUnitOfKind("paragraph", "x", "norm-x", 1),
		sqgUnitOfKind("paragraph", "y", "norm-y", 2),
	}
	report := CheckSemanticQuality(SemanticQualityInput{Feed: feed})
	got := append([]SemanticQualityFinding{}, report.Findings...)
	sort.SliceStable(got, func(i, j int) bool {
		return semanticFindingLess(got[i], got[j])
	})
	if !sameFindingOrder(got, report.Findings) {
		t.Fatal("re-applying the comparator changed the order — not stable")
	}
}
