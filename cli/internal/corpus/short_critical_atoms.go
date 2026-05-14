package corpus

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

const (
	ShortCriticalAtomsFormat = "nomos.short-critical-atoms.v1"
	ShortCriticalMaxRunes    = 10

	ShortCriticalNonSemantic            = "non_semantic"
	ShortCriticalContextualizedInParent = "contextualized_in_parent"
	ShortCriticalLexiconAtom            = "lexicon_atom"
	ShortCriticalIdentifierAtom         = "identifier_atom"
	ShortCriticalNormativeValueAtom     = "normative_value_atom"
	ShortCriticalRequiresReview         = "requires_review"
)

// ShortCriticalAtom records the disposition of a short source fragment whose
// standalone semantics would otherwise be lost by feed/RAG noise filters.
type ShortCriticalAtom struct {
	Fragment           string   `json:"fragment"`
	SourceID           string   `json:"source_id"`
	SourcePath         string   `json:"source_path"`
	StartByte          int      `json:"start_byte"`
	EndByte            int      `json:"end_byte"`
	StartLine          int      `json:"start_line"`
	EndLine            int      `json:"end_line"`
	ParentChain        []string `json:"parent_chain,omitempty"`
	TableID            string   `json:"table_id,omitempty"`
	RowIndex           int      `json:"row_index,omitempty"`
	ColumnHeaders      []string `json:"column_headers,omitempty"`
	YAMLPath           string   `json:"yaml_path,omitempty"`
	JSONPath           string   `json:"json_path,omitempty"`
	StructuredPath     string   `json:"structured_path,omitempty"`
	StructuredFormat   string   `json:"structured_format,omitempty"`
	NodeKind           string   `json:"node_kind,omitempty"`
	SurroundingContext string   `json:"surrounding_context"`
	Disposition        string   `json:"disposition"`
	Reason             string   `json:"reason,omitempty"`
	PromotedArtifactID string   `json:"promoted_artifact_id,omitempty"`
}

// ShortCriticalAtomsReport is emitted with corpus feeds and as a standalone
// artifact so reviewers can verify every governed short fragment has a
// disposition without admitting it as an orphan RAG chunk.
type ShortCriticalAtomsReport struct {
	Format          string              `json:"format"`
	GeneratedAt     string              `json:"generated_at"`
	SourceCount     int                 `json:"source_count"`
	AtomCount       int                 `json:"atom_count"`
	UnresolvedCount int                 `json:"unresolved_count"`
	Atoms           []ShortCriticalAtom `json:"atoms"`
}

type shortCriticalContext struct {
	source           ManifestSource
	fragment         string
	startByte        int
	endByte          int
	startLine        int
	endLine          int
	parentChain      []string
	tableID          string
	rowIndex         int
	columnHeaders    []string
	columnHeader     string
	yamlPath         string
	jsonPath         string
	structuredPath   string
	structuredFormat string
	nodeKind         string
	surrounding      string
	embedded         bool
	headerCell       bool
}

