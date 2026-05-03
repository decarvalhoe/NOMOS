package validation_packs

import (
	"embed"
	"fmt"
	"strings"

	"github.com/RBOKproject/Nomos/cli/internal/fidelity"
)

//go:embed testdata/*
var fixtureFS embed.FS

// PackID identifies a validation pack.
type PackID string

const (
	PackRBOK      PackID = "rbok"
	PackLegal     PackID = "legal"
	PackGameRules PackID = "game-rules"
)

// Expectation defines what a validation pack expects from atomization.
type Expectation struct {
	MinAtoms       int
	MinHeadings    int
	MinParagraphs  int
	MinListItems   int
	RequiredRefs   []string
	MaxDepth       int
	ProfileID      string
	Domain         string
}

// ValidationResult reports whether a pack passed.
type ValidationResult struct {
	PackID     PackID   `json:"pack_id"`
	Pass       bool     `json:"pass"`
	AtomCount  int      `json:"atom_count"`
	Failures   []string `json:"failures,omitempty"`
}

// Pack bundles a fixture with its expectations and profile.
type Pack struct {
	ID          PackID
	FixturePath string
	Profile     fidelity.Profile
	Expected    Expectation
}

// AllPacks returns the complete set of validation packs.
func AllPacks() []Pack {
	profiles := fidelity.PredefinedProfiles()
	return []Pack{
		{
			ID:          PackRBOK,
			FixturePath: "testdata/rbok/fixture.md",
			Profile:     profiles["rbok"],
			Expected: Expectation{
				MinAtoms:      8,
				MinHeadings:   5,
				MinParagraphs: 3,
				MinListItems:  2,
				RequiredRefs:  []string{"article-l-111-1", "article-l-111-2", "article-l-121-1"},
				MaxDepth:      3,
				ProfileID:     "rbok",
				Domain:        "legal",
			},
		},
		{
			ID:          PackLegal,
			FixturePath: "testdata/legal/fixture.md",
			Profile:     profiles["legal"],
			Expected: Expectation{
				MinAtoms:      6,
				MinHeadings:   5,
				MinParagraphs: 3,
				MinListItems:  0,
				RequiredRefs:  []string{"article-1-subject-matter-and-objectives", "article-5-principles-relating-to-processing-of-personal-data"},
				MaxDepth:      3,
				ProfileID:     "legal",
				Domain:        "legal",
			},
		},
		{
			ID:          PackGameRules,
			FixturePath: "testdata/game-rules/fixture.md",
			Profile:     profiles["game-rules"],
			Expected: Expectation{
				MinAtoms:      10,
				MinHeadings:   7,
				MinParagraphs: 3,
				MinListItems:  5,
				RequiredRefs:  []string{"king", "pawn", "castling"},
				MaxDepth:      3,
				ProfileID:     "game-rules",
				Domain:        "gaming",
			},
		},
	}
}

// LoadFixture reads a pack's fixture content from embedded FS.
func LoadFixture(pack Pack) (string, error) {
	data, err := fixtureFS.ReadFile(pack.FixturePath)
	if err != nil {
		return "", fmt.Errorf("load fixture %s: %w", pack.FixturePath, err)
	}
	return string(data), nil
}

