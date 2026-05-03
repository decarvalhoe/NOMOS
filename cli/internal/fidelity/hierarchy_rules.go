package fidelity

import (
	"fmt"
	"sort"
	"strings"
)

// HierarchyLevel defines one level in a profile's document hierarchy.
type HierarchyLevel struct {
	Name     string `json:"name"`
	Depth    int    `json:"depth"`
	MDLevel  int    `json:"md_level"`  // Markdown heading level (1-6), 0 = not heading-mapped
	Required bool   `json:"required"`
	Atomic   bool   `json:"atomic"`    // true = leaf level producing atoms
}

// HierarchyProfile defines the mapping rules for a document profile.
type HierarchyProfile struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Levels      []HierarchyLevel `json:"levels"`
}

// Depth returns the depth for a named level, or -1 if not found.
func (p HierarchyProfile) Depth(levelName string) int {
	for _, l := range p.Levels {
		if strings.EqualFold(l.Name, levelName) {
			return l.Depth
		}
	}
	return -1
}

// LevelForMD returns the hierarchy level mapped to a Markdown heading level.
func (p HierarchyProfile) LevelForMD(mdLevel int) (HierarchyLevel, bool) {
	for _, l := range p.Levels {
		if l.MDLevel == mdLevel {
			return l, true
		}
	}
	return HierarchyLevel{}, false
}

// LevelByDepth returns the hierarchy level at a given depth.
func (p HierarchyProfile) LevelByDepth(depth int) (HierarchyLevel, bool) {
	for _, l := range p.Levels {
		if l.Depth == depth {
			return l, true
		}
	}
	return HierarchyLevel{}, false
}

// MaxDepth returns the deepest level's depth.
func (p HierarchyProfile) MaxDepth() int {
	max := 0
	for _, l := range p.Levels {
		if l.Depth > max {
			max = l.Depth
		}
	}
	return max
}

// AtomicLevels returns the levels that produce atoms.
func (p HierarchyProfile) AtomicLevels() []HierarchyLevel {
	var result []HierarchyLevel
	for _, l := range p.Levels {
		if l.Atomic {
			result = append(result, l)
		}
	}
	return result
}

// ValidateNesting checks that a parent→child depth pair is valid
// according to the profile's hierarchy.
func (p HierarchyProfile) ValidateNesting(parentDepth, childDepth int) error {
	if childDepth <= parentDepth {
		return fmt.Errorf("child depth %d must be greater than parent depth %d", childDepth, parentDepth)
	}
	parentLevel, parentOK := p.LevelByDepth(parentDepth)
	childLevel, childOK := p.LevelByDepth(childDepth)
	if !parentOK {
		return fmt.Errorf("no level defined at depth %d", parentDepth)
	}
	if !childOK {
		return fmt.Errorf("no level defined at depth %d", childDepth)
	}
	_ = parentLevel
	_ = childLevel
	return nil
}

// --- Built-in profiles ---

// RBOKProfile returns the RBOK lawbook hierarchy profile.
// document > chapter > section > subsection > article > paragraph > alinea
func RBOKProfile() HierarchyProfile {
	return HierarchyProfile{
		Name:        "rbok",
		Description: "RBOK lawbook hierarchy: document > chapter > section > subsection > article > paragraph > alinea",
		Levels: []HierarchyLevel{
			{Name: "document", Depth: 0, MDLevel: 1, Required: true, Atomic: false},
			{Name: "chapter", Depth: 1, MDLevel: 2, Required: true, Atomic: false},
			{Name: "section", Depth: 2, MDLevel: 3, Required: false, Atomic: false},
			{Name: "subsection", Depth: 3, MDLevel: 4, Required: false, Atomic: false},
			{Name: "article", Depth: 4, MDLevel: 0, Required: false, Atomic: true},
			{Name: "paragraph", Depth: 5, MDLevel: 0, Required: false, Atomic: true},
			{Name: "alinea", Depth: 6, MDLevel: 0, Required: false, Atomic: true},
		},
	}
}

// LegalProfile returns the legal/regulatory document hierarchy.
// part > title > subtitle > chapter > section > article
func LegalProfile() HierarchyProfile {
	return HierarchyProfile{
		Name:        "legal",
		Description: "Legal/regulatory hierarchy: part > title > subtitle > chapter > section > article",
		Levels: []HierarchyLevel{
			{Name: "part", Depth: 0, MDLevel: 1, Required: true, Atomic: false},
			{Name: "title", Depth: 1, MDLevel: 2, Required: true, Atomic: false},
			{Name: "subtitle", Depth: 2, MDLevel: 3, Required: false, Atomic: false},
			{Name: "chapter", Depth: 3, MDLevel: 4, Required: false, Atomic: false},
			{Name: "section", Depth: 4, MDLevel: 5, Required: false, Atomic: false},
			{Name: "article", Depth: 5, MDLevel: 6, Required: false, Atomic: true},
		},
	}
}

