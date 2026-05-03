package fidelity

import (
	"testing"
)

func TestCheckTermGatesClean(t *testing.T) {
	result := CheckTermGates(TermGateInput{
		Definitions: []TermDefinition{
			{Term: "garantie", Definition: "Protection contractuelle.", Source: "glossary.yaml"},
			{Term: "franchise", Definition: "Part restant a charge.", Source: "glossary.yaml"},
		},
		Usages: []TermUsage{
			{Term: "garantie", Location: "article:1"},
			{Term: "franchise", Location: "article:2"},
		},
	})
	if !result.Pass {
		t.Fatalf("expected pass: %v", result.Findings)
	}
	if result.ConflictCount != 0 {
		t.Fatalf("conflicts: %d", result.ConflictCount)
	}
	if result.UndefinedCount != 0 {
		t.Fatalf("undefined: %d", result.UndefinedCount)
	}
}

func TestCheckTermGatesConflict(t *testing.T) {
	result := CheckTermGates(TermGateInput{
		Definitions: []TermDefinition{
			{Term: "garantie", Definition: "Protection contractuelle.", Source: "glossary-v1.yaml"},
			{Term: "garantie", Definition: "Engagement de couverture.", Source: "glossary-v2.yaml"},
		},
	})
	if result.Pass {
		t.Fatal("conflicting definitions should fail")
	}
	if result.ConflictCount != 1 {
		t.Fatalf("expected 1 conflict, got %d", result.ConflictCount)
	}
	assertTermFindingCode(t, result, CodeDefinitionConflict)
	f := findTermFinding(result, CodeDefinitionConflict, "garantie")
	if f == nil {
		t.Fatal("expected DEFINITION_CONFLICT for garantie")
	}
	if !f.Blocking {
		t.Fatal("conflict should be blocking")
	}
	if len(f.Sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(f.Sources))
	}
}

func TestCheckTermGatesDuplicateSameDefinition(t *testing.T) {
	result := CheckTermGates(TermGateInput{
		Definitions: []TermDefinition{
			{Term: "franchise", Definition: "Part restant a charge.", Source: "src-a.yaml"},
			{Term: "franchise", Definition: "Part restant a charge.", Source: "src-b.yaml"},
		},
	})
	if !result.Pass {
		t.Fatal("identical definitions should not block")
	}
	assertTermFindingCode(t, result, CodeDuplicateDefinition)
}

func TestCheckTermGatesUndefined(t *testing.T) {
	result := CheckTermGates(TermGateInput{
		Definitions: []TermDefinition{
			{Term: "garantie", Definition: "Protection.", Source: "glossary.yaml"},
		},
		Usages: []TermUsage{
			{Term: "garantie", Location: "article:1"},
			{Term: "sinistre", Location: "article:2", Line: 15},
		},
	})
	if result.Pass {
		t.Fatal("undefined term should fail")
	}
	if result.UndefinedCount != 1 {
		t.Fatalf("expected 1 undefined, got %d", result.UndefinedCount)
	}
	f := findTermFinding(result, CodeUndefinedTerm, "sinistre")
	if f == nil {
		t.Fatal("expected UNDEFINED_TERM for sinistre")
	}
	if !f.Blocking {
		t.Fatal("undefined should be blocking")
	}
}

func TestCheckTermGatesCaseInsensitive(t *testing.T) {
	result := CheckTermGates(TermGateInput{
		Definitions: []TermDefinition{
			{Term: "Garantie", Definition: "Protection.", Source: "glossary.yaml"},
		},
		Usages: []TermUsage{
			{Term: "garantie", Location: "article:1"},
			{Term: "GARANTIE", Location: "article:2"},
		},
	})
	if !result.Pass {
		t.Fatalf("case-insensitive match should pass: %v", result.Findings)
	}
}

func TestCheckTermGatesMultipleConflicts(t *testing.T) {
	result := CheckTermGates(TermGateInput{
		Definitions: []TermDefinition{
			{Term: "garantie", Definition: "Def A.", Source: "src-a"},
			{Term: "garantie", Definition: "Def B.", Source: "src-b"},
			{Term: "franchise", Definition: "Def X.", Source: "src-a"},
			{Term: "franchise", Definition: "Def Y.", Source: "src-c"},
		},
	})
	if result.ConflictCount != 2 {
		t.Fatalf("expected 2 conflicts, got %d", result.ConflictCount)
	}
}

func TestCheckTermGatesMultipleUndefined(t *testing.T) {
	result := CheckTermGates(TermGateInput{
		Usages: []TermUsage{
			{Term: "alpha", Location: "l1"},
			{Term: "beta", Location: "l2"},
			{Term: "gamma", Location: "l3"},
		},
	})
	if result.UndefinedCount != 3 {
		t.Fatalf("expected 3 undefined, got %d", result.UndefinedCount)
	}
}

