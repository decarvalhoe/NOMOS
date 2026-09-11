package answer

import (
	"encoding/json"
	"testing"
)

// VRC-46 (#582) — the public cite-or-abstain bench measures the gate against
// labelled ground truth instead of declaring it. The proofs here are
// adversarial (doctrine §2.3): the bench must be honest about what the gate
// MISSES (a false cite is published, not averaged away), must refuse to score
// an unlabelled corpus, must measure over-abstention as a defect, and must
// stay byte-identical when the corpus order changes (reproducibility is the
// whole point of publishing it).

func groundedBenchItem(id string, category string, expected string) BenchItem {
	a := groundedAnswer()
	a.AnswerID = id
	return BenchItem{Answer: a, Category: category, ExpectedDecision: expected}
}

func negatedBenchItem(id string, category string, expected string) BenchItem {
	a := negatedAnswer()
	a.AnswerID = id
	return BenchItem{Answer: a, Category: category, ExpectedDecision: expected}
}

func forgedBenchItem(id string, category string, expected string) BenchItem {
	a := groundedAnswer()
	a.AnswerID = id
	a.SourceSpans[0].ChunkID = ""
	a.SourceSpans[0].SourceHash = "sha256:TAMPERED"
	a.RetrievedChunks = []Chunk{{SourceID: "S1", SourceHash: "sha256:abc", Span: "L1-L2",
		Text: "Le gabarit retient une hauteur de neuf metres au faite."}}
	return BenchItem{Answer: a, Category: category, ExpectedDecision: expected}
}

func ptrf(v float64) *float64 { return &v }

func TestBench_MeasuresAgreementAndBothErrorDirections(t *testing.T) {
	items := []BenchItem{
		groundedBenchItem("B-GOOD", "grounded", ExpectCite),
		forgedBenchItem("B-FORGED", "forged_citation", ExpectAbstain),
		negatedBenchItem("B-NEGATED", "negation", ExpectAbstain),
	}
	res := Bench(items, Defaults(), BenchThresholds{})
	if res.Status != "measured" {
		t.Fatalf("a labelled corpus is a measurement, not a failure: %+v", res.Violations)
	}
	if res.Items != 3 || res.MustCiteTotal != 1 || res.MustAbstainTotal != 2 {
		t.Fatalf("counts are wrong: %+v", res)
	}
	// The forged citation is blocked; the negation is the KNOWN blind spot of
	// the lexical proxy — the bench publishes it instead of hiding it.
	if res.FalseCites != 1 || res.FalseCiteRate != 0.5 {
		t.Fatalf("the negation must appear as a false cite, got %+v", res)
	}
	if res.MustAbstainRecall != 0.5 {
		t.Fatalf("must_abstain_recall must be measured, got %v", res.MustAbstainRecall)
	}
	if res.MustCiteRecall != 1 {
		t.Fatalf("the grounded item must be cited, got %v", res.MustCiteRecall)
	}
	if got := round4(2.0 / 3.0); res.Agreement != got {
		t.Fatalf("agreement must be 2/3, got %v", res.Agreement)
	}
	cats := map[string]BenchCategory{}
	for _, c := range res.Categories {
		cats[c.Category] = c
	}
	if cats["negation"].FalseCites != 1 || cats["negation"].Items != 1 {
		t.Fatalf("the negation category must carry its own false cite: %+v", cats["negation"])
	}
	if cats["forged_citation"].BlockedAsExpected != 1 {
		t.Fatalf("the forged citation must be blocked: %+v", cats["forged_citation"])
	}
}

func TestBench_TheSecondJudgeMovesTheMeasurement(t *testing.T) {
	// The published bench is run twice (lexical, then with a second judge):
	// the negation item is expected to flip. If it does not, the upgrade is
	// not upgrade — and this test says so.
	items := []BenchItem{negatedBenchItem("B-NEGATED", "negation", ExpectAbstain)}
	lexical := Bench(items, Defaults(), BenchThresholds{})
	if lexical.FalseCites != 1 {
		t.Fatalf("the lexical proxy is negation-blind by construction, got %+v", lexical)
	}
	scored := Bench(items, withScorer(ScorerFunc(fakeNLI)), BenchThresholds{})
	if scored.FalseCites != 0 || scored.MustAbstainRecall != 1 {
		t.Fatalf("the second judge must block the negation, got %+v", scored)
	}
	if !scored.Gates.ScorerConfigured {
		t.Fatalf("a measurement taken with a second judge must say so: %+v", scored.Gates)
	}
}

