package fidelity

import (
	"testing"
)

func TestRunAQ4ProofAllPass(t *testing.T) {
	input := AQ4Input{
		Atoms: []AQ4Atom{
			{ID: "A1", SemanticRole: "rule", Text: "The insured must declare risks.", HasRef: true},
			{ID: "A2", SemanticRole: "clause", Text: "Premium payment is mandatory.", HasRef: true},
			{ID: "A3", SemanticRole: "definition", Text: "Risk means potential loss.", HasRef: true},
		},
		CrossRefs: []AQ4CrossRef{
			{SourceID: "A1", TargetID: "A3", Resolved: true},
			{SourceID: "A2", TargetID: "A1", Resolved: true},
		},
		Lexicon: []LexiconEntry{
			{Term: "insured", Definition: "Person covered by policy", Governed: true},
			{Term: "risk", Definition: "Potential loss event", Governed: true, Aliases: []string{"risque"}},
			{Term: "premium", Definition: "Payment for coverage", Governed: true},
		},
	}

	report := RunAQ4Proof(input)
	if !report.Pass {
		t.Fatalf("expected pass, got: %s", report.Summary)
	}
	if report.Score < 0.8 {
		t.Fatalf("expected score >= 0.8, got %.2f", report.Score)
	}
	if report.TotalAtoms != 3 {
		t.Fatalf("expected 3 atoms, got %d", report.TotalAtoms)
	}
}

func TestRunAQ4ProofSemanticRolesFail(t *testing.T) {
	input := AQ4Input{
		Atoms: []AQ4Atom{
			{ID: "A1", SemanticRole: "", Text: "No role assigned."},
			{ID: "A2", SemanticRole: "unknown", Text: "Unknown role."},
			{ID: "A3", SemanticRole: "", Text: "Also no role."},
			{ID: "A4", SemanticRole: "rule", Text: "Only this one has a role."},
		},
		Lexicon: []LexiconEntry{{Term: "role", Governed: true}},
	}

	report := RunAQ4Proof(input)
	if report.Pass {
		t.Fatal("expected fail — only 25% roles assigned")
	}

	var roleCheck AQ4CheckResult
	for _, c := range report.Criteria {
		if c.Criterion == AQ4SemanticRoles {
			roleCheck = c
		}
	}
	if roleCheck.Pass {
		t.Fatal("semantic_roles criterion should fail")
	}
	if roleCheck.Score >= 0.8 {
		t.Fatalf("expected score < 0.8, got %.2f", roleCheck.Score)
	}
}

func TestRunAQ4ProofCrossRefsFail(t *testing.T) {
	input := AQ4Input{
		Atoms: []AQ4Atom{
			{ID: "A1", SemanticRole: "rule", Text: "Content."},
		},
		CrossRefs: []AQ4CrossRef{
			{SourceID: "A1", TargetID: "A2", Resolved: false},
			{SourceID: "A1", TargetID: "A3", Resolved: false},
			{SourceID: "A1", TargetID: "A4", Resolved: true},
		},
		Lexicon: []LexiconEntry{{Term: "content", Governed: true}},
	}

	report := RunAQ4Proof(input)

	var refsCheck AQ4CheckResult
	for _, c := range report.Criteria {
		if c.Criterion == AQ4CrossRefs {
			refsCheck = c
		}
	}
	if refsCheck.Pass {
		t.Fatal("cross_refs criterion should fail — only 33% resolved")
	}
	if len(refsCheck.Details) == 0 {
		t.Fatal("expected broken ref details")
	}
}

func TestRunAQ4ProofCrossRefsEmpty(t *testing.T) {
	input := AQ4Input{
		Atoms:   []AQ4Atom{{ID: "A1", SemanticRole: "rule", Text: "Solo atom."}},
		Lexicon: []LexiconEntry{{Term: "solo", Governed: true}},
	}

	report := RunAQ4Proof(input)

	var refsCheck AQ4CheckResult
	for _, c := range report.Criteria {
		if c.Criterion == AQ4CrossRefs {
			refsCheck = c
		}
	}
	if !refsCheck.Pass {
		t.Fatal("no cross-refs should pass")
	}
	if refsCheck.Score != 1.0 {
		t.Fatalf("expected score 1.0 for empty refs, got %.2f", refsCheck.Score)
	}
}

