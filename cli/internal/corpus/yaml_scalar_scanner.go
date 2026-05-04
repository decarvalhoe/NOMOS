package corpus

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

const (
	KindYAMLScalar = "yaml_scalar"
	KindYAMLGap    = "yaml_gap"
)

type yamlScalarSegment struct {
	Value   string
	Segment SourceSegment
}

// ScanYAMLScalars emits a byte-covering ledger for a structured YAML source.
// Scalar values become canonical atoms; every byte outside those scalar spans
// becomes coverage-only YAML structure. This keeps structured sources
// traceable without pretending keys and indentation are retrievable doctrine.
func ScanYAMLScalars(sourceID, sourcePath string, content []byte) ([]SourceSegment, error) {
	scalars, err := scanYAMLScalarSegmentsWithValues(sourceID, sourcePath, content)
	if err != nil {
		return nil, err
	}
	semantic := make([]SourceSegment, 0, len(scalars))
	for _, scalar := range scalars {
		semantic = append(semantic, scalar.Segment)
	}
	return yamlSegmentsWithCoverage(sourceID, sourcePath, content, semantic), nil
}

func scanYAMLScalarSegmentsWithValues(sourceID, sourcePath string, content []byte) ([]yamlScalarSegment, error) {
	if strings.TrimSpace(sourceID) == "" {
		return nil, fmt.Errorf("scan yaml: sourceID required")
	}
	if strings.TrimSpace(sourcePath) == "" {
		return nil, fmt.Errorf("scan yaml: sourcePath required")
	}
	if len(content) == 0 {
		return nil, nil
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return nil, fmt.Errorf("scan yaml %s: %w", sourcePath, err)
	}
	lines := splitMDLines(content)
	var out []yamlScalarSegment
	walkYAMLValueNodes(&doc, func(node *yaml.Node) {
		value := strings.TrimSpace(node.Value)
		if value == "" {
			return
		}
		span, ok := yamlScalarSpan(node, lines)
		if !ok || span.start >= span.end || span.end > len(content) {
			return
		}
		raw := content[span.start:span.end]
		disposition := DispositionCanonicalAtom
		includeInFeed := true
		includeInRAG := true
		if isJunkSemantic([]byte(node.Value)) {
			disposition = DispositionCoverageOnly
			includeInFeed = false
			includeInRAG = false
		}
		seg := SourceSegment{
			SegmentID:          segmentID(sourceID, span.start, span.end, KindYAMLScalar),
			SourceID:           sourceID,
			SourcePath:         sourcePath,
			Kind:               KindYAMLScalar,
			Disposition:        disposition,
			StartByte:          span.start,
			EndByte:            span.end,
			StartLine:          span.startLine,
			StartColumn:        span.startColumn,
			EndLine:            span.endLine,
			EndColumn:          span.endColumn,
			RawTextHash:        ComputeRawTextHash(raw),
			NormalizedTextHash: ComputeNormalizedTextHash(node.Value),
			IncludeInFeed:      includeInFeed,
			IncludeInRAG:       includeInRAG,
		}
		out = append(out, yamlScalarSegment{Value: node.Value, Segment: seg})
	})
	sort.Slice(out, func(i, j int) bool {
		return out[i].Segment.StartByte < out[j].Segment.StartByte
	})
	return out, nil
}

func walkYAMLValueNodes(node *yaml.Node, visit func(*yaml.Node)) {
	if node == nil {
		return
	}
	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			walkYAMLValueNodes(child, visit)
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			walkYAMLValueNodes(node.Content[i+1], visit)
		}
	case yaml.SequenceNode:
		for _, child := range node.Content {
			walkYAMLValueNodes(child, visit)
		}
	case yaml.ScalarNode:
		visit(node)
	}
}

type yamlByteSpan struct {
	start       int
	end         int
	startLine   int
	startColumn int
	endLine     int
	endColumn   int
}

func yamlScalarSpan(node *yaml.Node, lines []mdLine) (yamlByteSpan, bool) {
	if node.Line <= 0 || node.Line > len(lines) {
		return yamlByteSpan{}, false
	}
	if node.Style == yaml.LiteralStyle || node.Style == yaml.FoldedStyle {
		return yamlBlockScalarSpan(node, lines)
	}
	line := lines[node.Line-1]
	startColumn := node.Column
	if startColumn <= 0 {
		startColumn = 1
	}
	start := yamlColumnByte(line, startColumn)
	end := line.startB + len(line.text)
	if end <= start {
		return yamlByteSpan{}, false
	}
	return yamlByteSpan{
		start:       start,
		end:         end,
		startLine:   line.lineNum,
		startColumn: startColumn,
		endLine:     line.lineNum,
		endColumn:   line.contentLen + 1,
	}, true
}

