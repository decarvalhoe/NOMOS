package atomization

import (
	"path/filepath"
	"testing"
)

func TestLoadGoldCorpusRBOK(t *testing.T) {
	corpus, err := LoadGoldCorpus(filepath.Join("testdata", "gold", "rbok-doctrine.json"))
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if corpus.Corpus != "rbok-doctrine" {
		t.Fatalf("expected rbok-doctrine, got %s", corpus.Corpus)
	}
	if len(corpus.Annotations) != 2 {
		t.Fatalf("expected 2 annotations, got %d", len(corpus.Annotations))
	}
}

func TestLoadGoldCorpusLegal(t *testing.T) {
	corpus, err := LoadGoldCorpus(filepath.Join("testdata", "gold", "legal-text.json"))
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if corpus.Corpus != "legal-text" {
		t.Fatalf("expected legal-text, got %s", corpus.Corpus)
	}
	if len(corpus.Annotations) != 3 {
		t.Fatalf("expected 3 annotations, got %d", len(corpus.Annotations))
	}
}

func TestLoadGoldCorpusKW(t *testing.T) {
	corpus, err := LoadGoldCorpus(filepath.Join("testdata", "gold", "kw-rules.json"))
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if corpus.Corpus != "kw-rules" {
		t.Fatalf("expected kw-rules, got %s", corpus.Corpus)
	}
	if len(corpus.Annotations) != 3 {
		t.Fatalf("expected 3 annotations, got %d", len(corpus.Annotations))
	}
}

func TestValidateGoldCorpusValid(t *testing.T) {
	corpus, _ := LoadGoldCorpus(filepath.Join("testdata", "gold", "rbok-doctrine.json"))
	errs := ValidateGoldCorpus(corpus)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestValidateGoldCorpusAllFixtures(t *testing.T) {
	fixtures := []string{"rbok-doctrine.json", "legal-text.json", "kw-rules.json"}
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			corpus, err := LoadGoldCorpus(filepath.Join("testdata", "gold", name))
			if err != nil {
				t.Fatalf("load error: %v", err)
			}
			errs := ValidateGoldCorpus(corpus)
			if len(errs) != 0 {
				t.Fatalf("validation errors: %v", errs)
			}
		})
	}
}

func TestValidateGoldCorpusInvalid(t *testing.T) {
	corpus := GoldCorpus{Corpus: "", Annotations: nil}
	errs := ValidateGoldCorpus(corpus)
	if len(errs) < 2 {
		t.Fatalf("expected at least 2 errors, got %d: %v", len(errs), errs)
	}
}

func TestRunRegressionAllMatched(t *testing.T) {
	corpus, _ := LoadGoldCorpus(filepath.Join("testdata", "gold", "legal-text.json"))

	produced := []ProducedAtom{
		{AtomID: "ATOM-CC-1382", Kind: "rule", Title: "Responsabilité du fait personnel"},
		{AtomID: "ATOM-CC-1383", Kind: "rule", Title: "Responsabilité par négligence"},
		{AtomID: "ATOM-CC-1384", Kind: "rule", Title: "Responsabilité du fait d'autrui"},
		{AtomID: "ATOM-CC-1384-ENUM", Kind: "enumeration", Title: "Liste des personnes responsables"},
	}
	refs := []ProducedRef{
		{FromAtom: "ATOM-CC-1382", RefType: "cites", TargetID: "DOMAT-LOIS-CIVILES"},
		{FromAtom: "ATOM-CC-1383", RefType: "depends_on", TargetID: "ATOM-CC-1382"},
	}

	result := RunRegression(corpus, produced, refs)

	if result.Score != 1.0 {
		t.Fatalf("expected score 1.0, got %f", result.Score)
	}
	if result.Missing != 0 {
		t.Fatalf("expected 0 missing, got %d", result.Missing)
	}
	if result.Matched != 3 {
		t.Fatalf("expected 3 matched, got %d", result.Matched)
	}
}

func TestRunRegressionPartialMatch(t *testing.T) {
	corpus, _ := LoadGoldCorpus(filepath.Join("testdata", "gold", "legal-text.json"))

	// Only produce 2 of 4 expected atoms.
	produced := []ProducedAtom{
		{AtomID: "ATOM-CC-1382", Kind: "rule", Title: "Art 1382"},
		{AtomID: "ATOM-CC-1383", Kind: "rule", Title: "Art 1383"},
	}
	refs := []ProducedRef{
		{FromAtom: "ATOM-CC-1382", RefType: "cites", TargetID: "DOMAT-LOIS-CIVILES"},
		{FromAtom: "ATOM-CC-1383", RefType: "depends_on", TargetID: "ATOM-CC-1382"},
	}

	result := RunRegression(corpus, produced, refs)

	// GOLD-LEGAL-001 matched, GOLD-LEGAL-002 matched, GOLD-LEGAL-003 partial (has 1384 missing)
	if result.Score < 0.5 {
		t.Fatalf("expected score >= 0.5, got %f", result.Score)
	}
}

