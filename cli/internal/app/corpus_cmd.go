package app

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/RBOKproject/Nomos/cli/internal/corpus"
	"github.com/RBOKproject/Nomos/cli/internal/guard"
)

type listFlag []string

func (f *listFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *listFlag) Set(value string) error {
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			*f = append(*f, part)
		}
	}
	return nil
}

func corpusCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		return corpusHelp(stdout)
	}

	switch args[0] {
	case "help", "-h", "--help":
		return corpusHelp(stdout)
	case "scan":
		return corpusScanCommand(args[1:], stdout, stderr)
	case "manifest":
		return corpusManifestCommand(args[1:], stdout, stderr)
	case "validate-sidecar":
		return corpusValidateSidecarCommand(args[1:], stdout, stderr)
	case "diff":
		return corpusDiffCommand(args[1:], stdout, stderr)
	case "feed":
		return corpusFeedDispatch(args[1:], stdout, stderr)
	case "body-ledger":
		return corpusBodyLedgerCommand(args[1:], stdout, stderr)
	case "attest":
		return corpusAttestCommand(args[1:], stdout, stderr)
	case "diagnose":
		return corpusDiagnoseCommand(args[1:], stdout, stderr)
	case "profiles":
		return corpusProfilesCommand(args[1:], stdout, stderr)
	case "snapshot":
		return corpusSnapshotCommand(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown corpus command %q\n\n", args[0])
		corpusHelp(stderr)
		return 2
	}
}

func corpusHelp(stdout io.Writer) int {
	fmt.Fprintln(stdout, "nomos corpus")
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "Usage:")
	fmt.Fprintln(stdout, "  nomos corpus scan --root <corpus> --out <snapshot.json> [--format json|csv|markdown] [--ext .md] [--allow 01_rbok/**] [--ignore 99_archive/**]")
	fmt.Fprintln(stdout, "  nomos corpus manifest --snapshot <snapshot.json> --out <source-manifest.yaml>")
	fmt.Fprintln(stdout, "  nomos corpus validate-sidecar --root <corpus> --manifest <source-manifest.yaml>")
	fmt.Fprintln(stdout, "  nomos corpus diff --old <snapshot.json> --new <snapshot.json>")
	fmt.Fprintln(stdout, "  nomos corpus feed --root <corpus> --snapshot <snapshot.json> --manifest <source-manifest.yaml> [--matrix <canonical-matrix.yaml>] [--lockfile <corpus.lock.json>]")
	fmt.Fprintln(stdout, "  nomos corpus body-ledger --root <corpus> --manifest <source-manifest.yaml> --out <body-ledger.json>")
	fmt.Fprintln(stdout, "  nomos corpus body-ledger --verify <body-ledger.json>")
	fmt.Fprintln(stdout, "  nomos corpus attest --snapshot <snapshot.json> --corpus-id <id> --project-id <id>")
	fmt.Fprintln(stdout, "  nomos corpus feed --profile rbok-lawbook --root <corpus> [--outputs index,governance] [--format json|text]")
	fmt.Fprintln(stdout, "  nomos corpus diagnose --profile rbok-lawbook --root <corpus> [--format json|text]")
	fmt.Fprintln(stdout, "  nomos corpus profiles")
	fmt.Fprintln(stdout, "  nomos corpus snapshot verify --envelope <snapshot.json> [--records <sources.jsonl>] [--out <verification.json>]")
	fmt.Fprintln(stdout, "  nomos corpus snapshot import --envelope <snapshot.json> [--records <sources.jsonl>] --out <source-manifest.yaml> [--domain d --owner o --license l --confidentiality c]")
	fmt.Fprintln(stdout, "  nomos corpus snapshot seal --records <sources.jsonl> --snapshot-id <id> --producer <name/version> [--db-schema <v>] --out <snapshot.json>")
	return 0
}

// --- snapshot: external immutable snapshots (#611) ---------------------------

func corpusSnapshotCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: nomos corpus snapshot verify|import|seal ...")
		return 2
	}
	switch args[0] {
	case "verify":
		return corpusSnapshotVerify(args[1:], stdout, stderr)
	case "import":
		return corpusSnapshotImport(args[1:], stdout, stderr)
	case "seal":
		return corpusSnapshotSeal(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown snapshot command %q (verify|import|seal)\n", args[0])
		return 2
	}
}