func yamlBlockScalarSpan(node *yaml.Node, lines []mdLine) (yamlByteSpan, bool) {
	startSearch := node.Line
	if startSearch >= len(lines) {
		return yamlScalarSpanOnIndicator(node, lines)
	}
	startIdx := -1
	contentColumn := 0
	for i := startSearch; i < len(lines); i++ {
		if strings.TrimSpace(lines[i].text) == "" {
			continue
		}
		contentColumn = yamlFirstContentColumn(lines[i].text)
		startIdx = i
		break
	}
	if startIdx < 0 {
		return yamlScalarSpanOnIndicator(node, lines)
	}
	endIdx := startIdx
	for i := startIdx; i < len(lines); i++ {
		if strings.TrimSpace(lines[i].text) != "" && yamlFirstContentColumn(lines[i].text) < contentColumn {
			break
		}
		endIdx = i
	}
	start := yamlColumnByte(lines[startIdx], contentColumn)
	end := lines[endIdx].endB
	if end <= start {
		return yamlByteSpan{}, false
	}
	return yamlByteSpan{
		start:       start,
		end:         end,
		startLine:   lines[startIdx].lineNum,
		startColumn: contentColumn,
		endLine:     lines[endIdx].lineNum,
		endColumn:   lines[endIdx].contentLen + 1,
	}, true
}

func yamlScalarSpanOnIndicator(node *yaml.Node, lines []mdLine) (yamlByteSpan, bool) {
	line := lines[node.Line-1]
	startColumn := node.Column
	if startColumn <= 0 {
		startColumn = 1
	}
	start := yamlColumnByte(line, startColumn)
	end := line.startB + len(line.text)
	if end <= start {
		return yamlByteSpan{}, false
	}
	return yamlByteSpan{
		start:       start,
		end:         end,
		startLine:   line.lineNum,
		startColumn: startColumn,
		endLine:     line.lineNum,
		endColumn:   line.contentLen + 1,
	}, true
}

func yamlSegmentsWithCoverage(sourceID, sourcePath string, content []byte, semantic []SourceSegment) []SourceSegment {
	sort.Slice(semantic, func(i, j int) bool {
		if semantic[i].StartByte != semantic[j].StartByte {
			return semantic[i].StartByte < semantic[j].StartByte
		}
		return semantic[i].EndByte < semantic[j].EndByte
	})
	lines := splitMDLines(content)
	var out []SourceSegment
	cursor := 0
	for _, seg := range semantic {
		if seg.StartByte > cursor {
			out = append(out, yamlCoverageSegment(sourceID, sourcePath, content, lines, cursor, seg.StartByte))
		}
		out = append(out, seg)
		if seg.EndByte > cursor {
			cursor = seg.EndByte
		}
	}
	if cursor < len(content) {
		out = append(out, yamlCoverageSegment(sourceID, sourcePath, content, lines, cursor, len(content)))
	}
	return out
}

func yamlCoverageSegment(sourceID, sourcePath string, content []byte, lines []mdLine, start, end int) SourceSegment {
	startLine, startColumn := yamlBytePosition(lines, start)
	endLine, endColumn := yamlBytePosition(lines, end)
	raw := content[start:end]
	return SourceSegment{
		SegmentID:     segmentID(sourceID, start, end, KindYAMLGap),
		SourceID:      sourceID,
		SourcePath:    sourcePath,
		Kind:          KindYAMLGap,
		Disposition:   DispositionCoverageOnly,
		StartByte:     start,
		EndByte:       end,
		StartLine:     startLine,
		StartColumn:   startColumn,
		EndLine:       endLine,
		EndColumn:     endColumn,
		RawTextHash:   ComputeRawTextHash(raw),
		IncludeInFeed: false,
		IncludeInRAG:  false,
	}
}

func yamlColumnByte(line mdLine, column int) int {
	if column <= 1 {
		return line.startB
	}
	target := column - 1
	offset := 0
	for offset < len(line.text) && target > 0 {
		_, size := utf8.DecodeRuneInString(line.text[offset:])
		offset += size
		target--
	}
	return line.startB + offset
}

func yamlFirstContentColumn(text string) int {
	column := 1
	for _, r := range text {
		if r != ' ' && r != '\t' {
			break
		}
		column++
	}
	return column
}

func yamlBytePosition(lines []mdLine, offset int) (int, int) {
	if len(lines) == 0 {
		return 1, 1
	}
	for _, line := range lines {
		if offset < line.startB || offset > line.endB {
			continue
		}
		column := 1
		textEnd := offset - line.startB
		if textEnd > len(line.text) {
			textEnd = len(line.text)
		}
		for i := 0; i < textEnd; {
			_, size := utf8.DecodeRuneInString(line.text[i:])
			i += size
			column++
		}
		return line.lineNum, column
	}
	last := lines[len(lines)-1]
	return last.lineNum, last.contentLen + 1
}
