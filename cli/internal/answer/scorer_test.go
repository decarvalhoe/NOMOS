package answer

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// #622 — the pluggable second judge. The central proofs: a negated claim the
// lexical proxy accepts is blocked once a scorer is plugged in; a scorer can
// only tighten the verdict, never loosen it; a scorer that did not judge fails
// the gate closed; the external protocol is validated in every direction.

// negatedAnswer contradicts its own source: the source says the delay runs,
// the answer says it does not. Content tokens overlap almost entirely.
func negatedAnswer() Answer {
	return Answer{
		AnswerID:       "NEG-1",
		PromptID:       "P-NEG",
		Answer:         "Le delai ne court pas des la notification.",
		CitationStatus: "source_backed",
		PolicyOutcome:  "acceptable",
		Confidence:     0.99,
		SourceSpans: []Span{{
			SourceID: "S1", SourceHash: "sha256:abc", Span: "L4-L5", ChunkID: "c-delai",
			Text: "Le delai court des la notification.",
		}},
		RetrievedChunks: []Chunk{{ChunkID: "c-delai", Text: "Le delai court des la notification."}},
	}
}

// fakeNLI is an in-process stand-in for an entailment model: it recognises
// the negation the lexical proxy cannot see.
func fakeNLI(pairs []Pair) (ScoreResult, error) {
	res := ScoreResult{Method: "fake-nli"}
	for _, p := range pairs {
		if strings.Contains(" "+p.Hypothesis+" ", " ne ") || strings.Contains(" "+p.Hypothesis+" ", " pas ") {
			res.Scores = append(res.Scores, 0.03)
		} else {
			res.Scores = append(res.Scores, 0.97)
		}
	}
	return res, nil
}

func withScorer(s Scorer) Config {
	cfg := Defaults()
	cfg.Scorer = s
	return cfg
}

// The documented limitation, kept as a test so the day the lexical proxy
// stops being negation-blind, this file says so.
func TestGate_LexicalProxyIsNegationBlind_KnownLimitation(t *testing.T) {
	v := Evaluate(negatedAnswer(), Defaults())
	if v.Decision != "cite" || v.Faithfulness != 1.0 {
		t.Fatalf("the lexical proxy alone was expected to accept the negated claim (documented limitation), got decision=%q faithfulness=%v", v.Decision, v.Faithfulness)
	}
	if v.Groundedness.Method != methodLexical {
		t.Fatalf("without a scorer the method stays lexical, got %q", v.Groundedness.Method)
	}
}

func TestGate_NegatedClaimBlockedByScorerNotByLexicalProxy(t *testing.T) {
	v := Evaluate(negatedAnswer(), withScorer(ScorerFunc(fakeNLI)))
	if v.Decision != "abstain" {
		t.Fatalf("the scorer must block the negated claim, got %q (findings %+v)", v.Decision, v.Findings)
	}
	if v.Faithfulness != 0 {
		t.Fatalf("expected faithfulness 0 under strictest-wins, got %v", v.Faithfulness)
	}
	g := v.Groundedness
	if g.Method != methodLexical+"+fake-nli" || g.ScorerMethod != "fake-nli" {
		t.Fatalf("method must record both judges, got %q / %q", g.Method, g.ScorerMethod)
	}
	if g.LexicalSupportedSentences != 1 || g.ScorerSupportedSentences != 0 || g.SupportedSentences != 0 {
		t.Fatalf("per-judge counts must show who refused: lexical=%d scorer=%d final=%d", g.LexicalSupportedSentences, g.ScorerSupportedSentences, g.SupportedSentences)
	}
	if !hasFinding(v, "DEEPEVAL_FAITHFULNESS_BELOW_GATE") {
		t.Fatalf("expected the faithfulness gate finding, got %+v", v.Findings)
	}
	if v.TrustTier != "unverified" {
		t.Fatalf("a blocked answer cannot be trusted, got %q", v.TrustTier)
	}
	if !strings.Contains(g.Limitation, "strictest-wins") {
		t.Fatalf("the composition rule must be stated in the limitation, got %q", g.Limitation)
	}
}