// BuildShortCriticalAtomsReport scans every admitted+atomized source in a
// manifest and classifies short fragments without changing feed eligibility.
func BuildShortCriticalAtomsReport(root string, manifest SidecarManifest, generatedAt time.Time) (*ShortCriticalAtomsReport, error) {
	if strings.TrimSpace(root) == "" {
		return nil, nil
	}
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	report := &ShortCriticalAtomsReport{
		Format:      ShortCriticalAtomsFormat,
		GeneratedAt: generatedAt.Format(time.RFC3339),
		Atoms:       []ShortCriticalAtom{},
	}

	seenSources := 0
	for _, source := range manifest.Sources {
		if source.Status != "" && source.Status != "active" && source.Status != "needs_review" {
			continue
		}
		if !shouldExtractSourceUnits(source) {
			continue
		}
		absPath := filepath.Join(root, filepath.FromSlash(source.Path))
		content, err := os.ReadFile(absPath)
		if err != nil {
			return nil, fmt.Errorf("read short critical source %s: %w", source.Path, err)
		}
		seenSources++
		ext := strings.ToLower(filepath.Ext(source.Path))
		switch {
		case source.Type == "markdown" || ext == ".md" || ext == ".mdx":
			atoms, err := shortCriticalAtomsFromMarkdown(content, source)
			if err != nil {
				return nil, err
			}
			report.Atoms = append(report.Atoms, atoms...)
		case ext == ".yaml" || ext == ".yml" || ext == ".json":
			format, ok := StructuredFormatForPath(source.Path)
			if !ok {
				continue
			}
			atoms, err := shortCriticalAtomsFromStructured(content, source, format)
			if err != nil {
				return nil, err
			}
			report.Atoms = append(report.Atoms, atoms...)
		}
	}

	report.SourceCount = seenSources
	report.Atoms = dedupeShortCriticalAtoms(report.Atoms)
	sort.SliceStable(report.Atoms, func(i, j int) bool {
		a, b := report.Atoms[i], report.Atoms[j]
		if a.SourcePath != b.SourcePath {
			return a.SourcePath < b.SourcePath
		}
		if a.StartByte != b.StartByte {
			return a.StartByte < b.StartByte
		}
		if a.EndByte != b.EndByte {
			return a.EndByte < b.EndByte
		}
		return a.Fragment < b.Fragment
	})
	report.AtomCount = len(report.Atoms)
	for _, atom := range report.Atoms {
		if atom.Disposition == "" || atom.Disposition == ShortCriticalRequiresReview {
			report.UnresolvedCount++
		}
	}
	return report, nil
}

func shortCriticalAtomsFromMarkdown(content []byte, source ManifestSource) ([]ShortCriticalAtom, error) {
	segments, err := ScanMarkdown(source.ID, source.Path, content)
	if err != nil {
		return nil, fmt.Errorf("scan markdown short critical %s: %w", source.Path, err)
	}
	segByID := make(map[string]SourceSegment, len(segments))
	for _, seg := range segments {
		segByID[seg.SegmentID] = seg
	}
	cellIndexByID := markdownTableCellIndexes(segments)

	type headingFrame struct {
		level int
		title string
	}
	var headings []headingFrame
	headingChain := func() []string {
		if len(headings) == 0 {
			return nil
		}
		out := make([]string, len(headings))
		for i, h := range headings {
			out[i] = h.title
		}
		return out
	}

	var atoms []ShortCriticalAtom
	for _, seg := range segments {
		if seg.Kind == KindHeading && seg.ParentSegmentID == "" {
			level, title := parseHeadingLevelTitle(segmentText(content, seg))
			if level >= 1 && level <= 6 && strings.TrimSpace(title) != "" {
				for len(headings) > 0 && headings[len(headings)-1].level >= level {
					headings = headings[:len(headings)-1]
				}
				headings = append(headings, headingFrame{level: level, title: title})
			}
			continue
		}

		switch seg.Kind {
		case KindTableCell:
			raw := segmentText(content, seg)
			fragment, start, end := trimFragmentSpan(raw, seg.StartByte)
			if !isShortCriticalFragment(fragment) {
				continue
			}
			parent := segByID[seg.ParentSegmentID]
			columnHeaders := append([]string(nil), parent.ColumnHeaders...)
			cellIndex := cellIndexByID[seg.SegmentID]
			columnHeader := ""
			if cellIndex >= 0 && cellIndex < len(columnHeaders) {
				columnHeader = columnHeaders[cellIndex]
			}
			parentChain := headingChain()
			if parent.ParentSegmentID != "" {
				parentChain = append(parentChain, parent.ParentSegmentID)
			}
			if parent.SegmentID != "" {
				parentChain = append(parentChain, parent.SegmentID)
			}
			context := parent.RowCanonicalText
			if context == "" {
				context = segmentText(content, parent)
			}
			atoms = append(atoms, newShortCriticalAtom(shortCriticalContext{
				source:        source,
				fragment:      fragment,
				startByte:     start,
				endByte:       end,
				startLine:     seg.StartLine,
				endLine:       seg.EndLine,
				parentChain:   parentChain,
				tableID:       parent.TableID,
				rowIndex:      parent.RowIndex,
				columnHeaders: columnHeaders,
				columnHeader:  columnHeader,
				nodeKind:      seg.Kind,
				surrounding:   context,
				headerCell:    parent.Kind == KindTableHeader,
			}))
		case KindParagraph, KindListItem:
			raw := segmentText(content, seg)
			body := raw
			if seg.Kind == KindListItem {
				body = listItemBody(raw)
			}
			plain := markdownPlainText(body)
			trimmed := strings.TrimSpace(plain)
			if isShortCriticalFragment(trimmed) && (seg.Disposition != DispositionCanonicalAtom || looksStandaloneCritical(trimmed, "", seg.Kind)) {
				start := findFragmentByteStart(raw, trimmed, seg.StartByte)
				atoms = append(atoms, newShortCriticalAtom(shortCriticalContext{
					source:      source,
					fragment:    trimmed,
					startByte:   start,
					endByte:     start + len(trimmed),
					startLine:   seg.StartLine,
					endLine:     seg.EndLine,
					parentChain: headingChain(),
					nodeKind:    seg.Kind,
					surrounding: trimmed,
				}))
				continue
			}
			if runeCount(trimmed) <= ShortCriticalMaxRunes {
				continue
			}
			for _, match := range inlineShortCriticalMatches(raw, seg.StartByte) {
				atoms = append(atoms, newShortCriticalAtom(shortCriticalContext{
					source:      source,
					fragment:    match.fragment,
					startByte:   match.startByte,
					endByte:     match.endByte,
					startLine:   seg.StartLine,
					endLine:     seg.EndLine,
					parentChain: headingChain(),
					nodeKind:    seg.Kind,
					surrounding: trimmed,
					embedded:    true,
				}))
			}
		}
	}
	return atoms, nil
}

