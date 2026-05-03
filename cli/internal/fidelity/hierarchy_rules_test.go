package fidelity

import (
	"testing"
)

// --- RBOK profile ---

func TestRBOKProfileLevels(t *testing.T) {
	p := RBOKProfile()
	if p.Name != "rbok" {
		t.Fatalf("name: %q", p.Name)
	}
	if len(p.Levels) != 7 {
		t.Fatalf("expected 7 levels, got %d", len(p.Levels))
	}
}

func TestRBOKProfileDepths(t *testing.T) {
	p := RBOKProfile()
	cases := map[string]int{
		"document": 0, "chapter": 1, "section": 2, "subsection": 3,
		"article": 4, "paragraph": 5, "alinea": 6,
	}
	for name, want := range cases {
		got := p.Depth(name)
		if got != want {
			t.Fatalf("%s: expected depth %d, got %d", name, want, got)
		}
	}
}

func TestRBOKProfileMDMapping(t *testing.T) {
	p := RBOKProfile()
	level, ok := p.LevelForMD(1)
	if !ok || level.Name != "document" {
		t.Fatalf("MD level 1 should map to document, got %v", level)
	}
	level, ok = p.LevelForMD(2)
	if !ok || level.Name != "chapter" {
		t.Fatalf("MD level 2 should map to chapter, got %v", level)
	}
	_, ok = p.LevelForMD(5)
	if ok {
		t.Fatal("MD level 5 should not map in RBOK profile")
	}
}

func TestRBOKProfileAtomicLevels(t *testing.T) {
	p := RBOKProfile()
	atomic := p.AtomicLevels()
	if len(atomic) != 3 {
		t.Fatalf("expected 3 atomic levels, got %d", len(atomic))
	}
	names := map[string]bool{}
	for _, l := range atomic {
		names[l.Name] = true
	}
	for _, want := range []string{"article", "paragraph", "alinea"} {
		if !names[want] {
			t.Fatalf("expected %q as atomic", want)
		}
	}
}

func TestRBOKProfileMaxDepth(t *testing.T) {
	p := RBOKProfile()
	if p.MaxDepth() != 6 {
		t.Fatalf("max depth: %d", p.MaxDepth())
	}
}

// --- Legal profile ---

func TestLegalProfileLevels(t *testing.T) {
	p := LegalProfile()
	if p.Name != "legal" {
		t.Fatalf("name: %q", p.Name)
	}
	if len(p.Levels) != 6 {
		t.Fatalf("expected 6 levels, got %d", len(p.Levels))
	}
}

func TestLegalProfileDepths(t *testing.T) {
	p := LegalProfile()
	cases := map[string]int{
		"part": 0, "title": 1, "subtitle": 2, "chapter": 3, "section": 4, "article": 5,
	}
	for name, want := range cases {
		got := p.Depth(name)
		if got != want {
			t.Fatalf("%s: expected %d, got %d", name, want, got)
		}
	}
}

func TestLegalProfileMDMapping(t *testing.T) {
	p := LegalProfile()
	level, ok := p.LevelForMD(1)
	if !ok || level.Name != "part" {
		t.Fatalf("MD 1 should map to part")
	}
	level, ok = p.LevelForMD(6)
	if !ok || level.Name != "article" {
		t.Fatalf("MD 6 should map to article")
	}
}

func TestLegalProfileAtomicLevels(t *testing.T) {
	p := LegalProfile()
	atomic := p.AtomicLevels()
	if len(atomic) != 1 || atomic[0].Name != "article" {
		t.Fatalf("legal atomic: %v", atomic)
	}
}

// --- Technical profile ---

func TestTechnicalProfileLevels(t *testing.T) {
	p := TechnicalProfile()
	if len(p.Levels) != 4 {
		t.Fatalf("expected 4 levels, got %d", len(p.Levels))
	}
}

func TestTechnicalAtomicLevels(t *testing.T) {
	p := TechnicalProfile()
	atomic := p.AtomicLevels()
	if len(atomic) != 2 {
		t.Fatalf("expected 2 atomic, got %d", len(atomic))
	}
}

// --- Flat profile ---

func TestFlatProfileSingleLevel(t *testing.T) {
	p := FlatProfile()
	if len(p.Levels) != 1 {
		t.Fatalf("expected 1 level, got %d", len(p.Levels))
	}
	if !p.Levels[0].Atomic {
		t.Fatal("flat document should be atomic")
	}
}

// --- Lookup ---

func TestLookupHierarchyProfile(t *testing.T) {
	for _, name := range []string{"rbok", "legal", "technical", "flat"} {
		p, err := LookupHierarchyProfile(name)
		if err != nil {
			t.Fatalf("lookup %q: %v", name, err)
		}
		if p.Name != name {
			t.Fatalf("expected %q, got %q", name, p.Name)
		}
	}
}

