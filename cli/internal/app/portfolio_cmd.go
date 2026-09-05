package app

// portfolioCommand is `nomos portfolio` (NRT-019 #667): the portfolio status
// computed from committed machine sources. A view, not a decision.

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/RBOKproject/Nomos/cli/internal/portfolio"
)

func portfolioCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, portfolioUsage)
		return 2
	}
	switch args[0] {
	case "status":
		return runPortfolioStatus(args[1:], stdout, stderr)
	case "projects":
		return runPortfolioProjects(args[1:], stdout, stderr)
	case "findings":
		return runPortfolioFindings(args[1:], stdout, stderr)
	case "reviews":
		return runPortfolioReviews(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		fmt.Fprintln(stdout, portfolioUsage)
		return 0
	default:
		fmt.Fprintf(stderr, "portfolio: unknown subcommand %q\n%s\n", args[0], portfolioUsage)
		return 2
	}
}

const portfolioUsage = `usage:
  nomos portfolio status --repo-root <dir> [--out <status.json>] [--format json|md] [--stale-after-days N] [--release-candidate <manifest.json>] [--now <RFC3339>]
      Compute the portfolio status from committed machine sources only (registry + matrix,
      roadmap lanes, ledger gaps, CAPA and review records, repeated-CI index, Praxis gate,
      competence files, domain packs, public-source snapshots, optional release candidate).
      Unavailable or stale sources are shown as such. It is a view: it lifts no claim.
  nomos portfolio projects --project <nomos.project.yaml> [--exceptions <exceptions.yaml>] ... [--verdict v]... [--stack s]... [--risk r]... [--owner o]... [--format json|md] [--now RFC3339]
      Multi-project view (NRT-022 #670) over real project and exceptions manifests: verdict counts, critical
      surfaces, stacks, owners, exceptions with expiry computed at view time. Repeat --project for each
      project; an --exceptions applies to the --project that precedes it. Neither validates nor grants anything.
  nomos portfolio findings --repo-root <dir> [--severity s]... [--status s]... [--kind k]... [--lane l]... [--format json|md] [--out f] [--now RFC3339]
      Every open finding across committed sources (ledger gaps, CAPA, audit findings, review actions, Praxis gate
      requirements, blocked public captures, wiring mismatches) plus consistency findings where two sources
      disagree (NRT-020 #668). Traceable to path + hash. Closes, waives or prioritises nothing.
  nomos portfolio reviews --repo-root <dir> [--format json|md] [--out f] [--now RFC3339]
      The periodic review records indexed: decisions, actions with due state, findings, cited artifacts and their existence.`