func shortCriticalAtomsFromStructured(content []byte, source ManifestSource, format string) ([]ShortCriticalAtom, error) {
	scan, err := ScanStructuredScalars(source, content, format)
	if err != nil {
		return nil, fmt.Errorf("scan structured short critical %s: %w", source.Path, err)
	}
	var atoms []ShortCriticalAtom
	for _, scalar := range scan.Scalars {
		value := strings.TrimSpace(scalar.DecodedValue)
		yamlPath := ""
		jsonPath := ""
		if format == StructuredFormatYAML {
			yamlPath = scalar.Path
		}
		if format == StructuredFormatJSON {
			jsonPath = scalar.Path
		}
		if isShortCriticalFragment(value) {
			start, end := structuredScalarValueSpan(content, scalar, value)
			atoms = append(atoms, newShortCriticalAtom(shortCriticalContext{
				source:           source,
				fragment:         value,
				startByte:        start,
				endByte:          end,
				startLine:        scalar.StartLine,
				endLine:          scalar.EndLine,
				parentChain:      structuredParentChain(scalar.Path),
				yamlPath:         yamlPath,
				jsonPath:         jsonPath,
				structuredPath:   scalar.Path,
				structuredFormat: format,
				nodeKind:         scalar.NodeKind,
				surrounding:      scalar.Path + " = " + value,
			}))
			continue
		}
		for _, match := range inlineShortCriticalMatches(scalar.RawText, scalar.StartByte) {
			atoms = append(atoms, newShortCriticalAtom(shortCriticalContext{
				source:           source,
				fragment:         match.fragment,
				startByte:        match.startByte,
				endByte:          match.endByte,
				startLine:        scalar.StartLine,
				endLine:          scalar.EndLine,
				parentChain:      structuredParentChain(scalar.Path),
				yamlPath:         yamlPath,
				jsonPath:         jsonPath,
				structuredPath:   scalar.Path,
				structuredFormat: format,
				nodeKind:         scalar.NodeKind,
				surrounding:      scalar.Path + " = " + value,
				embedded:         true,
			}))
		}
	}
	return atoms, nil
}

