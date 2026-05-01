package corpus

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strings"
)

// HeadingUnit represents a content unit extracted from a Markdown heading.
type HeadingUnit struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Level   int    `json:"level"`
	Title   string `json:"title"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

// ExtractMarkdownUnits parses a Markdown file and extracts units by heading (H1-H4).
// Each unit spans from its heading to the next heading of equal or higher level.
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
	scanner := bufio.NewScanner(r)
	var units []HeadingUnit
	var current *headingAccum
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		level, title := parseHeading(line)
		if level >= 1 && level <= 4 {
			if current != nil {
				units = append(units, current.toUnit(path))
			}
			current = &headingAccum{
				level: level,
				title: title,
				line:  lineNum,
			}
			continue
		}

		if current != nil {
			current.addLine(line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if current != nil {
		units = append(units, current.toUnit(path))
	}

	return units, nil
}

// UnitID generates a stable ID from path and heading title.
func UnitID(path, title string) string {
	raw := path + "#" + slugify(title)
	hash := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", hash[:8])
}

type headingAccum struct {
	level   int
	title   string
	line    int
	content strings.Builder
}

func (h *headingAccum) addLine(line string) {
	if h.content.Len() > 0 {
		h.content.WriteByte('\n')
	}
	h.content.WriteString(line)
}

func (h *headingAccum) toUnit(path string) HeadingUnit {
	return HeadingUnit{
		ID:      UnitID(path, h.title),
		Path:    path,
		Level:   h.level,
		Title:   h.title,
		Line:    h.line,
		Content: strings.TrimSpace(h.content.String()),
	}
}

func parseHeading(line string) (int, string) {
	trimmed := strings.TrimLeft(line, " ")
	// ATX headings only (# style)
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
	if level > 4 {
		return 0, ""
	}
	// Must have space after hashes (or be empty heading)
	rest := trimmed[level:]
	if len(rest) > 0 && rest[0] != ' ' {
		return 0, ""
	}
	title := strings.TrimSpace(rest)
	// Strip trailing hashes
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
