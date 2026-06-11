package answer

import "testing"

// VRC-13 (#559, B2) — the eval harness is non-regressive: a golden corpus
// holds above the versioned floor, and a citation-recall regression on any
// answer turns it red.

func evalThresholds() EvalThresholds {
	return EvalThresholds{
		MinMeanCitationRecall:    0.95,
		MinMeanCitationPrecision: 0.95,
		MinMeanFaithfulness:      0.95,
		MaxFailedAnswers:         0,
	}
}

func TestEval_GoldenCorpusHoldsAboveFloor(t *testing.T) {
	res := Eval([]Answer{groundedAnswer()}, Defaults(), evalThresholds())
	if res.Status != "pass" {
		t.Fatalf("golden corpus must hold, got regressions %+v", res.Regressions)
	}
	if res.MeanCitationRecall < 0.95 || res.MeanFaithfulness < 0.95 {
		t.Fatalf("golden means below floor: %+v", res)
	}
}

func TestEval_CitationRecallRegressionTurnsRed(t *testing.T) {
	// Drop one answer's citation off its retrieved chunk → recall collapses →
	// the per-answer gate fails AND the mean recall falls below the floor.
	a := groundedAnswer()
	a.SourceSpans[0].ChunkID = ""
	a.SourceSpans[0].SourceHash = "sha256:REGRESSED"
	a.RetrievedChunks = []Chunk{{SourceID: "S1", SourceHash: "sha256:abc", Span: "L1-L2",
		Text: "Le gabarit retient une hauteur de neuf metres au faite."}}
	res := Eval([]Answer{a}, Defaults(), evalThresholds())
	if res.Status != "fail" {
		t.Fatalf("a citation-recall regression must turn the harness red, got %+v", res)
	}
	if res.FailedAnswers != 1 {
		t.Fatalf("expected the regressed answer counted as failed, got %d", res.FailedAnswers)
	}
}

func TestEval_EmptyCorpusFailsClosed(t *testing.T) {
	if res := Eval(nil, Defaults(), evalThresholds()); res.Status != "fail" {
		t.Fatal("an empty eval corpus must fail closed, never vacuously pass")
	}
}