// Strictest wins: a scorer saying 1.0 cannot rescue a sentence the lexical
// proxy does not support. Plugging a scorer in never loosens the gate.
func TestGate_ScorerCannotLoosenTheLexicalVerdict(t *testing.T) {
	a := groundedAnswer()
	a.Answer = "Cette phrase parle de tout autre chose, completement hors sujet ici."
	generous := ScorerFunc(func(pairs []Pair) (ScoreResult, error) {
		res := ScoreResult{Method: "always-yes"}
		for range pairs {
			res.Scores = append(res.Scores, 1.0)
		}
		return res, nil
	})
	lexicalOnly := Evaluate(a, Defaults())
	withYes := Evaluate(a, withScorer(generous))
	if lexicalOnly.Decision != "abstain" {
		t.Fatalf("precondition: the off-topic answer must abstain lexically, got %q", lexicalOnly.Decision)
	}
	if withYes.Decision != "abstain" || withYes.Faithfulness > lexicalOnly.Faithfulness {
		t.Fatalf("a generous scorer loosened the verdict: lexical=%v with-scorer=%v decision=%q", lexicalOnly.Faithfulness, withYes.Faithfulness, withYes.Decision)
	}
	if withYes.Groundedness.ScorerSupportedSentences != 1 || withYes.Groundedness.SupportedSentences != 0 {
		t.Fatalf("expected scorer=1 final=0, got %+v", withYes.Groundedness)
	}
}

func TestGate_ScorerFailureFailsClosed(t *testing.T) {
	cases := map[string]ScorerFunc{
		"error": func([]Pair) (ScoreResult, error) { return ScoreResult{}, errors.New("model not available") },
		"wrong length": func([]Pair) (ScoreResult, error) {
			return ScoreResult{Method: "short", Scores: nil}, nil
		},
		"out of range": func(pairs []Pair) (ScoreResult, error) {
			res := ScoreResult{Method: "loud"}
			for range pairs {
				res.Scores = append(res.Scores, 1.2)
			}
			return res, nil
		},
		"no method": func(pairs []Pair) (ScoreResult, error) {
			res := ScoreResult{}
			for range pairs {
				res.Scores = append(res.Scores, 0.9)
			}
			return res, nil
		},
	}
	for name, scorer := range cases {
		t.Run(name, func(t *testing.T) {
			// A grounded answer that the lexical proxy cites: the failure of
			// the second judge must not let that verdict stand.
			v := Evaluate(groundedAnswer(), withScorer(scorer))
			if v.Decision != "abstain" || v.Faithfulness != 0 {
				t.Fatalf("a failed scorer must fail closed, got decision=%q faithfulness=%v", v.Decision, v.Faithfulness)
			}
			if v.Groundedness.Method != methodScorerFailed || v.Groundedness.ScorerError == "" {
				t.Fatalf("the failure must be recorded, got %+v", v.Groundedness)
			}
			if !hasFinding(v, FindingScorerFailed) {
				t.Fatalf("expected %s, got %+v", FindingScorerFailed, v.Findings)
			}
		})
	}
	// The finding is raised whatever the policy outcome, so the batch fails.
	a := groundedAnswer()
	a.PolicyOutcome = "needs_review"
	res := Gate([]Answer{a}, withScorer(cases["error"]))
	if res.Status != "fail" || !hasFinding(res.Verdicts[0], FindingScorerFailed) {
		t.Fatalf("a batch whose judge failed must fail, got %+v", res)
	}
}

func TestGate_ScorerIsNotConsultedForRefusals(t *testing.T) {
	exploding := ScorerFunc(func([]Pair) (ScoreResult, error) {
		return ScoreResult{}, errors.New("must not be called")
	})
	a := Answer{
		AnswerID: "R1", PolicyOutcome: "acceptable_refusal",
		RefusalStatus: "refused", CitationStatus: "none",
	}
	v := Evaluate(a, withScorer(exploding))
	if v.Decision != "abstain" || len(v.Findings) != 0 || v.Faithfulness != 1.0 {
		t.Fatalf("a refusal asserts nothing and needs no judge, got %+v", v)
	}
}

// --- external protocol ------------------------------------------------------

// helperScorer runs this test binary as the external scorer, in the mode
// given (see TestHelperScorerProcess).
func helperScorer(mode string, timeout time.Duration) ExternalScorer {
	return ExternalScorer{
		Command: []string{os.Args[0], "-test.run=^TestHelperScorerProcess$", "--"},
		Timeout: timeout,
		Env:     []string{"NOMOS_SCORER_HELPER=1", "NOMOS_SCORER_MODE=" + mode},
	}
}

