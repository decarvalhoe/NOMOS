package corpus

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Markdown segment kinds emitted by ScanMarkdown. They match the kinds
// enumerated in SFI-02 (#340).
const (
	KindHeading             = "heading"
	KindParagraph           = "paragraph"
	KindBlank               = "blank"
	KindDecorativeSeparator = "decorative_separator"
	KindList                = "list"
	KindListItem            = "list_item"
	KindTable               = "table"
	KindTableHeader         = "table_header"
	KindTableSeparator      = "table_separator"
	KindTableRow            = "table_row"
	KindTableCell           = "table_cell"
	KindBlockquote          = "blockquote"
	KindCallout             = "callout"
	KindCodeBlock           = "code_block"
	KindLink                = "link"
	KindImageRef            = "image_ref"
	KindMetadata            = "metadata"
	KindUnsupportedBlock    = "unsupported_block"
	KindYAMLScalar          = "yaml_scalar"
	KindJSONScalar          = "json_scalar"
	KindStructuredCoverage  = "structured_coverage"
)

// ScanMarkdown walks the given Markdown content once and emits a typed
// SourceSegment ledger covering every byte of the input. The scanner is a
// generic, line-oriented Markdown reader; no third-party dependencies are
// added. It is the sole entry point for SFI-02 (#340).
//
// Coverage invariant: the union of root-level segments (those with empty
// ParentSegmentID) covers exactly [0, len(content)) without overlap. Child
// segments are nested under their parent via ParentSegmentID and may share
// or sit inside the parent's byte range.
func ScanMarkdown(sourceID, sourcePath string, content []byte) ([]SourceSegment, error) {
	if strings.TrimSpace(sourceID) == "" {
		return nil, fmt.Errorf("scan markdown: sourceID required")
	}
	if strings.TrimSpace(sourcePath) == "" {
		return nil, fmt.Errorf("scan markdown: sourcePath required")
	}
	s := &mdScanner{
		sourceID:   sourceID,
		sourcePath: sourcePath,
		content:    content,
		lines:      splitMDLines(content),
	}
	return s.scan()
}

// mdLine captures one source line and its byte/line span in the document.
// startB is inclusive; endB is exclusive and includes the trailing newline
// (when present).
type mdLine struct {
	text       string
	startB     int
	endB       int
	lineNum    int
	contentLen int
}

// splitMDLines partitions the input into newline-delimited lines while
// preserving exact byte offsets. The final line need not end in '\n'.
func splitMDLines(content []byte) []mdLine {
	if len(content) == 0 {
		return nil
	}
	var lines []mdLine
	pos := 0
	lineNum := 1
	for pos < len(content) {
		nl := bytes.IndexByte(content[pos:], '\n')
		var endB int
		var text string
		if nl == -1 {
			endB = len(content)
			text = string(content[pos:endB])
		} else {
			endB = pos + nl + 1
			text = string(content[pos : pos+nl])
		}
		lines = append(lines, mdLine{
			text:       text,
			startB:     pos,
			endB:       endB,
			lineNum:    lineNum,
			contentLen: len(text),
		})
		pos = endB
		lineNum++
	}
	return lines
}

type mdScanner struct {
	sourceID   string
	sourcePath string
	content    []byte
	lines      []mdLine
	out        []SourceSegment
}