func TestRunRegressionNoneMatched(t *testing.T) {
	corpus, _ := LoadGoldCorpus(filepath.Join("testdata", "gold", "kw-rules.json"))

	// Produce nothing.
	result := RunRegression(corpus, nil, nil)

	if result.Score != 0 {
		t.Fatalf("expected score 0, got %f", result.Score)
	}
	if result.Missing != 3 {
		t.Fatalf("expected 3 missing, got %d", result.Missing)
	}
}

func TestRunRegressionExtraAtoms(t *testing.T) {
	corpus := GoldCorpus{
		Corpus: "test",
		Annotations: []GoldAnnotation{
			{
				ID: "GOLD-1", SourceFile: "test.md",
				ExpectedAtoms: []ExpectedAtom{{AtomID: "ATOM-A", Kind: "rule", Title: "A"}},
			},
		},
	}

	produced := []ProducedAtom{
		{AtomID: "ATOM-A", Kind: "rule", Title: "A"},
		{AtomID: "ATOM-EXTRA", Kind: "rule", Title: "Extra"},
	}

	result := RunRegression(corpus, produced, nil)

	if result.Extra != 1 {
		t.Fatalf("expected 1 extra, got %d", result.Extra)
	}
	if result.Matched != 1 {
		t.Fatalf("expected 1 matched, got %d", result.Matched)
	}
}

func TestRunRegressionWrongKind(t *testing.T) {
	corpus := GoldCorpus{
		Corpus: "test",
		Annotations: []GoldAnnotation{
			{
				ID: "GOLD-1", SourceFile: "test.md",
				ExpectedAtoms: []ExpectedAtom{{AtomID: "ATOM-A", Kind: "rule", Title: "A"}},
			},
		},
	}

	// Produce with wrong kind.
	produced := []ProducedAtom{
		{AtomID: "ATOM-A", Kind: "exception", Title: "A"},
	}

	result := RunRegression(corpus, produced, nil)

	// Kind mismatch = not matched.
	if result.Missing != 1 {
		t.Fatalf("expected 1 missing (kind mismatch), got %d", result.Missing)
	}
}

func TestRunRegressionRefMatching(t *testing.T) {
	corpus := GoldCorpus{
		Corpus: "test",
		Annotations: []GoldAnnotation{
			{
				ID: "GOLD-1", SourceFile: "test.md",
				ExpectedAtoms: []ExpectedAtom{{AtomID: "ATOM-A", Kind: "rule", Title: "A"}},
				ExpectedRefs: []ExpectedRef{
					{FromAtom: "ATOM-A", RefType: "cites", TargetID: "EXT-1"},
					{FromAtom: "ATOM-A", RefType: "depends_on", TargetID: "ATOM-B"},
				},
			},
		},
	}

	produced := []ProducedAtom{{AtomID: "ATOM-A", Kind: "rule", Title: "A"}}
	refs := []ProducedRef{
		{FromAtom: "ATOM-A", RefType: "cites", TargetID: "EXT-1"},
	}

	result := RunRegression(corpus, produced, refs)

	// Partial: atom matched, 1 of 2 refs matched.
	detail := result.Details[0]
	if detail.Status != "partial" {
		t.Fatalf("expected partial (missing ref), got %s", detail.Status)
	}
	if len(detail.MissingRefs) != 1 {
		t.Fatalf("expected 1 missing ref, got %d", len(detail.MissingRefs))
	}
}

func TestSortDetails(t *testing.T) {
	details := []RegressionDetail{
		{AnnotationID: "C"},
		{AnnotationID: "A"},
		{AnnotationID: "B"},
	}
	SortDetails(details)
	if details[0].AnnotationID != "A" || details[2].AnnotationID != "C" {
		t.Fatalf("expected sorted, got %v", details)
	}
}

func TestParseGoldCorpusInvalidJSON(t *testing.T) {
	_, err := ParseGoldCorpus([]byte(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadGoldCorpusMissingFile(t *testing.T) {
	_, err := LoadGoldCorpus("/nonexistent/file.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
