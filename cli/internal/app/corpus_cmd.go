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
		return corpusFeedCommand(args[1:], stdout, stderr)
	case "attest":
		return corpusAttestCommand(args[1:], stdout, stderr)
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
	fmt.Fprintln(stdout, "  nomos corpus feed --matrix <canonical-matrix.yaml> --manifest <source-manifest.yaml>")
	fmt.Fprintln(stdout, "  nomos corpus attest --snapshot <snapshot.json> --corpus-id <id> --project-id <id>")
	return 0
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
		return 0
	}
	if err := writeFile(*out, func(w io.Writer) error { return corpus.WriteManifestYAML(w, manifest) }); err != nil {
		fmt.Fprintf(stderr, "corpus manifest: %v\n", err)
		return 1
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
	return writeJSONPath(report, *out, stdout, stderr, "corpus diff")
}

func corpusFeedCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("corpus feed", flag.ContinueOnError)
	flags.SetOutput(stderr)
	matrixPath := flags.String("matrix", "", "canonical matrix YAML path")
	manifestPath := flags.String("manifest", "", "source manifest YAML path")
	out := flags.String("out", "", "feed output path")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *matrixPath == "" || *manifestPath == "" {
		fmt.Fprintln(stderr, "corpus feed: --matrix and --manifest are required")
		return 2
	}
	matrixYAML, err := os.ReadFile(*matrixPath)
	if err != nil {
		fmt.Fprintf(stderr, "corpus feed: read matrix: %v\n", err)
		return 1
	}
	manifestYAML, err := os.ReadFile(*manifestPath)
	if err != nil {
		fmt.Fprintf(stderr, "corpus feed: read manifest: %v\n", err)
		return 1
	}
	feed, err := corpus.GenerateFeed(corpus.FeedInput{
		MatrixYAML:   matrixYAML,
		ManifestYAML: manifestYAML,
		GeneratedAt:  time.Now().UTC(),
	})
	if err != nil {
		fmt.Fprintf(stderr, "corpus feed: %v\n", err)
		return 1
	}
	return writeJSONPath(feed, *out, stdout, stderr, "corpus feed")
}

func corpusAttestCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("corpus attest", flag.ContinueOnError)
	flags.SetOutput(stderr)
	snapshotPath := flags.String("snapshot", "", "snapshot JSON path")
	corpusID := flags.String("corpus-id", "", "corpus identifier")
	projectID := flags.String("project-id", "", "consumer project identifier")
	verdict := flags.String("verdict", "corpus_admissible", "corpus verdict")
	confidence := flags.String("confidence", "high", "corpus confidence")
	out := flags.String("out", "", "attestation output path")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *snapshotPath == "" || *corpusID == "" || *projectID == "" {
		fmt.Fprintln(stderr, "corpus attest: --snapshot, --corpus-id, and --project-id are required")
		return 2
	}
	snapshot, err := readSnapshot(*snapshotPath)
	if err != nil {
		fmt.Fprintf(stderr, "corpus attest: %v\n", err)
		return 1
	}
	files := make([]string, 0, len(snapshot.Sources))
	for _, source := range snapshot.Sources {
		files = append(files, source.Path+" "+source.Hash)
	}
	statement, err := corpus.GenerateCorpusAttestation(corpus.CorpusAttestationOptions{
		CorpusID:       *corpusID,
		ProjectID:      *projectID,
		ScannerVersion: Version,
		Verdict:        *verdict,
		Confidence:     *confidence,
		FilesScanned:   snapshot.TotalFiles,
		ScannedFiles:   files,
		Now:            time.Now().UTC(),
		Metadata: map[string]any{
			"commit":     snapshot.Commit,
			"branch":     snapshot.Branch,
			"repository": snapshot.Repository,
		},
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
		return 0
	}
	if err := writeFile(*out, func(w io.Writer) error { return corpus.WriteAttestation(w, statement) }); err != nil {
		fmt.Fprintf(stderr, "corpus attest: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "generated corpus attestation into %s\n", filepath.Clean(*out))
	return 0
}

func readOnlyBefore(root string) (guard.Snapshot, bool, error) {
	if err := guard.RequireClean(root); err != nil {
		if errors.Is(err, guard.ErrNotGitRepo) {
			return guard.Snapshot{}, false, nil
		}
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