// corpusSnapshotVerify recomputes the root and fails closed. The verification
// is written even on failure, with the problem named: evidence of a refused
// snapshot is still evidence.
func corpusSnapshotVerify(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("corpus snapshot verify", flag.ContinueOnError)
	flags.SetOutput(stderr)
	envelope := flags.String("envelope", "", "snapshot envelope JSON (required)")
	records := flags.String("records", "", "records JSONL (default: envelope's records_file or sources.jsonl beside it)")
	out := flags.String("out", "", "write the verification JSON here (default: stdout)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *envelope == "" {
		fmt.Fprintln(stderr, "snapshot verify: --envelope is required")
		return 2
	}
	env, recs, err := corpus.LoadExternalSnapshot(*envelope, *records)
	if err != nil {
		fmt.Fprintf(stderr, "snapshot verify: %v\n", err)
		return 1
	}
	verification, verr := corpus.VerifyExternalSnapshot(env, recs)
	verification.Proofs = nil // proofs are for the ledger path, not the human verdict
	if err := writeSnapshotJSON(*out, stdout, verification); err != nil {
		fmt.Fprintf(stderr, "snapshot verify: %v\n", err)
		return 1
	}
	if verr != nil {
		fmt.Fprintf(stderr, "snapshot verify: REFUSED — %v\n", verr)
		return 1
	}
	return 0
}