func TestRunAQ4ProofLexiconFail(t *testing.T) {
	input := AQ4Input{
		Atoms: []AQ4Atom{
			{ID: "A1", SemanticRole: "rule", Text: "Some content here."},
		},
		Lexicon: []LexiconEntry{
			{Term: "insurance", Governed: false},
			{Term: "risk", Governed: false},
			{Term: "premium", Governed: true},
		},
	}

	report := RunAQ4Proof(input)

	var lexCheck AQ4CheckResult
	for _, c := range report.Criteria {
		if c.Criterion == AQ4Lexicon {
			lexCheck = c
		}
	}
	if lexCheck.Pass {
		t.Fatal("lexicon should fail — only 33% governed")
	}
}

func TestRunAQ4ProofNoLexicon(t *testing.T) {
	input := AQ4Input{
		Atoms: []AQ4Atom{{ID: "A1", SemanticRole: "rule", Text: "Content."}},
	}

	report := RunAQ4Proof(input)

	var lexCheck AQ4CheckResult
	for _, c := range report.Criteria {
		if c.Criterion == AQ4Lexicon {
			lexCheck = c
		}
	}
	if lexCheck.Pass {
		t.Fatal("no lexicon defined should fail")
	}
}

func TestRunAQ4ProofNoAtoms(t *testing.T) {
	input := AQ4Input{
		Lexicon: []LexiconEntry{{Term: "x", Governed: true}},
	}

	report := RunAQ4Proof(input)
	if report.Pass {
		t.Fatal("expected fail with no atoms")
	}
}

func TestRunAQ4ProofLexiconUsageBoost(t *testing.T) {
	input := AQ4Input{
		Atoms: []AQ4Atom{
			{ID: "A1", SemanticRole: "rule", Text: "The insured must pay the premium."},
			{ID: "A2", SemanticRole: "clause", Text: "Risk assessment is required."},
		},
		CrossRefs: []AQ4CrossRef{
			{SourceID: "A1", TargetID: "A2", Resolved: true},
		},
		Lexicon: []LexiconEntry{
			{Term: "insured", Definition: "covered person", Governed: true},
			{Term: "premium", Definition: "payment", Governed: true},
			{Term: "risk", Definition: "potential loss", Governed: true},
		},
	}

	report := RunAQ4Proof(input)

	var lexCheck AQ4CheckResult
	for _, c := range report.Criteria {
		if c.Criterion == AQ4Lexicon {
			lexCheck = c
		}
	}
	// All terms governed + atoms use them → high score.
	if lexCheck.Score < 0.8 {
		t.Fatalf("expected high lexicon score with usage, got %.2f", lexCheck.Score)
	}
}

func TestRunAQ4ProofSummaryContent(t *testing.T) {
	input := AQ4Input{
		Atoms:   []AQ4Atom{{ID: "A1", SemanticRole: "rule", Text: "Term usage."}},
		Lexicon: []LexiconEntry{{Term: "term", Governed: true}},
	}
	report := RunAQ4Proof(input)

	if report.Summary == "" {
		t.Fatal("expected non-empty summary")
	}
}

func TestRunAQ4ProofCriteriaCount(t *testing.T) {
	input := AQ4Input{
		Atoms:   []AQ4Atom{{ID: "A1", SemanticRole: "rule", Text: "x"}},
		Lexicon: []LexiconEntry{{Term: "x", Governed: true}},
	}
	report := RunAQ4Proof(input)

	if len(report.Criteria) != 3 {
		t.Fatalf("expected 3 criteria, got %d", len(report.Criteria))
	}
}
