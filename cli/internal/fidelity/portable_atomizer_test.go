package fidelity

import (
	"strings"
	"testing"
)

func sampleUnits() []SourceUnit {
	return []SourceUnit{
		{ID: "H1", Type: "heading", Text: "Introduction", Level: 1, Line: 1},
		{ID: "P1", Type: "paragraph", Text: "This document defines the core rules.", Level: 1, ParentID: "H1", Line: 3},
		{ID: "H2", Type: "heading", Text: "Movement Rules", Level: 2, ParentID: "H1", Line: 5},
		{ID: "P2", Type: "paragraph", Text: "Each piece moves according to its type.", Level: 2, ParentID: "H2", Line: 7},
		{ID: "L1", Type: "list_item", Text: "Pawns move forward one square.", Level: 2, ParentID: "H2", Line: 9},
		{ID: "L2", Type: "list_item", Text: "Knights move in L-shape.", Level: 2, ParentID: "H2", Line: 10},
	}
}

func TestAtomizeWithGameRulesProfile(t *testing.T) {
	profiles := PredefinedProfiles()
	output := Atomize(sampleUnits(), profiles["game-rules"])

	if len(output.Atoms) != 6 {
		t.Fatalf("expected 6 atoms (all units emit), got %d", len(output.Atoms))
	}
}

func TestAtomizeWithLegalProfile(t *testing.T) {
	profiles := PredefinedProfiles()
	output := Atomize(sampleUnits(), profiles["legal"])

	// Legal profile: headings + paragraphs, no list items, min 10 chars.
	// H1 "Introduction" = 12 chars → emit
	// P1 = long → emit
	// H2 "Movement Rules" = 14 → emit
	// P2 = long → emit
	// L1, L2 = list items → skip
	if len(output.Atoms) != 4 {
		t.Fatalf("expected 4 atoms (legal skips list items), got %d", len(output.Atoms))
	}
}

func TestAtomizeWithRBOKProfile(t *testing.T) {
	profiles := PredefinedProfiles()
	output := Atomize(sampleUnits(), profiles["rbok"])

	if len(output.Atoms) != 6 {
		t.Fatalf("expected 6 atoms (rbok emits all), got %d", len(output.Atoms))
	}

	// Check type mapping.
	for _, atom := range output.Atoms {
		switch {
		case strings.Contains(atom.ID, "H1") || strings.Contains(atom.ID, "H2"):
			if atom.Type != "clause" {
				t.Fatalf("expected heading→clause, got %s for %s", atom.Type, atom.ID)
			}
		case strings.Contains(atom.ID, "P1") || strings.Contains(atom.ID, "P2"):
			if atom.Type != "rule" {
				t.Fatalf("expected paragraph→rule, got %s for %s", atom.Type, atom.ID)
			}
		case strings.Contains(atom.ID, "L1") || strings.Contains(atom.ID, "L2"):
			if atom.Type != "provision" {
				t.Fatalf("expected list_item→provision, got %s for %s", atom.Type, atom.ID)
			}
		}
	}
}

func TestAtomizeAtomIDs(t *testing.T) {
	profiles := PredefinedProfiles()
	output := Atomize(sampleUnits(), profiles["rbok"])

	for _, atom := range output.Atoms {
		if !strings.HasPrefix(atom.ID, "RBOK.") {
			t.Fatalf("expected RBOK. prefix, got %s", atom.ID)
		}
	}
}

func TestAtomizeContentHashes(t *testing.T) {
	profiles := PredefinedProfiles()
	output := Atomize(sampleUnits(), profiles["game-rules"])

	for _, atom := range output.Atoms {
		if !strings.HasPrefix(atom.ContentHash, "sha256:") {
			t.Fatalf("expected sha256: prefix, got %s", atom.ContentHash)
		}
		if len(atom.ContentHash) < 20 {
			t.Fatalf("hash too short: %s", atom.ContentHash)
		}
	}
}

func TestAtomizeCanonicalRefs(t *testing.T) {
	profiles := PredefinedProfiles()
	output := Atomize(sampleUnits(), profiles["game-rules"])

	for _, atom := range output.Atoms {
		if atom.CanonicalRef == "" {
			t.Fatalf("empty canonical_ref for %s", atom.ID)
		}
		if strings.Contains(atom.CanonicalRef, " ") {
			t.Fatalf("canonical_ref should be slug, got %s", atom.CanonicalRef)
		}
	}
}

func TestAtomizeTraceMatrix(t *testing.T) {
	profiles := PredefinedProfiles()
	output := Atomize(sampleUnits(), profiles["game-rules"])

	matrix := output.Matrix
	if matrix.TotalAtoms != 6 {
		t.Fatalf("expected 6 atoms in matrix, got %d", matrix.TotalAtoms)
	}
	if matrix.TotalSource != 6 {
		t.Fatalf("expected 6 source units, got %d", matrix.TotalSource)
	}
	if matrix.ProfileID != "game-rules" {
		t.Fatalf("expected profile game-rules, got %s", matrix.ProfileID)
	}
	if len(matrix.Entries) != 6 {
		t.Fatalf("expected 6 trace entries, got %d", len(matrix.Entries))
	}
}