func TestBench_OverAbstentionIsMeasuredAsADefect(t *testing.T) {
	// A gate that blocks everything scores 100% on the safety side. The bench
	// must also measure what that costs: every legitimate answer blocked.
	items := []BenchItem{
		groundedBenchItem("B-GOOD-1", "grounded", ExpectCite),
		groundedBenchItem("B-GOOD-2", "grounded", ExpectCite),
	}
	res := Bench(items, Defaults(), BenchThresholds{})
	if res.MissedCites != 0 || res.MustCiteRecall != 1 {
		t.Fatalf("grounded answers must not be missed: %+v", res)
	}

	unreachable := Defaults()
	unreachable.ALCEGate = 2.0 // unreachable floor: even a perfect citation fails
	blocked := Bench(items, unreachable, BenchThresholds{})
	if blocked.MustCiteRecall != 0 || blocked.MissedCites != 2 {
		t.Fatalf("over-abstention must be visible, got %+v", blocked)
	}
	if blocked.MustAbstainTotal != 0 {
		t.Fatalf("no item expects abstain here: %+v", blocked)
	}
}

func TestBench_UnlabelledItemIsADefectNotAMeasurement(t *testing.T) {
	items := []BenchItem{
		groundedBenchItem("B-GOOD", "grounded", ExpectCite),
		{Answer: groundedAnswer(), Category: "forgotten"},
		groundedBenchItem("B-WRONG-LABEL", "grounded", "maybe"),
	}
	res := Bench(items, Defaults(), BenchThresholds{})
	if res.Status != "fail" {
		t.Fatalf("an unlabelled corpus must fail, got %+v", res)
	}
	if len(res.Defects) != 2 {
		t.Fatalf("both label defects must be named: %+v", res.Defects)
	}
	// The defective items are excluded from the measurement, never averaged in.
	if res.Items != 1 || res.Agreement != 1 {
		t.Fatalf("only labelled items may be measured, got %+v", res)
	}
}

func TestBench_EmptyCorpusFailsClosed(t *testing.T) {
	res := Bench(nil, Defaults(), BenchThresholds{})
	if res.Status != "fail" || len(res.Defects) != 1 {
		t.Fatalf("an empty bench measures nothing: %+v", res)
	}
}

func TestBench_ABoundNobodyMeasuredFailsClosed(t *testing.T) {
	// #620's rule applied to the bench: a floor over an empty side of the
	// confusion matrix would satisfy every bound.
	onlyCite := []BenchItem{groundedBenchItem("B-GOOD", "grounded", ExpectCite)}
	res := Bench(onlyCite, Defaults(), BenchThresholds{MaxFalseCiteRate: ptrf(0.0), MinMustAbstainRecall: ptrf(1.0)})
	if res.Status != "fail" || len(res.Violations) != 2 {
		t.Fatalf("a bound over nothing measured must fail: %+v", res)
	}

	// With both sides present, the bound bites.
	both := append(onlyCite, forgedBenchItem("B-FORGED", "forged_citation", ExpectAbstain))
	ok := Bench(both, Defaults(), BenchThresholds{MaxFalseCiteRate: ptrf(0.0), MinMustAbstainRecall: ptrf(1.0)})
	if ok.Status != "measured" || len(ok.Violations) != 0 {
		t.Fatalf("a held bound must not fail: %+v", ok)
	}
	// A missed block turns the bench red.
	negation := append(onlyCite, negatedBenchItem("B-NEGATED", "negation", ExpectAbstain))
	red := Bench(negation, Defaults(), BenchThresholds{MaxFalseCiteRate: ptrf(0.0)})
	if red.Status != "fail" {
		t.Fatalf("a false cite above the ceiling must fail: %+v", red)
	}
	if len(red.Violations) != 1 || red.Violations[0] != "false_cite_rate above the versioned ceiling" {
		t.Fatalf("the violation must be named: %+v", red.Violations)
	}
}

func TestBench_IsDeterministicUnderItemOrder(t *testing.T) {
	items := []BenchItem{
		groundedBenchItem("B-GOOD", "grounded", ExpectCite),
		forgedBenchItem("B-FORGED", "forged_citation", ExpectAbstain),
		negatedBenchItem("B-NEGATED", "negation", ExpectAbstain),
	}
	shuffled := []BenchItem{items[2], items[0], items[1]}
	a, err := json.Marshal(Bench(items, Defaults(), BenchThresholds{}))
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(Bench(shuffled, Defaults(), BenchThresholds{}))
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Fatalf("the bench must not depend on corpus order:\n%s\n%s", a, b)
	}
}