// Validate runs a validation pack and checks expectations.
func Validate(pack Pack) (ValidationResult, error) {
	content, err := LoadFixture(pack)
	if err != nil {
		return ValidationResult{PackID: pack.ID, Pass: false}, err
	}

	units := parseToSourceUnits(content)
	output := fidelity.Atomize(units, pack.Profile)

	result := ValidationResult{
		PackID:    pack.ID,
		AtomCount: len(output.Atoms),
	}

	var failures []string

	if len(output.Atoms) < pack.Expected.MinAtoms {
		failures = append(failures, fmt.Sprintf("expected >= %d atoms, got %d", pack.Expected.MinAtoms, len(output.Atoms)))
	}

	headings, paragraphs, listItems := countTypes(output.Atoms)
	if headings < pack.Expected.MinHeadings {
		failures = append(failures, fmt.Sprintf("expected >= %d heading atoms, got %d", pack.Expected.MinHeadings, headings))
	}
	if paragraphs < pack.Expected.MinParagraphs {
		failures = append(failures, fmt.Sprintf("expected >= %d paragraph atoms, got %d", pack.Expected.MinParagraphs, paragraphs))
	}
	if listItems < pack.Expected.MinListItems {
		failures = append(failures, fmt.Sprintf("expected >= %d list_item atoms, got %d", pack.Expected.MinListItems, listItems))
	}

	for _, ref := range pack.Expected.RequiredRefs {
		if !hasRef(output.Atoms, ref) {
			failures = append(failures, fmt.Sprintf("missing required ref %q", ref))
		}
	}

	for _, atom := range output.Atoms {
		if atom.Domain != pack.Expected.Domain {
			failures = append(failures, fmt.Sprintf("atom %s has domain %q, expected %q", atom.ID, atom.Domain, pack.Expected.Domain))
			break
		}
	}

	result.Failures = failures
	result.Pass = len(failures) == 0
	return result, nil
}

// parseToSourceUnits converts markdown content to SourceUnit slice for portable atomizer.
func parseToSourceUnits(content string) []fidelity.SourceUnit {
	var units []fidelity.SourceUnit
	lines := strings.Split(content, "\n")
	var currentParent string
	lineNum := 0

	for _, line := range lines {
		lineNum++
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		level, title := parseHeadingLine(trimmed)
		if level > 0 {
			id := fmt.Sprintf("H%d-L%d", lineNum, level)
			units = append(units, fidelity.SourceUnit{
				ID: id, Type: "heading", Text: title, Level: level,
				ParentID: currentParent, Line: lineNum,
			})
			currentParent = id
			continue
		}

		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			text := strings.TrimPrefix(strings.TrimPrefix(trimmed, "- "), "* ")
			units = append(units, fidelity.SourceUnit{
				ID: fmt.Sprintf("LI%d", lineNum), Type: "list_item", Text: text,
				Level: 0, ParentID: currentParent, Line: lineNum,
			})
			continue
		}

		units = append(units, fidelity.SourceUnit{
			ID: fmt.Sprintf("P%d", lineNum), Type: "paragraph", Text: trimmed,
			Level: 0, ParentID: currentParent, Line: lineNum,
		})
	}
	return units
}

func parseHeadingLine(line string) (int, string) {
	if !strings.HasPrefix(line, "#") {
		return 0, ""
	}
	level := 0
	for _, ch := range line {
		if ch == '#' {
			level++
		} else {
			break
		}
	}
	if level > 6 || (len(line) > level && line[level] != ' ') {
		return 0, ""
	}
	return level, strings.TrimSpace(line[level:])
}

func countTypes(atoms []fidelity.PortableAtom) (headings, paragraphs, listItems int) {
	profiles := fidelity.PredefinedProfiles()
	headingTypes := map[string]bool{}
	paraTypes := map[string]bool{}
	listTypes := map[string]bool{}
	for _, p := range profiles {
		if t, ok := p.TypeMapping["heading"]; ok {
			headingTypes[t] = true
		}
		if t, ok := p.TypeMapping["paragraph"]; ok {
			paraTypes[t] = true
		}
		if t, ok := p.TypeMapping["list_item"]; ok {
			listTypes[t] = true
		}
	}
	// Also match raw types.
	headingTypes["heading"] = true
	paraTypes["paragraph"] = true
	listTypes["list_item"] = true

	for _, atom := range atoms {
		if headingTypes[atom.Type] {
			headings++
		} else if paraTypes[atom.Type] {
			paragraphs++
		} else if listTypes[atom.Type] {
			listItems++
		}
	}
	return
}

func hasRef(atoms []fidelity.PortableAtom, ref string) bool {
	for _, atom := range atoms {
		if strings.Contains(atom.CanonicalRef, ref) {
			return true
		}
	}
	return false
}
