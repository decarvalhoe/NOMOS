package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