// Pre-compiled detection regexes. Pure stdlib, no third-party deps.
// All names are prefixed mdScan* to avoid collision with other regex
// vars already living in the corpus package.
var (
	mdScanHeadingRe        = regexp.MustCompile(`^(#{1,6})\s+`)
	mdScanListItemRe       = regexp.MustCompile(`^(\s*)([-*+]|\d+[.)])\s+`)
	mdScanTableSeparatorRe = regexp.MustCompile(`^\s*\|?\s*:?-{3,}:?\s*(\|\s*:?-{3,}:?\s*)*\|?\s*$`)
	mdScanBlockquoteRe     = regexp.MustCompile(`^\s*>`)
	mdScanBlockquoteStrip  = regexp.MustCompile(`^\s*>\s?`)
	mdScanCalloutPrefixRe  = regexp.MustCompile(`^\s*>\s*\[!([A-Za-z]+)\]`)
	mdScanInlineRefRe      = regexp.MustCompile(`(!?)\[([^\]\n]*)\]\(([^)\n]*)\)`)
	mdScanTOCOnlyLinkRe    = regexp.MustCompile(`^\s*(?:\d+[.)]\s*)?\[[^\]\n]+\]\(\s*#[^)]+\)\s*$`)
	mdScanStrongLabelRe    = regexp.MustCompile(`^\s*(?:\*\*|__)[^*_]{1,120}(?:\*\*|__)\s*:?\s*$`)
	mdScanPlaceholderRe    = regexp.MustCompile(`^\s*(?:\*\*)?\[[^\]\n]+\](?:\*\*)?\s*(?:[-—–:])?\s*$`)
	mdScanChapterRefRe     = regexp.MustCompile(`(?i)^\s*(chapitre|chapter)\s+\d+\s*[-—–]`)
)

// decorativeChars enumerates the punctuation characters that may compose a
// decorative separator line ("---", "***", "___", "...", "~~~~", etc.).
const decorativeChars = "*-_.~+ \t"

func (s *mdScanner) scan() ([]SourceSegment, error) {
	i := 0
	if next := s.tryEmitFrontMatter(); next > 0 {
		i = next
	}
	for i < len(s.lines) {
		line := s.lines[i].text
		switch {
		case isHeadingLine(line):
			s.emitHeading(i)
			i++
		case isFenceStart(line):
			i = s.emitFencedCodeBlock(i)
		case isHTMLBlockStart(line):
			i = s.emitHTMLBlock(i)
		case isBlankLine(line):
			s.emitBlank(i)
			i++
		case s.isTableStart(i):
			i = s.emitTable(i)
		case isBlockquoteLine(line):
			i = s.emitBlockquote(i)
		case isDecorativeSeparator(line):
			s.emitDecorativeSeparator(i)
			i++
		case isListItemLine(line):
			i = s.emitList(i)
		default:
			i = s.emitParagraph(i)
		}
	}
	return s.out, nil
}

// segmentID returns a deterministic id for a segment, encoding sourceID,
// byte range, and kind. Two distinct (range, kind) pairs always yield
// distinct ids; the same input always yields the same id.
func segmentID(sourceID string, startByte, endByte int, kind string) string {
	return fmt.Sprintf("seg:%s:%d-%d:%s", sourceID, startByte, endByte, kind)
}

func (s *mdScanner) makeBlockSegment(startIdx, endIdx int, kind, parentID string) SourceSegment {
	sl := s.lines[startIdx]
	el := s.lines[endIdx]
	return SourceSegment{
		SegmentID:       segmentID(s.sourceID, sl.startB, el.endB, kind),
		SourceID:        s.sourceID,
		SourcePath:      s.sourcePath,
		Kind:            kind,
		StartByte:       sl.startB,
		EndByte:         el.endB,
		StartLine:       sl.lineNum,
		StartColumn:     1,
		EndLine:         el.lineNum,
		EndColumn:       el.contentLen + 1,
		ParentSegmentID: parentID,
	}
}

func (s *mdScanner) append(seg SourceSegment) {
	s.out = append(s.out, seg)
}

func (s *mdScanner) lineBytes(idx int) []byte {
	return s.content[s.lines[idx].startB:s.lines[idx].endB]
}

func (s *mdScanner) sliceText(startIdx, endIdx int) string {
	return string(s.content[s.lines[startIdx].startB:s.lines[endIdx].endB])
}

// ----------------------------------------------------------------------------
// Front matter.
// ----------------------------------------------------------------------------

