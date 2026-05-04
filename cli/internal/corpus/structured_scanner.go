package corpus

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

const (
	StructuredFormatYAML = "yaml"
	StructuredFormatJSON = "json"
)

// StructuredScan is the generic scanner output for normative structured
// documents. It is intentionally schema-neutral: domain profiles decide
// which scalar paths are doctrinal, while the scanner proves exact source
// spans and complete byte coverage for YAML/JSON-like files.
type StructuredScan struct {
	Format   string
	Scalars  []StructuredScalar
	Segments []SourceSegment
}

// StructuredScalar is one scalar value found in a structured source file.
// Path is the stable dotted/indexed source path (for example
// "rules[0].text"). Segment points to the corresponding canonical atom in
// the full SourceSegment ledger.
type StructuredScalar struct {
	Path         string
	Format       string
	RawText      string
	DecodedValue string
	NodeKind     string
	StartByte    int
	EndByte      int
	StartLine    int
	StartColumn  int
	EndLine      int
	EndColumn    int
	Segment      SourceSegment
}

func StructuredFormatForPath(path string) (string, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return StructuredFormatYAML, true
	case ".json":
		return StructuredFormatJSON, true
	default:
		return "", false
	}
}

func ScanStructuredScalars(source ManifestSource, content []byte, format string) (StructuredScan, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	var scalars []StructuredScalar
	var err error
	switch format {
	case StructuredFormatYAML:
		scalars = structuredYAMLScalars(content)
	case StructuredFormatJSON:
		scalars, err = structuredJSONScalars(content)
		if err != nil {
			return StructuredScan{}, err
		}
	default:
		return StructuredScan{}, fmt.Errorf("unsupported structured format %q", format)
	}
	sort.SliceStable(scalars, func(i, j int) bool {
		if scalars[i].StartByte != scalars[j].StartByte {
			return scalars[i].StartByte < scalars[j].StartByte
		}
		return scalars[i].EndByte < scalars[j].EndByte
	})
	for i := range scalars {
		scalars[i].Format = format
		scalars[i].Segment = structuredScalarSegment(source, scalars[i])
	}
	return StructuredScan{
		Format:   format,
		Scalars:  scalars,
		Segments: buildStructuredCoverageSegments(source, content, scalars),
	}, nil
}

func structuredYAMLScalars(content []byte) []StructuredScalar {
	index := indexYAMLScalars(content)
	out := make([]StructuredScalar, 0, len(index))
	offsets := computeYAMLLineOffsets(content)
	for path, loc := range index {
		endLine, endColumn := lineColumnForByteOffset(content, offsets, loc.EndByte)
		out = append(out, StructuredScalar{
			Path:         path,
			Format:       StructuredFormatYAML,
			RawText:      loc.RawText,
			DecodedValue: loc.DecodedValue,
			NodeKind:     loc.NodeKind,
			StartByte:    loc.StartByte,
			EndByte:      loc.EndByte,
			StartLine:    loc.StartLine,
			StartColumn:  loc.StartColumn,
			EndLine:      endLine,
			EndColumn:    endColumn,
		})
	}
	return out
}

type jsonContext struct {
	kind       byte
	path       string
	expectKey  bool
	currentKey string
	index      int
}

func structuredJSONScalars(content []byte) ([]StructuredScalar, error) {
	dec := json.NewDecoder(bytes.NewReader(content))
	dec.UseNumber()
	offsets := computeYAMLLineOffsets(content)
	var stack []jsonContext
	var out []StructuredScalar
	prev := 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse json: %w", err)
		}
		end := int(dec.InputOffset())
		if delim, ok := tok.(json.Delim); ok {
			switch delim {
			case '{', '[':
				path, isValue := consumeJSONValuePath(stack)
				if !isValue {
					return nil, fmt.Errorf("json container without value path near byte %d", end)
				}
				stack = updateJSONParentAfterContainer(stack)
				stack = append(stack, jsonContext{
					kind:      byte(delim),
					path:      path,
					expectKey: delim == '{',
				})
			case '}', ']':
				if len(stack) == 0 {
					return nil, fmt.Errorf("json close delimiter without context near byte %d", end)
				}
				stack = stack[:len(stack)-1]
			}
			prev = end
			continue
		}
		if key, ok := tok.(string); ok && len(stack) > 0 {
			top := &stack[len(stack)-1]
			if top.kind == '{' && top.expectKey {
				top.currentKey = key
				top.expectKey = false
				prev = end
				continue
			}
		}
		path, isValue := consumeJSONValuePath(stack)
		if !isValue {
			prev = end
			continue
		}
		start, trimmedEnd := jsonTokenSpan(content, prev, end)
		startLine, startColumn := lineColumnForByteOffset(content, offsets, start)
		endLine, endColumn := lineColumnForByteOffset(content, offsets, trimmedEnd)
		raw := ""
		if start >= 0 && trimmedEnd >= start && trimmedEnd <= len(content) {
			raw = string(content[start:trimmedEnd])
		}
		out = append(out, StructuredScalar{
			Path:         path,
			Format:       StructuredFormatJSON,
			RawText:      raw,
			DecodedValue: jsonScalarDecodedValue(tok),
			NodeKind:     jsonScalarNodeKind(tok),
			StartByte:    start,
			EndByte:      trimmedEnd,
			StartLine:    startLine,
			StartColumn:  startColumn,
			EndLine:      endLine,
			EndColumn:    endColumn,
		})
		stack = updateJSONParentAfterScalar(stack)
		prev = end
	}
	return out, nil
}

