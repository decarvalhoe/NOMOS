package app

// portfolioCommand is `nomos portfolio` (NRT-019 #667): the portfolio status
// computed from committed machine sources. A view, not a decision.

import (
	"flag"
	"fmt"
	"io"
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
      Unavailable or stale sources are shown as such. It is a view: it lifts no claim.`

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