func (s *mdScanner) tryEmitFrontMatter() int {
	if len(s.lines) < 2 || strings.TrimSpace(s.lines[0].text) != "---" {
		return 0
	}
	for i := 1; i < len(s.lines); i++ {
		if strings.TrimSpace(s.lines[i].text) == "---" {
			seg := s.makeBlockSegment(0, i, KindMetadata, "")
			seg.Disposition = DispositionExcludedByPolicy
			seg.RawTextHash = ComputeRawTextHash([]byte(s.sliceText(0, i)))
			s.append(seg)
			return i + 1
		}
	}
	return 0
}

// ----------------------------------------------------------------------------
// Heading.
// ----------------------------------------------------------------------------

func isHeadingLine(text string) bool {
	return mdScanHeadingRe.MatchString(text)
}

func (s *mdScanner) emitHeading(idx int) {
	seg := s.makeBlockSegment(idx, idx, KindHeading, "")
	seg.Disposition = DispositionStructureOnly
	seg.RawTextHash = ComputeRawTextHash(s.lineBytes(idx))
	s.append(seg)
}

// ----------------------------------------------------------------------------
// Blank line.
// ----------------------------------------------------------------------------

func isBlankLine(text string) bool {
	return strings.TrimSpace(text) == ""
}

func (s *mdScanner) emitBlank(idx int) {
	seg := s.makeBlockSegment(idx, idx, KindBlank, "")
	seg.Disposition = DispositionCoverageOnly
	seg.RawTextHash = ComputeRawTextHash(s.lineBytes(idx))
	s.append(seg)
}

// ----------------------------------------------------------------------------
// Decorative separator (thematic break).
// ----------------------------------------------------------------------------

// isDecorativeSeparator returns true when the line contains only punctuation
// characters from decorativeChars and at least three non-whitespace symbols.
// It excludes lines containing pipes (table separator), '>' (blockquote),
// '#' (heading) or alphanumerics.
func isDecorativeSeparator(text string) bool {
	trimmed := strings.TrimSpace(text)
	if len(trimmed) < 3 {
		return false
	}
	nonWhite := 0
	for _, r := range trimmed {
		if !strings.ContainsRune(decorativeChars, r) {
			return false
		}
		if !unicode.IsSpace(r) {
			nonWhite++
		}
	}
	return nonWhite >= 3
}

func (s *mdScanner) emitDecorativeSeparator(idx int) {
	seg := s.makeBlockSegment(idx, idx, KindDecorativeSeparator, "")
	seg.Disposition = DispositionCoverageOnly
	seg.RawTextHash = ComputeRawTextHash(s.lineBytes(idx))
	s.append(seg)
}

// ----------------------------------------------------------------------------
// Fenced code block.
// ----------------------------------------------------------------------------

func isFenceStart(text string) bool {
	trimmed := strings.TrimLeft(text, " ")
	return strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")
}

func (s *mdScanner) emitFencedCodeBlock(start int) int {
	openTrim := strings.TrimLeft(s.lines[start].text, " ")
	fence := "```"
	if strings.HasPrefix(openTrim, "~~~") {
		fence = "~~~"
	}
	end := start + 1
	for end < len(s.lines) {
		t := strings.TrimLeft(s.lines[end].text, " ")
		if strings.HasPrefix(t, fence) {
			end++
			break
		}
		end++
	}
	if end > len(s.lines) {
		end = len(s.lines)
	}
	seg := s.makeBlockSegment(start, end-1, KindCodeBlock, "")
	seg.Disposition = DispositionCoverageOnly
	seg.RawTextHash = ComputeRawTextHash([]byte(s.sliceText(start, end-1)))
	s.append(seg)
	return end
}

// ----------------------------------------------------------------------------
// HTML block (treated as unsupported).
// ----------------------------------------------------------------------------

func isHTMLBlockStart(text string) bool {
	trimmed := strings.TrimLeft(text, " \t")
	if len(trimmed) < 2 || trimmed[0] != '<' {
		return false
	}
	c := trimmed[1]
	return c == '!' || c == '/' || isASCIIAlpha(c)
}

