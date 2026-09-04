package answer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Pluggable faithfulness scorer (#622, follow-up of VRC-10 A1).
//
// The gate's built-in support judge is the lexical proxy, negation-blind by
// construction. A Scorer is a second judge over the same (support text,
// answer sentence) pairs — typically a natural-language-inference model such
// as HHEM-2.1-Open — run OUTSIDE the engine: Nomos ships no model, downloads
// nothing, and stays deterministic. The composition rule is strictest-wins
// per sentence: a sentence counts as supported only when the lexical proxy
// AND the scorer support it, so plugging a scorer in can only tighten the
// verdict, never loosen it (the same direction as the self-declared-score
// rule: a producer may only lower its score).
//
// Fail-closed: a scorer that crashes, times out, answers in the wrong schema,
// omits or duplicates a pair, or returns a score outside [0,1] does not fall
// back to the lexical verdict silently — the answer scores 0 and carries a
// FAITHFULNESS_SCORER_FAILED finding. A configured judge that did not judge
// is not a pass.

// Pair is one entailment query: does Premise (the support text) support
// Hypothesis (one answer sentence)?
type Pair struct {
	ID         string `json:"id"`
	Premise    string `json:"premise"`
	Hypothesis string `json:"hypothesis"`
}

// ScoreResult carries one score per pair, aligned with the request, each the
// probability in [0,1] that the premise supports the hypothesis.
type ScoreResult struct {
	Method string
	Scores []float64
}

// Scorer judges (premise, hypothesis) pairs.
type Scorer interface {
	Score(pairs []Pair) (ScoreResult, error)
}

// ScorerFunc adapts a function to Scorer.
type ScorerFunc func(pairs []Pair) (ScoreResult, error)

// Score implements Scorer.
func (f ScorerFunc) Score(pairs []Pair) (ScoreResult, error) { return f(pairs) }

// Versions of the external scorer exchange. Any change to the shape MUST
// bump these: the engine refuses a response in another version.
const (
	ScorerRequestSchema  = "nomos-scorer-request-v1"
	ScorerResponseSchema = "nomos-scorer-response-v1"
)

// ScorerRequest is what an external scorer reads on stdin.
type ScorerRequest struct {
	SchemaVersion string `json:"schema_version"`
	Pairs         []Pair `json:"pairs"`
}

// ScorerResponse is what an external scorer writes on stdout.
type ScorerResponse struct {
	SchemaVersion string        `json:"schema_version"`
	Method        string        `json:"method"`
	Scores        []ScorerScore `json:"scores"`
}

// ScorerScore is one scored pair.
type ScorerScore struct {
	ID    string  `json:"id"`
	Score float64 `json:"score"`
}

// DefaultScorerTimeout bounds one external scorer batch.
const DefaultScorerTimeout = 2 * time.Minute

// ExternalScorer runs a command per batch: the request JSON on stdin, the
// response JSON on stdout, a non-zero exit (or silence) being a failure.
type ExternalScorer struct {
	Command []string
	Timeout time.Duration
	// Env is appended to the inherited environment (e.g. HF_HOME).
	Env []string
}

// Score implements Scorer.
func (e ExternalScorer) Score(pairs []Pair) (ScoreResult, error) {
	if len(e.Command) == 0 || strings.TrimSpace(e.Command[0]) == "" {
		return ScoreResult{}, errors.New("external scorer: empty command")
	}
	name := e.Command[0]
	if len(pairs) == 0 {
		// Nothing to judge: no process is spawned.
		return ScoreResult{Method: "external:" + name, Scores: []float64{}}, nil
	}
	request, err := json.Marshal(ScorerRequest{SchemaVersion: ScorerRequestSchema, Pairs: pairs})
	if err != nil {
		return ScoreResult{}, fmt.Errorf("external scorer %q: encode request: %w", name, err)
	}
	timeout := e.Timeout
	if timeout <= 0 {
		timeout = DefaultScorerTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, e.Command[1:]...)
	cmd.Stdin = bytes.NewReader(request)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), e.Env...)
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return ScoreResult{}, fmt.Errorf("external scorer %q timed out after %s", name, timeout)
		}
		return ScoreResult{}, fmt.Errorf("external scorer %q failed: %v%s", name, err, stderrTail(stderr.String()))
	}
	var resp ScorerResponse
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &resp); err != nil {
		return ScoreResult{}, fmt.Errorf("external scorer %q: response is not %s JSON: %v%s", name, ScorerResponseSchema, err, stderrTail(stderr.String()))
	}
	res, err := decodeScorerResponse(pairs, resp)
	if err != nil {
		return ScoreResult{}, fmt.Errorf("external scorer %q: %w", name, err)
	}
	return res, nil
}

// decodeScorerResponse maps a response onto the request order (by id) and
// validates it: every pair scored exactly once, nothing else.
func decodeScorerResponse(pairs []Pair, resp ScorerResponse) (ScoreResult, error) {
	if resp.SchemaVersion != ScorerResponseSchema {
		return ScoreResult{}, fmt.Errorf("response schema %q is not %s", resp.SchemaVersion, ScorerResponseSchema)
	}
	byID := make(map[string]float64, len(resp.Scores))
	for _, s := range resp.Scores {
		if _, dup := byID[s.ID]; dup {
			return ScoreResult{}, fmt.Errorf("response scores pair %q twice", s.ID)
		}
		byID[s.ID] = s.Score
	}
	scores := make([]float64, len(pairs))
	for i, p := range pairs {
		v, ok := byID[p.ID]
		if !ok {
			return ScoreResult{}, fmt.Errorf("response carries no score for pair %q", p.ID)
		}
		scores[i] = v
		delete(byID, p.ID)
	}
	if len(byID) > 0 {
		unknown := make([]string, 0, len(byID))
		for id := range byID {
			unknown = append(unknown, id)
		}
		return ScoreResult{}, fmt.Errorf("response scores %d unknown pair(s): %s", len(unknown), strings.Join(unknown, ", "))
	}
	res := ScoreResult{Method: resp.Method, Scores: scores}
	if err := validateScores(pairs, res); err != nil {
		return ScoreResult{}, err
	}
	return res, nil
}

// validateScores enforces the contract every scorer, in-process or external,
// must meet before its verdict counts: a named method, one score per pair,
// each a finite number in [0,1].
func validateScores(pairs []Pair, res ScoreResult) error {
	if strings.TrimSpace(res.Method) == "" {
		return errors.New("scorer declared no method")
	}
	if len(res.Scores) != len(pairs) {
		return fmt.Errorf("scorer %s returned %d score(s) for %d pair(s)", res.Method, len(res.Scores), len(pairs))
	}
	for i, s := range res.Scores {
		if math.IsNaN(s) || math.IsInf(s, 0) || s < 0 || s > 1 {
			return fmt.Errorf("scorer %s: score %v for pair %q is outside [0,1]", res.Method, s, pairs[i].ID)
		}
	}
	return nil
}

// stderrTail keeps the end of a scorer's stderr for the error message.
func stderrTail(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	const max = 400
	if len(s) > max {
		s = "…" + s[len(s)-max:]
	}
	return " (stderr: " + s + ")"
}
