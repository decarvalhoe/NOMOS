package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RBOKproject/Nomos/cli/internal/answer"
)

// VRC-10 (#556) — the `answer gate` CLI surface: cites a grounded answer,
// abstains + exits 1 on an ungrounded one.

const groundedFixture = `answers:
  - answer_id: A1
    prompt_id: P1
    answer: "Le gabarit retient une hauteur de neuf metres au faite."
    citation_status: source_backed
    policy_outcome: acceptable
    confidence: 0.99
    source_spans:
      - source_id: S1
        source_hash: "sha256:abc"
        span: L1-L2
        chunk_id: c1
        text: "Le gabarit retient une hauteur de neuf metres au faite pour le volume principal."
    retrieved_chunks:
      - chunk_id: c1
        text: "Le gabarit retient une hauteur de neuf metres au faite."
`

const ungroundedFixture = `answers:
  - answer_id: B1
    prompt_id: P1
    answer: "Une affirmation fabriquee sans aucune source pour la verifier."
    citation_status: source_backed
    policy_outcome: acceptable
    confidence: 0.99
    source_spans: []
    retrieved_chunks: []
`

func writeAnswerFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "answers.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAnswerGate_CitesGroundedAnswer(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"answer", "gate", "--fixtures", writeAnswerFixture(t, groundedFixture)}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("grounded answer must pass, got %d: %s", code, stderr.String())
	}
	var res map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		t.Fatalf("output not JSON: %v", err)
	}
	if res["status"] != "pass" {
		t.Fatalf("expected pass, got %v", res["status"])
	}
}

func TestAnswerGate_AbstainsAndFailsOnUngrounded(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"answer", "gate", "--fixtures", writeAnswerFixture(t, ungroundedFixture)}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("an ungrounded answer must exit 1, got %d", code)
	}
	var res map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		t.Fatalf("output not JSON: %v", err)
	}
	if res["status"] != "fail" {
		t.Fatalf("expected fail, got %v", res["status"])
	}
}

// #620 — `answer eval` with context metrics: a corpus whose answer leans on a
// distractor passes the per-answer gate (lexical faithfulness cannot see it)
// and is caught by the versioned noise ceiling.

const noisyEvalCorpus = `answers:
  - answer_id: EVAL-NOISY
    prompt_id: Q-GABARIT
    expected_chunk_ids: [chunk-gabarit]
    answer: "Le gabarit retient une hauteur de neuf metres au faite. La mise a l'enquete publique dure trente jours."
    citation_status: source_backed
    policy_outcome: acceptable
    confidence: 0.95
    source_spans:
      - source_id: VD-CONCEPTION
        source_hash: "sha256:1111"
        span: L13-L14
        chunk_id: chunk-gabarit
        text: "Le gabarit retient une hauteur de neuf metres au faite pour le volume principal."
      - source_id: VD-PERMIS
        source_hash: "sha256:3333"
        span: L12-L13
        chunk_id: chunk-enquete
        text: "La mise a l'enquete publique dure trente jours pour le dossier d'autorisation."
    retrieved_chunks:
      - chunk_id: chunk-gabarit
        text: "Le gabarit retient une hauteur de neuf metres au faite pour le volume principal."
      - chunk_id: chunk-enquete
        text: "La mise a l'enquete publique dure trente jours pour le dossier d'autorisation."
`

const baseThresholds = `min_mean_citation_recall: 0.95
min_mean_citation_precision: 0.95
min_mean_faithfulness: 0.95
max_failed_answers: 0
`

