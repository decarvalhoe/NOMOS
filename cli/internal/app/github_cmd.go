package app

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/RBOKproject/Nomos/cli/internal/githubworkflow"
)

// githubCommand dispatches the `nomos github` command group introduced by
// NGW-03 (#388). The only subcommand today is `plan`. Adding a subcommand
// is a one-line entry in the switch below; the design treats the group
// as a stable surface for the remaining NGW tickets (#389..#395).
func githubCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "nomos github: subcommand required (try `nomos github plan`)")
		printGithubUsage(stderr)
		return 2
	}
	switch args[0] {
	case "help", "-h", "--help":
		printGithubUsage(stdout)
		return 0
	case "plan":
		return githubPlanCommand(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "nomos github: unknown subcommand: %s\n", args[0])
		printGithubUsage(stderr)
		return 2
	}
}

func printGithubUsage(w io.Writer) {
	fmt.Fprintln(w, "nomos github")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  nomos github plan --config .nomos/corpus-workflows.yaml \\")
	fmt.Fprintln(w, "                    --changed-paths changed-paths.txt \\")
	fmt.Fprintln(w, "                    --out nomos-diff.json [--format json|text] [--frozen-time TIMESTAMP]")
}

func githubPlanCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("github plan", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "path to .nomos/corpus-workflows.yaml (required)")
	changedPathsFile := flags.String("changed-paths", "", "path to a newline-delimited list of changed paths (required)")
	outPath := flags.String("out", "", "path to write the nomos-diff.json artifact (required)")
	format := flags.String("format", "json", "output format: json|text")
	frozenTime := flags.String("frozen-time", "", "override generated_at with this RFC3339 timestamp (test-only)")

	if err := flags.Parse(args); err != nil {
		return 1
	}
	if strings.TrimSpace(*configPath) == "" ||
		strings.TrimSpace(*changedPathsFile) == "" ||
		strings.TrimSpace(*outPath) == "" {
		fmt.Fprintln(stderr, "nomos github plan: --config, --changed-paths, and --out are required")
		return 1
	}
	if *format != "json" && *format != "text" {
		fmt.Fprintf(stderr, "nomos github plan: unsupported --format %q (expected json or text)\n", *format)
		return 1
	}

	cfg, findings, err := githubworkflow.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "nomos github plan: %v\n", err)
		return 1
	}
	for _, f := range findings {
		fmt.Fprintf(stderr, "nomos github plan: warning [%s] workflow=%s: %s\n", f.Code, f.WorkflowID, f.Message)
	}

	changedPaths, err := readChangedPaths(*changedPathsFile)
	if err != nil {
		fmt.Fprintf(stderr, "nomos github plan: read changed-paths %s: %v\n", *changedPathsFile, err)
		return 1
	}

	now, err := resolvePlanTimestamp(*frozenTime)
	if err != nil {
		fmt.Fprintf(stderr, "nomos github plan: invalid --frozen-time %q: %v\n", *frozenTime, err)
		return 1
	}

	plan := githubworkflow.PlanScopedDiff(cfg, changedPaths)
	plan.ConfigPath = *configPath
	plan.GeneratedAt = now.UTC().Format(time.RFC3339)

	if err := writePlanArtifact(*outPath, *format, plan); err != nil {
		fmt.Fprintf(stderr, "nomos github plan: write %s: %v\n", *outPath, err)
		return 1
	}
	return 0
}

// readChangedPaths reads the changed-paths file: one path per line, blank
// lines skipped, '#'-prefixed lines treated as comments. Whitespace is
// trimmed.
func readChangedPaths(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func resolvePlanTimestamp(frozen string) (time.Time, error) {
	if strings.TrimSpace(frozen) == "" {
		return time.Now().UTC(), nil
	}
	return time.Parse(time.RFC3339, frozen)
}

func writePlanArtifact(path, format string, plan githubworkflow.DiffPlan) error {
	switch format {
	case "json":
		data, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(path, append(data, '\n'), 0o644)
	case "text":
		return os.WriteFile(path, []byte(renderPlanText(plan)), 0o644)
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

// renderPlanText is a deterministic plain-text rendering for human review.
// It is not the canonical artifact (the JSON is) — this is a convenience.
func renderPlanText(plan githubworkflow.DiffPlan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "ngw-plan %s\n", plan.SchemaVersion)
	fmt.Fprintf(&b, "generated_at: %s\n", plan.GeneratedAt)
	fmt.Fprintf(&b, "config_path: %s\n", plan.ConfigPath)
	fmt.Fprintf(&b, "changed_path_count: %d\n", plan.ChangedPathCount)
	fmt.Fprintf(&b, "ignored_generated_paths: %d\n", len(plan.IgnoredGeneratedPaths))
	fmt.Fprintf(&b, "impacted: %d\n", len(plan.Impacted))
	for _, w := range plan.Impacted {
		fmt.Fprintf(&b, "  - %s (%d matched)\n", w.WorkflowID, len(w.MatchedPaths))
	}
	fmt.Fprintf(&b, "skipped: %d\n", len(plan.Skipped))
	for _, s := range plan.Skipped {
		fmt.Fprintf(&b, "  - %s [%s]\n", s.WorkflowID, s.Reason)
	}
	return b.String()
}