func newShortCriticalAtom(ctx shortCriticalContext) ShortCriticalAtom {
	disposition, reason := classifyShortCriticalDisposition(ctx)
	return ShortCriticalAtom{
		Fragment:           ctx.fragment,
		SourceID:           ctx.source.ID,
		SourcePath:         ctx.source.Path,
		StartByte:          ctx.startByte,
		EndByte:            ctx.endByte,
		StartLine:          ctx.startLine,
		EndLine:            ctx.endLine,
		ParentChain:        cleanStringList(ctx.parentChain),
		TableID:            ctx.tableID,
		RowIndex:           ctx.rowIndex,
		ColumnHeaders:      cleanStringList(ctx.columnHeaders),
		YAMLPath:           ctx.yamlPath,
		JSONPath:           ctx.jsonPath,
		StructuredPath:     ctx.structuredPath,
		StructuredFormat:   ctx.structuredFormat,
		NodeKind:           ctx.nodeKind,
		SurroundingContext: truncateShortCriticalContext(ctx.surrounding),
		Disposition:        disposition,
		Reason:             reason,
		PromotedArtifactID: promotedShortCriticalArtifactID(disposition, ctx),
	}
}

func classifyShortCriticalDisposition(ctx shortCriticalContext) (string, string) {
	fragment := strings.TrimSpace(ctx.fragment)
	if fragment == "" || isJunkSemantic([]byte(fragment)) {
		return ShortCriticalNonSemantic, "blank or layout-only fragment"
	}
	if ctx.headerCell {
		return ShortCriticalNonSemantic, "table header labels stay structural"
	}
	if ctx.embedded {
		return ShortCriticalContextualizedInParent, "short fragment appears inside a longer parent atom"
	}
	if isNormativeShortValue(fragment, ctx) {
		return ShortCriticalNormativeValueAtom, "path, column, or scalar kind identifies a controlled value"
	}
	if isIdentifierShortValue(fragment) {
		return ShortCriticalIdentifierAtom, "fragment matches identifier/reference code shape"
	}
	if isLexiconShortValue(fragment) {
		return ShortCriticalLexiconAtom, "fragment matches governed acronym/term shape"
	}
	if isLikelyNonSemanticShortValue(fragment, ctx) {
		return ShortCriticalNonSemantic, "fragment is a structural label or non-doctrinal value"
	}
	return ShortCriticalRequiresReview, "short fragment is meaningful enough to require explicit review"
}

func isShortCriticalFragment(fragment string) bool {
	fragment = strings.TrimSpace(fragment)
	return fragment != "" && runeCount(fragment) <= ShortCriticalMaxRunes
}

func isNormativeShortValue(fragment string, ctx shortCriticalContext) bool {
	folded := strings.ToLower(strings.TrimSpace(fragment))
	switch folded {
	case "yes", "no", "true", "false", "oui", "non", "pass", "fail", "passed", "failed", "blocked", "active", "draft":
		return true
	}
	if ctx.nodeKind == "scalar_bool" {
		return true
	}
	hints := []string{
		ctx.columnHeader,
		ctx.yamlPath,
		ctx.jsonPath,
		ctx.structuredPath,
		strings.Join(ctx.parentChain, "."),
	}
	for _, hint := range hints {
		h := strings.ToLower(hint)
		if strings.Contains(h, "status") ||
			strings.Contains(h, "statut") ||
			strings.Contains(h, "priority") ||
			strings.Contains(h, "priorite") ||
			strings.Contains(h, "severity") ||
			strings.Contains(h, "criticality") ||
			strings.Contains(h, "risk") ||
			strings.Contains(h, "approved") ||
			strings.Contains(h, "required") ||
			strings.Contains(h, "enabled") ||
			strings.Contains(h, "mode") ||
			strings.Contains(h, "level") {
			return true
		}
	}
	return false
}