func TestAtomizeTraceMatrixCoverage(t *testing.T) {
	profiles := PredefinedProfiles()
	output := Atomize(sampleUnits(), profiles["game-rules"])

	if output.Matrix.Coverage != 1.0 {
		t.Fatalf("expected 1.0 coverage (all emit), got %f", output.Matrix.Coverage)
	}
}

func TestAtomizeTraceMatrixPartialCoverage(t *testing.T) {
	profiles := PredefinedProfiles()
	output := Atomize(sampleUnits(), profiles["legal"])

	// 4 out of 6 emit → 4/6 eligible.
	if output.Matrix.Coverage >= 1.0 {
		t.Fatal("expected partial coverage for legal profile")
	}
	if output.Matrix.Coverage <= 0 {
		t.Fatal("expected some coverage")
	}
}

func TestAtomizeTraceMatrixHash(t *testing.T) {
	profiles := PredefinedProfiles()
	output := Atomize(sampleUnits(), profiles["rbok"])

	if !strings.HasPrefix(output.Matrix.MatrixHash, "sha256:") {
		t.Fatalf("expected sha256: prefix for matrix hash, got %s", output.Matrix.MatrixHash)
	}
}

func TestAtomizeTraceMatrixDeterministic(t *testing.T) {
	profiles := PredefinedProfiles()
	o1 := Atomize(sampleUnits(), profiles["rbok"])
	o2 := Atomize(sampleUnits(), profiles["rbok"])

	if o1.Matrix.MatrixHash != o2.Matrix.MatrixHash {
		t.Fatal("matrix hash should be deterministic")
	}
}

func TestAtomizeEmptyInput(t *testing.T) {
	profiles := PredefinedProfiles()
	output := Atomize(nil, profiles["rbok"])

	if len(output.Atoms) != 0 {
		t.Fatalf("expected 0 atoms for empty input, got %d", len(output.Atoms))
	}
	if output.Matrix.TotalAtoms != 0 {
		t.Fatal("expected 0 in matrix")
	}
}

func TestAtomizeMinTextLengthFilter(t *testing.T) {
	units := []SourceUnit{
		{ID: "short", Type: "paragraph", Text: "Hi", Level: 1, Line: 1},
		{ID: "long", Type: "paragraph", Text: "This is a sufficiently long paragraph.", Level: 1, Line: 2},
	}
	profile := Profile{
		ID: "strict", Name: "Strict", Domain: "test",
		ParagraphAsAtom: true, MinTextLength: 10,
	}
	output := Atomize(units, profile)

	if len(output.Atoms) != 1 {
		t.Fatalf("expected 1 atom (short filtered), got %d", len(output.Atoms))
	}
}

func TestAtomizeMaxDepthFilter(t *testing.T) {
	units := []SourceUnit{
		{ID: "H1", Type: "heading", Text: "Top Level", Level: 1, Line: 1},
		{ID: "H5", Type: "heading", Text: "Very Deep", Level: 5, Line: 3},
	}
	profile := Profile{
		ID: "shallow", Name: "Shallow", Domain: "test",
		HeadingAsAtom: true, MinTextLength: 3, MaxDepth: 3,
	}
	output := Atomize(units, profile)

	if len(output.Atoms) != 1 {
		t.Fatalf("expected 1 atom (depth 5 filtered), got %d", len(output.Atoms))
	}
}

func TestAtomizeParentIDPreserved(t *testing.T) {
	profiles := PredefinedProfiles()
	output := Atomize(sampleUnits(), profiles["rbok"])

	for _, atom := range output.Atoms {
		if strings.Contains(atom.ID, "P1") {
			if atom.ParentID != "H1" {
				t.Fatalf("expected P1 parent H1, got %s", atom.ParentID)
			}
		}
	}
}

func TestAtomizeDomainFromProfile(t *testing.T) {
	profiles := PredefinedProfiles()
	output := Atomize(sampleUnits(), profiles["game-rules"])

	for _, atom := range output.Atoms {
		if atom.Domain != "gaming" {
			t.Fatalf("expected domain gaming, got %s", atom.Domain)
		}
	}
}

func TestPredefinedProfilesExist(t *testing.T) {
	profiles := PredefinedProfiles()
	for _, id := range []string{"rbok", "legal", "game-rules"} {
		if _, ok := profiles[id]; !ok {
			t.Fatalf("expected profile %s", id)
		}
	}
}

func TestAtomizeSourceLinePreserved(t *testing.T) {
	profiles := PredefinedProfiles()
	output := Atomize(sampleUnits(), profiles["rbok"])

	for _, atom := range output.Atoms {
		if atom.SourceLine == 0 {
			t.Fatalf("expected non-zero source line for %s", atom.ID)
		}
	}
}
