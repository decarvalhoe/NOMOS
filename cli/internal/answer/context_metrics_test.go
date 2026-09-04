package answer

import (
	"strings"
	"testing"
)

// #620 — retrieval-side metrics. The central proof: a sentence supported only
// by a distractor chunk passes lexical faithfulness (its tokens ARE in the
// support corpus) and is caught by noise_sensitivity alone.

func f64(v float64) *float64 { return &v }

func expectedAnswer() Answer {
	a := groundedAnswer()
	a.ExpectedChunkIDs = []string{"c1"}
	return a
}

func contextThresholds() EvalThresholds {
	th := evalThresholds()
	th.MinMeanContextRecall = f64(1.0)
	th.MinMeanContextPrecision = f64(1.0)
	th.MaxMeanNoiseSensitivity = f64(0.0)
	return th
}

// noisyAnswer retrieves AND cites a distractor (so citation recall/precision
// stay 1.0), and its second sentence is supported by the distractor only.
func noisyAnswer() Answer {
	a := expectedAnswer()
	a.Answer = "Le gabarit retient une hauteur de neuf metres au faite. La mise a l'enquete publique dure trente jours."
	a.RetrievedChunks = append(a.RetrievedChunks, Chunk{ChunkID: "d1",
		Text: "La mise a l'enquete publique dure trente jours pour le dossier d'autorisation."})
	a.SourceSpans = append(a.SourceSpans, Span{SourceID: "S2", SourceHash: "sha256:def", Span: "L9-L10", ChunkID: "d1",
		Text: "La mise a l'enquete publique dure trente jours pour le dossier d'autorisation."})
	return a
}

func hasRegression(res EvalResult, needle string) bool {
	for _, r := range res.Regressions {
		if strings.Contains(r, needle) {
			return true
		}
	}
	return false
}

func TestContext_GoldenAnswerIsPerfect(t *testing.T) {
	v := Evaluate(expectedAnswer(), Defaults())
	if v.Context == nil {
		t.Fatal("an answer with expected_chunk_ids must carry context metrics")
	}
	c := v.Context
	if c.ContextRecall != 1 || c.ContextPrecision != 1 || c.NoiseSensitivity != 0 || c.DistractorChunks != 0 {
		t.Fatalf("golden answer must score perfectly: %+v", c)
	}
	if c.Method != methodContext || c.Limitation == "" {
		t.Fatalf("context metrics must name their method and limitation: %+v", c)
	}
}

func TestContext_AbsentWithoutExpectations(t *testing.T) {
	if v := Evaluate(groundedAnswer(), Defaults()); v.Context != nil {
		t.Fatalf("gate answers declare no expectations and get no context block, got %+v", v.Context)
	}
}

func TestEval_NoiseSensitivityCatchesWhatFaithfulnessCannot(t *testing.T) {
	v := Evaluate(noisyAnswer(), Defaults())
	if v.Decision != "cite" || v.Faithfulness < Defaults().FaithfulnessGate || len(v.Findings) != 0 {
		t.Fatalf("the lexical proxy must NOT see the contamination (that is the point): %+v", v)
	}
	if v.Context == nil || v.Context.NoiseSensitivity != 0.5 || v.Context.NoiseInducedSentences != 1 || v.Context.DistractorChunks != 1 {
		t.Fatalf("one of two sentences is supported by the distractor only: %+v", v.Context)
	}

	gated := Eval([]Answer{noisyAnswer()}, Defaults(), contextThresholds())
	if gated.Status != "fail" || !hasRegression(gated, "mean_noise_sensitivity above the versioned ceiling") {
		t.Fatalf("noise above the ceiling must turn the harness red, got %+v", gated.Regressions)
	}
	reported := Eval([]Answer{noisyAnswer()}, Defaults(), evalThresholds())
	if reported.Status != "pass" || reported.MeanNoiseSensitivity != 0.5 || reported.ContextEvaluated != 1 {
		t.Fatalf("without a ceiling the metric is reported, not gated: %+v", reported)
	}
}

func TestEval_ContextRecallRegressionTurnsRed(t *testing.T) {
	a := expectedAnswer()
	a.ExpectedChunkIDs = []string{"c1", "c2"} // c2 was never retrieved
	res := Eval([]Answer{a}, Defaults(), contextThresholds())
	if res.Status != "fail" || !hasRegression(res, "mean_context_recall below the versioned floor") {
		t.Fatalf("a missed expected chunk must turn the harness red, got %+v", res.Regressions)
	}
	if res.MeanContextRecall != 0.5 {
		t.Fatalf("expected recall 0.5, got %v", res.MeanContextRecall)
	}
	if res.FailedAnswers != 0 {
		t.Fatal("the per-answer gate is untouched by context metrics: the answer itself still cites")
	}
}

func TestContext_DistractorRankedFirstLowersPrecision(t *testing.T) {
	a := expectedAnswer()
	a.RetrievedChunks = []Chunk{
		{ChunkID: "d1", Text: "Un chunk sans rapport avec la question."},
		{ChunkID: "c1", Text: "Le gabarit retient une hauteur de neuf metres au faite."},
	}
	v := Evaluate(a, Defaults())
	if v.Context.ContextPrecision != 0.5 || v.Context.ContextRecall != 1 {
		t.Fatalf("a distractor ranked above the relevant chunk halves precision@2: %+v", v.Context)
	}
	a.RetrievedChunks[0], a.RetrievedChunks[1] = a.RetrievedChunks[1], a.RetrievedChunks[0]
	if v := Evaluate(a, Defaults()); v.Context.ContextPrecision != 1 {
		t.Fatalf("a distractor ranked below the relevant chunk costs nothing: %+v", v.Context)
	}
}

func TestEval_ContextThresholdWithoutExpectationsFailsClosed(t *testing.T) {
	res := Eval([]Answer{groundedAnswer()}, Defaults(), contextThresholds())
	if res.Status != "fail" || !hasRegression(res, "expected_chunk_ids") {
		t.Fatalf("a floor nobody measures against must fail closed, got %+v", res.Regressions)
	}
	if res := Eval([]Answer{groundedAnswer()}, Defaults(), evalThresholds()); res.Status != "pass" || res.ContextEvaluated != 0 {
		t.Fatalf("without context thresholds an expectation-less corpus still passes: %+v", res)
	}
}

func TestContext_RefusalAssertsNothing(t *testing.T) {
	a := Answer{
		AnswerID: "R1", PromptID: "P9", Answer: "", CitationStatus: "none", RefusalStatus: "unsupported",
		PolicyOutcome: "acceptable_refusal", ExpectedChunkIDs: []string{"c1"},
		RetrievedChunks: []Chunk{{ChunkID: "d1", Text: "Un distracteur retrouve pour rien."}},
	}
	v := Evaluate(a, Defaults())
	if v.Context == nil || v.Context.NoiseSensitivity != 0 || v.Context.TotalSentences != 0 || v.Context.ContextRecall != 0 {
		t.Fatalf("a refusal asserts nothing (no noise) but the miss is still measured (recall 0): %+v", v.Context)
	}
}