// corpusSnapshotImport verifies FIRST and imports only a passing snapshot.
// A snapshot that does not verify produces no manifest at all.
func corpusSnapshotImport(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("corpus snapshot import", flag.ContinueOnError)
	flags.SetOutput(stderr)
	envelope := flags.String("envelope", "", "snapshot envelope JSON (required)")
	records := flags.String("records", "", "records JSONL (default: beside the envelope)")
	out := flags.String("out", "", "source-manifest YAML to write (required)")
	domain := flags.String("domain", "external-snapshot", "manifest domain")
	owner := flags.String("owner", "not_assigned", "manifest owner")
	license := flags.String("license", "unknown", "manifest license")
	confidentiality := flags.String("confidentiality", "internal", "manifest confidentiality")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *envelope == "" || *out == "" {
		fmt.Fprintln(stderr, "snapshot import: --envelope and --out are required")
		return 2
	}
	env, recs, err := corpus.LoadExternalSnapshot(*envelope, *records)
	if err != nil {
		fmt.Fprintf(stderr, "snapshot import: %v\n", err)
		return 1
	}
	if _, verr := corpus.VerifyExternalSnapshot(env, recs); verr != nil {
		fmt.Fprintf(stderr, "snapshot import: REFUSED, nothing written — %v\n", verr)
		return 1
	}
	manifest := corpus.ImportSnapshotToManifest(env, recs, corpus.SnapshotImportOptions{
		Domain: *domain, Owner: *owner, License: *license, Confidentiality: *confidentiality,
	})
	f, err := os.Create(*out)
	if err != nil {
		fmt.Fprintf(stderr, "snapshot import: %v\n", err)
		return 1
	}
	defer f.Close()
	if err := corpus.WriteManifestYAML(f, manifest); err != nil {
		fmt.Fprintf(stderr, "snapshot import: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "snapshot import: %d source(s) written to %s from %s (root %s)\n",
		len(manifest.Sources), *out, env.SnapshotID, env.ContentHashRoot)
	return 0
}

// corpusSnapshotSeal is the PRODUCER side: compute counts and root over the
// records about to be exported and write the envelope.
func corpusSnapshotSeal(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("corpus snapshot seal", flag.ContinueOnError)
	flags.SetOutput(stderr)
	records := flags.String("records", "", "records JSONL (required)")
	snapshotID := flags.String("snapshot-id", "", "snapshot id (required)")
	producer := flags.String("producer", "", "producer name/version (required)")
	dbSchema := flags.String("db-schema", "", "producer schema version")
	generatedAt := flags.String("generated-at", "", "RFC 3339 (default: now, UTC)")
	out := flags.String("out", "", "envelope JSON to write (required)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *records == "" || *snapshotID == "" || *producer == "" || *out == "" {
		fmt.Fprintln(stderr, "snapshot seal: --records, --snapshot-id, --producer and --out are required")
		return 2
	}
	f, err := os.Open(*records)
	if err != nil {
		fmt.Fprintf(stderr, "snapshot seal: %v\n", err)
		return 1
	}
	recs, err := corpus.ReadSnapshotRecords(f)
	f.Close()
	if err != nil {
		fmt.Fprintf(stderr, "snapshot seal: %v\n", err)
		return 1
	}
	when := *generatedAt
	if when == "" {
		when = time.Now().UTC().Format(time.RFC3339)
	}
	env, err := corpus.SealExternalSnapshot(*snapshotID, *producer, *dbSchema, when, filepath.Base(*records), recs)
	if err != nil {
		fmt.Fprintf(stderr, "snapshot seal: REFUSED — %v\n", err)
		return 1
	}
	if err := writeSnapshotJSON(*out, stdout, env); err != nil {
		fmt.Fprintf(stderr, "snapshot seal: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "snapshot seal: %s sealed — %d source(s), %d version(s), root %s\n",
		env.SnapshotID, env.SourceCount, env.VersionCount, env.ContentHashRoot)
	return 0
}

func writeSnapshotJSON(path string, stdout io.Writer, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if path == "" {
		_, err = stdout.Write(data)
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func corpusScanCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("corpus scan", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "source corpus root")
	out := flags.String("out", "", "output path outside the source corpus root")
	flags.StringVar(out, "output", "", "output path outside the source corpus root")
	format := flags.String("format", "json", "output format: json, csv, or markdown")
	var exts listFlag
	var allow listFlag
	var ignore listFlag
	flags.Var(&exts, "ext", "extension to include; may be repeated or comma-separated")
	flags.Var(&allow, "allow", "allow glob for corpus paths; may be repeated")
	flags.Var(&ignore, "ignore", "ignore glob for corpus paths; may be repeated")

	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 0 {
		if *root != "." {
			fmt.Fprintln(stderr, "use either --root or a positional root, not both")
			return 2
		}
		*root = flags.Arg(0)
	}

	if err := validateOutputPath(*root, *out); err != nil {
		fmt.Fprintf(stderr, "corpus scan: %v\n", err)
		return 2
	}
	before, hadSnapshot, err := readOnlyBefore(*root)
	if err != nil {
		fmt.Fprintf(stderr, "corpus scan: %v\n", err)
		return 2
	}

	policy := scanPolicy(allow, ignore)
	snapshot, err := corpus.Scan(*root, corpus.ScanOptions{Extensions: []string(exts), Policy: policy})
	if err != nil {
		fmt.Fprintf(stderr, "corpus scan: %v\n", err)
		return 1
	}
	enrichSnapshotGit(&snapshot, *root)

	if err := writeSnapshot(snapshot, strings.ToLower(*format), *out, stdout); err != nil {
		fmt.Fprintf(stderr, "corpus scan: %v\n", err)
		return 1
	}
	if hadSnapshot {
		if err := guard.GuardReadOnly(before); err != nil {
			fmt.Fprintf(stderr, "corpus scan: %v\n", err)
			return 1
		}
	}

	if *out != "" {
		fmt.Fprintf(stdout, "scanned %d files into %s\n", snapshot.TotalFiles, filepath.Clean(*out))
	}
	return 0
}

func scanPolicy(allow listFlag, ignore listFlag) *corpus.Policy {
	if len(allow) == 0 && len(ignore) == 0 {
		return nil
	}
	policy := corpus.DefaultPolicy()
	if len(allow) > 0 {
		policy.Allow = []string(allow)
	}
	if len(ignore) > 0 {
		policy.Ignore = append(policy.Ignore, []string(ignore)...)
	}
	return &policy
}

func corpusManifestCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("corpus manifest", flag.ContinueOnError)
	flags.SetOutput(stderr)
	snapshotPath := flags.String("snapshot", "", "snapshot JSON path")
	out := flags.String("out", "", "manifest output path")
	flags.StringVar(out, "output", "", "manifest output path")
	domain := flags.String("domain", "", "source domain")
	owner := flags.String("owner", "", "source owner")
	confidentiality := flags.String("confidentiality", "internal", "source confidentiality")
	idPrefix := flags.String("id-prefix", "CORPUS", "source ID prefix")
	license := flags.String("license", "internal", "source license")

	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *snapshotPath == "" {
		fmt.Fprintln(stderr, "corpus manifest: --snapshot is required")
		return 2
	}

	snapshot, err := readSnapshot(*snapshotPath)
	if err != nil {
		fmt.Fprintf(stderr, "corpus manifest: %v\n", err)
		return 1
	}
	if err := validateOutputPath(snapshot.CorpusRoot, *out); err != nil {
		fmt.Fprintf(stderr, "corpus manifest: %v\n", err)
		return 2
	}
	before, hadSnapshot, err := readOnlyBefore(snapshot.CorpusRoot)
	if err != nil {
		fmt.Fprintf(stderr, "corpus manifest: %v\n", err)
		return 2
	}

	manifest := corpus.GenerateManifest(snapshot, corpus.ManifestOptions{
		Domain:          *domain,
		Owner:           *owner,
		Confidentiality: *confidentiality,
		IDPrefix:        *idPrefix,
		License:         *license,
	})
	if *out == "" {
		if err := corpus.WriteManifestYAML(stdout, manifest); err != nil {
			fmt.Fprintf(stderr, "corpus manifest: %v\n", err)
			return 1
		}
		if hadSnapshot {
			if err := guard.GuardReadOnly(before); err != nil {
				fmt.Fprintf(stderr, "corpus manifest: %v\n", err)
				return 1
			}
		}
		return 0
	}
	if err := writeFile(*out, func(w io.Writer) error { return corpus.WriteManifestYAML(w, manifest) }); err != nil {
		fmt.Fprintf(stderr, "corpus manifest: %v\n", err)
		return 1
	}
	if hadSnapshot {
		if err := guard.GuardReadOnly(before); err != nil {
			fmt.Fprintf(stderr, "corpus manifest: %v\n", err)
			return 1
		}
	}
	fmt.Fprintf(stdout, "generated manifest with %d sources into %s\n", len(manifest.Sources), filepath.Clean(*out))
	return 0
}

func corpusValidateSidecarCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("corpus validate-sidecar", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "source corpus root")
	manifestPath := flags.String("manifest", "", "source manifest path")
	format := flags.String("format", "json", "output format: json or markdown")
	var allow listFlag
	var ignore listFlag
	var exts listFlag
	flags.Var(&allow, "allow", "allow glob for corpus paths; may be repeated")
	flags.Var(&ignore, "ignore", "ignore glob for corpus paths; may be repeated")
	flags.Var(&exts, "ext", "extension to validate as in-scope; may be repeated")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *manifestPath == "" {
		fmt.Fprintln(stderr, "corpus validate-sidecar: --manifest is required")
		return 2
	}
	before, hadSnapshot, err := readOnlyBefore(*root)
	if err != nil {
		fmt.Fprintf(stderr, "corpus validate-sidecar: %v\n", err)
		return 2
	}

	manifest, err := corpus.ParseSidecarManifest(*manifestPath)
	if err != nil {
		fmt.Fprintf(stderr, "corpus validate-sidecar: %v\n", err)
		return 1
	}
	result := corpus.ValidateSidecarWithOptions(manifest, *root, scanPolicy(allow, ignore), []string(exts))
	switch strings.ToLower(*format) {
	case "json":
		if err := writeJSONErr(stdout, result); err != nil {
			fmt.Fprintf(stderr, "corpus validate-sidecar: %v\n", err)
			return 1
		}
	case "markdown", "md":
		fmt.Fprintf(stdout, "# Sidecar Validation\n\nValid: %t\nSources: %d\n", result.Valid, result.SourceCount)
		for _, item := range result.Errors {
			fmt.Fprintf(stdout, "- %s %s: %s\n", item.Code, item.SourceID, item.Message)
		}
	default:
		fmt.Fprintf(stderr, "corpus validate-sidecar: unknown format %q\n", *format)
		return 2
	}
	if !result.Valid {
		return 1
	}
	if hadSnapshot {
		if err := guard.GuardReadOnly(before); err != nil {
			fmt.Fprintf(stderr, "corpus validate-sidecar: %v\n", err)
			return 1
		}
	}
	return 0
}

func corpusDiffCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("corpus diff", flag.ContinueOnError)
	flags.SetOutput(stderr)
	oldPath := flags.String("old", "", "old snapshot JSON path")
	newPath := flags.String("new", "", "new snapshot JSON path")
	out := flags.String("out", "", "diff output path")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *oldPath == "" || *newPath == "" {
		fmt.Fprintln(stderr, "corpus diff: --old and --new are required")
		return 2
	}
	oldSnapshot, err := readSnapshot(*oldPath)
	if err != nil {
		fmt.Fprintf(stderr, "corpus diff: %v\n", err)
		return 1
	}
	newSnapshot, err := readSnapshot(*newPath)
	if err != nil {
		fmt.Fprintf(stderr, "corpus diff: %v\n", err)
		return 1
	}
	report := corpus.Diff(oldSnapshot, newSnapshot)
	if err := validateOutputPath(oldSnapshot.CorpusRoot, *out); err != nil {
		fmt.Fprintf(stderr, "corpus diff: %v\n", err)
		return 2
	}
	if err := validateOutputPath(newSnapshot.CorpusRoot, *out); err != nil {
		fmt.Fprintf(stderr, "corpus diff: %v\n", err)
		return 2
	}
	return writeJSONPath(report, *out, stdout, stderr, "corpus diff")
}