func evalFixtureFile(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestAnswerEval_NoiseRegressionTurnsRed(t *testing.T) {
	corpus := evalFixtureFile(t, "corpus.yaml", noisyEvalCorpus)

	var stdout, stderr bytes.Buffer
	lenient := evalFixtureFile(t, "lenient.yaml", baseThresholds)
	if code := Run([]string{"answer", "eval", "--corpus", corpus, "--thresholds", lenient}, &stdout, &stderr); code != 0 {
		t.Fatalf("without a noise ceiling the corpus passes (reported, not gated), got %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"mean_noise_sensitivity": 0.5`) || !strings.Contains(stdout.String(), `"noise_induced_sentences": 1`) {
		t.Fatalf("context metrics must be reported: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	strict := evalFixtureFile(t, "strict.yaml", baseThresholds+"max_mean_noise_sensitivity: 0.0\nmin_mean_context_recall: 1.0\n")
	if code := Run([]string{"answer", "eval", "--corpus", corpus, "--thresholds", strict}, &stdout, &stderr); code != 1 {
		t.Fatalf("noise above the ceiling must exit 1, got %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "mean_noise_sensitivity above the versioned ceiling") {
		t.Fatalf("regression must be named: %s", stdout.String())
	}
}

func TestAnswerGate_RequiresFixtures(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"answer", "gate"}, &stdout, &stderr); code != 2 {
		t.Fatalf("missing --fixtures must exit 2, got %d", code)
	}
}

// --- external faithfulness scorer (#622) --------------------------------------

// negatedFixture contradicts its own source; the lexical proxy cannot tell.
const negatedFixture = `answers:
  - answer_id: NEG-1
    prompt_id: P-NEG
    answer: "Le delai ne court pas des la notification."
    citation_status: source_backed
    policy_outcome: acceptable
    confidence: 0.99
    source_spans:
      - source_id: S1
        source_hash: "sha256:abc"
        span: L4-L5
        chunk_id: c-delai
        text: "Le delai court des la notification."
    retrieved_chunks:
      - chunk_id: c-delai
        text: "Le delai court des la notification."
`

// TestHelperScorerProcess is not a test: it is the external scorer the CLI
// tests spawn — this test binary re-executed in helper mode.
func TestHelperScorerProcess(t *testing.T) {
	if os.Getenv("NOMOS_SCORER_HELPER") != "1" {
		return
	}
	defer os.Exit(0)
	var req answer.ScorerRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		fmt.Fprintln(os.Stderr, "helper: bad request:", err)
		os.Exit(2)
	}
	if os.Getenv("NOMOS_SCORER_MODE") == "crash" {
		fmt.Fprintln(os.Stderr, "model backend unavailable")
		os.Exit(3)
	}
	resp := answer.ScorerResponse{SchemaVersion: answer.ScorerResponseSchema, Method: "helper-nli"}
	for _, p := range req.Pairs {
		score := 0.95
		if strings.Contains(" "+p.Hypothesis+" ", " ne ") {
			score = 0.05
		}
		resp.Scores = append(resp.Scores, answer.ScorerScore{ID: p.ID, Score: score})
	}
	_ = json.NewEncoder(os.Stdout).Encode(resp)
}

// helperScorerCmd returns the --scorer-cmd value that re-executes this binary
// as the scorer, in the given mode.
func helperScorerCmd(t *testing.T, mode string) string {
	t.Helper()
	if strings.ContainsAny(os.Args[0], " \t") {
		t.Skip("--scorer-cmd is whitespace-split; the test binary path contains spaces")
	}
	t.Setenv("NOMOS_SCORER_HELPER", "1")
	t.Setenv("NOMOS_SCORER_MODE", mode)
	return os.Args[0] + " -test.run=^TestHelperScorerProcess$ --"
}

func TestAnswerGate_ScorerCmdBlocksNegatedClaim(t *testing.T) {
	fixtures := evalFixtureFile(t, "answers.yaml", negatedFixture)
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"answer", "gate", "--fixtures", fixtures}, &stdout, &stderr); code != 0 {
		t.Fatalf("precondition: the lexical proxy alone accepts the negated claim (documented limitation), got %d: %s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	cmd := helperScorerCmd(t, "ok")
	if code := Run([]string{"answer", "gate", "--fixtures", fixtures, "--scorer-cmd", cmd}, &stdout, &stderr); code != 1 {
		t.Fatalf("with the scorer the negated claim must be blocked (exit 1), got %d: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, `"method": "lexical_entailment_v1+helper-nli"`) || !strings.Contains(out, `"decision": "abstain"`) {
		t.Fatalf("the verdict must show both judges and abstain: %s", out)
	}
}

func TestAnswerGate_ScorerCmdFailureFailsClosed(t *testing.T) {
	fixtures := evalFixtureFile(t, "answers.yaml", groundedFixture)
	cmd := helperScorerCmd(t, "crash")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"answer", "gate", "--fixtures", fixtures, "--scorer-cmd", cmd}, &stdout, &stderr); code != 1 {
		t.Fatalf("a crashed scorer must fail the gate closed, got %d: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, answer.FindingScorerFailed) || !strings.Contains(out, `"method": "scorer_failed"`) {
		t.Fatalf("the failure must be a named finding, got: %s", out)
	}
	if strings.Contains(out, `"decision": "cite"`) {
		t.Fatalf("a failed judge must not leave a cite standing: %s", out)
	}
}

func TestAnswerEval_ScorerCmdAppliesToTheHarness(t *testing.T) {
	corpus := evalFixtureFile(t, "corpus.yaml", negatedFixture)
	thresholds := evalFixtureFile(t, "thresholds.yaml", baseThresholds)
	cmd := helperScorerCmd(t, "ok")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"answer", "eval", "--corpus", corpus, "--thresholds", thresholds, "--scorer-cmd", cmd}, &stdout, &stderr); code != 1 {
		t.Fatalf("the harness must apply the scorer and turn red on the negated claim, got %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "mean_faithfulness below the versioned floor") {
		t.Fatalf("regression must be named: %s", stdout.String())
	}
}

func TestAnswerGate_ScorerFlagsAreValidated(t *testing.T) {
	fixtures := evalFixtureFile(t, "answers.yaml", groundedFixture)
	for _, extra := range [][]string{
		{"--scorer-cmd", "some-scorer", "--scorer-threshold", "1.5"},
		{"--scorer-cmd", "some-scorer", "--scorer-threshold", "-0.1"},
		{"--scorer-cmd", "some-scorer", "--scorer-timeout", "0s"},
	} {
		var stdout, stderr bytes.Buffer
		args := append([]string{"answer", "gate", "--fixtures", fixtures}, extra...)
		if code := Run(args, &stdout, &stderr); code != 2 {
			t.Fatalf("%v must be a usage error, got %d: %s", extra, code, stderr.String())
		}
	}
}
