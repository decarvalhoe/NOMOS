package app

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/RBOKproject/Nomos/cli/internal/answer"
)

// answerCommand is `nomos answer`: the cite-or-abstain gate (VRC-10 #556, A1).
// `answer gate` recomputes faithfulness from the retrieved span text and emits
// a cite/abstain verdict per answer; it exits 1 when any answer carries a
// blocking finding (the gate is bounding, not advisory).
func answerCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: nomos answer gate --fixtures <answers.yaml> [--format json]")
		return 2
	}
	switch args[0] {
	case "gate":
		return answerGateCommand(args[1:], stdout, stderr)
	case "eval":
		return answerEvalCommand(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown answer subcommand %q (try: gate, eval)\n", args[0])
		return 2
	}
}

type answerFixtureDoc struct {
	Answers []answer.Answer `json:"answers"`
}

func answerGateCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("answer gate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	fixtures := flags.String("fixtures", "", "RAG answer fixtures YAML (answers: [...]) (required)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*fixtures) == "" {
		fmt.Fprintln(stderr, "answer gate: --fixtures is required")
		return 2
	}
	raw, err := os.ReadFile(*fixtures)
	if err != nil {
		fmt.Fprintf(stderr, "answer gate: read fixtures: %v\n", err)
		return 1
	}
	// Bridge yaml → json so the engine structs (json-tagged) decode the sidecar
	// fixture field names without a second tag set.
	var generic any
	if err := yaml.Unmarshal(raw, &generic); err != nil {
		fmt.Fprintf(stderr, "answer gate: parse fixtures: %v\n", err)
		return 1
	}
	bridged, err := json.Marshal(normalizeYAML(generic))
	if err != nil {
		fmt.Fprintf(stderr, "answer gate: normalize fixtures: %v\n", err)
		return 1
	}
	var doc answerFixtureDoc
	if err := json.Unmarshal(bridged, &doc); err != nil {
		fmt.Fprintf(stderr, "answer gate: decode fixtures: %v\n", err)
		return 1
	}

	result := answer.Gate(doc.Answers, answer.Defaults())
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintf(stderr, "answer gate: write: %v\n", err)
		return 1
	}
	if result.Status == "fail" {
		return 1
	}
	return 0
}

// answerEvalCommand runs the regulated RAG evaluation harness (VRC-13): the
// gate over a golden corpus PLUS the aggregate-vs-versioned-thresholds check.
func answerEvalCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("answer eval", flag.ContinueOnError)
	flags.SetOutput(stderr)
	corpus := flags.String("corpus", "", "golden RAG eval corpus YAML (required)")
	thresholdsPath := flags.String("thresholds", "", "versioned thresholds YAML (required)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*corpus) == "" || strings.TrimSpace(*thresholdsPath) == "" {
		fmt.Fprintln(stderr, "answer eval: --corpus and --thresholds are required")
		return 2
	}
	var doc answerFixtureDoc
	if code := loadYAMLInto(*corpus, &doc, "answer eval: corpus", stderr); code != 0 {
		return code
	}
	var th answer.EvalThresholds
	if code := loadYAMLInto(*thresholdsPath, &th, "answer eval: thresholds", stderr); code != 0 {
		return code
	}

	result := answer.Eval(doc.Answers, answer.Defaults(), th)
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintf(stderr, "answer eval: write: %v\n", err)
		return 1
	}
	if result.Status == "fail" {
		return 1
	}
	return 0
}

// loadYAMLInto bridges a YAML file into a json-tagged struct (yaml → generic →
// json). Returns 0 on success or a non-zero exit code after logging.
func loadYAMLInto(path string, dst any, prefix string, stderr io.Writer) int {
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "%s: read: %v\n", prefix, err)
		return 1
	}
	var generic any
	if err := yaml.Unmarshal(raw, &generic); err != nil {
		fmt.Fprintf(stderr, "%s: parse: %v\n", prefix, err)
		return 1
	}
	bridged, err := json.Marshal(normalizeYAML(generic))
	if err != nil {
		fmt.Fprintf(stderr, "%s: normalize: %v\n", prefix, err)
		return 1
	}
	if err := json.Unmarshal(bridged, dst); err != nil {
		fmt.Fprintf(stderr, "%s: decode: %v\n", prefix, err)
		return 1
	}
	return 0
}