// corpusFeedDispatch routes to the profile-aware feed or the generic feed.
func corpusFeedDispatch(args []string, stdout io.Writer, stderr io.Writer) int {
	for _, arg := range args {
		if arg == "--profile" || strings.HasPrefix(arg, "--profile=") {
			return corpusProfileFeedCommand(args, stdout, stderr)
		}
	}
	return corpusFeedCommand(args, stdout, stderr)
}

func corpusFeedCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("corpus feed", flag.ContinueOnError)
	flags.SetOutput(stderr)
	matrixPath := flags.String("matrix", "", "canonical matrix YAML path")
	manifestPath := flags.String("manifest", "", "source manifest YAML path")
	root := flags.String("root", "", "source corpus root for extraction and read-only guard")
	snapshotPath := flags.String("snapshot", "", "snapshot JSON path")
	lockfilePath := flags.String("lockfile", "", "accepted corpus lockfile JSON path")
	corpusID := flags.String("corpus-id", "", "corpus identifier for embedded attestation")
	projectID := flags.String("project-id", "", "consumer project identifier for embedded attestation")
	out := flags.String("out", "", "feed output path")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *manifestPath == "" {
		fmt.Fprintln(stderr, "corpus feed: --manifest is required")
		return 2
	}
	if *matrixPath == "" && *root == "" {
		fmt.Fprintln(stderr, "corpus feed: either --matrix or --root is required")
		return 2
	}
	if err := validateOutputPath(*root, *out); err != nil {
		fmt.Fprintf(stderr, "corpus feed: %v\n", err)
		return 2
	}
	before, hadSnapshot, err := readOnlyBefore(*root)
	if err != nil {
		fmt.Fprintf(stderr, "corpus feed: %v\n", err)
		return 2
	}

	var matrixYAML []byte
	if *matrixPath != "" {
		matrixYAML, err = os.ReadFile(*matrixPath)
		if err != nil {
			fmt.Fprintf(stderr, "corpus feed: read matrix: %v\n", err)
			return 1
		}
	}
	manifestYAML, err := os.ReadFile(*manifestPath)
	if err != nil {
		fmt.Fprintf(stderr, "corpus feed: read manifest: %v\n", err)
		return 1
	}
	var snapshot *corpus.Snapshot
	if *snapshotPath != "" {
		loaded, err := readSnapshot(*snapshotPath)
		if err != nil {
			fmt.Fprintf(stderr, "corpus feed: %v\n", err)
			return 1
		}
		snapshot = &loaded
	}
	var lockfile *corpus.Lockfile
	if *lockfilePath != "" {
		loaded, err := corpus.ReadLockfile(*lockfilePath)
		if err != nil {
			fmt.Fprintf(stderr, "corpus feed: read lockfile: %v\n", err)
			return 1
		}
		lockfile = loaded
	}
	feed, err := corpus.GenerateFeed(corpus.FeedInput{
		MatrixYAML:     matrixYAML,
		ManifestYAML:   manifestYAML,
		Root:           *root,
		Snapshot:       snapshot,
		Lockfile:       lockfile,
		CorpusID:       *corpusID,
		ProjectID:      *projectID,
		ScannerVersion: Version,
		GeneratedAt:    time.Now().UTC(),
	})
	if err != nil {
		fmt.Fprintf(stderr, "corpus feed: %v\n", err)
		return 1
	}
	code := writeJSONPath(feed, *out, stdout, stderr, "corpus feed")
	if code != 0 {
		return code
	}
	if hadSnapshot {
		if err := guard.GuardReadOnly(before); err != nil {
			fmt.Fprintf(stderr, "corpus feed: %v\n", err)
			return 1
		}
	}
	return 0
}

func corpusBodyLedgerCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("corpus body-ledger", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "source corpus root")
	manifestPath := flags.String("manifest", "", "source manifest YAML path")
	out := flags.String("out", "", "body ledger output path outside the source corpus root")
	flags.StringVar(out, "output", "", "body ledger output path outside the source corpus root")
	verifyPath := flags.String("verify", "", "verify the Merkle inclusion proofs of an existing body ledger JSON and exit")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *verifyPath != "" {
		// VRC-07 (#553): verification mode — recompute every leaf from the
		// ledger rows and walk each inclusion proof to the recorded root.
		ledger, err := readCorpusBodyLedger(*verifyPath)
		if err != nil {
			fmt.Fprintf(stderr, "corpus body-ledger: %v\n", err)
			return 1
		}
		if err := corpus.VerifyCorpusBodyLedgerProofs(ledger); err != nil {
			fmt.Fprintf(stderr, "corpus body-ledger: verification FAILED: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "body ledger merkle verification: ok (%d leaves, root %s)\n",
			ledger.Merkle.LeafCount, ledger.Merkle.Root)
		return 0
	}
	if *root == "" || *manifestPath == "" {
		fmt.Fprintln(stderr, "corpus body-ledger: --root and --manifest are required")
		return 2
	}
	if err := validateOutputPath(*root, *out); err != nil {
		fmt.Fprintf(stderr, "corpus body-ledger: %v\n", err)
		return 2
	}
	before, hadSnapshot, err := readOnlyBefore(*root)
	if err != nil {
		fmt.Fprintf(stderr, "corpus body-ledger: %v\n", err)
		return 2
	}

	manifest, err := corpus.ParseSidecarManifest(*manifestPath)
	if err != nil {
		fmt.Fprintf(stderr, "corpus body-ledger: %v\n", err)
		return 1
	}
	ledger, err := buildCorpusBodyLedgerFromManifest(*root, manifest)
	if err != nil {
		fmt.Fprintf(stderr, "corpus body-ledger: %v\n", err)
		return 1
	}
	if *out == "" {
		data, err := corpus.MarshalCorpusBodyLedger(ledger)
		if err != nil {
			fmt.Fprintf(stderr, "corpus body-ledger: %v\n", err)
			return 1
		}
		if _, err := stdout.Write(append(data, '\n')); err != nil {
			fmt.Fprintf(stderr, "corpus body-ledger: %v\n", err)
			return 1
		}
	} else if err := writeFile(*out, func(w io.Writer) error {
		data, err := corpus.MarshalCorpusBodyLedger(ledger)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}); err != nil {
		fmt.Fprintf(stderr, "corpus body-ledger: %v\n", err)
		return 1
	}
	if hadSnapshot {
		if err := guard.GuardReadOnly(before); err != nil {
			fmt.Fprintf(stderr, "corpus body-ledger: %v\n", err)
			return 1
		}
	}
	if *out != "" {
		fmt.Fprintf(stdout, "wrote body ledger %s\n", filepath.Clean(*out))
	}
	return 0
}