func isASCIIAlpha(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func (s *mdScanner) emitHTMLBlock(start int) int {
	end := start + 1
	for end < len(s.lines) {
		if isBlankLine(s.lines[end].text) {
			break
		}
		end++
	}
	seg := s.makeBlockSegment(start, end-1, KindUnsupportedBlock, "")
	seg.Disposition = DispositionUnsupportedBlocking
	seg.UnsupportedReason = "HTML block: typed Markdown scanner does not yet classify raw HTML"
	seg.RawTextHash = ComputeRawTextHash([]byte(s.sliceText(start, end-1)))
	s.append(seg)
	return end
}

// ----------------------------------------------------------------------------
// Tables.
// ----------------------------------------------------------------------------

func (s *mdScanner) isTableStart(i int) bool {
	if i+1 >= len(s.lines) {
		return false
	}
	header := s.lines[i].text
	separator := s.lines[i+1].text
	if !strings.Contains(header, "|") || !strings.Contains(separator, "|") {
		return false
	}
	return mdScanTableSeparatorRe.MatchString(separator)
}

func (s *mdScanner) emitTable(start int) int {
	end := start + 2
	for end < len(s.lines) {
		t := s.lines[end].text
		if isBlankLine(t) || !strings.Contains(t, "|") {
			break
		}
		end++
	}

	// FSQ-03 (#366): parse header column names once and use them to (a)
	// detect metadata-table tables and (b) build per-row canonical text
	// like "Col1=Val1; Col2=Val2; ..." on data rows.
	columnHeaders := parseTableRowCells(s.lines[start].text)
	tableRole := classifyTableRole(columnHeaders)

	tableSeg := s.makeBlockSegment(start, end-1, KindTable, "")
	tableSeg.Disposition = DispositionStructureOnly
	tableSeg.RawTextHash = ComputeRawTextHash(s.lineBytes(start))
	tableSeg.TableID = tableSeg.SegmentID
	tableSeg.TableRole = tableRole
	s.append(tableSeg)

	headerSeg := s.makeBlockSegment(start, start, KindTableHeader, tableSeg.SegmentID)
	headerSeg.Disposition = DispositionStructureOnly
	headerSeg.RawTextHash = ComputeRawTextHash(s.lineBytes(start))
	headerSeg.TableID = tableSeg.SegmentID
	headerSeg.TableRole = tableRole
	s.append(headerSeg)
	s.emitTableCells(start, headerSeg.SegmentID)

	sepSeg := s.makeBlockSegment(start+1, start+1, KindTableSeparator, tableSeg.SegmentID)
	sepSeg.Disposition = DispositionCoverageOnly
	sepSeg.RawTextHash = ComputeRawTextHash(s.lineBytes(start + 1))
	s.append(sepSeg)

	for r := start + 2; r < end; r++ {
		rowSeg := s.makeBlockSegment(r, r, KindTableRow, tableSeg.SegmentID)
		rowSeg.RawTextHash = ComputeRawTextHash(s.lineBytes(r))
		rowSeg.TableID = tableSeg.SegmentID
		rowSeg.TableRole = tableRole
		rowSeg.RowIndex = r - (start + 2)
		rowSeg.ColumnHeaders = append([]string(nil), columnHeaders...)
		rowCells := parseTableRowCells(s.lines[r].text)
		rowSeg.RowCanonicalText = buildRowCanonicalText(columnHeaders, rowCells)

		switch {
		case tableRole == "metadata_table":
			// Metadata tables describe document properties (Field/Value),
			// not retrievable doctrine. Rows stay in the ledger but do not
			// enter the feed.
			rowSeg.Disposition = DispositionCoverageOnly
		case rowSeg.RowCanonicalText != "" &&
			!isPlaceholderOnlyTableRow(rowCells) &&
			!isJunkSemantic([]byte(rowSeg.RowCanonicalText)):
			rowSeg.Disposition = DispositionCanonicalAtom
			rowSeg.NormalizedTextHash = ComputeNormalizedTextHash(rowSeg.RowCanonicalText)
			rowSeg.IncludeInFeed = true
			rowSeg.IncludeInRAG = true
		default:
			rowSeg.Disposition = DispositionCoverageOnly
		}

		s.append(rowSeg)
		s.emitTableCells(r, rowSeg.SegmentID)
	}

	return end
}

// emitTableCells splits the given line on '|' (escapes not handled) and
// emits one table_cell child segment per cell. Cells are siblings with no
// overlap; they do not need to cover the full row span (pipe glyphs are
// gaps between cells).
//
// FSQ-03 (#366): table_cell segments are always coverage_only — they remain
// in the source ledger so fidelity is preserved, but the canonical_atom
// status now lives on the parent table_row data segment. Header cells and
// metadata-table cells were already non-canonical; data cells now follow
// the same rule.
func (s *mdScanner) emitTableCells(rowIdx int, parentID string) {
	line := s.lines[rowIdx]
	text := line.text
	var pipes []int
	for j := 0; j < len(text); j++ {
		if text[j] == '|' {
			pipes = append(pipes, j)
		}
	}
	if len(pipes) == 0 {
		return
	}
	type span struct{ start, end int }
	var ranges []span
	if pipes[0] > 0 {
		ranges = append(ranges, span{0, pipes[0]})
	}
	for k := 0; k < len(pipes)-1; k++ {
		ranges = append(ranges, span{pipes[k] + 1, pipes[k+1]})
	}
	last := pipes[len(pipes)-1]
	if last < len(text) {
		if rest := strings.TrimSpace(text[last+1:]); rest != "" {
			ranges = append(ranges, span{last + 1, len(text)})
		}
	}
	for _, r := range ranges {
		cellText := text[r.start:r.end]
		startB := line.startB + r.start
		endB := line.startB + r.end
		seg := SourceSegment{
			SegmentID:       segmentID(s.sourceID, startB, endB, KindTableCell),
			SourceID:        s.sourceID,
			SourcePath:      s.sourcePath,
			Kind:            KindTableCell,
			Disposition:     DispositionCoverageOnly,
			StartByte:       startB,
			EndByte:         endB,
			StartLine:       line.lineNum,
			StartColumn:     r.start + 1,
			EndLine:         line.lineNum,
			EndColumn:       r.end + 1,
			ParentSegmentID: parentID,
			RawTextHash:     ComputeRawTextHash([]byte(cellText)),
		}
		s.append(seg)
	}
}

// parseTableRowCells splits a Markdown table row line on '|' and returns
// the trimmed cell text in document order. Leading/trailing empty cells
// (created by the line-bounding pipes) are dropped, mirroring the cell
// segments emitted by emitTableCells.
func parseTableRowCells(text string) []string {
	text = strings.TrimRight(text, "\r\n")
	var pipes []int
	for j := 0; j < len(text); j++ {
		if text[j] == '|' {
			pipes = append(pipes, j)
		}
	}
	if len(pipes) == 0 {
		return nil
	}
	var cells []string
	if pipes[0] > 0 {
		cells = append(cells, strings.TrimSpace(text[:pipes[0]]))
	}
	for k := 0; k < len(pipes)-1; k++ {
		cells = append(cells, strings.TrimSpace(text[pipes[k]+1:pipes[k+1]]))
	}
	last := pipes[len(pipes)-1]
	if last < len(text) {
		if rest := strings.TrimSpace(text[last+1:]); rest != "" {
			cells = append(cells, rest)
		}
	}
	return cells
}

// buildRowCanonicalText assembles a row's structured "Col=Val; ..." text
// using the header column names. Empty cells are skipped; if every cell is
// empty the result is "" so the caller can mark the row coverage_only.
// Headers shorter than the row produce bare "Val" entries (no "=") for
// extra columns; this is conservative for malformed tables.
func buildRowCanonicalText(headers, cells []string) string {
	var b strings.Builder
	emitted := 0
	for i, cell := range cells {
		if cell == "" {
			continue
		}
		if emitted > 0 {
			b.WriteString("; ")
		}
		if i < len(headers) && headers[i] != "" {
			b.WriteString(headers[i])
			b.WriteString("=")
		}
		b.WriteString(cell)
		emitted++
	}
	return b.String()
}

func isPlaceholderOnlyTableRow(cells []string) bool {
	nonEmpty := 0
	for _, cell := range cells {
		clean := strings.TrimSpace(cell)
		if clean == "" {
			continue
		}
		nonEmpty++
		if !isPlaceholderCell(clean) {
			return false
		}
	}
	return nonEmpty > 0
}

func isPlaceholderCell(cell string) bool {
	clean := strings.ToLower(strings.TrimSpace(cell))
	clean = strings.Trim(clean, "`*_ ")
	if clean == "" {
		return true
	}
	if strings.Trim(clean, ".…-—–_ ") == "" {
		return true
	}
	switch clean {
	case "n/a", "na", "nd", "tbd", "todo",
		"a definir", "à définir",
		"a creer", "à créer",
		"a concevoir", "à concevoir":
		return true
	default:
		return false
	}
}

// classifyTableRole tags two-column tables whose header is a
// (Field/Champ/Key/Property, Value/Valeur/Setting/Statut) pair. Such tables
// describe document properties rather than retrievable doctrine; their data
// rows stay in the ledger as coverage_only and never enter the feed.
//
// French and English labels are both recognised. The match is case-
// insensitive and trims trailing punctuation. Order is irrelevant: either
// column may be the key column.
func classifyTableRole(headers []string) string {
	if len(headers) != 2 {
		return ""
	}
	keyTerms := map[string]struct{}{
		"field": {}, "champ": {}, "key": {}, "cle": {}, "clé": {},
		"property": {}, "propriete": {}, "propriété": {},
	}
	valueTerms := map[string]struct{}{
		"value": {}, "valeur": {}, "setting": {}, "statut": {}, "status": {},
	}
	h0 := normalizeHeaderToken(headers[0])
	h1 := normalizeHeaderToken(headers[1])
	_, k0 := keyTerms[h0]
	_, k1 := keyTerms[h1]
	_, v0 := valueTerms[h0]
	_, v1 := valueTerms[h1]
	if (k0 && v1) || (k1 && v0) {
		return "metadata_table"
	}
	return ""
}

func normalizeHeaderToken(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, ":-—–.|")
	return strings.ToLower(strings.TrimSpace(s))
}