// TechnicalProfile returns a technical documentation hierarchy.
// document > chapter > section > subsection
func TechnicalProfile() HierarchyProfile {
	return HierarchyProfile{
		Name:        "technical",
		Description: "Technical documentation: document > chapter > section > subsection",
		Levels: []HierarchyLevel{
			{Name: "document", Depth: 0, MDLevel: 1, Required: true, Atomic: false},
			{Name: "chapter", Depth: 1, MDLevel: 2, Required: true, Atomic: false},
			{Name: "section", Depth: 2, MDLevel: 3, Required: false, Atomic: true},
			{Name: "subsection", Depth: 3, MDLevel: 4, Required: false, Atomic: true},
		},
	}
}

// FlatProfile returns a flat hierarchy (single document level).
func FlatProfile() HierarchyProfile {
	return HierarchyProfile{
		Name:        "flat",
		Description: "Flat hierarchy: all content at document level",
		Levels: []HierarchyLevel{
			{Name: "document", Depth: 0, MDLevel: 0, Required: true, Atomic: true},
		},
	}
}

// --- Profile registry ---

var builtinProfiles = map[string]func() HierarchyProfile{
	"rbok":      RBOKProfile,
	"legal":     LegalProfile,
	"technical": TechnicalProfile,
	"flat":      FlatProfile,
}

// LookupHierarchyProfile returns a built-in profile by name.
func LookupHierarchyProfile(name string) (HierarchyProfile, error) {
	fn, ok := builtinProfiles[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return HierarchyProfile{}, fmt.Errorf("unknown hierarchy profile %q; known: %s",
			name, strings.Join(KnownHierarchyProfiles(), ", "))
	}
	return fn(), nil
}

// KnownHierarchyProfiles returns sorted profile names.
func KnownHierarchyProfiles() []string {
	names := make([]string, 0, len(builtinProfiles))
	for n := range builtinProfiles {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// --- Hierarchy validation ---

// HierarchyFinding describes a nesting or structure violation.
type HierarchyFinding struct {
	Level    string `json:"level"`
	Depth    int    `json:"depth"`
	Line     int    `json:"line,omitempty"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Blocking bool   `json:"blocking"`
}

const (
	CodeSkippedLevel   = "SKIPPED_LEVEL"
	CodeInvalidNesting = "INVALID_NESTING"
	CodeMissingRequired = "MISSING_REQUIRED_LEVEL"
)

// HierarchyCheckResult holds the outcome of hierarchy validation.
type HierarchyCheckResult struct {
	Profile  string             `json:"profile"`
	Pass     bool               `json:"pass"`
	Findings []HierarchyFinding `json:"findings,omitempty"`
}

// NodeDepthEntry represents a node's depth and line for hierarchy checking.
type NodeDepthEntry struct {
	LevelName string
	Depth     int
	Line      int
}

// CheckHierarchy validates a sequence of node depths against a profile.
func CheckHierarchy(profile HierarchyProfile, nodes []NodeDepthEntry) HierarchyCheckResult {
	result := HierarchyCheckResult{Profile: profile.Name, Pass: true}

	// Check required levels are present.
	present := map[int]bool{}
	for _, n := range nodes {
		present[n.Depth] = true
	}
	for _, level := range profile.Levels {
		if level.Required && !present[level.Depth] {
			result.Findings = append(result.Findings, HierarchyFinding{
				Level: level.Name, Depth: level.Depth,
				Code:     CodeMissingRequired,
				Message:  fmt.Sprintf("required level %q (depth %d) not found", level.Name, level.Depth),
				Blocking: true,
			})
			result.Pass = false
		}
	}

	// Check nesting: no skipped levels, child always deeper than parent.
	prevDepth := -1
	for _, n := range nodes {
		if prevDepth >= 0 && n.Depth > prevDepth+1 {
			// Check if intermediate levels exist in profile.
			skipped := []string{}
			for d := prevDepth + 1; d < n.Depth; d++ {
				if l, ok := profile.LevelByDepth(d); ok {
					skipped = append(skipped, l.Name)
				}
			}
			if len(skipped) > 0 {
				result.Findings = append(result.Findings, HierarchyFinding{
					Level: n.LevelName, Depth: n.Depth, Line: n.Line,
					Code:    CodeSkippedLevel,
					Message: fmt.Sprintf("skipped level(s) %s between depth %d and %d", strings.Join(skipped, ", "), prevDepth, n.Depth),
				})
			}
		}
		if prevDepth >= 0 && n.Depth < prevDepth && n.Depth > 0 {
			// Going back up is fine (sibling or uncle).
		}
		prevDepth = n.Depth
	}

	return result
}
