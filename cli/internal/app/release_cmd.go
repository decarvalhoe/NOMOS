package app

// releaseCommand is `nomos release`: assemble and verify a release CANDIDATE
// bundle (#639). It never approves, tags or publishes anything — those are
// human acts under the release SOP (#561). A refused candidate writes nothing.

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/RBOKproject/Nomos/cli/internal/compliance"
)

func releaseCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, releaseUsage)
		return 2
	}
	switch args[0] {
	case "candidate":
		return runReleaseCandidate(args[1:], stdout, stderr)
	case "verify":
		return runReleaseVerify(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		fmt.Fprintln(stdout, releaseUsage)
		return 0
	default:
		fmt.Fprintf(stderr, "release: unknown subcommand %q\n%s\n", args[0], releaseUsage)
		return 2
	}
}

const releaseUsage = `usage:
  nomos release candidate --spec <candidate.yaml> --gates <gates.json> --commit <sha> --repo-root <dir> --out <dir>
      Assemble a release CANDIDATE: artifacts (present AND readable), gates
      (green on the same commit), open ledger gaps (all acknowledged), risks
      (read from their measured source), waivers and deviations. Writes
      <out>/candidate-manifest.json and <out>/<version>-candidate.zip, or
      refuses and writes nothing. approval_status is always pending.
  nomos release verify --bundle <zip> [--repo-root <dir>]
      Re-verify a candidate bundle offline: manifest invariants, gates digest,
      every evidence entry's bytes against the manifest hashes, and (with
      --repo-root) the files in the tree. Any change is a refusal.`

func runReleaseCandidate(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("release candidate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	spec := flags.String("spec", "", "candidate spec YAML (required)")
	gates := flags.String("gates", "", "gate evidence JSON from scripts/release_candidate_gates.py (required)")
	commit := flags.String("commit", "", "commit the candidate is bound to (required)")
	repoRoot := flags.String("repo-root", ".", "repository root the artifact paths resolve against")
	out := flags.String("out", "", "output directory (required)")
	generatedBy := flags.String("generated-by", "nomos release candidate "+Version, "generator identity recorded in the manifest")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *spec == "" || *gates == "" || *commit == "" || *out == "" {
		fmt.Fprintln(stderr, "release candidate: --spec, --gates, --commit and --out are required")
		return 2
	}
	manifest, err := compliance.AssembleCandidate(compliance.CandidateInput{
		SpecPath: *spec, GatesPath: *gates, RepoRoot: *repoRoot, Commit: *commit, GeneratedBy: *generatedBy,
	})
	if err != nil {
		fmt.Fprintf(stderr, "release candidate: REFUSED, nothing written — %v\n", err)
		return 1
	}
	gatesRaw, err := os.ReadFile(*gates)
	if err != nil {
		fmt.Fprintf(stderr, "release candidate: %v\n", err)
		return 1
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintf(stderr, "release candidate: %v\n", err)
		return 1
	}
	zipPath := filepath.Join(*out, manifest.Version+"-candidate.zip")
	if err := compliance.WriteCandidateZip(manifest, gatesRaw, *repoRoot, zipPath); err != nil {
		fmt.Fprintf(stderr, "release candidate: write bundle: %v\n", err)
		return 1
	}
	manifestPath := filepath.Join(*out, compliance.CandidateManifestName)
	if err := writeFile(manifestPath, func(w io.Writer) error {
		return compliance.MarshalCandidateManifest(w, manifest)
	}); err != nil {
		fmt.Fprintf(stderr, "release candidate: %v\n", err)
		return 1
	}
	// The bundle we just wrote must verify — otherwise we wrote something we cannot stand behind.
	if _, err := compliance.VerifyCandidateZip(zipPath); err != nil {
		os.Remove(zipPath)
		os.Remove(manifestPath)
		fmt.Fprintf(stderr, "release candidate: bundle failed self-verification, removed — %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "release candidate: %s %s bound to %s — approval_status=%s, %d artifact(s), %d gate(s) green, %d open gap(s), %d risk(s) recorded → %s\n",
		manifest.Product, manifest.Version, shortCommit(manifest.Commit), manifest.ApprovalStatus,
		len(manifest.Artifacts), len(manifest.Gates), len(manifest.GapsOpen), len(manifest.Risks), zipPath)
	return 0
}

func runReleaseVerify(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("release verify", flag.ContinueOnError)
	flags.SetOutput(stderr)
	bundle := flags.String("bundle", "", "candidate bundle zip (required)")
	repoRoot := flags.String("repo-root", "", "also verify artifact hashes against this tree")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *bundle == "" {
		fmt.Fprintln(stderr, "release verify: --bundle is required")
		return 2
	}
	manifest, err := compliance.VerifyCandidateZip(*bundle)
	if err != nil {
		fmt.Fprintf(stderr, "release verify: REFUSED — %v\n", err)
		return 1
	}
	if *repoRoot != "" {
		if err := compliance.VerifyCandidateManifest(manifest, *repoRoot); err != nil {
			fmt.Fprintf(stderr, "release verify: REFUSED against %s — %v\n", *repoRoot, err)
			return 1
		}
	}
	fmt.Fprintf(stdout, "release verify: OK — %s %s @ %s, approval_status=%s, release_executed=%v, %d artifact(s) intact, gates: %s\n",
		manifest.Product, manifest.Version, shortCommit(manifest.Commit), manifest.ApprovalStatus, manifest.ReleaseExecuted,
		len(manifest.Artifacts), gateSummary(manifest))
	return 0
}

func gateSummary(m compliance.CandidateManifest) string {
	ids := make([]string, 0, len(m.Gates))
	for _, g := range m.Gates {
		ids = append(ids, g.ID)
	}
	return strings.Join(ids, ",")
}

func shortCommit(c string) string {
	if len(c) > 12 {
		return c[:12]
	}
	return c
}