func isIdentifierShortValue(fragment string) bool {
	fragment = strings.TrimSpace(fragment)
	if fragment == "" {
		return false
	}
	hasLetter := false
	hasDigit := false
	for _, r := range fragment {
		if unicode.IsLetter(r) {
			hasLetter = true
		}
		if unicode.IsDigit(r) {
			hasDigit = true
		}
	}
	if hasLetter && hasDigit {
		return true
	}
	return false
}

func isLexiconShortValue(fragment string) bool {
	fragment = strings.TrimSpace(fragment)
	if fragment == "" {
		return false
	}
	letters := 0
	upperOrSpecial := 0
	for _, r := range fragment {
		switch {
		case unicode.IsLetter(r):
			letters++
			if unicode.IsUpper(r) {
				upperOrSpecial++
			}
		case r == '+' || r == '&' || r == '/':
			upperOrSpecial++
		case r == '-' || r == '_':
			// allowed separator
		default:
			return false
		}
	}
	return letters >= 2 && upperOrSpecial >= 2
}

func isLikelyNonSemanticShortValue(fragment string, ctx shortCriticalContext) bool {
	folded := strings.ToLower(strings.TrimSpace(fragment))
	switch folded {
	case "field", "value", "champ", "valeur", "key", "name", "id", "date", "type":
		return true
	}
	return ctx.nodeKind == "scalar_null"
}

func looksStandaloneCritical(fragment string, pathOrHeader string, nodeKind string) bool {
	ctx := shortCriticalContext{
		fragment:       fragment,
		structuredPath: pathOrHeader,
		columnHeader:   pathOrHeader,
		nodeKind:       nodeKind,
	}
	if isNormativeShortValue(fragment, ctx) || isIdentifierShortValue(fragment) || isLexiconShortValue(fragment) {
		return true
	}
	return false
}

func promotedShortCriticalArtifactID(disposition string, ctx shortCriticalContext) string {
	switch disposition {
	case ShortCriticalLexiconAtom:
		return "lexicon:" + shortCriticalSlug(ctx.fragment)
	case ShortCriticalIdentifierAtom:
		return "identifier:" + shortCriticalSlug(ctx.fragment)
	case ShortCriticalNormativeValueAtom:
		scope := firstNonEmptyTrim(ctx.columnHeader, ctx.yamlPath, ctx.jsonPath, ctx.structuredPath, "value")
		return "value:" + shortCriticalSlug(scope) + ":" + shortCriticalSlug(ctx.fragment)
	default:
		return ""
	}
}

func shortCriticalSlug(s string) string {
	return strings.ToLower(toUpperSlug(s))
}

type inlineShortCriticalMatch struct {
	fragment  string
	startByte int
	endByte   int
}

func inlineShortCriticalMatches(raw string, baseByte int) []inlineShortCriticalMatch {
	var out []inlineShortCriticalMatch
	searchFrom := 0
	seen := map[string]struct{}{}
	for _, field := range strings.Fields(raw) {
		token := cleanInlineShortToken(field)
		if !isShortCriticalFragment(token) {
			searchFrom += len(field)
			continue
		}
		if !looksStandaloneCritical(token, "", "") {
			searchFrom += len(field)
			continue
		}
		idx := strings.Index(raw[searchFrom:], token)
		if idx < 0 {
			idx = strings.Index(raw, token)
			if idx < 0 {
				searchFrom += len(field)
				continue
			}
		} else {
			idx += searchFrom
		}
		key := fmt.Sprintf("%d:%s", idx, token)
		if _, ok := seen[key]; ok {
			searchFrom = idx + len(token)
			continue
		}
		seen[key] = struct{}{}
		out = append(out, inlineShortCriticalMatch{
			fragment:  token,
			startByte: baseByte + idx,
			endByte:   baseByte + idx + len(token),
		})
		searchFrom = idx + len(token)
	}
	return out
}