func TestLookupHierarchyCaseInsensitive(t *testing.T) {
	p, err := LookupHierarchyProfile("RBOK")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if p.Name != "rbok" {
		t.Fatalf("name: %q", p.Name)
	}
}

func TestLookupHierarchyUnknown(t *testing.T) {
	_, err := LookupHierarchyProfile("custom")
	if err == nil {
		t.Fatal("expected error for unknown profile")
	}
}

func TestKnownHierarchyProfiles(t *testing.T) {
	names := KnownHierarchyProfiles()
	if len(names) != 4 {
		t.Fatalf("expected 4, got %d: %v", len(names), names)
	}
	// Should be sorted.
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Fatalf("not sorted: %v", names)
		}
	}
}

// --- Depth and level queries ---

func TestDepthUnknownLevel(t *testing.T) {
	p := RBOKProfile()
	if p.Depth("nonexistent") != -1 {
		t.Fatal("unknown level should return -1")
	}
}

func TestLevelByDepth(t *testing.T) {
	p := RBOKProfile()
	l, ok := p.LevelByDepth(4)
	if !ok || l.Name != "article" {
		t.Fatalf("depth 4: %v", l)
	}
	_, ok = p.LevelByDepth(99)
	if ok {
		t.Fatal("depth 99 should not exist")
	}
}

// --- ValidateNesting ---

func TestValidateNestingValid(t *testing.T) {
	p := RBOKProfile()
	if err := p.ValidateNesting(0, 1); err != nil {
		t.Fatalf("0→1 should be valid: %v", err)
	}
}

func TestValidateNestingInvalid(t *testing.T) {
	p := RBOKProfile()
	if err := p.ValidateNesting(2, 1); err == nil {
		t.Fatal("2→1 should be invalid")
	}
}

func TestValidateNestingSameDepth(t *testing.T) {
	p := RBOKProfile()
	if err := p.ValidateNesting(2, 2); err == nil {
		t.Fatal("same depth should be invalid")
	}
}

// --- CheckHierarchy ---

func TestCheckHierarchyValid(t *testing.T) {
	p := RBOKProfile()
	nodes := []NodeDepthEntry{
		{LevelName: "document", Depth: 0, Line: 1},
		{LevelName: "chapter", Depth: 1, Line: 3},
		{LevelName: "section", Depth: 2, Line: 5},
		{LevelName: "article", Depth: 4, Line: 10},
	}
	result := CheckHierarchy(p, nodes)
	if !result.Pass {
		t.Fatalf("expected pass: %v", result.Findings)
	}
}

func TestCheckHierarchyMissingRequired(t *testing.T) {
	p := RBOKProfile()
	// Missing document (depth 0) and chapter (depth 1).
	nodes := []NodeDepthEntry{
		{LevelName: "section", Depth: 2, Line: 1},
	}
	result := CheckHierarchy(p, nodes)
	if result.Pass {
		t.Fatal("should fail: missing required levels")
	}
	found := 0
	for _, f := range result.Findings {
		if f.Code == CodeMissingRequired {
			found++
		}
	}
	if found < 2 {
		t.Fatalf("expected at least 2 MISSING_REQUIRED findings, got %d", found)
	}
}

func TestCheckHierarchySkippedLevel(t *testing.T) {
	p := RBOKProfile()
	nodes := []NodeDepthEntry{
		{LevelName: "document", Depth: 0, Line: 1},
		{LevelName: "chapter", Depth: 1, Line: 3},
		{LevelName: "article", Depth: 4, Line: 5}, // skips section(2), subsection(3)
	}
	result := CheckHierarchy(p, nodes)
	found := false
	for _, f := range result.Findings {
		if f.Code == CodeSkippedLevel {
			found = true
		}
	}
	if !found {
		t.Fatal("expected SKIPPED_LEVEL finding")
	}
}

func TestCheckHierarchyLegalProfile(t *testing.T) {
	p := LegalProfile()
	nodes := []NodeDepthEntry{
		{LevelName: "part", Depth: 0, Line: 1},
		{LevelName: "title", Depth: 1, Line: 5},
		{LevelName: "chapter", Depth: 3, Line: 10},
		{LevelName: "article", Depth: 5, Line: 15},
	}
	result := CheckHierarchy(p, nodes)
	if !result.Pass {
		t.Fatalf("expected pass: %v", result.Findings)
	}
}

func TestCheckHierarchyEmpty(t *testing.T) {
	p := RBOKProfile()
	result := CheckHierarchy(p, nil)
	if result.Pass {
		t.Fatal("empty nodes should fail (missing required)")
	}
}

func TestCheckHierarchyFlat(t *testing.T) {
	p := FlatProfile()
	nodes := []NodeDepthEntry{
		{LevelName: "document", Depth: 0, Line: 1},
	}
	result := CheckHierarchy(p, nodes)
	if !result.Pass {
		t.Fatalf("flat should pass: %v", result.Findings)
	}
}