// ----------------------------------------------------------------------------
// Blockquote and callout.
// ----------------------------------------------------------------------------

func isBlockquoteLine(text string) bool {
	return mdScanBlockquoteRe.MatchString(text)
}

func (s *mdScanner) emitBlockquote(start int) int {
	end := start
	for end < len(s.lines) && isBlockquoteLine(s.lines[end].text) {
		end++
	}

	if mdScanCalloutPrefixRe.MatchString(s.lines[start].text) {
		seg := s.makeBlockSegment(start, end-1, KindCallout, "")
		raw := []byte(s.sliceText(start, end-1))
		seg.RawTextHash = ComputeRawTextHash(raw)
		if canonical, ok := canonicalMarkdownAtomText(stripBlockquoteMarkers(string(raw))); ok {
			seg.Disposition = DispositionCanonicalAtom
			seg.NormalizedTextHash = ComputeNormalizedTextHash(canonical)
			seg.IncludeInFeed = true
			seg.IncludeInRAG = true
		} else {
			seg.Disposition = DispositionCoverageOnly
		}
		s.append(seg)
		return end
	}

	bq := s.makeBlockSegment(start, end-1, KindBlockquote, "")
	bq.Disposition = DispositionStructureOnly
	bq.RawTextHash = ComputeRawTextHash(s.lineBytes(start))
	s.append(bq)

	inner := s.makeBlockSegment(start, end-1, KindParagraph, bq.SegmentID)
	raw := []byte(s.sliceText(start, end-1))
	inner.RawTextHash = ComputeRawTextHash(raw)
	if canonical, ok := canonicalMarkdownAtomText(stripBlockquoteMarkers(string(raw))); ok {
		inner.Disposition = DispositionCanonicalAtom
		inner.NormalizedTextHash = ComputeNormalizedTextHash(canonical)
		inner.IncludeInFeed = true
		inner.IncludeInRAG = true
	} else {
		inner.Disposition = DispositionCoverageOnly
	}
	s.append(inner)
	return end
}