func TestCheckTermGatesDeduplicateUndefined(t *testing.T) {
	result := CheckTermGates(TermGateInput{
		Usages: []TermUsage{
			{Term: "alpha", Location: "l1"},
			{Term: "alpha", Location: "l2"},
			{Term: "alpha", Location: "l3"},
		},
	})
	if result.UndefinedCount != 1 {
		t.Fatalf("should deduplicate: got %d", result.UndefinedCount)
	}
}

func TestCheckTermGatesEmpty(t *testing.T) {
	result := CheckTermGates(TermGateInput{})
	if !result.Pass {
		t.Fatal("empty should pass")
	}
	if result.DefinitionCount != 0 || result.UsageCount != 0 {
		t.Fatal("counts should be 0")
	}
}

func TestCheckTermGatesNoUsages(t *testing.T) {
	result := CheckTermGates(TermGateInput{
		Definitions: []TermDefinition{
			{Term: "garantie", Definition: "Def.", Source: "glossary.yaml"},
		},
	})
	if !result.Pass {
		t.Fatal("definitions without usages should pass")
	}
}

func TestCheckTermGatesConflictSources(t *testing.T) {
	result := CheckTermGates(TermGateInput{
		Definitions: []TermDefinition{
			{Term: "garantie", Definition: "Def A.", Source: "src-a"},
			{Term: "garantie", Definition: "Def A.", Source: "src-a"}, // same source, same def
			{Term: "garantie", Definition: "Def B.", Source: "src-b"},
		},
	})
	f := findTermFinding(result, CodeDefinitionConflict, "garantie")
	if f == nil {
		t.Fatal("expected conflict")
	}
	// Sources should be deduplicated.
	if len(f.Sources) != 2 {
		t.Fatalf("expected 2 unique sources, got %d: %v", len(f.Sources), f.Sources)
	}
}

func TestCheckTermGatesFindingsSorted(t *testing.T) {
	result := CheckTermGates(TermGateInput{
		Definitions: []TermDefinition{
			{Term: "zebra", Definition: "A.", Source: "s1"},
			{Term: "zebra", Definition: "B.", Source: "s2"},
		},
		Usages: []TermUsage{
			{Term: "alpha", Location: "l1"},
		},
	})
	if len(result.Findings) < 2 {
		t.Fatalf("expected >=2 findings, got %d", len(result.Findings))
	}
	// Should be sorted by code then term.
	for i := 1; i < len(result.Findings); i++ {
		a, b := result.Findings[i-1], result.Findings[i]
		if a.Code > b.Code || (a.Code == b.Code && a.Term > b.Term) {
			t.Fatalf("findings not sorted: %q/%q before %q/%q", a.Code, a.Term, b.Code, b.Term)
		}
	}
}

func TestCheckTermGatesThreeWayConflict(t *testing.T) {
	result := CheckTermGates(TermGateInput{
		Definitions: []TermDefinition{
			{Term: "sinistre", Definition: "Def 1.", Source: "a"},
			{Term: "sinistre", Definition: "Def 2.", Source: "b"},
			{Term: "sinistre", Definition: "Def 3.", Source: "c"},
		},
	})
	f := findTermFinding(result, CodeDefinitionConflict, "sinistre")
	if f == nil {
		t.Fatal("expected conflict")
	}
	if len(f.Sources) != 3 {
		t.Fatalf("expected 3 sources, got %d", len(f.Sources))
	}
}

func TestCheckTermGatesWhitespaceTrimming(t *testing.T) {
	result := CheckTermGates(TermGateInput{
		Definitions: []TermDefinition{
			{Term: "  garantie  ", Definition: "Def.", Source: "s1"},
		},
		Usages: []TermUsage{
			{Term: "garantie", Location: "l1"},
		},
	})
	if !result.Pass {
		t.Fatalf("whitespace-trimmed match should pass: %v", result.Findings)
	}
}

func TestTermGateResultCounts(t *testing.T) {
	result := CheckTermGates(TermGateInput{
		Definitions: []TermDefinition{
			{Term: "a", Definition: "d1", Source: "s1"},
			{Term: "b", Definition: "d2", Source: "s1"},
		},
		Usages: []TermUsage{
			{Term: "a", Location: "l1"},
			{Term: "c", Location: "l2"},
		},
	})
	if result.DefinitionCount != 2 {
		t.Fatalf("definition_count: %d", result.DefinitionCount)
	}
	if result.UsageCount != 2 {
		t.Fatalf("usage_count: %d", result.UsageCount)
	}
}

// --- helpers ---

func assertTermFindingCode(t *testing.T, result TermGateResult, code string) {
	t.Helper()
	for _, f := range result.Findings {
		if f.Code == code {
			return
		}
	}
	t.Fatalf("expected finding code %q in %v", code, result.Findings)
}

func findTermFinding(result TermGateResult, code, term string) *TermFinding {
	for _, f := range result.Findings {
		if f.Code == code && normalize(f.Term) == normalize(term) {
			return &f
		}
	}
	return nil
}