func buildCorpusBodyLedgerFromManifest(root string, manifest corpus.SidecarManifest) (corpus.CorpusBodyLedger, error) {
	inputs := make([]corpus.BodyLedgerSourceInput, 0, len(manifest.Sources))
	for _, source := range manifest.Sources {
		src := source
		adm := src.Admission()
		corpus.BackfillAdmission(&adm, src.Path)
		src.AdmissionStatus = adm.AdmissionStatus
		src.AtomizationStatus = adm.AtomizationStatus
		src.ExclusionReason = adm.ExclusionReason
		src.SourceRole = adm.SourceRole
		src.FormatSupport = adm.FormatSupport
		src.DerivativeOf = adm.DerivativeOf
		if err := src.Validate(); err != nil {
			return corpus.CorpusBodyLedger{}, fmt.Errorf("manifest source %q: %w", src.ID, err)
		}

		absPath := filepath.Join(root, filepath.FromSlash(src.Path))
		info, err := os.Stat(absPath)
		if err != nil {
			return corpus.CorpusBodyLedger{}, fmt.Errorf("stat %s: %w", src.Path, err)
		}

		var content []byte
		var segments []corpus.SourceSegment
		if isBodyLedgerTextSource(src) {
			content, err = os.ReadFile(absPath)
			if err != nil {
				return corpus.CorpusBodyLedger{}, fmt.Errorf("read %s: %w", src.Path, err)
			}
			segments, err = corpus.ScanMarkdown(src.ID, src.Path, content)
			if err != nil {
				return corpus.CorpusBodyLedger{}, fmt.Errorf("scan %s: %w", src.Path, err)
			}
		} else if format, ok := corpus.StructuredFormatForPath(src.Path); ok {
			content, err = os.ReadFile(absPath)
			if err != nil {
				return corpus.CorpusBodyLedger{}, fmt.Errorf("read %s: %w", src.Path, err)
			}
			scan, err := corpus.ScanStructuredScalars(src, content, format)
			if err != nil {
				return corpus.CorpusBodyLedger{}, fmt.Errorf("scan structured %s: %w", src.Path, err)
			}
			segments = scan.Segments
		}
		inputs = append(inputs, corpus.BodyLedgerSourceInput{
			Source:    src,
			Content:   content,
			Segments:  segments,
			SizeBytes: info.Size(),
		})
	}
	return corpus.BuildCorpusBodyLedger(corpus.BodyLedgerInput{
		CorpusRoot: root,
		Sources:    inputs,
	})
}

func isBodyLedgerTextSource(source corpus.ManifestSource) bool {
	ext := strings.ToLower(filepath.Ext(source.Path))
	return source.Type == "markdown" || ext == ".md" || ext == ".mdx" || ext == ".txt"
}

func corpusAttestCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("corpus attest", flag.ContinueOnError)
	flags.SetOutput(stderr)
	snapshotPath := flags.String("snapshot", "", "snapshot JSON path")
	corpusID := flags.String("corpus-id", "", "corpus identifier")
	projectID := flags.String("project-id", "", "consumer project identifier")
	verdict := flags.String("verdict", "corpus_admissible", "corpus verdict")
	confidence := flags.String("confidence", "high", "corpus confidence")
	scope := flags.String("scope", "snapshot", "claim scope (snapshot, restricted_snapshot, full_profile)")
	profile := flags.String("profile", "", "diagnose profile to bind into the attestation")
	root := flags.String("root", "", "corpus root for profile diagnosis")
	out := flags.String("out", "", "attestation output path")
	bodyLedgerPath := flags.String("corpus-body-ledger", "",
		"body ledger JSON whose Merkle proofs are verified, then claim_coverage is computed from it (FSQ-05 / VRC-07 #553)")
	feedPath := flags.String("feed", "",
		"feed JSON used to count extracted units for claim_coverage (calculated, never declared)")
	extSnapshot := flags.String("external-snapshot", "",
		"external snapshot envelope (#611/#612): verified fail-closed, then its identity, root, counts and web-source coverage are bound into the attestation metadata")
	extSnapshotRecords := flags.String("external-snapshot-records", "",
		"records JSONL for --external-snapshot (default: sources.jsonl beside the envelope)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *snapshotPath == "" || *corpusID == "" || *projectID == "" {
		fmt.Fprintln(stderr, "corpus attest: --snapshot, --corpus-id, and --project-id are required")
		return 2
	}
	if (*profile == "") != (*root == "") {
		fmt.Fprintln(stderr, "corpus attest: --profile and --root must be provided together")
		return 2
	}
	snapshot, err := readSnapshot(*snapshotPath)
	if err != nil {
		fmt.Fprintf(stderr, "corpus attest: %v\n", err)
		return 1
	}
	if err := validateOutputPath(snapshot.CorpusRoot, *out); err != nil {
		fmt.Fprintf(stderr, "corpus attest: %v\n", err)
		return 2
	}
	before, hadSnapshot, err := readOnlyBefore(snapshot.CorpusRoot)
	if err != nil {
		fmt.Fprintf(stderr, "corpus attest: %v\n", err)
		return 2
	}
	files := make([]string, 0, len(snapshot.Sources))
	for _, source := range snapshot.Sources {
		files = append(files, source.Path+" "+source.Hash)
	}
	var diagnosis *corpus.DiagnoseVerdict
	if *profile != "" {
		diagnosed, err := corpus.DiagnoseProfile(*profile, *root)
		if err != nil {
			fmt.Fprintf(stderr, "corpus attest: %v\n", err)
			return 1
		}
		diagnosis = &diagnosed
		if *scope == "snapshot" {
			*scope = "full_profile"
		}
	}
	// VRC-07 (#553): a supplied body ledger is verified (Merkle inclusion
	// proofs recomputed from its rows) BEFORE its coverage feeds the
	// claim_coverage scoping — a tampered ledger must fail the attestation,
	// not decorate it.
	var bodyLedger *corpus.CorpusBodyLedger
	if *bodyLedgerPath != "" {
		ledger, err := readCorpusBodyLedger(*bodyLedgerPath)
		if err != nil {
			fmt.Fprintf(stderr, "corpus attest: %v\n", err)
			return 1
		}
		if err := corpus.VerifyCorpusBodyLedgerProofs(ledger); err != nil {
			fmt.Fprintf(stderr, "corpus attest: body ledger verification FAILED: %v\n", err)
			return 1
		}
		bodyLedger = &ledger
	}
	unitsExtracted := 0
	if *feedPath != "" {
		count, err := countFeedUnits(*feedPath)
		if err != nil {
			fmt.Fprintf(stderr, "corpus attest: %v\n", err)
			return 1
		}
		unitsExtracted = count
	}
	metadata := map[string]any{
		"commit":     snapshot.Commit,
		"branch":     snapshot.Branch,
		"repository": snapshot.Repository,
	}
	if *extSnapshot != "" {
		env, records, err := corpus.LoadExternalSnapshot(*extSnapshot, *extSnapshotRecords)
		if err != nil {
			fmt.Fprintf(stderr, "corpus attest: external snapshot: %v\n", err)
			return 1
		}
		if _, verr := corpus.VerifyExternalSnapshot(env, records); verr != nil {
			fmt.Fprintf(stderr, "corpus attest: external snapshot REFUSED, no attestation written: %v\n", verr)
			return 1
		}
		metadata["external_snapshot"] = corpus.SnapshotCoverageMetadata(env, records)
	}
	statement, err := corpus.GenerateCorpusAttestation(corpus.CorpusAttestationOptions{
		CorpusID:       *corpusID,
		ProjectID:      *projectID,
		ScannerVersion: Version,
		Scope:          *scope,
		Verdict:        *verdict,
		Confidence:     *confidence,
		FilesScanned:   snapshot.TotalFiles,
		ScannedFiles:   files,
		Diagnosis:      diagnosis,
		UnitsExtracted: unitsExtracted,
		BodyLedger:     bodyLedger,
		Now:            time.Now().UTC(),
		Metadata:       metadata,
	})
	if err != nil {
		fmt.Fprintf(stderr, "corpus attest: %v\n", err)
		return 1
	}
	if *out == "" {
		if err := corpus.WriteAttestation(stdout, statement); err != nil {
			fmt.Fprintf(stderr, "corpus attest: %v\n", err)
			return 1
		}
		if hadSnapshot {
			if err := guard.GuardReadOnly(before); err != nil {
				fmt.Fprintf(stderr, "corpus attest: %v\n", err)
				return 1
			}
		}
		return 0
	}
	if err := writeFile(*out, func(w io.Writer) error { return corpus.WriteAttestation(w, statement) }); err != nil {
		fmt.Fprintf(stderr, "corpus attest: %v\n", err)
		return 1
	}
	if hadSnapshot {
		if err := guard.GuardReadOnly(before); err != nil {
			fmt.Fprintf(stderr, "corpus attest: %v\n", err)
			return 1
		}
	}
	fmt.Fprintf(stdout, "generated corpus attestation into %s\n", filepath.Clean(*out))
	return 0
}

// readCorpusBodyLedger loads a body ledger JSON from disk (VRC-07 #553).
func readCorpusBodyLedger(path string) (corpus.CorpusBodyLedger, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return corpus.CorpusBodyLedger{}, fmt.Errorf("read body ledger: %w", err)
	}
	var ledger corpus.CorpusBodyLedger
	if err := json.Unmarshal(data, &ledger); err != nil {
		return corpus.CorpusBodyLedger{}, fmt.Errorf("decode body ledger: %w", err)
	}
	return ledger, nil
}

