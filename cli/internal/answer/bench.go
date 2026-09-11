package answer

import "sort"

// VRC-46 (#582) — the public cite-or-abstain bench, in the engine.
//
// The bench MEASURES the gate; it does not replace it and it does not
// adjudicate answers itself. Every item is an answer record ALREADY produced,
// carrying its own retrieved spans and citations, and labelled with the
// decision the gate is expected to reach (`cite` when the answer is fully
// grounded and citable, `abstain` when it must not be published as-is).
//
// There is no model, no embedding and no network in the loop: the run is
// byte-deterministic (items sorted by id, no wall clock is read), so published
// results can be replayed and compared instead of being trusted.
//
// The measurement is deliberately asymmetric, because the two error directions
// do not cost the same:
//
//   - must_abstain_recall / false_cite_rate — of the items that must NOT be
//     published, how many the gate blocked, and how many slipped through. A
//     false cite is the dangerous error (an ungrounded answer published as
//     source-backed), so it is reported on its own, never folded into a single
//     accuracy number;
//   - must_cite_recall — of the items that legitimately may be published, how
//     many the gate cited. Over-abstention is a real defect too (a gate that
//     blocks everything measures 100% on the safety side and is useless), so
//     it is measured as well.
//
// A label is mandatory: an item without one is reported as a bench defect and
// fails the run. Nothing is "measured" without ground truth.

// BenchItem is one labelled item of the public corpus: an answer record plus
// the decision the gate is expected to reach and the failure mode it probes.
type BenchItem struct {
	Answer
	ExpectedDecision string `json:"expected_decision"` // "cite" | "abstain"
	Category         string `json:"category"`          // the failure mode probed by the item
}

// Expectations are the decision labels a corpus may carry.
const (
	ExpectCite    = "cite"
	ExpectAbstain = "abstain"
)

// BenchThresholds are the optional bounds that turn the bench into a CI gate.
// Every field is a pointer: unset means REPORTED, NOT GATED — a bound nobody
// set must never pass silently (the #620 fail-closed rule, applied to the
// bench itself).
type BenchThresholds struct {
	MaxFalseCiteRate     *float64 `json:"max_false_cite_rate,omitempty"`
	MinMustCiteRecall    *float64 `json:"min_must_cite_recall,omitempty"`
	MinMustAbstainRecall *float64 `json:"min_must_abstain_recall,omitempty"`
}

// BenchItemResult is the measured outcome of one item.
type BenchItemResult struct {
	AnswerID         string   `json:"answer_id"`
	Category         string   `json:"category"`
	ExpectedDecision string   `json:"expected_decision"`
	Decision         string   `json:"decision"`
	Blocked          bool     `json:"blocked"`
	Agrees           bool     `json:"agrees"`
	FalseCite        bool     `json:"false_cite"`
	MissedCite       bool     `json:"missed_cite"`
	Findings         []string `json:"findings"`
	Groundedness     string   `json:"groundedness_method"`
}

// BenchCategory aggregates one failure mode across the corpus.
type BenchCategory struct {
	Category          string  `json:"category"`
	Items             int     `json:"items"`
	ExpectedAbstain   int     `json:"expected_abstain"`
	ExpectedCite      int     `json:"expected_cite"`
	BlockedAsExpected int     `json:"blocked_as_expected"`
	CitedAsExpected   int     `json:"cited_as_expected"`
	FalseCites        int     `json:"false_cites"`
	MissedCites       int     `json:"missed_cites"`
	MustAbstainRecall float64 `json:"must_abstain_recall"`
	MustCiteRecall    float64 `json:"must_cite_recall"`
}

// BenchResult is the measurement over a whole corpus.
type BenchResult struct {
	Status    string  `json:"status"` // "measured" | "fail"
	Items     int     `json:"items"`
	Agreement float64 `json:"agreement"`

	MustAbstainTotal   int     `json:"must_abstain_total"`
	MustAbstainBlocked int     `json:"must_abstain_blocked"`
	MustAbstainRecall  float64 `json:"must_abstain_recall"`
	FalseCites         int     `json:"false_cites"`
	FalseCiteRate      float64 `json:"false_cite_rate"`

	MustCiteTotal  int     `json:"must_cite_total"`
	MustCiteCited  int     `json:"must_cite_cited"`
	MustCiteRecall float64 `json:"must_cite_recall"`
	MissedCites    int     `json:"missed_cites"`

	Categories  []BenchCategory   `json:"categories"`
	Thresholds  BenchThresholds   `json:"thresholds"`
	Gates       Gates             `json:"gates"`
	Violations  []string          `json:"violations"`
	Defects     []string          `json:"defects"`
	ItemsDetail []BenchItemResult `json:"items_detail"`
}

