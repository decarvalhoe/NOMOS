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
)

// decorativeChars enumerates the punctuation characters that may compose a
// decorative separator line ("---", "***", "___", "...", "~~~~", etc.).
const decorativeChars = "*-_.~+ \t\u2013\u2014\u2011\u2026"

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
		return isSingleUnicodeSeparator(trimmed)
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
	return nonWhite >= 3 || isSingleUnicodeSeparator(trimmed)
}

func isSingleUnicodeSeparator(text string) bool {
	switch text {
	case "\u2013", "\u2014", "\u2011", "\u2026":
		return true
	default:
		return false
	}
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

	tableSeg := s.makeBlockSegment(start, end-1, KindTable, "")
	tableSeg.Disposition = DispositionStructureOnly
	tableSeg.RawTextHash = ComputeRawTextHash(s.lineBytes(start))
	s.append(tableSeg)

	headerSeg := s.makeBlockSegment(start, start, KindTableHeader, tableSeg.SegmentID)
	headerSeg.Disposition = DispositionStructureOnly
	headerSeg.RawTextHash = ComputeRawTextHash(s.lineBytes(start))
	s.append(headerSeg)
	s.emitTableCells(start, headerSeg.SegmentID)

	sepSeg := s.makeBlockSegment(start+1, start+1, KindTableSeparator, tableSeg.SegmentID)
	sepSeg.Disposition = DispositionCoverageOnly
	sepSeg.RawTextHash = ComputeRawTextHash(s.lineBytes(start + 1))
	s.append(sepSeg)

	for r := start + 2; r < end; r++ {
		rowSeg := s.makeBlockSegment(r, r, KindTableRow, tableSeg.SegmentID)
		rowSeg.Disposition = DispositionStructureOnly
		rowSeg.RawTextHash = ComputeRawTextHash(s.lineBytes(r))
		s.append(rowSeg)
		s.emitTableCells(r, rowSeg.SegmentID)
	}

	return end
}

// emitTableCells splits the given line on '|' (escapes not handled) and
// emits one table_cell child segment per cell. Cells are siblings with no
// overlap; they do not need to cover the full row span (pipe glyphs are
// gaps between cells).
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
			StartByte:       startB,
			EndByte:         endB,
			StartLine:       line.lineNum,
			StartColumn:     r.start + 1,
			EndLine:         line.lineNum,
			EndColumn:       r.end + 1,
			ParentSegmentID: parentID,
			RawTextHash:     ComputeRawTextHash([]byte(cellText)),
		}
		if strings.TrimSpace(cellText) == "" || isJunkSemantic([]byte(cellText)) {
			seg.Disposition = DispositionCoverageOnly
		} else {
			seg.Disposition = DispositionCanonicalAtom
			seg.NormalizedTextHash = ComputeNormalizedTextHash(cellText)
			seg.IncludeInFeed = true
			seg.IncludeInRAG = true
		}
		s.append(seg)
	}
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
		seg.Disposition = DispositionCanonicalAtom
		raw := []byte(s.sliceText(start, end-1))
		seg.RawTextHash = ComputeRawTextHash(raw)
		seg.NormalizedTextHash = ComputeNormalizedTextHash(string(raw))
		seg.IncludeInFeed = true
		seg.IncludeInRAG = true
		s.append(seg)
		return end
	}

	bq := s.makeBlockSegment(start, end-1, KindBlockquote, "")
	bq.Disposition = DispositionStructureOnly
	bq.RawTextHash = ComputeRawTextHash(s.lineBytes(start))
	s.append(bq)

	inner := s.makeBlockSegment(start, end-1, KindParagraph, bq.SegmentID)
	inner.Disposition = DispositionCanonicalAtom
	raw := []byte(s.sliceText(start, end-1))
	inner.RawTextHash = ComputeRawTextHash(raw)
	inner.NormalizedTextHash = ComputeNormalizedTextHash(stripBlockquoteMarkers(string(raw)))
	inner.IncludeInFeed = true
	inner.IncludeInRAG = true
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
		if strings.TrimSpace(body) == "" {
			item.Disposition = DispositionCoverageOnly
		} else {
			item.Disposition = DispositionCanonicalAtom
			item.NormalizedTextHash = ComputeNormalizedTextHash(body)
			item.IncludeInFeed = true
			item.IncludeInRAG = true
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
	seg.Disposition = DispositionCanonicalAtom
	raw := []byte(s.sliceText(start, end-1))
	seg.RawTextHash = ComputeRawTextHash(raw)
	seg.NormalizedTextHash = ComputeNormalizedTextHash(string(raw))
	seg.IncludeInFeed = true
	seg.IncludeInRAG = true
	s.append(seg)

	for i := start; i < end; i++ {
		s.emitInlineRefsInLine(i, seg.SegmentID)
	}
	return end
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