// countFeedUnits counts the units in a feed JSON document so that
// claim_coverage.covers_curated_feed is CALCULATED from the artifact, never
// declared by the caller (doctrine §2.3).
func countFeedUnits(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read feed: %w", err)
	}
	var probe struct {
		Units []json.RawMessage `json:"units"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return 0, fmt.Errorf("decode feed: %w", err)
	}
	return len(probe.Units), nil
}

func readOnlyBefore(root string) (guard.Snapshot, bool, error) {
	if strings.TrimSpace(root) == "" {
		return guard.Snapshot{}, false, nil
	}
	if err := guard.RequireClean(root); err != nil {
		if errors.Is(err, guard.ErrNotGitRepo) {
			return guard.Snapshot{}, false, nil
		}
		return guard.Snapshot{}, false, err
	}
	if err := guard.CheckNoPushRemote(root); err != nil {
		return guard.Snapshot{}, false, err
	}
	snapshot, err := guard.TakeSnapshot(root)
	if err != nil {
		if errors.Is(err, guard.ErrNotGitRepo) {
			return guard.Snapshot{}, false, nil
		}
		return guard.Snapshot{}, false, err
	}
	return snapshot, true, nil
}

func validateOutputPath(root string, out string) error {
	if strings.TrimSpace(out) == "" || strings.TrimSpace(root) == "" {
		return nil
	}
	return guard.CheckOutputNotInSource(root, out)
}

func writeSnapshot(snapshot corpus.Snapshot, format string, out string, stdout io.Writer) error {
	switch format {
	case "json":
		return writePathOrStdout(out, stdout, func(w io.Writer) error { return corpus.WriteJSON(w, snapshot) })
	case "csv":
		return writePathOrStdout(out, stdout, func(w io.Writer) error { return writeSnapshotCSV(w, snapshot) })
	case "markdown", "md":
		return writePathOrStdout(out, stdout, func(w io.Writer) error { return writeSnapshotMarkdown(w, snapshot) })
	default:
		return fmt.Errorf("unknown format %q", format)
	}
}

func writeSnapshotCSV(w io.Writer, snapshot corpus.Snapshot) error {
	writer := csv.NewWriter(w)
	if err := writer.Write([]string{"path", "hash", "size_bytes", "extension", "classification"}); err != nil {
		return err
	}
	for _, source := range snapshot.Sources {
		if err := writer.Write([]string{
			source.Path,
			source.Hash,
			fmt.Sprintf("%d", source.SizeBytes),
			source.Extension,
			source.Classification,
		}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func writeSnapshotMarkdown(w io.Writer, snapshot corpus.Snapshot) error {
	fmt.Fprintf(w, "# Corpus Snapshot\n\n")
	fmt.Fprintf(w, "- Repository: `%s`\n", snapshot.Repository)
	fmt.Fprintf(w, "- Branch: `%s`\n", snapshot.Branch)
	fmt.Fprintf(w, "- Commit: `%s`\n", snapshot.Commit)
	fmt.Fprintf(w, "- Total files: `%d`\n\n", snapshot.TotalFiles)
	fmt.Fprintln(w, "| Path | Hash | Size | Class |")
	fmt.Fprintln(w, "|---|---|---:|---|")
	for _, source := range snapshot.Sources {
		fmt.Fprintf(w, "| `%s` | `%s` | %d | `%s` |\n", source.Path, source.Hash, source.SizeBytes, source.Classification)
	}
	return nil
}

func readSnapshot(path string) (corpus.Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return corpus.Snapshot{}, fmt.Errorf("read snapshot: %w", err)
	}
	var snapshot corpus.Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return corpus.Snapshot{}, fmt.Errorf("parse snapshot: %w", err)
	}
	return snapshot, nil
}

func writeJSONPath(value any, out string, stdout io.Writer, stderr io.Writer, label string) int {
	if out == "" {
		if err := writeJSONErr(stdout, value); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", label, err)
			return 1
		}
		return 0
	}
	if err := writeFile(out, func(w io.Writer) error { return writeJSONErr(w, value) }); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", label, err)
		return 1
	}
	fmt.Fprintf(stdout, "wrote %s\n", filepath.Clean(out))
	return 0
}

func writeJSONErr(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func writePathOrStdout(path string, stdout io.Writer, write func(io.Writer) error) error {
	if path == "" {
		return write(stdout)
	}
	return writeFile(path, write)
}

func writeFile(path string, write func(io.Writer) error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return write(file)
}

func enrichSnapshotGit(snapshot *corpus.Snapshot, root string) {
	snapshot.Repository = gitValue(root, "config", "--get", "remote.origin.url")
	snapshot.Branch = gitValue(root, "rev-parse", "--abbrev-ref", "HEAD")
	snapshot.Commit = gitValue(root, "rev-parse", "HEAD")
}

func gitValue(root string, args ...string) string {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