// Bench runs the gate over a labelled corpus and MEASURES its agreement with
// the labels. A corpus defect (empty, unlabelled, unknown label) is never
// averaged into the metrics: it fails the run.
func Bench(items []BenchItem, cfg Config, th BenchThresholds) BenchResult {
	// Slices start empty, never nil: a published measurement reads `[]`, not
	// `null`, whichever side of the confusion matrix stayed empty.
	res := BenchResult{
		Status:      "measured",
		Thresholds:  th,
		Gates:       cfg.Gates(),
		Categories:  []BenchCategory{},
		Violations:  []string{},
		Defects:     []string{},
		ItemsDetail: []BenchItemResult{},
	}
	if len(items) == 0 {
		res.Status = "fail"
		res.Defects = []string{"empty bench corpus: nothing to measure"}
		return res
	}

	sort.SliceStable(items, func(i, j int) bool { return items[i].AnswerID < items[j].AnswerID })

	byCategory := map[string]*BenchCategory{}
	categoryOrder := []string{}
	for _, item := range items {
		expected := item.ExpectedDecision
		if expected != ExpectCite && expected != ExpectAbstain {
			res.Defects = append(res.Defects,
				"item "+label(item.AnswerID)+" carries no usable expected_decision (want \"cite\" or \"abstain\", got "+quote(expected)+")")
			continue
		}
		v := Evaluate(item.Answer, cfg)
		blocked := len(v.Findings) > 0 || v.Decision != ExpectCite
		agrees := (expected == ExpectAbstain && blocked) || (expected == ExpectCite && !blocked)
		codes := make([]string, 0, len(v.Findings))
		for _, f := range v.Findings {
			codes = append(codes, f.Code)
		}
		detail := BenchItemResult{
			AnswerID:         item.AnswerID,
			Category:         item.Category,
			ExpectedDecision: expected,
			Decision:         v.Decision,
			Blocked:          blocked,
			Agrees:           agrees,
			FalseCite:        expected == ExpectAbstain && !blocked,
			MissedCite:       expected == ExpectCite && blocked,
			Findings:         codes,
			Groundedness:     v.Groundedness.Method,
		}
		res.ItemsDetail = append(res.ItemsDetail, detail)
		res.Items++

		category := item.Category
		if category == "" {
			category = "unlabelled"
		}
		cat, ok := byCategory[category]
		if !ok {
			cat = &BenchCategory{Category: category}
			byCategory[category] = cat
			categoryOrder = append(categoryOrder, category)
		}
		cat.Items++
		switch expected {
		case ExpectAbstain:
			cat.ExpectedAbstain++
			res.MustAbstainTotal++
			if blocked {
				cat.BlockedAsExpected++
				res.MustAbstainBlocked++
			} else {
				cat.FalseCites++
				res.FalseCites++
			}
		case ExpectCite:
			cat.ExpectedCite++
			res.MustCiteTotal++
			if blocked {
				cat.MissedCites++
				res.MissedCites++
			} else {
				cat.CitedAsExpected++
				res.MustCiteCited++
			}
		}
	}

	if len(res.Defects) > 0 {
		res.Status = "fail"
	}
	if res.Items == 0 {
		res.Status = "fail"
		return res
	}

	agreed := 0
	for _, d := range res.ItemsDetail {
		if d.Agrees {
			agreed++
		}
	}
	res.Agreement = round4(float64(agreed) / float64(res.Items))
	if res.MustAbstainTotal > 0 {
		res.MustAbstainRecall = round4(float64(res.MustAbstainBlocked) / float64(res.MustAbstainTotal))
		res.FalseCiteRate = round4(float64(res.FalseCites) / float64(res.MustAbstainTotal))
	}
	if res.MustCiteTotal > 0 {
		res.MustCiteRecall = round4(float64(res.MustCiteCited) / float64(res.MustCiteTotal))
	}

	sort.Strings(categoryOrder)
	for _, name := range categoryOrder {
		cat := byCategory[name]
		if cat.ExpectedAbstain > 0 {
			cat.MustAbstainRecall = round4(float64(cat.BlockedAsExpected) / float64(cat.ExpectedAbstain))
		}
		if cat.ExpectedCite > 0 {
			cat.MustCiteRecall = round4(float64(cat.CitedAsExpected) / float64(cat.ExpectedCite))
		}
		res.Categories = append(res.Categories, *cat)
	}

	// The bench gates itself: a bound may only be set on a quantity that was
	// actually measured, otherwise an empty side of the confusion matrix would
	// satisfy every floor.
	switch {
	case th.MaxFalseCiteRate != nil && res.MustAbstainTotal == 0:
		res.Violations = append(res.Violations,
			"max_false_cite_rate is set but no item expects abstain: nothing was measured, refusing to pass")
	case th.MaxFalseCiteRate != nil && res.FalseCiteRate > *th.MaxFalseCiteRate:
		res.Violations = append(res.Violations, "false_cite_rate above the versioned ceiling")
	}
	switch {
	case th.MinMustAbstainRecall != nil && res.MustAbstainTotal == 0:
		res.Violations = append(res.Violations,
			"min_must_abstain_recall is set but no item expects abstain: nothing was measured, refusing to pass")
	case th.MinMustAbstainRecall != nil && res.MustAbstainRecall < *th.MinMustAbstainRecall:
		res.Violations = append(res.Violations, "must_abstain_recall below the versioned floor")
	}
	switch {
	case th.MinMustCiteRecall != nil && res.MustCiteTotal == 0:
		res.Violations = append(res.Violations,
			"min_must_cite_recall is set but no item expects cite: nothing was measured, refusing to pass")
	case th.MinMustCiteRecall != nil && res.MustCiteRecall < *th.MinMustCiteRecall:
		res.Violations = append(res.Violations, "must_cite_recall below the versioned floor")
	}

	sort.Strings(res.Violations)
	if len(res.Defects) == 0 && len(res.Violations) > 0 {
		res.Status = "fail"
	}
	return res
}

func label(id string) string {
	if id == "" {
		return "<unnamed item>"
	}
	return id
}

func quote(value string) string {
	if value == "" {
		return "\"\""
	}
	return "\"" + value + "\""
}