func cleanInlineShortToken(field string) string {
	return strings.Trim(field, " \t\r\n.,;:!?()[]{}\"'`*_<>\u00ab\u00bb")
}

func trimFragmentSpan(raw string, baseByte int) (string, int, int) {
	start := 0
	for start < len(raw) {
		r := rune(raw[start])
		if !unicode.IsSpace(r) {
			break
		}
		start++
	}
	end := len(raw)
	for end > start {
		r := rune(raw[end-1])
		if !unicode.IsSpace(r) {
			break
		}
		end--
	}
	return raw[start:end], baseByte + start, baseByte + end
}

func segmentText(content []byte, seg SourceSegment) string {
	if seg.StartByte < 0 || seg.EndByte < seg.StartByte || seg.EndByte > len(content) {
		return ""
	}
	return string(content[seg.StartByte:seg.EndByte])
}

func findFragmentByteStart(raw string, fragment string, baseByte int) int {
	if idx := strings.Index(raw, fragment); idx >= 0 {
		return baseByte + idx
	}
	return baseByte
}

func structuredScalarValueSpan(content []byte, scalar StructuredScalar, value string) (int, int) {
	if scalar.StartByte >= 0 && scalar.EndByte <= len(content) && scalar.EndByte >= scalar.StartByte {
		raw := string(content[scalar.StartByte:scalar.EndByte])
		if idx := strings.Index(raw, value); idx >= 0 {
			start := scalar.StartByte + idx
			return start, start + len(value)
		}
	}
	return scalar.StartByte, scalar.EndByte
}

func structuredParentChain(path string) []string {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	parts := strings.Split(path, ".")
	if len(parts) <= 1 {
		return nil
	}
	out := make([]string, 0, len(parts)-1)
	for i := 1; i < len(parts); i++ {
		parent := strings.Join(parts[:i], ".")
		if parent != "" {
			out = append(out, parent)
		}
	}
	return out
}

func markdownTableCellIndexes(segments []SourceSegment) map[string]int {
	type cell struct {
		id    string
		start int
	}
	byParent := map[string][]cell{}
	for _, seg := range segments {
		if seg.Kind != KindTableCell || seg.ParentSegmentID == "" {
			continue
		}
		byParent[seg.ParentSegmentID] = append(byParent[seg.ParentSegmentID], cell{id: seg.SegmentID, start: seg.StartByte})
	}
	out := map[string]int{}
	for _, cells := range byParent {
		sort.SliceStable(cells, func(i, j int) bool { return cells[i].start < cells[j].start })
		for i, cell := range cells {
			out[cell.id] = i
		}
	}
	return out
}

func cleanStringList(in []string) []string {
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func truncateShortCriticalContext(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	const max = 240
	if runeCount(s) <= max {
		return s
	}
	var out []rune
	for _, r := range s {
		if len(out) >= max {
			break
		}
		out = append(out, r)
	}
	return string(out) + "..."
}

func dedupeShortCriticalAtoms(atoms []ShortCriticalAtom) []ShortCriticalAtom {
	seen := map[string]struct{}{}
	out := make([]ShortCriticalAtom, 0, len(atoms))
	for _, atom := range atoms {
		key := fmt.Sprintf("%s:%d:%d:%s", atom.SourceID, atom.StartByte, atom.EndByte, atom.Fragment)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, atom)
	}
	return out
}

func governedShortCriticalAtomCountsBySource(report *ShortCriticalAtomsReport) map[string]int {
	out := map[string]int{}
	if report == nil {
		return out
	}
	for _, atom := range report.Atoms {
		switch atom.Disposition {
		case ShortCriticalLexiconAtom, ShortCriticalIdentifierAtom, ShortCriticalNormativeValueAtom:
			if strings.TrimSpace(atom.SourceID) != "" {
				out[atom.SourceID]++
			}
		}
	}
	return out
}