// stripBlockquoteMarkers removes leading '> ' or '>' from each line so the
// normalized hash captures the semantic body, not the citation marker.
func stripBlockquoteMarkers(s string) string {
	parts := strings.Split(s, "\n")
	for i, p := range parts {
		parts[i] = mdScanBlockquoteStrip.ReplaceAllString(p, "")
	}
	return strings.Join(parts, "\n")
}

// ----------------------------------------------------------------------------
// Lists.
// ----------------------------------------------------------------------------

func isListItemLine(text string) bool {
	return mdScanListItemRe.MatchString(text)
}

func (s *mdScanner) emitList(start int) int {
	end := start
	for end < len(s.lines) && isListItemLine(s.lines[end].text) {
		end++
	}

	listSeg := s.makeBlockSegment(start, end-1, KindList, "")
	listSeg.Disposition = DispositionStructureOnly
	listSeg.RawTextHash = ComputeRawTextHash(s.lineBytes(start))
	s.append(listSeg)

	for i := start; i < end; i++ {
		item := s.makeBlockSegment(i, i, KindListItem, listSeg.SegmentID)
		body := listItemBody(s.lines[i].text)
		raw := s.lineBytes(i)
		item.RawTextHash = ComputeRawTextHash(raw)
		if canonical, ok := canonicalMarkdownAtomText(body); ok {
			item.Disposition = DispositionCanonicalAtom
			item.NormalizedTextHash = ComputeNormalizedTextHash(canonical)
			item.IncludeInFeed = true
			item.IncludeInRAG = true
		} else {
			item.Disposition = DispositionCoverageOnly
		}
		s.append(item)
		s.emitInlineRefsInLine(i, item.SegmentID)
	}
	return end
}

