package profiles

import (
	"testing"
)

// --- Registry ---

func TestKnownProfilesRegistered(t *testing.T) {
	ids := KnownIDs()
	expected := []string{"game-rules", "law-regulation", "rbok-lawbook"}
	if len(ids) != len(expected) {
		t.Fatalf("expected %d profiles, got %d: %v", len(expected), len(ids), ids)
	}
	for i, id := range expected {
		if ids[i] != id {
			t.Fatalf("expected %q at index %d, got %q", id, i, ids[i])
		}
	}
}

func TestLookupValid(t *testing.T) {
	for _, id := range []string{"law-regulation", "game-rules", "rbok-lawbook"} {
		p, err := Lookup(id)
		if err != nil {
			t.Fatalf("lookup %s: %v", id, err)
		}
		if p.ID != id {
			t.Fatalf("expected ID %q, got %q", id, p.ID)
		}
	}
}

func TestLookupCaseInsensitive(t *testing.T) {
	p, err := Lookup("LAW-REGULATION")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "law-regulation" {
		t.Fatalf("expected law-regulation, got %q", p.ID)
	}
}

func TestLookupUnknown(t *testing.T) {
	_, err := Lookup("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- Law & Regulation ---

func TestLawRegulationProfile(t *testing.T) {
	p := LawRegulation()
	if p.ID != "law-regulation" {
		t.Fatalf("expected law-regulation, got %q", p.ID)
	}
	if p.Domain != "legal" {
		t.Fatalf("expected legal domain, got %q", p.Domain)
	}
	if !p.HasNodeType("article") {
		t.Fatal("expected article node type")
	}
	if !p.HasNodeType("code") {
		t.Fatal("expected code node type")
	}
	if !p.HasNodeType("chapter") {
		t.Fatal("expected chapter node type")
	}
	if p.HasNodeType("spell") {
		t.Fatal("unexpected spell node type")
	}
}

func TestLawRegulationHierarchy(t *testing.T) {
	p := LawRegulation()
	if len(p.Hierarchy) == 0 {
		t.Fatal("expected hierarchy levels")
	}
	if p.Hierarchy[0].NodeType != "code" {
		t.Fatalf("expected code at depth 0, got %q", p.Hierarchy[0].NodeType)
	}
	if p.MaxDepth() < 5 {
		t.Fatalf("expected max depth >= 5, got %d", p.MaxDepth())
	}
}

func TestLawRegulationRequiredMetadata(t *testing.T) {
	p := LawRegulation()
	required := p.RequiredMetadata()
	names := map[string]bool{}
	for _, m := range required {
		names[m.Name] = true
	}
	for _, expected := range []string{"jurisdiction", "version", "owner", "status", "domain"} {
		if !names[expected] {
			t.Fatalf("expected required field %q", expected)
		}
	}
}

func TestLawRegulationStructuralNodes(t *testing.T) {
	p := LawRegulation()
	structural := 0
	content := 0
	for _, nt := range p.NodeTypes {
		if nt.Structural {
			structural++
			if len(nt.CanContain) == 0 {
				t.Fatalf("structural node %q should have CanContain", nt.Name)
			}
		} else {
			content++
		}
	}
	if structural == 0 || content == 0 {
		t.Fatal("expected both structural and content nodes")
	}
}

// --- Game Rules ---

func TestGameRulesProfile(t *testing.T) {
	p := GameRules()
	if p.ID != "game-rules" {
		t.Fatalf("expected game-rules, got %q", p.ID)
	}
	if p.Domain != "game-design" {
		t.Fatalf("expected game-design domain, got %q", p.Domain)
	}
	for _, expected := range []string{"rule", "ability", "stat_block", "class_feature", "exception", "sidebar"} {
		if !p.HasNodeType(expected) {
			t.Fatalf("expected node type %q", expected)
		}
	}
}

func TestGameRulesHierarchy(t *testing.T) {
	p := GameRules()
	if p.Hierarchy[0].NodeType != "rulebook" {
		t.Fatalf("expected rulebook at depth 0, got %q", p.Hierarchy[0].NodeType)
	}
	if p.MaxDepth() < 3 {
		t.Fatalf("expected max depth >= 3, got %d", p.MaxDepth())
	}
}

func TestGameRulesRequiredMetadata(t *testing.T) {
	p := GameRules()
	required := p.RequiredMetadata()
	names := map[string]bool{}
	for _, m := range required {
		names[m.Name] = true
	}
	for _, expected := range []string{"game_system", "edition", "version", "author", "status"} {
		if !names[expected] {
			t.Fatalf("expected required field %q", expected)
		}
	}
}

func TestGameRulesGameSpecificNodes(t *testing.T) {
	p := GameRules()
	gameNodes := []string{"ability", "class_feature", "stat_block", "sidebar", "example", "glossary_entry"}
	for _, name := range gameNodes {
		if !p.HasNodeType(name) {
			t.Fatalf("expected game-specific node %q", name)
		}
	}
}

// --- RBOK Lawbook ---

func TestRBOKLawbookProfile(t *testing.T) {
	p := RBOKLawbook()
	if p.ID != "rbok-lawbook" {
		t.Fatalf("expected rbok-lawbook, got %q", p.ID)
	}
	if p.Domain != "insurance" {
		t.Fatalf("expected insurance domain, got %q", p.Domain)
	}
	for _, expected := range []string{"article", "clause", "garantie", "exclusion", "franchise", "bareme", "parcours_etape"} {
		if !p.HasNodeType(expected) {
			t.Fatalf("expected RBOK node type %q", expected)
		}
	}
}

func TestRBOKLawbookConfidentialityRequired(t *testing.T) {
	p := RBOKLawbook()
	required := p.RequiredMetadata()
	found := false
	for _, m := range required {
		if m.Name == "confidentiality" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected confidentiality as required for RBOK")
	}
}

// --- Profile methods ---

func TestNodeTypeNames(t *testing.T) {
	p := LawRegulation()
	names := p.NodeTypeNames()
	if len(names) != len(p.NodeTypes) {
		t.Fatalf("expected %d names, got %d", len(p.NodeTypes), len(names))
	}
	// Should be sorted
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Fatalf("names not sorted: %q before %q", names[i-1], names[i])
		}
	}
}

// --- Cross-profile checks ---

func TestProfilesHaveUniqueIDs(t *testing.T) {
	ids := KnownIDs()
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Fatalf("duplicate profile ID %q", id)
		}
		seen[id] = true
	}
}

func TestAllProfilesHaveNodeTypes(t *testing.T) {
	for _, id := range KnownIDs() {
		p, _ := Lookup(id)
		if len(p.NodeTypes) == 0 {
			t.Fatalf("profile %q has no node types", id)
		}
	}
}

func TestAllProfilesHaveHierarchy(t *testing.T) {
	for _, id := range KnownIDs() {
		p, _ := Lookup(id)
		if len(p.Hierarchy) == 0 {
			t.Fatalf("profile %q has no hierarchy", id)
		}
	}
}

func TestAllProfilesHaveRequiredMetadata(t *testing.T) {
	for _, id := range KnownIDs() {
		p, _ := Lookup(id)
		required := p.RequiredMetadata()
		if len(required) == 0 {
			t.Fatalf("profile %q has no required metadata", id)
		}
		// All profiles should require version and status.
		names := map[string]bool{}
		for _, m := range required {
			names[m.Name] = true
		}
		if !names["version"] {
			t.Fatalf("profile %q missing required version", id)
		}
		if !names["status"] {
			t.Fatalf("profile %q missing required status", id)
		}
	}
}
