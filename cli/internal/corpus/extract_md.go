package corpus

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strings"
)

// HeadingUnit represents one unit produced by Markdown extraction.
//
// SFI-03 (#341): a HeadingUnit is either a structural heading entry
// (Kind == "heading") with no body text, or a canonical semantic leaf
// (Kind == one of the typed-scanner canonical-atom kinds: "paragraph",
// "list_item", "callout") whose Content is its own bytes from the
// source. The same source byte span never appears in two HeadingUnit
// records: heading entries do not carry descendant body content, so
// downstream consumers cannot create two canonical atoms over one span.
type HeadingUnit struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Level   int    `json:"level"`
	Title   string `json:"title"`
	Line    int    `json:"line"`
	Content string `json:"content"`

	// Kind is "heading" for structural heading entries, or the
	// typed-scanner canonical-atom kind for semantic leaves.
	Kind string `json:"kind,omitempty"`
	// HeadingPath is the chain of enclosing heading titles, root-first.
	// For a heading entry it lists strict ancestors (excluding the
	// heading itself). For a semantic leaf it lists every enclosing
	// heading title up to and including the nearest heading.
	HeadingPath []string `json:"heading_path,omitempty"`
	// StartByte and EndByte locate the unit's exact source byte range.
	StartByte int `json:"start_byte,omitempty"`
	EndByte   int `json:"end_byte,omitempty"`
}

// HeadingUnitKindHeading is the Kind value used for structural heading
// entries (no body text). Semantic leaves use the typed-scanner kind
// of the underlying canonical atom.
const HeadingUnitKindHeading = "heading"

// IsSemanticLeaf reports whether u carries semantic body text rather
// than structural heading context. SFI-03 (#341) consumers that want
// only canonical semantic atoms should filter on this predicate.
func (u HeadingUnit) IsSemanticLeaf() bool {
	return u.Kind != "" && u.Kind != HeadingUnitKindHeading
}

// ExtractMarkdownUnits parses a Markdown file and extracts both
// structural heading entries (H1-H6) and the canonical semantic
// leaves underneath them (paragraphs, list items, callouts).
//
// SFI-03 (#341): heading entries do not own descendant body bytes;
// each semantic leaf owns its own bytes exactly once. The same source
// byte range cannot appear in two HeadingUnit records.
func ExtractMarkdownUnits(path string) ([]HeadingUnit, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return extractFromReader(path, f)
}

// ExtractMarkdownUnitsFromReader parses Markdown content from a reader.
func ExtractMarkdownUnitsFromReader(path string, r io.Reader) ([]HeadingUnit, error) {
	return extractFromReader(path, r)
}

func extractFromReader(path string, r io.Reader) ([]HeadingUnit, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return extractFromBytes(path, raw)
}

func extractFromBytes(path string, content []byte) ([]HeadingUnit, error) {
	if len(content) == 0 {
		return nil, nil
	}
	scanID := path
	if strings.TrimSpace(scanID) == "" {
		scanID = "extract"
	}
	segments, err := ScanMarkdown(scanID, scanID, content)
	if err != nil {
		return nil, err
	}

	type headingFrame struct {
		level int
		title string
	}
	var stack []headingFrame
	cloneAncestry := func() []string {
		if len(stack) == 0 {
			return nil
		}
		out := make([]string, len(stack))
		for i, f := range stack {
			out[i] = f.title
		}
		return out
	}

	var units []HeadingUnit
	for _, seg := range segments {
		switch seg.Kind {
		case KindHeading:
			if seg.ParentSegmentID != "" {
				continue
			}
			level, title := parseHeadingLevelTitle(string(content[seg.StartByte:seg.EndByte]))
			if level < 1 || level > 6 || strings.TrimSpace(title) == "" {
				continue
			}
			for len(stack) > 0 && stack[len(stack)-1].level >= level {
				stack = stack[:len(stack)-1]
			}
			ancestors := cloneAncestry()
			stack = append(stack, headingFrame{level: level, title: title})
			units = append(units, HeadingUnit{
				ID:          UnitID(path, title),
				Path:        path,
				Level:       level,
				Title:       title,
				Line:        seg.StartLine,
				Content:     "",
				Kind:        HeadingUnitKindHeading,
				HeadingPath: ancestors,
				StartByte:   seg.StartByte,
				EndByte:     seg.EndByte,
			})
		case KindParagraph, KindListItem, KindCallout:
			if seg.Disposition != DispositionCanonicalAtom {
				continue
			}
			// Only emit semantic leaves that sit inside a heading
			// scope. Pre-heading text is intentionally out of scope
			// for SFI-03 (#341); SFI-04 will surface it as a coverage
			// finding via the source-integrity gate.
			if len(stack) == 0 {
				continue
			}
			top := stack[len(stack)-1]
			ancestors := cloneAncestry()
			text := strings.TrimRight(string(content[seg.StartByte:seg.EndByte]), "\n")
			units = append(units, HeadingUnit{
				ID:          unitIDLeaf(path, top.title, seg.Kind, seg.StartLine),
				Path:        path,
				Level:       top.level,
				Title:       top.title,
				Line:        seg.StartLine,
				Content:     text,
				Kind:        seg.Kind,
				HeadingPath: ancestors,
				StartByte:   seg.StartByte,
				EndByte:     seg.EndByte,
			})
		}
	}

	return units, nil
}

// UnitID generates a stable ID from path and heading title. It is
// preserved unchanged from the pre-SFI-03 surface so callers that
// reference the same heading across content edits get the same ID.
func UnitID(path, title string) string {
	raw := path + "#" + slugify(title)
	hash := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", hash[:8])
}

// unitIDLeaf generates a stable ID for a non-heading semantic leaf.
// Multiple leaves under the same heading would collide on UnitID
// alone, so the leaf ID also encodes the typed-scanner kind and the
// leaf's start line.
func unitIDLeaf(path, parentTitle, kind string, line int) string {
	raw := fmt.Sprintf("%s#%s:%s:%d", path, slugify(parentTitle), kind, line)
	hash := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", hash[:8])
}

// parseHeadingLevelTitle parses an ATX heading line (1-6 hashes) and
// returns the level and trimmed title. If the line is not a valid
// heading the returned level is 0.
func parseHeadingLevelTitle(line string) (int, string) {
	line = strings.TrimRight(line, "\r\n")
	trimmed := strings.TrimLeft(line, " ")
	if !strings.HasPrefix(trimmed, "#") {
		return 0, ""
	}
	level := 0
	for _, ch := range trimmed {
		if ch == '#' {
			level++
		} else {
			break
		}
	}
	if level < 1 || level > 6 {
		return 0, ""
	}
	rest := trimmed[level:]
	if len(rest) > 0 && rest[0] != ' ' && rest[0] != '\t' {
		return 0, ""
	}
	title := strings.TrimSpace(rest)
	title = strings.TrimRight(title, "# ")
	return level, title
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