// listItemBody returns the text of a list item with its leading marker
// (and indentation) removed.
func listItemBody(text string) string {
	loc := mdScanListItemRe.FindStringIndex(text)
	if loc == nil {
		return text
	}
	return text[loc[1]:]
}

// ----------------------------------------------------------------------------
// Paragraph.
// ----------------------------------------------------------------------------

func (s *mdScanner) emitParagraph(start int) int {
	end := start + 1
	for end < len(s.lines) {
		line := s.lines[end].text
		if isBlankLine(line) ||
			isHeadingLine(line) ||
			isFenceStart(line) ||
			isHTMLBlockStart(line) ||
			s.isTableStart(end) ||
			isBlockquoteLine(line) ||
			isDecorativeSeparator(line) ||
			isListItemLine(line) {
			break
		}
		end++
	}
	seg := s.makeBlockSegment(start, end-1, KindParagraph, "")
	raw := []byte(s.sliceText(start, end-1))
	seg.RawTextHash = ComputeRawTextHash(raw)
	if canonical, ok := canonicalMarkdownAtomText(string(raw)); ok {
		seg.Disposition = DispositionCanonicalAtom
		seg.NormalizedTextHash = ComputeNormalizedTextHash(canonical)
		seg.IncludeInFeed = true
		seg.IncludeInRAG = true
	} else {
		seg.Disposition = DispositionCoverageOnly
	}
	s.append(seg)

	for i := start; i < end; i++ {
		s.emitInlineRefsInLine(i, seg.SegmentID)
	}
	return end
}

