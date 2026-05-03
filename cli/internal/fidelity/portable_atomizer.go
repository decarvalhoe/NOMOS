package fidelity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// Profile defines the atomization rules for a specific domain.
type Profile struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Domain         string            `json:"domain"`
	HeadingAsAtom  bool              `json:"heading_as_atom"`
	ParagraphAsAtom bool             `json:"paragraph_as_atom"`
	ListItemAsAtom bool              `json:"list_item_as_atom"`
	MinTextLength  int               `json:"min_text_length"`
	MaxDepth       int               `json:"max_depth"`
	TypeMapping    map[string]string `json:"type_mapping,omitempty"`
}

// PredefinedProfiles returns built-in profiles for common domains.
func PredefinedProfiles() map[string]Profile {
	return map[string]Profile{
		"rbok": {
			ID: "rbok", Name: "RBOK Lawbook", Domain: "legal",
			HeadingAsAtom: true, ParagraphAsAtom: true, ListItemAsAtom: true,
			MinTextLength: 5, MaxDepth: 6,
			TypeMapping: map[string]string{
				"heading":   "clause",
				"paragraph": "rule",
				"list_item": "provision",
			},
		},
		"legal": {
			ID: "legal", Name: "Legal Document", Domain: "legal",
			HeadingAsAtom: true, ParagraphAsAtom: true, ListItemAsAtom: false,
			MinTextLength: 10, MaxDepth: 4,
			TypeMapping: map[string]string{
				"heading":   "section",
				"paragraph": "article",
			},
		},
		"game-rules": {
			ID: "game-rules", Name: "Game Rules", Domain: "gaming",
			HeadingAsAtom: true, ParagraphAsAtom: true, ListItemAsAtom: true,
			MinTextLength: 3, MaxDepth: 6,
			TypeMapping: map[string]string{
				"heading":   "category",
				"paragraph": "rule",
				"list_item": "condition",
			},
		},
	}
}

// SourceUnit is a generic input block for the portable atomizer.
type SourceUnit struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // heading, paragraph, list_item, code, table
	Text     string `json:"text"`
	Level    int    `json:"level"`
	ParentID string `json:"parent_id,omitempty"`
	Line     int    `json:"line"`
}

// PortableAtom is the output of the portable atomizer.
type PortableAtom struct {
	ID           string `json:"id"`
	CanonicalRef string `json:"canonical_ref"`
	Type         string `json:"type"`
	Text         string `json:"text"`
	ContentHash  string `json:"content_hash"`
	Depth        int    `json:"depth"`
	ParentID     string `json:"parent_id,omitempty"`
	Domain       string `json:"domain"`
	Profile      string `json:"profile"`
	SourceLine   int    `json:"source_line"`
}

// TraceEntry links an atom to its source and any downstream targets.
type TraceEntry struct {
	AtomID    string   `json:"atom_id"`
	SourceID  string   `json:"source_id"`
	ParentID  string   `json:"parent_id,omitempty"`
	Targets   []string `json:"targets,omitempty"`
	Depth     int      `json:"depth"`
}

// TraceMatrix is the traceability matrix mapping atoms to source units.
type TraceMatrix struct {
	ProfileID   string       `json:"profile_id"`
	TotalAtoms  int          `json:"total_atoms"`
	TotalSource int          `json:"total_source"`
	Coverage    float64      `json:"coverage"`
	Entries     []TraceEntry `json:"entries"`
	MatrixHash  string       `json:"matrix_hash"`
}

// AtomizationOutput combines atoms and their traceability matrix.
type AtomizationOutput struct {
	Atoms  []PortableAtom `json:"atoms"`
	Matrix TraceMatrix    `json:"matrix"`
}

// Atomize processes source units through a profile and produces atoms + trace matrix.
func Atomize(units []SourceUnit, profile Profile) AtomizationOutput {
	var atoms []PortableAtom
	emitted := map[string]bool{}

	for _, unit := range units {
		if !shouldEmit(unit, profile) {
			continue
		}

		atom := buildPortableAtom(unit, profile)
		atoms = append(atoms, atom)
		emitted[unit.ID] = true
	}

	matrix := buildTraceMatrix(units, atoms, profile, emitted)

	return AtomizationOutput{
		Atoms:  atoms,
		Matrix: matrix,
	}
}

func shouldEmit(unit SourceUnit, profile Profile) bool {
	if len(strings.TrimSpace(unit.Text)) < profile.MinTextLength {
		return false
	}
	if profile.MaxDepth > 0 && unit.Level > profile.MaxDepth {
		return false
	}
	switch unit.Type {
	case "heading":
		return profile.HeadingAsAtom
	case "paragraph":
		return profile.ParagraphAsAtom
	case "list_item":
		return profile.ListItemAsAtom
	default:
		return false
	}
}

func buildPortableAtom(unit SourceUnit, profile Profile) PortableAtom {
	atomType := profile.TypeMapping[unit.Type]
	if atomType == "" {
		atomType = unit.Type
	}

	text := strings.TrimSpace(unit.Text)
	hash := sha256.Sum256([]byte(text))

	return PortableAtom{
		ID:           fmt.Sprintf("%s.%s", strings.ToUpper(profile.ID), strings.ToUpper(unit.ID)),
		CanonicalRef: buildRef(unit),
		Type:         atomType,
		Text:         text,
		ContentHash:  "sha256:" + hex.EncodeToString(hash[:]),
		Depth:        unit.Level,
		ParentID:     unit.ParentID,
		Domain:       profile.Domain,
		Profile:      profile.ID,
		SourceLine:   unit.Line,
	}
}

func buildRef(unit SourceUnit) string {
	if unit.Type == "heading" {
		return slugRef(unit.Text)
	}
	return unit.ID
}

func slugRef(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	var b strings.Builder
	lastDash := false
	for _, r := range text {
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

func buildTraceMatrix(units []SourceUnit, atoms []PortableAtom, profile Profile, emitted map[string]bool) TraceMatrix {
	var entries []TraceEntry

	atomIndex := map[string]PortableAtom{}
	for _, a := range atoms {
		atomIndex[a.ID] = a
	}

	for _, atom := range atoms {
		// Find source unit.
		sourceID := strings.TrimPrefix(atom.ID, strings.ToUpper(profile.ID)+".")
		entry := TraceEntry{
			AtomID:   atom.ID,
			SourceID: sourceID,
			ParentID: atom.ParentID,
			Depth:    atom.Depth,
		}
		entries = append(entries, entry)
	}

	// Compute coverage: emitted / total eligible.
	eligible := 0
	for _, unit := range units {
		if shouldEmit(unit, profile) {
			eligible++
		}
	}
	coverage := 0.0
	if len(units) > 0 {
		coverage = float64(eligible) / float64(len(units))
	}

	matrixHash := hashMatrix(entries)

	return TraceMatrix{
		ProfileID:   profile.ID,
		TotalAtoms:  len(atoms),
		TotalSource: len(units),
		Coverage:    coverage,
		Entries:     entries,
		MatrixHash:  matrixHash,
	}
}

func hashMatrix(entries []TraceEntry) string {
	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString(e.AtomID)
		sb.WriteByte('|')
		sb.WriteString(e.SourceID)
		sb.WriteByte('|')
		sb.WriteString(e.ParentID)
		sb.WriteByte('\n')
	}
	h := sha256.Sum256([]byte(sb.String()))
	return "sha256:" + hex.EncodeToString(h[:])
}