func consumeJSONValuePath(stack []jsonContext) (string, bool) {
	if len(stack) == 0 {
		return "$", true
	}
	top := stack[len(stack)-1]
	switch top.kind {
	case '{':
		if top.expectKey || strings.TrimSpace(top.currentKey) == "" {
			return "", false
		}
		return joinStructuredPath(top.path, top.currentKey), true
	case '[':
		return fmt.Sprintf("%s[%d]", top.path, top.index), true
	default:
		return "", false
	}
}

func updateJSONParentAfterScalar(stack []jsonContext) []jsonContext {
	if len(stack) == 0 {
		return stack
	}
	top := &stack[len(stack)-1]
	switch top.kind {
	case '{':
		top.currentKey = ""
		top.expectKey = true
	case '[':
		top.index++
	}
	return stack
}

func updateJSONParentAfterContainer(stack []jsonContext) []jsonContext {
	return updateJSONParentAfterScalar(stack)
}

func joinStructuredPath(parent, child string) string {
	parent = strings.TrimSpace(parent)
	child = strings.TrimSpace(child)
	if parent == "" || parent == "$" {
		return child
	}
	if child == "" {
		return parent
	}
	return parent + "." + child
}

func jsonTokenSpan(content []byte, from, end int) (int, int) {
	if from < 0 {
		from = 0
	}
	if end > len(content) {
		end = len(content)
	}
	start := from
	for start < end {
		b := content[start]
		if b == ':' || b == ',' || unicode.IsSpace(rune(b)) {
			start++
			continue
		}
		break
	}
	trimmedEnd := end
	for trimmedEnd > start && unicode.IsSpace(rune(content[trimmedEnd-1])) {
		trimmedEnd--
	}
	return start, trimmedEnd
}

func jsonScalarDecodedValue(tok any) string {
	switch v := tok.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	case bool:
		if v {
			return "true"
		}
		return "false"
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

func jsonScalarNodeKind(tok any) string {
	switch v := tok.(type) {
	case string:
		return "scalar_string"
	case json.Number:
		if strings.ContainsAny(v.String(), ".eE") {
			return "scalar_float"
		}
		return "scalar_int"
	case bool:
		return "scalar_bool"
	case nil:
		return "scalar_null"
	default:
		return "scalar_string"
	}
}

func structuredScalarSegment(source ManifestSource, scalar StructuredScalar) SourceSegment {
	kind := KindYAMLScalar
	if scalar.Format == StructuredFormatJSON {
		kind = KindJSONScalar
	}
	raw := scalar.RawText
	if raw == "" && scalar.StartByte < scalar.EndByte {
		raw = scalar.DecodedValue
	}
	return SourceSegment{
		SegmentID:          segmentID(source.ID, scalar.StartByte, scalar.EndByte, kind),
		SourceID:           source.ID,
		SourcePath:         source.Path,
		Kind:               kind,
		Disposition:        DispositionCanonicalAtom,
		StartByte:          scalar.StartByte,
		EndByte:            scalar.EndByte,
		StartLine:          scalar.StartLine,
		StartColumn:        scalar.StartColumn,
		EndLine:            scalar.EndLine,
		EndColumn:          scalar.EndColumn,
		RawTextHash:        ComputeRawTextHash([]byte(raw)),
		NormalizedTextHash: ComputeNormalizedTextHash(scalar.DecodedValue),
		CanonicalUnitID:    structuredScalarUnitID(source.ID, scalar.Path),
		IncludeInFeed:      true,
		IncludeInRAG:       true,
	}
}

func buildStructuredCoverageSegments(source ManifestSource, content []byte, scalars []StructuredScalar) []SourceSegment {
	var out []SourceSegment
	offsets := computeYAMLLineOffsets(content)
	cursor := 0
	for _, scalar := range scalars {
		if scalar.StartByte < cursor || scalar.EndByte <= scalar.StartByte || scalar.EndByte > len(content) {
			continue
		}
		if cursor < scalar.StartByte {
			out = append(out, structuredCoverageSegment(source, content, offsets, cursor, scalar.StartByte))
		}
		out = append(out, scalar.Segment)
		cursor = scalar.EndByte
	}
	if cursor < len(content) {
		out = append(out, structuredCoverageSegment(source, content, offsets, cursor, len(content)))
	}
	return out
}

func structuredCoverageSegment(source ManifestSource, content []byte, offsets []int, start, end int) SourceSegment {
	startLine, startColumn := lineColumnForByteOffset(content, offsets, start)
	endLine, endColumn := lineColumnForByteOffset(content, offsets, end)
	return SourceSegment{
		SegmentID:   segmentID(source.ID, start, end, KindStructuredCoverage),
		SourceID:    source.ID,
		SourcePath:  source.Path,
		Kind:        KindStructuredCoverage,
		Disposition: DispositionCoverageOnly,
		StartByte:   start,
		EndByte:     end,
		StartLine:   startLine,
		StartColumn: startColumn,
		EndLine:     endLine,
		EndColumn:   endColumn,
	}
}

func structuredScalarUnitID(sourceID, path string) string {
	id := toUpperSlug("STRUCT-" + sourceID + "-" + path)
	if id == "" {
		return "STRUCT-SCALAR"
	}
	return id
}

func lineColumnForByteOffset(content []byte, offsets []int, offset int) (int, int) {
	if offset < 0 {
		offset = 0
	}
	if offset > len(content) {
		offset = len(content)
	}
	lineIdx := sort.Search(len(offsets), func(i int) bool {
		return offsets[i] > offset
	}) - 1
	if lineIdx < 0 {
		lineIdx = 0
	}
	return lineIdx + 1, offset - offsets[lineIdx] + 1
}