func runPortfolioStatus(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("portfolio status", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repoRoot := flags.String("repo-root", ".", "repository root")
	out := flags.String("out", "", "write the status here (default: stdout)")
	format := flags.String("format", "json", "json or md")
	staleDays := flags.Int("stale-after-days", portfolio.DefaultStaleAfterDays, "a dated source older than this is flagged stale")
	candidate := flags.String("release-candidate", "", "optional candidate-manifest.json (run output) to include")
	nowFlag := flags.String("now", "", "generation time (RFC3339, default now) — for reproducible runs")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	now := time.Now().UTC()
	if *nowFlag != "" {
		t, err := time.Parse(time.RFC3339, *nowFlag)
		if err != nil {
			fmt.Fprintf(stderr, "portfolio status: --now: %v\n", err)
			return 2
		}
		now = t.UTC()
	}
	st, err := portfolio.Compute(portfolio.Options{RepoRoot: *repoRoot, Now: now, StaleAfterDays: *staleDays, ReleaseCandidatePath: *candidate})
	if err != nil {
		fmt.Fprintf(stderr, "portfolio status: %v\n", err)
		return 1
	}
	var render func(io.Writer) error
	switch *format {
	case "json":
		render = func(w io.Writer) error { return portfolio.WriteJSON(w, st) }
	case "md":
		render = func(w io.Writer) error { return portfolio.WriteMarkdown(w, st) }
	default:
		fmt.Fprintf(stderr, "portfolio status: --format must be json or md\n")
		return 2
	}
	if *out == "" {
		if err := render(stdout); err != nil {
			return 1
		}
		return 0
	}
	if err := writeFile(*out, render); err != nil {
		fmt.Fprintf(stderr, "portfolio status: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "portfolio status: %d section(s) unavailable, %d stale, digest %s → %s\n", st.SectionsUnavailable, st.SectionsStale, st.StatusDigest[:19], *out)
	return 0
}

// runPortfolioProjects is `nomos portfolio projects` (NRT-022 #670).
func runPortfolioProjects(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("portfolio projects", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var projects, exceptions, verdicts, stacks, risks, owners multiFlag
	flags.Var(&projects, "project", "nomos.project.yaml path (repeatable)")
	flags.Var(&exceptions, "exceptions", "exceptions manifest for the preceding --project (repeatable)")
	flags.Var(&verdicts, "verdict", "keep projects with this scope verdict (repeatable)")
	flags.Var(&stacks, "stack", "keep projects with this stack (repeatable)")
	flags.Var(&risks, "risk", "keep projects with this risk level (repeatable)")
	flags.Var(&owners, "owner", "keep projects with this owner (repeatable, case-insensitive)")
	format := flags.String("format", "json", "json or md")
	nowFlag := flags.String("now", "", "view time (RFC3339, default now) — decides expiry")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if len(projects) == 0 {
		fmt.Fprintln(stderr, "portfolio projects: at least one --project is required")
		return 2
	}
	// Pair exceptions with projects by order of appearance on the command line.
	pairs := pairProjectsAndExceptions(args)
	now := time.Now().UTC()
	if *nowFlag != "" {
		t, err := time.Parse(time.RFC3339, *nowFlag)
		if err != nil {
			fmt.Fprintf(stderr, "portfolio projects: --now: %v\n", err)
			return 2
		}
		now = t.UTC()
	}
	view, err := portfolio.BuildProjects(pairs, now)
	if err != nil {
		fmt.Fprintf(stderr, "portfolio projects: %v\n", err)
		return 1
	}
	view = portfolio.FilterProjects(view, portfolio.ProjectFilter{Verdicts: verdicts, Stacks: stacks, RiskLevels: risks, Owners: owners})
	switch *format {
	case "json":
		if err := portfolio.WriteJSON(stdout, view); err != nil {
			return 1
		}
	case "md":
		portfolio.WriteProjectsMarkdown(stdout, view)
	default:
		fmt.Fprintln(stderr, "portfolio projects: --format must be json or md")
		return 2
	}
	return 0
}

// pairProjectsAndExceptions walks the raw args so that an --exceptions binds to
// the --project immediately before it, whatever other flags sit between.
func pairProjectsAndExceptions(args []string) []portfolio.ProjectInput {
	var out []portfolio.ProjectInput
	value := func(i int, name string) (string, int) {
		a := args[i]
		if strings.HasPrefix(a, name+"=") {
			return strings.TrimPrefix(a, name+"="), i
		}
		if i+1 < len(args) {
			return args[i+1], i + 1
		}
		return "", i
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--project" || a == "-project" || strings.HasPrefix(a, "--project=") || strings.HasPrefix(a, "-project="):
			v, j := value(i, strings.TrimLeft(strings.SplitN(a, "=", 2)[0], "-"))
			if v == "" {
				v, j = value(i, "--project")
			}
			out = append(out, portfolio.ProjectInput{ProjectPath: v})
			i = j
		case a == "--exceptions" || a == "-exceptions" || strings.HasPrefix(a, "--exceptions=") || strings.HasPrefix(a, "-exceptions="):
			v, j := value(i, strings.TrimLeft(strings.SplitN(a, "=", 2)[0], "-"))
			if v == "" {
				v, j = value(i, "--exceptions")
			}
			if len(out) > 0 {
				out[len(out)-1].ExceptionsPath = v
			}
			i = j
		}
	}
	return out
}

func parseNow(flagValue string, stderr io.Writer) (time.Time, bool) {
	if flagValue == "" {
		return time.Now().UTC(), true
	}
	t, err := time.Parse(time.RFC3339, flagValue)
	if err != nil {
		fmt.Fprintf(stderr, "--now: %v\n", err)
		return time.Time{}, false
	}
	return t.UTC(), true
}

// runPortfolioFindings is `nomos portfolio findings` (NRT-020 #668).
func runPortfolioFindings(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("portfolio findings", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repoRoot := flags.String("repo-root", ".", "repository root")
	var severities, statuses, kinds, lanes multiFlag
	flags.Var(&severities, "severity", "keep findings with this severity (repeatable)")
	flags.Var(&statuses, "status", "keep findings with this status (repeatable)")
	flags.Var(&kinds, "kind", "keep findings of this kind (repeatable)")
	flags.Var(&lanes, "lane", "keep findings of this lane (repeatable)")
	format := flags.String("format", "json", "json or md")
	out := flags.String("out", "", "write here (default: stdout)")
	nowFlag := flags.String("now", "", "evaluation time (RFC3339, default now)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	now, ok := parseNow(*nowFlag, stderr)
	if !ok {
		return 2
	}
	rep, err := portfolio.CollectFindings(*repoRoot, now)
	if err != nil {
		fmt.Fprintf(stderr, "portfolio findings: %v\n", err)
		return 1
	}
	rep = portfolio.FilterFindings(rep, portfolio.FindingsFilter{Severities: severities, Statuses: statuses, Kinds: kinds, Lanes: lanes})
	var render func(io.Writer) error
	switch *format {
	case "json":
		render = func(w io.Writer) error { return portfolio.WriteJSON(w, rep) }
	case "md":
		render = func(w io.Writer) error { portfolio.WriteFindingsMarkdown(w, rep); return nil }
	default:
		fmt.Fprintln(stderr, "portfolio findings: --format must be json or md")
		return 2
	}
	if *out == "" {
		if err := render(stdout); err != nil {
			return 1
		}
		return 0
	}
	if err := writeFile(*out, render); err != nil {
		fmt.Fprintf(stderr, "portfolio findings: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "portfolio findings: %d finding(s), %d consistency, %d source(s) unavailable → %s\n", rep.Total, rep.Consistency, len(rep.Unavailable), *out)
	return 0
}

// runPortfolioReviews is `nomos portfolio reviews` (NRT-020 #668).
func runPortfolioReviews(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("portfolio reviews", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repoRoot := flags.String("repo-root", ".", "repository root")
	format := flags.String("format", "json", "json or md")
	out := flags.String("out", "", "write here (default: stdout)")
	nowFlag := flags.String("now", "", "evaluation time (RFC3339, default now)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	now, ok := parseNow(*nowFlag, stderr)
	if !ok {
		return 2
	}
	rep, err := portfolio.IndexReviews(*repoRoot, now)
	if err != nil {
		fmt.Fprintf(stderr, "portfolio reviews: %v\n", err)
		return 1
	}
	var render func(io.Writer) error
	switch *format {
	case "json":
		render = func(w io.Writer) error { return portfolio.WriteJSON(w, rep) }
	case "md":
		render = func(w io.Writer) error { portfolio.WriteReviewsMarkdown(w, rep); return nil }
	default:
		fmt.Fprintln(stderr, "portfolio reviews: --format must be json or md")
		return 2
	}
	if *out == "" {
		if err := render(stdout); err != nil {
			return 1
		}
		return 0
	}
	if err := writeFile(*out, render); err != nil {
		fmt.Fprintf(stderr, "portfolio reviews: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "portfolio reviews: %d record(s), %d overdue action(s) → %s\n", rep.Total, rep.OverdueActions, *out)
	return 0
}
