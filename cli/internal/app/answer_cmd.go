package app

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/RBOKproject/Nomos/cli/internal/answer"
)

// answerCommand is `nomos answer`: the cite-or-abstain gate (VRC-10 #556, A1).
// `answer gate` recomputes faithfulness from the retrieved span text and emits
// a cite/abstain verdict per answer; it exits 1 when any answer carries a
// blocking finding (the gate is bounding, not advisory). Both subcommands
// accept an external faithfulness scorer (#622) as a second judge.
func answerCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: nomos answer <gate --fixtures <answers.yaml> | eval --corpus <corpus.yaml> --thresholds <thresholds.yaml>> [--scorer-cmd <cmd> [--scorer-threshold 0.5] [--scorer-timeout 2m]]")
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

// scorerFlags are the optional second-judge flags (#622), shared by `gate`
// and `eval`. Nomos ships no model: the scorer is an external command that
// speaks the versioned JSON protocol (answer.ScorerRequestSchema on stdin,
// answer.ScorerResponseSchema on stdout).
type scorerFlags struct {
	cmd       *string
	threshold *float64
	timeout   *time.Duration
}

func registerScorerFlags(flags *flag.FlagSet) scorerFlags {
	return scorerFlags{
		cmd: flags.String("scorer-cmd", "",
			"external faithfulness scorer command, whitespace-split (JSON protocol "+answer.ScorerRequestSchema+" on stdin, "+answer.ScorerResponseSchema+" on stdout); default: lexical proxy only"),
		threshold: flags.Float64("scorer-threshold", answer.Defaults().ScorerThreshold,
			"scorer probability at or above which a sentence counts as supported (strictest-wins with the lexical proxy)"),
		timeout: flags.Duration("scorer-timeout", answer.DefaultScorerTimeout, "external scorer timeout per batch"),
	}
}

// config builds the gate configuration; a non-zero code is a usage error.
func (s scorerFlags) config(prefix string, stderr io.Writer) (answer.Config, int) {
	cfg := answer.Defaults()
	command := strings.Fields(*s.cmd)
	if len(command) == 0 {
		return cfg, 0
	}
	if *s.threshold < 0 || *s.threshold > 1 {
		fmt.Fprintf(stderr, "%s: --scorer-threshold must be within [0,1], got %v\n", prefix, *s.threshold)
		return cfg, 2
	}
	if *s.timeout <= 0 {
		fmt.Fprintf(stderr, "%s: --scorer-timeout must be positive, got %s\n", prefix, *s.timeout)
		return cfg, 2
	}
	cfg.Scorer = answer.ExternalScorer{Command: command, Timeout: *s.timeout}
	cfg.ScorerThreshold = *s.threshold
	return cfg, 0
}

func answerGateCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("answer gate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	fixtures := flags.String("fixtures", "", "RAG answer fixtures YAML (answers: [...]) (required)")
	scorer := registerScorerFlags(flags)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*fixtures) == "" {
		fmt.Fprintln(stderr, "answer gate: --fixtures is required")
		return 2
	}
	cfg, code := scorer.config("answer gate", stderr)
	if code != 0 {
		return code
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

	result := answer.Gate(doc.Answers, cfg)
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
	scorer := registerScorerFlags(flags)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*corpus) == "" || strings.TrimSpace(*thresholdsPath) == "" {
		fmt.Fprintln(stderr, "answer eval: --corpus and --thresholds are required")
		return 2
	}
	cfg, code := scorer.config("answer eval", stderr)
	if code != 0 {
		return code
	}
	var doc answerFixtureDoc
	if code := loadYAMLInto(*corpus, &doc, "answer eval: corpus", stderr); code != 0 {
		return code
	}
	var th answer.EvalThresholds
	if code := loadYAMLInto(*thresholdsPath, &th, "answer eval: thresholds", stderr); code != 0 {
		return code
	}

	result := answer.Eval(doc.Answers, cfg, th)
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
