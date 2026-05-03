package corpus

import (
	"bytes"
	"strings"
	"unicode/utf8"

	parsemodel "github.com/RBOKproject/Nomos/cli/internal/corpus/parse"
)

type sourceSpanMatch struct {
	Span      *parsemodel.SourceSpan
	Precision string
}

func fullFileSourceSpan(sourceID, rel, sourceHash string, data []byte) *parsemodel.SourceSpan {
	startLine, startColumn := lineColumnForByteOffset(data, 0)
	endLine, endColumn := lineColumnForByteOffset(data, len(data))
	startByte := 0
	endByte := len(data)
	exact := strings.TrimSpace(string(data))
	span := &parsemodel.SourceSpan{
		SourceID:    sourceID,
		Path:        rel,
		Hash:        sourceHash,
		StartLine:   &startLine,
		EndLine:     &endLine,
		StartColumn: &startColumn,
		EndColumn:   &endColumn,
		StartByte:   &startByte,
		EndByte:     &endByte,
		Locator:     "",
		TextQuote: &parsemodel.TextQuoteSelector{
			Exact: exact,
		},
	}
	span.Locator = sourceSpanLocator(rel, span)
	return span
}

func exactSourceSpanForNeedles(sourceID, rel, sourceHash string, data []byte, needles ...string) sourceSpanMatch {
	for _, needle := range needles {
		needle = strings.TrimSpace(needle)
		if needle == "" {
			continue
		}
		needleBytes := []byte(needle)
		startByte := bytes.Index(data, needleBytes)
		if startByte < 0 {
			continue
		}
		endByte := startByte + len(needleBytes)
		startLine, startColumn := lineColumnForByteOffset(data, startByte)
		endLine, endColumn := lineColumnForByteOffset(data, endByte)
		span := &parsemodel.SourceSpan{
			SourceID:    sourceID,
			Path:        rel,
			Hash:        sourceHash,
			StartLine:   &startLine,
			EndLine:     &endLine,
			StartColumn: &startColumn,
			EndColumn:   &endColumn,
			StartByte:   &startByte,
			EndByte:     &endByte,
			TextQuote: &parsemodel.TextQuoteSelector{
				Exact: needle,
			},
		}
		span.Locator = sourceSpanLocator(rel, span)
		return sourceSpanMatch{Span: span, Precision: "exact_text_quote"}
	}
	return sourceSpanMatch{
		Span:      fullFileSourceSpan(sourceID, rel, sourceHash, data),
		Precision: "file_fallback",
	}
}

func lineColumnForByteOffset(data []byte, offset int) (int, int) {
	if offset < 0 {
		offset = 0
	}
	if offset > len(data) {
		offset = len(data)
	}
	line := 1
	column := 1
	i := 0
	for i < offset {
		switch data[i] {
		case '\r':
			line++
			column = 1
			i++
			if i < offset && data[i] == '\n' {
				i++
			}
		case '\n':
			line++
			column = 1
			i++
		default:
			r, size := utf8.DecodeRune(data[i:])
			if r == utf8.RuneError && size == 0 {
				size = 1
			}
			column++
			i += size
		}
	}
	return line, column
}
