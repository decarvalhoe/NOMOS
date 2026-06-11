package answer

import "sort"

// VRC-13 (#559, B2) — the regulated RAG evaluation harness. Eval runs the
// cite-or-abstain gate over a golden corpus and checks the AGGREGATE metrics
// against versioned thresholds, so a citation-recall (or faithfulness)
// regression below the recorded floor turns the CI job red. The per-answer
// gate (any blocking finding fails) still applies on top.

// EvalThresholds are the versioned floors a non-regressive corpus must hold.
type EvalThresholds struct {
	MinMeanCitationRecall    float64 `json:"min_mean_citation_recall"`
	MinMeanCitationPrecision float64 `json:"min_mean_citation_precision"`
	MinMeanFaithfulness      float64 `json:"min_mean_faithfulness"`
	MaxFailedAnswers         int     `json:"max_failed_answers"`
}

// EvalResult is the harness verdict over a corpus.
type EvalResult struct {
	Status               string         `json:"status"` // "pass" | "fail"
	Checked              int            `json:"checked"`
	FailedAnswers        int            `json:"failed_answers"`
	MeanCitationRecall   float64        `json:"mean_citation_recall"`
	MeanCitationPrecision float64       `json:"mean_citation_precision"`
	MeanFaithfulness     float64        `json:"mean_faithfulness"`
	Thresholds           EvalThresholds `json:"thresholds"`
	Regressions          []string       `json:"regressions"`
	Verdicts             []Verdict      `json:"verdicts"`
}

// Eval evaluates a corpus and fails closed on any per-answer finding or any
// aggregate metric below its versioned floor.
func Eval(answers []Answer, cfg Config, th EvalThresholds) EvalResult {
	res := EvalResult{Status: "pass", Thresholds: th}
	if len(answers) == 0 {
		res.Status = "fail"
		res.Regressions = []string{"empty eval corpus: nothing to evaluate"}
		return res
	}
	var sumR, sumP, sumF float64
	failed := 0
	for _, a := range answers {
		v := Evaluate(a, cfg)
		res.Verdicts = append(res.Verdicts, v)
		sumR += v.CitationRecall
		sumP += v.CitationPrecision
		sumF += v.Faithfulness
		if len(v.Findings) > 0 {
			failed++
		}
	}
	n := float64(len(answers))
	res.Checked = len(answers)
	res.FailedAnswers = failed
	res.MeanCitationRecall = round4(sumR / n)
	res.MeanCitationPrecision = round4(sumP / n)
	res.MeanFaithfulness = round4(sumF / n)

	if failed > th.MaxFailedAnswers {
		res.Regressions = append(res.Regressions,
			"failed_answers above the versioned maximum")
	}
	if res.MeanCitationRecall < th.MinMeanCitationRecall {
		res.Regressions = append(res.Regressions, "mean_citation_recall below the versioned floor")
	}
	if res.MeanCitationPrecision < th.MinMeanCitationPrecision {
		res.Regressions = append(res.Regressions, "mean_citation_precision below the versioned floor")
	}
	if res.MeanFaithfulness < th.MinMeanFaithfulness {
		res.Regressions = append(res.Regressions, "mean_faithfulness below the versioned floor")
	}
	sort.Strings(res.Regressions)
	if len(res.Regressions) > 0 {
		res.Status = "fail"
	}
	sort.SliceStable(res.Verdicts, func(i, j int) bool { return res.Verdicts[i].AnswerID < res.Verdicts[j].AnswerID })
	return res
}