func canonicalMarkdownAtomText(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false
	}
	if mdScanTOCOnlyLinkRe.MatchString(trimmed) {
		return "", false
	}
	if mdScanPlaceholderRe.MatchString(trimmed) {
		return "", false
	}
	plain := markdownPlainText(trimmed)
	if strings.TrimSpace(plain) == "" {
		return "", false
	}
	if mdScanStrongLabelRe.MatchString(trimmed) {
		return "", false
	}
	if isReferenceOnlySemanticLine(plain) {
		return "", false
	}
	if isPlaceholderSemantic(plain) {
		return "", false
	}
	if isIntroductoryLeadIn(plain) {
		return "", false
	}
	if semanticTokenCountMarkdown(plain) <= 2 {
		return "", false
	}
	if isJunkSemantic([]byte(plain)) {
		return "", false
	}
	return strings.TrimSpace(trimmed), true
}

func semanticTokenCountMarkdown(s string) int {
	count := 0
	inToken := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if !inToken {
				count++
				inToken = true
			}
			continue
		}
		inToken = false
	}
	return count
}

func markdownPlainText(s string) string {
	s = mdScanInlineRefRe.ReplaceAllString(s, "$2")
	replacer := strings.NewReplacer(
		"`", "",
		"**", "",
		"__", "",
		"*", "",
		"_", "",
	)
	s = replacer.Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

func isReferenceOnlySemanticLine(s string) bool {
	folded := foldLatinLight(strings.ToLower(strings.TrimSpace(s)))
	folded = strings.TrimLeft(folded, "-*+0123456789.) ")
	if strings.HasPrefix(folded, "statut :") ||
		strings.HasPrefix(folded, "status :") {
		return true
	}
	return mdScanChapterRefRe.MatchString(folded)
}

func isPlaceholderSemantic(s string) bool {
	folded := foldLatinLight(strings.ToLower(strings.TrimSpace(s)))
	return strings.Contains(folded, "section a completer") ||
		strings.Contains(folded, "a completer lors de la prochaine revision") ||
		strings.Contains(folded, "to be completed")
}

func isIntroductoryLeadIn(s string) bool {
	folded := foldLatinLight(strings.ToLower(strings.TrimSpace(s)))
	if !strings.HasSuffix(folded, ":") {
		return false
	}
	return semanticTokenCountMarkdown(folded) >= 3
}

func foldLatinLight(s string) string {
	replacer := strings.NewReplacer(
		"à", "a", "â", "a", "ä", "a",
		"é", "e", "è", "e", "ê", "e", "ë", "e",
		"î", "i", "ï", "i",
		"ô", "o", "ö", "o",
		"ù", "u", "û", "u", "ü", "u",
		"ç", "c",
	)
	return replacer.Replace(s)
}

// ----------------------------------------------------------------------------
// Inline references (links and images) emitted as child segments.
// ----------------------------------------------------------------------------

func (s *mdScanner) emitInlineRefsInLine(idx int, parentID string) {
	line := s.lines[idx]
	matches := mdScanInlineRefRe.FindAllSubmatchIndex([]byte(line.text), -1)
	for _, m := range matches {
		fullStart, fullEnd := m[0], m[1]
		isImage := m[2] != m[3]
		kind := KindLink
		if isImage {
			kind = KindImageRef
		}
		startB := line.startB + fullStart
		endB := line.startB + fullEnd
		matchText := line.text[fullStart:fullEnd]
		seg := SourceSegment{
			SegmentID:       segmentID(s.sourceID, startB, endB, kind),
			SourceID:        s.sourceID,
			SourcePath:      s.sourcePath,
			Kind:            kind,
			Disposition:     DispositionCoverageOnly,
			StartByte:       startB,
			EndByte:         endB,
			StartLine:       line.lineNum,
			StartColumn:     fullStart + 1,
			EndLine:         line.lineNum,
			EndColumn:       fullEnd + 1,
			ParentSegmentID: parentID,
			RawTextHash:     ComputeRawTextHash([]byte(matchText)),
		}
		s.append(seg)
	}
}