// TestHelperScorerProcess is not a test: it is the external scorer the tests
// above spawn. It reads the protocol request on stdin and answers according
// to NOMOS_SCORER_MODE.
func TestHelperScorerProcess(t *testing.T) {
	if os.Getenv("NOMOS_SCORER_HELPER") != "1" {
		return
	}
	defer os.Exit(0)
	var req ScorerRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		fmt.Fprintln(os.Stderr, "helper: bad request:", err)
		os.Exit(2)
	}
	if req.SchemaVersion != ScorerRequestSchema {
		fmt.Fprintln(os.Stderr, "helper: unexpected request schema", req.SchemaVersion)
		os.Exit(2)
	}
	resp := ScorerResponse{SchemaVersion: ScorerResponseSchema, Method: "helper-nli"}
	for _, p := range req.Pairs {
		score := 0.95
		if strings.Contains(" "+p.Hypothesis+" ", " ne ") || strings.Contains(" "+p.Hypothesis+" ", " pas ") {
			score = 0.05
		}
		resp.Scores = append(resp.Scores, ScorerScore{ID: p.ID, Score: score})
	}
	switch os.Getenv("NOMOS_SCORER_MODE") {
	case "ok":
		// Answer in REVERSE order: the engine must align by id, not position.
		for i, j := 0, len(resp.Scores)-1; i < j; i, j = i+1, j-1 {
			resp.Scores[i], resp.Scores[j] = resp.Scores[j], resp.Scores[i]
		}
	case "missing":
		resp.Scores = resp.Scores[:len(resp.Scores)-1]
	case "duplicate":
		resp.Scores = append(resp.Scores, resp.Scores[0])
	case "unknown":
		resp.Scores = append(resp.Scores, ScorerScore{ID: "ghost", Score: 0.5})
	case "range":
		resp.Scores[0].Score = 1.5
	case "no-method":
		resp.Method = ""
	case "wrong-schema":
		resp.SchemaVersion = "nomos-scorer-response-v0"
	case "garbage":
		fmt.Print("this is not json")
		return
	case "crash":
		fmt.Fprintln(os.Stderr, "model backend unavailable: transformers not importable")
		os.Exit(3)
	case "hang":
		time.Sleep(5 * time.Second)
	default:
		fmt.Fprintln(os.Stderr, "helper: unknown mode")
		os.Exit(2)
	}
	_ = json.NewEncoder(os.Stdout).Encode(resp)
}

func TestExternalScorer_RoundTripAlignsByID(t *testing.T) {
	pairs := []Pair{
		{ID: "s0", Premise: "Le delai court des la notification.", Hypothesis: "Le delai court des la notification"},
		{ID: "s1", Premise: "Le delai court des la notification.", Hypothesis: "Le delai ne court pas des la notification"},
		{ID: "s2", Premise: "Le delai court des la notification.", Hypothesis: "Un delai est prevu"},
	}
	res, err := helperScorer("ok", 10*time.Second).Score(pairs)
	if err != nil {
		t.Fatalf("round trip failed: %v", err)
	}
	if res.Method != "helper-nli" {
		t.Fatalf("method must come from the response, got %q", res.Method)
	}
	want := []float64{0.95, 0.05, 0.95}
	for i, w := range want {
		if res.Scores[i] != w {
			t.Fatalf("scores must be aligned by id despite reversed response order: got %v want %v", res.Scores, want)
		}
	}
}

func TestExternalScorer_NegatedClaimIsBlockedEndToEnd(t *testing.T) {
	v := Evaluate(negatedAnswer(), withScorer(helperScorer("ok", 10*time.Second)))
	if v.Decision != "abstain" || v.Groundedness.Method != methodLexical+"+helper-nli" {
		t.Fatalf("expected the external judge to block the negated claim, got decision=%q method=%q err=%q", v.Decision, v.Groundedness.Method, v.Groundedness.ScorerError)
	}
}

func TestExternalScorer_RejectsMalformedResponses(t *testing.T) {
	pairs := []Pair{{ID: "s0", Premise: "p", Hypothesis: "h"}, {ID: "s1", Premise: "p", Hypothesis: "h2"}}
	for _, mode := range []string{"missing", "duplicate", "unknown", "range", "no-method", "wrong-schema", "garbage", "crash"} {
		t.Run(mode, func(t *testing.T) {
			_, err := helperScorer(mode, 10*time.Second).Score(pairs)
			if err == nil {
				t.Fatalf("mode %s must be refused", mode)
			}
			if mode == "crash" && !strings.Contains(err.Error(), "transformers not importable") {
				t.Fatalf("the scorer's stderr must surface in the error, got %v", err)
			}
		})
	}
}

func TestExternalScorer_TimesOut(t *testing.T) {
	_, err := helperScorer("hang", 300*time.Millisecond).Score([]Pair{{ID: "s0", Premise: "p", Hypothesis: "h"}})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("a hanging scorer must time out, got %v", err)
	}
}

func TestExternalScorer_EmptyCommandAndEmptyBatch(t *testing.T) {
	if _, err := (ExternalScorer{}).Score([]Pair{{ID: "s0"}}); err == nil {
		t.Fatal("an empty command must be an error")
	}
	// No pairs: nothing to judge, no process spawned (a bogus command proves it).
	res, err := (ExternalScorer{Command: []string{"/nonexistent/scorer"}}).Score(nil)
	if err != nil || len(res.Scores) != 0 {
		t.Fatalf("an empty batch must not spawn the scorer, got %+v / %v", res, err)
	}
}
